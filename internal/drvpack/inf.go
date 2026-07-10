package drvpack

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type infSection byte

const (
	secUnknown infSection = iota
	secManufacturer
	secStrings
	secModel
)

var modelLineRe = regexp.MustCompile(`^\s*"([^"]+)"\s*=\s*(\S+)\s*,\s*(\S+)`)

type infParser struct {
	path     string
	lines    []string
	sec      infSection
	secName  string
	models   []InfEntry
	strings  map[string]string
}

func ParseInfDirectory(dir string) ([]InfEntry, error) {
	var all []InfEntry
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.EqualFold(filepath.Ext(path), ".inf") {
			return nil
		}
		entries, err := parseInf(path)
		if err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		all = append(all, entries...)
		return nil
	})
	return all, err
}

func parseInf(path string) ([]InfEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	p := &infParser{
		path:    path,
		strings: map[string]string{},
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		p.lines = append(p.lines, line)
	}

	if err := p.parse(); err != nil {
		return nil, err
	}

	// resolve string tokens in model names
	for i := range p.models {
		p.models[i].ModelName = p.resolve(p.models[i].ModelName)
	}
	return p.models, nil
}

func (p *infParser) parse() error {
	for _, line := range p.lines {
		line = strings.TrimSpace(line)

		// skip comments
		if line == "" || strings.HasPrefix(line, ";") {
			continue
		}

		// section header
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			p.secName = strings.TrimSpace(line[1 : len(line)-1])
			p.sec = classifySection(p.secName)
			continue
		}

		switch p.sec {
		case secManufacturer:
			// %FF% = IDPFS, NTamd64, NTamd64.6.0
			// we extract the model section names
			if idx := strings.IndexByte(line, '='); idx >= 0 {
				rest := strings.TrimSpace(line[idx+1:])
				parts := strings.Split(rest, ",")
				for i := 0; i < len(parts); i++ {
					part := strings.TrimSpace(parts[i])
					if part != "" && !strings.HasPrefix(part, "NT") {
						// this could be a model section name like "IDPFS"
						_ = part // we'll match on model sections directly
					}
				}
			}

		case secModel:
			// "FF Apeos C2571" = FF_A_PLW, USBPRINT\...
			matches := modelLineRe.FindStringSubmatch(line)
			if matches != nil {
				p.models = append(p.models, InfEntry{
					InfFile:        p.path,
					ModelName:      matches[1],
					InstallSection: matches[2],
					HardwareID:     matches[3],
				})
			}

		case secStrings:
			if idx := strings.IndexByte(line, '='); idx >= 0 {
				key := strings.TrimSpace(line[:idx])
				val := strings.TrimSpace(line[idx+1:])
				val = strings.Trim(val, "\"")
				key = strings.Trim(key, "%")
				p.strings[key] = val
			}
		}
	}
	return nil
}

func classifySection(name string) infSection {
	name = strings.ToLower(name)
	switch {
	case name == "manufacturer":
		return secManufacturer
	case name == "strings":
		return secStrings
	case strings.Contains(name, ".ntamd64"):
		return secModel
	default:
		return secUnknown
	}
}

func (p *infParser) resolve(s string) string {
	replacer := func(match string) string {
		key := strings.Trim(match, "%")
		if v, ok := p.strings[key]; ok {
			return v
		}
		return match
	}
	// replace %STR% patterns
	return regexp.MustCompile(`%[^%]+%`).ReplaceAllStringFunc(s, replacer)
}
