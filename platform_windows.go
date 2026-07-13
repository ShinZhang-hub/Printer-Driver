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
