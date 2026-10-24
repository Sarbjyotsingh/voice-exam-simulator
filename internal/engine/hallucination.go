package engine

import (
	"voice-exam-simulator/internal/contracts"
	"voice-exam-simulator/services"
)

// IsHallucination filters out Whisper silence/subtitle hallucinations for the given language.
func IsHallucination(lang contracts.LanguageCode, text string) bool {
	_ = lang
	return services.IsHallucination(text)
}
