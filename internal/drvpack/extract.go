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
)

var (
	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	procGetShortPath   = kernel32.NewProc("GetShortPathNameW")
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
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-time.After(timeout):
		// 杀整个进程树（InnoSetup 可能衍生子进程）
		exec.Command("taskkill", "/f", "/t", "/pid", fmt.Sprintf("%d", cmd.Process.Pid)).Run()
		cmd.Process.Kill()
		return fmt.Errorf("超时 (%v)", timeout)
	case err := <-done:
		return err
	}
}

func cleanupExtractProcesses() {
	exec.Command("taskkill", "/f", "/im", "ffcomist.exe").Run()
	exec.Command("taskkill", "/f", "/im", "Launcher.exe").Run()
}

func extract(exePath string) (string, error) {
	exePath, err := filepath.Abs(exePath)
	if err != nil {
		return "", fmt.Errorf("无法解析驱动路径: %w", err)
	}

	workDir, err := os.MkdirTemp("", "printer-installer-extract-")
	if err != nil {
		return "", fmt.Errorf("创建临时解压目录失败: %w", err)
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
			return "", fmt.Errorf("创建临时解压目录失败: %w", err)
		}

		err := runWithTimeout(60*time.Second, exePath, args...)
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
		return "", fmt.Errorf("无法解压 %s（已尝试静默参数：%s）\n可尝试手动解压后用 --extracted 指定目录", exePath, strings.Join(attemptErrs, "; "))
	}
	return "", fmt.Errorf("无法解压 %s\n可尝试手动解压后用 --extracted 指定目录", exePath)
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
