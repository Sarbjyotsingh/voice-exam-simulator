package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	GroqAPIKey             string
	ElevenLabsAPIKey       string
	GroqModel              string
	ElevenLabsVoiceID      string
	AWSRegion              string
	S3BucketName           string
	Port                   string
	MaxConnections         int32
	MaxAudioBytes          int64
	MaxHistoryMessages     int
	GroqMaxConcurrency     int
	ElevenLabsMaxConcurrency int
	ArchiveWorkers         int
	ArchiveQueueSize       int
	ArchiveInputAudio      bool
	UpstreamTimeout        time.Duration
}

func MustLoad() Config {
	c := Config{
		GroqAPIKey:             os.Getenv("GROQ_API_KEY"),
		ElevenLabsAPIKey:       os.Getenv("ELEVENLABS_API_KEY"),
		GroqModel:              getEnv("GROQ_MODEL", "openai/gpt-oss-120b"),
		ElevenLabsVoiceID:      getEnv("ELEVENLABS_VOICE_ID", "Jvf6TAXwMUVTSR20U0f9"),
		AWSRegion:              getEnv("AWS_REGION", "eu-central-1"),
		S3BucketName:           os.Getenv("S3_BUCKET_NAME"),
		Port:                   getEnv("PORT", "8000"),
		MaxConnections:         int32(getEnvInt("MAX_CONNECTIONS", 200)),
		MaxAudioBytes:          int64(getEnvInt("MAX_AUDIO_BYTES", 10485760)),
		MaxHistoryMessages:     getEnvInt("MAX_HISTORY_MESSAGES", 100),
		GroqMaxConcurrency:     getEnvInt("GROQ_MAX_CONCURRENCY", 8),
		ElevenLabsMaxConcurrency: getEnvInt("ELEVENLABS_MAX_CONCURRENCY", 4),
		ArchiveWorkers:         getEnvInt("ARCHIVE_WORKERS", 2),
		ArchiveQueueSize:       getEnvInt("ARCHIVE_QUEUE_SIZE", 16),
		ArchiveInputAudio:      getEnvBool("ARCHIVE_INPUT_AUDIO", true),
		UpstreamTimeout:        60 * time.Second,
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
