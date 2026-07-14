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

func findPrnportVbs() string {
	base := `C:\WINDOWS\System32\Printing_Admin_Scripts`
	for _, locale := range []string{"en-US", "zh-CN", "ja-JP", "ko-KR"} {
		path := base + "\\" + locale + "\\prnport.vbs"
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

func createPort(p Params) error {
	removePortByName(p.PortName)
	script := findPrnportVbs()
	if script == "" {
		return fmt.Errorf("prnport.vbs not found")
	}
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
	// Remove any existing printer at this IP FIRST, so even if subsequent
	// steps fail the old printer is already gone.
	replaced := false
	foundName := FindPrinterByIP(p.PrinterIP)
	if foundName != "" {
		log.Info("Found existing printer at %s (%s), removing before install...", p.PrinterIP, foundName)
		if err := removePrinterByName(foundName); err != nil {
			log.Warn("Failed to remove existing printer %s: %v", foundName, err)
		} else {
			replaced = true
		}
	}

	if err := installDriver(p); err != nil {
		return fmt.Errorf("install driver failed: %w", err)
	}

	if err := createPort(p); err != nil {
		return fmt.Errorf("create port failed: %w", err)
	}

	_, err := runCmd("rundll32", "printui.dll,PrintUIEntry",
		"/if", "/b", p.PrinterName,
		"/f", p.InfFile,
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

func DeletePrinterByName(name string) error {
	name = strings.TrimSpace(name)
	log.Info("Remove-Printer -Name %q", name)
	_, err := runCmd("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command",
		fmt.Sprintf(`Remove-Printer -Name "%s" -Confirm:$false -ErrorAction SilentlyContinue`, name))
	if err != nil {
		log.Warn("Remove-Printer %q failed: %v, trying Win32 API", name, err)
		return removePrinterByName(name)
	}
	log.Info("Printer %q removed via PowerShell", name)
	return nil
}

func FindPrinterByIP(ip string) string {
	out, err := runCmd("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command",
		fmt.Sprintf(`Get-Printer | ForEach-Object { $p = Get-PrinterPort -Name $_.PortName -ErrorAction SilentlyContinue; if ($p) { $addr = if ($p.Name -match '^IP_(\d+\.\d+\.\d+\.\d+)$') { $matches[1] } elseif ($p.HostAddress) { $p.HostAddress } else { $null }; if ($addr -eq "%s") { $_.Name } } }`, ip))
	if err != nil {
		log.Warn("FindPrinterByIP(%s) PowerShell error: %v", ip, err)
		return ""
	}
	out = strings.TrimSpace(out)
	if out != "" {
		log.Info("FindPrinterByIP(%s) found: %s", ip, out)
	}
	return out
}

func ListPrintersWithIPs() map[string]string {
	out, err := runCmd("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command",
		`Get-Printer | ForEach-Object { $name = $_.Name; $port = Get-PrinterPort -Name $_.PortName -ErrorAction SilentlyContinue; if ($port) { $ip = if ($port.Name -match '^IP_(\d+\.\d+\.\d+\.\d+)$') { $matches[1] } elseif ($port.HostAddress) { $port.HostAddress } else { $null }; if ($ip) { $name + "=" + $ip } } }`)
	if err != nil || out == "" {
		return nil
	}
	result := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		if idx := strings.Index(line, "="); idx > 0 {
			result[line[:idx]] = line[idx+1:]
		}
	}
	return result
}

func ListPrinters(exclude string) string {
	names := getOtherPrinterNames(exclude)
	if len(names) == 0 {
		return "none"
	}
	return strings.Join(names, ", ")
}

func getOtherPrinterNames(exclude string) []string {
	out, err := runCmd("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command",
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
