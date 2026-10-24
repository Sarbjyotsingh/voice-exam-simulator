package contracts

import "strings"

// PromptKind identifies which prompt stack to compose.
type PromptKind string

const (
	PromptKindDecisions      PromptKind = "decisions"
	PromptKindFallbackSystem PromptKind = "fallback_system"
	PromptKindFallbackUser   PromptKind = "fallback_user"
	PromptKindEvaluate       PromptKind = "evaluate"
	PromptKindConversation   PromptKind = "conversation"
)

// PromptVar defines template placeholder tokens resolved by PromptComposer.
type PromptVar string

const (
	VarLevel            PromptVar = "{{level}}"
	VarTeilName         PromptVar = "{{teil_name}}"
	VarMaxScore         PromptVar = "{{max_score}}"
	VarPrimarySpeaker   PromptVar = "{{primary_speaker}}"
	VarSecondarySpeaker PromptVar = "{{secondary_speaker}}"
	VarExamHint         PromptVar = "{{exam_hint}}"
	VarConversationText PromptVar = "{{conversation_text}}"
	VarTranscript       PromptVar = "{{transcript}}"
	VarScriptJSON       PromptVar = "{{script_json}}"
	VarScenarioTitle    PromptVar = "{{scenario_title}}"
	VarScenarioRole     PromptVar = "{{scenario_role}}"
)

// ApplyPromptVars replaces all PromptVar placeholders in text with their corresponding values.
func ApplyPromptVars(text string, vars map[PromptVar]string) string {
	if len(vars) == 0 || text == "" {
		return text
	}
	pairs := make([]string, 0, len(vars)*2)
	for k, v := range vars {
		pairs = append(pairs, string(k), v)
	}
	// If {{transcript}} or {{conversation_text}} is set, mirror it to the other if missing.
	if v, ok := vars[VarConversationText]; ok {
		if _, has := vars[VarTranscript]; !has {
			pairs = append(pairs, string(VarTranscript), v)
		}
	} else if v, ok := vars[VarTranscript]; ok {
		pairs = append(pairs, string(VarConversationText), v)
	}
	return strings.NewReplacer(pairs...).Replace(text)
}
