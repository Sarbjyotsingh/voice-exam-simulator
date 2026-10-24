package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"voice-engine/config"
	"voice-engine/internal/contracts"
	"voice-engine/services"
)

// OpenAIDecisionsProvider implements DecisionProvider using OpenAI's /v1/decisions endpoint (gpt-6-luna).
type OpenAIDecisionsProvider struct {
	apiKey    string
	baseURL   string
	model     string
	client    *http.Client
	semaphore chan struct{}
}

// NewOpenAIDecisionsProvider creates a new OpenAIDecisionsProvider.
func NewOpenAIDecisionsProvider(apiKey, model string, maxConcurrency int, client *http.Client) *OpenAIDecisionsProvider {
	if model == "" {
		model = config.OpenAIDecisionsModel
	}
	if maxConcurrency <= 0 {
		maxConcurrency = 8
	}
	return &OpenAIDecisionsProvider{
		apiKey:    apiKey,
		baseURL:   config.OpenAIBaseURL,
		model:     model,
		client:    client,
		semaphore: make(chan struct{}, maxConcurrency),
	}
}

func (d *OpenAIDecisionsProvider) Name() contracts.ProviderID {
	return contracts.ProviderOpenAIDecisions
}

func (d *OpenAIDecisionsProvider) acquire(ctx context.Context) error {
	select {
	case d.semaphore <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (d *OpenAIDecisionsProvider) release() {
	<-d.semaphore
}

func (d *OpenAIDecisionsProvider) Decide(ctx context.Context, req DecisionRequest) (*DecisionResult, contracts.CallUsage, error) {
	var usage contracts.CallUsage
	usage.Provider = d.Name()

	if d.apiKey == "" {
		return nil, usage, fmt.Errorf("openai decisions api key not configured")
	}
	if err := d.acquire(ctx); err != nil {
		return nil, usage, err
	}
	defer d.release()

	payload := map[string]interface{}{
		"model":     d.model,
		"input":     req.Input,
		"questions": req.Questions,
	}

	b, err := json.Marshal(payload)
	if err != nil {
		return nil, usage, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, d.baseURL+"/decisions", bytes.NewReader(b))
	if err != nil {
		return nil, usage, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+d.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := d.client.Do(httpReq)
	if err != nil {
		return nil, usage, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, usage, fmt.Errorf("openai decisions error %d: %s", resp.StatusCode, string(body))
	}

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, usage, err
	}

	var parsed struct {
		ID      string                     `json:"id"`
		Model   string                     `json:"model"`
		Answers []contracts.DecisionAnswer `json:"answers"`
		Usage   struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
			TotalTokens  int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(respBytes, &parsed); err != nil {
		return nil, usage, fmt.Errorf("decode decisions response: %w", err)
	}

	usage.DecisionTokens = parsed.Usage.InputTokens
	services.UsageFrom(ctx).AddDecisions(parsed.Usage.InputTokens)

	return &DecisionResult{
		ID:      parsed.ID,
		Model:   parsed.Model,
		Answers: parsed.Answers,
	}, usage, nil
}
