package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"voice-exam-simulator/internal/contracts"
)

// PromptLayerQuery specifies which 4-layer prompt stack to compose.
type PromptLayerQuery struct {
	Mode         contracts.ModeID
	Language     contracts.LanguageCode
	ExamProvider contracts.ExamProvider
	Level        contracts.CEFRLevel
	TeilNumber   int
	ScenarioID   string
	Kind         contracts.PromptKind
}

// PromptComposer loads and concatenates the 4-layer Markdown prompt stack (Base -> Provider -> Level -> Teil/Scenario)
// and resolves {{...}} PromptVars.
type PromptComposer struct {
	catalogDir string
	mu         sync.RWMutex
	fileCache  map[string]string
}

// NewPromptComposer creates a PromptComposer rooted at catalogDir (default "catalog").
func NewPromptComposer(catalogDir string) *PromptComposer {
	if catalogDir == "" {
		catalogDir = "catalog"
	}
	return &PromptComposer{
		catalogDir: catalogDir,
		fileCache:  make(map[string]string),
	}
}

// Build assembles the 4-layer prompt for query and applies vars.
func (c *PromptComposer) Build(query PromptLayerQuery, vars map[contracts.PromptVar]string) string {
	relPaths := c.layerPaths(query)
	var layers []string
	for _, rel := range relPaths {
		if content := c.readCatalogFile(rel); content != "" {
			layers = append(layers, strings.TrimSpace(content))
		}
	}
	combined := strings.Join(layers, "\n\n")
	return contracts.ApplyPromptVars(combined, vars)
}

func (c *PromptComposer) layerPaths(q PromptLayerQuery) []string {
	mode := contracts.NormalizeMode(string(q.Mode))
	lang := strings.ToLower(string(contracts.NormalizeLanguage(string(q.Language))))
	prov := strings.ToLower(string(contracts.NormalizeExamProvider(string(q.ExamProvider))))
	lvl := strings.ToLower(string(contracts.NormalizeCEFRLevel(string(q.Level))))

	if mode == contracts.ModeConversation {
		scen := q.ScenarioID
		if scen == "" {
			scen = "b1_apartment_viewing"
		}
		if q.Kind == contracts.PromptKindEvaluate {
			return []string{
				filepath.Join("prompts", "levels", lang, lvl, "level_rules.md"),
				filepath.Join("conversations", lang, scen, "prompts", "feedback.md"),
			}
		}
		return []string{
			filepath.Join("prompts", "base", "conversation_base.md"),
			filepath.Join("prompts", "levels", lang, lvl, "level_rules.md"),
			filepath.Join("conversations", lang, scen, "prompts", "scenario.md"),
		}
	}

	// ModeExam: 4-Layer Composable Prompt Stack (Base -> Provider -> Level -> Teil)
	teilDir := fmt.Sprintf("teil_%d", q.TeilNumber)
	examFolder := fmt.Sprintf("%s_%s", prov, lvl)

	switch q.Kind {
	case contracts.PromptKindDecisions:
		return []string{
			filepath.Join("prompts", "base", "exam_decisions_base.md"),
			filepath.Join("prompts", "providers", prov, "decisions_provider.md"),
			filepath.Join("prompts", "levels", lang, lvl, "level_rules.md"),
			filepath.Join("exams", lang, examFolder, teilDir, "prompts", "decisions_teil.md"),
		}
	case contracts.PromptKindEvaluate:
		return []string{
			filepath.Join("prompts", "base", "exam_evaluate_base.md"),
			filepath.Join("prompts", "providers", prov, "evaluate_provider.md"),
			filepath.Join("prompts", "levels", lang, lvl, "level_rules.md"),
			filepath.Join("exams", lang, examFolder, teilDir, "prompts", "evaluate_teil.md"),
		}
	default: // PromptKindFallbackSystem
		return []string{
			filepath.Join("prompts", "base", "exam_fallback_base.md"),
			filepath.Join("prompts", "providers", prov, "fallback_provider.md"),
			filepath.Join("prompts", "levels", lang, lvl, "level_rules.md"),
			filepath.Join("exams", lang, examFolder, teilDir, "prompts", "fallback_teil.md"),
		}
	}
}

func (c *PromptComposer) readCatalogFile(relPath string) string {
	c.mu.RLock()
	if cached, ok := c.fileCache[relPath]; ok {
		c.mu.RUnlock()
		return cached
	}
	c.mu.RUnlock()

	candidates := []string{
		filepath.Join(c.catalogDir, relPath),
		filepath.Join("..", c.catalogDir, relPath),
		filepath.Join("../..", c.catalogDir, relPath),
	}
	for _, p := range candidates {
		if b, err := os.ReadFile(p); err == nil {
			content := string(b)
			c.mu.Lock()
			c.fileCache[relPath] = content
			c.mu.Unlock()
			return content
		}
	}
	return ""
}
