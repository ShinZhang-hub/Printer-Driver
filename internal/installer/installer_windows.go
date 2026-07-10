//go:build windows

package installer

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
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

	return output, fmt.Errorf("%s failed (exit %d):\n%s", name, cmd.ProcessState.ExitCode(), output)
}

func installDriver(p Params) error {
	log.Info("pnputil /add-driver %s", p.InfFile)
	_, err := runCmd("pnputil", "/add-driver", p.InfFile)
	if err != nil {
		return err
	}
	log.Info("pnputil: driver installed")
	return nil
}

func createPort(p Params) error {
	removePortByName(p.PortName)
	script := `C:\WINDOWS\System32\Printing_Admin_Scripts\ja-JP\prnport.vbs`
	_, err := runCmd("cscript", "//NoLogo", "//B", script,
		"-a", "-r", p.PortName,
		"-h", p.PrinterIP,
		"-o", p.Protocol,
		"-n", fmt.Sprintf("%d", p.PortNum))
	if err != nil {
		return err
	}
	log.Info("Port %s created", p.PortName)
	return nil
}

func addPrinter(p Params) error {
	if err := createPort(p); err != nil {
		return fmt.Errorf("create port failed: %w", err)
	}

	replaced := false
	foundName := FindPrinterByIP(p.PrinterIP)
	if foundName != "" {
		log.Info("Found existing printer at %s (%s), removing...", p.PrinterIP, foundName)
		if err := removePrinterByName(foundName); err != nil {
			return err
		}
		replaced = true
	}

	_, err := runCmd("rundll32", "printui.dll,PrintUIEntry",
		"/if", "/b", p.PrinterName,
		"/r", p.PortName,
		"/m", p.ModelName)
	if err != nil {
		return err
	}

	if replaced {
		log.Info("Printer %s removed and reinstalled", p.PrinterName)
	} else {
		log.Info("Printer %s added", p.PrinterName)
	}
	ResultMessage = fmt.Sprintf("%s installed", p.PrinterName)
	return nil
}

func FindPrinterByIP(ip string) string {
	out, err := runCmd("powershell", "-NoProfile", "-Command",
		fmt.Sprintf(`Get-Printer | Where-Object { try { $port = Get-PrinterPort -Name $_.PortName -ErrorAction Stop; $port.HostAddress -eq "%s" } catch {} } | Select-Object -ExpandProperty Name`, ip))
	if err != nil || out == "" {
		return ""
	}
	return strings.TrimSpace(out)
}

func ListPrinters(exclude string) string {
	names := getOtherPrinterNames(exclude)
	if len(names) == 0 {
		return "none"
	}
	return strings.Join(names, ", ")
}

func getOtherPrinterNames(exclude string) []string {
	out, err := runCmd("powershell", "-NoProfile", "-Command",
		`Get-Printer | Select-Object -ExpandProperty Name`)
	if err != nil || out == "" {
		return nil
	}
	var names []string
	for _, name := range strings.Split(strings.TrimSpace(out), "\n") {
		name = strings.TrimSpace(name)
		if name != "" && name != exclude {
			names = append(names, name)
		}
	}
	return names
}

func DeletePrintersFromFile(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	for _, name := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		removePrinterByName(name)
	}
	return nil
}

func closeProgressWindow() {
	closeWindowByTitle("Printer Driver Installation")
	killProcessByName("ffcomist.exe")
}

func setDefault(p Params) error {
	_, err := runCmd("rundll32", "printui.dll,PrintUIEntry",
		"/y", "/n", p.PrinterName)
	if err != nil {
		return err
	}
	log.Info("Printer %s set as default", p.PrinterName)
	return nil
}
