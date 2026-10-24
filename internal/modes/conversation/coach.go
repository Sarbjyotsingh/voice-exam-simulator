package conversation

import (
	"context"
	"strings"

	"voice-engine/config"
	"voice-engine/internal/contracts"
	"voice-engine/internal/engine"
	"voice-engine/internal/modes/exam"
	"voice-engine/internal/providers"
)

// EvalClient is the subset of ProviderRegistry used by Coach.
type EvalClient interface {
	CompleteEvaluation(ctx context.Context, req providers.LLMRequest) (string, contracts.CallUsage, error)
}

// Coach generates friendly coaching and spoken grammar feedback for Conversation Helper sessions.
type Coach struct {
	llm      EvalClient
	composer *engine.PromptComposer
}

// NewCoach creates a Coach.
func NewCoach(llm EvalClient, composer *engine.PromptComposer) *Coach {
	return &Coach{
		llm:      llm,
		composer: composer,
	}
}

// Evaluate grades a Conversation Helper transcript and returns the standard evaluation JSON structure.
func (c *Coach) Evaluate(ctx context.Context, req contracts.EvaluateRequest) (string, error) {
	transcript := strings.TrimSpace(req.Transcript)
	if transcript == "" && len(req.History) > 0 {
		transcript = exam.FormatTranscriptFromHistory(req.History, req.PrimarySpeaker, req.SecondarySpeaker)
	}

	var fullPrompt string
	if strings.TrimSpace(req.EvaluationPrompt) != "" {
		fullPrompt = req.EvaluationPrompt + config.EvaluationGuardrails + config.EvaluationJSONInstruction
	} else {
		level := contracts.NormalizeCEFRLevel(req.ExamLevel)
		lang := contracts.NormalizeLanguage(req.Language)
		scen := req.ScenarioID
		if scen == "" {
			scen = req.ExamID
		}
		if scen == "" {
			scen = "b1_apartment_viewing"
		}
		fullPrompt = c.composer.Build(engine.PromptLayerQuery{
			Mode:       contracts.ModeConversation,
			Language:   lang,
			Level:      level,
			ScenarioID: scen,
			Kind:       contracts.PromptKindEvaluate,
		}, map[contracts.PromptVar]string{
			contracts.VarLevel:            string(level),
			contracts.VarScenarioTitle:    scen,
			contracts.VarPrimarySpeaker:   req.PrimarySpeaker,
			contracts.VarConversationText: transcript,
			contracts.VarTranscript:       transcript,
		}) + config.EvaluationJSONInstruction
	}

	content, _, err := c.llm.CompleteEvaluation(ctx, providers.LLMRequest{
		Messages: []contracts.ChatMessage{
			{Role: string(contracts.RoleSystem), Content: fullPrompt},
			{Role: string(contracts.RoleUser), Content: transcript},
		},
	})
	if err != nil {
		return "", err
	}
	return exam.NormalizeEvaluationJSON(content), nil
}
