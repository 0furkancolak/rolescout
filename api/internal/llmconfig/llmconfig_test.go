package llmconfig

import (
	"context"
	"errors"
	"sync"
	"testing"

	"example.com/rolescout/api/internal/config"
	"example.com/rolescout/api/internal/llm"
)

type memStore struct {
	mu    sync.Mutex
	m     map[string]string
	reads int
}

func (s *memStore) All(context.Context) (map[string]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reads++
	out := map[string]string{}
	for k, v := range s.m {
		out[k] = v
	}
	return out, nil
}

func (s *memStore) Apply(_ context.Context, ch map[string]*string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.m == nil {
		s.m = map[string]string{}
	}
	for k, v := range ch {
		if v == nil {
			delete(s.m, k)
		} else {
			s.m[k] = *v
		}
	}
	return nil
}

func str(s string) *string { return &s }
func yes() *bool           { b := true; return &b }

func envCfg() config.LLM {
	return config.LLM{BaseURL: "http://host.docker.internal:11434/v1", Model: "qwen"}
}

func TestStoredSettingsOverrideTheEnvironment(t *testing.T) {
	r := &Resolver{Env: envCfg(), Store: &memStore{}}
	ctx := context.Background()
	if r.Current(ctx).Enabled {
		t.Fatal("AI must stay off until someone turns it on")
	}
	err := r.Save(ctx, Update{Enabled: yes(), BaseURL: str(" https://api.openai.com/v1/ "), Model: str("gpt-x"), APIKey: str("sk-secret-1234")})
	if err != nil {
		t.Fatal(err)
	}
	c := r.Current(ctx)
	if !c.Enabled || c.BaseURL != "https://api.openai.com/v1" || c.Model != "gpt-x" || c.APIKey != "sk-secret-1234" {
		t.Fatalf("config = %+v", c)
	}
	// Clearing a value brings the environment default back.
	if err := r.Save(ctx, Update{BaseURL: str(""), Model: str(""), APIKey: str("")}); err != nil {
		t.Fatal(err)
	}
	c = r.Current(ctx)
	if c.BaseURL != envCfg().BaseURL || c.Model != "qwen" || c.APIKey != "" || !c.Enabled {
		t.Fatalf("after clear: %+v", c)
	}
}

func TestOmittedFieldsStayAsTheyAre(t *testing.T) {
	r := &Resolver{Env: envCfg(), Store: &memStore{}}
	ctx := context.Background()
	_ = r.Save(ctx, Update{APIKey: str("sk-keep-me-1234"), Model: str("m1")})
	_ = r.Save(ctx, Update{Model: str("m2")})
	c := r.Current(ctx)
	if c.APIKey != "sk-keep-me-1234" || c.Model != "m2" {
		t.Fatalf("config = %+v", c)
	}
}

func TestReadsAreCachedUntilSave(t *testing.T) {
	st := &memStore{}
	r := &Resolver{Env: envCfg(), Store: st}
	ctx := context.Background()
	r.Current(ctx)
	r.Current(ctx)
	if st.reads != 1 {
		t.Fatalf("reads = %d, want 1", st.reads)
	}
	_ = r.Save(ctx, Update{Enabled: yes()})
	if !r.Current(ctx).Enabled || st.reads != 2 {
		t.Fatalf("a save must drop the cache (reads %d)", st.reads)
	}
}

func TestValidation(t *testing.T) {
	for name, u := range map[string]Update{
		"relative url":   {BaseURL: str("api.openai.com/v1")},
		"ftp":            {BaseURL: str("ftp://example.com")},
		"credentials":    {BaseURL: str("https://user:pw@example.com/v1")},
		"key with space": {APIKey: str("sk abc")},
	} {
		err := u.Validate()
		var inv *InvalidError
		if !errors.Is(err, ErrInvalid) || !errors.As(err, &inv) || inv.Field == "" {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	if err := (Update{BaseURL: str("https://api.openai.com/v1"), APIKey: str("sk-ok")}).Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestMaskKeyNeverShowsTheKey(t *testing.T) {
	if got := MaskKey("sk-abcdefgh1234"); got != "••••1234" {
		t.Fatalf("mask = %q", got)
	}
	if got := MaskKey("short"); got != "••••" {
		t.Fatalf("mask = %q", got)
	}
	if MaskKey("") != "" {
		t.Fatal("no key, no hint")
	}
}

func TestBackgroundChoice(t *testing.T) {
	r := &Resolver{Env: envCfg(), Store: &memStore{}}
	ctx := context.Background()
	if r.Current(ctx).Background != nil {
		t.Fatal("automatic by default")
	}
	b := false
	_ = r.Save(ctx, Update{Background: &b})
	if c := r.Current(ctx); c.Background == nil || *c.Background {
		t.Fatalf("background = %v", c.Background)
	}
	_ = r.Save(ctx, Update{ClearBackground: true})
	if r.Current(ctx).Background != nil {
		t.Fatal("reset must return to automatic")
	}
}

func TestDynamicClientRefusesWhileOff(t *testing.T) {
	r := &Resolver{Env: envCfg(), Store: &memStore{}}
	_, err := Dynamic{R: r}.Complete(context.Background(), llm.Request{User: "x"})
	if !errors.Is(err, llm.ErrUnavailable) {
		t.Fatalf("err = %v", err)
	}
}
