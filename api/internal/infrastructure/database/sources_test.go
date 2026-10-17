package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"example.com/rolescout/api/internal/domain"
)

func TestSourceMustBeReviewedBeforeEnabling(t *testing.T) {
	pool := requireDB(t)
	ctx := context.Background()
	repo := NewSourceRepository(pool)

	// An adapter type without an enabled review needs a review per source.
	s, err := repo.Create(ctx, SourceParams{
		Key: uniq("wd-"), Name: "Acme", BaseURL: "https://acme.wd5.myworkdayjobs.com",
		Adapter: uniq("unreviewed_"), Config: map[string]any{"board_token": "acme"}, SyncInterval: 2 * time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	if s.Enabled || s.ReviewedAt != nil || s.SyncInterval != 2*time.Hour || s.Config["board_token"] != "acme" {
		t.Fatalf("unexpected new source: %+v", s)
	}
	if _, err := repo.SetEnabled(ctx, s.Key, true); !errors.Is(err, ErrSourceNotReviewed) {
		t.Fatalf("enable without review: err = %v", err)
	}
	if _, err := repo.Review(ctx, s.Key, "Public Job Board API, no auth; terms checked 2026-10-17"); err != nil {
		t.Fatal(err)
	}
	s, err = repo.SetEnabled(ctx, s.Key, true)
	if err != nil || !s.Enabled {
		t.Fatalf("enable after review: %v %+v", err, s)
	}
	// The database enforces the rule even for direct updates.
	_, err = pool.Exec(ctx, `UPDATE job_sources SET review_note = '  ' WHERE id = $1`, s.ID)
	requireErr(t, translate(err, "update"), domain.ErrInvalid)

	defaults, err := repo.Create(ctx, SourceParams{Key: uniq("manual-"), Name: "Manual", BaseURL: "https://example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if defaults.Adapter != "manual" || defaults.SyncInterval != 6*time.Hour {
		t.Fatalf("defaults not applied: %+v", defaults)
	}
	if _, err := repo.Create(ctx, SourceParams{Key: uniq("fast-"), Name: "Fast", BaseURL: "https://example.com", SyncInterval: time.Minute}); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("interval below 5 minutes: err = %v", err)
	}
}

func enabledSource(t *testing.T, repo *SourceRepository, interval time.Duration) domain.JobSource {
	t.Helper()
	ctx := context.Background()
	s, err := repo.Create(ctx, SourceParams{Key: uniq("due-"), Name: "Due", BaseURL: "https://example.com", Adapter: "greenhouse", SyncInterval: interval})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Review(ctx, s.Key, "reviewed"); err != nil {
		t.Fatal(err)
	}
	if s, err = repo.SetEnabled(ctx, s.Key, true); err != nil {
		t.Fatal(err)
	}
	return s
}

func isDue(t *testing.T, repo *SourceRepository, id string, at time.Time) bool {
	t.Helper()
	due, err := repo.Due(context.Background(), at)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range due {
		if s.ID == id {
			return true
		}
	}
	return false
}

func TestDueSourcesAndRuns(t *testing.T) {
	pool := requireDB(t)
	ctx := context.Background()
	repo := NewSourceRepository(pool)
	s := enabledSource(t, repo, time.Hour)
	now := time.Now()

	if !isDue(t, repo, s.ID, now) {
		t.Fatal("a source that never ran must be due")
	}
	run, err := repo.StartRun(ctx, s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.StartRun(ctx, s.ID); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("second concurrent run: err = %v", err)
	}
	if isDue(t, repo, s.ID, now.Add(24*time.Hour)) {
		t.Fatal("a running source must not be due")
	}

	run.Status, run.Attempts, run.FetchedCount, run.UpsertedCount = domain.RunSucceeded, 1, 3, 3
	done, err := repo.FinishRun(ctx, run)
	if err != nil || done.FinishedAt == nil || done.UpsertedCount != 3 {
		t.Fatalf("finish: %v %+v", err, done)
	}
	if _, err := repo.FinishRun(ctx, run); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("finishing twice: err = %v", err)
	}
	if isDue(t, repo, s.ID, now.Add(30*time.Minute)) {
		t.Fatal("a source must not be due before its interval passed")
	}
	if !isDue(t, repo, s.ID, now.Add(61*time.Minute)) {
		t.Fatal("a source must be due after its interval")
	}

	// A failed run is retried after a quarter of the interval (min. 5 minutes).
	run, err = repo.StartRun(ctx, s.ID)
	if err != nil {
		t.Fatal(err)
	}
	msg := "board not found"
	run.Status, run.Error = domain.RunFailed, &msg
	if _, err := repo.FinishRun(ctx, run); err != nil {
		t.Fatal(err)
	}
	if isDue(t, repo, s.ID, time.Now().Add(10*time.Minute)) || !isDue(t, repo, s.ID, time.Now().Add(16*time.Minute)) {
		t.Fatal("failed runs must be retried after a quarter of the interval")
	}

	runs, err := repo.ListRuns(ctx, s.ID, 10)
	if err != nil || len(runs) != 2 || runs[0].Status != domain.RunFailed {
		t.Fatalf("runs: %v %+v", err, runs)
	}

	if _, err := repo.SetEnabled(ctx, s.Key, false); err != nil {
		t.Fatal(err)
	}
	if isDue(t, repo, s.ID, now.Add(48*time.Hour)) {
		t.Fatal("disabled sources are never due")
	}
}

func TestFailStaleRuns(t *testing.T) {
	pool := requireDB(t)
	ctx := context.Background()
	repo := NewSourceRepository(pool)
	s := enabledSource(t, repo, time.Hour)
	run, err := repo.StartRun(ctx, s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE ingestion_runs SET started_at = NOW() - INTERVAL '2 hours' WHERE id = $1`, run.ID); err != nil {
		t.Fatal(err)
	}
	n, err := repo.FailStaleRuns(ctx, time.Hour)
	if err != nil || n < 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	runs, err := repo.ListRuns(ctx, s.ID, 1)
	if err != nil || runs[0].Status != domain.RunFailed || runs[0].Error == nil {
		t.Fatalf("stale run not failed: %v %+v", err, runs)
	}
	if !isDue(t, repo, s.ID, time.Now().Add(16*time.Minute)) {
		t.Fatal("a source with an abandoned run must become due again")
	}
}
