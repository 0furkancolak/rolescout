package enrich

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"

	"example.com/rolescout/api/internal/llm"
)

// SummaryStore keeps written summaries.
type SummaryStore interface {
	Get(ctx context.Context, jobID, language, hash string, promptVersion int) (string, bool, error)
	Put(ctx context.Context, jobID, language, hash string, promptVersion int, model, summary string) error
}

// Summarizer writes a job summary on first request and reuses it after. Only
// one request per job and language asks the model at a time.
type Summarizer struct {
	Client  llm.Completer
	Model   string
	Store   SummaryStore
	Timeout time.Duration

	mu     sync.Mutex
	flight map[string]*call
}

type call struct {
	done chan struct{}
	text string
	err  error
}

// waitFor is how long a request waits for a summary being written before it
// answers "pending" (the web client asks again); it stays under the server's
// write timeout.
const waitFor = 8 * time.Second

// Summary returns the summary and true, writing it if needed. When the model
// is still working after a short wait it returns ("", false, nil) and the
// work continues in the background; errors mean the model is unavailable or
// answered badly.
func (s *Summarizer) Summary(ctx context.Context, jobID, title, company, description, language string) (string, bool, error) {
	sum := sha256.Sum256([]byte(title + "\x00" + description))
	hash := hex.EncodeToString(sum[:8])
	if text, ok, err := s.Store.Get(ctx, jobID, language, hash, llm.SummaryPromptVersion); err == nil && ok {
		return text, true, nil
	}
	key := jobID + "/" + language
	s.mu.Lock()
	if s.flight == nil {
		s.flight = map[string]*call{}
	}
	if c, ok := s.flight[key]; ok {
		s.mu.Unlock()
		return s.await(ctx, c)
	}
	c := &call{done: make(chan struct{})}
	s.flight[key] = c
	s.mu.Unlock()

	// The model call outlives a closed browser tab so the next visit finds it.
	run, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.timeout())
	go func() {
		defer cancel()
		c.text, c.err = llm.Summarize(run, s.Client, title, company, description, language)
		if c.err == nil {
			_ = s.Store.Put(run, jobID, language, hash, llm.SummaryPromptVersion, s.Model, c.text)
		}
		s.mu.Lock()
		delete(s.flight, key)
		s.mu.Unlock()
		close(c.done)
	}()
	return s.await(ctx, c)
}

func (s *Summarizer) await(ctx context.Context, c *call) (string, bool, error) {
	select {
	case <-c.done:
		return c.text, c.err == nil, c.err
	case <-time.After(waitFor):
		return "", false, nil
	case <-ctx.Done():
		return "", false, ctx.Err()
	}
}

func (s *Summarizer) timeout() time.Duration {
	if s.Timeout > 0 {
		return s.Timeout
	}
	return 3 * time.Minute
}
