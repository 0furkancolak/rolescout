package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/recover"

	"example.com/rolescout/api/internal/config"
	"example.com/rolescout/api/internal/enrich"
	"example.com/rolescout/api/internal/infrastructure/database"
	"example.com/rolescout/api/internal/ingest"
	"example.com/rolescout/api/internal/llmconfig"
	httptransport "example.com/rolescout/api/internal/transport/http"
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

	// Data migrations on a large catalog (30,000 jobs with long descriptions) can
	// take a while; this must not be the reason an upgrade fails.
	migrationCtx, migrationCancel := context.WithTimeout(ctx, 10*time.Minute)
	err = database.RunMigrations(migrationCtx, pool, cfg.Migrations)
	migrationCancel()
	if err != nil {
		return err
	}

	app := fiber.New(fiber.Config{
		AppName:     "RoleScout API",
		ReadTimeout: 10 * time.Second,
		// Long enough for a summary streamed from the local model.
		WriteTimeout: 3 * time.Minute,
		IdleTimeout:  60 * time.Second,
		BodyLimit:    1 << 20,
		ErrorHandler: httptransport.ErrorHandler,
		// The client address comes from X-Real-IP only when the request was
		// forwarded by a trusted proxy (nginx in the Compose setup).
		TrustProxy:       len(cfg.TrustedProxies) > 0,
		TrustProxyConfig: fiber.TrustProxyConfig{Proxies: cfg.TrustedProxies},
		ProxyHeader:      "X-Real-IP",
	})

	metrics := httptransport.NewMetrics()
	app.Use(httptransport.RequestID(), httptransport.Observe(logger, metrics), recover.New())
	httptransport.RegisterHealthRoutes(app, pool)
	httptransport.RegisterMetricsRoute(app, metrics, pool)
	workspaces := database.NewWorkspaceRepository(pool)
	enrichments := database.NewEnrichmentRepository(pool)
	// AI settings saved in the web app override the environment, so a model,
	// URL or API key can change without a restart.
	resolver := &llmconfig.Resolver{Env: cfg.LLM, Store: database.NewSettingsRepository(pool)}
	httptransport.RegisterAPIRoutes(app, httptransport.Deps{
		Auth: database.NewAuthRepository(pool),
		AuthConfig: httptransport.AuthConfig{
			SessionTTL:        cfg.SessionTTL,
			CookieSecure:      cfg.CookieSecure,
			AllowRegistration: cfg.AllowRegistration,
		},
		RateLimits: httptransport.RateLimits{
			Login:    httptransport.Limit{Max: cfg.LoginRateLimit, Window: httptransport.DefaultRateLimits.Login.Window},
			Register: httptransport.Limit{Max: cfg.RegisterRateLimit, Window: httptransport.DefaultRateLimits.Register.Window},
		},
		WorkspaceStore: workspaces,
		Jobs:           database.NewJobRepository(pool),
		Applications:   database.NewApplicationRepository(pool),
		Annotations:    database.NewAnnotationRepository(pool),
		Research:       database.NewResearchRepository(pool),
		Companies:      database.NewCompanyRepository(pool),
		Activity:       database.NewActivityRepository(pool),
		UserState:      database.NewUserStateRepository(pool),
		Enrichments:    enrichments,
		LLM:            &enrich.Checker{Repo: enrichments, Resolve: resolver.Client},
		LLMSettings:    resolver,
		AI: &enrich.Analyzer{Repo: enrichments, LLM: llmconfig.Dynamic{R: resolver},
			Resolve: resolver.Current, ResolveCountry: ingest.InferCountry},
		Suggester: &enrich.ProfileSuggester{LLM: llmconfig.Dynamic{R: resolver}, Resolve: resolver.Current},
		Summaries: &enrich.Summarizer{Client: llmconfig.Dynamic{R: resolver}, ModelFn: func(ctx context.Context) string { return resolver.Current(ctx).Model },
			Store: database.NewSummaryRepository(pool)},
		CrawlSettings: crawlSettings(cfg.Crawl),
		Workspaces:    httptransport.MembershipAuthorizer{Workspaces: workspaces},
	})
	if !cfg.CookieSecure {
		logger.Warn("COOKIE_SECURE=false; session cookies are sent over plain HTTP")
	}

	addr := fmt.Sprintf("%s:%d", cfg.HTTPHost, cfg.HTTPPort)
	logger.Info("API listening", "version", version, "addr", addr, "trusted_proxies", cfg.TrustedProxies)

	// Listen returns once ctx is cancelled and graceful shutdown completes.
	return app.Listen(addr, fiber.ListenConfig{
		GracefulContext:       ctx,
		ShutdownTimeout:       10 * time.Second,
		DisableStartupMessage: true,
	})
}

func main() {
	if err := run(); err != nil {
		slog.Error("API stopped", "error", err)
		os.Exit(1)
	}
}

// crawlSettings lists the worker's crawl settings for administrators. The
// API reads the same variables as the worker (see compose.yaml).
func crawlSettings(c config.Crawl) []httptransport.Setting {
	return []httptransport.Setting{
		{Name: "CRAWL_CONCURRENCY", Value: strconv.Itoa(c.Concurrency)},
		{Name: "CRAWL_HOST_INTERVAL", Value: c.HostInterval.String()},
		{Name: "CRAWL_VENDOR_CONCURRENCY", Value: strconv.Itoa(c.VendorConcurrency)},
		{Name: "CRAWL_USER_AGENT_URL", Value: c.UserAgentURL},
		{Name: "DISCOVERY_INTERVAL", Value: c.DiscoveryInterval.String()},
		{Name: "COMPANY_SYNC_INTERVAL", Value: c.CompanySyncInterval.String()},
		{Name: "RESEARCH_INTERVAL", Value: c.ResearchInterval.String()},
		{Name: "JSONLD_MAX_PAGES", Value: strconv.Itoa(c.JSONLDMaxPages)},
		{Name: "MATCH_MIN_SCORE", Value: strconv.Itoa(c.MatchMinScore)},
		{Name: "CLOSED_JOB_RETENTION", Value: c.ClosedJobRetention.String()},
	}
}
