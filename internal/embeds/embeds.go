package embeds

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

//go:embed drivers
var Drivers embed.FS

func ExtractDrivers(dst string) error {
	return fs.WalkDir(Drivers, "drivers", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel("drivers", path)
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		data, err := Drivers.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0644)
	})
}

// ExtractWindowsInstaller copies the embedded full driver installer (the same
// InnoSetup package used by the drivers/<brand> folder) into dst and returns
// its path. Used when no local drivers folder is present, so fresh machines get
// the same complete driver package instead of a partial file set.
func ExtractWindowsInstaller(dst string) (string, error) {
	var out string
	err := fs.WalkDir(Drivers, "drivers", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if out == "" && !d.IsDir() && strings.EqualFold(filepath.Ext(d.Name()), ".exe") {
			data, err := Drivers.ReadFile(path)
			if err != nil {
				return err
			}
			p := filepath.Join(dst, d.Name())
			if err := os.WriteFile(p, data, 0644); err != nil {
				return err
			}
			out = p
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if out == "" {
		return "", os.ErrNotExist
	}
	return out, nil
}

func ExtractMacPPD(dst string) error {
	ppdPath := "drivers/ff-mac-driver.ppd"
	data, err := Drivers.ReadFile(ppdPath)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dst, filepath.Base(ppdPath)), data, 0644)
}
