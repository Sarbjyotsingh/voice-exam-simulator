package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
)

// ScriptClip represents one pre-recorded audio snippet from script.json.
type ScriptClip struct {
	AudioID string `json:"audio_id"`
	Text    string `json:"text"`
	Speaker string `json:"speaker"`
	Role    string `json:"role"`
}

// ScriptCatalogue indexes all available pre-recorded audio clips in a session by slot.
type ScriptCatalogue struct {
	PrimarySpeaker   string
	SecondarySpeaker string
	ClipsByID        map[string]ScriptClip

	// Partner clips: key is slot code, e.g. "ORIG", "EXP_01", "DUSIE", "CLOSE_01"
	PartnerQuestions map[string]ScriptClip // e.g. "ORIG" -> _Q1 clip (or _Q2 if only Q2 exists)
	PartnerQ2        map[string]ScriptClip // e.g. "ORIG" -> _Q2 clip
	PartnerAnswers   map[string]ScriptClip // e.g. "ORIG" -> _A1 clip
	PartnerExp       map[string]ScriptClip // e.g. "EXP_01" -> EXP_01 clip
	PartnerSpecial   map[string]ScriptClip // e.g. "DUSIE_A1", "CLOSE_01", "CLAR_Q1", "SIL_Q1"

	// Examiner clips: key is slot code, e.g. "FOLLOW_01", "INT_TOPIC", "OUTRO_01"
	ExaminerClips map[string]ScriptClip
}

// Regex to parse generic clip IDs like:
// DE_LEY_ORIG_Q1 -> Partner="LEY", Slot="ORIG", Kind="Q1"
// DE_LEY_EXP_01  -> Partner="LEY", Slot="EXP_01"
// DE_EX1_FOLLOW_01 -> Examiner="EX1", Slot="FOLLOW_01"
// DE_EX_INT_TOPIC -> Examiner="EX", Slot="INT_TOPIC"
var (
	partnerClipRe  = regexp.MustCompile(`^DE_([A-Z]{3})_([A-Z0-9]+)(?:_([A-Z0-9]+))?$`)
	examinerClipRe = regexp.MustCompile(`^DE_(?:EX\d*|W(?:_\d+)?)(?:_([A-Z0-9_]+))?$`)
)

