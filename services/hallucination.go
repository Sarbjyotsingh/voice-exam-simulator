package services

import (
	"regexp"
	"strings"
)

var (
	nonAlphanumeric = regexp.MustCompile(`[^\w\s]`)

	knownHallucinations = []string{
		"Vielen Dank.",
		"Vielen Dank für das Zuschauen.",
		"Danke fürs Zuschauen.",
		"Untertitelung im Auftrag des ZDF",
		"Untertitelung von TV-M",
		"Untertitelung der ARD",
		"Danke.",
		"Vielen dank.",
		"Veilen danke",
		"Vielen Dank!",
		"Danke!",
	}

	hallucinationsSet = make(map[string]struct{})
)

func init() {
	for _, h := range knownHallucinations {
		norm := normalize(h)
		if norm != "" {
			hallucinationsSet[norm] = struct{}{}
		}
	}
}

func normalize(text string) string {
	text = nonAlphanumeric.ReplaceAllString(text, "")
	return strings.ToLower(strings.TrimSpace(text))
}

func IsHallucination(text string) bool {
	norm := normalize(text)
	if norm == "" {
		return true
	}
	_, exists := hallucinationsSet[norm]
	return exists
}
