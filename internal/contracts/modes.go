package contracts

import "strings"

// ModeID identifies the pluggable voice simulator mode.
type ModeID string

const (
	ModeExam         ModeID = "exam"
	ModeConversation ModeID = "conversation"
)

// NormalizeMode returns a canonical ModeID, defaulting to ModeExam.
func NormalizeMode(raw string) ModeID {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(ModeConversation):
		return ModeConversation
	default:
		return ModeExam
	}
}

// LanguageCode represents an ISO 639-1 language code.
type LanguageCode string

const (
	LangDE LanguageCode = "de"
	LangEN LanguageCode = "en"
	LangES LanguageCode = "es"
)

// NormalizeLanguage returns a canonical LanguageCode, defaulting to LangDE.
func NormalizeLanguage(raw string) LanguageCode {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(LangEN):
		return LangEN
	case string(LangES):
		return LangES
	default:
		return LangDE
	}
}

// ExamProvider identifies the examination board.
type ExamProvider string

const (
	ExamTelc   ExamProvider = "telc"
	ExamGoethe ExamProvider = "goethe"
	ExamDTZ    ExamProvider = "dtz"
)

// NormalizeExamProvider returns a canonical ExamProvider, defaulting to ExamTelc.
func NormalizeExamProvider(raw string) ExamProvider {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(ExamGoethe):
		return ExamGoethe
	case string(ExamDTZ):
		return ExamDTZ
	default:
		return ExamTelc
	}
}

// CEFRLevel represents a Common European Framework of Reference level.
type CEFRLevel string

const (
	LevelA1 CEFRLevel = "A1"
	LevelA2 CEFRLevel = "A2"
	LevelB1 CEFRLevel = "B1"
	LevelB2 CEFRLevel = "B2"
	LevelC1 CEFRLevel = "C1"
)

// NormalizeCEFRLevel returns a canonical uppercase CEFRLevel, defaulting to LevelB1.
func NormalizeCEFRLevel(raw string) CEFRLevel {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case string(LevelA1):
		return LevelA1
	case string(LevelA2):
		return LevelA2
	case string(LevelB1):
		return LevelB1
	case string(LevelB2):
		return LevelB2
	case string(LevelC1):
		return LevelC1
	default:
		return LevelB1
	}
}

// ProviderID identifies an AI upstream provider or model entry.
type ProviderID string

const (
	ProviderGroqSTT          ProviderID = "groq_stt"
	ProviderOpenAISTT        ProviderID = "openai_stt"
	ProviderOpenAIDecisions  ProviderID = "openai_decisions"
	ProviderSchemaDecisions  ProviderID = "schema_decisions"
	ProviderGroqLLM          ProviderID = "groq_llm"
	ProviderOpenAILLM        ProviderID = "openai_llm"
	ProviderElevenLabsTTS    ProviderID = "elevenlabs_tts"
	ProviderOpenAITTS        ProviderID = "openai_tts"
)
