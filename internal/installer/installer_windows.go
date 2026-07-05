//go:build windows

package installer

import (
	"fmt"
	"os/exec"
	"syscall"

	"printer-installer/internal/log"
)

func runCmd(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.CombinedOutput()
	output := string(out)

	if err == nil {
		return output, nil
	}

	if name == "pnputil" && cmd.ProcessState != nil && cmd.ProcessState.ExitCode() == 5 {
		return output, nil
	}

	return output, fmt.Errorf("%s 失败 (exit %d):\n%s", name, cmd.ProcessState.ExitCode(), output)
}

func installDriver(p Params) error {
	log.Info("pnputil /add-driver %s", p.InfFile)
	_, err := runCmd("pnputil", "/add-driver", p.InfFile)
	if err != nil {
		return err
	}
	log.Info("pnputil: 驱动已安装")
	return nil
}

func addPort(p Params) error {
	log.Info("创建端口 %s [%s:%d]", p.PortName, p.PrinterIP, p.PortNum)
	removePrinterByName(p.PrinterName)
	removePortByName(p.PortName)
	// use prnport.vbs (standard Windows component, no dialog with //B)
	script := `C:\WINDOWS\System32\Printing_Admin_Scripts\ja-JP\prnport.vbs`
	_, err := runCmd("cscript", "//NoLogo", "//B", script,
		"-a", "-r", p.PortName,
		"-h", p.PrinterIP,
		"-o", p.Protocol,
		"-n", fmt.Sprintf("%d", p.PortNum))
	if err != nil {
		return err
	}
	log.Info("端口 %s 已创建", p.PortName)
	return nil
}

func addPrinter(p Params) error {
	removePrinterByName(p.PrinterName)
	err := addPrinterAPI(p.ModelName, p.PortName, p.PrinterName)
	if err != nil {
		return err
	}
	log.Info("打印机 %s 已添加", p.PrinterName)
	return nil
}

func closeProgressWindow() {
	closeWindowByTitle("Printer Driver Installation")
	killProcessByName("ffcomist.exe")
}

func setDefault(p Params) error {
	err := setDefaultPrinter(p.PrinterName)
	if err != nil {
		return err
	}
	log.Info("打印机 %s 已设为默认", p.PrinterName)
	return nil
}
