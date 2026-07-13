package config

import (
	"net"
	"time"
)

type Config struct {
	Version            int              `json:"version"`
	UpdatedAt          string           `json:"updated_at"`
	AdminPasswordHash  string           `json:"admin_password_hash,omitempty"`
	ConfigURL          string           `json:"config_url,omitempty"`
	Subnet             string           `json:"subnet,omitempty"`
	PrinterIPs         []string         `json:"printer_ips,omitempty"`
	DriversDir         string           `json:"drivers_dir,omitempty"`
	PortNumber         int              `json:"port_number,omitempty"`
	Protocol           string           `json:"protocol,omitempty"`
	SetDefault         *bool            `json:"set_default,omitempty"`
	Locations          []LocationConfig `json:"locations,omitempty"`
	Drivers            []DriverConfig   `json:"drivers"`
}

type PrinterInfo struct {
	IP    string `json:"ip"`
	Name  string `json:"name,omitempty"`
	Model string `json:"model,omitempty"`
}

type LocationConfig struct {
	Name         string        `json:"name"`
	Subnets      []string      `json:"subnets"`
	PrinterIP    string        `json:"printer_ip,omitempty"`
	PrinterName  string        `json:"printer_name,omitempty"`
	PrinterModel string        `json:"printer_model,omitempty"`
	Printers     []PrinterInfo `json:"printers,omitempty"`
	PortNumber   int           `json:"port_number,omitempty"`
	Protocol     string        `json:"protocol,omitempty"`
}

func (l *LocationConfig) AllPrinters() []PrinterInfo {
	if len(l.Printers) > 0 {
		return l.Printers
	}
	if l.PrinterIP != "" {
		return []PrinterInfo{{IP: l.PrinterIP, Name: l.PrinterName, Model: l.PrinterModel}}
	}
	return nil
}

type DriverConfig struct {
	Brand       string   `json:"brand"`
	Model       string   `json:"model"`
	ID          string   `json:"id"`
	PkgURL      string   `json:"pkg_url,omitempty"`
	PkgURLWin   string   `json:"pkg_url_win,omitempty"`
	PkgURLMac   string   `json:"pkg_url_mac,omitempty"`
	Checksum    string   `json:"checksum,omitempty"`
	InstallArgs []string `json:"install_args,omitempty"`
	Version     string   `json:"version,omitempty"`
	Enabled     bool     `json:"enabled"`
}

func (c *Config) NextVersion() int {
	return c.Version + 1
}

func (c *Config) Touch() {
	c.Version = c.NextVersion()
	c.UpdatedAt = time.Now().Format(time.RFC3339)
}

func (c *Config) MatchLocation(localIP string) *LocationConfig {
	if localIP == "" {
		return nil
	}
	ip := net.ParseIP(localIP)
	if ip == nil {
		return nil
	}
	for i := range c.Locations {
		loc := &c.Locations[i]
		for _, subnet := range loc.Subnets {
			_, cidr, err := net.ParseCIDR(subnet)
			if err != nil {
				continue
			}
			if cidr.Contains(ip) {
				return loc
			}
		}
	}
	return nil
}

func (c *Config) GetPrinterIP(localIP string) string {
	if loc := c.MatchLocation(localIP); loc != nil {
		all := loc.AllPrinters()
		if len(all) > 0 {
			return all[0].IP
		}
	}
	if len(c.PrinterIPs) > 0 {
		return c.PrinterIPs[0]
	}
	return ""
}

func (c *Config) GetPrinterName(localIP string) string {
	if loc := c.MatchLocation(localIP); loc != nil {
		all := loc.AllPrinters()
		if len(all) > 0 && all[0].Name != "" {
			return all[0].Name
		}
	}
	return ""
}

func (c *Config) GetPrinterModel(localIP string) string {
	if loc := c.MatchLocation(localIP); loc != nil {
		all := loc.AllPrinters()
		if len(all) > 0 && all[0].Model != "" {
			return all[0].Model
		}
	}
	return ""
}

func (c *Config) LookupModelByIP(ip string) string {
	for _, loc := range c.Locations {
		for _, p := range loc.AllPrinters() {
			if p.IP == ip && p.Model != "" {
				return p.Model
			}
		}
	}
	return ""
}

func (c *Config) LookupLocationByPrinterName(name string) *LocationConfig {
	for i := range c.Locations {
		for _, p := range c.Locations[i].AllPrinters() {
			if p.Name == name {
				return &c.Locations[i]
			}
		}
	}
	return nil
}
