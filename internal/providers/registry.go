package providers

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"voice-exam-simulator/config"
	"voice-exam-simulator/internal/contracts"
	"voice-exam-simulator/services"
)

// RouteConfig specifies primary and fallback model keys for one capability.
type RouteConfig struct {
	Primary  string `json:"primary" yaml:"primary"`
	Fallback string `json:"fallback" yaml:"fallback"`
}

// ModelSpec defines one model entry from catalog/providers.yaml.
type ModelSpec struct {
	Provider        string                       `json:"provider" yaml:"provider"`
	Model           string                       `json:"model" yaml:"model"`
	Temperature     float64                      `json:"temperature,omitempty" yaml:"temperature,omitempty"`
	Stability       float64                      `json:"stability,omitempty" yaml:"stability,omitempty"`
	SimilarityBoost float64                      `json:"similarity_boost,omitempty" yaml:"similarity_boost,omitempty"`
	Pricing         contracts.ModelPricingConfig `json:"pricing" yaml:"pricing"`
}

// CatalogConfig holds the parsed catalog/providers.yaml configuration.
type CatalogConfig struct {
	Routing struct {
		STT          RouteConfig `json:"stt" yaml:"stt"`
		Decisions    RouteConfig `json:"decisions" yaml:"decisions"`
		ChatFallback RouteConfig `json:"chat_fallback" yaml:"chat_fallback"`
		Evaluation   RouteConfig `json:"evaluation" yaml:"evaluation"`
		TTS          RouteConfig `json:"tts" yaml:"tts"`
	} `json:"routing" yaml:"routing"`
	Models map[string]ModelSpec `json:"models" yaml:"models"`
}

// PersonaVoiceSpec holds voice mapping and bio for a persona from catalog/personas/de_voices.yaml.
type PersonaVoiceSpec struct {
	Name           string            `json:"name"`
	Role           string            `json:"role"`
	Gender         string            `json:"gender"`
	Bio            string            `json:"bio"`
	Voices         map[string]string `json:"voices"`
	CostMultiplier float64           `json:"cost_multiplier"`
}

// ProviderRegistry manages STT, Decision, LLM, and TTS providers with automatic fallback retry and dynamic pricing.
type ProviderRegistry struct {
	cfg     config.Config
	catalog CatalogConfig
	voices  map[string]PersonaVoiceSpec // lowercase persona name -> PersonaVoiceSpec
	log     *slog.Logger

	sttModels       map[string]STTProvider
	decisionModels  map[string]DecisionProvider
	llmModels       map[string]LLMProvider
	ttsModels       map[string]TTSProvider
	pricingSnapshot services.Pricing
}

