package engine

import (
	"voice-engine/internal/contracts"
	"voice-engine/services"
)

// IsHallucination filters out Whisper silence/subtitle hallucinations for the given language.
func IsHallucination(lang contracts.LanguageCode, text string) bool {
	_ = lang
	return services.IsHallucination(text)
}
