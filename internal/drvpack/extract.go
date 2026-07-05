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

func runQuiet(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd
}

func runWithTimeout(timeout time.Duration, name string, args ...string) error {
	cmd := runQuiet(name, args...)
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

func extract(exePath string) (string, error) {
	exePath, err := filepath.Abs(exePath)
	if err != nil {
		return "", fmt.Errorf("无法解析驱动路径: %w", err)
	}

	workDir := filepath.Join(filepath.Dir(exePath), ".extract-auto")
	os.RemoveAll(workDir)
	os.MkdirAll(workDir, 0755)

	// NSIS 静默解压: /S (silent) + /D=<dir> (输出目录，必须用短路径避免空格)
	runWithTimeout(60*time.Second, exePath, "/S", fmt.Sprintf("/D=%s", shortPath(workDir)))

	// 清残留子进程
	exec.Command("taskkill", "/f", "/im", "ffcomist.exe").Run()
	exec.Command("taskkill", "/f", "/im", "Launcher.exe").Run()

	if root := findDriverRoot(workDir); root != "" {
		return root, nil
	}

	os.RemoveAll(workDir)
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


