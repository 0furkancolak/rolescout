// Package llmconfig resolves the language-model connection at run time: the
// environment gives the defaults and settings saved in the web app (stored in
// the database) override them, so an administrator can switch to a hosted
// OpenAI-compatible service with an API key without restarting anything.
package llmconfig

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"example.com/rolescout/api/internal/config"
	"example.com/rolescout/api/internal/llm"
)

// Setting keys in the app_settings table.
const (
	KeyEnabled    = "llm.enabled"
	KeyBaseURL    = "llm.base_url"
	KeyModel      = "llm.model"
	KeyAPIKey     = "llm.api_key"
	KeyBackground = "llm.background"
)

// Store reads and writes the stored settings.
type Store interface {
	All(ctx context.Context) (map[string]string, error)
	Apply(ctx context.Context, changes map[string]*string) error
}

// Resolver merges the environment defaults with the stored overrides. Reads
// are cached for a few seconds so every model call does not hit the database;
// Save drops the cache of this process, other processes see it within the TTL.
type Resolver struct {
	Env   config.LLM
	Store Store
	TTL   time.Duration

	mu     sync.Mutex
	cached config.LLM
	at     time.Time
	valid  bool
}

func (r *Resolver) ttl() time.Duration {
	if r.TTL > 0 {
		return r.TTL
	}
	return 5 * time.Second
}

// Current returns the effective configuration. If the database cannot be read
// it returns the last known one (or the environment on first use).
func (r *Resolver) Current(ctx context.Context) config.LLM {
	r.mu.Lock()
	if r.valid && time.Since(r.at) < r.ttl() {
		c := r.cached
		r.mu.Unlock()
		return c
	}
	r.mu.Unlock()

	cfg := r.Env
	if r.Store != nil {
		stored, err := r.Store.All(ctx)
		if err != nil {
			r.mu.Lock()
			defer r.mu.Unlock()
			if r.valid {
				return r.cached
			}
			return r.Env
		}
		cfg = apply(r.Env, stored)
	}
	r.mu.Lock()
	r.cached, r.at, r.valid = cfg, time.Now(), true
	r.mu.Unlock()
	return cfg
}

func apply(env config.LLM, s map[string]string) config.LLM {
	c := env
	if v, ok := s[KeyEnabled]; ok {
		c.Enabled = v == "true"
	}
	if v := s[KeyBaseURL]; v != "" {
		c.BaseURL = v
	}
	if v := s[KeyModel]; v != "" {
		c.Model = v
	}
	if v := s[KeyAPIKey]; v != "" {
		c.APIKey = v
	}
	if v, ok := s[KeyBackground]; ok {
		b := v == "true"
		c.Background = &b
	}
	return c
}

// Update is an edit of the stored connection. Nil fields stay as they are; an
// empty string clears the stored value and brings the environment default back.
type Update struct {
	Enabled *bool
	BaseURL *string
	Model   *string
	APIKey  *string
	// Background is the automatic reading of the catalog; ClearBackground
	// returns it to automatic.
	Background      *bool
	ClearBackground bool
}

// ErrInvalid matches every InvalidError.
var ErrInvalid = errors.New("invalid AI settings")

// InvalidError names the field and says, in words safe to show, what is wrong.
type InvalidError struct{ Field, Message string }

func (e *InvalidError) Error() string        { return e.Field + ": " + e.Message }
func (e *InvalidError) Is(target error) bool { return target == ErrInvalid }

// Validate checks an update before it is stored.
func (u Update) Validate() error {
	if u.BaseURL != nil && *u.BaseURL != "" {
		p, err := url.Parse(strings.TrimSpace(*u.BaseURL))
		if err != nil || (p.Scheme != "http" && p.Scheme != "https") || p.Host == "" || p.User != nil {
			return &InvalidError{"base_url", "must be an absolute http(s) URL such as https://api.openai.com/v1"}
		}
	}
	if u.Model != nil && len([]rune(*u.Model)) > 120 {
		return &InvalidError{"model", "must be at most 120 characters"}
	}
	if u.APIKey != nil && (len(*u.APIKey) > 512 || strings.ContainsAny(*u.APIKey, " \t\r\n")) {
		return &InvalidError{"api_key", "must be one token without spaces"}
	}
	return nil
}

// Save stores an update.
func (r *Resolver) Save(ctx context.Context, u Update) error {
	if err := u.Validate(); err != nil {
		return err
	}
	changes := map[string]*string{}
	put := func(key string, v *string, clean func(string) string) {
		if v == nil {
			return
		}
		s := clean(*v)
		if s == "" {
			changes[key] = nil
			return
		}
		changes[key] = &s
	}
	trim := func(s string) string { return strings.TrimSpace(s) }
	put(KeyBaseURL, u.BaseURL, func(s string) string { return strings.TrimRight(trim(s), "/") })
	put(KeyModel, u.Model, trim)
	put(KeyAPIKey, u.APIKey, trim)
	if u.Enabled != nil {
		v := strconv.FormatBool(*u.Enabled)
		changes[KeyEnabled] = &v
	}
	if u.Background != nil {
		v := strconv.FormatBool(*u.Background)
		changes[KeyBackground] = &v
	} else if u.ClearBackground {
		changes[KeyBackground] = nil
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
	out := make(map[string]bool, len(m))
	for k := range m {
		out[k] = true
	}
	return out, nil
}

// MaskKey shows enough of a key to recognise it, never the key.
func MaskKey(k string) string {
	if k == "" {
		return ""
	}
	if len(k) <= 8 {
		return "••••"
	}
	return "••••" + k[len(k)-4:]
}

// Client returns a client for the current configuration.
func (r *Resolver) Client(ctx context.Context) (*llm.Client, config.LLM) {
	c := r.Current(ctx)
	return llm.NewClient(llm.Options{BaseURL: c.BaseURL, Model: c.Model, APIKey: c.APIKey, Timeout: c.Timeout}), c
}

// Dynamic is an llm.Streamer that follows the current configuration on every
// call, and reports llm.ErrUnavailable while the model is switched off.
type Dynamic struct{ R *Resolver }

func (d Dynamic) Complete(ctx context.Context, req llm.Request) (llm.Response, error) {
	cl, cfg := d.R.Client(ctx)
	if !cfg.Enabled {
		return llm.Response{}, fmt.Errorf("%w: AI is switched off", llm.ErrUnavailable)
	}
	return cl.Complete(ctx, req)
}

func (d Dynamic) Stream(ctx context.Context, req llm.Request, onDelta func(string)) (llm.Response, error) {
	cl, cfg := d.R.Client(ctx)
	if !cfg.Enabled {
		return llm.Response{}, fmt.Errorf("%w: AI is switched off", llm.ErrUnavailable)
	}
	return cl.Stream(ctx, req, onDelta)
}
