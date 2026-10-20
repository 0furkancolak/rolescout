package httptransport

import (
	"context"
	"testing"
	"time"

	"example.com/rolescout/api/internal/config"
	"example.com/rolescout/api/internal/crawlconfig"
)

type paceStore map[string]string

func (m paceStore) All(context.Context) (map[string]string, error) { return m, nil }
func (m paceStore) Apply(_ context.Context, changes map[string]*string) error {
	for k, v := range changes {
		if v == nil {
			delete(m, k)
		} else {
			m[k] = *v
		}
	}
	return nil
}

func TestCrawlPaceIsAdminOnlyAndSaved(t *testing.T) {
	pace := &crawlconfig.Resolver{
		Env:   config.Crawl{Concurrency: 128, HostInterval: 500 * time.Millisecond, VendorConcurrency: 16, VendorInterval: 100 * time.Millisecond},
		Store: paceStore{}, TTL: time.Nanosecond,
	}
	admin, user := adminApp(t, Deps{CrawlPace: pace})

	expectError(t, call(t, user, "GET", "/api/v1/admin/crawl/config", nil), 403, "forbidden")
	expectError(t, call(t, user, "PUT", "/api/v1/admin/crawl/config", map[string]any{"concurrency": 5}), 403, "forbidden")

	r := call(t, admin, "GET", "/api/v1/admin/crawl/config", nil)
	expectStatus(t, r, 200)
	if d := r.data(); d["concurrency"] != float64(128) || d["host_interval_ms"] != float64(500) {
		t.Fatalf("defaults: %v", d)
	}

	r = call(t, admin, "PUT", "/api/v1/admin/crawl/config", map[string]any{"concurrency": 400, "vendor_interval_ms": 50})
	expectStatus(t, r, 200)
	d := r.data()
	if d["concurrency"] != float64(400) || d["vendor_interval_ms"] != float64(50) || d["vendor_concurrency"] != float64(16) {
		t.Fatalf("saved: %v", d)
	}
	if def := d["defaults"].(map[string]any); def["concurrency"] != float64(128) {
		t.Fatalf("defaults must stay the environment's: %v", def)
	}

	expectStatus(t, call(t, admin, "PUT", "/api/v1/admin/crawl/config", map[string]any{"concurrency": 5000}), 422)
	r = call(t, admin, "PUT", "/api/v1/admin/crawl/config", map[string]any{"concurrency": 0})
	expectStatus(t, r, 200)
	if r.data()["concurrency"] != float64(128) {
		t.Fatalf("0 should bring the default back: %v", r.data())
	}
}
