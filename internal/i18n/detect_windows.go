//go:build windows

package i18n

import (
	"syscall"
	"unsafe"
)

func detectWindows() string {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	p := kernel32.NewProc("GetUserDefaultUILanguage")
	langID, _, _ := p.Call()
	if langID == 0 {
		return "en"
	}
	primary := langID & 0x3FF
	switch primary {
	case 0x04:
		return "zh"
	case 0x11:
		return "ja"
	case 0x12:
		return "ko"
	}
	getLocaleInfo := kernel32.NewProc("GetLocaleInfoW")
	buf := make([]uint16, 16)
	ret, _, _ := getLocaleInfo.Call(uintptr(langID), 0x59, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if ret > 0 {
		lang := syscall.UTF16ToString(buf[:ret-1])
		switch lang {
		case "en":
			return "en"
		case "zh":
			return "zh"
		case "ja":
			return "ja"
		case "ko":
			return "ko"
		}
	}
	return "en"
}
