// Package crawlconfig resolves how fast the crawl runs. The environment gives
// the defaults and values saved in the web app (stored in the database)
// override them, so an administrator can speed the research up or slow it down
// without restarting the worker.
package crawlconfig

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"time"

	"example.com/rolescout/api/internal/config"
	"example.com/rolescout/api/internal/crawl"
)

// Setting keys in the app_settings table.
const (
	KeyConcurrency       = "crawl.concurrency"
	KeyHostIntervalMS    = "crawl.host_interval_ms"
	KeyVendorConcurrency = "crawl.vendor_concurrency"
	KeyVendorIntervalMS  = "crawl.vendor_interval_ms"
)

// Allowed ranges.
const (
	MaxConcurrency       = 1024
	MaxVendorConcurrency = 128
	MinHostIntervalMS    = 100
	MaxHostIntervalMS    = 60_000
	MinVendorIntervalMS  = 10
	MaxVendorIntervalMS  = 10_000
)

// Store reads and writes the stored settings.
type Store interface {
	All(ctx context.Context) (map[string]string, error)
	Apply(ctx context.Context, changes map[string]*string) error
}

// Resolver merges the environment defaults with the stored overrides. Reads
// are cached for a few seconds; Save drops the cache of this process.
type Resolver struct {
	Env   config.Crawl
	Store Store
	TTL   time.Duration

	mu     sync.Mutex
	cached crawl.Tuning
	at     time.Time
	valid  bool
}

// Defaults is the tuning the environment asks for.
func (r *Resolver) Defaults() crawl.Tuning {
	return crawl.Tuning{
		Concurrency: r.Env.Concurrency, HostInterval: r.Env.HostInterval,
		VendorConcurrency: r.Env.VendorConcurrency, VendorInterval: r.Env.VendorInterval,
	}
}

func (r *Resolver) ttl() time.Duration {
	if r.TTL > 0 {
		return r.TTL
	}
	return 5 * time.Second
}

// Current returns the effective tuning. If the database cannot be read it
// returns the last known one (or the environment on first use).
func (r *Resolver) Current(ctx context.Context) crawl.Tuning {
	r.mu.Lock()
	if r.valid && time.Since(r.at) < r.ttl() {
		t := r.cached
		r.mu.Unlock()
		return t
	}
	r.mu.Unlock()

	t := r.Defaults()
	if r.Store != nil {
		stored, err := r.Store.All(ctx)
		if err != nil {
			r.mu.Lock()
			defer r.mu.Unlock()
			if r.valid {
				return r.cached
			}
			return t
		}
		t = apply(t, stored)
	}
	r.mu.Lock()
	r.cached, r.at, r.valid = t, time.Now(), true
	r.mu.Unlock()
	return t
}

func apply(t crawl.Tuning, s map[string]string) crawl.Tuning {
	num := func(key string, lo, hi int) (int, bool) {
		v, err := strconv.Atoi(s[key])
		return v, err == nil && v >= lo && v <= hi
	}
	if v, ok := num(KeyConcurrency, 1, MaxConcurrency); ok {
		t.Concurrency = v
	}
	if v, ok := num(KeyHostIntervalMS, MinHostIntervalMS, MaxHostIntervalMS); ok {
		t.HostInterval = time.Duration(v) * time.Millisecond
	}
	if v, ok := num(KeyVendorConcurrency, 1, MaxVendorConcurrency); ok {
		t.VendorConcurrency = v
	}
	if v, ok := num(KeyVendorIntervalMS, MinVendorIntervalMS, MaxVendorIntervalMS); ok {
		t.VendorInterval = time.Duration(v) * time.Millisecond
	}
	return t
}

// Update is an edit of the stored tuning. Nil fields stay as they are; 0
// clears the stored value and brings the environment default back.
type Update struct {
	Concurrency       *int
	HostIntervalMS    *int
	VendorConcurrency *int
	VendorIntervalMS  *int
}

// ErrInvalid matches every InvalidError.
var ErrInvalid = errors.New("invalid crawl settings")

// InvalidError names the field and says, in words safe to show, what is wrong.
type InvalidError struct{ Field, Message string }

func (e *InvalidError) Error() string        { return e.Field + ": " + e.Message }
func (e *InvalidError) Is(target error) bool { return target == ErrInvalid }

type field struct {
	key    string
	name   string
	value  *int
	lo, hi int
}

func (u Update) fields() []field {
	return []field{
		{KeyConcurrency, "concurrency", u.Concurrency, 1, MaxConcurrency},
		{KeyHostIntervalMS, "host_interval_ms", u.HostIntervalMS, MinHostIntervalMS, MaxHostIntervalMS},
		{KeyVendorConcurrency, "vendor_concurrency", u.VendorConcurrency, 1, MaxVendorConcurrency},
		{KeyVendorIntervalMS, "vendor_interval_ms", u.VendorIntervalMS, MinVendorIntervalMS, MaxVendorIntervalMS},
	}
}

// Validate checks an update before it is stored.
func (u Update) Validate() error {
	for _, f := range u.fields() {
		if f.value != nil && *f.value != 0 && (*f.value < f.lo || *f.value > f.hi) {
			return &InvalidError{f.name, "must be a whole number from " + strconv.Itoa(f.lo) + " to " + strconv.Itoa(f.hi)}
		}
	}
	return nil
}

// Save stores an update.
func (r *Resolver) Save(ctx context.Context, u Update) error {
	if err := u.Validate(); err != nil {
		return err
	}
	changes := map[string]*string{}
	for _, f := range u.fields() {
		switch {
		case f.value == nil:
		case *f.value == 0:
			changes[f.key] = nil
		default:
			s := strconv.Itoa(*f.value)
			changes[f.key] = &s
		}
	}
	if len(changes) == 0 {
		return nil
	}
	if err := r.Store.Apply(ctx, changes); err != nil {
		return err
	}
	r.mu.Lock()
	r.valid = false
	r.mu.Unlock()
	return nil
}

// Stored reports which keys have an override in the database.
func (r *Resolver) Stored(ctx context.Context) (map[string]bool, error) {
	if r.Store == nil {
		return map[string]bool{}, nil
	}
	m, err := r.Store.All(ctx)
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for k := range m {
		out[k] = true
	}
	return out, nil
}

// Follow keeps live in step with the resolver until ctx is cancelled.
func (r *Resolver) Follow(ctx context.Context, live *crawl.Live, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			live.Set(r.Current(ctx))
		}
	}
}
