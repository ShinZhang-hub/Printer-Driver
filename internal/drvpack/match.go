package drvpack

import (
	"regexp"
	"strings"
	"unicode"
)

var brandPrefixRe = regexp.MustCompile(`^(?i)(ff\s+|fujifilm\s+|hp\s+|canon\s+|ricoh\s+|kyocera\s+|brother\s+|epson\s+)`)

func normalizeModel(s string) string {
	s = strings.TrimSpace(s)
	s = brandPrefixRe.ReplaceAllString(s, "")
	s = strings.Join(strings.Fields(s), " ")
	return strings.ToLower(s)
}

func matchScore(infModel, snmpModel string) int {
	a := normalizeModel(infModel)
	b := normalizeModel(snmpModel)

	if a == "" || b == "" {
		return 0
	}
	if a == b {
		return 100
	}

	// extract core identifiers like "C2571"
	aNums := extractNumbers(a)
	bNums := extractNumbers(b)

	if aNums != "" && aNums == bNums {
		// same model number, check prefix overlap
		if containsWords(a, b) || containsWords(b, a) {
			return 80
		}
		return 60
	}

	if strings.Contains(a, b) || strings.Contains(b, a) {
		return 50
	}

	return 0
}

func extractNumbers(s string) string {
	var nums []rune
	for _, r := range s {
		if unicode.IsDigit(r) || r == ' ' {
			nums = append(nums, r)
		}
	}
	return strings.Join(strings.Fields(string(nums)), "")
}

func FindModelStrict(entries []InfEntry, modelName string) *InfEntry {
	clean := normalizeModel(modelName)
	for _, e := range entries {
		if normalizeModel(e.ModelName) == clean {
			return &e
		}
	}
	return FindModel(entries, modelName)
}

func FindModel(entries []InfEntry, modelName string) *InfEntry {
	var best *InfEntry
	var bestScore int
	for _, e := range entries {
		if score := matchScore(e.ModelName, modelName); score > bestScore {
			bestScore = score
			e := e
			best = &e
		}
	}
	return best
}

func containsWords(a, b string) bool {
	words := strings.Fields(b)
	if len(words) == 0 {
		return false
	}
	matchCount := 0
	for _, w := range words {
		if strings.Contains(a, w) {
			matchCount++
		}
	}
	return matchCount >= len(words)/2
}
