package engine

import (
	"log/slog"

	"voice-engine/config"
	"voice-engine/services"
)

// Ledger aliases services.Ledger so engine consumers have a unified package entry point.
type Ledger = services.Ledger
type TurnInput = services.TurnInput
type EvalInput = services.EvalInput
type AttemptKey = services.AttemptKey

// NewLedger creates a new cost & attempt Ledger.
func NewLedger(cfg config.Config, log *slog.Logger) *Ledger {
	return services.NewLedger(cfg, log)
}
