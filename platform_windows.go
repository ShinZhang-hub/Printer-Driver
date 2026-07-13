//go:build windows

package main

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

func writeConsole(s string) {
	// Go writes UTF-8 to stdout, but when piped to PowerShell it's decoded using
	// the system code page (e.g., GBK for zh-CN), causing garbled text.
	// Convert UTF-8 to the active console code page for pipe compatibility.
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	proc := kernel32.NewProc("GetStdHandle")
	handle, _, _ := proc.Call(uintptr(0xFFFFFFF5)) // STD_OUTPUT_HANDLE = -11
	var mode uint32
	if kernel32.NewProc("GetConsoleMode").Call(handle, uintptr(unsafe.Pointer(&mode))) == 0 {
		// Not a console — piped. Convert to active code page.
		utf16, _ := syscall.UTF16FromString(s)
		cp := kernel32.NewProc("GetConsoleOutputCP")
		codePage, _, _ := cp.Call()
		if codePage == 0 {
			codePage = 65001 // UTF-8 fallback
		}
		// WideCharToMultiByte: CP_ACP -> UTF-16 -> code page
		wcm := kernel32.NewProc("WideCharToMultiByte")
		cbLen, _, _ := wcm.Call(codePage, 0,
			uintptr(unsafe.Pointer(&utf16[0])), uintptr(len(utf16)-1),
			0, 0, 0, 0)
		if cbLen > 0 {
			buf := make([]byte, cbLen)
			wcm.Call(codePage, 0,
				uintptr(unsafe.Pointer(&utf16[0])), uintptr(len(utf16)-1),
				uintptr(unsafe.Pointer(&buf[0])), cbLen, 0, 0)
			os.Stdout.Write(buf)
			return
		}
	}
	// Console or fallback — Go handles UTF-8 correctly via WriteConsoleW
	os.Stdout.WriteString(s)
}

func hideConsole() {
	syscall.NewLazyDLL("kernel32.dll").NewProc("FreeConsole").Call()
}

func isAdmin() bool {
	ret, _, _ := syscall.NewLazyDLL("shell32.dll").NewProc("IsUserAnAdmin").Call()
	return ret != 0
}

func elevateSelf() {
	exe, _ := os.Executable()
	verb, _ := syscall.UTF16PtrFromString("runas")
	file, _ := syscall.UTF16PtrFromString(exe)
	args, _ := syscall.UTF16PtrFromString(strings.Join(os.Args[1:], " "))
	syscall.NewLazyDLL("shell32.dll").NewProc("ShellExecuteW").Call(
		0, uintptr(unsafe.Pointer(verb)), uintptr(unsafe.Pointer(file)),
		uintptr(unsafe.Pointer(args)), 0, 5)
}

func isShiftPressed() bool {
	mod := syscall.NewLazyDLL("user32.dll")
	proc := mod.NewProc("GetAsyncKeyState")
	for i := 0; i < 5; i++ {
		for _, vk := range []uintptr{0x10, 0xA0, 0xA1} {
			ret, _, _ := proc.Call(vk)
			if int16(ret) < 0 {
				return true
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}

func killExistingInstance() {
	data, err := os.ReadFile(lockFilePath())
	if err != nil {
		return
	}
	parts := strings.SplitN(string(data), "\n", 2)
	if len(parts) < 1 {
		return
	}
	pidStr := strings.TrimSpace(parts[0])
	if _, err := strconv.Atoi(pidStr); err != nil {
		return
	}
	exec.Command("taskkill", "/F", "/PID", pidStr).Run()
	os.Remove(lockFilePath())
}

func processExists(pid int) bool {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	proc := kernel32.NewProc("OpenProcess")
	handle, _, _ := proc.Call(0x0400, 0, uintptr(pid))
	if handle == 0 {
		return false
	}
	defer kernel32.NewProc("CloseHandle").Call(handle)
	var exitCode uint32
	proc2 := kernel32.NewProc("GetExitCodeProcess")
	ret, _, _ := proc2.Call(handle, uintptr(unsafe.Pointer(&exitCode)))
	_ = ret
	return exitCode == 259
}
