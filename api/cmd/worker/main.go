package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"example.com/rolescout/api/internal/config"
	"example.com/rolescout/api/internal/discovery"
	"example.com/rolescout/api/internal/infrastructure/database"
	"example.com/rolescout/api/internal/ingest"
	"example.com/rolescout/api/internal/ingest/adapters"
	"example.com/rolescout/api/internal/research"
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
	jobs := database.NewJobRepository(pool)
	companies := database.NewCompanyRepository(pool)
	runner := &ingest.Runner{
		Sources:   database.NewSourceRepository(pool),
		Listings:  jobs,
		Companies: companies,
		Adapters:  adapters.Registry(crawler, adapters.Options{JSONLDMaxPages: cfg.Crawl.JSONLDMaxPages}),
		Logger:    logger,
	}
	pipeline := &research.Pipeline{
		Runs:      database.NewResearchRepository(pool),
		Companies: companies,
		Jobs:      jobs,
		Ingest:    runner,
		Detector:  &discovery.Detector{Client: crawler},
		Sources: []discovery.Source{
			discovery.SeedSource{},
			&discovery.HackerNews{Client: crawler},
			&discovery.MarkdownList{Client: crawler, URLs: discovery.DefaultLists},
		},
		Config: research.Config{
			DiscoveryInterval:   cfg.Crawl.DiscoveryInterval,
			CompanySyncInterval: cfg.Crawl.CompanySyncInterval,
			Concurrency:         cfg.Crawl.Concurrency,
			MatchMinScore:       cfg.Crawl.MatchMinScore,
			ClosedJobRetention:  cfg.Crawl.ClosedJobRetention,
		},
		Logger: logger,
	}
	logger.Info("worker started", "version", version, "tick", cfg.WorkerTick.String(),
		"adapters", runner.Adapters.Names(), "concurrency", cfg.Crawl.Concurrency)

	// Scheduled source syncs keep data fresh between research runs; research
	// runs execute the full pipeline. Both stop when ctx is cancelled.
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		runner.Loop(ctx, cfg.WorkerTick, ingest.RunDueOptions{Concurrency: cfg.Crawl.Concurrency})
	}()
	go func() {
		defer wg.Done()
		pipeline.Loop(ctx, cfg.WorkerTick)
	}()
	wg.Wait()
	logger.Info("worker stopped")
	return nil
}

func main() {
	if err := run(); err != nil {
		slog.Error("worker failed", "error", err)
		os.Exit(1)
	}
}