// ParseScriptCatalogue extracts all dialogues from the system prompt or script JSON string.
func ParseScriptCatalogue(rawScript string, primary, secondary string) (*ScriptCatalogue, error) {
	// 1. Locate the JSON array of dialogues
	jsonText := rawScript
	if start := strings.Index(rawScript, "AVAILABLE PRE-RECORDED AUDIO SCRIPT:\n"); start != -1 {
		jsonText = rawScript[start+len("AVAILABLE PRE-RECORDED AUDIO SCRIPT:\n"):]
		if end := strings.Index(jsonText, "\n\n### 1. PERSONAS"); end != -1 {
			jsonText = jsonText[:end]
		}
	}

	jsonText = strings.TrimSpace(jsonText)
	if !strings.HasPrefix(jsonText, "[") {
		return nil, fmt.Errorf("script text does not start with JSON array")
	}

	var dialogues []struct {
		Speaker     string `json:"speaker"`
		Role        string `json:"role"`
		Topic       string `json:"topic"`
		Interaction map[string]struct {
			AudioID string `json:"audio_id"`
			Text    string `json:"text"`
		} `json:"interaction"`
	}

	if err := json.Unmarshal([]byte(jsonText), &dialogues); err != nil {
		return nil, fmt.Errorf("failed to parse dialogue script json: %w", err)
	}

	cat := &ScriptCatalogue{
		PrimarySpeaker:   primary,
		SecondarySpeaker: secondary,
		ClipsByID:        make(map[string]ScriptClip),
		PartnerQuestions: make(map[string]ScriptClip),
		PartnerQ2:        make(map[string]ScriptClip),
		PartnerAnswers:   make(map[string]ScriptClip),
		PartnerExp:       make(map[string]ScriptClip),
		PartnerSpecial:   make(map[string]ScriptClip),
		ExaminerClips:    make(map[string]ScriptClip),
	}

	for _, d := range dialogues {
		speaker := d.Speaker
		if speaker == "" {
			if strings.EqualFold(d.Role, "Examiner") {
				speaker = secondary
			} else {
				speaker = primary
			}
		}
		role := d.Role
		if role == "" {
			if strings.EqualFold(speaker, secondary) {
				role = "Examiner"
			} else {
				role = "Participant"
			}
		}

		for _, item := range d.Interaction {
			if item.AudioID == "" {
				continue
			}
			clip := ScriptClip{
				AudioID: item.AudioID,
				Text:    item.Text,
				Speaker: speaker,
				Role:    role,
			}
			cat.ClipsByID[item.AudioID] = clip

			// Categorize clip
			id := item.AudioID
			if strings.Contains(id, "_EXP_") {
				// e.g. DE_LEY_EXP_01 -> key "EXP_01"
				idx := strings.Index(id, "EXP_")
				expCode := id[idx:]
				cat.PartnerExp[expCode] = clip
			} else if strings.Contains(id, "_EX") || strings.Contains(id, "DE_W_") {
				// Examiner clip: find slot name
				// e.g. DE_EX1_FOLLOW_01 -> "FOLLOW_01"
				// e.g. DE_EX1_INT_TOPIC -> "INT_TOPIC"
				// e.g. DE_EX1_OUTRO_01 -> "OUTRO_01"
				for _, slot := range []string{"FOLLOW_01", "FOLLOW_02", "FOLLOW_03", "FOLLOW_04", "FOLLOW_05",
					"INT_TOPIC", "INT_POLITE", "INT_DIALOG", "OUTRO_01", "OUTRO_02", "INTRO_01", "INTRO_02"} {
					if strings.Contains(id, slot) {
						cat.ExaminerClips[slot] = clip
						break
					}
				}
			} else {
				// Partner topic question / answer
				// e.g. DE_LEY_ORIG_Q1, DE_LEY_ORIG_A1, DE_LEY_DUSIE_A1, DE_LEY_CLOSE_01, DE_LEY_CLAR_Q1, DE_LEY_SIL_Q1
				parts := strings.Split(id, "_")
				if len(parts) >= 4 {
					slot := parts[2]
					kind := parts[3]
					switch kind {
					case "Q1":
						cat.PartnerQuestions[slot] = clip
					case "Q2":
						cat.PartnerQ2[slot] = clip
						if _, exists := cat.PartnerQuestions[slot]; !exists {
							cat.PartnerQuestions[slot] = clip
						}
					case "A1":
						cat.PartnerAnswers[slot] = clip
					default:
						cat.PartnerSpecial[slot+"_"+kind] = clip
					}
				}
			}
		}
	}

	return cat, nil
}

// DecisionRouter coordinates OpenAI Decisions API and deterministic clip composition.
type DecisionRouter struct {
	decisions *OpenAIDecisions
	registry  *SpecRegistry
	log       *slog.Logger
}

func NewDecisionRouter(decisions *OpenAIDecisions, registry *SpecRegistry, logger *slog.Logger) *DecisionRouter {
	return &DecisionRouter{
		decisions: decisions,
		registry:  registry,
		log:       logger.With("component", "decision_router"),
	}
}

// RouteResult is returned by RouteTurn.
type RouteResult struct {
	Fallback  bool                   // if true, handler must fall back to Groq Chat + ElevenLabs
	Response  string                 // JSON string matching {"exam_ended": bool, "parts": [...]}
	ExamEnded bool
	Parts     []map[string]interface{}
}

// ConversationState holds information extracted from previous turn history.
type ConversationState struct {
	CoveredTopics          map[string]bool
	UsedExpressions        map[string]bool
	UsedAnswers            map[string]bool
	LastAskedTopic         string
	AwaitingFollowUpAnswer bool
	TurnCount              int
}

