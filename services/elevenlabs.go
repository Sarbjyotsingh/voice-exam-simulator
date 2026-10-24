package services

import (
	"context"
	"net/http"

	"voice-engine/config"
)

type ElevenLabs struct {
	apiKey    string
	client    *http.Client
	semaphore chan struct{}
}

func NewElevenLabs(cfg config.Config, client *http.Client) *ElevenLabs {
	return &ElevenLabs{
		apiKey:    cfg.ElevenLabsAPIKey,
		client:    client,
		semaphore: make(chan struct{}, cfg.ElevenLabsMaxConcurrency),
	}
}

func (e *ElevenLabs) acquire(ctx context.Context) error {
	select {
	case e.semaphore <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (e *ElevenLabs) release() {
	<-e.semaphore
}

func (e *ElevenLabs) Synthesize(ctx context.Context, text, voiceID string) ([]byte, error) {
	if err := e.acquire(ctx); err != nil {
		return nil, err
	}
	defer e.release()

	url := config.ElevenLabsBaseURL + "/text-to-speech/" + voiceID

	payload := map[string]interface{}{
		"text":     text,
		"model_id": config.ElevenLabsModel,
		"voice_settings": map[string]interface{}{
			"stability":        0.5,
			"similarity_boost": 0.5,
		},
	}

	headers := map[string]string{
		"Accept":     "audio/mpeg",
		"xi-api-key": e.apiKey,
	}

	audio, err := postJSON(ctx, e.client, url, headers, payload)
	if err != nil {
		return nil, err
	}
	UsageFrom(ctx).AddTTS(voiceID, text)
	return audio, nil
}
