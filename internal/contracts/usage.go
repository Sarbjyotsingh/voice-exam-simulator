package contracts

import "math"

// TTSCall records one dynamic TTS synthesis call.
type TTSCall struct {
	VoiceID    string `json:"voice_id"`
	Characters int    `json:"characters"`
	ModelID    string `json:"model_id,omitempty"`
}

// CallUsage represents raw usage from a single provider call.
type CallUsage struct {
	Provider         ProviderID `json:"provider,omitempty"`
	ModelKey         string     `json:"model_key,omitempty"`
	STTSeconds       float64    `json:"stt_audio_seconds,omitempty"`
	PromptTokens     int        `json:"prompt_tokens,omitempty"`
	CachedTokens     int        `json:"cached_tokens,omitempty"`
	CompletionTokens int        `json:"completion_tokens,omitempty"`
	DecisionTokens   int        `json:"decision_tokens,omitempty"`
	TTSCharacters    int        `json:"tts_characters,omitempty"`
	VoiceID          string     `json:"voice_id,omitempty"`
}

// UsageSnapshot is an immutable snapshot of usage across a turn or evaluation.
type UsageSnapshot struct {
	STTSeconds       float64   `json:"stt_audio_seconds"`
	PromptTokens     int       `json:"prompt_tokens"`
	CachedTokens     int       `json:"cached_tokens,omitempty"`
	CompletionTokens int       `json:"completion_tokens"`
	DecisionTokens   int       `json:"decision_tokens"`
	TTSCharacters    int       `json:"tts_characters"`
	TTSCalls         []TTSCall `json:"tts_calls,omitempty"`
}

// ModelPricingConfig defines unit prices in USD for a model in catalog/providers.yaml.
type ModelPricingConfig struct {
	PerAudioSecondUSD   float64 `json:"per_audio_second_usd" yaml:"per_audio_second_usd"`
	InputPer1MUSD       float64 `json:"input_per_1m_usd" yaml:"input_per_1m_usd"`
	CachedInputPer1MUSD float64 `json:"cached_input_per_1m_usd" yaml:"cached_input_per_1m_usd"`
	OutputPer1MUSD      float64 `json:"output_per_1m_usd" yaml:"output_per_1m_usd"`
	PerCharUSD          float64 `json:"per_char_usd" yaml:"per_char_usd"`
}

// Cost is a USD breakdown for a turn, evaluation, or attempt.
type Cost struct {
	STT       float64 `json:"stt_cost"`
	ChatLLM   float64 `json:"chat_llm_cost"`
	Decisions float64 `json:"decisions_cost,omitempty"`
	EvalLLM   float64 `json:"eval_llm_cost"`
	TTS       float64 `json:"tts_cost"`
	Total     float64 `json:"total_cost"`
}

// RoundUSD8 rounds a USD amount to 8 decimal places.
func RoundUSD8(f float64) float64 {
	return math.Round(f*1e8) / 1e8
}
