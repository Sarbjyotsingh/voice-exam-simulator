package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"

	"voice-exam-simulator/config"
	"voice-exam-simulator/internal/contracts"
	"voice-exam-simulator/services"
)

// GroqSTT implements STTProvider using Groq's Whisper API.
type GroqSTT struct {
	apiKey    string
	baseURL   string
	model     string
	client    *http.Client
	semaphore chan struct{}
}

// NewGroqSTT creates a Groq Whisper STTProvider.
func NewGroqSTT(apiKey, model string, maxConcurrency int, client *http.Client) *GroqSTT {
	if model == "" {
		model = config.GroqWhisperModel
	}
	if maxConcurrency <= 0 {
		maxConcurrency = 8
	}
	return &GroqSTT{
		apiKey:    apiKey,
		baseURL:   config.GroqBaseURL,
		model:     model,
		client:    client,
		semaphore: make(chan struct{}, maxConcurrency),
	}
}

func (g *GroqSTT) Name() contracts.ProviderID {
	return contracts.ProviderGroqSTT
}

func (g *GroqSTT) acquire(ctx context.Context) error {
	select {
	case g.semaphore <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (g *GroqSTT) release() {
	<-g.semaphore
}

func (g *GroqSTT) Transcribe(ctx context.Context, req STTRequest) (*STTResult, contracts.CallUsage, error) {
	var usage contracts.CallUsage
	usage.Provider = g.Name()

	if err := g.acquire(ctx); err != nil {
		return nil, usage, err
	}
	defer g.release()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	filename := "audio.wav"
	if len(req.Audio) >= 8 && string(req.Audio[4:8]) == "ftyp" {
		filename = "audio.m4a"
	}
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, usage, err
	}
	if _, err := io.Copy(part, bytes.NewReader(req.Audio)); err != nil {
		return nil, usage, err
	}

	prompt := req.Prompt
	if prompt == "" {
		prompt = config.GroqWhisperPrompt
	}
	if len(prompt) > 890 {
		prompt = prompt[:890]
	}
	lang := string(req.Language)
	if lang == "" {
		lang = config.GroqSTTLanguage
	}

	_ = writer.WriteField("model", g.model)
	_ = writer.WriteField("response_format", "verbose_json")
	_ = writer.WriteField("language", lang)
	_ = writer.WriteField("prompt", prompt)
	_ = writer.WriteField("temperature", "0")

	if err := writer.Close(); err != nil {
		return nil, usage, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, g.baseURL+"/audio/transcriptions", &buf)
	if err != nil {
		return nil, usage, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+g.apiKey)
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := g.client.Do(httpReq)
	if err != nil {
		return nil, usage, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, usage, fmt.Errorf("groq stt error %d: %s", resp.StatusCode, string(b))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, usage, err
	}

	var parsed struct {
		Text     string  `json:"text"`
		Duration float64 `json:"duration"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, usage, fmt.Errorf("decode groq stt response: %w", err)
	}

	usage.STTSeconds = parsed.Duration
	services.UsageFrom(ctx).AddSTT(parsed.Duration)

	return &STTResult{
		Text:            parsed.Text,
		DurationSeconds: parsed.Duration,
	}, usage, nil
}
