package services

import (
	"io"
	"log/slog"
	"math"
	"strings"
	"testing"

	"voice-exam-simulator/config"
)

func testCfg(dir string) config.Config {
	return config.Config{
		WhisperPricePerHourUSD:  0.111,
		GroqInputPer1MUSD:       0.15,
		GroqOutputPer1MUSD:      0.60,
		ElevenLabsPer1KCharsUSD: 0.05,
		VoiceCostMultipliers:    map[string]float64{"std": 1.0, "premium": 2.0},
		LedgerDir:               dir,
		ArchiveInputAudio:       true,
	}
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestPricing(t *testing.T) {
	p := NewPricing(testCfg(""))

	// 20s of audio (above the 10s minimum).
	if got, want := p.STT(20), 20.0/3600*0.111; !near(got, want) {
		t.Errorf("STT = %v, want %v", got, want)
	}
	// 1000 prompt + 200 completion tokens.
	if got, want := p.LLM(1000, 200), 1000.0/1e6*0.15+200.0/1e6*0.60; !near(got, want) {
		t.Errorf("LLM = %v, want %v", got, want)
	}
	// Voice multiplier doubles the price; unknown voices default to 1.0.
	calls := []TTSCall{{VoiceID: "premium", Characters: 1000}, {VoiceID: "unknown", Characters: 1000}}
	if got, want := p.TTS(calls), 0.05*2+0.05; !near(got, want) {
		t.Errorf("TTS = %v, want %v", got, want)
	}
	// Turns shorter than Groq's 10s minimum are billed as 10s.
	c := p.TurnCost(UsageSnapshot{STTSeconds: 3})
	if want := 10.0 / 3600 * 0.111; !near(c.STT, want) {
		t.Errorf("min-billed STT = %v, want %v", c.STT, want)
	}
	if p.TurnCost(UsageSnapshot{}).Total != 0 {
		t.Error("zero usage must cost zero (pre-recorded turns are free)")
	}
}

func TestSanitizeID(t *testing.T) {
	if got := SanitizeID("../../etc/passwd", "x"); strings.Contains(got, "/") || strings.HasPrefix(got, ".") {
		t.Errorf("unsafe id not sanitised: %q", got)
	}
	if got := SanitizeID("  ", "anonymous"); got != "anonymous" {
		t.Errorf("fallback = %q", got)
	}
}

func TestLedgerRollupAndPersistence(t *testing.T) {
	dir := t.TempDir()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	l := NewLedger(testCfg(dir), log)

	key := AttemptKey{UserID: "u1", ExamID: "B1_S01", ExamLevel: "B1", TeilNumber: 1, AttemptID: "att_1"}
	parts := []map[string]interface{}{{"audio_id": "DE_LEY_EXP_01", "text": "Ah, interessant!"}}

	// Turn 1 then the user "closes the app": totals must already be recorded.
	r1 := l.RecordTurn(TurnInput{
		Key: key, Status: TurnOK, UserText: "Hallo", AIParts: parts,
		Usage:   UsageSnapshot{STTSeconds: 12, PromptTokens: 1000, CompletionTokens: 100},
		AudioIn: []byte("RIFF....WAVE"),
	})
	if r1.TurnCostUSD <= 0 || !near(r1.TurnCostUSD, r1.AttemptCostUSD) {
		t.Fatalf("turn1 cost wrong: %+v", r1)
	}
	var keys []string
	for _, f := range r1.Batch.Files {
		keys = append(keys, f.Key)
	}
	joined := strings.Join(keys, "\n")
	for _, want := range []string{
		"users/u1/exams/B1_B1_S01_teil1/attempts/att_1/turns/turn_01.json",
		"users/u1/exams/B1_B1_S01_teil1/attempts/att_1/turns/turn_01.input.wav",
		"users/u1/exams/B1_B1_S01_teil1/attempts/att_1/attempt_summary.json",
		"users/u1/exams/B1_B1_S01_teil1/exam_summary.json",
		"users/u1/user_summary.json",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing S3 key %q in:\n%s", want, joined)
		}
	}

	// Turn 2 ends the exam, then evaluation lands.
	r2 := l.RecordTurn(TurnInput{
		Key: key, Status: TurnOK, ExamEnded: true, AIParts: parts,
		Usage: UsageSnapshot{STTSeconds: 12, PromptTokens: 2000, CompletionTokens: 100},
	})
	if r2.TurnsCount != 2 || r2.AttemptCostUSD <= r1.AttemptCostUSD {
		t.Fatalf("turn2 rollup wrong: %+v", r2)
	}
	ev := l.RecordEvaluation(EvalInput{
		Key:       key,
		ResultRaw: `{"numerical_score":"22 / 25","telc_grade":"Pass with Distinction"}`,
		Usage:     UsageSnapshot{PromptTokens: 2500, CompletionTokens: 400},
	})
	if ev.AttemptCostUSD <= r2.AttemptCostUSD {
		t.Fatalf("evaluation cost not added: %+v", ev)
	}

	// A second attempt on the same exam rolls up into exam + user totals.
	key2 := key
	key2.AttemptID = "att_2"
	l.RecordTurn(TurnInput{Key: key2, Status: TurnOK, Usage: UsageSnapshot{PromptTokens: 1000}})

	st := l.user("u1")
	es := buildExamSummary(key.normalized(), st)
	if es.TotalAttempts != 2 || es.BestScore != "22 / 25" {
		t.Errorf("exam summary wrong: %+v", es)
	}
	us := buildUserSummary("u1", st)
	if us.TotalAttempts != 2 || us.TotalTurnsSpoken != 3 {
		t.Errorf("user summary wrong: %+v", us)
	}
	var sum float64
	for _, a := range st.Attempts {
		sum += a.Cost.Total
	}
	if !near(us.LifetimeCostUSD, sum) || !near(es.TotalExamCostUSD, sum) {
		t.Errorf("totals drifted: user=%v exam=%v attempts=%v", us.LifetimeCostUSD, es.TotalExamCostUSD, sum)
	}
	if a := st.Attempts[key.normalized().stateKey()]; a.Status != StatusEvaluated {
		t.Errorf("status = %q, want evaluated", a.Status)
	}

	// Simulate a server restart: a fresh ledger must reload totals from disk.
	l2 := NewLedger(testCfg(dir), log)
	us2 := buildUserSummary("u1", l2.user("u1"))
	if !near(us2.LifetimeCostUSD, us.LifetimeCostUSD) {
		t.Errorf("ledger not restored: %v vs %v", us2.LifetimeCostUSD, us.LifetimeCostUSD)
	}
}
