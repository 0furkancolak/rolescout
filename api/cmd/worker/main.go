package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"example.com/rolescout/api/internal/config"
	"example.com/rolescout/api/internal/infrastructure/database"
	"example.com/rolescout/api/internal/ingest"
	"example.com/rolescout/api/internal/ingest/adapters"
)

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	startupCtx, startupCancel := context.WithTimeout(ctx, 10*time.Second)
	pool, err := database.Open(startupCtx, cfg.DatabaseURL)
	startupCancel()
	if err != nil {
		return err
	}
	defer pool.Close()

	runner := &ingest.Runner{
		Sources:  database.NewSourceRepository(pool),
		Listings: database.NewJobRepository(pool),
		Adapters: adapters.Registry(),
		Logger:   logger,
	}
	logger.Info("worker started", "tick", cfg.WorkerTick.String(), "adapters", runner.Adapters.Names())
	runner.Loop(ctx, cfg.WorkerTick)
	logger.Info("worker stopped")
	return nil
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("worker failed", "error", err)
		os.Exit(1)
	}
}
