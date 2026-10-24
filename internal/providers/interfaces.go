package providers

import (
	"context"

	"voice-exam-simulator/internal/contracts"
)

// STTRequest defines a Speech-to-Text transcription request.
type STTRequest struct {
	Audio    []byte
	Language contracts.LanguageCode
	Prompt   string
}

// STTResult contains the transcribed text and audio duration.
type STTResult struct {
	Text            string
	DurationSeconds float64
}

// STTProvider abstracts Speech-To-Text engines (Groq Whisper, OpenAI, Deepgram, AssemblyAI).
type STTProvider interface {
	Name() contracts.ProviderID
	Transcribe(ctx context.Context, req STTRequest) (*STTResult, contracts.CallUsage, error)
}

// DecisionRequest defines a structured turn classification request.
type DecisionRequest struct {
	Input     string
	Questions []contracts.DecisionQuestion
}

// DecisionResult contains the classified answers and O(1) lookup helper.
type DecisionResult struct {
	ID      string                     `json:"id"`
	Model   string                     `json:"model"`
	Answers []contracts.DecisionAnswer `json:"answers"`
}

// AnswerMap converts Answers into a map keyed by question name.
func (r *DecisionResult) AnswerMap() map[string]contracts.DecisionAnswer {
	if r == nil {
		return map[string]contracts.DecisionAnswer{}
	}
	m := make(map[string]contracts.DecisionAnswer, len(r.Answers))
	for _, a := range r.Answers {
		m[a.Name] = a
	}
	return m
}

// DecisionProvider abstracts structured turn classification (OpenAI /v1/decisions or JSON Schema LLM Adapter).
type DecisionProvider interface {
	Name() contracts.ProviderID
	Decide(ctx context.Context, req DecisionRequest) (*DecisionResult, contracts.CallUsage, error)
}

// LLMRequest defines a generative chat or evaluation completion request.
type LLMRequest struct {
	Messages    []contracts.ChatMessage
	Temperature *float64
	JSONSchema  map[string]interface{}
}

// LLMProvider abstracts Chat and Evaluation LLMs (Groq, OpenAI, Anthropic, Gemini).
type LLMProvider interface {
	Name() contracts.ProviderID
	CompleteJSON(ctx context.Context, req LLMRequest) (string, contracts.CallUsage, error)
}

// TTSRequest defines a Text-To-Speech synthesis request.
type TTSRequest struct {
	Text    string
	VoiceID string
}

// TTSProvider abstracts Text-To-Speech engines (ElevenLabs, OpenAI TTS, Cartesia, Azure).
type TTSProvider interface {
	Name() contracts.ProviderID
	Synthesize(ctx context.Context, req TTSRequest) ([]byte, contracts.CallUsage, error)
}
