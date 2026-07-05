package config

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
)

func Load(configURL string) *Config {
	cfg := Defaults()

	readLocal(cfg)

	if remote, err := fetchRemote(configURL); err == nil {
		*cfg = *remote
		saveLocal(cfg)
	}

	return cfg
}

func LoadFile(path string) *Config {
	cfg := Defaults()
	f, err := os.Open(path)
	if err != nil {
			return cfg
	}
	defer f.Close()
	if err := json.NewDecoder(f).Decode(cfg); err != nil {
		return Defaults()
	}
	return cfg
}

func Defaults() *Config {
	return &Config{
		Version:    1,
		UpdatedAt:  "2025-01-01T00:00:00Z",
		Subnet:     "192.168.1.0/24",
		DriversDir: "drivers",
		PortNumber: 9100,
		Protocol:   "raw",
		Drivers: []DriverConfig{
			{
				Brand:     "fujifilm",
				Model:     "ApeosPort C3070",
				ID:        "fujifilm-apeosport-c3070",
				PkgURLWin: "",
				InstallArgs: []string{"/S"},
				Version:   "1.0.0",
				Enabled:   true,
			},
		},
	}
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

func readLocal(cfg *Config) error {
	f, err := os.Open(localPath())
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewDecoder(f).Decode(cfg)
}

func saveLocal(cfg *Config) error {
	p := localPath()
	os.MkdirAll(filepath.Dir(p), 0755)
	f, err := os.Create(p)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(cfg)
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
	resp, err := http.Get(url + "/api/v1/config")
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
