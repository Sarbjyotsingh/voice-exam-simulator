package server

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"voice-engine/config"
	"voice-engine/internal/api"
	"voice-engine/internal/contracts"
	"voice-engine/internal/engine"
	"voice-engine/internal/modes/conversation"
	"voice-engine/internal/modes/exam"
	"voice-engine/internal/providers"
	"voice-engine/services"
)

// Run starts the HTTP & WebSocket server with all providers, engine components, and pluggable mode handlers wired.
func Run() {
	healthCheck := flag.Bool("healthcheck", false, "run healthcheck and exit")
	flag.Parse()

	if *healthCheck {
		cfg := config.MustLoad()
		resp, err := http.Get("http://127.0.0.1" + cfg.Addr() + "/health")
		if err != nil || resp.StatusCode != 200 {
			os.Exit(1)
		}
		os.Exit(0)
	}

	cfg := config.MustLoad()
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	httpClient := services.NewHTTPClient(cfg.UpstreamTimeout)
	registry := providers.NewRegistry(cfg, "catalog", httpClient, log)
	composer := engine.NewPromptComposer("catalog")
	scriptCache := exam.NewScriptCache(".", "", httpClient, log)
	arch := engine.NewArchiver(ctx, cfg, log)
	ledger := engine.NewLedger(cfg, log)

	var examRouter *exam.ExamDecisionRouter
	if cfg.OpenAIAPIKey != "" || cfg.GroqAPIKey != "" {
		examRouter = exam.NewExamDecisionRouter(registry, "catalog", log)
		log.Info("Exam DecisionRouter initialized", "enabled_teils", cfg.DecisionsEnabledTeils)
	}

	examPlugin := exam.NewPlugin(cfg, registry, composer, scriptCache, examRouter, log)
	convPlugin := conversation.NewPlugin(registry, composer)

	modes := map[contracts.ModeID]engine.ModeHandler{
		contracts.ModeExam:         examPlugin,
		contracts.ModeConversation: convPlugin,
	}

	wsExamHandler := api.NewWSExamHandler(cfg, registry, modes, arch, ledger, log)
	evalHandler := api.NewEvaluateHandler(modes, ledger, arch, log)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", api.Health)
	mux.Handle("POST /evaluate", evalHandler)
	mux.Handle("POST /api/v1/evaluate", evalHandler)
	mux.Handle("GET /exam", wsExamHandler)
	mux.Handle("GET /api/v1/turn", wsExamHandler)

	srv := &http.Server{
		Addr:              cfg.Addr(),
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		log.Info("Server listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("ListenAndServe failed", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	log.Info("Shutting down gracefully...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("HTTP shutdown error", "error", err)
	}

	arch.Close()
	log.Info("Shutdown complete")
}
