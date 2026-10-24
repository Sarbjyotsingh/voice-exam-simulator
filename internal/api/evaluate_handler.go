package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"voice-engine/internal/contracts"
	"voice-engine/internal/engine"
	"voice-engine/services"
)

// NewEvaluateHandler handles POST /evaluate (and /api/v1/evaluate) for both thin-client server-assembled prompts and legacy evaluation_prompt payloads.
func NewEvaluateHandler(
	modes map[contracts.ModeID]engine.ModeHandler,
	ledger *engine.Ledger,
	arch engine.Archiver,
	log *slog.Logger,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB limit

		var req contracts.EvaluateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		mode := contracts.NormalizeMode(req.Mode)
		modeHandler := modes[mode]
		if modeHandler == nil {
			modeHandler = modes[contracts.ModeExam]
		}

		log.Info("Received evaluation request",
			"mode", mode,
			"attempt_id", req.AttemptID,
			"exam_id", req.ExamID,
			"teil_number", req.TeilNumber,
		)

		ctx, usage := services.WithUsage(r.Context())
		result, err := modeHandler.Evaluate(ctx, req)
		if err != nil {
			log.Error("evaluation failed", "error", err)
			http.Error(w, "evaluation failed", http.StatusInternalServerError)
			return
		}

		resp := contracts.EvaluateResponse{Result: result}
		if req.AttemptID != "" {
			res := ledger.RecordEvaluation(services.EvalInput{
				Key: services.AttemptKey{
					UserID:     req.UserID,
					ExamID:     req.ExamID,
					ExamLevel:  req.ExamLevel,
					TeilNumber: req.TeilNumber,
					AttemptID:  req.AttemptID,
				},
				ResultRaw: result,
				Usage:     usage.Snapshot(),
			})
			arch.Enqueue(res.Batch)
			resp.CostSummary = &contracts.CostSummary{
				EvalCostUSD:    res.TurnCostUSD,
				AttemptCostUSD: res.AttemptCostUSD,
				TurnsCount:     res.TurnsCount,
			}
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Error("failed to encode evaluation response", "error", err)
		}
	}
}
