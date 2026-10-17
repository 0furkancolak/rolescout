package config

import (
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
