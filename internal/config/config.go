package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

func LoadRemote(embedded []byte) *Config {
	cfg := ParseEmbedded(embedded)
	if cfg == nil {
		cfg = Defaults()
	}

	if cfg.ConfigURL != "" {
		if remote, err := fetchRemote(cfg.ConfigURL); err == nil {
			remote.ConfigURL = cfg.ConfigURL
			return remote
		}
	}

	return cfg
}

func ParseEmbedded(data []byte) *Config {
	if len(data) == 0 {
		return nil
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil
	}
	return &cfg
}

func Defaults() *Config {
	return &Config{
		Version:    1,
		UpdatedAt:  "2025-01-01T00:00:00Z",
		Subnet:     "30.61.40.0/24",
		DriversDir: "drivers",
		PortNumber: 9100,
		Protocol:   "raw",
		Locations: []LocationConfig{
			{
				Name:        "Default",
				Subnets:     []string{"30.61.40.0/24"},
				PrinterIP:   "30.61.40.40",
				PrinterName: "Printer-Osaka",
				PortNumber:  9100,
				Protocol:    "raw",
			},
		},
		Drivers: []DriverConfig{
			{
				Brand:       "fujifilm",
				Model:       "ApeosPort C3070",
				ID:          "fujifilm-apeosport-c3070",
				PkgURLWin:   "",
				InstallArgs: []string{"/S"},
				Version:     "1.0.0",
				Enabled:     true,
			},
		},
	}
}

func (c *Config) Save() error {
	c.Touch()
	if err := saveLocal(c); err != nil {
		return fmt.Errorf("failed to save local config: %w", err)
	}
	if c.ConfigURL != "" {
		saveRemote(c) // remote failure does not block local save
	}
	return nil
}

func (c *Config) LookupDriver(model, brand string) *DriverConfig {
	for _, d := range c.Drivers {
		if !d.Enabled {
			continue
		}
		if d.Model == model && d.Brand == brand {
			return &d
		}
	}
	for _, d := range c.Drivers {
		if !d.Enabled {
			continue
		}
		if d.Brand == brand {
			return &d
		}
	}
	return nil
}

func (c *Config) PlatformURL(d *DriverConfig) string {
	if runtime.GOOS == "windows" && d.PkgURLWin != "" {
		return d.PkgURLWin
	}
	if runtime.GOOS == "darwin" && d.PkgURLMac != "" {
		return d.PkgURLMac
	}
	return d.PkgURL
}

func saveLocal(cfg *Config) error {
	p := localPath()
	os.MkdirAll(filepath.Dir(p), 0755)
	f, err := os.Create(p)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(cfg)
}

func saveRemote(cfg *Config) error {
	body, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 2 * time.Second}
	req, err := http.NewRequest("PUT", cfg.ConfigURL+"/api/v1/config", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if token := os.Getenv("ADMIN_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("remote write failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("remote returned %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

func localPath() string {
	var dir string
	switch runtime.GOOS {
	case "windows":
		dir = os.Getenv("PROGRAMDATA")
		if dir == "" {
			dir = "C:\\ProgramData"
		}
	case "darwin":
		dir = "/Library/Application Support"
	default:
		dir = "/var/lib"
	}
	return filepath.Join(dir, "PrinterInstaller", "config.json")
}

func fetchRemote(url string) (*Config, error) {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url + "/api/v1/config")
	if err != nil {
		return nil, fmt.Errorf("fetch config: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(body, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
