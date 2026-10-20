package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"ai_exam_simulator/services"
)

type EvaluateRequest struct {
	EvaluationPrompt string `json:"evaluation_prompt"`
	Transcript       string `json:"transcript"`

	// Attempt identity (optional). When attempt_id is present the evaluation result and its
	// LLM cost are attached to that attempt and rolled up into exam/user summaries.
	UserID     string `json:"user_id"`
	ExamID     string `json:"exam_id"`
	ExamLevel  string `json:"exam_level"`
	TeilNumber int    `json:"teil_number"`
	AttemptID  string `json:"attempt_id"`
}

type CostSummary struct {
	EvalCostUSD    float64 `json:"eval_cost_usd"`
	AttemptCostUSD float64 `json:"attempt_cost_usd"`
	TurnsCount     int     `json:"turns_count"`
}

type EvaluateResponse struct {
	Result      string       `json:"result"`
	CostSummary *CostSummary `json:"cost_summary,omitempty"`
}

func NewEvaluate(evaluator Evaluator, ledger *services.Ledger, arch services.Archiver, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB limit for safety

		var req EvaluateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		log.Info("Received evaluation request", "attempt_id", req.AttemptID, "exam_id", req.ExamID)

		ctx, usage := services.WithUsage(r.Context())
		result, err := evaluator.Evaluate(ctx, req.EvaluationPrompt, req.Transcript)
		if err != nil {
			log.Error("evaluation failed", "error", err)
			http.Error(w, "evaluation failed", http.StatusInternalServerError)
			return
		}

		resp := EvaluateResponse{Result: result}
		if req.AttemptID != "" {
			res := ledger.RecordEvaluation(services.EvalInput{
				Key: services.AttemptKey{
					UserID: req.UserID, ExamID: req.ExamID, ExamLevel: req.ExamLevel,
					TeilNumber: req.TeilNumber, AttemptID: req.AttemptID,
				},
				ResultRaw: result,
				Usage:     usage.Snapshot(),
			})
			arch.Enqueue(res.Batch)
			resp.CostSummary = &CostSummary{
				EvalCostUSD: res.TurnCostUSD, AttemptCostUSD: res.AttemptCostUSD, TurnsCount: res.TurnsCount,
			}
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Error("failed to encode evaluation response", "error", err)
		}
	}
}
