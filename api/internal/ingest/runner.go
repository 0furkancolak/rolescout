package ingest

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"example.com/rolescout/api/internal/crawl"
	"example.com/rolescout/api/internal/domain"
	"example.com/rolescout/api/internal/infrastructure/database"
)

type SourceStore interface {
	Due(ctx context.Context, now time.Time) ([]domain.JobSource, error)
	StartRun(ctx context.Context, sourceID string) (domain.IngestionRun, error)
	FinishRun(ctx context.Context, run domain.IngestionRun) (domain.IngestionRun, error)
	FailStaleRuns(ctx context.Context, maxAge time.Duration) (int64, error)
	PurgeRuns(ctx context.Context, olderThan time.Duration) (int64, error)
}

type ListingStore interface {
	UpsertListing(ctx context.Context, in database.IngestedListing) (domain.Job, domain.JobListing, error)
	CloseMissing(ctx context.Context, sourceID string, seenExternalIDs []string, at time.Time) (int64, error)
	CloseFailedSources(ctx context.Context, failingSince time.Duration) (int64, error)
}

// CompanyOutcomes records sync results on the company a source belongs to.
type CompanyOutcomes interface {
	RecordSyncOutcome(ctx context.Context, companyID string, run domain.IngestionRun) error
}

// Runner synchronizes sources. Every synchronization is recorded as an
// ingestion run with its counts and error, so failures can be diagnosed and
// are retried on a later tick.
type Runner struct {
	Sources  SourceStore
	Listings ListingStore
	Adapters Registry
	Logger   *slog.Logger
	// Companies, when set, is told the outcome of every company source sync.
	Companies CompanyOutcomes

	// MaxAttempts is how often a retryable fetch error is tried (default 3).
	MaxAttempts int
	// Backoff returns the wait before the given retry (default 2^n seconds).
	Backoff func(retry int) time.Duration
	// FetchTimeout bounds one fetch attempt (default 2 minutes).
	FetchTimeout time.Duration
	// StaleAfter is when a running run is considered abandoned (default 1 hour).
	StaleAfter time.Duration
	// CloseFailedAfter closes the listings of a source that has only failed
	// for this long (default 7 days).
	CloseFailedAfter time.Duration
	// RunRetention is how long finished runs are kept (default 90 days).
	RunRetention time.Duration
	Now          func() time.Time

	maintenanceMu sync.Mutex
	lastMaint     time.Time
}

// RunDueOptions controls one pass over due sources.
type RunDueOptions struct {
	// Concurrency is how many sources sync at once (default 1).
	Concurrency int
	// ConcurrencyFn, when set, is asked at the start of each pass and takes
	// the place of Concurrency.
	ConcurrencyFn func() int
	// OnStart is told how many sources are due before syncing starts.
	OnStart func(total int)
	// OnResult is called after each sync, possibly from several goroutines.
	OnResult func(source domain.JobSource, run domain.IngestionRun, err error)
}

func (r *Runner) defaults() {
	if r.MaxAttempts <= 0 {
		r.MaxAttempts = 3
	}
	if r.Backoff == nil {
		r.Backoff = func(retry int) time.Duration { return time.Duration(1<<retry) * time.Second }
	}
	if r.FetchTimeout <= 0 {
		r.FetchTimeout = 2 * time.Minute
	}
	if r.StaleAfter <= 0 {
		r.StaleAfter = time.Hour
	}
	if r.CloseFailedAfter <= 0 {
		r.CloseFailedAfter = 7 * 24 * time.Hour
	}
	if r.RunRetention <= 0 {
		r.RunRetention = 90 * 24 * time.Hour
	}
	if r.Now == nil {
		r.Now = time.Now
	}
	if r.Logger == nil {
		r.Logger = slog.Default()
	}
}

