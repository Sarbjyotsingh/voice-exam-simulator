package services

import (
	"fmt"
	"sync"
)

// TopicSpec defines one topic / planning item within a Teil.
type TopicSpec struct {
	Key         string `json:"key"`         // e.g. "origin" or "work"
	SlotCode    string `json:"slot_code"`   // e.g. "ORIG", "WORK" (maps to DE_<SPEAKER>_<SLOT>_Q1/_A1)
	Description string `json:"description"` // descriptive text given to gpt-6-luna
	Required    bool   `json:"required"`    // whether this topic counts toward wrap-up completion
}

// ReactionSpec defines one sentiment expression clip.
type ReactionSpec struct {
	Code        string `json:"code"`        // e.g. "EXP_01", "EXP_02"
	Description string `json:"description"` // context description given to gpt-6-luna
}

// FollowUpRule defines which Examiner follow-up question to play in Phase 3.
type FollowUpRule struct {
	SlotCode           string `json:"slot_code"`            // e.g. "FOLLOW_01", "FOLLOW_02"
	RequiredTopicCheck string `json:"required_topic_check"` // if specified and NOT covered, select this slot
}

// InterventionSpec defines the clip slot codes for examiner interventions and recoveries.
type InterventionSpec struct {
	OffTopicSlot    string `json:"off_topic_slot"`   // e.g. "INT_TOPIC"
	PolitenessSlot  string `json:"politeness_slot"`  // e.g. "INT_POLITE"
	DialogueSlot    string `json:"dialogue_slot"`    // e.g. "INT_DIALOG"
	ClarifyExpSlot  string `json:"clarify_exp_slot"` // e.g. "EXP_05" ("Ach so, ich verstehe.")
	ClarifyQuestion string `json:"clarify_question"` // e.g. "CLAR_Q1"
	SilenceQuestion string `json:"silence_question"` // e.g. "SIL_Q1"
	ThanksExpSlot   string `json:"thanks_exp_slot"`  // e.g. "EXP_15" ("Gerne! Kein Problem.")
}

// TeilDecisionSpec declaratively configures how OpenAI Decisions API routes a specific
// (ExamLevel, TeilNumber) — e.g. ("B1", 1), ("B1", 2), ("B1", 3).
type TeilDecisionSpec struct {
	Level               string  `json:"level"`                // e.g. "B1"
	TeilNumber          int     `json:"teil_number"`          // 1, 2, 3
	TeilName            string  `json:"teil_name"`            // "Teil 1: Kontaktaufnahme"
	ContextSummary      string  `json:"context_summary"`      // Short intro for prompt
	FallbackInstruction string  `json:"fallback_instruction"` // custom instruction for needs_custom_fallback predicate
	FallbackThreshold   float64 `json:"fallback_threshold"`   // probability threshold to trigger fallback (default 0.70)

	Topics               []TopicSpec  `json:"topics"`
	InitialCoveredTopics []string     `json:"initial_covered_topics"` // topics covered in auto_play_sequence before Turn 1
	NoSelfAnswerOnTurn1  []string     `json:"no_self_answer_on_turn1"`  // slots partner shouldn't self-answer in Turn 1 (e.g. "name", "du_sie")

	Reactions         []ReactionSpec `json:"reactions"`
	NoRepeatReactions bool           `json:"no_repeat_reactions"` // true -> don't repeat any EXP_* in the same exam

	MinTopicsForWrapUp  int              `json:"min_topics_for_wrap_up"` // e.g. 6 for B1 Teil 1
	HasExaminerFollowUp bool             `json:"has_examiner_follow_up"` // true for B1 Teil 1 (Phase 3 FOLLOW before OUTRO)
	FollowUpSlots       []FollowUpRule   `json:"follow_up_slots"`
	PartnerCloseSlot    string           `json:"partner_close_slot"` // e.g. "CLOSE_01"
	ExaminerOutroSlot   string           `json:"examiner_outro_slot"` // e.g. "OUTRO_01"
	Interventions       InterventionSpec `json:"interventions"`
}

