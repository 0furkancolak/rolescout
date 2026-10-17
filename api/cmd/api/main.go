package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/recover"

	"example.com/rolescout/api/internal/config"
	"example.com/rolescout/api/internal/infrastructure/database"
	httptransport "example.com/rolescout/api/internal/transport/http"
)

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

	migrationCtx, migrationCancel := context.WithTimeout(ctx, 30*time.Second)
	err = database.RunMigrations(migrationCtx, pool, cfg.Migrations)
	migrationCancel()
	if err != nil {
		return err
	}

	app := fiber.New(fiber.Config{
		AppName:      "RoleScout API",
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
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
		Workspaces:     httptransport.MembershipAuthorizer{Workspaces: workspaces},
	})
	if !cfg.CookieSecure {
		logger.Warn("COOKIE_SECURE=false; session cookies are sent over plain HTTP")
	}

	addr := fmt.Sprintf("%s:%d", cfg.HTTPHost, cfg.HTTPPort)
	logger.Info("API listening", "addr", addr, "trusted_proxies", cfg.TrustedProxies)

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
