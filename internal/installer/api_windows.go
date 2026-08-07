//go:build windows

package installer

import (
	"fmt"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"printer-installer/internal/log"
)

var (
	winspool = syscall.NewLazyDLL("winspool.drv")
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	procOpenPrinter       = winspool.NewProc("OpenPrinterW")
	procClosePrinter      = winspool.NewProc("ClosePrinter")
	procDeletePrinter     = winspool.NewProc("DeletePrinter")
	procAddPrinter        = winspool.NewProc("AddPrinterW")
	procSetDefaultPrinter = winspool.NewProc("SetDefaultPrinterW")

	procFindWindow               = user32.NewProc("FindWindowW")
	procPostMessage              = user32.NewProc("PostMessageW")
	procEnumWindows              = user32.NewProc("EnumWindows")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	procShowWindow               = user32.NewProc("ShowWindow")

	procCreateToolhelp32Snapshot = kernel32.NewProc("CreateToolhelp32Snapshot")
	procProcess32First           = kernel32.NewProc("Process32FirstW")
	procProcess32Next            = kernel32.NewProc("Process32NextW")
	procOpenProcess              = kernel32.NewProc("OpenProcess")
	procTerminateProcess         = kernel32.NewProc("TerminateProcess")
	procCloseHandle              = kernel32.NewProc("CloseHandle")
)

const (
	processTerminate      = 0x0001
	th32csSnapProcess     = 0x00000002
	wmClose               = 0x0010
	swHide                = 0x0000
	printerAttributeLocal = 0x0080
)

type processEntry32W struct {
	dwSize              uint32
	cntUsage            uint32
	th32ProcessID       uint32
	th32DefaultHeapID   uintptr
	th32ModuleID        uint32
	cntThreads          uint32
	th32ParentProcessID uint32
	pcPriClassBase      int32
	dwFlags             uint32
	szExeFile           [260]uint16
}

type printerInfo2 struct {
	pServerName         *uint16
	pPrinterName        *uint16
	pShareName          *uint16
	pPortName           *uint16
	pDriverName         *uint16
	pComment            *uint16
	pLocation           *uint16
	pDevMode            uintptr
	pSepFile            *uint16
	pPrintProcessor     *uint16
	pDatatype           *uint16
	pParameters         *uint16
	pSecurityDescriptor uintptr
	Attributes          uint32
	Priority            uint32
	DefaultPriority     uint32
	StartTime           uint32
	UntilTime           uint32
	Status              uint32
	cJobs               uint32
	AveragePPM          uint32
}

func openPrinter(name string) (syscall.Handle, error) {
	var h syscall.Handle
	namePtr, _ := syscall.UTF16PtrFromString(name)
	r, _, err := procOpenPrinter.Call(uintptr(unsafe.Pointer(namePtr)), uintptr(unsafe.Pointer(&h)), 0)
	if r == 0 {
		return 0, fmt.Errorf("OpenPrinter(%s) failed: %v", name, err)
	}
	return h, nil
}

func closePrinter(h syscall.Handle) {
	procClosePrinter.Call(uintptr(h))
}

func printerExists(name string) bool {
	h, err := openPrinter(name)
	if err != nil {
		return false
	}
	closePrinter(h)
	return true
}

