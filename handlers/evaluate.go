package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

type EvaluateRequest struct {
	EvaluationPrompt string `json:"evaluation_prompt"`
	Transcript       string `json:"transcript"`
}

type EvaluateResponse struct {
	Result string `json:"result"`
}

func NewEvaluate(evaluator Evaluator, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB limit for safety

		var req EvaluateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		log.Info("Received evaluation request")
		result, err := evaluator.Evaluate(r.Context(), req.EvaluationPrompt, req.Transcript)
		if err != nil {
			log.Error("evaluation failed", "error", err)
			http.Error(w, "evaluation failed", http.StatusInternalServerError)
			return
		}

		resp := EvaluateResponse{Result: result}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Error("failed to encode evaluation response", "error", err)
		}
	}
}
