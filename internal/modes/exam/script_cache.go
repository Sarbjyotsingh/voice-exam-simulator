package exam

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"

	"voice-exam-simulator/internal/contracts"
	"voice-exam-simulator/services"
)

// CachedScriptEntry holds the parsed ScriptCatalogue alongside script.json metadata (exam_hint, voice_profiles, raw dialogues JSON).
type CachedScriptEntry struct {
	ExamID           string
	TopicTitle       string
	ExamHint         string
	PrimarySpeaker   string
	SecondarySpeaker string
	VoiceProfiles    map[string]string
	DialoguesJSON    string
	Catalogue        *services.ScriptCatalogue
}

// ScriptCache loads and caches script.json by (provider, level, exam_id, teil) from local disk or S3.
type ScriptCache struct {
	baseDir   string
	s3BaseURL string
	client    *http.Client
	log       *slog.Logger

	mu      sync.RWMutex
	entries map[string]*CachedScriptEntry
}

// NewScriptCache creates a ScriptCache.
func NewScriptCache(baseDir, s3BaseURL string, client *http.Client, log *slog.Logger) *ScriptCache {
	if log == nil {
		log = slog.Default()
	}
	if s3BaseURL == "" {
		s3BaseURL = "https://lingua-stories-exams.s3.eu-central-1.amazonaws.com"
	}
	return &ScriptCache{
		baseDir:   baseDir,
		s3BaseURL: s3BaseURL,
		client:    client,
		log:       log.With("component", "script_cache"),
		entries:   make(map[string]*CachedScriptEntry),
	}
}

func cacheKey(provider contracts.ExamProvider, level contracts.CEFRLevel, examID string, teil int) string {
	folder := contracts.ExamFolderFromID(level, examID)
	return fmt.Sprintf("%s:%s:%s:%d",
		contracts.NormalizeExamProvider(string(provider)),
		contracts.NormalizeCEFRLevel(string(level)),
		folder,
		teil,
	)
}

// Get retrieves or loads a CachedScriptEntry for (provider, level, examID, teil).
func (c *ScriptCache) Get(ctx context.Context, provider contracts.ExamProvider, level contracts.CEFRLevel, examID string, teil int) (*CachedScriptEntry, error) {
	if teil <= 0 {
		teil = 1
	}
	key := cacheKey(provider, level, examID, teil)

	c.mu.RLock()
	if entry, ok := c.entries[key]; ok {
		c.mu.RUnlock()
		return entry, nil
	}
	c.mu.RUnlock()

	rawBytes, err := c.readScriptBytes(ctx, provider, level, examID, teil)
	if err != nil {
		return nil, err
	}

	entry, err := parseScriptFileBytes(rawBytes)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.entries[key] = entry
	c.mu.Unlock()
	return entry, nil
}

func (c *ScriptCache) readScriptBytes(ctx context.Context, provider contracts.ExamProvider, level contracts.CEFRLevel, examID string, teil int) ([]byte, error) {
	candidates := contracts.LocalScriptCandidates(c.baseDir, provider, level, examID, teil)
	for _, p := range candidates {
		if b, err := os.ReadFile(p); err == nil && len(b) > 0 {
			return b, nil
		}
	}

	if c.client != nil && c.s3BaseURL != "" {
		url := contracts.ContentScriptURL(c.s3BaseURL, provider, level, examID, teil)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err == nil {
			resp, err := c.client.Do(req)
			if err == nil {
				defer resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					if b, err := io.ReadAll(resp.Body); err == nil && len(b) > 0 {
						return b, nil
					}
				}
			}
		}
	}
	return nil, fmt.Errorf("script.json not found for provider=%s level=%s exam=%s teil=%d", provider, level, examID, teil)
}

func parseScriptFileBytes(rawBytes []byte) (*CachedScriptEntry, error) {
	var rawDoc struct {
		ExamID        string            `json:"exam_id"`
		TopicTitle    string            `json:"topic_title"`
		ExamHint      string            `json:"exam_hint"`
		VoiceProfiles map[string]string `json:"voice_profiles"`
		Dialogues     []struct {
			Speaker     string `json:"speaker"`
			Role        string `json:"role"`
			Topic       string `json:"topic"`
			Interaction map[string]struct {
				AudioID string `json:"audio_id"`
				Text    string `json:"text"`
			} `json:"interaction"`
		} `json:"dialogues"`
	}
	if err := json.Unmarshal(rawBytes, &rawDoc); err != nil {
		return nil, fmt.Errorf("parse script.json: %w", err)
	}

	var primary, secondary string
	for _, d := range rawDoc.Dialogues {
		if strings.EqualFold(d.Role, contracts.SpeakerRoleExaminer) && secondary == "" && d.Speaker != "" {
			secondary = d.Speaker
		} else if strings.EqualFold(d.Role, contracts.SpeakerRoleParticipant) && primary == "" && d.Speaker != "" {
			primary = d.Speaker
		}
	}

	dialoguesBytes, err := json.Marshal(rawDoc.Dialogues)
	if err != nil {
		return nil, err
	}
	cat, err := services.ParseScriptCatalogue(string(dialoguesBytes), primary, secondary)
	if err != nil {
		return nil, err
	}

	return &CachedScriptEntry{
		ExamID:           rawDoc.ExamID,
		TopicTitle:       rawDoc.TopicTitle,
		ExamHint:         rawDoc.ExamHint,
		PrimarySpeaker:   primary,
		SecondarySpeaker: secondary,
		VoiceProfiles:    rawDoc.VoiceProfiles,
		DialoguesJSON:    string(dialoguesBytes),
		Catalogue:        cat,
	}, nil
}
