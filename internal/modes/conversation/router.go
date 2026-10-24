package conversation

import (
	"context"
	"encoding/json"

	"voice-exam-simulator/internal/contracts"
	"voice-exam-simulator/internal/engine"
	"voice-exam-simulator/internal/providers"
)

// ChatClient is the subset of ProviderRegistry used by ConversationRouter.
type ChatClient interface {
	CompleteChat(ctx context.Context, req providers.LLMRequest) (string, contracts.CallUsage, error)
}

// ConversationRouter routes turns for everyday roleplay and free-talk conversations.
type ConversationRouter struct {
	llm ChatClient
}

// NewConversationRouter creates a ConversationRouter.
func NewConversationRouter(llm ChatClient) *ConversationRouter {
	return &ConversationRouter{llm: llm}
}

// RouteTurn generates the next conversational turn via Chat LLM.
func (r *ConversationRouter) RouteTurn(ctx context.Context, s *engine.SessionContext, _ string) (engine.TurnOutcome, error) {
	chatResp, _, err := r.llm.CompleteChat(ctx, providers.LLMRequest{
		Messages: s.Messages,
	})
	if err != nil {
		return engine.TurnOutcome{}, err
	}

	var parsed struct {
		Parts     []map[string]interface{} `json:"parts"`
		ExamEnded bool                     `json:"exam_ended"`
	}
	if err := json.Unmarshal([]byte(chatResp), &parsed); err != nil {
		parsed.Parts = []map[string]interface{}{}
	}
	return engine.TurnOutcome{
		RawJSON:   chatResp,
		Parts:     parsed.Parts,
		ExamEnded: parsed.ExamEnded,
	}, nil
}
