package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	t.Setenv("API_HOST", "")
	t.Setenv("API_PORT", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("MIGRATIONS_DIR", "")

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
}

func TestLoadRejectsInvalidPort(t *testing.T) {
	t.Setenv("API_PORT", "70000")

	if _, err := Load(); err == nil {
		t.Fatal("expected invalid API_PORT to fail")
	}
}