// AnalyzeHistory inspects the message history and reconstructs state.
func AnalyzeHistory(history []Message, cat *ScriptCatalogue, spec *TeilDecisionSpec) ConversationState {
	st := ConversationState{
		CoveredTopics:   make(map[string]bool),
		UsedExpressions: make(map[string]bool),
		UsedAnswers:     make(map[string]bool),
	}

	// Seed with initial covered topics from spec (e.g. "name" covered in Turn 0 intro)
	for _, t := range spec.InitialCoveredTopics {
		st.CoveredTopics[t] = true
	}

	for _, m := range history {
		if m.Role == "system" {
			// Skip the system prompt! The system prompt embeds the entire script.json
			// catalogue, so scanning it would falsely match every clip and follow-up question.
			continue
		}
		if m.Role == "user" {
			st.TurnCount++
			continue
		}
		// Inspect assistant message to see which clips were used
		for code, clip := range cat.PartnerExp {
			if strings.Contains(m.Content, clip.Text) || strings.Contains(m.Content, clip.AudioID) {
				st.UsedExpressions[code] = true
			}
		}

		for _, topic := range spec.Topics {
			if q, ok := cat.PartnerQuestions[topic.SlotCode]; ok {
				if strings.Contains(m.Content, q.Text) || strings.Contains(m.Content, q.AudioID) {
					st.CoveredTopics[topic.Key] = true
					st.LastAskedTopic = topic.SlotCode
				}
			}
			if q2, ok := cat.PartnerQ2[topic.SlotCode]; ok {
				if strings.Contains(m.Content, q2.Text) || strings.Contains(m.Content, q2.AudioID) {
					st.CoveredTopics[topic.Key] = true
					st.LastAskedTopic = topic.SlotCode
				}
			}
			if a, ok := cat.PartnerAnswers[topic.SlotCode]; ok {
				if strings.Contains(m.Content, a.Text) || strings.Contains(m.Content, a.AudioID) {
					st.UsedAnswers[topic.SlotCode] = true
					// In Turn 1, learners often introduce name + origin together without ORIG_Q1 being asked;
					// when the partner responded with ORIG_A1, origin is already covered.
					if topic.Key == "origin" {
						st.CoveredTopics[topic.Key] = true
					}
				}
			}
		}

		// Check if Examiner asked a follow-up question
		for _, f := range spec.FollowUpSlots {
			if clip, ok := cat.ExaminerClips[f.SlotCode]; ok {
				if strings.Contains(m.Content, clip.Text) || strings.Contains(m.Content, clip.AudioID) {
					st.AwaitingFollowUpAnswer = true
				}
			}
		}
	}

	return st
}

// RouteTurn evaluates the learner's message using Decisions API and composes parts.
func (r *DecisionRouter) RouteTurn(
	ctx context.Context,
	level string,
	teilNumber int,
	cat *ScriptCatalogue,
	history []Message,
	userUtterance string,
) (RouteResult, error) {
	if r.decisions == nil {
		return RouteResult{Fallback: true}, nil
	}

	spec := r.registry.Get(level, teilNumber)
	if spec == nil {
		return RouteResult{Fallback: true}, nil
	}

	st := AnalyzeHistory(history, cat, spec)

	// If learner was answering the Examiner's follow-up question (Phase 3 -> Phase 4 outro):
	if st.AwaitingFollowUpAnswer {
		outroClip, ok := cat.ExaminerClips[spec.ExaminerOutroSlot]
		if !ok {
			return RouteResult{Fallback: true}, nil
		}
		parts := []map[string]interface{}{
			{
				"audio_id": outroClip.AudioID,
				"text":     outroClip.Text,
				"speaker":  outroClip.Speaker,
				"role":     outroClip.Role,
			},
		}
		respJSON, _ := json.Marshal(map[string]interface{}{
			"exam_ended": true,
			"parts":      parts,
		})
		return RouteResult{
			Fallback:  false,
			Response:  string(respJSON),
			ExamEnded: true,
			Parts:     parts,
		}, nil
	}

	// Build questions for Decisions API
	questions := r.buildQuestions(spec, st)

	// Build concise input prompt
	var coveredList []string
	for t := range st.CoveredTopics {
		coveredList = append(coveredList, t)
	}
	recentHistory := formatRecentHistory(history, 4)
	input := fmt.Sprintf("%s Partner: %s. Topics already covered: %s.\nRecent conversation:\n%s\nLearner's latest message: %s",
		spec.ContextSummary, cat.PrimarySpeaker, strings.Join(coveredList, ", "), recentHistory, userUtterance)

	decResp, err := r.decisions.Decide(ctx, input, questions)
	if err != nil {
		r.log.Warn("decisions api error, falling back to groq", "error", err)
		return RouteResult{Fallback: true}, nil
	}

	ans := decResp.AnswerMap()

	// 1. Check fallback predicate
	if fallbackAns, ok := ans["needs_custom_fallback"]; ok && fallbackAns.Probability != nil {
		if *fallbackAns.Probability >= spec.FallbackThreshold {
			r.log.Info("decisions api flagged custom response needed, falling back to groq", "prob", *fallbackAns.Probability)
			return RouteResult{Fallback: true}, nil
		}
	}

	// 2. Check rude / inappropriate
	if rudeAns, ok := ans["rude_or_inappropriate"]; ok && rudeAns.Probability != nil && *rudeAns.Probability >= 0.70 {
		return r.composeIntervention(spec.Interventions.PolitenessSlot, cat, st)
	}

	// 3. Check off-topic
	if offAns, ok := ans["off_topic"]; ok && offAns.Probability != nil && *offAns.Probability >= 0.70 {
		return r.composeIntervention(spec.Interventions.OffTopicSlot, cat, st)
	}

	// 4. Check clarification requested
	if clarAns, ok := ans["asked_clarification"]; ok && clarAns.Probability != nil && *clarAns.Probability >= 0.70 {
		return r.composeClarification(spec, cat, st)
	}

	// 5. Standard Turn Composition
	return r.composeNormalTurn(spec, cat, st, ans)
}

