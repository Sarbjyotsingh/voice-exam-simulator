package services

import (
	"context"
	"sync"
	"unicode/utf8"

	"voice-exam-simulator/config"
)

// TTSCall records one dynamic ElevenLabs synthesis (pre-recorded clips never appear here).
type TTSCall struct {
	VoiceID    string `json:"voice_id"`
	Characters int    `json:"characters"`
}

// Usage collects raw upstream usage (STT seconds, LLM tokens, TTS characters, Decisions tokens)
// for one unit of work (a single turn or a single evaluation). It is safe for concurrent use and all
// methods are nil-safe so services work unchanged when no collector is attached.
type Usage struct {
	mu               sync.Mutex
	STTSeconds       float64
	PromptTokens     int
	CompletionTokens int
	DecisionTokens   int
	TTS              []TTSCall
}

type usageKey struct{}

// WithUsage attaches a fresh Usage collector to ctx.
func WithUsage(ctx context.Context) (context.Context, *Usage) {
	u := &Usage{}
	return context.WithValue(ctx, usageKey{}, u), u
}

// UsageFrom returns the collector attached to ctx, or nil.
func UsageFrom(ctx context.Context) *Usage {
	u, _ := ctx.Value(usageKey{}).(*Usage)
	return u
}

func (u *Usage) AddSTT(seconds float64) {
	if u == nil {
		return
	}
	u.mu.Lock()
	u.STTSeconds += seconds
	u.mu.Unlock()
}

func (u *Usage) AddLLM(prompt, completion int) {
	if u == nil {
		return
	}
	u.mu.Lock()
	u.PromptTokens += prompt
	u.CompletionTokens += completion
	u.mu.Unlock()
}

func (u *Usage) AddDecisions(tokens int) {
	if u == nil {
		return
	}
	u.mu.Lock()
	u.DecisionTokens += tokens
	u.mu.Unlock()
}

func (u *Usage) AddTTS(voiceID, text string) {
	if u == nil {
		return
	}
	u.mu.Lock()
	u.TTS = append(u.TTS, TTSCall{VoiceID: voiceID, Characters: utf8.RuneCountInString(text)})
	u.mu.Unlock()
}

// UsageSnapshot is an immutable copy of collected usage.
type UsageSnapshot struct {
	STTSeconds       float64   `json:"stt_audio_seconds"`
	PromptTokens     int       `json:"prompt_tokens"`
	CompletionTokens int       `json:"completion_tokens"`
	DecisionTokens   int       `json:"decision_tokens"`
	TTSCharacters    int       `json:"tts_characters"`
	TTSCalls         []TTSCall `json:"tts_calls,omitempty"`
}

func (u *Usage) Snapshot() UsageSnapshot {
	if u == nil {
		return UsageSnapshot{}
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	s := UsageSnapshot{
		STTSeconds:       u.STTSeconds,
		PromptTokens:     u.PromptTokens,
		CompletionTokens: u.CompletionTokens,
		DecisionTokens:   u.DecisionTokens,
		TTSCalls:         append([]TTSCall(nil), u.TTS...),
	}
	for _, c := range u.TTS {
		s.TTSCharacters += c.Characters
	}
	return s
}

// Pricing turns usage into USD using configurable unit prices.
type Pricing struct {
	WhisperPerHour   float64
	LLMInputPer1M    float64
	LLMOutputPer1M   float64
	DecisionsPer1M   float64
	TTSPer1KChars    float64
	VoiceMultipliers map[string]float64
}

func NewPricing(cfg config.Config) Pricing {
	return Pricing{
		WhisperPerHour:   cfg.WhisperPricePerHourUSD,
		LLMInputPer1M:    cfg.GroqInputPer1MUSD,
		LLMOutputPer1M:   cfg.GroqOutputPer1MUSD,
		DecisionsPer1M:   cfg.DecisionsInputPer1MUSD,
		TTSPer1KChars:    cfg.ElevenLabsPer1KCharsUSD,
		VoiceMultipliers: cfg.VoiceCostMultipliers,
	}
}

// minBilledSTTSeconds is Groq's minimum billed length per transcription request.
const minBilledSTTSeconds = 10.0

func (p Pricing) STT(seconds float64) float64 {
	if seconds <= 0 {
		return 0
	}
	return seconds / 3600.0 * p.WhisperPerHour
}

// sttBilledSeconds applies the per-request minimum; usage collected per turn is already
// one request, so the turn-level snapshot is billed with the minimum applied once.
func sttBilledSeconds(seconds float64) float64 {
	if seconds > 0 && seconds < minBilledSTTSeconds {
		return minBilledSTTSeconds
	}
	return seconds
}

func (p Pricing) LLM(prompt, completion int) float64 {
	return float64(prompt)/1e6*p.LLMInputPer1M + float64(completion)/1e6*p.LLMOutputPer1M
}

func (p Pricing) Decisions(tokens int) float64 {
	if tokens <= 0 {
		return 0
	}
	return float64(tokens) / 1e6 * p.DecisionsPer1M
}

func (p Pricing) TTS(calls []TTSCall) float64 {
	var total float64
	for _, c := range calls {
		mult := 1.0
		if m, ok := p.VoiceMultipliers[c.VoiceID]; ok && m > 0 {
			mult = m
		}
		total += float64(c.Characters) / 1000.0 * p.TTSPer1KChars * mult
	}
	return total
}

// Cost is a USD breakdown. Only the components relevant to a unit of work are non-zero.
type Cost struct {
	STT       float64 `json:"stt_cost"`
	ChatLLM   float64 `json:"chat_llm_cost"`
	Decisions float64 `json:"decisions_cost,omitempty"`
	EvalLLM   float64 `json:"eval_llm_cost"`
	TTS       float64 `json:"tts_cost"`
	Total     float64 `json:"total_cost"`
}

// TurnCost prices a conversational turn (STT + chat LLM + Decisions + dynamic TTS).
func (p Pricing) TurnCost(s UsageSnapshot) Cost {
	c := Cost{
		STT:       p.STT(sttBilledSeconds(s.STTSeconds)),
		ChatLLM:   p.LLM(s.PromptTokens, s.CompletionTokens),
		Decisions: p.Decisions(s.DecisionTokens),
		TTS:       p.TTS(s.TTSCalls),
	}
	c.Total = c.STT + c.ChatLLM + c.Decisions + c.TTS
	return c
}

// EvalCost prices an evaluation call (LLM only).
func (p Pricing) EvalCost(s UsageSnapshot) Cost {
	c := Cost{EvalLLM: p.LLM(s.PromptTokens, s.CompletionTokens)}
	c.Total = c.EvalLLM
	return c
}
