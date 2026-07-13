package main

import (
	"bufio"
	_ "embed"
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"printer-installer/internal/config"
	"printer-installer/internal/drvpack"
	"printer-installer/internal/embeds"
	"printer-installer/internal/i18n"
	"printer-installer/internal/winui"
	"printer-installer/internal/installer"
	"printer-installer/internal/log"
	"printer-installer/internal/scanner"
	"printer-installer/internal/web"
)

//go:embed config.json
var embeddedConfig []byte

func main() {
	ip := flag.String("ip", "", "Printer IP address")
	driversDir := flag.String("drivers", "drivers", "Driver directory")
	extracted := flag.String("extracted", "", "Extracted driver directory (skip extraction)")
	name := flag.String("name", "", "Printer name")
	setDefault := flag.Bool("default", true, "Set as default printer")
	admin := flag.Bool("admin", false, "Open admin panel")
	debugPrinters := flag.Bool("debug-printers", false, "List all printers and exit")
	debugOthers := flag.String("debug-others", "", "Test getOtherPrinterNames with given exclude name")
	hashPassword := flag.Bool("hash-password", false, "Read password from stdin and print bcrypt hash")
	deletePrintersFile := flag.String("delete-printers-file", "", "Path to file with printer names to delete (one per line)")
	discover := flag.Bool("discover", false, "Probe printer and output discovered info")
	noSnmp := flag.Bool("no-snmp", false, "Skip SNMP probe when discovering")
	listLocations := flag.Bool("list-locations", false, "List all location names from config")
	resolveLocation := flag.String("resolve-location", "", "Resolve location name to PrinterIP and PrinterName")
	printerAtIP := flag.String("printer-at-ip", "", "Returns printer name at given IP if exists")
	printerLocation := flag.String("printer-location", "", "Returns location name for given printer name if found")
	uiEnv := flag.Bool("ui-env", false, "Output all UI strings for detected language as shell env vars")
	location := flag.String("location", "", "Install using config location by name")
	flag.Parse()

	if *uiEnv {
		if runtime.GOOS == "windows" {
			writeConsole(i18n.AllEnv(""))
		} else {
			fmt.Print(i18n.AllEnv(""))
		}
		return
	}

	if *debugPrinters {
		fmt.Println(installer.ListPrinters(""))
		return
	}

	if *debugOthers != "" {
		fmt.Printf("getOtherPrinterNames(%q): %v\n", *debugOthers, installer.ListPrinters(*debugOthers))
		return
	}

	if *deletePrintersFile != "" {
		if err := installer.DeletePrintersFromFile(*deletePrintersFile); err != nil {
			fmt.Fprintf(os.Stderr, "error deleting printers: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if *hashPassword {
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		pw := scanner.Text()
		if err := scanner.Err(); err != nil {
			fmt.Fprintf(os.Stderr, "error reading password: %v\n", err)
			os.Exit(1)
		}
		pw = strings.TrimRight(pw, "\n\r")
		hash, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(hash))
		return
	}

	needsAdmin := !*discover && !*listLocations && *resolveLocation == "" && *printerAtIP == "" && *printerLocation == "" && !*uiEnv && !*debugPrinters && *deletePrintersFile == "" && !*hashPassword && *debugOthers == ""
	if runtime.GOOS == "windows" && !isAdmin() && needsAdmin {
		elevateSelf()
		return
	}
	if runtime.GOOS == "windows" && isAdmin() && needsAdmin {
		hideConsole()
	}

	adminMode := *admin || isShiftPressed()

	if adminMode {
		killExistingInstance()
	}

	log.Init()
	defer log.Close()

	if *discover || *listLocations || *resolveLocation != "" || *printerAtIP != "" || *printerLocation != "" {
		var cfg *config.Config
		if *noSnmp {
			cfg = config.ParseEmbedded(embeddedConfig)
			if cfg == nil {
				cfg = config.LoadRemote(embeddedConfig)
			}
		} else {
			cfg = config.LoadRemote(embeddedConfig)
		}
		if *discover {
			localIP := localIPAddr()
			printerIP := *ip
			if printerIP == "" {
				printerIP = cfg.GetPrinterIP(localIP)
			}
			model := ""
			brand := ""
			if !*noSnmp {
				p, err := scanner.ProbeSingleIP(printerIP)
				if err == nil {
					model = p.Model
					brand = p.Brand
				}
			}
			if model == "" {
				model = cfg.GetPrinterModel(localIP)
				if model == "" {
					model = cfg.LookupModelByIP(printerIP)
				}
			}
			loc := cfg.MatchLocation(localIP)
			locName := ""
			if loc != nil {
				locName = loc.Name
			}
			fmt.Printf("IP=%s\nModel=%s\nBrand=%s\nLocation=%s\n", printerIP, model, brand, locName)
			return
		}
		if *listLocations {
			names := make([]string, 0, len(cfg.Locations))
			for _, loc := range cfg.Locations {
				names = append(names, loc.Name)
			}
			if len(names) == 0 {
				os.Exit(1)
			}
			fmt.Println(strings.Join(names, ","))
			return
		}
		if *resolveLocation != "" {
			for _, loc := range cfg.Locations {
				if loc.Name == *resolveLocation {
					for _, p := range loc.AllPrinters() {
						fmt.Printf("IP=%s\nName=%s\n", p.IP, p.Name)
					}
					return
				}
			}
			os.Exit(1)
		}
		if *printerAtIP != "" {
			name := installer.FindPrinterByIP(*printerAtIP)
			if name != "" {
				fmt.Println(name)
			} else {
				os.Exit(1)
			}
			return
		}
		if *printerLocation != "" {
			loc := cfg.LookupLocationByPrinterName(*printerLocation)
			if loc != nil {
				fmt.Println(loc.Name)
			} else {
				os.Exit(1)
			}
			return
		}
	}

	cfg := config.LoadRemote(embeddedConfig)

	if *location != "" {
		var printers []config.PrinterInfo
		for _, loc := range cfg.Locations {
			if loc.Name == *location {
				printers = loc.AllPrinters()
				break
			}
		}
		if len(printers) == 0 {
			fmt.Fprintf(os.Stderr, "error: location %q not found in config\n", *location)
			os.Exit(1)
		}
		log.Info("Using location: %s (%d printers)", *location, len(printers))
		if err := installAllPrinters(cfg, *driversDir, printers, *setDefault); err != nil {
			log.Error("Installation failed: %v", err)
			os.Exit(1)
		}
		os.WriteFile(filepath.Join(os.TempDir(), "printer-installer-status.txt"), []byte(installer.ResultMessage), 0644)
		log.Info("Installation successful")
		return
	}

	if adminMode {
		url, done := web.StartAdminPanel(cfg, embeddedConfig, func(installIP, installName string) error {
			return runInstall(cfg, *driversDir, installIP, installName, *setDefault)
		})
		writeLockFile(url)
		defer removeLockFile()
		<-done
		return
	}

	if *ip == "" && *name == "" && *location == "" && !*noSnmp {
		showNativeUI(cfg)
		return
	}

	localIP := localIPAddr()

	if *ip == "" {
		*ip = cfg.GetPrinterIP(localIP)
	}
	if *ip == "" {
		log.Error("Printer IP not specified. Set it in config.json or use --ip")
		os.Exit(1)
	}
	if *name == "" {
		*name = cfg.GetPrinterName(localIP)
	}
	if !*setDefault && cfg.SetDefault != nil {
		*setDefault = *cfg.SetDefault
	}

	model := ""
	brand := ""
	p, err := scanner.ProbeSingleIP(*ip)
	if err == nil {
		brand = p.Brand
		model = p.Model
		log.Info("Discovered: %s %s (%s)", p.Brand, p.Model, p.IP)
	} else {
		log.Warn("Printer probe failed: %v, trying config model", err)
		model = cfg.GetPrinterModel(localIP)
		if model == "" {
			log.Error("SNMP probe failed and printer_model not configured")
			os.Exit(1)
		}
		log.Info("Using config model: %s", model)
	}

	if *name == "" {
		*name = model
	}

	pkg, err := resolveDriver(*driversDir, brand, *extracted)
	if err != nil {
		log.Error("%v", err)
		os.Exit(1)
	}
	defer pkg.Cleanup()

	entry := pkg.FindModelStrict(model)
	if entry == nil {
		if runtime.GOOS == "darwin" && pkg.FirstEntry() != nil {
			entry = pkg.FirstEntry()
			log.Warn("No exact match for %s, using generic driver: %s", model, entry.ModelName)
		} else {
			log.Error("No driver found for model %s", model)
			fmt.Println("Available models:")
			models := map[string]bool{}
			for _, e := range pkg.Entries {
				if !models[e.ModelName] {
					fmt.Printf("  - %s\n", e.ModelName)
					models[e.ModelName] = true
				}
			}
			os.Exit(1)
		}
	}

	if runtime.GOOS == "darwin" {
		log.Info("Matched: %s (PPD: %s)", entry.ModelName, filepath.Base(entry.InfFile))
	} else {
		log.Info("Matched: %s (INF: %s)", entry.ModelName, filepath.Base(entry.InfFile))
	}
	if err := installPrinter(cfg, entry.InfFile, entry.ModelName, p.IP, *name, portNum(cfg, localIP), protocol(cfg, localIP), *setDefault); err != nil {
		log.Error("Installation failed: %v", err)
		os.Exit(1)
	}
	os.WriteFile(filepath.Join(os.TempDir(), "printer-installer-status.txt"), []byte(installer.ResultMessage), 0644)
	log.Info("Installation successful")
}

const lockFileName = "printer-admin.lock"

func lockFilePath() string {
	return filepath.Join(os.TempDir(), lockFileName)
}

func writeLockFile(url string) {
	data := fmt.Sprintf("%d\n%s", os.Getpid(), url)
	os.WriteFile(lockFilePath(), []byte(data), 0644)
}

func removeLockFile() {
	os.Remove(lockFilePath())
}

func resolveDriver(driversDir, brand, extracted string) (*drvpack.DriverPackage, error) {
	if extracted != "" {
		if runtime.GOOS == "darwin" {
			entries, err := drvpack.ParsePPDDirectory(extracted)
			if err != nil {
				return nil, fmt.Errorf("failed to parse extracted PPD directory: %w", err)
			}
			return &drvpack.DriverPackage{WorkDir: extracted, Entries: entries}, nil
		}
		entries, err := drvpack.ParseInfDirectory(extracted)
		if err != nil {
			return nil, fmt.Errorf("failed to parse extracted directory: %w", err)
		}
		return &drvpack.DriverPackage{WorkDir: extracted, Entries: entries}, nil
	}
	brandDir := filepath.Join(driversDir, brand)

	if runtime.GOOS == "darwin" {
		dmgPath, err := drvpack.FindDmg(brandDir)
		if err != nil && brandDir != driversDir {
			dmgPath, err = drvpack.FindDmg(driversDir)
		}
		if err == nil {
			pkg, pkgErr := drvpack.OpenDMG(dmgPath)
			if pkgErr == nil {
				return pkg, nil
			}
			log.Warn("DMG extraction failed: %v, using embedded driver", pkgErr)
		}

		tmpDir, err := os.MkdirTemp("", "printer-installer-embedded-")
		if err != nil {
			return nil, fmt.Errorf("failed to create temp directory: %w", err)
		}
		if err := embeds.ExtractMacPPD(tmpDir); err != nil {
			os.RemoveAll(tmpDir)
			return nil, fmt.Errorf("failed to extract embedded PPD: %w", err)
		}
		log.Info("Using embedded macOS driver")

		entries, err := drvpack.ParsePPDDirectory(tmpDir)
		if err != nil {
			os.RemoveAll(tmpDir)
			return nil, fmt.Errorf("failed to parse embedded PPD: %w", err)
		}
		return &drvpack.DriverPackage{WorkDir: tmpDir, Entries: entries}, nil
	}

	exePath, err := drvpack.FindExe(brandDir)
	if err == nil {
		pkg, pkgErr := drvpack.Open(exePath)
		if pkgErr == nil {
			return pkg, nil
		}
		log.Warn("Driver extraction failed: %v, using embedded driver", pkgErr)
	}

	tmpDir, err := os.MkdirTemp("", "printer-installer-embedded-")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	if err := embeds.ExtractDrivers(tmpDir); err != nil {
		os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("failed to extract embedded drivers: %w", err)
	}
	log.Info("Using embedded driver")

	entries, err := drvpack.ParseInfDirectory(tmpDir)
	if err != nil {
		os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("failed to parse embedded drivers: %w", err)
	}
	return &drvpack.DriverPackage{WorkDir: tmpDir, Entries: entries}, nil
}

func runInstall(cfg *config.Config, driversDir, printerIP, printerName string, setDefault bool) error {
	p, probeErr := scanner.ProbeSingleIP(printerIP)
	brand := ""
	model := cfg.LookupModelByIP(printerIP)
	if probeErr == nil {
		brand = p.Brand
		if p.Model != "" {
			model = p.Model
		}
	}
	if model == "" {
		return fmt.Errorf("cannot determine printer model (SNMP failed and no printer_model in config for %s)", printerIP)
	}
	pkg, err := resolveDriver(driversDir, brand, "")
	if err != nil {
		return err
	}
	defer pkg.Cleanup()

	entry := pkg.FindModelStrict(model)
	if entry == nil {
		if runtime.GOOS == "darwin" && pkg.FirstEntry() != nil {
			entry = pkg.FirstEntry()
		} else {
			return fmt.Errorf("no matching driver model found in package")
		}
	}
	if printerName == "" {
		printerName = entry.ModelName
	}
	return installPrinter(cfg, entry.InfFile, entry.ModelName, printerIP, printerName, cfg.PortNumber, cfg.Protocol, setDefault)
}

func installAllPrinters(cfg *config.Config, driversDir string, printers []config.PrinterInfo, setDefault bool) error {
	var names []string
	for i, p := range printers {
		log.Info("Installing printer %d/%d: %s @ %s", i+1, len(printers), p.Name, p.IP)
		defaultThis := setDefault && i == 0
		if err := runInstall(cfg, driversDir, p.IP, p.Name, defaultThis); err != nil {
			return fmt.Errorf("printer %s: %w", p.Name, err)
		}
		names = append(names, p.Name)
	}
	if len(names) == 1 {
		installer.ResultMessage = names[0] + " installed"
	} else {
		installer.ResultMessage = strings.Join(names, ", ") + " installed"
	}
	return nil
}

func installPrinter(cfg *config.Config, infFile, modelName, printerIP, printerName string, portNum int, protocol string, setDefault bool) error {
	return installer.Install(installer.Params{
		InfFile:     infFile,
		ModelName:   modelName,
		PrinterIP:   printerIP,
		PrinterName: printerName,
		PortName:    fmt.Sprintf("IP_%s", printerIP),
		PortNum:     portNum,
		Protocol:    protocol,
		SetDefault:  setDefault,
	})
}

func portNum(cfg *config.Config, localIP string) int {
	if loc := cfg.MatchLocation(localIP); loc != nil && loc.PortNumber > 0 {
		return loc.PortNumber
	}
	return cfg.PortNumber
}

func protocol(cfg *config.Config, localIP string) string {
	if loc := cfg.MatchLocation(localIP); loc != nil && loc.Protocol != "" {
		return loc.Protocol
	}
	return cfg.Protocol
}

func localIPAddr() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ip := ipnet.IP.To4(); ip != nil {
				return ip.String()
			}
		}
	}
	return ""
}

func showNativeUI(cfg *config.Config) {
	localIP := localIPAddr()
	detectedLoc := ""
	if loc := cfg.MatchLocation(localIP); loc != nil {
		detectedLoc = loc.Name
	}

	allLocNames := make([]string, len(cfg.Locations))
	for i, loc := range cfg.Locations {
		allLocNames[i] = loc.Name
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

	result := winui.Run(detectedLoc, allLocNames, deleteItems)
	if result == nil || result.Cancelled {
		return
	}

	log.Info("WinUI: location=%s overwrite=%t", result.Location, result.Overwrite)

	var printers []config.PrinterInfo
	for _, loc := range cfg.Locations {
		if loc.Name == result.Location {
			printers = loc.AllPrinters()
			break
		}
	}
	if len(printers) == 0 {
		log.Error("No printers for location %q", result.Location)
		return
	}

	if err := installAllPrinters(cfg, "", printers, true); err != nil {
		log.Error("Installation failed: %v", err)
	}
}
