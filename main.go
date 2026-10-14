package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ai_exam_simulator/config"
	"ai_exam_simulator/handlers"
	"ai_exam_simulator/services"
)

func main() {
	healthCheck := flag.Bool("healthcheck", false, "run healthcheck and exit")
	flag.Parse()

	if *healthCheck {
		cfg := config.MustLoad() // Still fails if API keys are missing, which is fine
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
	groq := services.NewGroq(cfg, httpClient)
	tts := services.NewElevenLabs(cfg, httpClient)
	arch := services.NewArchiver(ctx, cfg, log)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handlers.Health)
	mux.Handle("POST /evaluate", handlers.NewEvaluate(groq, log))
	mux.Handle("GET /exam", handlers.NewExam(cfg, groq, groq, tts, arch, log))

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
	
	arch.Close() // Drains pending S3 uploads
	log.Info("Shutdown complete")
}
