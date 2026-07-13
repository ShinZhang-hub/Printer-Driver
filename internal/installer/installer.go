package installer

import (
	"fmt"
	"time"

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
	log.Info("Installing printer: %s (%s @ %s)", p.PrinterName, p.ModelName, p.PrinterIP)
	log.Info("  Driver: %s", p.InfFile)
	log.Info("  Port: %s [%s:%d/%s]", p.PortName, p.PrinterIP, p.PortNum, p.Protocol)

	if err := installDriver(p); err != nil {
		return fmt.Errorf("install driver failed: %w", err)
	}
	if err := addPrinter(p); err != nil {
		return fmt.Errorf("add printer failed: %w", err)
	}
	if p.SetDefault {
		time.Sleep(500 * time.Millisecond)
		if err := setDefault(p); err != nil {
			return fmt.Errorf("set default failed: %w", err)
		}
	}
	closeProgressWindow()
	log.Info("Installation complete")
	return nil
}

var ResultMessage string
