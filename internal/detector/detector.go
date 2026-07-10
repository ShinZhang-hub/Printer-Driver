package detector

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
)

// IsDriverInstalled checks if a driver is installed via wmic
func IsDriverInstalled(driverID string) bool {
	cmd := exec.Command("wmic", "product", "where", fmt.Sprintf("name like '%%%s%%'", driverID), "get", "name")
	out, err := cmd.Output()
	if err != nil {
		log.Printf("failed to query installed driver: %v", err)
		return false
	}
	return strings.Contains(string(out), driverID)
}

// IsShiftHeld detects whether the Shift key is pressed
func IsShiftHeld() bool {
	// Uses GetAsyncKeyState for Shift detection
	// Temporary workaround: check for shift flag file
	if _, err := exec.LookPath("powershell"); err == nil {
		cmd := exec.Command("powershell",
			"-Command",
			"[System.Windows.Forms.Control]::ModifierKeys -band [System.Windows.Forms.Keys]::Shift")
		out, _ := cmd.Output()
		return strings.TrimSpace(string(out)) != "0"
	}
	return false
}

// ConfirmReinstall shows a Windows native confirmation dialog
func ConfirmReinstall() bool {
	cmd := exec.Command("powershell",
		"-Command",
		`$r = [System.Windows.Forms.MessageBox]::Show('Driver already installed, reinstall?','Driver Exists','YesNo','Question')
		 if ($r -eq 'Yes') { exit 0 } else { exit 1 }`)
	err := cmd.Run()
	return err == nil
}
