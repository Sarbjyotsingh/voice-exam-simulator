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
	// Register built-in B1 Teil 1 spec
	r.Register(DefaultB1Teil1Spec())
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
