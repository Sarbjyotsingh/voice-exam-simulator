package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"

	"voice-engine/config"
	"voice-engine/internal/contracts"
	"voice-engine/services"
)

// OpenAISTT implements STTProvider using OpenAI's audio transcriptions endpoint.
type OpenAISTT struct {
	apiKey    string
	baseURL   string
	model     string
	client    *http.Client
	semaphore chan struct{}
}

// NewOpenAISTT creates an OpenAI STTProvider.
func NewOpenAISTT(apiKey, model string, maxConcurrency int, client *http.Client) *OpenAISTT {
	if model == "" {
		model = "gpt-4o-mini-transcribe"
	}
	if maxConcurrency <= 0 {
		maxConcurrency = 8
	}
	return &OpenAISTT{
		apiKey:    apiKey,
		baseURL:   config.OpenAIBaseURL,
		model:     model,
		client:    client,
		semaphore: make(chan struct{}, maxConcurrency),
	}
}

func (o *OpenAISTT) Name() contracts.ProviderID {
	return contracts.ProviderOpenAISTT
}

func (o *OpenAISTT) acquire(ctx context.Context) error {
	select {
	case o.semaphore <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (o *OpenAISTT) release() {
	<-o.semaphore
}

func (o *OpenAISTT) Transcribe(ctx context.Context, req STTRequest) (*STTResult, contracts.CallUsage, error) {
	var usage contracts.CallUsage
	usage.Provider = o.Name()

	if o.apiKey == "" {
		return nil, usage, fmt.Errorf("openai stt api key not configured")
	}
	if err := o.acquire(ctx); err != nil {
		return nil, usage, err
	}
	defer o.release()

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

	lang := string(req.Language)
	if lang == "" {
		lang = config.GroqSTTLanguage
	}
	prompt := req.Prompt
	if prompt == "" {
		prompt = config.GroqWhisperPrompt
	}

	_ = writer.WriteField("model", o.model)
	_ = writer.WriteField("language", lang)
	_ = writer.WriteField("prompt", prompt)
	_ = writer.WriteField("response_format", "json")

	if err := writer.Close(); err != nil {
		return nil, usage, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+"/audio/transcriptions", &buf)
	if err != nil {
		return nil, usage, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+o.apiKey)
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := o.client.Do(httpReq)
	if err != nil {
		return nil, usage, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, usage, fmt.Errorf("openai stt error %d: %s", resp.StatusCode, string(b))
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
		return nil, usage, fmt.Errorf("decode openai stt response: %w", err)
	}
	if parsed.Duration <= 0 {
		parsed.Duration = 10.0
	}
	usage.STTSeconds = parsed.Duration
	services.UsageFrom(ctx).AddSTT(parsed.Duration)

	return &STTResult{
		Text:            parsed.Text,
		DurationSeconds: parsed.Duration,
	}, usage, nil
}
