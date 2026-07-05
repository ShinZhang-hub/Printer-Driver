package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"printer-installer/internal/config"
	"printer-installer/internal/drvpack"
	"printer-installer/internal/installer"
	"printer-installer/internal/log"
	"printer-installer/internal/scanner"
)

func main() {
	log.Init()
	defer log.Close()

	cfgPath := flag.String("config", "config.json", "配置文件路径")
	ip := flag.String("ip", "", "打印机 IP 地址")
	driversDir := flag.String("drivers", "", "驱动包目录（覆盖配置文件）")
	extracted := flag.String("extracted", "", "已解压的驱动目录，跳过 exe 解压")
	name := flag.String("name", "", "打印机名称（覆盖配置文件/位置匹配）")
	setDefault := flag.Bool("default", true, "设为默认打印机（覆盖配置文件）")
	flag.Parse()

	cfg := config.LoadFile(*cfgPath)
	localIP := localIPAddr()

	if *ip == "" {
		*ip = cfg.GetPrinterIP(localIP)
	}
	if *name == "" {
		*name = cfg.GetPrinterName(localIP)
	}
	if *driversDir == "" {
		*driversDir = cfg.DriversDir
	}
	if !*setDefault && cfg.SetDefault != nil {
		*setDefault = *cfg.SetDefault
	}

	if *ip == "" {
		locName := ""
		if loc := cfg.MatchLocation(localIP); loc != nil {
			locName = loc.Name
		}
		fmt.Printf(`用法: printer-installer [选项]

选项:
  --config <文件>   配置文件路径，默认 config.json
  --ip <IP>        打印机 IP 地址
  --drivers <目录>  驱动包目录（内含品牌名子目录）
  --extracted <目录> 已解压的驱动目录，跳过 exe 解压步骤
  --name <名称>     打印机显示名称
  --default        设为默认打印机

当前配置:
  本机 IP:      %s
  匹配位置:     %s
  目标打印机:   %s
  打印机名称:   %s
  drivers_dir:  %s
  port_number:  %d
  protocol:     %s
  set_default:  %v
日志文件: %%TEMP%%\PrinterInstaller\*.log
`, localIP, locName, cfg.GetPrinterIP(localIP), cfg.GetPrinterName(localIP),
			cfg.DriversDir, cfg.PortNumber, cfg.Protocol, derefBool(cfg.SetDefault))
		os.Exit(1)
	}

	if loc := cfg.MatchLocation(localIP); loc != nil {
		log.Info("位置: %s → %s (%s)", loc.Name, loc.PrinterName, loc.PrinterIP)
	}

	log.Info("目标IP: %s", *ip)
	p, err := scanner.ProbeSingleIP(*ip)
	if err != nil {
		log.Error("探测打印机失败: %v", err)
		os.Exit(1)
	}
	log.Info("发现: %s %s (%s)", p.Brand, p.Model, p.IP)

	workDir := *extracted
	if workDir == "" {
		brandDir := drvpack.DriverDir(*driversDir, p.Brand)
		exePath, err := drvpack.FindExe(brandDir)
		if err != nil {
			log.Error("在 %s 中未找到 %s 驱动包\n请放入驱动 exe 或用 --extracted 指定已解压目录", brandDir, p.Brand)
			os.Exit(1)
		}
		log.Info("驱动包: %s", exePath)

		pkg, err := drvpack.Open(exePath)
		if err != nil {
			log.Error("解压驱动包失败: %v\n可尝试用 --extracted 指定已解压的目录", err)
			os.Exit(1)
		}
		defer pkg.Cleanup()
		workDir = pkg.WorkDir
		log.Info("解压到: %s", workDir)
		log.Info("共 %d 个驱动条目", len(pkg.Entries))

		entry := pkg.FindModelStrict(p.Model)
		if entry == nil {
			log.Error("未找到匹配 %s 的型号", p.Model)
			fmt.Println("可用型号:")
			models := map[string]bool{}
			for _, e := range pkg.Entries {
				if !models[e.ModelName] {
					fmt.Printf("  - %s\n", e.ModelName)
					models[e.ModelName] = true
				}
			}
			os.Exit(1)
		}
		log.Info("匹配: %s (INF: %s)", entry.ModelName, filepath.Base(entry.InfFile))

		if err := installPrinter(cfg, entry.InfFile, entry.ModelName, p.IP, printerName(cfg, *name, entry, localIP), portNum(cfg, localIP), protocol(cfg, localIP), *setDefault); err != nil {
			log.Error("安装失败: %v", err)
			os.Exit(1)
		}
	} else {
		// 使用已解压目录
		log.Info("使用已解压目录: %s", workDir)
		entries, err := drvpack.ParseInfDirectory(workDir)
		if err != nil {
			log.Error("解析目录失败: %v", err)
			os.Exit(1)
		}
		log.Info("共 %d 个驱动条目", len(entries))

		entry := drvpack.FindModelStrict(entries, p.Model)
		if entry == nil {
			log.Error("未找到匹配 %s 的型号", p.Model)
			fmt.Println("可用型号:")
			models := map[string]bool{}
			for _, e := range entries {
				if !models[e.ModelName] {
					fmt.Printf("  - %s\n", e.ModelName)
					models[e.ModelName] = true
				}
			}
			os.Exit(1)
		}
		log.Info("匹配: %s (INF: %s)", entry.ModelName, filepath.Base(entry.InfFile))

		if err := installPrinter(cfg, entry.InfFile, entry.ModelName, p.IP, printerName(cfg, *name, entry, localIP), portNum(cfg, localIP), protocol(cfg, localIP), *setDefault); err != nil {
			log.Error("安装失败: %v", err)
			os.Exit(1)
		}
	}

	log.Info("安装成功")
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

func printerName(cfg *config.Config, cliName string, entry *drvpack.InfEntry, localIP string) string {
	if cliName != "" {
		return cliName
	}
	if n := cfg.GetPrinterName(localIP); n != "" {
		return n
	}
	return entry.ModelName
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

func derefBool(b *bool) bool {
	if b == nil {
		return false
	}
	return *b
}
