package handlers

import (
	"testing"
)

func TestParseInit(t *testing.T) {
	raw := []byte(`{
		"voice_mapping": {"Examiner": "voice1", "Assistant": "voice2"},
		"history": [{"role": "system", "content": "drop me"}, {"role": "user", "content": "keep me"}]
	}`)

	s, err := parseInit(raw, "session-1", 100)
	if err != nil {
		t.Fatalf("parseInit failed: %v", err)
	}

	if s.DefaultVoiceID != "voice1" && s.DefaultVoiceID != "voice2" {
		t.Errorf("expected default voice to be resolved from mapping, got %q", s.DefaultVoiceID)
	}

	if len(s.Messages) != 2 {
		t.Errorf("expected 2 messages (1 new system, 1 user), got %d", len(s.Messages))
	}
	if s.Messages[1].Content != "keep me" {
		t.Errorf("history was not preserved correctly")
	}

	v := s.voiceFor(map[string]interface{}{"role": "examiner"})
	if v != "voice1" {
		t.Errorf("expected voice1 for examiner, got %q", v)
	}
}
