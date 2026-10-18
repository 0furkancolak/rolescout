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
	Stats(ctx context.Context, model string) (domain.EnrichmentStats, error)
	RecentOK(ctx context.Context, model string, since time.Time) (int, error)
}

// Status is the local model's state as shown to an administrator. It never
// carries the API key or the URL path.
type Status struct {
	Enabled       bool
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
	Repo   StatsRepository
	Now    func() time.Time

	mu       sync.Mutex
	last     Status
	lastTime time.Time
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
	if !c.Config.Enabled {
		return Status{}, nil
	}
	c.mu.Lock()
	if !probe && !c.lastTime.IsZero() && c.now().Sub(c.lastTime) < statusCacheTTL {
		cached := c.last
		c.mu.Unlock()
		return cached, nil
	}
	c.mu.Unlock() // never hold the lock across network or database calls

	now := c.now()
	s := Status{
		Enabled: true, BaseHost: hostOnly(c.Config.BaseURL), Model: c.Config.Model, FallbackModel: c.Config.FallbackModel,
		Local: isLocalHost(hostOnly(c.Config.BaseURL)), LastCheckedAt: &now,
	}
	if h, err := c.Client.Health(ctx); err == nil {
		s.Reachable, s.ModelPresent = h.Reachable, h.ModelPresent
	}
	if probe && s.Reachable && s.ModelPresent {
		if resp, err := c.Client.Probe(ctx); err == nil {
			s.ProbeOK, s.ProbeMs = true, int(resp.Latency/time.Millisecond)
		}
	}
	if st, err := c.Repo.Stats(ctx, c.Config.Model); err == nil {
		s.Queue, s.Enriched, s.Failed, s.AvgLatencyMs, s.LastError = st.Queue, st.Enriched, st.Failed, st.AvgLatencyMs, st.LastError
	} else {
		return Status{}, err
	}
	if c.Config.FallbackModel != "" {
		since := now.Add(-15 * time.Minute)
		fb, _ := c.Repo.RecentOK(ctx, c.Config.FallbackModel, since)
		main, _ := c.Repo.RecentOK(ctx, c.Config.Model, since)
		s.Degraded = fb > 0 && main == 0
	}
	c.mu.Lock()
	c.last, c.lastTime = s, now
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
