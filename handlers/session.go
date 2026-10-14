package handlers

import (
	"encoding/json"
	"strings"

	"ai_exam_simulator/config"
	"ai_exam_simulator/services"
)

type InitData struct {
	SystemPrompt     string            `json:"system_prompt"`
	UserInstructions string            `json:"user_instructions"`
	History          []json.RawMessage `json:"history"`
	VoiceMapping     map[string]string `json:"voice_mapping"`
	VoiceID          string            `json:"voice_id"`
}

type Session struct {
	ID               string
	DefaultVoiceID   string
	VoiceMapping     map[string]string
	Messages         []services.Message
}

func parseInit(raw []byte, id string, maxHistory int) (*Session, error) {
	var data InitData
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, err
	}

	sysPrompt := data.SystemPrompt
	if sysPrompt == "" {
		sysPrompt = config.DefaultSystemPrompt
	}

	fullSystemPrompt := sysPrompt + "\nConstraints: " + data.UserInstructions + config.JSONInstruction

	msgs := []services.Message{
		{Role: "system", Content: fullSystemPrompt},
	}

	var hist []services.Message
	for _, rawMsg := range data.History {
		var m services.Message
		if err := json.Unmarshal(rawMsg, &m); err == nil && m.Role != "system" {
			hist = append(hist, m)
		}
	}

	if len(hist) > maxHistory {
		hist = hist[len(hist)-maxHistory:]
	}
	msgs = append(msgs, hist...)

	normMapping := make(map[string]string)
	for k, v := range data.VoiceMapping {
		normMapping[strings.ToLower(k)] = v
	}

	defVoice := data.VoiceID
	if defVoice == "" {
		for _, key := range []string{"default", "examiner", "assistant"} {
			if v, ok := normMapping[key]; ok {
				defVoice = v
				break
			}
		}
	}
	if defVoice == "" && len(normMapping) > 0 {
		for _, v := range normMapping {
			defVoice = v
			break // Go maps aren't ordered, but this matches Python fallback
		}
	}

	return &Session{
		ID:             id,
		DefaultVoiceID: defVoice,
		VoiceMapping:   normMapping,
		Messages:       msgs,
	}, nil
}

func (s *Session) voiceFor(part map[string]interface{}) string {
	role := ""
	if r, ok := part["role"].(string); ok {
		role = r
	} else if r, ok := part["speaker"].(string); ok {
		role = r
	}

	if role != "" {
		if v, ok := s.VoiceMapping[strings.ToLower(role)]; ok {
			return v
		}
	}
	return s.DefaultVoiceID
}
