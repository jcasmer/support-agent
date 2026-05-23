package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	httpAdapter "github.com/jcasmer/support-agent/internal/adapters/in/http"
	"github.com/jcasmer/support-agent/internal/adapters/out/anthropic"
	"github.com/jcasmer/support-agent/internal/app"
	"github.com/jcasmer/support-agent/internal/core"
	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables from .env
	if err := godotenv.Load(); err != nil {
		slog.Warn("no .env file found, reading from environment")
	}

	// Setup structured JSON logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Read required config
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		slog.Error("ANTHROPIC_API_KEY is required")
		os.Exit(1)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Build dependency graph — bottom up
	// out adapter → core → app → in adapter
	llmClient := anthropic.New(apiKey)

	classifier := core.NewClassifierAgent(llmClient)
	resolver := core.NewResolverAgent(llmClient)
	escalator := core.NewEscalatorAgent(llmClient)

	orch := app.New(classifier, resolver, escalator)

	supportHandler := httpAdapter.New(orch)

	// Register routes
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/support", supportHandler.Handle)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Build server with production-safe timeouts
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		slog.Info("server starting", "port", port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// Graceful shutdown on SIGINT or SIGTERM
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("forced shutdown", "error", err)
		os.Exit(1)
	}

	slog.Info("server stopped")
}