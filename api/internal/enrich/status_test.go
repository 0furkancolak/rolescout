package enrich

import (
	"context"
	"testing"
	"time"

	"example.com/rolescout/api/internal/config"
	"example.com/rolescout/api/internal/domain"
	"example.com/rolescout/api/internal/llm"
	"example.com/rolescout/api/internal/llm/llmtest"
)

type fakeStats struct {
	stats  domain.EnrichmentStats
	recent map[string]int
}

func (f fakeStats) Stats(context.Context, string) (domain.EnrichmentStats, error) {
	return f.stats, nil
}
func (f fakeStats) RecentOK(_ context.Context, model string, _ time.Time) (int, error) {
	return f.recent[model], nil
}

func checker(t *testing.T, srv *llmtest.Server, mutate func(*config.LLM), stats fakeStats) *Checker {
	t.Helper()
	cfg := config.LLM{Enabled: true, BaseURL: srv.BaseURL(), Model: "primary", FallbackModel: "tiny", APIKey: "sekret", Timeout: 2 * time.Second}
	if mutate != nil {
		mutate(&cfg)
	}
	now := time.Date(2031, 1, 1, 12, 0, 0, 0, time.UTC)
	return &Checker{
		Config: cfg, Repo: stats, Now: func() time.Time { return now },
		Client: llm.NewClient(llm.Options{BaseURL: cfg.BaseURL, Model: cfg.Model, APIKey: cfg.APIKey, Timeout: cfg.Timeout}),
	}
}

func TestStatusWhenDisabledMakesNoNetworkCall(t *testing.T) {
	srv := llmtest.New(t, "primary")
	c := checker(t, srv, func(c *config.LLM) { c.Enabled = false }, fakeStats{})
	for _, probe := range []bool{false, true} {
		s, err := c.Status(context.Background(), probe)
		if err != nil || s.Enabled || s.Reachable || s.BaseHost != "" || s.Model != "" {
			t.Fatalf("disabled status = %+v, %v", s, err)
		}
	}
	if len(srv.Requests()) != 0 {
		t.Fatalf("disabled must not call the server: %d requests", len(srv.Requests()))
	}
}

func TestStatusWorking(t *testing.T) {
	srv := llmtest.New(t, "primary", "tiny")
	st := fakeStats{stats: domain.EnrichmentStats{Queue: 40, Enriched: 12, Failed: 1, AvgLatencyMs: 3200, LastError: "timeout"}}
	c := checker(t, srv, nil, st)
	s, err := c.Status(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if !s.Enabled || !s.Reachable || !s.ModelPresent || s.Degraded || s.Queue != 40 || s.Enriched != 12 || s.Failed != 1 || s.AvgLatencyMs != 3200 ||
		s.Model != "primary" || s.FallbackModel != "tiny" || s.LastError != "timeout" || s.LastCheckedAt == nil {
		t.Fatalf("status = %+v", s)
	}
	if srv.Chats() != 0 {
		t.Fatal("a plain status read must not run the model")
	}
	if s.BaseHost == "" || !s.Local {
		t.Fatalf("host = %q local = %v (the test server is on loopback)", s.BaseHost, s.Local)
	}
}

func TestCheckRunsTheModel(t *testing.T) {
	srv := llmtest.New(t, "primary")
	c := checker(t, srv, nil, fakeStats{})
	s, err := c.Status(context.Background(), true)
	if err != nil || !s.Reachable || srv.Chats() != 1 || s.ProbeMs <= 0 && s.ProbeOK == false {
		t.Fatalf("status = %+v chats = %d err = %v", s, srv.Chats(), err)
	}
	if !s.ProbeOK {
		t.Fatalf("probe should have succeeded: %+v", s)
	}
}

func TestStatusUnreachableAndModelMissing(t *testing.T) {
	srv := llmtest.New(t, "something-else")
	c := checker(t, srv, nil, fakeStats{})
	s, err := c.Status(context.Background(), false)
	if err != nil || !s.Reachable || s.ModelPresent {
		t.Fatalf("model missing: %+v, %v", s, err)
	}

	down := checker(t, srv, func(c *config.LLM) { c.BaseURL = "http://127.0.0.1:1/v1" }, fakeStats{})
	down.Client = llm.NewClient(llm.Options{BaseURL: "http://127.0.0.1:1/v1", Model: "primary", Timeout: time.Second})
	s, err = down.Status(context.Background(), false)
	if err != nil || s.Reachable || s.ModelPresent {
		t.Fatalf("unreachable: %+v, %v", s, err)
	}
}

func TestStatusDegradedWhenOnlyTheFallbackWorksRecently(t *testing.T) {
	srv := llmtest.New(t, "primary", "tiny")
	c := checker(t, srv, nil, fakeStats{recent: map[string]int{"tiny": 5, "primary": 0}})
	s, _ := c.Status(context.Background(), false)
	if !s.Degraded {
		t.Fatalf("status = %+v, want degraded", s)
	}
	c2 := checker(t, srv, nil, fakeStats{recent: map[string]int{"tiny": 5, "primary": 3}})
	if s2, _ := c2.Status(context.Background(), false); s2.Degraded {
		t.Fatalf("primary working again: %+v", s2)
	}
}

func TestLocalDetection(t *testing.T) {
	for host, want := range map[string]bool{
		"127.0.0.1:11434": true, "localhost:11434": true, "[::1]:11434": true, "host.docker.internal:11434": true,
		"192.168.1.20:8080": true, "10.0.0.5": true, "172.16.3.4:1": true,
		"api.example.com": false, "8.8.8.8:80": false, "gpu.corp.example:443": false,
	} {
		if got := isLocalHost(host); got != want {
			t.Errorf("isLocalHost(%q) = %v, want %v", host, got, want)
		}
	}
}

func TestStatusNeverExposesTheKeyOrPath(t *testing.T) {
	srv := llmtest.New(t, "primary")
	c := checker(t, srv, func(c *config.LLM) { c.BaseURL = srv.BaseURL() + "/secret/path?key=sekret" }, fakeStats{})
	c.Client = llm.NewClient(llm.Options{BaseURL: srv.BaseURL(), Model: "primary", Timeout: time.Second})
	s, _ := c.Status(context.Background(), false)
	if s.BaseHost == "" || containsAny(s.BaseHost, "secret", "sekret", "/", "?") {
		t.Fatalf("base host leaks: %q", s.BaseHost)
	}
}

func containsAny(s string, subs ...string) bool {
	for _, x := range subs {
		for i := 0; i+len(x) <= len(s); i++ {
			if s[i:i+len(x)] == x {
				return true
			}
		}
	}
	return false
}
