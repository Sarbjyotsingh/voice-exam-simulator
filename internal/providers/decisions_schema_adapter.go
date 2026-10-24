package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"voice-engine/internal/contracts"
)

// SchemaDecisionAdapter implements DecisionProvider on top of ANY standard Chat LLMProvider
// by converting []contracts.DecisionQuestion into a structured JSON prompt & schema.
type SchemaDecisionAdapter struct {
	llm LLMProvider
}

// NewSchemaDecisionAdapter wraps any LLMProvider as a DecisionProvider.
func NewSchemaDecisionAdapter(llm LLMProvider) *SchemaDecisionAdapter {
	return &SchemaDecisionAdapter{llm: llm}
}

func (a *SchemaDecisionAdapter) Name() contracts.ProviderID {
	return contracts.ProviderSchemaDecisions
}

func (a *SchemaDecisionAdapter) Decide(ctx context.Context, req DecisionRequest) (*DecisionResult, contracts.CallUsage, error) {
	var usage contracts.CallUsage
	usage.Provider = a.Name()
	if a.llm == nil {
		return nil, usage, fmt.Errorf("schema decision adapter has nil llm provider")
	}

	var sb strings.Builder
	sb.WriteString("You are a fast, deterministic turn classifier for an oral language exam.\n")
	sb.WriteString("Evaluate the input below and answer every question in a single JSON object keyed by question name.\n")
	sb.WriteString("For predicate questions, output a boolean (true or false).\n")
	sb.WriteString("For choice questions, output the exact choice value string from the allowed options.\n\n")
	sb.WriteString("QUESTIONS:\n")

	for _, q := range req.Questions {
		if q.Type == contracts.DecisionTypePredicate {
			sb.WriteString(fmt.Sprintf("- %q (boolean): %s\n", q.Name, q.Instructions))
		} else if q.Type == contracts.DecisionTypeChoice {
			var opts []string
			for _, c := range q.Choices {
				opts = append(opts, fmt.Sprintf("%q (%s)", c.Value, c.Description))
			}
			sb.WriteString(fmt.Sprintf("- %q (string choice from [%s]): %s\n", q.Name, strings.Join(opts, ", "), q.Instructions))
		}
	}

	temp := 0.0
	rawJSON, llmUsage, err := a.llm.CompleteJSON(ctx, LLMRequest{
		Messages: []contracts.ChatMessage{
			{Role: string(contracts.RoleSystem), Content: sb.String()},
			{Role: string(contracts.RoleUser), Content: req.Input},
		},
		Temperature: &temp,
	})
	usage.DecisionTokens = llmUsage.PromptTokens + llmUsage.CompletionTokens
	if err != nil {
		return nil, usage, err
	}

	var rawMap map[string]interface{}
	if err := json.Unmarshal([]byte(rawJSON), &rawMap); err != nil {
		return nil, usage, fmt.Errorf("schema decision adapter invalid json: %w", err)
	}

	answers := make([]contracts.DecisionAnswer, 0, len(req.Questions))
	for _, q := range req.Questions {
		val, exists := rawMap[q.Name]
		if !exists {
			continue
		}
		ans := contracts.DecisionAnswer{
			Name: q.Name,
			Type: q.Type,
		}
		switch q.Type {
		case contracts.DecisionTypePredicate:
			prob := 0.0
			switch v := val.(type) {
			case bool:
				if v {
					prob = 1.0
				}
			case float64:
				prob = v
			case string:
				if strings.EqualFold(v, "true") || v == "1" {
					prob = 1.0
				}
			}
			ans.Probability = &prob
		case contracts.DecisionTypeChoice:
			if s, ok := val.(string); ok {
				ans.Choice = s
				conf := 0.90
				ans.Confidence = &conf
			}
		}
		answers = append(answers, ans)
	}

	return &DecisionResult{
		ID:      "schema_adapter",
		Model:   string(a.llm.Name()),
		Answers: answers,
	}, usage, nil
}