func removePrinterByName(name string) error {
	h, err := openPrinter(name)
	if err != nil {
		log.Warn("openPrinter(%s) failed, printer may not exist: %v", name, err)
		return nil
	}

	r, _, err := procDeletePrinter.Call(uintptr(h))
	closePrinter(h)
	if r == 0 {
		if isAccessDenied(err) {
			log.Warn("DeletePrinter(%s) returned Access is denied, releasing locks and retrying", name)
			if recoverErr := recoverPrinterDeleteLock(); recoverErr != nil {
				return fmt.Errorf("DeletePrinter(%s) failed: %v; lock recovery failed: %w", name, err, recoverErr)
			}

			h, retryOpenErr := openPrinter(name)
			if retryOpenErr != nil {
				return nil
			}
			r, _, err = procDeletePrinter.Call(uintptr(h))
			closePrinter(h)
			if r == 0 {
				log.Warn("DeletePrinter(%s) retry failed, falling back to printui", name)
				if fallbackErr := fallbackDeletePrinterByName(name); fallbackErr != nil {
					return fmt.Errorf("DeletePrinter(%s) retry failed: %v; fallback also failed: %w", name, err, fallbackErr)
				}
			}
		} else {
			return fmt.Errorf("DeletePrinter(%s) failed: %v", name, err)
		}
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !printerExists(name) {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("deleting printer %s timed out: object still exists", name)
}

func isAccessDenied(err error) bool {
	if err == nil {
		return false
	}
	if errno, ok := err.(syscall.Errno); ok {
		return errno == 5
	}
	return strings.Contains(strings.ToLower(err.Error()), "access is denied")
}

func recoverPrinterDeleteLock() error {
	killProcessByName("splwow64.exe")
	killProcessByName("PrintIsolationHost.exe")

	if err := restartService("spooler", 15*time.Second); err != nil {
		return err
	}
	time.Sleep(500 * time.Millisecond)
	return nil
}

func restartService(name string, timeout time.Duration) error {
	_, stopErr := runCmd("sc", "stop", name)
	if stopErr != nil {
		if err := waitServiceState(name, "STOPPED", timeout/2); err != nil {
			return fmt.Errorf("stopping service %s failed: %w", name, stopErr)
		}
	} else {
		if err := waitServiceState(name, "STOPPED", timeout/2); err != nil {
			return err
		}
	}

	if _, err := runCmd("sc", "start", name); err != nil {
		return fmt.Errorf("starting service %s failed: %w", name, err)
	}
	return waitServiceState(name, "RUNNING", timeout/2)
}

func waitServiceState(name, want string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	want = strings.ToUpper(want)
	for time.Now().Before(deadline) {
		out, err := runCmd("sc", "query", name)
		if err == nil && strings.Contains(strings.ToUpper(out), "STATE") && strings.Contains(strings.ToUpper(out), want) {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("waiting for service %s to reach %s timed out", name, want)
}

func fallbackDeletePrinterByName(name string) error {
	if _, err := runCmd("rundll32", "printui.dll,PrintUIEntry", "/dl", "/n", name); err != nil {
		return fmt.Errorf("printui: %v", err)
	}
	if !printerExists(name) {
		return nil
	}
	return fmt.Errorf("printer %s still exists after printui fallback", name)
}

func removePortByName(name string) {
	// PrintManagement module is present by default; prnport.vbs needs the
	// optional Printing Admin Scripts feature, so try PowerShell first.
	runCmd("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command",
		fmt.Sprintf(`Remove-PrinterPort -Name "%s" -ErrorAction SilentlyContinue`, name))
	script := findPrnportVbs()
	if script == "" {
		return
	}
	runCmd("cscript", "//NoLogo", "//B", script, "-d", "-r", name)
}

func closeWindowByTitle(title string) {
	titlePtr, _ := syscall.UTF16PtrFromString(title)
	r, _, _ := procFindWindow.Call(0, uintptr(unsafe.Pointer(titlePtr)))
	if r != 0 {
		procPostMessage.Call(r, wmClose, 0, 0)
	}
}

func killProcessByName(name string) {
	snapshot, _, _ := procCreateToolhelp32Snapshot.Call(th32csSnapProcess, 0)
	if snapshot == uintptr(syscall.InvalidHandle) {
		return
	}
	defer procCloseHandle.Call(snapshot)

	var pe processEntry32W
	pe.dwSize = uint32(unsafe.Sizeof(pe))

	r, _, _ := procProcess32First.Call(snapshot, uintptr(unsafe.Pointer(&pe)))
	if r == 0 {
		return
	}

	for {
		if processNameMatches(pe.szExeFile[:], name) {
			h, _, _ := procOpenProcess.Call(processTerminate, 0, uintptr(pe.th32ProcessID))
			if h != 0 {
				procTerminateProcess.Call(h, 0)
				procCloseHandle.Call(h)
			}
		}
		r, _, _ = procProcess32Next.Call(snapshot, uintptr(unsafe.Pointer(&pe)))
		if r == 0 {
			break
		}
	}
}

func hideWindowByTitle(title string) {
	titlePtr, _ := syscall.UTF16PtrFromString(title)
	r, _, _ := procFindWindow.Call(0, uintptr(unsafe.Pointer(titlePtr)))
	if r != 0 {
		procShowWindow.Call(r, swHide)
	}
}

func processNameMatches(exe []uint16, name string) bool {
	if len(name) == 0 {
		return false
	}
	for i := 0; i < len(name); i++ {
		c := exe[i]
		if c >= 'a' && c <= 'z' {
			c -= 0x20
		}
		if c != uint16(name[i]) {
			return false
		}
	}
	return exe[len(name)] == 0
}

// hidePIDSet is read by hideWindowEnumCB during EnumWindows. It is only
// accessed from the single driver-UI suppression loop goroutine.
var hidePIDSet map[uint32]bool

func hideWindowEnumCB(hwnd uintptr, lparam uintptr) uintptr {
	var pid uint32
	procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	if hidePIDSet[pid] {
		procShowWindow.Call(hwnd, swHide)
	}
	return 1
}

// hideWindowsOfProcesses hides all top-level windows owned by the given
// process image names (e.g. the Fujifilm driver installer UI), without
// killing the process, so a silent driver installation is not interrupted.
func hideWindowsOfProcesses(names ...string) {
	snapshot, _, _ := procCreateToolhelp32Snapshot.Call(th32csSnapProcess, 0)
	if snapshot == uintptr(syscall.InvalidHandle) {
		return
	}
	defer procCloseHandle.Call(snapshot)

	pidSet := make(map[uint32]bool)
	var pe processEntry32W
	pe.dwSize = uint32(unsafe.Sizeof(pe))
	r, _, _ := procProcess32First.Call(snapshot, uintptr(unsafe.Pointer(&pe)))
	for r != 0 {
		for _, n := range names {
			if processNameMatches(pe.szExeFile[:], n) {
				pidSet[pe.th32ProcessID] = true
				break
			}
		}
		r, _, _ = procProcess32Next.Call(snapshot, uintptr(unsafe.Pointer(&pe)))
	}
	if len(pidSet) == 0 {
		return
	}
	hidePIDSet = pidSet
	procEnumWindows.Call(syscall.NewCallback(hideWindowEnumCB), 0)
}

// HideDriverWindowsLoop repeatedly hides the Fujifilm driver installer's own
// windows (ffcomist.exe / Launcher.exe and the "Printer Driver Installation"
// window) so the user never sees them pop up during installation. It returns
// a stop function to call when installation finishes.
func HideDriverWindowsLoop(interval time.Duration) func() {
	stop := make(chan struct{})
	go func() {
		defer func() {
			recover() // best effort: never let a background goroutine kill the app
		}()
		for {
			select {
			case <-stop:
				return
			default:
			}
			hideWindowsOfProcesses("ffcomist.exe", "Launcher.exe")
			hideWindowByTitle("Printer Driver Installation")
			select {
			case <-stop:
				return
			case <-time.After(interval):
			}
		}
	}()
	return func() { close(stop) }
}