// DefaultCatalogConfig returns the built-in default configuration matching catalog/providers.yaml.
func DefaultCatalogConfig() CatalogConfig {
	var c CatalogConfig
	c.Routing.STT = RouteConfig{Primary: "groq_whisper_v3", Fallback: "openai_whisper_1"}
	c.Routing.Decisions = RouteConfig{Primary: "openai_luna_decisions", Fallback: "groq_gpt_oss_schema"}
	c.Routing.ChatFallback = RouteConfig{Primary: "groq_gpt_oss_120b", Fallback: "openai_gpt_4o_mini"}
	c.Routing.Evaluation = RouteConfig{Primary: "groq_gpt_oss_120b", Fallback: "openai_gpt_4o_mini"}
	c.Routing.TTS = RouteConfig{Primary: "elevenlabs_flash_v2_5", Fallback: "openai_tts_1"}

	c.Models = map[string]ModelSpec{
		"groq_whisper_v3": {
			Provider: string(contracts.ProviderGroqSTT),
			Model:    config.GroqWhisperModel,
			Pricing:  contracts.ModelPricingConfig{PerAudioSecondUSD: config.DefaultWhisperPricePerHourUSD / 3600.0},
		},
		"openai_whisper_1": {
			Provider: string(contracts.ProviderOpenAISTT),
			Model:    "gpt-4o-mini-transcribe",
			Pricing:  contracts.ModelPricingConfig{PerAudioSecondUSD: 0.18 / 3600.0},
		},
		"openai_luna_decisions": {
			Provider: string(contracts.ProviderOpenAIDecisions),
			Model:    config.OpenAIDecisionsModel,
			Pricing:  contracts.ModelPricingConfig{InputPer1MUSD: config.DefaultDecisionsInputPer1MUSD, CachedInputPer1MUSD: 0.02, OutputPer1MUSD: 0.80},
		},
		"groq_gpt_oss_schema": {
			Provider:    string(contracts.ProviderSchemaDecisions),
			Model:       "openai/gpt-oss-120b",
			Temperature: 0.1,
			Pricing:     contracts.ModelPricingConfig{InputPer1MUSD: config.DefaultGroqInputPer1MUSD, OutputPer1MUSD: config.DefaultGroqOutputPer1MUSD},
		},
		"groq_gpt_oss_120b": {
			Provider:    string(contracts.ProviderGroqLLM),
			Model:       "openai/gpt-oss-120b",
			Temperature: 0.2,
			Pricing:     contracts.ModelPricingConfig{InputPer1MUSD: config.DefaultGroqInputPer1MUSD, CachedInputPer1MUSD: 0.075, OutputPer1MUSD: config.DefaultGroqOutputPer1MUSD},
		},
		"openai_gpt_4o_mini": {
			Provider:    string(contracts.ProviderOpenAILLM),
			Model:       "gpt-4o-mini",
			Temperature: 0.2,
			Pricing:     contracts.ModelPricingConfig{InputPer1MUSD: 0.15, CachedInputPer1MUSD: 0.075, OutputPer1MUSD: 0.60},
		},
		"elevenlabs_flash_v2_5": {
			Provider:        string(contracts.ProviderElevenLabsTTS),
			Model:           config.ElevenLabsModel,
			Stability:       0.5,
			SimilarityBoost: 0.75,
			Pricing:         contracts.ModelPricingConfig{PerCharUSD: config.DefaultElevenLabsFlashPer1KCharsUSD / 1000.0},
		},
	}
	return c
}

// NewRegistry loads catalog/providers.yaml and catalog/personas/de_voices.yaml and wires all providers.
func NewRegistry(cfg config.Config, catalogDir string, client *http.Client, log *slog.Logger) *ProviderRegistry {
	if log == nil {
		log = slog.Default()
	}
	if catalogDir == "" {
		catalogDir = "catalog"
	}

	cat := DefaultCatalogConfig()
	if loaded, err := loadProvidersYAML(filepath.Join(catalogDir, "providers.yaml")); err == nil {
		mergeCatalogConfig(&cat, loaded)
	}

	voices := loadDefaultPersonas()
	if loadedVoices, err := loadPersonasYAML(filepath.Join(catalogDir, "personas", "de_voices.yaml")); err == nil {
		for k, v := range loadedVoices {
			voices[k] = v
		}
	}

	r := &ProviderRegistry{
		cfg:            cfg,
		catalog:        cat,
		voices:         voices,
		log:            log.With("component", "provider_registry"),
		sttModels:      make(map[string]STTProvider),
		decisionModels: make(map[string]DecisionProvider),
		llmModels:      make(map[string]LLMProvider),
		ttsModels:      make(map[string]TTSProvider),
	}

	r.initModels(client)
	r.pricingSnapshot = r.buildPricing()
	return r
}

