package handlers

import (
	"context"

	"voice-exam-simulator/services"
)

type Transcriber interface {
	Transcribe(ctx context.Context, wav []byte) (string, error)
}

type Chatter interface {
	Chat(ctx context.Context, msgs []services.Message) (string, error)
}

type Evaluator interface {
	Evaluate(ctx context.Context, prompt, transcript string) (string, error)
}

type Synthesizer interface {
	Synthesize(ctx context.Context, text, voiceID string) ([]byte, error)
}

type TurnRouter interface {
	RouteTurn(
		ctx context.Context,
		level string,
		teilNumber int,
		cat *services.ScriptCatalogue,
		history []services.Message,
		userUtterance string,
	) (services.RouteResult, error)
}
