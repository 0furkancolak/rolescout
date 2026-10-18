package enrich

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"example.com/rolescout/api/internal/domain"
	"example.com/rolescout/api/internal/llm"
)

// Repository is the queue and cache the enricher works through.
type Repository interface {
	Next(ctx context.Context, model string, promptVersion, schemaVersion, limit int, now time.Time) ([]domain.EnrichmentTask, error)
	Save(ctx context.Context, r domain.EnrichmentResult) error
}

// Enricher reads job descriptions with the local model, one bounded batch per
// Tick. It never blocks anything else: rules-based values stay the baseline and
// the fallback whenever the model is off, slow or wrong.
type Enricher struct {
	Repo           Repository
	LLM            llm.Completer
	Model          string
	FallbackModel  string
	Concurrency    int
	Batch          int
	MaxChars       int
	MaxTokens      int
	Breaker        *Breaker
	Now            func() time.Time
	Logger         *slog.Logger
	ResolveCountry func(place string) string

	warned atomic.Bool
}

// Result describes one Tick.
type Result struct {
	Enriched int
	Failed   int
	// Skipped is true when the breaker is open and there is no fallback model.
	Skipped bool
	// Degraded is true when the fallback model did the work.
	Degraded bool
}

func (e *Enricher) now() time.Time {
	if e.Now != nil {
		return e.Now()
	}
	return time.Now()
}

func (e *Enricher) log() *slog.Logger {
	if e.Logger != nil {
		return e.Logger
	}
	return slog.Default()
}

// Tick enriches up to Batch queued jobs.
func (e *Enricher) Tick(ctx context.Context) (Result, error) {
	var res Result
	model := e.Model
	probing := e.Breaker.Allow()
	if !probing {
		if e.FallbackModel == "" {
			res.Skipped = true
			e.warnOnce("local model unavailable, enrichment paused")
			return res, nil
		}
		model, res.Degraded = e.FallbackModel, true
		e.warnOnce("local model unavailable, using the fallback model")
	} else {
		e.warned.Store(false)
	}

	tasks, err := e.Repo.Next(ctx, model, llm.PromptVersion, llm.SchemaVersion, e.Batch, e.now())
	if err != nil || len(tasks) == 0 {
		return res, err
	}

	var (
		mu  sync.Mutex
		wg  sync.WaitGroup
		sem = make(chan struct{}, max(1, e.Concurrency))
	)
loop:
	for _, task := range tasks {
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			break loop
		}
		// Stop starting jobs once the breaker opens; the rest stay queued. The
		// check follows the wait for a free slot, so a failure that opened it
		// while this job waited is seen.
		if !res.Degraded && e.Breaker.Open() {
			<-sem
			break
		}
		wg.Add(1)
		go func(task domain.EnrichmentTask) {
			defer wg.Done()
			defer func() { <-sem }()
			ok, saved := e.one(ctx, model, task, res.Degraded)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case ok:
				res.Enriched++
			case saved:
				res.Failed++
			}
		}(task)
	}
	wg.Wait()
	if res.Enriched+res.Failed > 0 {
		e.log().Info("enrichment batch", "model", model, "enriched", res.Enriched, "failed", res.Failed, "degraded", res.Degraded)
	}
	return res, ctx.Err()
}

// one enriches a single job. It returns whether the job was enriched and
// whether a failure was stored (a cancelled or unavailable request is neither).
func (e *Enricher) one(ctx context.Context, model string, task domain.EnrichmentTask, degraded bool) (ok, stored bool) {
	enrichment, stats, err := llm.Extract(ctx, e.LLM, llm.Input{
		Model: model, Title: task.Title, Company: task.CompanyName, Description: task.Description,
		MaxChars: e.MaxChars, MaxTokens: e.MaxTokens, ResolveCountry: e.ResolveCountry,
	})
	base := domain.EnrichmentResult{
		JobID: task.JobID, DescriptionHash: task.DescriptionHash, Model: model,
		PromptVersion: llm.PromptVersion, SchemaVersion: llm.SchemaVersion, Now: e.now(),
		PromptTokens: stats.PromptTokens, CompletionTokens: stats.CompletionTokens, Latency: stats.Latency,
	}
	switch {
	case err == nil:
		data, mErr := json.Marshal(enrichment)
		if mErr != nil {
			return false, e.store(ctx, withError(base, "encode"))
		}
		if !degraded {
			e.Breaker.Success()
		}
		base.OK, base.Data = true, data
		return e.store(ctx, base), false
	case errors.Is(err, context.Canceled):
		return false, false
	case errors.Is(err, llm.ErrUnavailable):
		// A down server says nothing about this job: keep it queued.
		e.fail(degraded)
		return false, false
	case errors.Is(err, llm.ErrTimeout):
		e.fail(degraded)
		return false, e.store(ctx, withError(base, "timeout"))
	default:
		return false, e.store(ctx, withError(base, "invalid output"))
	}
}

func withError(r domain.EnrichmentResult, msg string) domain.EnrichmentResult {
	r.OK, r.Error = false, msg
	return r
}

func (e *Enricher) fail(degraded bool) {
	if degraded {
		return
	}
	if e.Breaker.Failure() {
		e.warnOnce("local model unavailable, enrichment paused")
	}
}

func (e *Enricher) store(ctx context.Context, r domain.EnrichmentResult) bool {
	if err := e.Repo.Save(ctx, r); err != nil {
		if ctx.Err() == nil {
			e.log().Error("save enrichment", "job", r.JobID, "error", err)
		}
		return false
	}
	return true
}

// warnOnce logs a state change once, not on every tick.
func (e *Enricher) warnOnce(msg string) {
	if e.warned.CompareAndSwap(false, true) {
		e.log().Warn(msg)
	}
}

// Loop runs Tick until ctx is cancelled.
func (e *Enricher) Loop(ctx context.Context, every time.Duration) {
	for {
		res, err := e.Tick(ctx)
		if err != nil && ctx.Err() == nil {
			e.log().Error("enrichment tick", "error", err)
		}
		// While there is work, go straight on; when idle, wait a full period.
		pause := every
		if res.Enriched+res.Failed > 0 {
			pause = min(every, time.Second)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(pause):
		}
	}
}
