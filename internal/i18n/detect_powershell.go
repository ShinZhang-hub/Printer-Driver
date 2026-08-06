//go:build !windows

package i18n

import (
	"os/exec"
	"strings"
)

// detectWindows is only ever called on Windows (detect_windows.go provides the
// real implementation there). This non-Windows copy exists so cross-compilation
// (e.g. building the Windows exe from macOS) still compiles.
func detectWindows() string {
	out, err := exec.Command("powershell", "-NoProfile", "-Command",
		"(Get-WinSystemLocale).Name").Output()
	if err == nil {
		lang := strings.TrimSpace(string(out))
		lang = strings.Split(lang, "-")[0]
		switch lang {
		case "ja", "ko", "zh":
			return lang
		}
	}
	return "en"
}
