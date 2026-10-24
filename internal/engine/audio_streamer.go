package engine

import (
	"context"
	"log/slog"
	"strings"
	"sync"

	"voice-exam-simulator/internal/contracts"
	"voice-exam-simulator/internal/providers"
)

// FrameSender abstracts WebSocket JSON and binary frame sending.
type FrameSender interface {
	SendJSON(v interface{}) error
	SendBytes(b []byte) error
}

// AudioStreamer coordinates hybrid pre-recorded audio_id frame emission and concurrent dynamic TTS synthesis.
type AudioStreamer struct {
	tts providers.TTSProvider
	log *slog.Logger
}

// NewAudioStreamer creates an AudioStreamer.
func NewAudioStreamer(tts providers.TTSProvider, log *slog.Logger) *AudioStreamer {
	if log == nil {
		log = slog.Default()
	}
	return &AudioStreamer{
		tts: tts,
		log: log.With("component", "audio_streamer"),
	}
}

// StreamParts synthesizes any dynamic TTS parts concurrently and streams play_audio_id or binary MP3 frames in order.
func (a *AudioStreamer) StreamParts(
	ctx context.Context,
	s *SessionContext,
	parts []map[string]interface{},
	sender FrameSender,
) []byte {
	type ttsItem struct {
		Index int
		Bytes []byte
		Err   error
	}

	results := make(chan ttsItem, len(parts))
	var wg sync.WaitGroup

	for i, p := range parts {
		audioID, _ := p["audio_id"].(string)
		if audioID == "" {
			audioID, _ = p["audioId"].(string)
		}
		if audioID != "" {
			continue // Pre-recorded clip exists; skip dynamic TTS
		}
		if text, ok := p["text"].(string); ok && text != "" && a.tts != nil {
			wg.Add(1)
			voiceID := ResolveVoiceForPart(s, p)
			go func(idx int, t, vID string) {
				defer wg.Done()
				b, _, err := a.tts.Synthesize(ctx, providers.TTSRequest{
					Text:    t,
					VoiceID: vID,
				})
				results <- ttsItem{Index: idx, Bytes: b, Err: err}
			}(i, text, voiceID)
		}
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	audioParts := make(map[int][]byte)
	for r := range results {
		if r.Err != nil {
			a.log.Error("TTS synthesis failed", "error", r.Err)
		} else {
			audioParts[r.Index] = r.Bytes
		}
	}

	var allGeneratedAudio []byte
	for i, p := range parts {
		audioID, _ := p["audio_id"].(string)
		if audioID == "" {
			audioID, _ = p["audioId"].(string)
		}
		if audioID != "" {
			_ = sender.SendJSON(map[string]string{
				"status":   string(contracts.WSStatusPlayAudioID),
				"audio_id": audioID,
			})
		} else if b, ok := audioParts[i]; ok && len(b) > 0 {
			_ = sender.SendBytes(b)
			allGeneratedAudio = append(allGeneratedAudio, b...)
		}
	}
	return allGeneratedAudio
}

// ResolveVoiceForPart resolves the TTS voice ID for a turn part based on speaker name, role, and session defaults.
func ResolveVoiceForPart(s *SessionContext, part map[string]interface{}) string {
	if s == nil {
		return ""
	}
	if speaker, ok := part["speaker"].(string); ok && speaker != "" {
		if v, ok := s.VoiceMapping[strings.ToLower(speaker)]; ok {
			return v
		}
	}
	if role, ok := part["role"].(string); ok && role != "" {
		roleKey := strings.ToLower(role)
		if v, ok := s.VoiceMapping[roleKey]; ok {
			return v
		}
		if roleKey == "participant" || roleKey == "candidate" || roleKey == "partner" {
			if s.PrimarySpeaker != "" {
				if v, ok := s.VoiceMapping[strings.ToLower(s.PrimarySpeaker)]; ok {
					return v
				}
			}
		} else if roleKey == "examiner" || roleKey == "moderator" {
			if s.SecondarySpeaker != "" {
				if v, ok := s.VoiceMapping[strings.ToLower(s.SecondarySpeaker)]; ok {
					return v
				}
			}
		}
	}
	return s.DefaultVoiceID
}