// Loop runs due sources every interval until ctx is canceled.
func (r *Runner) Loop(ctx context.Context, interval time.Duration, o RunDueOptions) {
	r.defaults()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if err := r.RunDue(ctx, o); err != nil && ctx.Err() == nil {
			r.Logger.Error("ingestion tick failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// RunDue synchronizes every source that is due, o.Concurrency at a time.
func (r *Runner) RunDue(ctx context.Context, o RunDueOptions) error {
	r.defaults()
	if n, err := r.Sources.FailStaleRuns(ctx, r.StaleAfter); err != nil {
		return err
	} else if n > 0 {
		r.Logger.Warn("marked abandoned ingestion runs as failed", "count", n)
	}
	if err := r.maintain(ctx); err != nil {
		return err
	}
	sources, err := r.Sources.Due(ctx, r.Now())
	if err != nil {
		return err
	}
	if o.OnStart != nil {
		o.OnStart(len(sources))
	}
	concurrency := max(o.Concurrency, 1)
	if o.ConcurrencyFn != nil {
		concurrency = max(o.ConcurrencyFn(), 1)
	}
	work := make(chan domain.JobSource)
	var wg sync.WaitGroup
	for range min(concurrency, max(len(sources), 1)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for source := range work {
				run, err := r.Sync(ctx, source)
				if err != nil {
					r.Logger.Error("source sync failed", "source", source.Key, "error", err)
				}
				if o.OnResult != nil {
					o.OnResult(source, run, err)
				}
			}
		}()
	}
feed:
	for _, source := range sources {
		select {
		case work <- source:
		case <-ctx.Done():
			break feed
		}
	}
	close(work)
	wg.Wait()
	return ctx.Err()
}

// maintain runs the daily housekeeping: closing jobs of sources that keep
// failing and deleting old run history. RunDue is called by more than one
// loop, so this runs at most once a day across them.
func (r *Runner) maintain(ctx context.Context) error {
	r.maintenanceMu.Lock()
	defer r.maintenanceMu.Unlock()
	now := r.Now()
	if !r.lastMaint.IsZero() && now.Sub(r.lastMaint) < 24*time.Hour {
		return nil
	}
	if n, err := r.Listings.CloseFailedSources(ctx, r.CloseFailedAfter); err != nil {
		return err
	} else if n > 0 {
		r.Logger.Info("closed jobs of sources failing for a long time", "jobs", n)
	}
	if n, err := r.Sources.PurgeRuns(ctx, r.RunRetention); err != nil {
		return err
	} else if n > 0 {
		r.Logger.Info("deleted old ingestion runs", "count", n)
	}
	r.lastMaint = now
	return nil
}

// Sync fetches and stores one source's listings and records the run. The
// returned error describes a failed run; the run itself is still stored.
func (r *Runner) Sync(ctx context.Context, source domain.JobSource) (domain.IngestionRun, error) {
	r.defaults()
	log := r.Logger.With("source", source.Key, "adapter", source.Adapter)

	run, err := r.Sources.StartRun(ctx, source.ID)
	if errors.Is(err, domain.ErrConflict) {
		log.Info("source sync already in progress; skipping")
		return run, nil
	}
	if err != nil {
		return run, err
	}
	started := time.Now()
	log = log.With("run_id", run.ID)
	log.Info("source sync started")

	runErr := r.sync(ctx, source, &run, log)
	if runErr != nil {
		msg := runErr.Error()
		run.Status, run.Error = domain.RunFailed, &msg
	} else {
		run.Status = domain.RunSucceeded
	}
	// Record the outcome even if the worker is shutting down.
	finishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	saved, err := r.Sources.FinishRun(finishCtx, run)
	if err != nil {
		return run, errors.Join(runErr, fmt.Errorf("record run outcome: %w", err))
	}

	attrs := []any{
		"status", saved.Status, "attempts", saved.Attempts, "fetched", saved.FetchedCount,
		"upserted", saved.UpsertedCount, "failed", saved.FailedCount,
		"duration_ms", time.Since(started).Milliseconds(),
	}
	if runErr != nil {
		log.Error("source sync failed", append(attrs, "error", runErr)...)
	} else {
		log.Info("source sync finished", attrs...)
	}
	if r.Companies != nil && source.CompanyID != nil {
		if err := r.Companies.RecordSyncOutcome(finishCtx, *source.CompanyID, saved); err != nil {
			log.Error("record company sync outcome", "error", err)
		}
	}
	return saved, runErr
}

func (r *Runner) sync(ctx context.Context, source domain.JobSource, run *domain.IngestionRun, log *slog.Logger) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("adapter panicked: %v", p)
		}
	}()

	adapter, err := r.Adapters.Get(source.Adapter)
	if err != nil {
		return err
	}
	raw, err := r.fetch(ctx, adapter, source, run, log)
	note, hasNote := asNote(err)
	if err != nil && !hasNote {
		return err
	}

	seenAt := r.Now().UTC()
	seen := make(map[string]bool, len(raw))
	var firstErr error
	for _, item := range raw {
		if ctx.Err() != nil {
			return fmt.Errorf("stopped after %d of %d listings: %w", run.UpsertedCount+run.FailedCount, len(raw), ctx.Err())
		}
		// Sources occasionally repeat a listing within one snapshot.
		if seen[item.ExternalID] {
			continue
		}
		seen[item.ExternalID] = true
		run.FetchedCount++

		listing, err := Normalize(source.ID, item, seenAt)
		listing.CompanyID = source.CompanyID
		if err == nil {
			_, _, err = r.Listings.UpsertListing(ctx, listing)
		}
		if err != nil {
			run.FailedCount++
			if firstErr == nil {
				firstErr = err
			}
			log.Warn("listing skipped", "external_id", item.ExternalID, "error", err)
			continue
		}
		run.UpsertedCount++
	}

	var notes []string
	if hasNote {
		notes = append(notes, note.msg)
	}
	if run.FailedCount > 0 {
		msg := fmt.Sprintf("%d of %d listings skipped; first error: %v", run.FailedCount, run.FetchedCount, firstErr)
		if run.UpsertedCount == 0 {
			return errors.New(strings.Join(append(notes, msg), "; "))
		}
		notes = append(notes, msg)
	}
	if len(notes) > 0 {
		msg := strings.Join(notes, "; ")
		run.Error = &msg
	}
	if hasNote && !note.complete {
		log.Warn("incomplete snapshot; nothing closed", "reason", note.msg)
		return nil
	}
	// The snapshot is complete: listings it no longer contains are closed.
	ids := make([]string, 0, len(seen))
	for _, item := range raw {
		if seen[item.ExternalID] {
			ids = append(ids, item.ExternalID)
			seen[item.ExternalID] = false
		}
	}
	if closed, err := r.Listings.CloseMissing(ctx, source.ID, ids, seenAt); err != nil {
		return fmt.Errorf("close missing listings: %w", err)
	} else if closed > 0 {
		log.Info("closed jobs no longer listed", "jobs", closed)
	}
	return nil
}

