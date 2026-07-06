package drvpack

import (
	"fmt"
	"os"
	"path/filepath"
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
		return nil, fmt.Errorf("解压驱动包失败: %w", err)
	}

	entries, err := ParseInfDirectory(workDir)
	if err != nil {
		return nil, fmt.Errorf("解析 INF 失败: %w", err)
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("未在 %s 中找到任何打印机驱动", workDir)
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
	return "", fmt.Errorf("在 %s 中未找到 .exe 文件", dir)
}