// SpecRegistry is a thread-safe registry of TeilDecisionSpecs keyed by (Level, TeilNumber).
type SpecRegistry struct {
	mu    sync.RWMutex
	specs map[string]*TeilDecisionSpec
}

func NewSpecRegistry() *SpecRegistry {
	r := &SpecRegistry{
		specs: make(map[string]*TeilDecisionSpec),
	}
	// Register built-in B1 Teil 1, Teil 2 & Teil 3 specs
	r.Register(DefaultB1Teil1Spec())
	r.Register(DefaultB1Teil2Spec())
	r.Register(DefaultB1Teil3Spec())
	return r
}

func specKey(level string, teil int) string {
	return fmt.Sprintf("%s_%d", level, teil)
}

func (r *SpecRegistry) Register(spec *TeilDecisionSpec) {
	if spec == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.specs[specKey(spec.Level, spec.TeilNumber)] = spec
}

func (r *SpecRegistry) Get(level string, teil int) *TeilDecisionSpec {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.specs[specKey(level, teil)]
}

// DefaultB1Teil1Spec returns the official Telc B1 Teil 1 specification.
func DefaultB1Teil1Spec() *TeilDecisionSpec {
	return &TeilDecisionSpec{
		Level:          "B1",
		TeilNumber:     1,
		TeilName:       "Teil 1: Kontaktaufnahme",
		ContextSummary: "Telc B1 German speaking exam, Teil 1 (getting to know each other).",
		FallbackInstruction: "Did the learner ask a specific, unusual question to the partner that CANNOT be answered by the partner's standard biography clips (name, age, origin, residence, family, work/profession, languages, learning German, hobbies, or du/Sie)? Answer true ONLY if pre-recorded biography clips cannot reasonably answer what the learner asked.",
		FallbackThreshold:   0.70,

		Topics: []TopicSpec{
			{Key: "name", SlotCode: "NAME", Description: "first name, surname, or family name", Required: true},
			{Key: "origin", SlotCode: "ORIG", Description: "country of origin, hometown, or city", Required: true},
			{Key: "residence", SlotCode: "RES", Description: "where and how they live in Germany (city, flat, house, WG)", Required: true},
			{Key: "family", SlotCode: "FAM", Description: "marital status, children, spouse, or siblings", Required: true},
			{Key: "work", SlotCode: "WORK", Description: "profession, job, study, or Ausbildung", Required: true},
			{Key: "languages", SlotCode: "LANG", Description: "native mother tongue and other languages spoken", Required: true},
			{Key: "german_learning", SlotCode: "GER", Description: "where, how long, and why they learned German", Required: true},
			{Key: "hobbies", SlotCode: "HOB", Description: "free time, sports, cooking, music, or leisure activities", Required: true},
			{Key: "age", SlotCode: "AGE", Description: "age in years", Required: false},
		},

		InitialCoveredTopics: []string{"name"},
		NoSelfAnswerOnTurn1:  []string{"NAME", "DUSIE"},

		Reactions: []ReactionSpec{
			{Code: "EXP_01", Description: "curious: learner shared origin/languages/profession (Ah, interessant!)"},
			{Code: "EXP_02", Description: "warm: family, hometown, nice home (Oh, schön!)"},
			{Code: "EXP_03", Description: "upbeat: agreed to 'du', positive hobby/job/achievement (Super!)"},
			{Code: "EXP_04", Description: "enthusiastic: enjoyable hobby, interesting job/study (Das klingt ja toll!)"},
			{Code: "EXP_05", Description: "empathetic: single, alone, unemployed, family far away (Ach so, ich verstehe.)"},
			{Code: "EXP_06", Description: "surprised: many languages, unusual job (Oh, wirklich? Das ist ja spannend!)"},
			{Code: "EXP_07", Description: "casual ack: name/spelling/age answered (Alles klar!)"},
			{Code: "EXP_08", Description: "relatable: German course, grammar hard, missing home (Ja, das kenne ich gut.)"},
			{Code: "EXP_09", Description: "casual: sport, music, films, travel (Cool!)"},
			{Code: "EXP_10", Description: "neutral transition after any plain fact (Ah, okay!)"},
			{Code: "EXP_11", Description: "polite pleasure: right after the learner introduces their name (Das freut mich!)"},
			{Code: "EXP_12", Description: "shared experience: learner mentions a shared trait (Oh, ich auch!)"},
			{Code: "EXP_13", Description: "agreeing with a general observation (Ja, das stimmt.)"},
			{Code: "EXP_14", Description: "supportive: looking for job/apartment/exam (Ich wünsche dir viel Erfolg dabei!)"},
			{Code: "EXP_15", Description: "you're welcome: learner said thanks (Gerne! Kein Problem.)"},
			{Code: "EXP_16", Description: "sympathy: learner mentions something sad/difficult (Oh, das tut mir leid.)"},
			{Code: "EXP_17", Description: "encouraging: clear multi-part answer (Sehr gut!)"},
			{Code: "EXP_18", Description: "reciprocation: learner wishes luck (Danke, gleichfalls!)"},
		},
		NoRepeatReactions: true,

		MinTopicsForWrapUp:  8,
		HasExaminerFollowUp: true,
		FollowUpSlots: []FollowUpRule{
			{SlotCode: "FOLLOW_01", RequiredTopicCheck: ""}, // Default examiner follow-up question
		},
		PartnerCloseSlot:  "CLOSE_01",
		ExaminerOutroSlot: "OUTRO_01",

		Interventions: InterventionSpec{
			OffTopicSlot:    "INT_TOPIC",
			PolitenessSlot:  "INT_POLITE",
			DialogueSlot:    "INT_DIALOG",
			ClarifyExpSlot:  "EXP_05",
			ClarifyQuestion: "CLAR_Q1",
			SilenceQuestion: "SIL_Q1",
			ThanksExpSlot:   "EXP_15",
		},
	}
}

