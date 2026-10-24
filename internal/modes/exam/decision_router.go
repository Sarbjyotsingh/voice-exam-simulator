package exam

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"voice-engine/internal/contracts"
	"voice-engine/internal/providers"
	"voice-engine/services"
)

// DecisionClient is the subset of ProviderRegistry or DecisionProvider used by ExamDecisionRouter.
type DecisionClient interface {
	Decide(ctx context.Context, req providers.DecisionRequest) (*providers.DecisionResult, contracts.CallUsage, error)
}

// ExamDecisionRouter routes exam turns using spec.yaml + DecisionClient + deterministic clip composition.
type ExamDecisionRouter struct {
	decisions    DecisionClient
	specs        *services.SpecRegistry
	defaultHints map[string]string
	maxScores    map[string]int
	log          *slog.Logger
}

// NewExamDecisionRouter loads catalog/exams/de/telc_b1/teil_1..3/spec.yaml into SpecRegistry and returns an ExamDecisionRouter.
func NewExamDecisionRouter(decisions DecisionClient, catalogDir string, log *slog.Logger) *ExamDecisionRouter {
	if log == nil {
		log = slog.Default()
	}
	if catalogDir == "" {
		catalogDir = "catalog"
	}
	reg := services.NewSpecRegistry()
	hints := map[string]string{
		"B1_1": "Name\nWoher sie oder er kommt\nWie sie oder er wohnt (Wohnung, Haus, Garten ...)\nFamilie\nWo sie oder er Deutsch gelernt hat\nWas sie oder er macht (Schule, Studium, Beruf ...)\nSprachen (welche? wie lange? warum?)",
		"B1_2": "1. Meinung im Text wiedergeben (Name, Alter, Beruf, Meinung & Grund)\n2. Eigene Meinung zum Thema sagen (Zustimmen / Widersprechen)\n3. Eigene Erfahrungen & Gewohnheiten hier in Deutschland\n4. Vergleich: Wie ist die Situation in Ihrem Heimatland?\n5. Fragen an den Partner stellen & auf Argumente reagieren",
		"B1_3": "1. Wann? (Tag, Uhrzeit & Ausweichtermin vereinbaren)\n2. Wo? (Ort vorschlagen & Alternative bei schlechtem Wetter)\n3. Was mitbringen / besorgen? (Essen, Getränke, Material oder Geschenk)\n4. Aufgabenverteilung & Transport (Wer kümmert sich um was? Wie fahren wir?)\n5. Zusammenfassung (Am Ende kurz prüfen: Haben wir alles besprochen?)",
	}
	maxScores := map[string]int{
		"B1_1": 15,
		"B1_2": 30,
		"B1_3": 30,
	}

	pattern := filepath.Join(catalogDir, "exams", "*", "*", "teil_*", "spec.yaml")
	matches, _ := filepath.Glob(pattern)
	if len(matches) == 0 {
		for _, teil := range []int{1, 2, 3} {
			matches = append(matches, filepath.Join(catalogDir, "exams", "de", "telc_b1", fmt.Sprintf("teil_%d", teil), "spec.yaml"))
		}
	}
	for _, rel := range matches {
		if spec, hint, maxScore, err := loadTeilSpecYAML(rel); err == nil && spec != nil {
			reg.Register(spec)
			k := fmt.Sprintf("%s_%d", strings.ToUpper(spec.Level), spec.TeilNumber)
			if hint != "" {
				hints[k] = hint
			}
			if maxScore > 0 {
				maxScores[k] = maxScore
			}
		}
	}

	return &ExamDecisionRouter{
		decisions:    decisions,
		specs:        reg,
		defaultHints: hints,
		maxScores:    maxScores,
		log:          log.With("component", "exam_decision_router"),
	}
}

// GetSpec returns the TeilDecisionSpec for (level, teilNumber).
func (r *ExamDecisionRouter) GetSpec(level string, teilNumber int) *services.TeilDecisionSpec {
	return r.specs.Get(level, teilNumber)
}

// DefaultHint returns the default exam hint for (level, teilNumber).
func (r *ExamDecisionRouter) DefaultHint(level string, teilNumber int) string {
	return r.defaultHints[fmt.Sprintf("%s_%d", strings.ToUpper(level), teilNumber)]
}

