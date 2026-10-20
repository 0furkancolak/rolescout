package crawl

import (
	"testing"
	"time"
)

func TestLiveTuningChangesTheLimiterOfLaterRequests(t *testing.T) {
	live := NewLive(Tuning{Concurrency: 8, HostInterval: time.Second, VendorConcurrency: 2, VendorInterval: time.Second})
	c := New(Options{UserAgent: "RoleScout/test", Live: live, VendorHosts: []string{"api.vendor.test"}, AllowPrivate: true})
	first := c.limiter("api.vendor.test")
	if cap(first.slots) != 2 {
		t.Fatalf("vendor slots = %d, want 2", cap(first.slots))
	}
	if c.limiter("api.vendor.test") != first {
		t.Fatal("limiter should be reused while the tuning is unchanged")
	}
	live.Set(Tuning{Concurrency: 8, HostInterval: 200 * time.Millisecond, VendorConcurrency: 9, VendorInterval: 20 * time.Millisecond})
	second := c.limiter("api.vendor.test")
	if cap(second.slots) != 9 || second.interval != 20*time.Millisecond {
		t.Fatalf("after change: slots %d interval %v", cap(second.slots), second.interval)
	}
	if h := c.limiter("shop.example"); h.interval != 200*time.Millisecond {
		t.Fatalf("ordinary host interval = %v", h.interval)
	}
}
