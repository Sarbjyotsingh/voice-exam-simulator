package engine

import (
	"context"

	"voice-exam-simulator/internal/contracts"
	"voice-exam-simulator/services"
)

// SessionContext carries resolved session state for one active WebSocket connection or evaluation request.
type SessionContext struct {
	SessionID        string
	Mode             contracts.ModeID
	Language         contracts.LanguageCode
	ExamProvider     contracts.ExamProvider
	ExamLevel        contracts.CEFRLevel
	TeilNumber       int
	ExamID           string
	ScenarioID       string
	PrimarySpeaker   string
	SecondarySpeaker string
	DefaultVoiceID   string
	VoiceMapping     map[string]string
	SystemPrompt     string
	ExamHint         string
	Messages         []contracts.ChatMessage
	Attempt          services.AttemptKey
	Catalogue        *services.ScriptCatalogue
}

// TurnOutcome represents the output of a routed or LLM-generated turn.
type TurnOutcome struct {
	RawJSON   string
	Parts     []map[string]interface{}
	ExamEnded bool
}

// ModeHandler is the plug-and-play interface implemented by domain modes (exam vs. conversation).
type ModeHandler interface {
	Mode() contracts.ModeID
	PrepareSession(ctx context.Context, initReq contracts.TurnInitRequest, s *SessionContext) error
	RouteTurn(ctx context.Context, s *SessionContext, userUtterance string) (TurnOutcome, error)
	Evaluate(ctx context.Context, req contracts.EvaluateRequest) (string, error)
}
