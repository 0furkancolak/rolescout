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

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(logger)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	startupCtx, startupCancel := context.WithTimeout(ctx, 10*time.Second)
	pool, err := database.Open(startupCtx, cfg.DatabaseURL)
	startupCancel()
	if err != nil {
		return err
	}
	defer pool.Close()

	crawler := adapters.NewCrawler(version, cfg.Crawl)
	runner := &ingest.Runner{
		Sources:   database.NewSourceRepository(pool),
		Listings:  database.NewJobRepository(pool),
		Companies: database.NewCompanyRepository(pool),
		Adapters:  adapters.Registry(crawler, adapters.Options{JSONLDMaxPages: cfg.Crawl.JSONLDMaxPages}),
		Logger:    logger,
	}
	logger.Info("worker started", "version", version, "tick", cfg.WorkerTick.String(),
		"adapters", runner.Adapters.Names(), "concurrency", cfg.Crawl.Concurrency)
	runner.Loop(ctx, cfg.WorkerTick, ingest.RunDueOptions{Concurrency: cfg.Crawl.Concurrency})
	logger.Info("worker stopped")
	return nil
}

func main() {
	if err := run(); err != nil {
		slog.Error("worker failed", "error", err)
		os.Exit(1)
	}
}
