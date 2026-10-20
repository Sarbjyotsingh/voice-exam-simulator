package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"ai_exam_simulator/config"
)

// DecisionQuestion defines a single question sent to OpenAI Decisions API.
// Supported types: "predicate", "choice", "score".
type DecisionQuestion struct {
	Type         string           `json:"type"`
	Name         string           `json:"name"`
	Instructions string           `json:"instructions"`
	Choices      []DecisionChoice `json:"choices,omitempty"`
}

type DecisionChoice struct {
	Value       string `json:"value"`
	Description string `json:"description"`
}

type ChoiceProbability struct {
	Value       string  `json:"value"`
	Probability float64 `json:"probability"`
}

type DecisionAnswer struct {
	Name          string              `json:"name"`
	Type          string              `json:"type"` // "predicate", "choice", "score", or "refusal"
	Probability   *float64            `json:"probability,omitempty"`
	Choice        string              `json:"choice,omitempty"`
	Confidence    *float64            `json:"confidence,omitempty"`
	Probabilities []ChoiceProbability `json:"probabilities,omitempty"`
	Score         *float64            `json:"score,omitempty"`
	Refusal       string              `json:"refusal,omitempty"`
}

type DecisionResponse struct {
	ID      string           `json:"id"`
	Model   string           `json:"model"`
	Answers []DecisionAnswer `json:"answers"`
	Usage   struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

// AnswerMap turns answers into a map keyed by question name for O(1) lookups.
func (r *DecisionResponse) AnswerMap() map[string]DecisionAnswer {
	m := make(map[string]DecisionAnswer, len(r.Answers))
	for _, a := range r.Answers {
		m[a.Name] = a
	}
	return m
}

// OpenAIDecisions is an HTTP client for POST https://api.openai.com/v1/decisions.
type OpenAIDecisions struct {
	apiKey    string
	model     string
	client    *http.Client
	semaphore chan struct{}
}

func NewOpenAIDecisions(cfg config.Config, client *http.Client) *OpenAIDecisions {
	if cfg.OpenAIAPIKey == "" {
		return nil
	}
	concurrency := cfg.DecisionsMaxConcurrency
	if concurrency <= 0 {
		concurrency = 8
	}
	return &OpenAIDecisions{
		apiKey:    cfg.OpenAIAPIKey,
		model:     config.OpenAIDecisionsModel,
		client:    client,
		semaphore: make(chan struct{}, concurrency),
	}
}

func (d *OpenAIDecisions) acquire(ctx context.Context) error {
	select {
	case d.semaphore <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (d *OpenAIDecisions) release() {
	<-d.semaphore
}

func (d *OpenAIDecisions) Decide(ctx context.Context, input string, questions []DecisionQuestion) (*DecisionResponse, error) {
	if err := d.acquire(ctx); err != nil {
		return nil, err
	}
	defer d.release()

	payload := map[string]interface{}{
		"model":     d.model,
		"input":     input,
		"questions": questions,
	}

	headers := map[string]string{
		"Authorization": "Bearer " + d.apiKey,
	}

	url := config.OpenAIBaseURL + "/decisions"
	respBytes, err := postJSON(ctx, d.client, url, headers, payload)
	if err != nil {
		return nil, err
	}

	var resp DecisionResponse
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		return nil, fmt.Errorf("failed to decode decisions response: %w", err)
	}

	// Meter input tokens on the current turn context
	UsageFrom(ctx).AddDecisions(resp.Usage.InputTokens)

	return &resp, nil
}