// DefaultB1Teil2Spec returns the official Telc B1 Teil 2 (Gespräch über ein Thema) specification.
func DefaultB1Teil2Spec() *TeilDecisionSpec {
	return &TeilDecisionSpec{
		Level:          "B1",
		TeilNumber:     2,
		TeilName:       "Teil 2: Gespräch über ein Thema",
		ContextSummary: "Telc B1 German speaking exam, Teil 2 (presenting a stimulus text opinion and discussing the topic with a partner).",
		FallbackInstruction: "Did the learner ask a specific, unusual question to the partner that CANNOT be answered by the partner's standard topic clips (stimulus text presentation, personal opinion on the topic, daily habits/experiences in Germany, comparison with home country, or pros/cons/compromises)? Answer true ONLY if pre-recorded topic clips cannot reasonably answer what the learner asked.",
		FallbackThreshold:   0.70,

		Topics: []TopicSpec{
			{Key: "presentation", SlotCode: "PRES", Description: "summarizing the stimulus text / person's quote (name, age, profession, opinion, reason)", Required: true},
			{Key: "opinion", SlotCode: "OPIN", Description: "personal opinion and stance on the topic", Required: true},
			{Key: "habits_germany", SlotCode: "HAB", Description: "personal habits and everyday experiences here in Germany", Required: true},
			{Key: "home_country", SlotCode: "HOME", Description: "comparison with how the topic is in their home country (Heimatland)", Required: true},
			{Key: "pros_cons", SlotCode: "PRO", Description: "advantages, disadvantages, prices, environment, or practical solutions/compromises", Required: true},
		},

		InitialCoveredTopics: []string{"presentation"},
		NoSelfAnswerOnTurn1:  []string{"PRES", "T2OPEN"},

		Reactions: []ReactionSpec{
			{Code: "EXP_21", Description: "strong agreement: clear argument or sensible opinion (Da hast du absolut recht.)"},
			{Code: "EXP_22", Description: "enthusiastic agreement: shared viewpoint (Ganz genau, das sehe ich genauso.)"},
			{Code: "EXP_23", Description: "polite formal agreement with opinion (Ich stimme dir vollkommen zu.)"},
			{Code: "EXP_24", Description: "natural agreement with everyday fact (Ja, das stimmt auf jeden Fall.)"},
			{Code: "EXP_25", Description: "validating an important argument like health/money/time/children (Das ist ein sehr wichtiges Argument.)"},
			{Code: "EXP_26", Description: "shared habit or similar experience in Germany/home country (Ja, das ist bei mir ganz ähnlich.)"},
			{Code: "EXP_27", Description: "polite disagreement: opposite preference (Da bin ich etwas anderer Meinung.)"},
			{Code: "EXP_28", Description: "gentle disagreement or nuance (Das sehe ich nicht ganz so.)"},
			{Code: "EXP_29", Description: "personal contrast in habits (Ehrlich gesagt ist das bei mir ein bisschen anders.)"},
			{Code: "EXP_30", Description: "balanced counter-perspective (Das stimmt zwar, aber es gibt auch eine andere Seite.)"},
			{Code: "EXP_31", Description: "weighing pros and cons (Einerseits hast du recht, aber andererseits kommt es auf die Situation an.)"},
			{Code: "EXP_32", Description: "curious engagement: home country comparison or interesting anecdote (Ach wirklich? Das ist ja interessant!)"},
			{Code: "EXP_33", Description: "appreciative engagement: good point raised (Guter Punkt! Darüber habe ich auch schon nachgedacht.)"},
			{Code: "EXP_34", Description: "empathetic validation: stressful/expensive/convenient experience (Das kann ich mir sehr gut vorstellen.)"},
			{Code: "EXP_35", Description: "understanding why someone prefers an option (Ja, das kann ich gut nachvollziehen.)"},
			{Code: "EXP_36", Description: "acknowledging stimulus text summary on Turn 1 (Danke für die Zusammenfassung! Das ist eine interessante Meinung.)"},
			{Code: "EXP_37", Description: "neutral clarification acknowledgment (Ach so, alles klar, ich verstehe.)"},
			{Code: "EXP_38", Description: "warm response to 'Danke' mid-discussion (Gerne! Das macht wirklich Spaß, mit dir darüber zu diskutieren.)"},
		},
		NoRepeatReactions: true,

		MinTopicsForWrapUp:  5,
		HasExaminerFollowUp: true,
		FollowUpSlots: []FollowUpRule{
			{SlotCode: "T2_FOLLOW_01", RequiredTopicCheck: ""}, // Default Teil 2 examiner follow-up question
		},
		PartnerCloseSlot:  "T2CLOSE_01",
		ExaminerOutroSlot: "T2_OUTRO_01",

		Interventions: InterventionSpec{
			OffTopicSlot:    "T2_INT_TOPIC",
			PolitenessSlot:  "T2_INT_POLITE",
			DialogueSlot:    "T2_INT_DIALOG",
			ClarifyExpSlot:  "EXP_37",
			ClarifyQuestion: "T2CLAR_Q1",
			SilenceQuestion: "T2SIL_Q1",
			ThanksExpSlot:   "EXP_38",
		},
	}
}

