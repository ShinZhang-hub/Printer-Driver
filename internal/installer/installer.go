package installer

import (
	"fmt"

	"printer-installer/internal/log"
)

type Params struct {
	InfFile     string
	ModelName   string
	PrinterIP   string
	PortName    string
	PortNum     int
	Protocol    string
	PrinterName string
	SetDefault  bool
}

func (p *Params) fillDefaults() {
	if p.PortNum == 0 {
		p.PortNum = 9100
	}
	if p.Protocol == "" {
		p.Protocol = "raw"
	}
	if p.PortName == "" {
		p.PortName = fmt.Sprintf("IP_%s", p.PrinterIP)
	}
	if p.PrinterName == "" {
		p.PrinterName = p.ModelName
	}
}

func Install(p Params) error {
	p.fillDefaults()
	log.Info("安装打印机: %s (%s @ %s)", p.PrinterName, p.ModelName, p.PrinterIP)
	log.Info("  INF: %s", p.InfFile)
	log.Info("  端口: %s [%s:%d/%s]", p.PortName, p.PrinterIP, p.PortNum, p.Protocol)

	if err := installDriver(p); err != nil {
		return fmt.Errorf("安装驱动失败: %w", err)
	}
	if err := addPort(p); err != nil {
		return fmt.Errorf("添加端口失败: %w", err)
	}
	if err := addPrinter(p); err != nil {
		return fmt.Errorf("添加打印机失败: %w", err)
	}
	if p.SetDefault {
		if err := setDefault(p); err != nil {
			return fmt.Errorf("设为默认打印机失败: %w", err)
		}
	}
	closeProgressWindow()
	log.Info("安装完成")
	return nil
}
