package enrich

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
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
	Client llm.Completer
	Model  string
	// ModelFn, when set, names the model that wrote a summary (it can change
	// at run time) instead of Model.
	ModelFn func(ctx context.Context) string
	Store   SummaryStore
	Timeout time.Duration

	mu     sync.Mutex
	flight map[string]*call
}

type call struct {
	done chan struct{}
	text string
	err  error

	mu   sync.Mutex
	buf  string        // what the model has written so far
	wake chan struct{} // closed and replaced on every write
}

func (c *call) write(piece string) {
	c.mu.Lock()
	c.buf += piece
	close(c.wake)
	c.wake = make(chan struct{})
	c.mu.Unlock()
}

// waitFor is how long a request waits for a summary being written before it
// answers "pending" (the web client asks again); it stays under the server's
// write timeout.
const waitFor = 8 * time.Second

func summaryHash(title, description string) string {
	sum := sha256.Sum256([]byte(title + "\x00" + description))
	return hex.EncodeToString(sum[:8])
}

// begin returns the summary when stored, or the call that is writing it
// (starting one if none is).
func (s *Summarizer) begin(ctx context.Context, jobID, title, company, description, language string) (string, *call) {
	hash := summaryHash(title, description)
	if text, ok, err := s.Store.Get(ctx, jobID, language, hash, llm.SummaryPromptVersion); err == nil && ok {
		return text, nil
	}
	key := jobID + "/" + language
	s.mu.Lock()
	if s.flight == nil {
		s.flight = map[string]*call{}
	}
	if c, ok := s.flight[key]; ok {
		s.mu.Unlock()
		return "", c
	}
	c := &call{done: make(chan struct{}), wake: make(chan struct{})}
	s.flight[key] = c
	s.mu.Unlock()

	// The model call outlives a closed browser tab so the next visit finds it.
	run, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.timeout())
	go func() {
		defer cancel()
		c.text, c.err = llm.Summarize(run, s.Client, title, company, description, language, c.write)
		if errors.Is(c.err, llm.ErrInvalidOutput) {
			// A small model often slips once (a number or tool the posting
			// lacks); a second try usually passes. If it does not, the posting's
			// own opening sentences are better than no summary at all.
			c.text, c.err = llm.Summarize(run, s.Client, title, company, description, language, nil)
			if errors.Is(c.err, llm.ErrInvalidOutput) {
				if text := llm.ExtractiveSummary(description); text != "" {
					c.text, c.err = text, nil
				}
			}
		}
		if c.err == nil {
			_ = s.Store.Put(run, jobID, language, hash, llm.SummaryPromptVersion, s.modelName(run), c.text)
		}
		s.mu.Lock()
		delete(s.flight, key)
		s.mu.Unlock()
		close(c.done)
	}()
	return "", c
}

// Summary returns the summary and true, writing it if needed. When the model
// is still working after a short wait it returns ("", false, nil) and the
// work continues in the background; errors mean the model is unavailable or
// answered badly.
func (s *Summarizer) Summary(ctx context.Context, jobID, title, company, description, language string) (string, bool, error) {
	text, c := s.begin(ctx, jobID, title, company, description, language)
	if c == nil {
		return text, true, nil
	}
	return s.await(ctx, c)
}

// Stream is Summary that hands the text over as the model writes it: emit gets
// each new piece, in order, also for a request that joins a summary already
// being written. A stored summary is emitted in one piece. The returned error
// means what was emitted is not a usable summary.
func (s *Summarizer) Stream(ctx context.Context, jobID, title, company, description, language string, emit func(string)) (string, error) {
	text, c := s.begin(ctx, jobID, title, company, description, language)
	if c == nil {
		emit(text)
		return text, nil
	}
	sent := 0
	for {
		c.mu.Lock()
		piece, wake := c.buf[sent:], c.wake
		sent = len(c.buf)
		c.mu.Unlock()
		if piece != "" {
			emit(piece)
		}
		select {
		case <-c.done:
			c.mu.Lock()
			rest := c.buf[sent:]
			c.mu.Unlock()
			if rest != "" && c.err == nil {
				emit(rest)
			}
			return c.text, c.err
		case <-wake:
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
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

func (s *Summarizer) modelName(ctx context.Context) string {
	if s.ModelFn != nil {
		return s.ModelFn(ctx)
	}
	return s.Model
}
