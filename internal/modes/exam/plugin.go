package exam

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"voice-exam-simulator/config"
	"voice-exam-simulator/internal/contracts"
	"voice-exam-simulator/internal/engine"
	"voice-exam-simulator/internal/providers"
	"voice-exam-simulator/services"
)

// Plugin implements engine.ModeHandler for structured multi-phase speaking exams (ModeExam).
type Plugin struct {
	cfg       config.Config
	registry  *providers.ProviderRegistry
	composer  *engine.PromptComposer
	cache     *ScriptCache
	router    *ExamDecisionRouter
	evaluator *ExamEvaluator
	log       *slog.Logger
}

// NewPlugin creates the ModeExam plugin.
func NewPlugin(
	cfg config.Config,
	registry *providers.ProviderRegistry,
	composer *engine.PromptComposer,
	cache *ScriptCache,
	router *ExamDecisionRouter,
	log *slog.Logger,
) *Plugin {
	if log == nil {
		log = slog.Default()
	}
	ev := NewExamEvaluator(registry, composer, cache, router)
	return &Plugin{
		cfg:       cfg,
		registry:  registry,
		composer:  composer,
		cache:     cache,
		router:    router,
		evaluator: ev,
		log:       log.With("component", "exam_plugin"),
	}
}

func (p *Plugin) Mode() contracts.ModeID {
	return contracts.ModeExam
}

// Router returns the underlying ExamDecisionRouter.
func (p *Plugin) Router() *ExamDecisionRouter {
	return p.router
}

// PrepareSession initializes a SessionContext from either a thin-client metadata payload or a legacy system_prompt payload.
func (p *Plugin) PrepareSession(ctx context.Context, initReq contracts.TurnInitRequest, s *engine.SessionContext) error {
	if s.TeilNumber <= 0 {
		s.TeilNumber = 1
	}

	// Merge default persona voice mappings from catalog/personas/de_voices.yaml with any client overrides
	if s.VoiceMapping == nil {
		s.VoiceMapping = make(map[string]string)
	}
	if p.registry != nil {
		for k, v := range p.registry.DefaultVoiceMapping() {
			if _, exists := s.VoiceMapping[k]; !exists {
				s.VoiceMapping[k] = v
			}
		}
	}
	for k, v := range initReq.VoiceMapping {
		s.VoiceMapping[strings.ToLower(k)] = v
	}

	// 1. Try loading pre-recorded script.json from ScriptCache if not embedded in SystemPrompt
	var dialoguesJSON string
	if s.Catalogue == nil && strings.TrimSpace(initReq.SystemPrompt) != "" {
		cat, err := services.ParseScriptCatalogue(initReq.SystemPrompt, s.PrimarySpeaker, s.SecondarySpeaker)
		if err == nil {
			s.Catalogue = cat
		}
	}
	if s.Catalogue == nil && p.cache != nil {
		if entry, err := p.cache.Get(ctx, s.ExamProvider, s.ExamLevel, s.ExamID, s.TeilNumber); err == nil && entry != nil {
			s.Catalogue = entry.Catalogue
			dialoguesJSON = entry.DialoguesJSON
			if s.PrimarySpeaker == "" {
				s.PrimarySpeaker = entry.PrimarySpeaker
			}
			if s.SecondarySpeaker == "" {
				s.SecondarySpeaker = entry.SecondarySpeaker
			}
			if s.ExamHint == "" {
				s.ExamHint = entry.ExamHint
			}
			for k, v := range entry.VoiceProfiles {
				s.VoiceMapping[strings.ToLower(k)] = v
			}
		}
	}

	// 2. Build or preserve SystemPrompt
	sysPrompt := strings.TrimSpace(initReq.SystemPrompt)
	if sysPrompt == "" && p.composer != nil {
		teilName := fmt.Sprintf("Teil %d", s.TeilNumber)
		if p.router != nil {
			if spec := p.router.GetSpec(string(s.ExamLevel), s.TeilNumber); spec != nil && spec.TeilName != "" {
				teilName = spec.TeilName
			}
		}
		primary := s.PrimarySpeaker
		if primary == "" {
			primary = "Leyla"
		}
		secondary := s.SecondarySpeaker
		if secondary == "" {
			secondary = "Herr Wagner"
		}
		sysPrompt = p.composer.Build(engine.PromptLayerQuery{
			Mode:         contracts.ModeExam,
			Language:     s.Language,
			ExamProvider: s.ExamProvider,
			Level:        s.ExamLevel,
			TeilNumber:   s.TeilNumber,
			Kind:         contracts.PromptKindFallbackSystem,
		}, map[contracts.PromptVar]string{
			contracts.VarLevel:            string(s.ExamLevel),
			contracts.VarTeilName:         teilName,
			contracts.VarPrimarySpeaker:   primary,
			contracts.VarSecondarySpeaker: secondary,
			contracts.VarScriptJSON:       dialoguesJSON,
		})
	}
	if sysPrompt == "" {
		sysPrompt = config.DefaultSystemPrompt
	}
	if initReq.UserInstructions != "" && !strings.Contains(sysPrompt, initReq.UserInstructions) {
		sysPrompt += "\nConstraints: " + initReq.UserInstructions
	}
	if !strings.Contains(sysPrompt, "parts") {
		sysPrompt += config.JSONInstruction
	}
	s.SystemPrompt = sysPrompt

	if len(s.Messages) == 0 || s.Messages[0].Role != string(contracts.RoleSystem) {
		s.Messages = append([]contracts.ChatMessage{{
			Role:    string(contracts.RoleSystem),
			Content: sysPrompt,
		}}, s.Messages...)
	} else {
		s.Messages[0].Content = sysPrompt
	}

	return nil
}

// RouteTurn routes the learner's utterance through ExamDecisionRouter first, falling back to Chat LLM if needed.
func (p *Plugin) RouteTurn(ctx context.Context, s *engine.SessionContext, userUtterance string) (engine.TurnOutcome, error) {
	if p.cfg.UseDecisionsAPI && p.cfg.DecisionsEnabledTeils[s.TeilNumber] && p.router != nil && s.Catalogue != nil {
		svcHistory := make([]services.Message, 0, len(s.Messages))
		for _, m := range s.Messages {
			content := m.Content
			if len(m.AudioIDs) > 0 {
				content += " " + strings.Join(m.AudioIDs, " ")
			}
			svcHistory = append(svcHistory, services.Message{Role: m.Role, Content: content})
		}
		res, err := p.router.RouteTurn(ctx, string(s.ExamLevel), s.TeilNumber, s.Catalogue, svcHistory, userUtterance)
		if err == nil && !res.Fallback && len(res.Parts) > 0 {
			return engine.TurnOutcome{
				RawJSON:   res.Response,
				Parts:     res.Parts,
				ExamEnded: res.ExamEnded,
			}, nil
		}
	}

	// Fallback to Chat LLM
	chatResp, _, err := p.registry.CompleteChat(ctx, providers.LLMRequest{
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

// Evaluate delegates to ExamEvaluator.
func (p *Plugin) Evaluate(ctx context.Context, req contracts.EvaluateRequest) (string, error) {
	return p.evaluator.Evaluate(ctx, req)
}