func (r *ProviderRegistry) initModels(client *http.Client) {
	// First instantiate LLM models so SchemaDecisionAdapter can wrap them
	for key, spec := range r.catalog.Models {
		switch contracts.ProviderID(spec.Provider) {
		case contracts.ProviderGroqLLM:
			modelName := spec.Model
			if r.cfg.GroqModel != "" && key == "groq_gpt_oss_120b" {
				modelName = r.cfg.GroqModel
			}
			r.llmModels[key] = NewOpenAICompatLLM(
				contracts.ProviderGroqLLM,
				r.cfg.GroqAPIKey,
				config.GroqBaseURL,
				modelName,
				spec.Temperature,
				r.cfg.GroqMaxConcurrency,
				client,
			)
		case contracts.ProviderOpenAILLM:
			r.llmModels[key] = NewOpenAICompatLLM(
				contracts.ProviderOpenAILLM,
				r.cfg.OpenAIAPIKey,
				config.OpenAIBaseURL,
				spec.Model,
				spec.Temperature,
				r.cfg.GroqMaxConcurrency,
				client,
			)
		}
	}

	for key, spec := range r.catalog.Models {
		switch contracts.ProviderID(spec.Provider) {
		case contracts.ProviderGroqSTT:
			r.sttModels[key] = NewGroqSTT(r.cfg.GroqAPIKey, spec.Model, r.cfg.GroqMaxConcurrency, client)
		case contracts.ProviderOpenAISTT:
			r.sttModels[key] = NewOpenAISTT(r.cfg.OpenAIAPIKey, spec.Model, r.cfg.GroqMaxConcurrency, client)
		case contracts.ProviderOpenAIDecisions:
			r.decisionModels[key] = NewOpenAIDecisionsProvider(r.cfg.OpenAIAPIKey, spec.Model, r.cfg.DecisionsMaxConcurrency, client)
		case contracts.ProviderSchemaDecisions:
			baseLLM := r.llmModels["groq_gpt_oss_120b"]
			if baseLLM == nil {
				baseLLM = NewOpenAICompatLLM(
					contracts.ProviderGroqLLM,
					r.cfg.GroqAPIKey,
					config.GroqBaseURL,
					spec.Model,
					spec.Temperature,
					r.cfg.GroqMaxConcurrency,
					client,
				)
			}
			r.decisionModels[key] = NewSchemaDecisionAdapter(baseLLM)
		case contracts.ProviderElevenLabsTTS:
			r.ttsModels[key] = NewElevenLabsTTS(
				r.cfg.ElevenLabsAPIKey,
				spec.Model,
				spec.Stability,
				spec.SimilarityBoost,
				r.cfg.ElevenLabsMaxConcurrency,
				client,
			)
		}
	}
}

func (r *ProviderRegistry) buildPricing() services.Pricing {
	p := services.NewPricing(r.cfg)

	if sttSpec, ok := r.catalog.Models[r.catalog.Routing.STT.Primary]; ok && sttSpec.Pricing.PerAudioSecondUSD > 0 {
		if os.Getenv("COST_WHISPER_PER_HOUR_USD") == "" {
			p.WhisperPerHour = sttSpec.Pricing.PerAudioSecondUSD * 3600.0
		}
	}
	if llmSpec, ok := r.catalog.Models[r.catalog.Routing.ChatFallback.Primary]; ok {
		if llmSpec.Pricing.InputPer1MUSD > 0 && os.Getenv("COST_GROQ_INPUT_PER_1M_USD") == "" {
			p.LLMInputPer1M = llmSpec.Pricing.InputPer1MUSD
		}
		if llmSpec.Pricing.OutputPer1MUSD > 0 && os.Getenv("COST_GROQ_OUTPUT_PER_1M_USD") == "" {
			p.LLMOutputPer1M = llmSpec.Pricing.OutputPer1MUSD
		}
	}
	if decSpec, ok := r.catalog.Models[r.catalog.Routing.Decisions.Primary]; ok && decSpec.Pricing.InputPer1MUSD > 0 {
		if os.Getenv("COST_DECISIONS_INPUT_PER_1M_USD") == "" {
			p.DecisionsPer1M = decSpec.Pricing.InputPer1MUSD
		}
	}
	if ttsSpec, ok := r.catalog.Models[r.catalog.Routing.TTS.Primary]; ok && ttsSpec.Pricing.PerCharUSD > 0 {
		if os.Getenv("COST_ELEVENLABS_PER_1K_CHARS_USD") == "" {
			p.TTSPer1KChars = ttsSpec.Pricing.PerCharUSD * 1000.0
		}
	}
	if p.VoiceMultipliers == nil {
		p.VoiceMultipliers = make(map[string]float64)
	}
	for _, persona := range r.voices {
		if vID, ok := persona.Voices[string(contracts.ProviderElevenLabsTTS)]; ok && vID != "" && persona.CostMultiplier > 0 {
			if _, exists := p.VoiceMultipliers[vID]; !exists {
				p.VoiceMultipliers[vID] = persona.CostMultiplier
			}
		}
	}
	return p
}

// Pricing returns the resolved Pricing table.
func (r *ProviderRegistry) Pricing() services.Pricing {
	return r.pricingSnapshot
}

// DefaultVoiceMapping returns a map of lowercase persona names to their primary TTS voice IDs.
func (r *ProviderRegistry) DefaultVoiceMapping() map[string]string {
	m := make(map[string]string, len(r.voices))
	for k, v := range r.voices {
		if id, ok := v.Voices[string(contracts.ProviderElevenLabsTTS)]; ok && id != "" {
			m[k] = id
		}
	}
	return m
}

