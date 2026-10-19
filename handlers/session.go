package handlers

import (
	"encoding/json"
	"sort"
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
	PrimarySpeaker   string            `json:"primary_speaker"`
	SecondarySpeaker string            `json:"secondary_speaker"`
}

type Session struct {
	ID               string
	DefaultVoiceID   string
	VoiceMapping     map[string]string
	PrimarySpeaker   string
	SecondarySpeaker string
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

	primary := strings.ToLower(data.PrimarySpeaker)
	secondary := strings.ToLower(data.SecondarySpeaker)

	defVoice := data.VoiceID
	// 1. Explicit primary speaker from config
	if defVoice == "" && primary != "" {
		if v, ok := normMapping[primary]; ok {
			defVoice = v
		}
	}
	// 2. Generic partner/candidate role names
	if defVoice == "" {
		for _, key := range []string{"participant", "candidate", "partner", "primary", "assistant", "default"} {
			if v, ok := normMapping[key]; ok {
				defVoice = v
				break
			}
		}
	}
	// 3. Fallback: select any voice that is not the examiner/secondary speaker
	if defVoice == "" && len(normMapping) > 0 {
		var nonExaminerKeys []string
		var allKeys []string
		for k := range normMapping {
			allKeys = append(allKeys, k)
			if (secondary == "" || k != secondary) && !strings.Contains(k, "examiner") && !strings.Contains(k, "prüfer") {
				nonExaminerKeys = append(nonExaminerKeys, k)
			}
		}
		sort.Strings(nonExaminerKeys)
		sort.Strings(allKeys)
		if len(nonExaminerKeys) > 0 {
			defVoice = normMapping[nonExaminerKeys[0]]
		} else if len(allKeys) > 0 {
			defVoice = normMapping[allKeys[0]]
		}
	}

	return &Session{
		ID:               id,
		DefaultVoiceID:   defVoice,
		VoiceMapping:     normMapping,
		PrimarySpeaker:   primary,
		SecondarySpeaker: secondary,
		Messages:         msgs,
	}, nil
}

func (s *Session) voiceFor(part map[string]interface{}) string {
	// 1. Direct speaker match (e.g. "Leyla", "Anna", "Herr Wagner", or any configured speaker name)
	if speaker, ok := part["speaker"].(string); ok && speaker != "" {
		if v, ok := s.VoiceMapping[strings.ToLower(speaker)]; ok {
			return v
		}
	}

	// 2. Role-based matching with dynamic primary/secondary speaker routing
	if role, ok := part["role"].(string); ok && role != "" {
		roleKey := strings.ToLower(role)
		// Exact role match in mapping (e.g. if voice mapping directly keys by "participant" or "examiner")
		if v, ok := s.VoiceMapping[roleKey]; ok {
			return v
		}

		// Dynamically route candidate/participant roles to primary speaker
		if roleKey == "participant" || roleKey == "candidate" || roleKey == "partner" {
			if s.PrimarySpeaker != "" {
				if v, ok := s.VoiceMapping[s.PrimarySpeaker]; ok {
					return v
				}
			}
		} else if roleKey == "examiner" || roleKey == "moderator" {
			// Dynamically route examiner roles to secondary speaker
			if s.SecondarySpeaker != "" {
				if v, ok := s.VoiceMapping[s.SecondarySpeaker]; ok {
					return v
				}
			}
		}
	}

	// 3. Fallback to default session voice
	return s.DefaultVoiceID
}

