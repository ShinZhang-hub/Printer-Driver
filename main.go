package main

import (
	_ "embed"
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"printer-installer/internal/config"
	"printer-installer/internal/drvpack"
	"printer-installer/internal/embeds"
	"printer-installer/internal/installer"
	"printer-installer/internal/log"
	"printer-installer/internal/scanner"
	"printer-installer/internal/web"
)

//go:embed config.json
var embeddedConfig []byte

func main() {
	ip := flag.String("ip", "", "打印机 IP 地址")
	driversDir := flag.String("drivers", "drivers", "驱动包目录")
	extracted := flag.String("extracted", "", "已解压的驱动目录，跳过解压")
	name := flag.String("name", "", "打印机名称")
	setDefault := flag.Bool("default", true, "设为默认打印机")
	admin := flag.Bool("admin", false, "打开管理面板")
	flag.Parse()

	log.Init()
	defer log.Close()

	cfg := config.LoadRemote(embeddedConfig)

	if *admin || isShiftPressed() {
		web.StartAdminPanel(cfg, func(installIP, installName string) error {
			return runInstall(cfg, *driversDir, installIP, installName, *setDefault)
		})
		return
	}
	localIP := localIPAddr()

	if *ip == "" {
		*ip = cfg.GetPrinterIP(localIP)
	}
	if *ip == "" {
		log.Error("未指定打印机 IP，请在 config.json 中配置或使用 --ip 参数")
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
		log.Info("发现: %s %s (%s)", p.Brand, p.Model, p.IP)
	} else {
		log.Warn("探测打印机失败: %v，尝试 config 中的型号", err)
		model = cfg.GetPrinterModel(localIP)
		if model == "" {
			log.Error("SNMP 探测失败且 config 中未配置 printer_model，无法确定型号")
			os.Exit(1)
		}
		log.Info("使用 config 型号: %s", model)
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
		log.Error("未找到匹配 %s 的型号", model)
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
	if err := installPrinter(cfg, entry.InfFile, entry.ModelName, p.IP, *name, portNum(cfg, localIP), protocol(cfg, localIP), *setDefault); err != nil {
		log.Error("安装失败: %v", err)
		os.Exit(1)
	}
	log.Info("安装成功")
}

func isShiftPressed() bool {
	mod := syscall.NewLazyDLL("user32.dll")
	proc := mod.NewProc("GetAsyncKeyState")
	// 轮询多次，应对启动延迟
	for i := 0; i < 5; i++ {
		for _, vk := range []uintptr{0x10, 0xA0, 0xA1} { // VK_SHIFT, VK_LSHIFT, VK_RSHIFT
			ret, _, _ := proc.Call(vk)
			if int16(ret) < 0 {
				return true
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}

func resolveDriver(driversDir, brand, extracted string) (*drvpack.DriverPackage, error) {
	if extracted != "" {
		entries, err := drvpack.ParseInfDirectory(extracted)
		if err != nil {
			return nil, fmt.Errorf("解析已解压目录失败: %w", err)
		}
		return &drvpack.DriverPackage{WorkDir: extracted, Entries: entries}, nil
	}
	brandDir := filepath.Join(driversDir, brand)
	exePath, err := drvpack.FindExe(brandDir)
	if err == nil {
		pkg, pkgErr := drvpack.Open(exePath)
		if pkgErr == nil {
			return pkg, nil
		}
		log.Warn("解压驱动包失败: %v，尝试嵌入驱动", pkgErr)
	}
	tmpDir, err := os.MkdirTemp("", "printer-installer-embedded-")
	if err != nil {
		return nil, fmt.Errorf("创建临时目录失败: %w", err)
	}
	if err := embeds.ExtractDrivers(tmpDir); err != nil {
		os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("解压嵌入驱动失败: %w", err)
	}
	log.Info("使用嵌入驱动")
	entries, err := drvpack.ParseInfDirectory(tmpDir)
	if err != nil {
		os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("解析嵌入驱动失败: %w", err)
	}
	return &drvpack.DriverPackage{WorkDir: tmpDir, Entries: entries}, nil
}

func runInstall(cfg *config.Config, driversDir, printerIP, printerName string, setDefault bool) error {
	p, probeErr := scanner.ProbeSingleIP(printerIP)
	brand := ""
	model := cfg.LookupModelByIP(printerIP)
	if probeErr == nil {
		brand = p.Brand
		model = p.Model
	}
	if model == "" {
		return fmt.Errorf("无法获取打印机型号（SNMP 失败且 config 未配置 %s 的 printer_model）", printerIP)
	}
	pkg, err := resolveDriver(driversDir, brand, "")
	if err != nil {
		return err
	}
	defer pkg.Cleanup()

	entry := pkg.FindModelStrict(model)
	if entry == nil {
		return fmt.Errorf("驱动包中未找到任何可用型号")
	}
	if printerName == "" {
		printerName = entry.ModelName
	}
	return installPrinter(cfg, entry.InfFile, entry.ModelName, printerIP, printerName, cfg.PortNumber, cfg.Protocol, setDefault)
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
