//go:build darwin

package installer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"printer-installer/internal/log"
)

func runCmd(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	output := string(out)
	if err != nil {
		return output, fmt.Errorf("%s failed: %s\n%w", name, output, err)
	}
	return output, nil
}

func installDriver(p Params) error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("root privileges required, please run with sudo")
	}

	ppdDir := "/Library/Printers/PPDs/Contents/Resources"
	if err := os.MkdirAll(ppdDir, 0755); err != nil {
		return fmt.Errorf("failed to create PPD directory: %w", err)
	}

	src := p.InfFile
	dst := filepath.Join(ppdDir, filepath.Base(src))
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("failed to read PPD file: %w", err)
	}
	if err := os.WriteFile(dst, data, 0644); err != nil {
		return fmt.Errorf("failed to copy PPD to %s: %w", dst, err)
	}
	log.Info("PPD installed: %s", dst)
	return nil
}

func addPrinter(p Params) error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("root privileges required, please run with sudo")
	}

	uri := fmt.Sprintf("socket://%s:%d", p.PrinterIP, p.PortNum)
	replaced := false

	out, _ := runCmd("lpstat", "-v")
	if strings.Contains(out, "://"+p.PrinterIP+":") {
		log.Info("Found existing printer at %s, removing...", p.PrinterIP)
		for _, line := range splitLines(out) {
			if strings.Contains(line, "://"+p.PrinterIP+":") {
				name := extractPrinterNameBeforeURI(line, "://"+p.PrinterIP+":")
				if name != "" {
					runCmd("lpadmin", "-x", name)
				}
			}
		}
		replaced = true
		time.Sleep(500 * time.Millisecond)
	}

	log.Info("lpadmin -E -p %s -v %s -P %s", p.PrinterName, uri, p.InfFile)
	_, err := runCmd("lpadmin", "-E", "-p", p.PrinterName, "-v", uri, "-P", p.InfFile)
	if err != nil {
		return err
	}

	runCmd("cupsenable", p.PrinterName)
	runCmd("cupsaccept", p.PrinterName)

	if replaced {
		log.Info("Printer %s removed and reinstalled", p.PrinterName)
	} else {
		log.Info("Printer %s added", p.PrinterName)
	}
	ResultMessage = fmt.Sprintf("%s installed", p.PrinterName)
	return nil
}

func FindPrinterByIP(ip string) string {
	out, _ := runCmd("lpstat", "-v")
	ipMatch := "://" + ip
	for _, line := range splitLines(out) {
		if strings.Contains(line, ipMatch) {
			name := extractPrinterNameBeforeURI(line, ipMatch)
			if name != "" {
				return name
			}
		}
	}
	return ""
}

func extractPrinterNameBeforeURI(line, uri string) string {
	uriIdx := strings.Index(line, "://")
	if uriIdx < 0 {
		return ""
	}
	sepIdx := strings.LastIndex(line[:uriIdx], ":")
	if sepIdx < 0 {
		sepIdx = strings.LastIndex(line[:uriIdx], "\uFF1A")
	}
	if sepIdx < 0 {
		return ""
	}
	prefix := line[:sepIdx]
	lastWord := ""
	i := 0
	for i < len(prefix) {
		if prefix[i] < 128 && prefix[i] != ' ' && prefix[i] != '\t' {
			start := i
			for i < len(prefix) && prefix[i] < 128 && prefix[i] != ' ' && prefix[i] != '\t' {
				i++
			}
			lastWord = prefix[start:i]
		} else {
			i++
		}
	}
	return lastWord
}

func setDefault(p Params) error {
	_, err := runCmd("lpadmin", "-d", p.PrinterName)
	if err != nil {
		return err
	}
	log.Info("Printer %s set as default", p.PrinterName)
	return nil
}

func ListPrinters(exclude string) string {
	names := getOtherPrinterNames(exclude)
	if len(names) == 0 {
		return "none"
	}
	return strings.Join(names, ", ")
}

func ListPrintersWithIPs() map[string]string {
	out, _ := runCmd("lpstat", "-v")
	result := make(map[string]string)
	for _, line := range splitLines(out) {
		name := extractPrinterNameBeforeURI(line, "")
		if name == "" {
			continue
		}
		if idx := strings.Index(line, "socket://"); idx >= 0 {
			rest := line[idx+len("socket://"):]
			if end := strings.IndexAny(rest, ": \n"); end >= 0 {
				result[name] = rest[:end]
			}
		}
	}
	return result
}

func DeletePrinterByName(name string) error {
	_, err := runCmd("lpadmin", "-x", name)
	return err
}

func getOtherPrinterNames(exclude string) []string {
	out, err := runCmd("lpstat", "-v")
	var debug strings.Builder
	debug.WriteString(fmt.Sprintf("err=%v\n", err))
	debug.WriteString(fmt.Sprintf("exclude=%q\n", exclude))
	debug.WriteString(fmt.Sprintf("out:\n%s\n", out))
	var names []string
	if err == nil {
		for _, line := range splitLines(out) {
			name := extractPrinterNameBeforeURI(line, "")
			debug.WriteString(fmt.Sprintf("  line=%q -> name=%q\n", line, name))
			if name != "" && name != exclude {
				names = append(names, name)
			}
		}
	}
	debug.WriteString(fmt.Sprintf("result=%v\n", names))
	os.WriteFile("/tmp/printer-installer-lpstat-debug.txt", []byte(debug.String()), 0644)
	if err != nil {
		return nil
	}
	return names
}

func DeletePrintersFromFile(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	for _, name := range strings.Split(string(data), "\n") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		runCmd("lpadmin", "-x", name)
	}
	return nil
}

func closeProgressWindow() {}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
