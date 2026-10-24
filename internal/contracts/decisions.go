package contracts

// DecisionQID defines canonical question identifiers for DecisionProvider.
type DecisionQID string

const (
	QNeedsCustomFallback  DecisionQID = "needs_custom_fallback"
	QAskedClarification   DecisionQID = "asked_clarification"
	QLearnerAskedPartner  DecisionQID = "learner_asked_partner"
	QOffTopic             DecisionQID = "off_topic"
	QRudeOrInappropriate  DecisionQID = "rude_or_inappropriate"
	QTopicAddressed       DecisionQID = "topic_addressed"
	QPartnerQuestionTopic DecisionQID = "partner_question_topic"
	QReactionClip         DecisionQID = "reaction_clip"
)

// Question types supported by DecisionProvider.
const (
	DecisionTypePredicate = "predicate"
	DecisionTypeChoice    = "choice"
	DecisionTypeScore     = "score"
)

// Special choice values for Decision questions.
const (
	ChoiceSame   = "same"
	ChoiceNone   = "none"
	ChoiceThanks = "thanks"
)

// Default confidence and probability thresholds for Decision routing.
const (
	ThresholdFallback        = 0.70
	ThresholdClarification   = 0.70
	ThresholdOffTopic        = 0.70
	ThresholdRude            = 0.70
	ThresholdLearnerAsked    = 0.50
	ThresholdTopicConfidence = 0.35
	ThresholdOriginTurn1Prob = 0.25
)

// DecisionChoice represents one selectable option in a choice question.
type DecisionChoice struct {
	Value       string `json:"value" yaml:"value"`
	Description string `json:"description" yaml:"description"`
}

// DecisionQuestion defines a single structured classification question.
type DecisionQuestion struct {
	Type         string           `json:"type" yaml:"type"`
	Name         string           `json:"name" yaml:"name"`
	Instructions string           `json:"instructions" yaml:"instructions"`
	Choices      []DecisionChoice `json:"choices,omitempty" yaml:"choices,omitempty"`
}

// ChoiceProbability holds the probability of one choice option.
type ChoiceProbability struct {
	Value       string  `json:"value"`
	Probability float64 `json:"probability"`
}

// DecisionAnswer represents the answer to one DecisionQuestion.
type DecisionAnswer struct {
	Name          string              `json:"name"`
	Type          string              `json:"type"`
	Probability   *float64            `json:"probability,omitempty"`
	Choice        string              `json:"choice,omitempty"`
	Confidence    *float64            `json:"confidence,omitempty"`
	Probabilities []ChoiceProbability `json:"probabilities,omitempty"`
	Score         *float64            `json:"score,omitempty"`
	Refusal       string              `json:"refusal,omitempty"`
}