// DefaultB1Teil3Spec returns the official Telc B1 Teil 3 (Gemeinsam etwas planen) specification.
func DefaultB1Teil3Spec() *TeilDecisionSpec {
	return &TeilDecisionSpec{
		Level:          "B1",
		TeilNumber:     3,
		TeilName:       "Teil 3: Gemeinsam etwas planen",
		ContextSummary: "Telc B1 German speaking exam, Teil 3 (collaborative planning task: agreeing on time, location/materials, food/items, task division/transport, and final plan summary).",
		FallbackInstruction: "Did the learner ask a specific, unusual question to the partner that CANNOT be answered by the partner's standard planning clips (date/time compromise, venue/materials/rain backup, food/drinks/gifts, task division/transport, or final summary)? Answer true ONLY if pre-recorded planning clips cannot reasonably answer what the learner asked.",
		FallbackThreshold:   0.70,

		Topics: []TopicSpec{
			{Key: "time", SlotCode: "TIME", Description: "when to meet or hold the event (day, date, time, weekend vs. weekday)", Required: true},
			{Key: "location", SlotCode: "LOC", Description: "where to meet/celebrate, weather rain backup, or where to get tools/materials", Required: true},
			{Key: "food_items", SlotCode: "FOOD", Description: "what food, drinks, gifts, or equipment to bring or buy", Required: true},
			{Key: "tasks_transport", SlotCode: "TASK", Description: "how to divide tasks (who does what) and how to travel/transport things", Required: true},
			{Key: "summary", SlotCode: "SUM", Description: "final summary of the agreed plan and checking if anything is missing", Required: true},
		},

		InitialCoveredTopics: []string{"time"},
		NoSelfAnswerOnTurn1:  []string{"T3OPEN"},

		Reactions: []ReactionSpec{
			{Code: "EXP_41", Description: "enthusiastic agreement with a great idea (Das ist eine super Idee!)"},
			{Code: "EXP_42", Description: "upbeat agreement on how to proceed (Klasse, so machen wir das!)"},
			{Code: "EXP_43", Description: "confirming a proposed time or date (Perfekt, das passt mir sehr gut.)"},
			{Code: "EXP_44", Description: "polite B1 agreement with a multi-step suggestion (Einverstanden, das klingt nach einem sehr guten Plan.)"},
			{Code: "EXP_45", Description: "energetic agreement with an activity or food proposal (Super Vorschlag, da bin ich sofort dabei!)"},
			{Code: "EXP_46", Description: "crisp confirmation sealing an agreement (Alles klar, abgemacht!)"},
			{Code: "EXP_47", Description: "polite doubt or hesitation (Hmm, da bin ich mir noch nicht ganz sicher.)"},
			{Code: "EXP_48", Description: "polite scheduling concern when time is tight (Das wird zeitlich vielleicht ein bisschen knapp.)"},
			{Code: "EXP_49", Description: "polite rejection of an inconvenient time (Ehrlich gesagt passt mir dieser Termin nicht so gut.)"},
			{Code: "EXP_50", Description: "practical weather caveat for outdoor plans (Die Idee ist gut, aber was machen wir, wenn das Wetter schlecht ist?)"},
			{Code: "EXP_51", Description: "constructive counter-proposal lead-in (Ich hätte da noch einen anderen Vorschlag.)"},
			{Code: "EXP_52", Description: "collaborative counter-suggestion to do something together (Wäre es nicht praktischer, wenn wir das zusammen erledigen?)"},
			{Code: "EXP_53", Description: "validating the learner's counter-proposal or objection (Ja, du hast recht, das ist wirklich viel praktischer so.)"},
			{Code: "EXP_54", Description: "gladly accepting a task assigned by the learner (Sehr gerne, darum kümmere ich mich!)"},
			{Code: "EXP_55", Description: "reciprocal task division between learner and partner (Abgemacht! Wenn du das übernimmst, kümmere ich mich um den Rest.)"},
			{Code: "EXP_56", Description: "upbeat transition to final plan summary (Super, wir haben jetzt fast alle Punkte geklärt!)"},
			{Code: "EXP_57", Description: "neutral clarification acknowledgement (Ach so, alles klar, jetzt habe ich dich verstanden.)"},
			{Code: "EXP_58", Description: "warm response to 'Danke' during planning (Gerne! Es macht richtig Spaß, das mit dir zusammen zu planen.)"},
		},
		NoRepeatReactions: true,

		MinTopicsForWrapUp:  5,
		HasExaminerFollowUp: false,
		FollowUpSlots:       nil,
		PartnerCloseSlot:    "T3CLOSE_01",
		ExaminerOutroSlot:   "T3_OUTRO_01",

		Interventions: InterventionSpec{
			OffTopicSlot:    "T3_INT_TOPIC",
			PolitenessSlot:  "T3_INT_POLITE",
			DialogueSlot:    "T3_INT_DIALOG",
			ClarifyExpSlot:  "EXP_57",
			ClarifyQuestion: "T3CLAR_Q1",
			SilenceQuestion: "T3SIL_Q1",
			ThanksExpSlot:   "EXP_58",
		},
	}
}
