package exam

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"voice-exam-simulator/config"
	"voice-exam-simulator/internal/contracts"
	"voice-exam-simulator/internal/engine"
	"voice-exam-simulator/internal/providers"
)

// CompletionClient is the subset of ProviderRegistry or LLMProvider used by ExamEvaluator.
type CompletionClient interface {
	CompleteEvaluation(ctx context.Context, req providers.LLMRequest) (string, contracts.CallUsage, error)
}

// ExamEvaluator grades exam attempts using either the 4-layer PromptComposer stack or legacy client prompts.
type ExamEvaluator struct {
	llm      CompletionClient
	composer *engine.PromptComposer
	cache    *ScriptCache
	router   *ExamDecisionRouter
}

// NewExamEvaluator creates an ExamEvaluator.
func NewExamEvaluator(
	llm CompletionClient,
	composer *engine.PromptComposer,
	cache *ScriptCache,
	router *ExamDecisionRouter,
) *ExamEvaluator {
	return &ExamEvaluator{
		llm:      llm,
		composer: composer,
		cache:    cache,
		router:   router,
	}
}

// Evaluate grades an exam transcript and guarantees the 5 required JSON keys.
func (e *ExamEvaluator) Evaluate(ctx context.Context, req contracts.EvaluateRequest) (string, error) {
	transcript := strings.TrimSpace(req.Transcript)
	if transcript == "" && len(req.History) > 0 {
		transcript = FormatTranscriptFromHistory(req.History, req.PrimarySpeaker, req.SecondarySpeaker)
	}

	var fullPrompt string
	if strings.TrimSpace(req.EvaluationPrompt) != "" {
		// 100% Backwards-Compatible Legacy Path: use client's evaluation_prompt + backend guardrails
		fullPrompt = req.EvaluationPrompt + config.EvaluationGuardrails + config.EvaluationJSONInstruction
	} else {
		// Thin-Client Server-Assembled 4-Layer Prompt Path
		level := contracts.NormalizeCEFRLevel(req.ExamLevel)
		provider := contracts.NormalizeExamProvider(req.ExamProvider)
		lang := contracts.NormalizeLanguage(req.Language)
		teil := req.TeilNumber
		if teil <= 0 {
			teil = 1
		}

		primary := req.PrimarySpeaker
		secondary := req.SecondarySpeaker
		examHint := req.ExamHint

		if e.cache != nil && req.ExamID != "" {
			if entry, err := e.cache.Get(ctx, provider, level, req.ExamID, teil); err == nil && entry != nil {
				if primary == "" {
					primary = entry.PrimarySpeaker
				}
				if secondary == "" {
					secondary = entry.SecondarySpeaker
				}
				if examHint == "" {
					examHint = entry.ExamHint
				}
			}
		}
		if primary == "" {
			primary = "Leyla"
		}
		if secondary == "" {
			secondary = "Herr Wagner"
		}
		if examHint == "" && e.router != nil {
			examHint = e.router.DefaultHint(string(level), teil)
		}

		teilName := fmt.Sprintf("Teil %d", teil)
		maxScore := 15
		if e.router != nil {
			if spec := e.router.GetSpec(string(level), teil); spec != nil && spec.TeilName != "" {
				teilName = spec.TeilName
			}
			maxScore = e.router.MaxScore(string(level), teil)
		}

		vars := map[contracts.PromptVar]string{
			contracts.VarLevel:            string(level),
			contracts.VarTeilName:         teilName,
			contracts.VarMaxScore:         strconv.Itoa(maxScore),
			contracts.VarPrimarySpeaker:   primary,
			contracts.VarSecondarySpeaker: secondary,
			contracts.VarExamHint:         examHint,
			contracts.VarConversationText: transcript,
			contracts.VarTranscript:       transcript,
		}

		fullPrompt = e.composer.Build(engine.PromptLayerQuery{
			Mode:         contracts.ModeExam,
			Language:     lang,
			ExamProvider: provider,
			Level:        level,
			TeilNumber:   teil,
			Kind:         contracts.PromptKindEvaluate,
		}, vars)
	}

	content, _, err := e.llm.CompleteEvaluation(ctx, providers.LLMRequest{
		Messages: []contracts.ChatMessage{
			{Role: string(contracts.RoleSystem), Content: fullPrompt},
			{Role: string(contracts.RoleUser), Content: transcript},
		},
	})
	if err != nil {
		return "", err
	}

	return NormalizeEvaluationJSON(content), nil
}

// FormatTranscriptFromHistory formats structured chat history into a human-readable exam transcript.
func FormatTranscriptFromHistory(history []contracts.ChatMessage, primary, secondary string) string {
	if primary == "" {
		primary = "Partner"
	}
	var lines []string
	for _, m := range history {
		if m.Role == string(contracts.RoleSystem) {
			continue
		}
		if m.Role == string(contracts.RoleUser) {
			lines = append(lines, "Candidate: "+m.Content)
			continue
		}
		// Assistant message may be JSON {"parts": [...]} or plain text
		var parsed struct {
			Parts []struct {
				Speaker string `json:"speaker"`
				Text    string `json:"text"`
			} `json:"parts"`
		}
		if err := json.Unmarshal([]byte(m.Content), &parsed); err == nil && len(parsed.Parts) > 0 {
			for _, p := range parsed.Parts {
				spk := p.Speaker
				if spk == "" {
					spk = primary
				}
				if p.Text != "" {
					lines = append(lines, fmt.Sprintf("%s: %s", spk, p.Text))
				}
			}
		} else if m.Content != "" {
			lines = append(lines, fmt.Sprintf("%s: %s", primary, m.Content))
		}
	}
	return strings.Join(lines, "\n")
}

// NormalizeEvaluationJSON ensures all 5 required evaluation keys are present in the returned JSON string.
func NormalizeEvaluationJSON(content string) string {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(content), &data); err != nil {
		return content
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
	b, err := json.Marshal(data)
	if err != nil {
		return content
	}
	return string(b)
}
