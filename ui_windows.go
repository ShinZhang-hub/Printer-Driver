//go:build windows

package main

import (
	"strings"

	"printer-installer/internal/config"
	"printer-installer/internal/fyneui"
	"printer-installer/internal/i18n"
	"printer-installer/internal/installer"
	"printer-installer/internal/log"
)
func showNativeUI(cfg *config.Config) {
	localIP := detectedLocalIP(cfg)
	detectedLoc := ""
	if loc := cfg.MatchLocation(localIP); loc != nil {
		detectedLoc = loc.Name
	}

	printerList := installer.ListPrinters("")
	printerNames := strings.FieldsFunc(printerList, func(r rune) bool { return r == ',' || r == '\n' })
	deleteItems := make([]string, 0)
	for _, pn := range printerNames {
		pn = strings.TrimSpace(pn)
		if pn != "" && pn != "none" {
			deleteItems = append(deleteItems, pn)
		}
	}

	// Get all installed printers with their IPs for checkbox disable logic
	printersIPs := installer.ListPrintersWithIPs()

	// For each location, list configured printer IPs
	locIPs := make(map[string][]string)
	locNames := make(map[string][]string)
	for _, loc := range cfg.Locations {
		var ips []string
		var names []string
		for _, p := range loc.AllPrinters() {
			ips = append(ips, p.IP)
			names = append(names, p.Name)
		}
		locIPs[loc.Name] = ips
		locNames[loc.Name] = names
	}

	allLocNames := make([]string, len(cfg.Locations))
	for i, loc := range cfg.Locations {
		allLocNames[i] = loc.Name
	}

	result, message := fyneui.Run(detectedLoc, allLocNames, deleteItems, printersIPs, locIPs, locNames, func(res *fyneui.Result) string {
		if res == nil {
			return ""
		}
		log.Info("WinUI: location=%s overwrite=%t", res.Location, res.Overwrite)

		// Delete checked printers first
		var delParts []string
		for _, name := range res.DeleteNames {
			log.Info("Removing printer: %s", name)
			if err := installer.DeletePrinterByName(name); err != nil {
				log.Warn("Failed to remove printer %s: %v", name, err)
				delParts = append(delParts, i18n.T("FAIL_PREFIX")+" "+name+": "+err.Error())
			} else {
				delParts = append(delParts, i18n.T("REMOVED_MSG", name))
			}
		}

		var printers []config.PrinterInfo
		for _, loc := range cfg.Locations {
			if loc.Name == res.Location {
				printers = loc.AllPrinters()
				break
			}
		}

		// Install / skip / overwrite messages
		var installMsg string
		if len(printers) > 0 {
			if err := installAllPrinters(cfg, "drivers", printers, true, res.Overwrite); err != nil {
				log.Error("Installation failed: %v", err)
				installMsg = i18n.T("FAIL_PREFIX") + " " + err.Error()
			} else {
				installMsg = installer.ResultMessage
			}
		}

		var allParts []string
		if installMsg != "" {
			allParts = append(allParts, installMsg)
		}
		delJoined := strings.Join(delParts, "\n")
		if delJoined != "" {
			if installMsg != "" {
				allParts = append(allParts, "")
			}
			allParts = append(allParts, delJoined)
		}
		return strings.Join(allParts, "\n")
	})
	if result == nil || result.Cancelled {
		return
	}
	if message != "" {
		showMessageBox(i18n.T("WINDOW_TITLE"), message)
	}
}
