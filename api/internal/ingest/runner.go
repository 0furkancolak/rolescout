package ingest

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"example.com/rolescout/api/internal/domain"
	"example.com/rolescout/api/internal/infrastructure/database"
)

type SourceStore interface {
	Due(ctx context.Context, now time.Time) ([]domain.JobSource, error)
	StartRun(ctx context.Context, sourceID string) (domain.IngestionRun, error)
	FinishRun(ctx context.Context, run domain.IngestionRun) (domain.IngestionRun, error)
	FailStaleRuns(ctx context.Context, maxAge time.Duration) (int64, error)
}

type ListingStore interface {
	UpsertListing(ctx context.Context, in database.IngestedListing) (domain.Job, domain.JobListing, error)
}

// Runner synchronizes sources. Every synchronization is recorded as an
// ingestion run with its counts and error, so failures can be diagnosed and
// are retried on a later tick.
type Runner struct {
	Sources  SourceStore
	Listings ListingStore
	Adapters Registry
	Logger   *slog.Logger

	// MaxAttempts is how often a retryable fetch error is tried (default 3).
	MaxAttempts int
	// Backoff returns the wait before the given retry (default 2^n seconds).
	Backoff func(retry int) time.Duration
	// FetchTimeout bounds one fetch attempt (default 2 minutes).
	FetchTimeout time.Duration
	// StaleAfter is when a running run is considered abandoned (default 1 hour).
	StaleAfter time.Duration
	Now        func() time.Time
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
	if r.Now == nil {
		r.Now = time.Now
	}
	if r.Logger == nil {
		r.Logger = slog.Default()
	}
}

// Loop runs due sources every interval until ctx is canceled.
func (r *Runner) Loop(ctx context.Context, interval time.Duration) {
	r.defaults()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if err := r.RunDue(ctx); err != nil && ctx.Err() == nil {
			r.Logger.Error("ingestion tick failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// RunDue synchronizes every source that is due, one after another.
func (r *Runner) RunDue(ctx context.Context) error {
	r.defaults()
	if n, err := r.Sources.FailStaleRuns(ctx, r.StaleAfter); err != nil {
		return err
	} else if n > 0 {
		r.Logger.Warn("marked abandoned ingestion runs as failed", "count", n)
	}
	sources, err := r.Sources.Due(ctx, r.Now())
	if err != nil {
		return err
	}
	for _, source := range sources {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if _, err := r.Sync(ctx, source); err != nil {
			r.Logger.Error("source sync failed", "source", source.Key, "error", err)
		}
	}
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
	if err != nil {
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

	if run.FailedCount > 0 {
		msg := fmt.Sprintf("%d of %d listings skipped; first error: %v", run.FailedCount, run.FetchedCount, firstErr)
		if run.UpsertedCount == 0 {
			return errors.New(msg)
		}
		run.Error = &msg
	}
	return nil
}

func (r *Runner) fetch(ctx context.Context, adapter Adapter, source domain.JobSource, run *domain.IngestionRun, log *slog.Logger) ([]RawListing, error) {
	for attempt := 1; ; attempt++ {
		run.Attempts = attempt
		fetchCtx, cancel := context.WithTimeout(ctx, r.FetchTimeout)
		listings, err := adapter.Fetch(fetchCtx, source.Config)
		cancel()
		if err == nil {
			return listings, nil
		}
		if !IsRetryable(err) || attempt >= r.MaxAttempts || ctx.Err() != nil {
			return nil, fmt.Errorf("fetch failed after %d attempt(s): %w", attempt, err)
		}
		wait := r.Backoff(attempt)
		log.Warn("fetch failed; retrying", "attempt", attempt, "retry_in", wait.String(), "error", err)
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("fetch canceled during retry: %w", ctx.Err())
		case <-time.After(wait):
		}
	}
}
