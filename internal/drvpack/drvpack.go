package drvpack

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type DriverPackage struct {
	Brand   string
	ExePath string
	WorkDir string
	Entries []InfEntry
}

type InfEntry struct {
	InfFile        string
	ModelName      string
	InstallSection string
	HardwareID     string
}

func Open(exePath string) (*DriverPackage, error) {
	workDir, err := extract(exePath)
	if err != nil {
		return nil, fmt.Errorf("failed to extract driver package: %w", err)
	}

	entries, err := ParseInfDirectory(workDir)
	if err != nil {
		return nil, fmt.Errorf("failed to parse INF: %w", err)
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("no printer drivers found in %s", workDir)
	}

	return &DriverPackage{
		ExePath: exePath,
		WorkDir: workDir,
		Entries: entries,
	}, nil
}

func (p *DriverPackage) Cleanup() {
	if p.WorkDir != "" {
		os.RemoveAll(p.WorkDir)
	}
}

func (p *DriverPackage) FindModel(modelName string) *InfEntry {
	var best *InfEntry
	var bestScore int

	for _, e := range p.Entries {
		if score := matchScore(e.ModelName, modelName); score > bestScore {
			bestScore = score
			e := e
			best = &e
		}
	}
	return best
}

func (p *DriverPackage) FirstEntry() *InfEntry {
	if len(p.Entries) == 0 {
		return nil
	}
	return &p.Entries[0]
}

func (p *DriverPackage) FindModelStrict(modelName string) *InfEntry {
	clean := normalizeModel(modelName)
	for _, e := range p.Entries {
		if normalizeModel(e.ModelName) == clean {
			e := e
			return &e
		}
	}
	return p.FindModel(modelName)
}

func DriverDir(baseDir, brand string) string {
	return filepath.Join(baseDir, brand)
}

func FindExe(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := filepath.Ext(e.Name())
		if ext == ".exe" {
			return filepath.Join(dir, e.Name()), nil
		}
	}
	return "", fmt.Errorf("no .exe file found in %s", dir)
}

func FindDmg(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.EqualFold(filepath.Ext(e.Name()), ".dmg") {
			return filepath.Join(dir, e.Name()), nil
		}
	}
	return "", fmt.Errorf("no .dmg file found in %s", dir)
}

func OpenDMG(dmgPath string) (*DriverPackage, error) {
	workDir, err := extract(dmgPath)
	if err != nil {
		return nil, fmt.Errorf("failed to extract DMG driver package: %w", err)
	}

	entries, err := ParsePPDDirectory(workDir)
	if err != nil {
		return nil, fmt.Errorf("failed to parse PPD: %w", err)
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("no printer drivers found in %s", workDir)
	}

	return &DriverPackage{
		ExePath: dmgPath,
		WorkDir: workDir,
		Entries: entries,
	}, nil
}
