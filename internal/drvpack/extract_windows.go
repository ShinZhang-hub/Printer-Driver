//go:build windows

package drvpack

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"printer-installer/internal/installer"
)

var (
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	procGetShortPath = kernel32.NewProc("GetShortPathNameW")
)

func shortPath(path string) string {
	p, _ := syscall.UTF16PtrFromString(path)
	buf := make([]uint16, 260)
	r, _, _ := procGetShortPath.Call(uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(&buf[0])), 260)
	if r == 0 {
		return path
	}
	return syscall.UTF16ToString(buf)
}

func runWithTimeout(timeout time.Duration, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-time.After(timeout):
		// 杀整个进程树（InnoSetup 可能衍生子进程）
		runTaskkill("/f", "/t", "/pid", fmt.Sprintf("%d", cmd.Process.Pid))
		cmd.Process.Kill()
		return fmt.Errorf("timeout (%v)", timeout)
	case err := <-done:
		return err
	}
}

// runTaskkill invokes taskkill with its console window hidden so the user never
// sees a terminal flash during installation.
func runTaskkill(args ...string) {
	cmd := exec.Command("taskkill", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	cmd.Run()
}

func cleanupExtractProcesses() {
	runTaskkill("/f", "/im", "ffcomist.exe")
	runTaskkill("/f", "/im", "Launcher.exe")
}

func extract(exePath string) (string, error) {
	exePath, err := filepath.Abs(exePath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve driver path: %w", err)
	}

	workDir, err := os.MkdirTemp("", "printer-installer-extract-")
	if err != nil {
		return "", fmt.Errorf("failed to create temp extraction directory: %w", err)
	}

	shortWorkDir := shortPath(workDir)
	attempts := [][]string{
		{"/S", "/D" + shortWorkDir},
		{"/s", "/d" + shortWorkDir},
	}

	var attemptErrs []string
	for _, args := range attempts {
		os.RemoveAll(workDir)
		if err := os.MkdirAll(workDir, 0755); err != nil {
			return "", fmt.Errorf("failed to create temp extraction directory: %w", err)
		}

		stopHide := installer.HideDriverWindowsLoop(200 * time.Millisecond)
		err := runWithTimeout(60*time.Second, exePath, args...)
		stopHide()
		cleanupExtractProcesses()
		if err != nil {
			attemptErrs = append(attemptErrs, fmt.Sprintf("%s: %v", strings.Join(args, " "), err))
		}
		if root := findDriverRoot(workDir); root != "" {
			return root, nil
		}
	}

	os.RemoveAll(workDir)
	if len(attemptErrs) > 0 {
		return "", fmt.Errorf("failed to extract %s (tried silent flags: %s)\ntry manual extraction and use --extracted", exePath, strings.Join(attemptErrs, "; "))
	}
	return "", fmt.Errorf("failed to extract %s\ntry manual extraction and use --extracted", exePath)
}

func findDriverRoot(dir string) string {
	var root string
	filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(path), ".inf") {
			root = filepath.Dir(path)
			return filepath.SkipAll
		}
		return nil
	})
	return root
}
