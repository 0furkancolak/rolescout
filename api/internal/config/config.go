package config

import (
	"fmt"
	"log/slog"
	"net"
	"net/url"
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
	Crawl    Crawl
}

// Crawl configures company discovery, job collection and research runs.
type Crawl struct {
	// Concurrency is how many hosts are crawled in parallel.
	Concurrency int
	// HostInterval spaces requests to one ordinary host.
	HostInterval time.Duration
	// VendorConcurrency is the parallel request limit for a job board API
	// host that serves many companies.
	VendorConcurrency int
	// UserAgentURL is linked from the User-Agent so site owners can find us.
	UserAgentURL        string
	DiscoveryInterval   time.Duration
	CompanySyncInterval time.Duration
	ResearchInterval    time.Duration
	JSONLDMaxPages      int
	MatchMinScore       int
	// ClosedJobRetention is how long closed jobs nobody saved or applied to
	// are kept; 0 keeps them forever.
	ClosedJobRetention time.Duration
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

	crawl, err := loadCrawl()
	if err != nil {
		return Config{}, err
	}

	return Config{
		Crawl:             crawl,
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

func loadCrawl() (Crawl, error) {
	c := Crawl{UserAgentURL: "https://github.com/0furkancolak/rolescout"}
	var err error
	if c.Concurrency, err = rangeEnv("CRAWL_CONCURRENCY", 16, 1, 256); err != nil {
		return c, err
	}
	if c.VendorConcurrency, err = rangeEnv("CRAWL_VENDOR_CONCURRENCY", 4, 1, 32); err != nil {
		return c, err
	}
	if c.JSONLDMaxPages, err = rangeEnv("JSONLD_MAX_PAGES", 200, 1, 10000); err != nil {
		return c, err
	}
	if c.MatchMinScore, err = rangeEnv("MATCH_MIN_SCORE", 40, 0, 100); err != nil {
		return c, err
	}
	durations := []struct {
		name     string
		dst      *time.Duration
		fallback time.Duration
		min      time.Duration
		example  string
	}{
		{"CRAWL_HOST_INTERVAL", &c.HostInterval, time.Second, 100 * time.Millisecond, "1s"},
		{"DISCOVERY_INTERVAL", &c.DiscoveryInterval, 168 * time.Hour, time.Hour, "168h"},
		{"COMPANY_SYNC_INTERVAL", &c.CompanySyncInterval, 12 * time.Hour, time.Hour, "12h"},
		{"RESEARCH_INTERVAL", &c.ResearchInterval, 12 * time.Hour, time.Hour, "12h"},
	}
	for _, d := range durations {
		*d.dst = d.fallback
		raw := os.Getenv(d.name)
		if raw == "" {
			continue
		}
		parsed, err := time.ParseDuration(raw)
		if err != nil || parsed < d.min {
			return c, fmt.Errorf("invalid %s: use a Go duration of at least %s, such as %s", d.name, d.min, d.example)
		}
		*d.dst = parsed
	}
	c.ClosedJobRetention = 2160 * time.Hour
	if raw := os.Getenv("CLOSED_JOB_RETENTION"); raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil || (parsed != 0 && parsed < 24*time.Hour) {
			return c, fmt.Errorf("invalid CLOSED_JOB_RETENTION: use 0 to keep closed jobs or a Go duration of at least 24h, such as 2160h")
		}
		c.ClosedJobRetention = parsed
	}
	if raw := os.Getenv("CRAWL_USER_AGENT_URL"); raw != "" {
		u, err := url.Parse(raw)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return c, fmt.Errorf("invalid CRAWL_USER_AGENT_URL: use an absolute http(s) URL")
		}
		c.UserAgentURL = raw
	}
	return c, nil
}

func rangeEnv(name string, fallback, min, max int) (int, error) {
	raw := os.Getenv(name)
	if raw == "" {
		return fallback, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < min || v > max {
		return 0, fmt.Errorf("invalid %s: use a whole number from %d to %d", name, min, max)
	}
	return v, nil
}