func (r *DecisionRouter) buildQuestions(spec *TeilDecisionSpec, st ConversationState) []DecisionQuestion {
	qs := []DecisionQuestion{
		{
			Type:         "predicate",
			Name:         "needs_custom_fallback",
			Instructions: spec.FallbackInstruction,
		},
		{
			Type:         "predicate",
			Name:         "asked_clarification",
			Instructions: "Is the learner saying they did not understand, asking to repeat (e.g. 'Wie bitte?', 'Kannst du das wiederholen?', 'Ich habe nicht verstanden'), or asking what something means?",
		},
		{
			Type:         "predicate",
			Name:         "learner_asked_partner",
			Instructions: fmt.Sprintf("Does the learner's message ask the partner (%s) a question or hand the turn back (e.g. 'Und du?', 'Woher kommst du?', 'Hast du Kinder?')?", spec.TeilName),
		},
		{
			Type:         "predicate",
			Name:         "off_topic",
			Instructions: "Is the learner's message completely unrelated to the exam topic (e.g. politics, news, sports commentary)?",
		},
		{
			Type:         "predicate",
			Name:         "rude_or_inappropriate",
			Instructions: "Is the learner's message rude, insulting, offensive, vulgar, or inappropriate for an exam?",
		},
	}

	// Topic choice question
	var topicChoices []DecisionChoice
	var partnerAskedChoices []DecisionChoice
	partnerAskedChoices = append(partnerAskedChoices,
		DecisionChoice{Value: "same", Description: "asked about the same topic with 'Und du?' / 'Und bei dir?'"},
		DecisionChoice{Value: "none", Description: "did not ask the partner any question"},
	)
	for _, t := range spec.Topics {
		topicChoices = append(topicChoices, DecisionChoice{Value: t.Key, Description: t.Description})
		partnerAskedChoices = append(partnerAskedChoices, DecisionChoice{Value: t.Key, Description: "asked the partner about " + t.Description})
	}
	topicChoices = append(topicChoices,
		DecisionChoice{Value: "thanks", Description: "thanking / polite closing remark (e.g. 'Danke')"},
		DecisionChoice{Value: "none", Description: "none of the standard topics"})

	qs = append(qs,
		DecisionQuestion{
			Type:         "choice",
			Name:         "topic_addressed",
			Instructions: "Which topic does the learner's own answer mainly talk about?",
			Choices:      topicChoices,
		},
		DecisionQuestion{
			Type:         "choice",
			Name:         "partner_question_topic",
			Instructions: "If the learner asked the partner a question at the end of their message, which topic did they ask the partner about?",
			Choices:      partnerAskedChoices,
		},
	)

	// Reaction choice question (filter out already-used reactions if spec.NoRepeatReactions is true)
	var reactionChoices []DecisionChoice
	for _, rx := range spec.Reactions {
		if spec.NoRepeatReactions && st.UsedExpressions[rx.Code] {
			continue // skip already used expressions
		}
		reactionChoices = append(reactionChoices, DecisionChoice{Value: rx.Code, Description: rx.Description})
	}
	if len(reactionChoices) == 0 {
		// All used: re-allow all
		for _, rx := range spec.Reactions {
			reactionChoices = append(reactionChoices, DecisionChoice{Value: rx.Code, Description: rx.Description})
		}
	}

	qs = append(qs, DecisionQuestion{
		Type:         "choice",
		Name:         "reaction_clip",
		Instructions: "Pick the single reaction clip matching the learner's sentiment and content.",
		Choices:      reactionChoices,
	})

	return qs
}

