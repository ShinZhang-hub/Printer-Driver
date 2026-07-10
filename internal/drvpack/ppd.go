package drvpack

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var ppdModelRe = regexp.MustCompile(`^\*ModelName:\s*"([^"]+)"`)
var ppdNickRe = regexp.MustCompile(`^\*NickName:\s*"([^"]+)"`)

func ParsePPDDirectory(dir string) ([]InfEntry, error) {
	var all []InfEntry
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.EqualFold(filepath.Ext(path), ".ppd") {
			return nil
		}
		entries, err := parsePPD(path)
		if err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		all = append(all, entries...)
		return nil
	})
	return all, err
}

func parsePPD(path string) ([]InfEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var entries []InfEntry
	seen := map[string]bool{}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "*%") {
			continue
		}

		var name string
		if m := ppdModelRe.FindStringSubmatch(line); m != nil {
			name = m[1]
		} else if m := ppdNickRe.FindStringSubmatch(line); m != nil {
			name = m[1]
		}
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		entries = append(entries, InfEntry{
			InfFile:   path,
			ModelName: name,
		})
	}

	if len(entries) == 0 {
		return nil, fmt.Errorf("%s: no ModelName/NickName found", path)
	}
	return entries, nil
}
