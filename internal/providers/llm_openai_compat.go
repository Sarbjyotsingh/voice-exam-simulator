package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"voice-exam-simulator/internal/contracts"
	"voice-exam-simulator/services"
)

// OpenAICompatLLM implements LLMProvider for any OpenAI-compatible Chat Completions endpoint (Groq, OpenAI, Together, etc.).
type OpenAICompatLLM struct {
	providerID  contracts.ProviderID
	apiKey      string
	baseURL     string
	model       string
	temperature float64
	client      *http.Client
	semaphore   chan struct{}
}

// NewOpenAICompatLLM creates a universal OpenAI-compatible LLMProvider.
func NewOpenAICompatLLM(
	providerID contracts.ProviderID,
	apiKey, baseURL, model string,
	temperature float64,
	maxConcurrency int,
	client *http.Client,
) *OpenAICompatLLM {
	if maxConcurrency <= 0 {
		maxConcurrency = 8
	}
	return &OpenAICompatLLM{
		providerID:  providerID,
		apiKey:      apiKey,
		baseURL:     baseURL,
		model:       model,
		temperature: temperature,
		client:      client,
		semaphore:   make(chan struct{}, maxConcurrency),
	}
}

func (l *OpenAICompatLLM) Name() contracts.ProviderID {
	return l.providerID
}

func (l *OpenAICompatLLM) acquire(ctx context.Context) error {
	select {
	case l.semaphore <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (l *OpenAICompatLLM) release() {
	<-l.semaphore
}

func (l *OpenAICompatLLM) CompleteJSON(ctx context.Context, req LLMRequest) (string, contracts.CallUsage, error) {
	var usage contracts.CallUsage
	usage.Provider = l.Name()

	if l.apiKey == "" {
		return "", usage, fmt.Errorf("%s api key not configured", l.providerID)
	}
	if err := l.acquire(ctx); err != nil {
		return "", usage, err
	}
	defer l.release()

	wireMsgs := make([]map[string]string, 0, len(req.Messages))
	for _, m := range req.Messages {
		wireMsgs = append(wireMsgs, map[string]string{
			"role":    m.Role,
			"content": m.Content,
		})
	}

	temp := l.temperature
	if req.Temperature != nil {
		temp = *req.Temperature
	}

	payload := map[string]interface{}{
		"model":           l.model,
		"messages":        wireMsgs,
		"response_format": map[string]string{"type": "json_object"},
		"stream":          false,
	}
	if temp > 0 {
		payload["temperature"] = temp
	}

	b, err := json.Marshal(payload)
	if err != nil {
		return "", usage, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, l.baseURL+"/chat/completions", bytes.NewReader(b))
	if err != nil {
		return "", usage, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+l.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := l.client.Do(httpReq)
	if err != nil {
		return "", usage, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", usage, fmt.Errorf("%s chat error %d: %s", l.providerID, resp.StatusCode, string(body))
	}

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", usage, err
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
		return "", usage, fmt.Errorf("decode %s chat response: %w", l.providerID, err)
	}

	usage.PromptTokens = result.Usage.PromptTokens
	usage.CompletionTokens = result.Usage.CompletionTokens
	services.UsageFrom(ctx).AddLLM(result.Usage.PromptTokens, result.Usage.CompletionTokens)

	if len(result.Choices) > 0 && result.Choices[0].Message.Content != "" {
		return result.Choices[0].Message.Content, usage, nil
	}
	return "{}", usage, nil
}
