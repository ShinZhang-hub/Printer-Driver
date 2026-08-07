//go:build darwin

package main

import "printer-installer/internal/config"

func showNativeUI(cfg *config.Config) {
	// macOS uses shell script + JXA, no Fyne UI
}
