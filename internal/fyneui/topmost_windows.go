//go:build windows

package fyneui

import (
	"syscall"
	"time"
	"unsafe"
)

var (
	user32 = syscall.NewLazyDLL("user32.dll")

	procEnumWindows              = user32.NewProc("EnumWindows")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	procSetWindowPos             = user32.NewProc("SetWindowPos")
	procGetCurrentProcessId      = syscall.NewLazyDLL("kernel32.dll").NewProc("GetCurrentProcessId")
)

const (
	swpNoSize     = 0x0001
	swpNoMove     = 0x0002
	swpNoActivate = 0x0010
)

var hwndTopmost = ^uintptr(0) // HWND_TOPMOST = (HWND)-1

var (
	ourPID      uint32
	foundWindow bool
)

func enumWindowCB(hwnd uintptr, lparam uintptr) uintptr {
	var pid uint32
	procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	if pid == ourPID {
		foundWindow = true
		procSetWindowPos.Call(hwnd, uintptr(hwndTopmost), 0, 0, 0, 0,
			swpNoSize|swpNoMove|swpNoActivate)
	}
	return 1
}

// setTopmostOnce moves every top-level window of this process to the topmost
// z-order. Returns true if at least one window was found and pinned.
func setTopmostOnce() bool {
	foundWindow = false
	r, _, _ := procGetCurrentProcessId.Call()
	ourPID = uint32(r)
	procEnumWindows.Call(syscall.NewCallback(enumWindowCB), 0)
	return foundWindow
}

// bringToFront forces the installer window to appear in front of all other
// windows. The native window is created shortly after ShowAndRun begins, so it
// polls briefly until the window exists and then pins it to the topmost layer.
func bringToFront() {
	defer func() {
		recover() // best effort: never let a background goroutine kill the app
	}()
	for i := 0; i < 30; i++ {
		if setTopmostOnce() {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}
