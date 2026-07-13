//go:build darwin

package main

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

func isAdmin() bool    { return true }
func elevateSelf()     {}
func hideConsole()     {}

func isShiftPressed() bool {
	cmd := exec.Command("osascript", "-l", "JavaScript", "-e",
		"ObjC.import('Cocoa'); ($.NSEvent.modifierFlags & 131072) != 0 ? '1' : '0'")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "1"
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
	exec.Command("kill", "-9", pidStr).Run()
	os.Remove(lockFilePath())
}

func processExists(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}
