package log

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	file *os.File
	mu   sync.Mutex
)

func Init() error {
	dir := filepath.Join(os.TempDir(), "PrinterInstaller")
	os.MkdirAll(dir, 0755)

	path := filepath.Join(dir, time.Now().Format("2006-01-02")+".log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("创建日志文件失败: %w", err)
	}

	// 空文件写入 UTF-8 BOM，让 Windows 记事本能正确识别中文
	info, _ := f.Stat()
	if info.Size() == 0 {
		f.Write([]byte{0xEF, 0xBB, 0xBF})
	}

	file = f
	Info("=== 启动 ===")
	return nil
}

func Close() {
	if file != nil {
		file.Close()
	}
}

func Info(format string, args ...interface{}) {
	write("INFO", format, args...)
}

func Warn(format string, args ...interface{}) {
	write("WARN", format, args...)
}

func Error(format string, args ...interface{}) {
	write("ERROR", format, args...)
}

func write(level, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	line := fmt.Sprintf("%s [%s] %s", time.Now().Format("15:04:05"), level, msg)

	mu.Lock()
	if file != nil {
		io.WriteString(file, line+"\n")
		file.Sync()
	}
	mu.Unlock()

	w := io.Writer(os.Stdout)
	if level == "ERROR" || level == "WARN" {
		w = os.Stderr
	}
	fmt.Fprintln(w, line)
}
