package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	GroqAPIKey               string
	ElevenLabsAPIKey         string
	GroqModel                string
	ElevenLabsVoiceID        string
	AWSRegion                string
	S3BucketName             string
	Port                     string
	MaxConnections           int32
	MaxAudioBytes            int64
	MaxHistoryMessages       int
	GroqMaxConcurrency       int
	ElevenLabsMaxConcurrency int
	ArchiveWorkers           int
	ArchiveQueueSize         int
	ArchiveInputAudio        bool
	UpstreamTimeout          time.Duration

	// Cost metering (USD). Overridable via env so price changes need no code change.
	WhisperPricePerHourUSD  float64
	GroqInputPer1MUSD       float64
	GroqOutputPer1MUSD      float64
	ElevenLabsPer1KCharsUSD float64
	DecisionsInputPer1MUSD  float64
	VoiceCostMultipliers    map[string]float64
	LedgerDir               string // on-disk per-user cost ledger (mount a volume in production)

	// OpenAI Decisions API (fast non-autoregressive turn router)
	OpenAIAPIKey          string
	UseDecisionsAPI       bool
	DecisionsMaxConcurrency int
	DecisionsEnabledTeils map[int]bool
}

func MustLoad() Config {
	c := Config{
		GroqAPIKey:               os.Getenv("GROQ_API_KEY"),
		ElevenLabsAPIKey:         os.Getenv("ELEVENLABS_API_KEY"),
		GroqModel:                getEnv("GROQ_MODEL", "openai/gpt-oss-120b"),
		ElevenLabsVoiceID:        getEnv("ELEVENLABS_VOICE_ID", "Jvf6TAXwMUVTSR20U0f9"),
		AWSRegion:                getEnv("AWS_REGION", "eu-central-1"),
		S3BucketName:             os.Getenv("S3_BUCKET_NAME"),
		Port:                     getEnv("PORT", "8000"),
		MaxConnections:           int32(getEnvInt("MAX_CONNECTIONS", 200)),
		MaxAudioBytes:            int64(getEnvInt("MAX_AUDIO_BYTES", 10485760)),
		MaxHistoryMessages:       getEnvInt("MAX_HISTORY_MESSAGES", 100),
		GroqMaxConcurrency:       getEnvInt("GROQ_MAX_CONCURRENCY", 8),
		ElevenLabsMaxConcurrency: getEnvInt("ELEVENLABS_MAX_CONCURRENCY", 4),
		ArchiveWorkers:           getEnvInt("ARCHIVE_WORKERS", 2),
		ArchiveQueueSize:         getEnvInt("ARCHIVE_QUEUE_SIZE", 16),
		ArchiveInputAudio:        getEnvBool("ARCHIVE_INPUT_AUDIO", true),
		UpstreamTimeout:          60 * time.Second,

		WhisperPricePerHourUSD:  getEnvFloat("COST_WHISPER_PER_HOUR_USD", DefaultWhisperPricePerHourUSD),
		GroqInputPer1MUSD:       getEnvFloat("COST_GROQ_INPUT_PER_1M_USD", DefaultGroqInputPer1MUSD),
		GroqOutputPer1MUSD:      getEnvFloat("COST_GROQ_OUTPUT_PER_1M_USD", DefaultGroqOutputPer1MUSD),
		ElevenLabsPer1KCharsUSD: getEnvFloat("COST_ELEVENLABS_PER_1K_CHARS_USD", DefaultElevenLabsFlashPer1KCharsUSD),
		DecisionsInputPer1MUSD:  getEnvFloat("COST_DECISIONS_INPUT_PER_1M_USD", DefaultDecisionsInputPer1MUSD),
		VoiceCostMultipliers:    loadVoiceMultipliers(os.Getenv("ELEVENLABS_VOICE_MULTIPLIERS")),
		LedgerDir:               getEnv("LEDGER_DIR", "./data/ledger"),

		OpenAIAPIKey:            os.Getenv("OPENAI_API_KEY"),
		UseDecisionsAPI:         getEnvBool("USE_DECISIONS_API", os.Getenv("OPENAI_API_KEY") != ""),
		DecisionsMaxConcurrency: getEnvInt("DECISIONS_MAX_CONCURRENCY", 8),
		DecisionsEnabledTeils:   loadEnabledTeils(getEnv("DECISIONS_ENABLED_TEILS", "1,2,3")),
	}

	if c.GroqAPIKey == "" {
		panic("GROQ_API_KEY is required")
	}
	if c.ElevenLabsAPIKey == "" {
		panic("ELEVENLABS_API_KEY is required")
	}

	return c
}

func (c *Config) Addr() string {
	return fmt.Sprintf(":%s", c.Port)
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return def
}

func getEnvBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

func getEnvFloat(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}

// loadVoiceMultipliers merges the built-in per-voice multipliers with an optional
// override string of the form "voiceID:1.5,otherVoiceID:2".
func loadVoiceMultipliers(override string) map[string]float64 {
	m := make(map[string]float64, len(VoiceCostMultipliers))
	for k, v := range VoiceCostMultipliers {
		m[k] = v
	}
	for _, pair := range strings.Split(override, ",") {
		kv := strings.SplitN(strings.TrimSpace(pair), ":", 2)
		if len(kv) != 2 {
			continue
		}
		if f, err := strconv.ParseFloat(strings.TrimSpace(kv[1]), 64); err == nil && f > 0 {
			m[strings.TrimSpace(kv[0])] = f
		}
	}
	return m
}

func loadEnabledTeils(s string) map[int]bool {
	m := make(map[int]bool)
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if n, err := strconv.Atoi(part); err == nil && n > 0 {
			m[n] = true
		}
	}
	if len(m) == 0 {
		m[1] = true
	}
	return m
}