// Transcribe runs STT on the primary provider and automatically falls back on error.
func (r *ProviderRegistry) Transcribe(ctx context.Context, req STTRequest) (*STTResult, contracts.CallUsage, error) {
	route := r.catalog.Routing.STT
	if primary, ok := r.sttModels[route.Primary]; ok {
		res, u, err := primary.Transcribe(ctx, req)
		if err == nil {
			u.ModelKey = route.Primary
			return res, u, nil
		}
		r.log.Warn("primary stt failed, attempting fallback", "primary", route.Primary, "fallback", route.Fallback, "error", err)
		if fallback, ok := r.sttModels[route.Fallback]; ok && route.Fallback != "" && route.Fallback != route.Primary {
			res2, u2, err2 := fallback.Transcribe(ctx, req)
			if err2 == nil {
				u2.ModelKey = route.Fallback
				return res2, u2, nil
			}
		}
		return nil, u, err
	}
	return nil, contracts.CallUsage{}, fmt.Errorf("no stt provider configured for %q", route.Primary)
}

// Decide runs structured turn classification on the primary DecisionProvider and falls back to SchemaDecisionAdapter if needed.
func (r *ProviderRegistry) Decide(ctx context.Context, req DecisionRequest) (*DecisionResult, contracts.CallUsage, error) {
	route := r.catalog.Routing.Decisions
	if primary, ok := r.decisionModels[route.Primary]; ok {
		res, u, err := primary.Decide(ctx, req)
		if err == nil {
			u.ModelKey = route.Primary
			return res, u, nil
		}
		r.log.Warn("primary decisions provider failed, attempting fallback", "primary", route.Primary, "fallback", route.Fallback, "error", err)
		if fallback, ok := r.decisionModels[route.Fallback]; ok && route.Fallback != "" && route.Fallback != route.Primary {
			res2, u2, err2 := fallback.Decide(ctx, req)
			if err2 == nil {
				u2.ModelKey = route.Fallback
				return res2, u2, nil
			}
		}
		return nil, u, err
	}
	return nil, contracts.CallUsage{}, fmt.Errorf("no decision provider configured for %q", route.Primary)
}

// CompleteChat runs chat completion on the primary ChatFallback LLM and falls back if needed.
func (r *ProviderRegistry) CompleteChat(ctx context.Context, req LLMRequest) (string, contracts.CallUsage, error) {
	return r.completeLLMWithRoute(ctx, r.catalog.Routing.ChatFallback, req)
}

// CompleteEvaluation runs evaluation completion on the primary Evaluation LLM and falls back if needed.
func (r *ProviderRegistry) CompleteEvaluation(ctx context.Context, req LLMRequest) (string, contracts.CallUsage, error) {
	return r.completeLLMWithRoute(ctx, r.catalog.Routing.Evaluation, req)
}

func (r *ProviderRegistry) completeLLMWithRoute(ctx context.Context, route RouteConfig, req LLMRequest) (string, contracts.CallUsage, error) {
	if primary, ok := r.llmModels[route.Primary]; ok {
		res, u, err := primary.CompleteJSON(ctx, req)
		if err == nil {
			u.ModelKey = route.Primary
			return res, u, nil
		}
		r.log.Warn("primary llm failed, attempting fallback", "primary", route.Primary, "fallback", route.Fallback, "error", err)
		if fallback, ok := r.llmModels[route.Fallback]; ok && route.Fallback != "" && route.Fallback != route.Primary {
			res2, u2, err2 := fallback.CompleteJSON(ctx, req)
			if err2 == nil {
				u2.ModelKey = route.Fallback
				return res2, u2, nil
			}
		}
		return "", u, err
	}
	return "", contracts.CallUsage{}, fmt.Errorf("no llm provider configured for %q", route.Primary)
}

// Synthesize runs TTS synthesis on the primary TTS provider and falls back if needed.
func (r *ProviderRegistry) Synthesize(ctx context.Context, req TTSRequest) ([]byte, contracts.CallUsage, error) {
	route := r.catalog.Routing.TTS
	if primary, ok := r.ttsModels[route.Primary]; ok {
		audio, u, err := primary.Synthesize(ctx, req)
		if err == nil {
			u.ModelKey = route.Primary
			return audio, u, nil
		}
		r.log.Warn("primary tts failed, attempting fallback", "primary", route.Primary, "fallback", route.Fallback, "error", err)
		if fallback, ok := r.ttsModels[route.Fallback]; ok && route.Fallback != "" && route.Fallback != route.Primary {
			audio2, u2, err2 := fallback.Synthesize(ctx, req)
			if err2 == nil {
				u2.ModelKey = route.Fallback
				return audio2, u2, nil
			}
		}
		return nil, u, err
	}
	return nil, contracts.CallUsage{}, fmt.Errorf("no tts provider configured for %q", route.Primary)
}

