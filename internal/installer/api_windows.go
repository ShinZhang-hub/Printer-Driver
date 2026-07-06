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

	procFindWindow  = user32.NewProc("FindWindowW")
	procPostMessage = user32.NewProc("PostMessageW")

	procCreateToolhelp32Snapshot = kernel32.NewProc("CreateToolhelp32Snapshot")
	procProcess32First           = kernel32.NewProc("Process32FirstW")
	procProcess32Next            = kernel32.NewProc("Process32NextW")
	procOpenProcess              = kernel32.NewProc("OpenProcess")
	procTerminateProcess         = kernel32.NewProc("TerminateProcess")
	procCloseHandle              = kernel32.NewProc("CloseHandle")
)

const (
	processTerminate   = 0x0001
	th32csSnapProcess  = 0x00000002
	wmClose            = 0x0010
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
	pServerName          *uint16
	pPrinterName         *uint16
	pShareName           *uint16
	pPortName            *uint16
	pDriverName          *uint16
	pComment             *uint16
	pLocation            *uint16
	pDevMode             uintptr
	pSepFile             *uint16
	pPrintProcessor      *uint16
	pDatatype            *uint16
	pParameters          *uint16
	pSecurityDescriptor  uintptr
	Attributes           uint32
	Priority             uint32
	DefaultPriority      uint32
	StartTime            uint32
	UntilTime            uint32
	Status               uint32
	cJobs                uint32
	AveragePPM           uint32
}

func openPrinter(name string) (syscall.Handle, error) {
	var h syscall.Handle
	namePtr, _ := syscall.UTF16PtrFromString(name)
	r, _, err := procOpenPrinter.Call(uintptr(unsafe.Pointer(namePtr)), uintptr(unsafe.Pointer(&h)), 0)
	if r == 0 {
		return 0, fmt.Errorf("OpenPrinter(%s) 失败: %v", name, err)
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
		return nil
	}

	r, _, err := procDeletePrinter.Call(uintptr(h))
	closePrinter(h)
	if r == 0 {
		if isAccessDenied(err) {
			log.Warn("DeletePrinter(%s) 返回 Access is denied，尝试释放占用后重试", name)
			if recoverErr := recoverPrinterDeleteLock(); recoverErr != nil {
				return fmt.Errorf("DeletePrinter(%s) 失败: %v；占用恢复失败: %w", name, err, recoverErr)
			}

			h, retryOpenErr := openPrinter(name)
			if retryOpenErr != nil {
				return nil
			}
			r, _, err = procDeletePrinter.Call(uintptr(h))
			closePrinter(h)
			if r == 0 {
				log.Warn("DeletePrinter(%s) 重试仍失败，尝试 printui 兜底", name)
				if fallbackErr := fallbackDeletePrinterByName(name); fallbackErr != nil {
					return fmt.Errorf("DeletePrinter(%s) 重试失败: %v；兜底删除失败: %w", name, err, fallbackErr)
				}
			}
		} else {
			return fmt.Errorf("DeletePrinter(%s) 失败: %v", name, err)
		}
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !printerExists(name) {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("删除打印机 %s 超时: 对象仍然存在", name)
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
			return fmt.Errorf("停止服务 %s 失败: %w", name, stopErr)
		}
	} else {
		if err := waitServiceState(name, "STOPPED", timeout/2); err != nil {
			return err
		}
	}

	if _, err := runCmd("sc", "start", name); err != nil {
		return fmt.Errorf("启动服务 %s 失败: %w", name, err)
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
	return fmt.Errorf("等待服务 %s 进入 %s 超时", name, want)
}

func fallbackDeletePrinterByName(name string) error {
	if _, err := runCmd("rundll32", "printui.dll,PrintUIEntry", "/dl", "/n", name); err != nil {
		return fmt.Errorf("printui: %v", err)
	}
	if !printerExists(name) {
		return nil
	}
	return fmt.Errorf("printui 执行后打印机 %s 仍然存在", name)
}



func removePortByName(name string) {
	script := `C:\WINDOWS\System32\Printing_Admin_Scripts\ja-JP\prnport.vbs`
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

	// upper-case target name for comparison
	nameUpper := make([]uint16, len(name)+1)
	for i, c := range name {
		if c >= 'a' && c <= 'z' {
			nameUpper[i] = uint16(c - 0x20)
		} else {
			nameUpper[i] = uint16(c)
		}
	}
	nameUpper[len(name)] = 0

	r, _, _ := procProcess32First.Call(snapshot, uintptr(unsafe.Pointer(&pe)))
	if r == 0 {
		return
	}

	for {
		match := true
		for i := 0; nameUpper[i] != 0; i++ {
			c := pe.szExeFile[i]
			if c >= 'a' && c <= 'z' {
				c -= 0x20
			}
			if c != nameUpper[i] {
				match = false
				break
			}
		}
		if match {
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
