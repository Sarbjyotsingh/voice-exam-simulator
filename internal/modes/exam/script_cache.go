package exam

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"voice-engine/internal/contracts"
	"voice-engine/services"
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

// ScriptCache loads and caches script.json by (provider, level, exam_id, teil) from local catalog/ disk, with optional S3 fallback.
type ScriptCache struct {
	baseDir   string
	s3BaseURL string
	client    *http.Client
	log       *slog.Logger

	mu      sync.RWMutex
	entries map[string]*CachedScriptEntry
}

// NewScriptCache creates a ScriptCache and pre-warms all local B1 scenario scripts from catalog/ into memory at startup.
func NewScriptCache(baseDir, s3BaseURL string, client *http.Client, log *slog.Logger) *ScriptCache {
	if log == nil {
		log = slog.Default()
	}
	if s3BaseURL == "" {
		s3BaseURL = "https://lingua-stories-exams.s3.eu-central-1.amazonaws.com"
	}
	c := &ScriptCache{
		baseDir:   baseDir,
		s3BaseURL: s3BaseURL,
		client:    client,
		log:       log.With("component", "script_cache"),
		entries:   make(map[string]*CachedScriptEntry),
	}
	c.preloadCatalogScripts()
	return c
}

func (c *ScriptCache) preloadCatalogScripts() {
	var scenariosDirs []string
	roots := []string{c.baseDir, ".", "..", "../..", "../../.."}
	seen := make(map[string]bool)
	for _, r := range roots {
		if r == "" {
			continue
		}
		for _, candidate := range []string{
			filepath.Join(r, "catalog", "exams", "de", "telc_b1", "scenarios"),
			filepath.Join(r, "exams", "de", "telc_b1", "scenarios"),
		} {
			abs, err := filepath.Abs(candidate)
			if err != nil {
				abs = candidate
			}
			if seen[abs] {
				continue
			}
			if info, err := os.Stat(candidate); err == nil && info.IsDir() {
				seen[abs] = true
				scenariosDirs = append(scenariosDirs, candidate)
			}
		}
	}

	loaded := 0
	for _, scenariosDir := range scenariosDirs {
		sampleEntries, err := os.ReadDir(scenariosDir)
		if err != nil {
			continue
		}
		for _, sampleEntry := range sampleEntries {
			if !sampleEntry.IsDir() {
				continue
			}
			sampleName := sampleEntry.Name()
			samplePath := filepath.Join(scenariosDir, sampleName)
			teilEntries, err := os.ReadDir(samplePath)
			if err != nil {
				continue
			}
			for _, teilEntry := range teilEntries {
				if !teilEntry.IsDir() {
					continue
				}
				teilName := teilEntry.Name()
				teilNum := parseTeilDirNumber(teilName)
				if teilNum <= 0 {
					continue
				}
				scriptPath := filepath.Join(samplePath, teilName, "script.json")
				rawBytes, err := os.ReadFile(scriptPath)
				if err != nil || len(rawBytes) == 0 {
					continue
				}
				entry, err := parseScriptFileBytes(rawBytes)
				if err != nil {
					c.log.Warn("Failed to parse scenario script.json during pre-load", "path", scriptPath, "error", err)
					continue
				}
				key := cacheKey(contracts.ExamTelc, contracts.LevelB1, sampleName, teilNum)
				c.mu.Lock()
				if _, exists := c.entries[key]; !exists {
					c.entries[key] = entry
					loaded++
				}
				c.mu.Unlock()
			}
		}
		if loaded > 0 {
			break
		}
	}

	if loaded > 0 {
		c.log.Info("Pre-loaded all B1 scenario scripts locally from catalog/ (0 network calls to S3)", "count", loaded)
	}
}

func parseTeilDirNumber(name string) int {
	trimmed := strings.TrimSpace(name)
	for _, prefix := range []string{"Teil_", "teil_", "Teil", "teil"} {
		if strings.HasPrefix(trimmed, prefix) {
			if n, err := strconv.Atoi(strings.TrimPrefix(trimmed, prefix)); err == nil && n > 0 {
				return n
			}
		}
	}
	return 0
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