func loadDefaultPersonas() map[string]PersonaVoiceSpec {
	defaults := []struct {
		name    string
		role    string
		voiceID string
	}{
		{"herr wagner", contracts.SpeakerRoleExaminer, "Jvf6TAXwMUVTSR20U0f9"},
		{"frau fischer", contracts.SpeakerRoleExaminer, "MGG5Irb57ATHvyIeTEYo"},
		{"frau erdoğan", contracts.SpeakerRoleExaminer, "zKHQdbB8oaQ7roNTiDTK"},
		{"herr richter", contracts.SpeakerRoleExaminer, "z1EhmmPwF0ENGYE8dBE6"},
		{"frau dr. meyer", contracts.SpeakerRoleExaminer, "4YKI1ONIhgWEKHyAHYPC"},
		{"leyla", contracts.SpeakerRoleParticipant, "NE7AIW5DoJ7lUosXV2KR"},
		{"omar", contracts.SpeakerRoleParticipant, "A2eYGaZmwykOiEVBlQ7h"},
		{"elena", contracts.SpeakerRoleParticipant, "gVOibprogMfmHVVyo5r6"},
		{"mateo", contracts.SpeakerRoleParticipant, "1J0wWp4zPQIvsK7Xwh34"},
		{"janusz", contracts.SpeakerRoleParticipant, "K5ZVtkkBnuPY6YqXs70E"},
	}
	m := make(map[string]PersonaVoiceSpec, len(defaults))
	for _, d := range defaults {
		m[d.name] = PersonaVoiceSpec{
			Name: d.name,
			Role: d.role,
			Voices: map[string]string{
				string(contracts.ProviderElevenLabsTTS): d.voiceID,
			},
			CostMultiplier: 1.0,
		}
	}
	return m
}

func mergeCatalogConfig(dst *CatalogConfig, src CatalogConfig) {
	if src.Routing.STT.Primary != "" {
		dst.Routing.STT = src.Routing.STT
	}
	if src.Routing.Decisions.Primary != "" {
		dst.Routing.Decisions = src.Routing.Decisions
	}
	if src.Routing.ChatFallback.Primary != "" {
		dst.Routing.ChatFallback = src.Routing.ChatFallback
	}
	if src.Routing.Evaluation.Primary != "" {
		dst.Routing.Evaluation = src.Routing.Evaluation
	}
	if src.Routing.TTS.Primary != "" {
		dst.Routing.TTS = src.Routing.TTS
	}
	for k, v := range src.Models {
		dst.Models[k] = v
	}
}

