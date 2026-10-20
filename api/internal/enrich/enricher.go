package enrich

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"example.com/rolescout/api/internal/config"
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
	Repo          Repository
	LLM           llm.Completer
	Model         string
	FallbackModel string
	Concurrency   int
	Batch         int
	MaxChars      int
	MaxTokens     int
	Breaker       *Breaker
	Now           func() time.Time
	// Resolve, when set, supplies the live configuration (settings saved in
	// the web app) and replaces Model and FallbackModel.
	Resolve func(ctx context.Context) config.LLM
	sig     atomic.Value // connection signature seen by the last tick

	// held back explanations after trouble: key -> when to try again.
	heldMu         sync.Mutex
	held           map[string]hold
	Logger         *slog.Logger
	ResolveCountry func(place string) string
	// ExplainMinScore turns on match explanations for matches at or above it;
	// 0 leaves them off. The Repo must also implement ExplanationRepository.
	ExplainMinScore int

	warned atomic.Bool
}

// ExplanationRepository is the optional second queue: short reasons for strong
// matches.
type ExplanationRepository interface {
	ExplanationQueue(ctx context.Context, model string, promptVersion, minScore, limit int) ([]domain.ExplanationTask, error)
	SaveExplanation(ctx context.Context, workspaceID, profileID, jobID, descriptionHash, model string, promptVersion int, text string) error
}

// Result describes one Tick.
type Result struct {
	Enriched int
	Failed   int
	// Skipped is true when the breaker is open and there is no fallback model.
	Skipped bool
	// Degraded is true when the fallback model did the work.
	Degraded bool
	// Explained counts match explanations written.
	Explained int
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
	model, fallback := e.Model, e.FallbackModel
	if e.Resolve != nil {
		cfg := e.Resolve(ctx)
		if !cfg.Enabled || !BackgroundOn(cfg) {
			res.Skipped = true
			return res, nil
		}
		model, fallback = cfg.Model, cfg.FallbackModel
		// A changed connection (new key, URL or model) deserves a fresh start.
		if sig := cfg.BaseURL + "\x00" + cfg.Model + "\x00" + cfg.APIKey; e.sig.Swap(sig) != sig {
			e.Breaker.Success()
		}
	}
	probing := e.Breaker.Allow()
	if !probing {
		if fallback == "" {
			res.Skipped = true
			e.warnOnce("local model unavailable, enrichment paused")
			return res, nil
		}
		model, res.Degraded = fallback, true
		e.warnOnce("local model unavailable, using the fallback model")
	} else {
		e.warned.Store(false)
	}

	tasks, err := e.Repo.Next(ctx, model, llm.PromptVersion, llm.SchemaVersion, e.Batch, e.now())
	if err != nil {
		return res, err
	}
	if len(tasks) == 0 {
		// Nothing to read: spend the tick on explanations, with the main model only.
		if !res.Degraded {
			res.Explained, err = e.explain(ctx, model)
		}
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
	if ctx.Err() == nil && !res.Degraded && e.Breaker.Allow() {
		if n, xerr := e.explain(ctx, model); xerr == nil {
			res.Explained = n
		}
	}
	if res.Enriched+res.Failed+res.Explained > 0 {
		e.log().Info("enrichment batch", "model", model, "enriched", res.Enriched, "failed", res.Failed, "explained", res.Explained, "degraded", res.Degraded)
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

// explain writes up to Batch match explanations. Transport trouble feeds the
// breaker; an answer that fails grounding is recorded empty and not retried.
func (e *Enricher) explain(ctx context.Context, model string) (int, error) {
	xr, ok := e.Repo.(ExplanationRepository)
	if !ok || e.ExplainMinScore <= 0 {
		return 0, nil
	}
	// Items waiting out a failure still sit at the head of the queue, so ask
	// for enough to fill a batch past them.
	tasks, err := xr.ExplanationQueue(ctx, model, llm.ExplainPromptVersion, e.ExplainMinScore, e.Batch+e.heldCount())
	if err != nil {
		return 0, err
	}
	done, tried := 0, 0
	for _, t := range tasks {
		if ctx.Err() != nil || e.Breaker.Open() || tried >= e.Batch {
			break
		}
		key := t.WorkspaceID + "/" + t.ProfileID + "/" + t.JobID + "/" + t.DescriptionHash
		if e.isHeld(key) {
			continue
		}
		tried++
		text, err := llm.Explain(ctx, e.LLM, llm.JobFacts{
			Title: t.Title, Company: t.CompanyName, Score: t.Score, Reasons: reasonFacts(t.Reasons),
			Technologies: t.Technologies, MatchedTechnologies: t.MatchedTechnologies,
			MissingTechnologies: t.MissingTechnologies, CVSkills: t.CVSkills, About: t.About,
			Seniority: t.Seniority, Arrangement: t.Arrangement, Country: t.Country, Language: t.Locale,
		})
		switch {
		case err == nil:
			done++
			e.release(key)
		case errors.Is(err, context.Canceled):
			return done, nil
		case errors.Is(err, llm.ErrUnavailable), errors.Is(err, llm.ErrTimeout):
			e.fail(false)
			e.hold(key)
			continue
		default:
			text = "" // ungrounded or malformed: remember the attempt, show nothing
		}
		if err := xr.SaveExplanation(ctx, t.WorkspaceID, t.ProfileID, t.JobID, t.DescriptionHash, model, llm.ExplainPromptVersion, text); err != nil && ctx.Err() == nil {
			e.log().Error("save explanation", "job", t.JobID, "error", err)
		}
	}
	return done, nil
}

// reasonFacts words match reasons for the prompt as "code: key value".
func reasonFacts(reasons []domain.MatchReason) []string {
	out := make([]string, 0, len(reasons))
	for _, r := range reasons {
		s := r.Code
		keys := make([]string, 0, len(r.Params))
		for k := range r.Params {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			s += " " + k + "=" + r.Params[k]
		}
		out = append(out, s)
	}
	return out
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

// hold is one explanation waiting out a failure.
type hold struct {
	until time.Time
	n     int
}

const (
	holdBase = 5 * time.Minute
	holdMax  = time.Hour
)

// hold delays an explanation after trouble: 5 minutes, doubling up to an hour,
// so one item that keeps timing out cannot take every tick.
func (e *Enricher) hold(key string) {
	e.heldMu.Lock()
	defer e.heldMu.Unlock()
	if e.held == nil {
		e.held = map[string]hold{}
	}
	h := e.held[key]
	h.n++
	wait := min(holdBase<<min(h.n-1, 6), holdMax)
	h.until = e.now().Add(wait)
	e.held[key] = h
}

func (e *Enricher) release(key string) {
	e.heldMu.Lock()
	delete(e.held, key)
	e.heldMu.Unlock()
}

func (e *Enricher) isHeld(key string) bool {
	e.heldMu.Lock()
	defer e.heldMu.Unlock()
	h, ok := e.held[key]
	if ok && !e.now().Before(h.until) {
		return false
	}
	return ok
}

func (e *Enricher) heldCount() int {
	e.heldMu.Lock()
	defer e.heldMu.Unlock()
	return len(e.held)
}

// BackgroundOn reports whether the worker may read the whole catalog on its
// own. Unless an administrator chose, that is on for a model on this machine or
// network and off for a hosted service, where each job read costs money.
func BackgroundOn(cfg config.LLM) bool {
	if cfg.Background != nil {
		return *cfg.Background
	}
	return isLocalHost(hostOnly(cfg.BaseURL))
}
