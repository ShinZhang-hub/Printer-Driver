//go:build !windows && !darwin

package installer

import "fmt"

func installDriver(p Params) error {
	return fmt.Errorf("pnputil install not supported on this platform")
}

func addPrinter(p Params) error {
	return fmt.Errorf("printui.dll not supported on this platform")
}

func setDefault(p Params) error {
	return fmt.Errorf("printui.dll not supported on this platform")
}

func ListPrinters(exclude string) string { return "none" }

func DeletePrintersFromFile(filePath string) error { return nil }

func FindPrinterByIP(ip string) string { return "" }

func closeProgressWindow() {}
