package crawlconfig

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"example.com/rolescout/api/internal/config"
	"example.com/rolescout/api/internal/crawl"
)

type memStore map[string]string

// memMu guards every memStore: the live-follow test reads it from a goroutine.
var memMu sync.Mutex

func (m memStore) All(context.Context) (map[string]string, error) {
	memMu.Lock()
	defer memMu.Unlock()
	out := map[string]string{}
	for k, v := range m {
		out[k] = v
	}
	return out, nil
}

func (m memStore) Apply(_ context.Context, changes map[string]*string) error {
	memMu.Lock()
	defer memMu.Unlock()
	for k, v := range changes {
		if v == nil {
			delete(m, k)
		} else {
			m[k] = *v
		}
	}
	return nil
}

func newResolver(store Store) *Resolver {
	return &Resolver{Env: config.Crawl{Concurrency: 128, HostInterval: 500 * time.Millisecond, VendorConcurrency: 16, VendorInterval: 100 * time.Millisecond}, Store: store}
}

func TestEnvironmentIsTheDefault(t *testing.T) {
	got := newResolver(memStore{}).Current(context.Background())
	want := crawl.Tuning{Concurrency: 128, HostInterval: 500 * time.Millisecond, VendorConcurrency: 16, VendorInterval: 100 * time.Millisecond}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestSavedValuesOverrideAndClearBringsDefaultsBack(t *testing.T) {
	r := newResolver(memStore{})
	ctx := context.Background()
	n, host := 400, 250
	if err := r.Save(ctx, Update{Concurrency: &n, HostIntervalMS: &host}); err != nil {
		t.Fatal(err)
	}
	got := r.Current(ctx)
	if got.Concurrency != 400 || got.HostInterval != 250*time.Millisecond || got.VendorConcurrency != 16 {
		t.Fatalf("after save: %+v", got)
	}
	stored, _ := r.Stored(ctx)
	if !stored[KeyConcurrency] || stored[KeyVendorConcurrency] {
		t.Fatalf("stored: %v", stored)
	}
	zero := 0
	if err := r.Save(ctx, Update{Concurrency: &zero}); err != nil {
		t.Fatal(err)
	}
	if got := r.Current(ctx); got.Concurrency != 128 {
		t.Fatalf("after clear: %+v", got)
	}
}

func TestOutOfRangeValuesAreRejected(t *testing.T) {
	r := newResolver(memStore{})
	for name, u := range map[string]Update{
		"too many":       {Concurrency: ptr(MaxConcurrency + 1)},
		"negative":       {Concurrency: ptr(-1)},
		"too fast host":  {HostIntervalMS: ptr(MinHostIntervalMS - 1)},
		"vendor too big": {VendorConcurrency: ptr(MaxVendorConcurrency + 1)},
		"vendor fast":    {VendorIntervalMS: ptr(1)},
	} {
		if err := r.Save(context.Background(), u); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestBadStoredValuesAreIgnored(t *testing.T) {
	r := newResolver(memStore{KeyConcurrency: "abc", KeyHostIntervalMS: "5"})
	if got := r.Current(context.Background()); got.Concurrency != 128 || got.HostInterval != 500*time.Millisecond {
		t.Fatalf("got %+v", got)
	}
}

func TestFollowUpdatesLive(t *testing.T) {
	store := memStore{}
	r := newResolver(store)
	r.TTL = time.Millisecond
	live := crawl.NewLive(r.Defaults())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go r.Follow(ctx, live, 5*time.Millisecond)
	_ = store.Apply(ctx, map[string]*string{KeyConcurrency: ptrStr("512")})
	deadline := time.Now().Add(2 * time.Second)
	for live.Get().Concurrency != 512 {
		if time.Now().After(deadline) {
			t.Fatal("live tuning never followed the saved value")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func ptr(n int) *int { return &n }

func ptrStr(s string) *string { return &s }