func (r *DecisionRouter) composeIntervention(examinerSlot string, cat *ScriptCatalogue, st ConversationState) (RouteResult, error) {
	examinerClip, ok := cat.ExaminerClips[examinerSlot]
	if !ok {
		return RouteResult{Fallback: true}, nil
	}

	parts := []map[string]interface{}{
		{
			"audio_id": examinerClip.AudioID,
			"text":     examinerClip.Text,
			"speaker":  examinerClip.Speaker,
			"role":     examinerClip.Role,
		},
	}

	// Re-ask the last asked topic question if available
	lastSlot := st.LastAskedTopic
	if lastSlot != "" {
		if q, ok := cat.PartnerQuestions[lastSlot]; ok {
			parts = append(parts, map[string]interface{}{
				"audio_id": q.AudioID,
				"text":     q.Text,
				"speaker":  q.Speaker,
				"role":     q.Role,
			})
		}
	}

	respJSON, _ := json.Marshal(map[string]interface{}{
		"exam_ended": false,
		"parts":      parts,
	})
	return RouteResult{
		Fallback:  false,
		Response:  string(respJSON),
		ExamEnded: false,
		Parts:     parts,
	}, nil
}

func (r *DecisionRouter) composeClarification(spec *TeilDecisionSpec, cat *ScriptCatalogue, st ConversationState) (RouteResult, error) {
	var parts []map[string]interface{}

	// EXP_05 ("Ach so, ich verstehe.")
	if expClip, ok := cat.PartnerExp[spec.Interventions.ClarifyExpSlot]; ok {
		parts = append(parts, map[string]interface{}{
			"audio_id": expClip.AudioID,
			"text":     expClip.Text,
			"speaker":  expClip.Speaker,
			"role":     expClip.Role,
		})
	}

	// Re-ask either the Q2 of the current topic or repeat Q1 or CLAR_Q1
	lastSlot := st.LastAskedTopic
	reasked := false
	if lastSlot != "" {
		if q2, ok := cat.PartnerQ2[lastSlot]; ok {
			parts = append(parts, map[string]interface{}{
				"audio_id": q2.AudioID,
				"text":     q2.Text,
				"speaker":  q2.Speaker,
				"role":     q2.Role,
			})
			reasked = true
		} else if q1, ok := cat.PartnerQuestions[lastSlot]; ok {
			parts = append(parts, map[string]interface{}{
				"audio_id": q1.AudioID,
				"text":     q1.Text,
				"speaker":  q1.Speaker,
				"role":     q1.Role,
			})
			reasked = true
		}
	}

	if !reasked {
		if clarClip, ok := cat.PartnerSpecial[spec.Interventions.ClarifyQuestion]; ok {
			parts = append(parts, map[string]interface{}{
				"audio_id": clarClip.AudioID,
				"text":     clarClip.Text,
				"speaker":  clarClip.Speaker,
				"role":     clarClip.Role,
			})
		}
	}

	respJSON, _ := json.Marshal(map[string]interface{}{
		"exam_ended": false,
		"parts":      parts,
	})
	return RouteResult{
		Fallback:  false,
		Response:  string(respJSON),
		ExamEnded: false,
		Parts:     parts,
	}, nil
}

