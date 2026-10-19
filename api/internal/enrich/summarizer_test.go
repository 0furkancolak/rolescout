package enrich

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"example.com/rolescout/api/internal/llm"
)

type memStore struct {
	mu sync.Mutex
	m  map[string]string
}

func (s *memStore) Get(_ context.Context, job, lang, _ string, _ int) (string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.m[job+lang]
	return v, ok, nil
}

func (s *memStore) Put(_ context.Context, job, lang, _ string, _ int, _, text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[job+lang] = text
	return nil
}

// slowModel writes its reply in pieces, waiting for release between them.
type slowModel struct {
	runs    atomic.Int32
	release chan struct{}
}

func (m *slowModel) Complete(ctx context.Context, r llm.Request) (llm.Response, error) {
	return m.Stream(ctx, r, nil)
}

func (m *slowModel) Stream(_ context.Context, _ llm.Request, on func(string)) (llm.Response, error) {
	m.runs.Add(1)
	var all strings.Builder
	for _, p := range []string{"Acme arar: ", "React ile ", "çalışacak biri."} {
		<-m.release
		all.WriteString(p)
		if on != nil {
			on(p)
		}
	}
	return llm.Response{Content: all.String()}, nil
}

func TestStreamSharesOneModelRunAndEmitsEverythingInOrder(t *testing.T) {
	model := &slowModel{release: make(chan struct{})}
	s := &Summarizer{Client: model, Store: &memStore{m: map[string]string{}}}
	read := func() (string, string) {
		var got strings.Builder
		final, err := s.Stream(context.Background(), "job", "Engineer", "Acme", "We build with React every day.", "tr", func(p string) { got.WriteString(p) })
		if err != nil {
			t.Error(err)
		}
		return got.String(), final
	}
	var wg sync.WaitGroup
	results := make([][2]string, 2)
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			a, b := read()
			results[i] = [2]string{a, b}
		}()
		time.Sleep(50 * time.Millisecond) // the second reader joins the run in progress
	}
	for range 3 {
		model.release <- struct{}{}
		time.Sleep(20 * time.Millisecond)
	}
	wg.Wait()
	const want = "Acme arar: React ile çalışacak biri."
	for i, r := range results {
		if r[0] != want || r[1] != want {
			t.Errorf("reader %d: emitted %q, final %q", i, r[0], r[1])
		}
	}
	if n := model.runs.Load(); n != 1 {
		t.Errorf("model ran %d times, want 1", n)
	}
	// Stored now: a later reader gets it in one piece without the model.
	if a, b := read(); a != want || b != want || model.runs.Load() != 1 {
		t.Errorf("stored: %q %q runs=%d", a, b, model.runs.Load())
	}
}
