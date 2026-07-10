package embeds

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
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

func ExtractMacPPD(dst string) error {
	ppdPath := "drivers/ff-mac-driver.ppd"
	data, err := Drivers.ReadFile(ppdPath)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dst, filepath.Base(ppdPath)), data, 0644)
}
