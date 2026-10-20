package config

import (
	"log/slog"
	"os"
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
		Concurrency: 32, HostInterval: time.Second, VendorConcurrency: 8,
		UserAgentURL:      "https://github.com/0furkancolak/rolescout",
		DiscoveryInterval: 168 * time.Hour, CompanySyncInterval: 12 * time.Hour,
		ResearchInterval: 12 * time.Hour, JSONLDMaxPages: 500, MatchMinScore: 40,
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

var llmVars = []string{
	"LLM_ENABLED", "LLM_BASE_URL", "LLM_MODEL", "LLM_FALLBACK_MODEL", "LLM_API_KEY", "LLM_TIMEOUT",
	"LLM_CONCURRENCY", "LLM_BATCH", "LLM_MAX_INPUT_CHARS", "LLM_MAX_OUTPUT_TOKENS",
}

// clearLLM unsets every LLM variable for the test (restored afterwards), so a
// default really is a default; an empty LLM_FALLBACK_MODEL means "none".
func clearLLM(t *testing.T) {
	t.Helper()
	for _, v := range llmVars {
		t.Setenv(v, "")
		os.Unsetenv(v)
	}
}

func TestLoadLLMDefaultsToOff(t *testing.T) {
	clearLLM(t)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	want := LLM{
		BaseURL: "http://host.docker.internal:11434/v1", Model: "qwen3:4b-instruct-2507-q4_K_M",
		Timeout: 60 * time.Second, Concurrency: 1, Batch: 25,
		MaxInputChars: 6000, MaxOutputTokens: 400,
	}
	if cfg.LLM != want {
		t.Fatalf("llm defaults = %+v, want %+v", cfg.LLM, want)
	}
}

func TestLoadLLMSettings(t *testing.T) {
	clearLLM(t)
	t.Setenv("LLM_ENABLED", "true")
	t.Setenv("LLM_BASE_URL", "http://127.0.0.1:8081/v1")
	t.Setenv("LLM_MODEL", "tiny")
	t.Setenv("LLM_FALLBACK_MODEL", "")
	t.Setenv("LLM_API_KEY", "k")
	t.Setenv("LLM_TIMEOUT", "5s")
	t.Setenv("LLM_CONCURRENCY", "2")
	t.Setenv("LLM_BATCH", "100")
	t.Setenv("LLM_MAX_INPUT_CHARS", "3000")
	t.Setenv("LLM_MAX_OUTPUT_TOKENS", "200")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	want := LLM{
		Enabled: true, BaseURL: "http://127.0.0.1:8081/v1", Model: "tiny", APIKey: "k", Timeout: 5 * time.Second,
		Concurrency: 2, Batch: 100, MaxInputChars: 3000, MaxOutputTokens: 200,
	}
	if cfg.LLM != want {
		t.Fatalf("llm = %+v, want %+v", cfg.LLM, want)
	}
}

func TestLoadRejectsInvalidLLMSettings(t *testing.T) {
	for name, env := range map[string]map[string]string{
		"relative url":      {"LLM_BASE_URL": "localhost:11434/v1"},
		"not http":          {"LLM_BASE_URL": "ftp://localhost/v1"},
		"no host":           {"LLM_BASE_URL": "http:///v1"},
		"concurrency 9":     {"LLM_CONCURRENCY": "9"},
		"concurrency 0":     {"LLM_CONCURRENCY": "0"},
		"batch 501":         {"LLM_BATCH": "501"},
		"batch 0":           {"LLM_BATCH": "0"},
		"zero timeout":      {"LLM_TIMEOUT": "0s"},
		"bad timeout":       {"LLM_TIMEOUT": "soon"},
		"tiny input":        {"LLM_MAX_INPUT_CHARS": "10"},
		"output too big":    {"LLM_MAX_OUTPUT_TOKENS": "100000"},
		"enabled no model":  {"LLM_ENABLED": "true", "LLM_MODEL": " "},
		"bad enabled value": {"LLM_ENABLED": "maybe"},
	} {
		t.Run(name, func(t *testing.T) {
			clearLLM(t)
			for k, v := range env {
				t.Setenv(k, v)
			}
			if _, err := Load(); err == nil {
				t.Fatalf("Load accepted %v", env)
			}
		})
	}
}

func TestEmptyModelIsFineWhileDisabled(t *testing.T) {
	clearLLM(t)
	t.Setenv("LLM_MODEL", " ")
	if _, err := Load(); err != nil {
		t.Fatalf("disabled with empty model: %v", err)
	}
}
