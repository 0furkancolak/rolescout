package main

import (
	"context"
	"log/slog"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"example.com/rolescout/api/internal/config"
	"example.com/rolescout/api/internal/discovery"
	"example.com/rolescout/api/internal/enrich"
	"example.com/rolescout/api/internal/infrastructure/database"
	"example.com/rolescout/api/internal/ingest"
	"example.com/rolescout/api/internal/ingest/adapters"
	"example.com/rolescout/api/internal/llmconfig"
	"example.com/rolescout/api/internal/research"
	"example.com/rolescout/api/internal/sysstat"
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
	resolver := &llmconfig.Resolver{Env: cfg.LLM, Store: database.NewSettingsRepository(pool)}
	pipeline := &research.Pipeline{
		Runs:      database.NewResearchRepository(pool),
		Companies: companies,
		Jobs:      jobs,
		Ingest:    runner,
		Detector:  &discovery.Detector{Client: crawler},
		Sources: []discovery.Source{
			discovery.SeedSource{},
			&discovery.Probe{Client: crawler},
			// Names from other places, turned into boards by the same probe: the
			// stock exchanges, and (when AI is on) sector by sector.
			&discovery.FeedSource{Feed: &discovery.ExchangeFeed{Client: crawler}, Probe: &discovery.Probe{Client: crawler}},
			&discovery.FeedSource{Feed: &discovery.SectorFeed{LLM: llmconfig.Dynamic{R: resolver},
				Enabled: func(ctx context.Context) bool { return resolver.Current(ctx).Enabled }}, Probe: &discovery.Probe{Client: crawler}},
			// The companies of the focus countries (and a few big ones) with a
			// website in DBpedia; the detector finds their career pages.
			&discovery.DBpedia{Client: crawler},
			&discovery.CatalogProbe{Probe: &discovery.Probe{Client: crawler}, Names: companies.NamesWithoutBoard},
			&discovery.HackerNews{Client: crawler, ThreadURL: "https://hn.algolia.com/api/v1"},
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
	go sysstat.Publish(ctx, database.NewStatsRepository(pool), "worker", 15*time.Second, logger)
	var wg sync.WaitGroup
	wg.Add(2)
	// The enricher always runs and asks the live settings each tick, so turning
	// AI on or changing its key in the web app takes effect without a restart.
	// Whether it reads the whole catalog on its own is decided there too: with
	// a hosted service that is off unless an administrator chooses it.
	wg.Add(1)
	enricher := &enrich.Enricher{
		Repo:    database.NewEnrichmentRepository(pool),
		LLM:     llmconfig.Dynamic{R: resolver},
		Resolve: resolver.Current,
		Model:   cfg.LLM.Model, FallbackModel: cfg.LLM.FallbackModel,
		Concurrency: cfg.LLM.Concurrency, Batch: cfg.LLM.Batch,
		MaxChars: cfg.LLM.MaxInputChars, MaxTokens: cfg.LLM.MaxOutputTokens,
		Breaker:         enrich.NewBreaker(3, 10*time.Minute, nil),
		Logger:          logger,
		ResolveCountry:  ingest.InferCountry,
		ExplainMinScore: 70,
	}
	logger.Info("AI enrichment loop started", "env_enabled", cfg.LLM.Enabled, "host", hostOf(cfg.LLM.BaseURL))
	go func() {
		defer wg.Done()
		enricher.Loop(ctx, cfg.WorkerTick)
	}()
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

// hostOf is the host of a URL for logging; the path and key are not logged.
func hostOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return u.Host
}

func main() {
	if err := run(); err != nil {
		slog.Error("worker failed", "error", err)
		os.Exit(1)
	}
}
