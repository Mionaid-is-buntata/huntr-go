package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/campbell/huntr-ai/internal/common"
	"github.com/campbell/huntr-ai/internal/web"
)

func main() {
	common.SetupLogger("web", os.Getenv("LOG_LEVEL"), nil)
	slog.Info("Huntr Web Service — Starting...")

	paths := web.DefaultDataPaths()

	// Allow template dir override for development
	if dir := os.Getenv("TEMPLATE_DIR"); dir != "" {
		paths.TemplateDir = dir
	}

	if err := paths.EnsureDirs(); err != nil {
		slog.Error("failed to create data directories", "error", err)
		os.Exit(1)
	}

	// Validate email credentials
	email := os.Getenv("HUNTR_EMAIL")
	password := os.Getenv("HUNTR_EMAIL_PASSWORD")
	recipient := os.Getenv("HUNTR_EMAIL_RECIPIENT")
	if email == "" || password == "" || recipient == "" {
		slog.Warn("email credentials not configured — notifications disabled")
	} else {
		slog.Info("email credentials validated")
	}

	srv := web.NewServer(paths)
	httpServer := &http.Server{
		Addr:         ":5000",
		Handler:      srv.Router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	go func() {
		slog.Info("listening on :5000")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-done
	slog.Info("shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		slog.Error("shutdown error", "error", err)
	}
	slog.Info("server stopped")
}
