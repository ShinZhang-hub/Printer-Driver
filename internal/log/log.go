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
		return fmt.Errorf("failed to create log file: %w", err)
	}

	// write UTF-8 BOM for Windows Notepad compatibility
	info, _ := f.Stat()
	if info.Size() == 0 {
		f.Write([]byte{0xEF, 0xBB, 0xBF})
	}

	file = f
	Info("=== Start ===")
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

	w := io.Writer(os.Stderr)
	fmt.Fprintln(w, line)
}
