package contracts

import "encoding/json"

// WSStatus defines all WebSocket server-to-client status codes.
type WSStatus string

const (
	WSStatusInitialized        WSStatus = "initialized"
	WSStatusGenerating         WSStatus = "generating_response"
	WSStatusEmptyTranscription WSStatus = "empty_transcription"
	WSStatusPlayAudioID        WSStatus = "play_audio_id"
	WSStatusResponseComplete   WSStatus = "response_complete"
	WSStatusError              WSStatus = "error"
)

// ChatRole defines standard LLM message roles.
type ChatRole string

const (
	RoleSystem    ChatRole = "system"
	RoleUser      ChatRole = "user"
	RoleAssistant ChatRole = "assistant"
)

// SpeakerRole defines exam/conversation persona roles in turn parts.
const (
	SpeakerRoleParticipant = "Participant"
	SpeakerRoleExaminer    = "Examiner"
	SpeakerRoleCandidate   = "Candidate"
)

// ChatMessage represents a single turn in conversation history.
type ChatMessage struct {
	Role     string   `json:"role"`
	Content  string   `json:"content"`
	AudioIDs []string `json:"audio_ids,omitempty"`
}

// TurnPart represents one modular spoken segment in an AI turn response.
type TurnPart struct {
	Text    string `json:"text,omitempty"`
	AudioID string `json:"audio_id,omitempty"`
	Speaker string `json:"speaker,omitempty"`
	Role    string `json:"role,omitempty"`
}

// ToMap converts TurnPart to map[string]interface{} for backwards-compatible wire serialization.
func (p TurnPart) ToMap() map[string]interface{} {
	m := make(map[string]interface{}, 4)
	if p.AudioID != "" {
		m["audio_id"] = p.AudioID
	}
	if p.Text != "" {
		m["text"] = p.Text
	}
	if p.Speaker != "" {
		m["speaker"] = p.Speaker
	}
	if p.Role != "" {
		m["role"] = p.Role
	}
	return m
}

// PartsToMaps converts a slice of TurnPart to []map[string]interface{}.
func PartsToMaps(parts []TurnPart) []map[string]interface{} {
	res := make([]map[string]interface{}, 0, len(parts))
	for _, p := range parts {
		res = append(res, p.ToMap())
	}
	return res
}

// TurnInitRequest supports both the new thin-client metadata payload and legacy mobile payloads.
type TurnInitRequest struct {
	// New thin-client metadata fields
	Mode         string `json:"mode"`
	Language     string `json:"language"`
	ExamProvider string `json:"exam_provider"`
	ScenarioID   string `json:"scenario_id"`

	// Shared identity fields
	UserID     string `json:"user_id"`
	ExamID     string `json:"exam_id"`
	ExamLevel  string `json:"exam_level"`
	TeilNumber int    `json:"teil_number"`
	AttemptID  string `json:"attempt_id"`

	// Conversation history
	History []json.RawMessage `json:"history"`

	// Legacy fields (still supported for 100% backwards compatibility)
	SystemPrompt     string            `json:"system_prompt"`
	UserInstructions string            `json:"user_instructions"`
	VoiceMapping     map[string]string `json:"voice_mapping"`
	VoiceID          string            `json:"voice_id"`
	PrimarySpeaker   string            `json:"primary_speaker"`
	SecondarySpeaker string            `json:"secondary_speaker"`
}

// TurnStatusEvent is sent over WebSocket for status transitions.
type TurnStatusEvent struct {
	Status      WSStatus `json:"status"`
	SessionID   string   `json:"session_id,omitempty"`
	Transcribed string   `json:"transcribed,omitempty"`
	AudioID     string   `json:"audio_id,omitempty"`
	Message     string   `json:"message,omitempty"`
	Error       string   `json:"error,omitempty"`
}

// TurnCompleteEvent is sent when a turn completes.
type TurnCompleteEvent struct {
	Status         WSStatus                 `json:"status"`
	AIText         string                   `json:"ai_text"`
	Parts          []map[string]interface{} `json:"parts"`
	ExamEnded      bool                     `json:"exam_ended"`
	AttemptID      string                   `json:"attempt_id"`
	TurnCostUSD    float64                  `json:"turn_cost_usd"`
	AttemptCostUSD float64                  `json:"attempt_cost_usd"`
}

// EvaluateRequest supports both thin-client server-assembled evaluation and legacy evaluation_prompt payloads.
type EvaluateRequest struct {
	// New thin-client fields
	Mode             string        `json:"mode"`
	Language         string        `json:"language"`
	ExamProvider     string        `json:"exam_provider"`
	ScenarioID       string        `json:"scenario_id"`
	PrimarySpeaker   string        `json:"primary_speaker"`
	SecondarySpeaker string        `json:"secondary_speaker"`
	ExamHint         string        `json:"exam_hint"`
	History          []ChatMessage `json:"history"`

	// Shared identity fields
	UserID     string `json:"user_id"`
	ExamID     string `json:"exam_id"`
	ExamLevel  string `json:"exam_level"`
	TeilNumber int    `json:"teil_number"`
	AttemptID  string `json:"attempt_id"`

	// Legacy fields
	EvaluationPrompt string `json:"evaluation_prompt"`
	Transcript       string `json:"transcript"`
}

// CostSummary reports evaluation and cumulative attempt cost on POST /evaluate.
type CostSummary struct {
	EvalCostUSD    float64 `json:"eval_cost_usd"`
	AttemptCostUSD float64 `json:"attempt_cost_usd"`
	TurnsCount     int     `json:"turns_count"`
}

// EvaluateResponse is returned by POST /evaluate.
type EvaluateResponse struct {
	Result      string       `json:"result"`
	CostSummary *CostSummary `json:"cost_summary,omitempty"`
}