// loadProvidersYAML parses catalog/providers.yaml without requiring external YAML dependencies.
func loadProvidersYAML(path string) (CatalogConfig, error) {
	var cfg CatalogConfig
	cfg.Models = make(map[string]ModelSpec)

	b, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}

	var section, subSection, subSubSection string
	lines := strings.Split(string(b), "\n")
	for _, rawLine := range lines {
		if idx := strings.Index(rawLine, "#"); idx != -1 {
			rawLine = rawLine[:idx]
		}
		trimmed := strings.TrimSpace(rawLine)
		if trimmed == "" {
			continue
		}
		indent := len(rawLine) - len(strings.TrimLeft(rawLine, " "))

		if indent == 0 && strings.HasSuffix(trimmed, ":") {
			section = strings.TrimSuffix(trimmed, ":")
			subSection = ""
			subSubSection = ""
			continue
		}
		if indent == 2 && strings.HasSuffix(trimmed, ":") {
			subSection = strings.TrimSuffix(trimmed, ":")
			subSubSection = ""
			if section == "models" {
				if _, ok := cfg.Models[subSection]; !ok {
					cfg.Models[subSection] = ModelSpec{}
				}
			}
			continue
		}
		if indent == 4 && strings.HasSuffix(trimmed, ":") {
			subSubSection = strings.TrimSuffix(trimmed, ":")
			continue
		}

		kv := strings.SplitN(trimmed, ":", 2)
		if len(kv) != 2 {
			continue
		}
		k := strings.TrimSpace(kv[0])
		v := strings.Trim(strings.TrimSpace(kv[1]), `"'`)

		if section == "routing" && indent == 4 {
			var rc *RouteConfig
			switch subSection {
			case "stt":
				rc = &cfg.Routing.STT
			case "decisions":
				rc = &cfg.Routing.Decisions
			case "chat_fallback":
				rc = &cfg.Routing.ChatFallback
			case "evaluation":
				rc = &cfg.Routing.Evaluation
			case "tts":
				rc = &cfg.Routing.TTS
			}
			if rc != nil {
				switch k {
				case "primary":
					rc.Primary = v
				case "fallback":
					rc.Fallback = v
				}
			}
		} else if section == "models" && subSection != "" {
			ms := cfg.Models[subSection]
			if indent == 4 {
				switch k {
				case "provider":
					ms.Provider = v
				case "model":
					ms.Model = v
				case "temperature":
					if f, err := strconv.ParseFloat(v, 64); err == nil {
						ms.Temperature = f
					}
				case "stability":
					if f, err := strconv.ParseFloat(v, 64); err == nil {
						ms.Stability = f
					}
				case "similarity_boost":
					if f, err := strconv.ParseFloat(v, 64); err == nil {
						ms.SimilarityBoost = f
					}
				}
			} else if indent == 6 && subSubSection == "pricing" {
				if f, err := strconv.ParseFloat(v, 64); err == nil {
					switch k {
					case "per_audio_second_usd":
						ms.Pricing.PerAudioSecondUSD = f
					case "input_per_1m_usd":
						ms.Pricing.InputPer1MUSD = f
					case "cached_input_per_1m_usd":
						ms.Pricing.CachedInputPer1MUSD = f
					case "output_per_1m_usd":
						ms.Pricing.OutputPer1MUSD = f
					case "per_char_usd":
						ms.Pricing.PerCharUSD = f
					}
				}
			}
			cfg.Models[subSection] = ms
		}
	}
	return cfg, nil
}

// loadPersonasYAML parses catalog/personas/de_voices.yaml.
func loadPersonasYAML(path string) (map[string]PersonaVoiceSpec, error) {
	res := make(map[string]PersonaVoiceSpec)
	b, err := os.ReadFile(path)
	if err != nil {
		return res, err
	}

	var section, currentPersona, subSection string
	lines := strings.Split(string(b), "\n")
	for _, rawLine := range lines {
		if idx := strings.Index(rawLine, "#"); idx != -1 {
			rawLine = rawLine[:idx]
		}
		trimmed := strings.TrimSpace(rawLine)
		if trimmed == "" {
			continue
		}
		indent := len(rawLine) - len(strings.TrimLeft(rawLine, " "))

		if indent == 0 && strings.HasSuffix(trimmed, ":") {
			section = strings.TrimSuffix(trimmed, ":")
			currentPersona = ""
			subSection = ""
			continue
		}
		if (section == "examiners" || section == "partners") && indent == 2 && strings.HasSuffix(trimmed, ":") {
			name := strings.TrimSuffix(trimmed, ":")
			currentPersona = strings.ToLower(name)
			subSection = ""
			res[currentPersona] = PersonaVoiceSpec{
				Name:           name,
				Voices:         make(map[string]string),
				CostMultiplier: 1.0,
			}
			continue
		}
		if currentPersona != "" && indent == 4 && strings.HasSuffix(trimmed, ":") {
			subSection = strings.TrimSuffix(trimmed, ":")
			continue
		}

		kv := strings.SplitN(trimmed, ":", 2)
		if len(kv) != 2 || currentPersona == "" {
			continue
		}
		k := strings.TrimSpace(kv[0])
		v := strings.Trim(strings.TrimSpace(kv[1]), `"'`)
		p := res[currentPersona]
		if indent == 4 {
			switch k {
			case "role":
				p.Role = v
			case "gender":
				p.Gender = v
			case "bio":
				p.Bio = v
			case "cost_multiplier":
				if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
					p.CostMultiplier = f
				}
			}
		} else if indent == 6 && subSection == "voices" {
			p.Voices[k] = v
		}
		res[currentPersona] = p
	}
	return res, nil
}
