package conversation

import (
	"context"
	"strings"

	"voice-exam-simulator/config"
	"voice-exam-simulator/internal/contracts"
	"voice-exam-simulator/internal/engine"
	"voice-exam-simulator/internal/providers"
)

// Plugin implements engine.ModeHandler for everyday speaking & roleplay practice (ModeConversation).
type Plugin struct {
	registry *providers.ProviderRegistry
	composer *engine.PromptComposer
	router   *ConversationRouter
	coach    *Coach
}

// NewPlugin creates the ModeConversation plugin.
func NewPlugin(registry *providers.ProviderRegistry, composer *engine.PromptComposer) *Plugin {
	return &Plugin{
		registry: registry,
		composer: composer,
		router:   NewConversationRouter(registry),
		coach:    NewCoach(registry, composer),
	}
}

func (p *Plugin) Mode() contracts.ModeID {
	return contracts.ModeConversation
}

// PrepareSession sets up the system prompt and voice mappings for a Conversation Helper session.
func (p *Plugin) PrepareSession(_ context.Context, initReq contracts.TurnInitRequest, s *engine.SessionContext) error {
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

	scen := s.ScenarioID
	if scen == "" {
		scen = s.ExamID
	}
	if scen == "" {
		scen = "b1_apartment_viewing"
	}
	primary := s.PrimarySpeaker
	if primary == "" {
		primary = "Leyla"
	}

	sysPrompt := strings.TrimSpace(initReq.SystemPrompt)
	if sysPrompt == "" && p.composer != nil {
		sysPrompt = p.composer.Build(engine.PromptLayerQuery{
			Mode:       contracts.ModeConversation,
			Language:   s.Language,
			Level:      s.ExamLevel,
			ScenarioID: scen,
			Kind:       contracts.PromptKindConversation,
		}, map[contracts.PromptVar]string{
			contracts.VarLevel:          string(s.ExamLevel),
			contracts.VarPrimarySpeaker: primary,
			contracts.VarScenarioTitle:  scen,
		})
	}
	if sysPrompt == "" {
		sysPrompt = config.DefaultSystemPrompt
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

// RouteTurn delegates to ConversationRouter.
func (p *Plugin) RouteTurn(ctx context.Context, s *engine.SessionContext, userUtterance string) (engine.TurnOutcome, error) {
	return p.router.RouteTurn(ctx, s, userUtterance)
}

// Evaluate delegates to Coach.
func (p *Plugin) Evaluate(ctx context.Context, req contracts.EvaluateRequest) (string, error) {
	return p.coach.Evaluate(ctx, req)
}
