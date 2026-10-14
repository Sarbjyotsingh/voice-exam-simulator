package services

import (
	"testing"
)

func TestIsHallucination(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"", true},
		{"   ", true},
		{"Vielen Dank.", true},
		{"vielen dank!", true},
		{"Untertitelung im Auftrag des ZDF", true},
		{"Danke fürs Zuschauen.", true},
		{"Veilen danke", true},
		{"Hallo, wie geht es dir?", false},
		{"Ich möchte Deutsch lernen.", false},
		{"Vielen Dank für Ihre Hilfe.", false}, // Not in the exact list
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := IsHallucination(tt.input); got != tt.want {
				t.Errorf("IsHallucination(%q) = %v; want %v", tt.input, got, tt.want)
			}
		})
	}
}
