package detector

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
)

// IsDriverInstalled 通过 wmic 查询是否已安装
func IsDriverInstalled(driverID string) bool {
	cmd := exec.Command("wmic", "product", "where", fmt.Sprintf("name like '%%%s%%'", driverID), "get", "name")
	out, err := cmd.Output()
	if err != nil {
		log.Printf("查询已安装驱动失败: %v", err)
		return false
	}
	return strings.Contains(string(out), driverID)
}

// IsShiftHeld 检测 Shift 键是否按下
func IsShiftHeld() bool {
	// 用 GetAsyncKeyState 查 Shift 键
	// 临时方案：检查是否有 shift 标志文件
	if _, err := exec.LookPath("powershell"); err == nil {
		cmd := exec.Command("powershell",
			"-Command",
			"[System.Windows.Forms.Control]::ModifierKeys -band [System.Windows.Forms.Keys]::Shift")
		out, _ := cmd.Output()
		return strings.TrimSpace(string(out)) != "0"
	}
	return false
}

// ConfirmReinstall 弹出 Windows 原生确认框
func ConfirmReinstall() bool {
	cmd := exec.Command("powershell",
		"-Command",
		`$r = [System.Windows.Forms.MessageBox]::Show('已安装相同驱动，是否重新安装？','驱动已存在','YesNo','Question')
		 if ($r -eq 'Yes') { exit 0 } else { exit 1 }`)
	err := cmd.Run()
	return err == nil
}
