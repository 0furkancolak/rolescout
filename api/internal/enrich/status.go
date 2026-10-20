package enrich

import (
	"context"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	"example.com/rolescout/api/internal/config"
	"example.com/rolescout/api/internal/domain"
	"example.com/rolescout/api/internal/llm"
)

// StatsRepository is what the status screen reads from the database.
type StatsRepository interface {
	Stats(ctx context.Context, model string, promptVersion, schemaVersion int) (domain.EnrichmentStats, error)
	RecentOK(ctx context.Context, model string, since time.Time) (int, error)
}

// Status is the local model's state as shown to an administrator. It never
// carries the API key or the URL path.
type Status struct {
	Enabled bool
	// Background is whether the worker reads the catalog on its own.
	Background    bool
	HasAPIKey     bool
	BaseHost      string
	Model         string
	FallbackModel string
	Reachable     bool
	ModelPresent  bool
	Degraded      bool
	Local         bool
	Queue         int
	Enriched      int
	Failed        int
	AvgLatencyMs  int
	LastError     string
	LastCheckedAt *time.Time
	// ProbeOK and ProbeMs describe the one-token completion of a check.
	ProbeOK bool
	ProbeMs int
}

// Checker answers the status endpoints. A plain read lists the server's models
// (cheap); a check also runs the model once.
type Checker struct {
	Config config.LLM
	Client *llm.Client
	// Resolve, when set, supplies the live configuration and client instead of
	// Config and Client.
	Resolve func(ctx context.Context) (*llm.Client, config.LLM)
	Repo    StatsRepository
	Now     func() time.Time

	mu       sync.Mutex
	last     Status
	lastTime time.Time
	// lastModel is the model and URL the cached status was taken for.
	lastModel string
}

const statusCacheTTL = 5 * time.Second

func (c *Checker) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

// Status returns the current state. With probe it also runs a one-token
// completion; without it a read is cached for a few seconds.
func (c *Checker) Status(ctx context.Context, probe bool) (Status, error) {
	cfg, client := c.Config, c.Client
	if c.Resolve != nil {
		client, cfg = c.Resolve(ctx)
	}
	if !cfg.Enabled {
		return Status{}, nil
	}
	c.mu.Lock()
	if !probe && !c.lastTime.IsZero() && c.now().Sub(c.lastTime) < statusCacheTTL && c.lastModel == cfg.Model+cfg.BaseURL {
		cached := c.last
		c.mu.Unlock()
		return cached, nil
	}
	c.mu.Unlock() // never hold the lock across network or database calls

	now := c.now()
	s := Status{
		Enabled: true, Background: BackgroundOn(cfg), HasAPIKey: cfg.APIKey != "", BaseHost: hostOnly(cfg.BaseURL), Model: cfg.Model, FallbackModel: cfg.FallbackModel,
		Local: isLocalHost(hostOnly(cfg.BaseURL)), LastCheckedAt: &now,
	}
	if h, err := client.Health(ctx); err == nil {
		s.Reachable, s.ModelPresent = h.Reachable, h.ModelPresent
	}
	if probe && s.Reachable && s.ModelPresent {
		if resp, err := client.Probe(ctx); err == nil {
			s.ProbeOK, s.ProbeMs = true, int(resp.Latency/time.Millisecond)
		}
	}
	if st, err := c.Repo.Stats(ctx, cfg.Model, llm.PromptVersion, llm.SchemaVersion); err == nil {
		s.Queue, s.Enriched, s.Failed, s.AvgLatencyMs, s.LastError = st.Queue, st.Enriched, st.Failed, st.AvgLatencyMs, st.LastError
	} else {
		return Status{}, err
	}
	if cfg.FallbackModel != "" {
		since := now.Add(-15 * time.Minute)
		fb, _ := c.Repo.RecentOK(ctx, cfg.FallbackModel, since)
		main, _ := c.Repo.RecentOK(ctx, cfg.Model, since)
		s.Degraded = fb > 0 && main == 0
	}
	c.mu.Lock()
	c.last, c.lastTime, c.lastModel = s, now, cfg.Model+cfg.BaseURL
	c.mu.Unlock()
	return s, nil
}

// hostOnly returns host[:port] of a URL, dropping scheme, path, query and any
// credentials.
func hostOnly(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	return u.Host
}

// isLocalHost reports whether a host[:port] is this machine or a private
// network, where job text stays within the user's own network.
func isLocalHost(hostport string) bool {
	host := hostport
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	switch strings.ToLower(host) {
	case "localhost", "host.docker.internal", "host.containers.internal":
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast()
	}
	return false
}

// IsLocalURL reports whether a base URL points at this machine or a private network.
func IsLocalURL(raw string) bool { return isLocalHost(hostOnly(raw)) }
