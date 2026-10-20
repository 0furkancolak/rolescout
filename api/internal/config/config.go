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
	LLM      LLM
}

// LLM configures the optional local language model that enriches job
// descriptions. It is off unless Enabled is true.
type LLM struct {
	Enabled bool
	// BaseURL is an OpenAI-compatible endpoint ending in /v1.
	BaseURL       string
	Model         string
	FallbackModel string
	// APIKey is sent as a bearer token when set (llama.cpp --api-key).
	APIKey string
	// Background turns the worker's automatic reading of the whole catalog on
	// or off. Nil means automatic: on for a model on this machine or network,
	// off for a hosted service, where every job read costs money.
	Background *bool
	// Timeout bounds one model request.
	Timeout     time.Duration
	Concurrency int
	// Batch is how many jobs one worker tick sends to the model.
	Batch           int
	MaxInputChars   int
	MaxOutputTokens int
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
	// AllowPrivateAddresses lets the crawler reach loopback and private
	// networks (career sites on an intranet). Off by default.
	AllowPrivateAddresses bool
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

	llmCfg, err := loadLLM()
	if err != nil {
		return Config{}, err
	}

	return Config{
		Crawl:             crawl,
		LLM:               llmCfg,
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
	if c.Concurrency, err = rangeEnv("CRAWL_CONCURRENCY", 32, 1, 256); err != nil {
		return c, err
	}
	if c.VendorConcurrency, err = rangeEnv("CRAWL_VENDOR_CONCURRENCY", 8, 1, 32); err != nil {
		return c, err
	}
	if c.JSONLDMaxPages, err = rangeEnv("JSONLD_MAX_PAGES", 500, 1, 10000); err != nil {
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
	if c.AllowPrivateAddresses, err = boolEnv("CRAWL_ALLOW_PRIVATE", false); err != nil {
		return c, err
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

func loadLLM() (LLM, error) {
	c := LLM{
		BaseURL: "http://host.docker.internal:11434/v1", Model: "qwen3:4b-instruct-2507-q4_K_M",
	}
	var err error
	if c.Enabled, err = boolEnv("LLM_ENABLED", false); err != nil {
		return c, err
	}
	if raw := os.Getenv("LLM_BASE_URL"); raw != "" {
		u, err := url.Parse(strings.TrimSpace(raw))
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return c, fmt.Errorf("invalid LLM_BASE_URL: use an absolute http(s) URL such as http://127.0.0.1:11434/v1")
		}
		c.BaseURL = strings.TrimRight(strings.TrimSpace(raw), "/")
	}
	if raw := os.Getenv("LLM_MODEL"); raw != "" {
		c.Model = strings.TrimSpace(raw)
	}
	if c.Enabled && c.Model == "" {
		return c, fmt.Errorf("invalid LLM_MODEL: set a model name while LLM_ENABLED is true")
	}
	// An explicitly empty fallback turns degraded mode off.
	if raw, ok := os.LookupEnv("LLM_FALLBACK_MODEL"); ok {
		c.FallbackModel = strings.TrimSpace(raw)
	}
	c.APIKey = os.Getenv("LLM_API_KEY")
	c.Timeout = 60 * time.Second
	if raw := os.Getenv("LLM_TIMEOUT"); raw != "" {
		d, err := time.ParseDuration(raw)
		if err != nil || d < time.Second {
			return c, fmt.Errorf("invalid LLM_TIMEOUT: use a Go duration of at least 1s, such as 60s")
		}
		c.Timeout = d
	}
	if c.Concurrency, err = rangeEnv("LLM_CONCURRENCY", 1, 1, 4); err != nil {
		return c, err
	}
	if c.Batch, err = rangeEnv("LLM_BATCH", 25, 1, 500); err != nil {
		return c, err
	}
	if c.MaxInputChars, err = rangeEnv("LLM_MAX_INPUT_CHARS", 6000, 500, 50000); err != nil {
		return c, err
	}
	if c.MaxOutputTokens, err = rangeEnv("LLM_MAX_OUTPUT_TOKENS", 400, 50, 4000); err != nil {
		return c, err
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
