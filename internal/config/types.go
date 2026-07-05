package config

import "time"

type Config struct {
	Version    int            `json:"version"`
	UpdatedAt  string         `json:"updated_at"`
	Subnet     string         `json:"subnet,omitempty"`
	PrinterIPs []string       `json:"printer_ips,omitempty"`
	Drivers    []DriverConfig `json:"drivers"`
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