// MaxScore returns the official max score for (level, teilNumber).
func (r *ExamDecisionRouter) MaxScore(level string, teilNumber int) int {
	if s, ok := r.maxScores[fmt.Sprintf("%s_%d", strings.ToUpper(level), teilNumber)]; ok && s > 0 {
		return s
	}
	if teilNumber == 1 {
		return 15
	}
	return 30
}

// RouteTurn implements handlers.TurnRouter and routes a learner's turn via DecisionClient.
func (r *ExamDecisionRouter) RouteTurn(
	ctx context.Context,
	level string,
	teilNumber int,
	cat *services.ScriptCatalogue,
	history []services.Message,
	userUtterance string,
) (services.RouteResult, error) {
	if r == nil || r.decisions == nil || cat == nil {
		return services.RouteResult{Fallback: true}, nil
	}

	spec := r.specs.Get(string(contracts.NormalizeCEFRLevel(level)), teilNumber)
	if spec == nil {
		return services.RouteResult{Fallback: true}, nil
	}

	st := services.AnalyzeHistory(history, cat, spec)

	// Phase 3 -> Phase 4 Examiner Outro after learner answers Examiner's follow-up question
	if st.AwaitingFollowUpAnswer {
		outroClip, ok := cat.ExaminerClips[spec.ExaminerOutroSlot]
		if !ok {
			return services.RouteResult{Fallback: true}, nil
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
		return services.RouteResult{
			Fallback:  false,
			Response:  string(respJSON),
			ExamEnded: true,
			Parts:     parts,
		}, nil
	}

	questions := buildContractQuestions(spec, st)
	var coveredList []string
	for t := range st.CoveredTopics {
		coveredList = append(coveredList, t)
	}
	recentHistory := formatRecentMessages(history, 4)
	input := fmt.Sprintf("%s Partner: %s. Topics already covered: %s.\nRecent conversation:\n%s\nLearner's latest message: %s",
		spec.ContextSummary, cat.PrimarySpeaker, strings.Join(coveredList, ", "), recentHistory, userUtterance)

	decRes, _, err := r.decisions.Decide(ctx, providers.DecisionRequest{
		Input:     input,
		Questions: questions,
	})
	if err != nil {
		r.log.Warn("decision provider error, falling back to chat llm", "error", err)
		return services.RouteResult{Fallback: true}, nil
	}

	ans := toServicesAnswerMap(decRes.AnswerMap())

	// 1. Check fallback predicate
	if fallbackAns, ok := ans[string(contracts.QNeedsCustomFallback)]; ok && fallbackAns.Probability != nil {
		if *fallbackAns.Probability >= spec.FallbackThreshold {
			return services.RouteResult{Fallback: true}, nil
		}
	}

	// 2. Check rude / inappropriate
	if rudeAns, ok := ans[string(contracts.QRudeOrInappropriate)]; ok && rudeAns.Probability != nil && *rudeAns.Probability >= contracts.ThresholdRude {
		return composeInterventionTurn(spec.Interventions.PolitenessSlot, cat, st)
	}

	// 3. Check off-topic
	if offAns, ok := ans[string(contracts.QOffTopic)]; ok && offAns.Probability != nil && *offAns.Probability >= contracts.ThresholdOffTopic {
		return composeInterventionTurn(spec.Interventions.OffTopicSlot, cat, st)
	}

	// 4. Check clarification requested
	if clarAns, ok := ans[string(contracts.QAskedClarification)]; ok && clarAns.Probability != nil && *clarAns.Probability >= contracts.ThresholdClarification {
		return composeClarificationTurn(spec, cat, st)
	}

	// 5. Standard Turn Composition
	return composeNormalExamTurn(spec, cat, st, ans)
}

func buildContractQuestions(spec *services.TeilDecisionSpec, st services.ConversationState) []contracts.DecisionQuestion {
	qs := []contracts.DecisionQuestion{
		{
			Type:         contracts.DecisionTypePredicate,
			Name:         string(contracts.QNeedsCustomFallback),
			Instructions: spec.FallbackInstruction,
		},
		{
			Type:         contracts.DecisionTypePredicate,
			Name:         string(contracts.QAskedClarification),
			Instructions: "Is the learner saying they did not understand, asking to repeat (e.g. 'Wie bitte?', 'Kannst du das wiederholen?', 'Ich habe nicht verstanden'), or asking what something means?",
		},
		{
			Type:         contracts.DecisionTypePredicate,
			Name:         string(contracts.QLearnerAskedPartner),
			Instructions: fmt.Sprintf("Does the learner's message ask the partner (%s) a question or hand the turn back (e.g. 'Und du?', 'Woher kommst du?', 'Hast du Kinder?')?", spec.TeilName),
		},
		{
			Type:         contracts.DecisionTypePredicate,
			Name:         string(contracts.QOffTopic),
			Instructions: "Is the learner's message completely unrelated to the exam topic (e.g. politics, news, sports commentary)?",
		},
		{
			Type:         contracts.DecisionTypePredicate,
			Name:         string(contracts.QRudeOrInappropriate),
			Instructions: "Is the learner's message rude, insulting, offensive, vulgar, or inappropriate for an exam?",
		},
	}

	var topicChoices []contracts.DecisionChoice
	partnerAskedChoices := []contracts.DecisionChoice{
		{Value: contracts.ChoiceSame, Description: "asked about the same topic with 'Und du?' / 'Und bei dir?'"},
		{Value: contracts.ChoiceNone, Description: "did not ask the partner any question"},
	}
	for _, t := range spec.Topics {
		topicChoices = append(topicChoices, contracts.DecisionChoice{Value: t.Key, Description: t.Description})
		partnerAskedChoices = append(partnerAskedChoices, contracts.DecisionChoice{Value: t.Key, Description: "asked the partner about " + t.Description})
	}
	topicChoices = append(topicChoices,
		contracts.DecisionChoice{Value: contracts.ChoiceThanks, Description: "thanking / polite closing remark (e.g. 'Danke')"},
		contracts.DecisionChoice{Value: contracts.ChoiceNone, Description: "none of the standard topics"},
	)

	qs = append(qs,
		contracts.DecisionQuestion{
			Type:         contracts.DecisionTypeChoice,
			Name:         string(contracts.QTopicAddressed),
			Instructions: "Which topic does the learner's own answer mainly talk about?",
			Choices:      topicChoices,
		},
		contracts.DecisionQuestion{
			Type:         contracts.DecisionTypeChoice,
			Name:         string(contracts.QPartnerQuestionTopic),
			Instructions: "If the learner asked the partner a question at the end of their message, which topic did they ask the partner about?",
			Choices:      partnerAskedChoices,
		},
	)

	var reactionChoices []contracts.DecisionChoice
	for _, rx := range spec.Reactions {
		if spec.NoRepeatReactions && st.UsedExpressions[rx.Code] {
			continue
		}
		reactionChoices = append(reactionChoices, contracts.DecisionChoice{Value: rx.Code, Description: rx.Description})
	}
	if len(reactionChoices) == 0 {
		for _, rx := range spec.Reactions {
			reactionChoices = append(reactionChoices, contracts.DecisionChoice{Value: rx.Code, Description: rx.Description})
		}
	}

	qs = append(qs, contracts.DecisionQuestion{
		Type:         contracts.DecisionTypeChoice,
		Name:         string(contracts.QReactionClip),
		Instructions: "Pick the single reaction clip matching the learner's sentiment and content.",
		Choices:      reactionChoices,
	})
	return qs
}

func toServicesAnswerMap(in map[string]contracts.DecisionAnswer) map[string]services.DecisionAnswer {
	out := make(map[string]services.DecisionAnswer, len(in))
	for k, v := range in {
		probs := make([]services.ChoiceProbability, 0, len(v.Probabilities))
		for _, p := range v.Probabilities {
			probs = append(probs, services.ChoiceProbability{Value: p.Value, Probability: p.Probability})
		}
		out[k] = services.DecisionAnswer{
			Name:          v.Name,
			Type:          v.Type,
			Probability:   v.Probability,
			Choice:        v.Choice,
			Confidence:    v.Confidence,
			Probabilities: probs,
			Score:         v.Score,
			Refusal:       v.Refusal,
		}
	}
	return out
}

func composeInterventionTurn(examinerSlot string, cat *services.ScriptCatalogue, st services.ConversationState) (services.RouteResult, error) {
	examinerClip, ok := cat.ExaminerClips[examinerSlot]
	if !ok {
		return services.RouteResult{Fallback: true}, nil
	}
	parts := []map[string]interface{}{
		{
			"audio_id": examinerClip.AudioID,
			"text":     examinerClip.Text,
			"speaker":  examinerClip.Speaker,
			"role":     examinerClip.Role,
		},
	}
	if st.LastAskedTopic != "" {
		if q, ok := cat.PartnerQuestions[st.LastAskedTopic]; ok {
			parts = append(parts, map[string]interface{}{
				"audio_id": q.AudioID,
				"text":     q.Text,
				"speaker":  q.Speaker,
				"role":     q.Role,
			})
		}
	}
	respJSON, _ := json.Marshal(map[string]interface{}{"exam_ended": false, "parts": parts})
	return services.RouteResult{Fallback: false, Response: string(respJSON), ExamEnded: false, Parts: parts}, nil
}

func composeClarificationTurn(spec *services.TeilDecisionSpec, cat *services.ScriptCatalogue, st services.ConversationState) (services.RouteResult, error) {
	var parts []map[string]interface{}
	if expClip, ok := cat.PartnerExp[spec.Interventions.ClarifyExpSlot]; ok {
		parts = append(parts, map[string]interface{}{
			"audio_id": expClip.AudioID,
			"text":     expClip.Text,
			"speaker":  expClip.Speaker,
			"role":     expClip.Role,
		})
	}
	reasked := false
	if st.LastAskedTopic != "" {
		if q2, ok := cat.PartnerQ2[st.LastAskedTopic]; ok {
			parts = append(parts, map[string]interface{}{
				"audio_id": q2.AudioID,
				"text":     q2.Text,
				"speaker":  q2.Speaker,
				"role":     q2.Role,
			})
			reasked = true
		} else if q1, ok := cat.PartnerQuestions[st.LastAskedTopic]; ok {
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
	respJSON, _ := json.Marshal(map[string]interface{}{"exam_ended": false, "parts": parts})
	return services.RouteResult{Fallback: false, Response: string(respJSON), ExamEnded: false, Parts: parts}, nil
}

func composeNormalExamTurn(
	spec *services.TeilDecisionSpec,
	cat *services.ScriptCatalogue,
	st services.ConversationState,
	ans map[string]services.DecisionAnswer,
) (services.RouteResult, error) {
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
	if ta, ok := ans[string(contracts.QTopicAddressed)]; ok && ta.Choice != "" {
		if ta.Confidence == nil || *ta.Confidence >= contracts.ThresholdTopicConfidence {
			topicAddressed = ta.Choice
		}
		if st.TurnCount <= 1 {
			for _, p := range ta.Probabilities {
				if p.Value == "origin" && p.Probability >= contracts.ThresholdOriginTurn1Prob {
					originMentionedOnTurn1 = true
					break
				}
			}
		}
	}

	learnerAsked := false
	if la, ok := ans[string(contracts.QLearnerAskedPartner)]; ok && la.Probability != nil && *la.Probability >= contracts.ThresholdLearnerAsked {
		learnerAsked = true
	}

	var addressedSlot string
	for _, t := range spec.Topics {
		if t.Key == topicAddressed {
			addressedSlot = t.SlotCode
			if t.Key == "origin" || spec.TeilNumber == 3 {
				st.CoveredTopics[t.Key] = true
			}
			break
		}
	}
	if st.TurnCount <= 1 && originMentionedOnTurn1 {
		st.CoveredTopics["origin"] = true
		if addressedSlot == "" || addressedSlot == contracts.CodeName {
			addressedSlot = contracts.CodeOrig
		}
	}

	answerSlot := addressedSlot
	if pq, ok := ans[string(contracts.QPartnerQuestionTopic)]; ok && pq.Choice != "" && pq.Choice != contracts.ChoiceNone && pq.Choice != contracts.ChoiceSame {
		if pq.Confidence == nil || *pq.Confidence >= contracts.ThresholdTopicConfidence {
			for _, t := range spec.Topics {
				if t.Key == pq.Choice {
					answerSlot = t.SlotCode
					break
				}
			}
		}
	}

	// Teil 2 Turn 1 Premature-Discussion Guard
	if st.TurnCount == 1 && spec.TeilNumber == 2 &&
		(topicAddressed == "opinion" || topicAddressed == "habits_germany" || topicAddressed == "home_country" || topicAddressed == "pros_cons") {
		if expClip, ok := cat.PartnerExp["EXP_27"]; ok {
			parts = append(parts, map[string]interface{}{
				"audio_id": expClip.AudioID,
				"text":     expClip.Text,
				"speaker":  expClip.Speaker,
				"role":     expClip.Role,
			})
		}
		if presQ2, ok := cat.PartnerQ2[contracts.CodePres]; ok {
			parts = append(parts, map[string]interface{}{
				"audio_id": presQ2.AudioID,
				"text":     presQ2.Text,
				"speaker":  presQ2.Speaker,
				"role":     presQ2.Role,
			})
			respJSON, _ := json.Marshal(map[string]interface{}{"exam_ended": false, "parts": parts})
			return services.RouteResult{Fallback: false, Response: string(respJSON), ExamEnded: false, Parts: parts}, nil
		}
	}

	reactionCode := ""
	if rx, ok := ans[string(contracts.QReactionClip)]; ok && rx.Choice != "" {
		reactionCode = rx.Choice
	}
	if st.TurnCount <= 1 && topicAddressed == "name" {
		reactionCode = "EXP_11"
	} else if spec.TeilNumber == 2 && topicAddressed == "presentation" && !st.UsedAnswers[contracts.CodeOpin] {
		reactionCode = "EXP_36"
	}
	if topicAddressed == contracts.ChoiceThanks {
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

	shouldAnswer := learnerAsked
	if st.TurnCount <= 1 && (topicAddressed == "origin" || originMentionedOnTurn1) {
		shouldAnswer = true
	}
	if spec.TeilNumber == 2 && topicAddressed == "presentation" && !st.UsedAnswers[contracts.CodeOpin] {
		shouldAnswer = true
		answerSlot = contracts.CodeOpin
	}
	if spec.TeilNumber == 3 {
		if st.TurnCount <= 1 && (answerSlot == "" || answerSlot == contracts.CodeTime) && !st.UsedAnswers[contracts.CodeTime] {
			answerSlot = contracts.CodeTime
			shouldAnswer = true
		} else if answerSlot != "" && answerSlot != contracts.CodeSum && !st.UsedAnswers[answerSlot] {
			shouldAnswer = true
		}
	}

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
			if answerSlot == contracts.CodeOrig {
				st.CoveredTopics["origin"] = true
			}
			if spec.TeilNumber == 3 {
				for _, t := range spec.Topics {
					if t.SlotCode == answerSlot {
						st.CoveredTopics[t.Key] = true
						break
					}
				}
			}
		}
	}

	coveredRequired := 0
	for _, t := range spec.Topics {
		if t.Required && st.CoveredTopics[t.Key] {
			coveredRequired++
		}
	}

	if coveredRequired >= spec.MinTopicsForWrapUp && spec.HasExaminerFollowUp {
		if closeClip, ok := cat.PartnerSpecial[spec.PartnerCloseSlot]; ok {
			parts = append(parts, map[string]interface{}{
				"audio_id": closeClip.AudioID,
				"text":     closeClip.Text,
				"speaker":  closeClip.Speaker,
				"role":     closeClip.Role,
			})
		}
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
		respJSON, _ := json.Marshal(map[string]interface{}{"exam_ended": false, "parts": parts})
		return services.RouteResult{Fallback: false, Response: string(respJSON), ExamEnded: false, Parts: parts}, nil
	} else if coveredRequired >= spec.MinTopicsForWrapUp && !spec.HasExaminerFollowUp {
		if closeClip, ok := cat.PartnerSpecial[spec.PartnerCloseSlot]; ok {
			parts = append(parts, map[string]interface{}{
				"audio_id": closeClip.AudioID,
				"text":     closeClip.Text,
				"speaker":  closeClip.Speaker,
				"role":     closeClip.Role,
			})
		}
		if outroClip, ok := cat.ExaminerClips[spec.ExaminerOutroSlot]; ok {
			parts = append(parts, map[string]interface{}{
				"audio_id": outroClip.AudioID,
				"text":     outroClip.Text,
				"speaker":  outroClip.Speaker,
				"role":     outroClip.Role,
			})
		}
		respJSON, _ := json.Marshal(map[string]interface{}{"exam_ended": true, "parts": parts})
		return services.RouteResult{Fallback: false, Response: string(respJSON), ExamEnded: true, Parts: parts}, nil
	}

	var nextTopic *services.TopicSpec
	for i := range spec.Topics {
		t := &spec.Topics[i]
		if !st.CoveredTopics[t.Key] {
			nextTopic = t
			break
		}
	}

	if nextTopic != nil {
		if nextTopic.SlotCode == contracts.CodeSum && !st.UsedAnswers[contracts.CodeSum] {
			if sumAns, ok := cat.PartnerAnswers[contracts.CodeSum]; ok {
				parts = append(parts, map[string]interface{}{
					"audio_id": sumAns.AudioID,
					"text":     sumAns.Text,
					"speaker":  sumAns.Speaker,
					"role":     sumAns.Role,
				})
				st.UsedAnswers[contracts.CodeSum] = true
			}
		}
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

	hasQuestion := false
	for _, p := range parts {
		if id, _ := p["audio_id"].(string); strings.Contains(id, "_Q") {
			hasQuestion = true
			break
		}
	}
	if !hasQuestion && cat.PartnerSpecial != nil {
		for _, backKey := range []string{contracts.CodeBack, contracts.CodeT2Back, contracts.CodeT3Back} {
			if backQ, ok := cat.PartnerQuestions[backKey]; ok {
				parts = append(parts, map[string]interface{}{
					"audio_id": backQ.AudioID,
					"text":     backQ.Text,
					"speaker":  backQ.Speaker,
					"role":     backQ.Role,
				})
				break
			}
		}
	}

	respJSON, _ := json.Marshal(map[string]interface{}{"exam_ended": false, "parts": parts})
	return services.RouteResult{Fallback: false, Response: string(respJSON), ExamEnded: false, Parts: parts}, nil
}

func formatRecentMessages(history []services.Message, max int) string {
	var conv []services.Message
	for _, m := range history {
		if m.Role != string(contracts.RoleSystem) {
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

// loadTeilSpecYAML parses a Teil spec.yaml file.
func loadTeilSpecYAML(path string) (*services.TeilDecisionSpec, string, int, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, "", 0, err
	}

	var teilNum, maxScore int
	var level, defaultHint string
	inHint := false
	var hintLines []string

	lines := strings.Split(string(b), "\n")
	for _, raw := range lines {
		trimmed := strings.TrimSpace(raw)
		indent := len(raw) - len(strings.TrimLeft(raw, " "))
		if inHint {
			if indent >= 2 {
				hintLines = append(hintLines, strings.TrimPrefix(raw, "  "))
				continue
			}
			inHint = false
		}
		if indent == 0 {
			if strings.HasPrefix(trimmed, "default_exam_hint:") {
				inHint = true
				continue
			}
			kv := strings.SplitN(trimmed, ":", 2)
			if len(kv) == 2 {
				k := strings.TrimSpace(kv[0])
				v := strings.Trim(strings.TrimSpace(kv[1]), `"'`)
				switch k {
				case "level":
					level = v
				case "teil_number":
					teilNum, _ = strconv.Atoi(v)
				case "max_score":
					maxScore, _ = strconv.Atoi(v)
				}
			}
		}
	}
	defaultHint = strings.TrimSpace(strings.Join(hintLines, "\n"))

	// Use the canonical spec for (level, teilNum) and allow YAML top-level overrides
	var spec *services.TeilDecisionSpec
	switch teilNum {
	case 1:
		spec = services.DefaultB1Teil1Spec()
	case 2:
		spec = services.DefaultB1Teil2Spec()
	case 3:
		spec = services.DefaultB1Teil3Spec()
	}
	if spec != nil && level != "" {
		spec.Level = level
	}
	return spec, defaultHint, maxScore, nil
}
