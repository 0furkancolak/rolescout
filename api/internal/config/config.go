package config

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HTTPHost    string
	HTTPPort    int
	DatabaseURL string
	Migrations  string

	// SessionTTL is the absolute lifetime of a login session.
	SessionTTL time.Duration
	// CookieSecure marks the session cookie Secure; disable only for local HTTP.
	CookieSecure bool
	// AllowRegistration permits sign-ups after the first account exists.
	AllowRegistration bool
	// WorkerTick is how often the worker checks for sources due to sync.
	WorkerTick time.Duration
	// TrustedProxies lists proxy IPs or CIDR ranges whose X-Real-IP header is
	// trusted for the client address (used by rate limiting).
	TrustedProxies []string
	// LoginRateLimit and RegisterRateLimit cap failed sign-ins per 15 minutes
	// and sign-ups per hour from one IP address; 0 disables the limit.
	LoginRateLimit    int
	RegisterRateLimit int
	// LogLevel is debug, info, warn or error.
	LogLevel slog.Level
}

func Load() (Config, error) {
	port := 8080
	if raw := os.Getenv("API_PORT"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 65535 {
			return Config{}, fmt.Errorf("invalid API_PORT")
		}
		port = parsed
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://rolescout:change-me-locally@127.0.0.1:5432/rolescout?sslmode=disable"
	}

	host := os.Getenv("API_HOST")
	if host == "" {
		host = "0.0.0.0"
	}

	migrations := os.Getenv("MIGRATIONS_DIR")
	if migrations == "" {
		migrations = "migrations"
	}

	sessionTTL := 30 * 24 * time.Hour
	if raw := os.Getenv("SESSION_TTL"); raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil || parsed < time.Minute {
			return Config{}, fmt.Errorf("invalid SESSION_TTL: use a Go duration of at least 1m, such as 720h")
		}
		sessionTTL = parsed
	}

	workerTick := time.Minute
	if raw := os.Getenv("WORKER_TICK"); raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil || parsed < 10*time.Second {
			return Config{}, fmt.Errorf("invalid WORKER_TICK: use a Go duration of at least 10s, such as 1m")
		}
		workerTick = parsed
	}

	var trustedProxies []string
	for _, p := range strings.Split(os.Getenv("TRUSTED_PROXIES"), ",") {
		if p = strings.TrimSpace(p); p == "" {
			continue
		}
		if net.ParseIP(p) == nil {
			if _, _, err := net.ParseCIDR(p); err != nil {
				return Config{}, fmt.Errorf("invalid TRUSTED_PROXIES entry %q: use IP addresses or CIDR ranges", p)
			}
		}
		trustedProxies = append(trustedProxies, p)
	}

	loginLimit, err := intEnv("LOGIN_RATE_LIMIT", 10)
	if err != nil {
		return Config{}, err
	}
	registerLimit, err := intEnv("REGISTER_RATE_LIMIT", 5)
	if err != nil {
		return Config{}, err
	}

	var logLevel slog.Level
	if raw := os.Getenv("LOG_LEVEL"); raw != "" {
		if err := logLevel.UnmarshalText([]byte(raw)); err != nil {
			return Config{}, fmt.Errorf("invalid LOG_LEVEL: use debug, info, warn or error")
		}
	}

	cookieSecure, err := boolEnv("COOKIE_SECURE", true)
	if err != nil {
		return Config{}, err
	}
	allowRegistration, err := boolEnv("ALLOW_REGISTRATION", false)
	if err != nil {
		return Config{}, err
	}

	return Config{
		HTTPHost:          host,
		HTTPPort:          port,
		DatabaseURL:       dbURL,
		Migrations:        migrations,
		SessionTTL:        sessionTTL,
		CookieSecure:      cookieSecure,
		AllowRegistration: allowRegistration,
		WorkerTick:        workerTick,
		TrustedProxies:    trustedProxies,
		LoginRateLimit:    loginLimit,
		RegisterRateLimit: registerLimit,
		LogLevel:          logLevel,
	}, nil
}

func intEnv(name string, fallback int) (int, error) {
	raw := os.Getenv(name)
	if raw == "" {
		return fallback, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 0 {
		return 0, fmt.Errorf("invalid %s: use a whole number of at least 0", name)
	}
	return v, nil
}

func boolEnv(name string, fallback bool) (bool, error) {
	raw := os.Getenv(name)
	if raw == "" {
		return fallback, nil
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("invalid %s: use true or false", name)
	}
	return v, nil
}
