package config

import (
	"log/slog"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("API_HOST", "")
	t.Setenv("API_PORT", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("MIGRATIONS_DIR", "")
	t.Setenv("SESSION_TTL", "")
	t.Setenv("COOKIE_SECURE", "")
	t.Setenv("ALLOW_REGISTRATION", "")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.HTTPPort != 8080 {
		t.Fatalf("expected port 8080, got %d", cfg.HTTPPort)
	}
	if cfg.HTTPHost != "0.0.0.0" {
		t.Fatalf("unexpected host: %s", cfg.HTTPHost)
	}
	if cfg.Migrations != "migrations" {
		t.Fatalf("unexpected migrations directory: %s", cfg.Migrations)
	}
	if cfg.SessionTTL != 720*time.Hour || !cfg.CookieSecure || cfg.AllowRegistration {
		t.Fatalf("unsafe auth defaults: %+v", cfg)
	}
}

func TestLoadAuthSettings(t *testing.T) {
	t.Setenv("SESSION_TTL", "12h")
	t.Setenv("COOKIE_SECURE", "false")
	t.Setenv("ALLOW_REGISTRATION", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SessionTTL != 12*time.Hour || cfg.CookieSecure || !cfg.AllowRegistration {
		t.Fatalf("settings not applied: %+v", cfg)
	}
}

func TestLoadRejectsInvalidAuthSettings(t *testing.T) {
	for name, value := range map[string]string{
		"SESSION_TTL": "30s", "COOKIE_SECURE": "maybe", "ALLOW_REGISTRATION": "yes please",
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv(name, value)
			if _, err := Load(); err == nil {
				t.Fatalf("%s=%q accepted", name, value)
			}
		})
	}
}

func TestLoadRejectsInvalidPort(t *testing.T) {
	t.Setenv("API_PORT", "70000")

	if _, err := Load(); err == nil {
		t.Fatal("expected invalid API_PORT to fail")
	}
}

func TestLoadOperationalSettings(t *testing.T) {
	t.Setenv("TRUSTED_PROXIES", " 10.0.0.0/8, 192.168.1.10 ")
	t.Setenv("LOGIN_RATE_LIMIT", "0")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("WORKER_TICK", "30s")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.TrustedProxies) != 2 || cfg.TrustedProxies[1] != "192.168.1.10" {
		t.Fatalf("trusted proxies = %v", cfg.TrustedProxies)
	}
	if cfg.LoginRateLimit != 0 || cfg.RegisterRateLimit != 5 || cfg.LogLevel != slog.LevelDebug || cfg.WorkerTick != 30*time.Second {
		t.Fatalf("settings not applied: %+v", cfg)
	}
}

func TestLoadRejectsInvalidOperationalSettings(t *testing.T) {
	for name, value := range map[string]string{
		"TRUSTED_PROXIES": "10.0.0.0/8,not-an-ip", "LOGIN_RATE_LIMIT": "-1",
		"REGISTER_RATE_LIMIT": "many", "LOG_LEVEL": "loud", "WORKER_TICK": "1s",
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv(name, value)
			if _, err := Load(); err == nil {
				t.Fatalf("%s=%q accepted", name, value)
			}
		})
	}
}

var crawlVars = []string{
	"CRAWL_CONCURRENCY", "CRAWL_HOST_INTERVAL", "CRAWL_VENDOR_CONCURRENCY", "CRAWL_USER_AGENT_URL", "CRAWL_ALLOW_PRIVATE",
	"DISCOVERY_INTERVAL", "COMPANY_SYNC_INTERVAL", "RESEARCH_INTERVAL", "JSONLD_MAX_PAGES",
	"MATCH_MIN_SCORE", "CLOSED_JOB_RETENTION",
}

func TestLoadCrawlDefaults(t *testing.T) {
	for _, v := range crawlVars {
		t.Setenv(v, "")
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	want := Crawl{
		Concurrency: 16, HostInterval: time.Second, VendorConcurrency: 4,
		UserAgentURL:      "https://github.com/0furkancolak/rolescout",
		DiscoveryInterval: 168 * time.Hour, CompanySyncInterval: 12 * time.Hour,
		ResearchInterval: 12 * time.Hour, JSONLDMaxPages: 200, MatchMinScore: 40,
		ClosedJobRetention: 2160 * time.Hour,
	}
	if cfg.Crawl != want {
		t.Fatalf("crawl defaults = %+v, want %+v", cfg.Crawl, want)
	}
}

func TestLoadCrawlSettings(t *testing.T) {
	t.Setenv("CRAWL_CONCURRENCY", "4")
	t.Setenv("CRAWL_HOST_INTERVAL", "2s")
	t.Setenv("CRAWL_VENDOR_CONCURRENCY", "2")
	t.Setenv("CRAWL_USER_AGENT_URL", "https://example.com/bot")
	t.Setenv("DISCOVERY_INTERVAL", "24h")
	t.Setenv("COMPANY_SYNC_INTERVAL", "6h")
	t.Setenv("RESEARCH_INTERVAL", "3h")
	t.Setenv("JSONLD_MAX_PAGES", "50")
	t.Setenv("MATCH_MIN_SCORE", "60")
	t.Setenv("CLOSED_JOB_RETENTION", "0")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	want := Crawl{
		Concurrency: 4, HostInterval: 2 * time.Second, VendorConcurrency: 2,
		UserAgentURL: "https://example.com/bot", DiscoveryInterval: 24 * time.Hour,
		CompanySyncInterval: 6 * time.Hour, ResearchInterval: 3 * time.Hour,
		JSONLDMaxPages: 50, MatchMinScore: 60, ClosedJobRetention: 0,
	}
	if cfg.Crawl != want {
		t.Fatalf("crawl = %+v, want %+v", cfg.Crawl, want)
	}
}

func TestLoadRejectsInvalidCrawlSettings(t *testing.T) {
	for name, value := range map[string]string{
		"CRAWL_CONCURRENCY":        "0",
		"CRAWL_HOST_INTERVAL":      "10ms",
		"CRAWL_VENDOR_CONCURRENCY": "0",
		"CRAWL_USER_AGENT_URL":     "not a url",
		"DISCOVERY_INTERVAL":       "5m",
		"COMPANY_SYNC_INTERVAL":    "abc",
		"RESEARCH_INTERVAL":        "10m",
		"JSONLD_MAX_PAGES":         "0",
		"MATCH_MIN_SCORE":          "101",
		"CLOSED_JOB_RETENTION":     "1h",
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv(name, value)
			if _, err := Load(); err == nil {
				t.Fatalf("%s=%s accepted", name, value)
			}
		})
	}
}
