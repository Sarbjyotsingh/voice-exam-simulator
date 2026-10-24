package services

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"voice-engine/config"
)

// ---------------------------------------------------------------------------
// Identity
// ---------------------------------------------------------------------------

// AttemptKey identifies one exam run. The client generates AttemptID once when the user
// starts an exam and sends it on every turn and on /evaluate.
type AttemptKey struct {
	UserID     string
	ExamID     string
	ExamLevel  string
	TeilNumber int
	AttemptID  string
}

var unsafeIDChars = regexp.MustCompile(`[^A-Za-z0-9._-]`)

// SanitizeID makes a client-supplied identifier safe to use inside an S3 key / file name.
func SanitizeID(s, fallback string) string {
	s = strings.Trim(unsafeIDChars.ReplaceAllString(strings.TrimSpace(s), "_"), "._")
	if s == "" {
		return fallback
	}
	if len(s) > 128 {
		s = s[:128]
	}
	return s
}

func (k AttemptKey) normalized() AttemptKey {
	k.UserID = SanitizeID(k.UserID, "anonymous")
	k.ExamID = SanitizeID(k.ExamID, "unknown")
	k.ExamLevel = SanitizeID(k.ExamLevel, "unknown")
	k.AttemptID = SanitizeID(k.AttemptID, "unknown")
	if k.TeilNumber < 0 {
		k.TeilNumber = 0
	}
	return k
}

func (k AttemptKey) examKey() string {
	return fmt.Sprintf("%s_%s_teil%d", k.ExamLevel, k.ExamID, k.TeilNumber)
}

func (k AttemptKey) stateKey() string { return k.examKey() + "|" + k.AttemptID }

func (k AttemptKey) userPrefix() string { return "users/" + k.UserID }

func (k AttemptKey) examPrefix() string { return k.userPrefix() + "/exams/" + k.examKey() }

func (k AttemptKey) attemptPrefix() string { return k.examPrefix() + "/attempts/" + k.AttemptID }

// ---------------------------------------------------------------------------
// Persisted / uploaded documents
// ---------------------------------------------------------------------------

const (
	StatusInProgress = "in_progress"
	StatusCompleted  = "completed_awaiting_eval"
	StatusEvaluated  = "evaluated"

	TurnOK              = "ok"
	TurnEmptyTranscript = "empty_transcription"
	TurnChatFailed      = "chat_failed"
)

type TurnEntry struct {
	TurnIndex    int                      `json:"turn_index"`
	Timestamp    time.Time                `json:"timestamp"`
	Status       string                   `json:"status"`
	UserText     string                   `json:"user_text"`
	UserAudioKey string                   `json:"user_audio_s3_key,omitempty"`
	AIParts      []map[string]interface{} `json:"ai_parts,omitempty"`
	AIAudioKey   string                   `json:"dynamic_tts_audio_s3_key,omitempty"`
	ExamEnded    bool                     `json:"exam_ended"`
	Usage        UsageSnapshot            `json:"usage"`
	Cost         Cost                     `json:"cost_usd"`
}

type AttemptUsage struct {
	STTSeconds           float64 `json:"stt_audio_seconds"`
	ChatPromptTokens     int     `json:"chat_prompt_tokens"`
	ChatCompletionTokens int     `json:"chat_completion_tokens"`
	DecisionTokens       int     `json:"decision_tokens,omitempty"`
	EvalPromptTokens     int     `json:"eval_prompt_tokens"`
	EvalCompletionTokens int     `json:"eval_completion_tokens"`
	TTSCharacters        int     `json:"tts_characters"`
	PrerecordedClips     int     `json:"prerecorded_clips_count"`
}

