//go:build windows

package installer

import (
	"fmt"
	"syscall"
	"unsafe"
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
	printerAttributeDirect = 0x0004
	printerAttributeLocal  = 0x0080
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

func removePrinterByName(name string) {
	h, err := openPrinter(name)
	if err != nil {
		return
	}
	defer closePrinter(h)
	procDeletePrinter.Call(uintptr(h))
}

func addPrinterAPI(driverName, portName, printerName string) error {
	driverPtr, _ := syscall.UTF16PtrFromString(driverName)
	portPtr, _ := syscall.UTF16PtrFromString(portName)
	namePtr, _ := syscall.UTF16PtrFromString(printerName)
	printProcPtr, _ := syscall.UTF16PtrFromString("WinPrint")

	info := printerInfo2{
		pPrinterName:    namePtr,
		pPortName:       portPtr,
		pDriverName:     driverPtr,
		pPrintProcessor: printProcPtr,
		Attributes:      printerAttributeDirect | printerAttributeLocal,
	}

	r, _, err := procAddPrinter.Call(0, 2, uintptr(unsafe.Pointer(&info)))
	if r == 0 {
		return fmt.Errorf("AddPrinter 失败: %v", err)
	}
	procClosePrinter.Call(r)
	return nil
}

func setDefaultPrinter(name string) error {
	namePtr, _ := syscall.UTF16PtrFromString(name)
	r, _, err := procSetDefaultPrinter.Call(uintptr(unsafe.Pointer(namePtr)))
	if r == 0 {
		return fmt.Errorf("SetDefaultPrinter 失败: %v", err)
	}
	return nil
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