func (r *Runner) fetch(ctx context.Context, adapter Adapter, source domain.JobSource, run *domain.IngestionRun, log *slog.Logger) ([]RawListing, error) {
	for attempt := 1; ; attempt++ {
		run.Attempts = attempt
		timeout := r.FetchTimeout
		if t, ok := adapter.(FetchTimeouter); ok {
			timeout = max(timeout, t.FetchTimeout(source.Config))
		}
		fetchCtx, cancel := context.WithTimeout(ctx, timeout)
		listings, err := adapter.Fetch(fetchCtx, source.Config)
		cancel()
		if _, ok := asNote(err); err == nil || ok {
			return listings, err
		}
		// A Retry-After longer than this is left to the next scheduled sync.
		const maxRetryAfter = time.Minute
		var se *crawl.StatusError
		retryAfter := time.Duration(0)
		if errors.As(err, &se) {
			retryAfter = se.RetryAfter
		}
		if !IsRetryable(err) || attempt >= r.MaxAttempts || ctx.Err() != nil || retryAfter > maxRetryAfter {
			return nil, fmt.Errorf("fetch failed after %d attempt(s): %w", attempt, err)
		}
		wait := max(r.Backoff(attempt), retryAfter)
		log.Warn("fetch failed; retrying", "attempt", attempt, "retry_in", wait.String(), "error", err)
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("fetch canceled during retry: %w", ctx.Err())
		case <-time.After(wait):
		}
	}
}
