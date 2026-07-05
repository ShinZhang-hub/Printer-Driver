package main

import (
	"fmt"
	"os"

	"printer-installer/internal/config"
	"printer-installer/internal/detector"
	"printer-installer/internal/installer"
	"printer-installer/internal/scanner"
	"printer-installer/internal/web"
)

func main() {
	args := parseArgs()

	cfg := config.Load(args.configURL)

	if args.shiftHeld || detector.IsShiftHeld() || args.ui {
		web.StartAdminPanel(cfg)
		return
	}

	printers := scanner.ScanNetwork(cfg.Subnet)
	if len(printers) == 0 {
		fmt.Println("未发现打印机")
		os.Exit(1)
	}
	p := printers[0]

	driver := cfg.LookupDriver(p.Model, p.Brand)
	if driver == nil {
		fmt.Printf("未找到 %s %s 的驱动配置\n", p.Brand, p.Model)
		os.Exit(1)
	}

	if detector.IsDriverInstalled(driver.ID) {
		if args.silent {
			fmt.Println("驱动已安装，跳过")
			return
		}
		if !detector.ConfirmReinstall() {
			return
		}
	}

	url := cfg.PlatformURL(driver)
	installer.Download(url, driver.Checksum)
	installer.Run(driver.InstallArgs)
}

type args struct {
	silent    bool
	shiftHeld bool
	configURL string
	ui        bool
}

func parseArgs() args {
	var a args
	for _, arg := range os.Args[1:] {
		switch arg {
		case "--silent":
			a.silent = true
		case "--shift":
			a.shiftHeld = true
		case "--ui":
			a.ui = true
		}
	}
	if v := os.Getenv("CONFIG_URL"); v != "" {
		a.configURL = v
	} else {
		a.configURL = "http://config.internal.company.com"
	}
	return a
}