func (r *DecisionRouter) composeNormalTurn(
	spec *TeilDecisionSpec,
	cat *ScriptCatalogue,
	st ConversationState,
	ans map[string]DecisionAnswer,
) (RouteResult, error) {
	if st.CoveredTopics == nil {
		st.CoveredTopics = make(map[string]bool)
	}
	if st.UsedExpressions == nil {
		st.UsedExpressions = make(map[string]bool)
	}
	if st.UsedAnswers == nil {
		st.UsedAnswers = make(map[string]bool)
	}
	var parts []map[string]interface{}

	topicAddressed := ""
	originMentionedOnTurn1 := false
	if ta, ok := ans["topic_addressed"]; ok && ta.Choice != "" {
		// Only consider topic addressed if confidence / top probability is reasonable (>= 0.35)
		if ta.Confidence == nil || *ta.Confidence >= 0.35 {
			topicAddressed = ta.Choice
		}
		if st.TurnCount <= 1 {
			for _, p := range ta.Probabilities {
				if p.Value == "origin" && p.Probability >= 0.25 {
					originMentionedOnTurn1 = true
					break
				}
			}
		}
	}

	learnerAsked := false
	if la, ok := ans["learner_asked_partner"]; ok && la.Probability != nil && *la.Probability >= 0.50 {
		learnerAsked = true
	}

	// Find the SlotCode for the topic addressed
	var addressedSlot string
	for _, t := range spec.Topics {
		if t.Key == topicAddressed {
			addressedSlot = t.SlotCode
			// Only mark "origin" covered directly from topicAddressed (for Turn 1/2 when volunteered before ORIG_Q1).
			// All other required topics are marked covered only when their _Q1/_Q2 question has been asked,
			// ensuring no topic question (like german_learning or hobbies) is ever skipped.
			if t.Key == "origin" {
				st.CoveredTopics[t.Key] = true
			}
			break
		}
	}
	if st.TurnCount <= 1 && originMentionedOnTurn1 {
		st.CoveredTopics["origin"] = true
		if addressedSlot == "" || addressedSlot == "NAME" {
			addressedSlot = "ORIG"
		}
	}

	// Determine which topic's answer clip (_A1) the partner should play if asked a counter-question
	answerSlot := addressedSlot
	if pq, ok := ans["partner_question_topic"]; ok && pq.Choice != "" && pq.Choice != "none" && pq.Choice != "same" {
		if pq.Confidence == nil || *pq.Confidence >= 0.35 {
			for _, t := range spec.Topics {
				if t.Key == pq.Choice {
					answerSlot = t.SlotCode
					break
				}
			}
		}
	}

	// 1. Part 1: Reaction Expression
	reactionCode := ""
	if rx, ok := ans["reaction_clip"]; ok && rx.Choice != "" {
		reactionCode = rx.Choice
	}
	// Turn 1 special rule: if learner just shared their name, use EXP_11 ("Das freut mich!")
	if st.TurnCount <= 1 && topicAddressed == "name" {
		reactionCode = "EXP_11"
	}
	// Mid-exam "Danke": use EXP_15 ("Gerne! Kein Problem.")
	if topicAddressed == "thanks" {
		reactionCode = spec.Interventions.ThanksExpSlot
	}

	if reactionCode != "" {
		if expClip, ok := cat.PartnerExp[reactionCode]; ok {
			parts = append(parts, map[string]interface{}{
				"audio_id": expClip.AudioID,
				"text":     expClip.Text,
				"speaker":  expClip.Speaker,
				"role":     expClip.Role,
			})
			st.UsedExpressions[reactionCode] = true
		}
	}

	// 2. Part 2: Partner Answer (only if learner asked a question, or shared origin on Turn 1)
	shouldAnswer := learnerAsked
	if st.TurnCount <= 1 && (topicAddressed == "origin" || originMentionedOnTurn1) {
		shouldAnswer = true
	}

	// Turn 1 anti-self-reply: partner must NEVER play NAME_A1 or DUSIE_A1 on Turn 1
	if st.TurnCount <= 1 {
		for _, blocked := range spec.NoSelfAnswerOnTurn1 {
			if answerSlot == blocked {
				shouldAnswer = false
			}
		}
	}

	if shouldAnswer && answerSlot != "" && !st.UsedAnswers[answerSlot] {
		if ansClip, ok := cat.PartnerAnswers[answerSlot]; ok {
			parts = append(parts, map[string]interface{}{
				"audio_id": ansClip.AudioID,
				"text":     ansClip.Text,
				"speaker":  ansClip.Speaker,
				"role":     ansClip.Role,
			})
			st.UsedAnswers[answerSlot] = true
			if answerSlot == "ORIG" {
				st.CoveredTopics["origin"] = true
			}
		}
	}

	// 3. Part 3: Next Question or Transition to Phase 3 (Examiner Follow-Up)
	// Count covered required topics
	coveredRequired := 0
	for _, t := range spec.Topics {
		if t.Required && st.CoveredTopics[t.Key] {
			coveredRequired++
		}
	}

	// If enough topics are covered and spec has examiner follow-up -> transition to Phase 3
	if coveredRequired >= spec.MinTopicsForWrapUp && spec.HasExaminerFollowUp {
		// Partner says thank you for the talk (CLOSE_01)
		if closeClip, ok := cat.PartnerSpecial[spec.PartnerCloseSlot]; ok {
			parts = append(parts, map[string]interface{}{
				"audio_id": closeClip.AudioID,
				"text":     closeClip.Text,
				"speaker":  closeClip.Speaker,
				"role":     closeClip.Role,
			})
		}

		// Pick Examiner follow-up question
		followUpSlot := "FOLLOW_01"
		for _, rule := range spec.FollowUpSlots {
			if rule.RequiredTopicCheck != "" && !st.CoveredTopics[rule.RequiredTopicCheck] {
				followUpSlot = rule.SlotCode
				break
			} else if rule.RequiredTopicCheck == "" {
				followUpSlot = rule.SlotCode
			}
		}

		if followClip, ok := cat.ExaminerClips[followUpSlot]; ok {
			parts = append(parts, map[string]interface{}{
				"audio_id": followClip.AudioID,
				"text":     followClip.Text,
				"speaker":  followClip.Speaker,
				"role":     followClip.Role,
			})
		}

		respJSON, _ := json.Marshal(map[string]interface{}{
			"exam_ended": false,
			"parts":      parts,
		})
		return RouteResult{
			Fallback:  false,
			Response:  string(respJSON),
			ExamEnded: false,
			Parts:     parts,
		}, nil
	}

	// Pick next uncovered topic in natural conversational order
	var nextTopic *TopicSpec
	for i := range spec.Topics {
		t := &spec.Topics[i]
		if !st.CoveredTopics[t.Key] {
			nextTopic = t
			break
		}
	}

	if nextTopic != nil {
		if qClip, ok := cat.PartnerQuestions[nextTopic.SlotCode]; ok {
			parts = append(parts, map[string]interface{}{
				"audio_id": qClip.AudioID,
				"text":     qClip.Text,
				"speaker":  qClip.Speaker,
				"role":     qClip.Role,
			})
			st.CoveredTopics[nextTopic.Key] = true
			st.LastAskedTopic = nextTopic.SlotCode
		}
	}

	// Safety check: ensure at least one question is present in the turn during Phase 2
	hasQuestion := false
	for _, p := range parts {
		if id, _ := p["audio_id"].(string); strings.Contains(id, "_Q") {
			hasQuestion = true
			break
		}
	}
	if !hasQuestion && cat.PartnerSpecial != nil {
		// If somehow no question was added, play the silence recovery / back question
		if backQ, ok := cat.PartnerQuestions["BACK"]; ok {
			parts = append(parts, map[string]interface{}{
				"audio_id": backQ.AudioID,
				"text":     backQ.Text,
				"speaker":  backQ.Speaker,
				"role":     backQ.Role,
			})
		}
	}

	// Never end exam in composeNormalTurn unless Phase 3 follow-up is already complete!
	respJSON, _ := json.Marshal(map[string]interface{}{
		"exam_ended": false,
		"parts":      parts,
	})
	return RouteResult{
		Fallback:  false,
		Response:  string(respJSON),
		ExamEnded: false,
		Parts:     parts,
	}, nil
}

func formatRecentHistory(history []Message, max int) string {
	var conv []Message
	for _, m := range history {
		if m.Role != "system" {
			conv = append(conv, m)
		}
	}
	if len(conv) == 0 {
		return "(start of conversation)"
	}
	start := 0
	if len(conv) > max {
		start = len(conv) - max
	}
	var lines []string
	for _, m := range conv[start:] {
		lines = append(lines, fmt.Sprintf("%s: %s", m.Role, m.Content))
	}
	return strings.Join(lines, "\n")
}
