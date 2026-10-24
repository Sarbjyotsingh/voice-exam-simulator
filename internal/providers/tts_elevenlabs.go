package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"unicode/utf8"

	"voice-engine/config"
	"voice-engine/internal/contracts"
	"voice-engine/services"
)

// ElevenLabsTTS implements TTSProvider using ElevenLabs Flash v2.5.
type ElevenLabsTTS struct {
	apiKey          string
	baseURL         string
	model           string
	stability       float64
	similarityBoost float64
	client          *http.Client
	semaphore       chan struct{}
}

// NewElevenLabsTTS creates a new ElevenLabsTTS provider.
func NewElevenLabsTTS(apiKey, model string, stability, similarityBoost float64, maxConcurrency int, client *http.Client) *ElevenLabsTTS {
	if model == "" {
		model = config.ElevenLabsModel
	}
	if stability <= 0 {
		stability = 0.5
	}
	if similarityBoost <= 0 {
		similarityBoost = 0.5
	}
	if maxConcurrency <= 0 {
		maxConcurrency = 4
	}
	return &ElevenLabsTTS{
		apiKey:          apiKey,
		baseURL:         config.ElevenLabsBaseURL,
		model:           model,
		stability:       stability,
		similarityBoost: similarityBoost,
		client:          client,
		semaphore:       make(chan struct{}, maxConcurrency),
	}
}

func (e *ElevenLabsTTS) Name() contracts.ProviderID {
	return contracts.ProviderElevenLabsTTS
}

func (e *ElevenLabsTTS) acquire(ctx context.Context) error {
	select {
	case e.semaphore <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (e *ElevenLabsTTS) release() {
	<-e.semaphore
}

func (e *ElevenLabsTTS) Synthesize(ctx context.Context, req TTSRequest) ([]byte, contracts.CallUsage, error) {
	var usage contracts.CallUsage
	usage.Provider = e.Name()
	usage.VoiceID = req.VoiceID

	if e.apiKey == "" {
		return nil, usage, fmt.Errorf("elevenlabs api key not configured")
	}
	if err := e.acquire(ctx); err != nil {
		return nil, usage, err
	}
	defer e.release()

	payload := map[string]interface{}{
		"text":     req.Text,
		"model_id": e.model,
		"voice_settings": map[string]interface{}{
			"stability":        e.stability,
			"similarity_boost": e.similarityBoost,
		},
	}

	b, err := json.Marshal(payload)
	if err != nil {
		return nil, usage, err
	}

	url := e.baseURL + "/text-to-speech/" + req.VoiceID
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return nil, usage, err
	}
	httpReq.Header.Set("Accept", "audio/mpeg")
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("xi-api-key", e.apiKey)

	resp, err := e.client.Do(httpReq)
	if err != nil {
		return nil, usage, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, usage, fmt.Errorf("elevenlabs tts error %d: %s", resp.StatusCode, string(body))
	}

	audio, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, usage, err
	}

	usage.TTSCharacters = utf8.RuneCountInString(req.Text)
	services.UsageFrom(ctx).AddTTS(req.VoiceID, req.Text)

	return audio, usage, nil
}