type AttemptSummary struct {
	AttemptID        string                 `json:"attempt_id"`
	UserID           string                 `json:"user_id"`
	ExamID           string                 `json:"exam_id"`
	ExamLevel        string                 `json:"exam_level"`
	TeilNumber       int                    `json:"teil_number"`
	Status           string                 `json:"status"`
	StartedAt        time.Time              `json:"started_at"`
	UpdatedAt        time.Time              `json:"updated_at"`
	CompletedAt      *time.Time             `json:"completed_at,omitempty"`
	TurnsCount       int                    `json:"turns_count"`
	Usage            AttemptUsage           `json:"usage"`
	Cost             Cost                   `json:"cost_usd"`
	Turns            []TurnEntry            `json:"turns"`
	EvalUsage        UsageSnapshot          `json:"-"`
	EvaluationCalls  int                    `json:"evaluation_calls"`
	EvaluationResult map[string]interface{} `json:"evaluation_result,omitempty"`
}

type ExamAttemptRef struct {
	AttemptID      string    `json:"attempt_id"`
	Status         string    `json:"status"`
	StartedAt      time.Time `json:"started_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	TurnsCount     int       `json:"turns_count"`
	CostUSD        float64   `json:"cost_usd"`
	NumericalScore string    `json:"numerical_score"`
	TelcGrade      string    `json:"telc_grade"`
}

type ExamSummary struct {
	UserID           string           `json:"user_id"`
	ExamKey          string           `json:"exam_key"`
	ExamID           string           `json:"exam_id"`
	ExamLevel        string           `json:"exam_level"`
	TeilNumber       int              `json:"teil_number"`
	TotalAttempts    int              `json:"total_attempts"`
	TotalExamCostUSD float64          `json:"total_exam_cost_usd"`
	BestScore        string           `json:"best_score"`
	BestGrade        string           `json:"best_grade"`
	UpdatedAt        time.Time        `json:"updated_at"`
	Attempts         []ExamAttemptRef `json:"attempts"`
}

type UserExamRef struct {
	ExamID        string  `json:"exam_id"`
	ExamLevel     string  `json:"exam_level"`
	TeilNumber    int     `json:"teil_number"`
	AttemptsCount int     `json:"attempts_count"`
	TotalCostUSD  float64 `json:"total_cost_usd"`
	BestScore     string  `json:"best_score"`
	LastStatus    string  `json:"last_status"`
}

type UserSummary struct {
	UserID           string                 `json:"user_id"`
	TotalAttempts    int                    `json:"total_attempts"`
	TotalTurnsSpoken int                    `json:"total_turns_spoken"`
	LifetimeCostUSD  float64                `json:"lifetime_cost_usd"`
	CostBreakdownUSD Cost                   `json:"cost_breakdown_usd"`
	Exams            map[string]UserExamRef `json:"exams"`
	UpdatedAt        time.Time              `json:"updated_at"`
}

type userState struct {
	mu       sync.Mutex
	Attempts map[string]*AttemptSummary `json:"attempts"`
}

// ---------------------------------------------------------------------------
// Ledger
// ---------------------------------------------------------------------------

// Ledger is the single source of truth for per-attempt / per-exam / per-user cost and
// results. It is updated synchronously on every turn so nothing is lost if the client
// disappears mid-exam. State is persisted to local disk because the log bucket is
// write-only (we can never read a summary back from S3 to extend it).
type Ledger struct {
	dir       string
	pricing   Pricing
	saveInput bool
	log       *slog.Logger

	mu    sync.Mutex
	users map[string]*userState
}

func NewLedger(cfg config.Config, logger *slog.Logger) *Ledger {
	l := &Ledger{
		dir:       cfg.LedgerDir,
		pricing:   NewPricing(cfg),
		saveInput: cfg.ArchiveInputAudio,
		log:       logger.With("component", "ledger"),
		users:     make(map[string]*userState),
	}
	if l.dir != "" {
		if err := os.MkdirAll(l.dir, 0o755); err != nil {
			l.log.Error("cannot create ledger dir, falling back to memory only", "dir", l.dir, "error", err)
			l.dir = ""
		}
	}
	return l
}

func (l *Ledger) Pricing() Pricing { return l.pricing }

func (l *Ledger) user(userID string) *userState {
	l.mu.Lock()
	defer l.mu.Unlock()
	if st, ok := l.users[userID]; ok {
		return st
	}
	st := &userState{Attempts: make(map[string]*AttemptSummary)}
	if l.dir != "" {
		if b, err := os.ReadFile(filepath.Join(l.dir, userID+".json")); err == nil {
			var disk struct {
				Attempts map[string]*AttemptSummary `json:"attempts"`
				Eval     map[string]UsageSnapshot   `json:"eval_usage"`
			}
			if err := json.Unmarshal(b, &disk); err == nil && disk.Attempts != nil {
				st.Attempts = disk.Attempts
				for k, u := range disk.Eval {
					if a, ok := st.Attempts[k]; ok {
						a.EvalUsage = u
					}
				}
			} else if err != nil {
				l.log.Error("corrupt ledger file, starting fresh", "user_id", userID, "error", err)
			}
		}
	}
	l.users[userID] = st
	return st
}

// persist writes the user's state atomically. Caller must hold st.mu.
func (l *Ledger) persist(userID string, st *userState) {
	if l.dir == "" {
		return
	}
	eval := make(map[string]UsageSnapshot, len(st.Attempts))
	for k, a := range st.Attempts {
		eval[k] = a.EvalUsage
	}
	b, err := json.Marshal(map[string]interface{}{"attempts": st.Attempts, "eval_usage": eval})
	if err != nil {
		l.log.Error("ledger marshal failed", "user_id", userID, "error", err)
		return
	}
	path := filepath.Join(l.dir, userID+".json")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		l.log.Error("ledger write failed", "user_id", userID, "error", err)
		return
	}
	if err := os.Rename(tmp, path); err != nil {
		l.log.Error("ledger rename failed", "user_id", userID, "error", err)
	}
}

func (l *Ledger) getAttempt(st *userState, k AttemptKey) *AttemptSummary {
	a, ok := st.Attempts[k.stateKey()]
	if !ok {
		now := time.Now().UTC()
		a = &AttemptSummary{
			AttemptID: k.AttemptID, UserID: k.UserID, ExamID: k.ExamID,
			ExamLevel: k.ExamLevel, TeilNumber: k.TeilNumber,
			Status: StatusInProgress, StartedAt: now, UpdatedAt: now,
			Turns: []TurnEntry{},
		}
		st.Attempts[k.stateKey()] = a
	}
	return a
}

func round8(f float64) float64 { return math.Round(f*1e8) / 1e8 }

func (a *AttemptSummary) recalc(p Pricing) {
	var u AttemptUsage
	var c Cost
	for _, t := range a.Turns {
		u.STTSeconds += t.Usage.STTSeconds
		u.ChatPromptTokens += t.Usage.PromptTokens
		u.ChatCompletionTokens += t.Usage.CompletionTokens
		u.DecisionTokens += t.Usage.DecisionTokens
		u.TTSCharacters += t.Usage.TTSCharacters
		for _, part := range t.AIParts {
			if id, _ := part["audio_id"].(string); id != "" {
				u.PrerecordedClips++
			} else if id, _ := part["audioId"].(string); id != "" {
				u.PrerecordedClips++
			}
		}
		c.STT += t.Cost.STT
		c.ChatLLM += t.Cost.ChatLLM
		c.Decisions += t.Cost.Decisions
		c.TTS += t.Cost.TTS
	}
	u.EvalPromptTokens = a.EvalUsage.PromptTokens
	u.EvalCompletionTokens = a.EvalUsage.CompletionTokens
	c.EvalLLM = p.LLM(a.EvalUsage.PromptTokens, a.EvalUsage.CompletionTokens)
	c.STT, c.ChatLLM, c.Decisions, c.TTS, c.EvalLLM = round8(c.STT), round8(c.ChatLLM), round8(c.Decisions), round8(c.TTS), round8(c.EvalLLM)
	c.Total = round8(c.STT + c.ChatLLM + c.Decisions + c.TTS + c.EvalLLM)
	a.Usage, a.Cost, a.TurnsCount = u, c, len(a.Turns)
}

// ---------------------------------------------------------------------------
// Recording
// ---------------------------------------------------------------------------

type TurnInput struct {
	Key       AttemptKey
	Status    string
	UserText  string
	LLMRaw    string
	AIParts   []map[string]interface{}
	ExamEnded bool
	Usage     UsageSnapshot
	AudioIn   []byte
	AudioOut  []byte
}

type LedgerResult struct {
	TurnCostUSD    float64
	AttemptCostUSD float64
	TurnsCount     int
	Batch          Batch
}

type turnFile struct {
	AttemptID  string `json:"attempt_id"`
	UserID     string `json:"user_id"`
	ExamID     string `json:"exam_id"`
	ExamLevel  string `json:"exam_level"`
	TeilNumber int    `json:"teil_number"`
	TurnEntry
	LLMRaw string `json:"llm_raw_response,omitempty"`
}

// RecordTurn adds one turn to its attempt, rolls up exam/user totals, persists the ledger
// and returns the S3 files to upload. Cost is recorded even for failed turns (an STT call
// that succeeded and was billed still costs money).
func (l *Ledger) RecordTurn(in TurnInput) LedgerResult {
	k := in.Key.normalized()
	st := l.user(k.UserID)
	st.mu.Lock()
	defer st.mu.Unlock()

	a := l.getAttempt(st, k)
	now := time.Now().UTC()
	idx := len(a.Turns) + 1
	prefix := fmt.Sprintf("%s/turns/turn_%02d", k.attemptPrefix(), idx)

	var files []File
	entry := TurnEntry{
		TurnIndex: idx, Timestamp: now, Status: SanitizeID(in.Status, TurnOK),
		UserText: in.UserText, AIParts: in.AIParts, ExamEnded: in.ExamEnded,
		Usage: in.Usage, Cost: l.pricing.TurnCost(in.Usage),
	}
	entry.Cost.STT, entry.Cost.ChatLLM, entry.Cost.Decisions, entry.Cost.TTS = round8(entry.Cost.STT), round8(entry.Cost.ChatLLM), round8(entry.Cost.Decisions), round8(entry.Cost.TTS)
	entry.Cost.Total = round8(entry.Cost.STT + entry.Cost.ChatLLM + entry.Cost.Decisions + entry.Cost.TTS)

	if l.saveInput && len(in.AudioIn) > 0 {
		ext, ct := sniffAudio(in.AudioIn)
		entry.UserAudioKey = prefix + ".input." + ext
		files = append(files, File{Key: entry.UserAudioKey, Body: in.AudioIn, ContentType: ct})
	}
	if len(in.AudioOut) > 0 {
		entry.AIAudioKey = prefix + ".output.mp3"
		files = append(files, File{Key: entry.AIAudioKey, Body: in.AudioOut, ContentType: "audio/mpeg"})
	}

	tf := turnFile{
		AttemptID: k.AttemptID, UserID: k.UserID, ExamID: k.ExamID,
		ExamLevel: k.ExamLevel, TeilNumber: k.TeilNumber, TurnEntry: entry, LLMRaw: in.LLMRaw,
	}
	files = append(files, jsonFile(prefix+".json", tf))

	a.Turns = append(a.Turns, entry)
	a.UpdatedAt = now
	if a.Status != StatusEvaluated {
		if in.ExamEnded && entry.Status == TurnOK {
			a.Status = StatusCompleted
			a.CompletedAt = &now
		} else if a.Status == "" {
			a.Status = StatusInProgress
		}
	}
	a.recalc(l.pricing)
	l.persist(k.UserID, st)

	files = append(files, l.summaryFiles(k, st, a)...)
	return LedgerResult{
		TurnCostUSD: entry.Cost.Total, AttemptCostUSD: a.Cost.Total, TurnsCount: a.TurnsCount,
		Batch: Batch{UserID: k.UserID, Files: files},
	}
}

type EvalInput struct {
	Key       AttemptKey
	ResultRaw string
	Usage     UsageSnapshot
}

var scoreRe = regexp.MustCompile(`(\d+(?:[.,]\d+)?)\s*/\s*(\d+(?:[.,]\d+)?)`)

func parseScore(s string) (score, max float64, ok bool) {
	m := scoreRe.FindStringSubmatch(s)
	if m == nil {
		return 0, 0, false
	}
	score, _ = strconv.ParseFloat(strings.Replace(m[1], ",", ".", 1), 64)
	max, _ = strconv.ParseFloat(strings.Replace(m[2], ",", ".", 1), 64)
	return score, max, max > 0
}

// RecordEvaluation attaches the evaluation result and its LLM cost to the attempt.
func (l *Ledger) RecordEvaluation(in EvalInput) LedgerResult {
	k := in.Key.normalized()
	st := l.user(k.UserID)
	st.mu.Lock()
	defer st.mu.Unlock()

	a := l.getAttempt(st, k)
	now := time.Now().UTC()

	result := map[string]interface{}{}
	if err := json.Unmarshal([]byte(in.ResultRaw), &result); err != nil || len(result) == 0 {
		result = map[string]interface{}{"raw": in.ResultRaw}
	}
	grade, _ := result["telc_grade"].(string)
	if s, _ := result["numerical_score"].(string); s != "" {
		if v, mx, ok := parseScore(s); ok {
			result["score_value"], result["max_score"] = v, mx
			if grade != "" {
				result["passed"] = !strings.Contains(strings.ToLower(grade), "not yet")
			} else {
				result["passed"] = v/mx >= 0.6
			}
		}
	}

	a.EvalUsage.PromptTokens += in.Usage.PromptTokens
	a.EvalUsage.CompletionTokens += in.Usage.CompletionTokens
	a.EvaluationCalls++
	a.EvaluationResult = result
	a.Status = StatusEvaluated
	a.UpdatedAt = now
	if a.CompletedAt == nil {
		a.CompletedAt = &now
	}
	a.recalc(l.pricing)
	l.persist(k.UserID, st)

	evalCost := l.pricing.EvalCost(in.Usage)
	files := []File{jsonFile(k.attemptPrefix()+"/evaluation.json", map[string]interface{}{
		"attempt_id": k.AttemptID, "user_id": k.UserID, "exam_id": k.ExamID,
		"exam_level": k.ExamLevel, "teil_number": k.TeilNumber, "timestamp": now,
		"result": result, "usage": in.Usage, "cost_usd": evalCost,
	})}
	files = append(files, l.summaryFiles(k, st, a)...)
	return LedgerResult{
		TurnCostUSD: round8(evalCost.Total), AttemptCostUSD: a.Cost.Total, TurnsCount: a.TurnsCount,
		Batch: Batch{UserID: k.UserID, Files: files},
	}
}

// ---------------------------------------------------------------------------
// Roll-ups (recomputed from attempts, so totals can never drift)
// ---------------------------------------------------------------------------

func (l *Ledger) summaryFiles(k AttemptKey, st *userState, current *AttemptSummary) []File {
	return []File{
		jsonFile(k.attemptPrefix()+"/attempt_summary.json", current),
		jsonFile(k.examPrefix()+"/exam_summary.json", buildExamSummary(k, st)),
		jsonFile(k.userPrefix()+"/user_summary.json", buildUserSummary(k.UserID, st)),
	}
}

func attemptScore(a *AttemptSummary) (num, grade string, value float64) {
	if a.EvaluationResult == nil {
		return "", "", -1
	}
	num, _ = a.EvaluationResult["numerical_score"].(string)
	grade, _ = a.EvaluationResult["telc_grade"].(string)
	if v, ok := a.EvaluationResult["score_value"].(float64); ok {
		return num, grade, v
	}
	return num, grade, -1
}

func buildExamSummary(k AttemptKey, st *userState) ExamSummary {
	es := ExamSummary{
		UserID: k.UserID, ExamKey: k.examKey(), ExamID: k.ExamID,
		ExamLevel: k.ExamLevel, TeilNumber: k.TeilNumber, Attempts: []ExamAttemptRef{},
	}
	best := -1.0
	for _, a := range st.Attempts {
		if a.UserID != k.UserID || fmt.Sprintf("%s_%s_teil%d", a.ExamLevel, a.ExamID, a.TeilNumber) != k.examKey() {
			continue
		}
		num, grade, val := attemptScore(a)
		es.Attempts = append(es.Attempts, ExamAttemptRef{
			AttemptID: a.AttemptID, Status: a.Status, StartedAt: a.StartedAt, UpdatedAt: a.UpdatedAt,
			TurnsCount: a.TurnsCount, CostUSD: a.Cost.Total, NumericalScore: num, TelcGrade: grade,
		})
		es.TotalExamCostUSD += a.Cost.Total
		if a.UpdatedAt.After(es.UpdatedAt) {
			es.UpdatedAt = a.UpdatedAt
		}
		if val > best {
			best, es.BestScore, es.BestGrade = val, num, grade
		}
	}
	sort.Slice(es.Attempts, func(i, j int) bool { return es.Attempts[i].StartedAt.Before(es.Attempts[j].StartedAt) })
	es.TotalAttempts = len(es.Attempts)
	es.TotalExamCostUSD = round8(es.TotalExamCostUSD)
	return es
}

func buildUserSummary(userID string, st *userState) UserSummary {
	us := UserSummary{UserID: userID, Exams: map[string]UserExamRef{}}
	best := map[string]float64{}
	latest := map[string]time.Time{}
	for _, a := range st.Attempts {
		ek := fmt.Sprintf("%s_%s_teil%d", a.ExamLevel, a.ExamID, a.TeilNumber)
		us.TotalAttempts++
		us.TotalTurnsSpoken += a.TurnsCount
		us.CostBreakdownUSD.STT += a.Cost.STT
		us.CostBreakdownUSD.ChatLLM += a.Cost.ChatLLM
		us.CostBreakdownUSD.Decisions += a.Cost.Decisions
		us.CostBreakdownUSD.EvalLLM += a.Cost.EvalLLM
		us.CostBreakdownUSD.TTS += a.Cost.TTS
		us.LifetimeCostUSD += a.Cost.Total
		if a.UpdatedAt.After(us.UpdatedAt) {
			us.UpdatedAt = a.UpdatedAt
		}
		ref, ok := us.Exams[ek]
		if !ok {
			ref = UserExamRef{ExamID: a.ExamID, ExamLevel: a.ExamLevel, TeilNumber: a.TeilNumber}
			best[ek] = -1
		}
		ref.AttemptsCount++
		ref.TotalCostUSD = round8(ref.TotalCostUSD + a.Cost.Total)
		if num, _, val := attemptScore(a); val > best[ek] {
			best[ek], ref.BestScore = val, num
		}
		if a.UpdatedAt.After(latest[ek]) {
			latest[ek], ref.LastStatus = a.UpdatedAt, a.Status
		}
		us.Exams[ek] = ref
	}
	c := &us.CostBreakdownUSD
	c.STT, c.ChatLLM, c.Decisions, c.EvalLLM, c.TTS = round8(c.STT), round8(c.ChatLLM), round8(c.Decisions), round8(c.EvalLLM), round8(c.TTS)
	c.Total = round8(c.STT + c.ChatLLM + c.Decisions + c.EvalLLM + c.TTS)
	us.LifetimeCostUSD = round8(us.LifetimeCostUSD)
	return us
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func jsonFile(key string, v interface{}) File {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		b = []byte(fmt.Sprintf(`{"error":%q}`, err.Error()))
	}
	return File{Key: key, Body: b, ContentType: "application/json"}
}

// sniffAudio returns the file extension and content type for recorded learner audio.
func sniffAudio(b []byte) (ext, contentType string) {
	switch {
	case len(b) >= 12 && string(b[0:4]) == "RIFF":
		return "wav", "audio/wav"
	case len(b) >= 8 && string(b[4:8]) == "ftyp":
		return "m4a", "audio/mp4"
	case len(b) >= 4 && string(b[0:4]) == "OggS":
		return "ogg", "audio/ogg"
	case len(b) >= 3 && (string(b[0:3]) == "ID3" || (b[0] == 0xFF && b[1]&0xE0 == 0xE0)):
		return "mp3", "audio/mpeg"
	case len(b) >= 4 && string(b[0:4]) == "fLaC":
		return "flac", "audio/flac"
	default:
		return "bin", "application/octet-stream"
	}
}
