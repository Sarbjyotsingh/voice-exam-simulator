package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"

	"voice-exam-simulator/config"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Groq struct {
	apiKey     string
	model      string
	client     *http.Client
	semaphore  chan struct{}
}

func NewGroq(cfg config.Config, client *http.Client) *Groq {
	return &Groq{
		apiKey:    cfg.GroqAPIKey,
		model:     cfg.GroqModel,
		client:    client,
		semaphore: make(chan struct{}, cfg.GroqMaxConcurrency),
	}
}

func (g *Groq) acquire(ctx context.Context) error {
	select {
	case g.semaphore <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (g *Groq) release() {
	<-g.semaphore
}

func (g *Groq) Transcribe(ctx context.Context, wav []byte) (string, error) {
	if err := g.acquire(ctx); err != nil {
		return "", err
	}
	defer g.release()

	buf := getBuffer()
	defer putBuffer(buf)
	writer := multipart.NewWriter(buf)

	filename := "audio.wav"
	if len(wav) >= 8 && string(wav[4:8]) == "ftyp" {
		filename = "audio.m4a"
	}
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(part, bytes.NewReader(wav)); err != nil {
		return "", err
	}

	prompt := config.GroqWhisperPrompt
	if len(prompt) > 890 {
		prompt = prompt[:890]
	}

	_ = writer.WriteField("model", config.GroqWhisperModel)
	_ = writer.WriteField("response_format", "verbose_json")
	_ = writer.WriteField("language", config.GroqSTTLanguage)
	_ = writer.WriteField("prompt", prompt)
	_ = writer.WriteField("temperature", "0")

	if err := writer.Close(); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, config.GroqBaseURL+"/audio/transcriptions", buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+g.apiKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := g.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("transcription error %d: %s", resp.StatusCode, string(b))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// verbose_json carries the audio duration, which is what Groq bills STT on.
	var parsed struct {
		Text     string  `json:"text"`
		Duration float64 `json:"duration"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("failed to decode transcription response: %w", err)
	}
	UsageFrom(ctx).AddSTT(parsed.Duration)

	text := parsed.Text
	if IsHallucination(text) {
		return "", nil
	}
	return text, nil
}

func (g *Groq) Chat(ctx context.Context, msgs []Message) (string, error) {
	if err := g.acquire(ctx); err != nil {
		return "", err
	}
	defer g.release()

	payload := map[string]interface{}{
		"model":           g.model,
		"messages":        msgs,
		"response_format": map[string]string{"type": "json_object"},
		"stream":          false,
	}

	headers := map[string]string{
		"Authorization": "Bearer " + g.apiKey,
	}

	respBytes, err := postJSON(ctx, g.client, config.GroqBaseURL+"/chat/completions", headers, payload)
	if err != nil {
		return "", err
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(respBytes, &result); err != nil {
		return "", fmt.Errorf("failed to decode chat response: %w", err)
	}

	UsageFrom(ctx).AddLLM(result.Usage.PromptTokens, result.Usage.CompletionTokens)

	if len(result.Choices) > 0 && result.Choices[0].Message.Content != "" {
		return result.Choices[0].Message.Content, nil
	}

	return "{}", nil
}

func (g *Groq) Evaluate(ctx context.Context, prompt, transcript string) (string, error) {
	fullPrompt := prompt + config.EvaluationGuardrails + config.EvaluationJSONInstruction
	msgs := []Message{
		{Role: "system", Content: fullPrompt},
		{Role: "user", Content: transcript},
	}

	content, err := g.Chat(ctx, msgs)
	if err != nil {
		return "", err
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(content), &data); err != nil {
		// Fallback to raw content if not parseable
		return content, nil
	}

	requiredKeys := []string{"numerical_score", "telc_grade", "top_3_mistakes", "interaction_feedback", "foreigner_success_tip"}
	for _, key := range requiredKeys {
		if _, ok := data[key]; !ok {
			if key == "top_3_mistakes" {
				data[key] = []string{}
			} else {
				data[key] = ""
			}
		}
	}

	b, _ := json.Marshal(data)
	return string(b), nil
}
