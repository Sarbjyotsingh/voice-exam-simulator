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

func TestDynamicSpeakers(t *testing.T) {
	raw := []byte(`{
		"primary_speaker": "Anna",
		"secondary_speaker": "Herr Mueller",
		"voice_mapping": {
			"Anna": "voice_anna_123",
			"Herr Mueller": "voice_mueller_456"
		}
	}`)

	s, err := parseInit(raw, "session-dynamic", 50)
	if err != nil {
		t.Fatalf("parseInit failed: %v", err)
	}

	// Default voice should be Anna (the primary speaker)
	if s.DefaultVoiceID != "voice_anna_123" {
		t.Errorf("expected default voice to be voice_anna_123, got %q", s.DefaultVoiceID)
	}

	// Speaker-based lookup for Anna
	vAnna := s.voiceFor(map[string]interface{}{"speaker": "Anna"})
	if vAnna != "voice_anna_123" {
		t.Errorf("expected voice_anna_123 for Anna, got %q", vAnna)
	}

	// Role-based lookup for Participant -> should dynamically map to Anna
	vPart := s.voiceFor(map[string]interface{}{"role": "Participant"})
	if vPart != "voice_anna_123" {
		t.Errorf("expected Participant to dynamically map to Anna's voice, got %q", vPart)
	}

	// Role-based lookup for Examiner -> should dynamically map to Herr Mueller
	vExam := s.voiceFor(map[string]interface{}{"role": "Examiner"})
	if vExam != "voice_mueller_456" {
		t.Errorf("expected Examiner to dynamically map to Herr Mueller's voice, got %q", vExam)
	}
}

