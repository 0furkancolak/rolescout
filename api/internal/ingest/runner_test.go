package ingest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"example.com/rolescout/api/internal/domain"
	"example.com/rolescout/api/internal/infrastructure/database"
)

type fakeSources struct {
	mu       sync.Mutex
	running  bool
	finished []domain.IngestionRun
	due      []domain.JobSource
	stale    int
}

func (f *fakeSources) Due(context.Context, time.Time) ([]domain.JobSource, error) { return f.due, nil }

func (f *fakeSources) StartRun(_ context.Context, sourceID string) (domain.IngestionRun, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.running {
		return domain.IngestionRun{}, domain.ErrConflict
	}
	f.running = true
	return domain.IngestionRun{ID: "run-1", SourceID: sourceID, Status: domain.RunRunning}, nil
}

func (f *fakeSources) FinishRun(ctx context.Context, run domain.IngestionRun) (domain.IngestionRun, error) {
	if ctx.Err() != nil {
		return run, ctx.Err()
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.running = false
	f.finished = append(f.finished, run)
	return run, nil
}

func (f *fakeSources) FailStaleRuns(context.Context, time.Duration) (int64, error) {
	f.stale++
	return 0, nil
}

type fakeListings struct {
	stored []database.IngestedListing
	failOn string
}

func (f *fakeListings) UpsertListing(_ context.Context, in database.IngestedListing) (domain.Job, domain.JobListing, error) {
	if in.ExternalID == f.failOn {
		return domain.Job{}, domain.JobListing{}, fmt.Errorf("store: %w", domain.ErrInvalid)
	}
	f.stored = append(f.stored, in)
	return domain.Job{}, domain.JobListing{}, nil
}

type fakeAdapter struct {
	calls   int
	results []func() ([]RawListing, error)
}

func (a *fakeAdapter) Validate(map[string]any) error { return nil }

func (a *fakeAdapter) Fetch(context.Context, map[string]any) ([]RawListing, error) {
	r := a.results[min(a.calls, len(a.results)-1)]
	a.calls++
	return r()
}

func listing(id string) RawListing {
	return RawListing{ExternalID: id, URL: "https://jobs.example.com/" + id, Title: "Engineer " + id, Company: "Acme"}
}

func ok(ls ...RawListing) func() ([]RawListing, error) {
	return func() ([]RawListing, error) { return ls, nil }
}

func fail(err error) func() ([]RawListing, error) {
	return func() ([]RawListing, error) { return nil, err }
}

func newRunner(adapter Adapter) (*Runner, *fakeSources, *fakeListings) {
	sources, listings := &fakeSources{}, &fakeListings{}
	return &Runner{
		Sources:  sources,
		Listings: listings,
		Adapters: Registry{"fake": adapter},
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		Backoff:  func(int) time.Duration { return time.Millisecond },
	}, sources, listings
}

var source = domain.JobSource{ID: "src-1", Key: "fake-acme", Adapter: "fake"}

func TestSyncRetriesTemporaryFailures(t *testing.T) {
	adapter := &fakeAdapter{results: []func() ([]RawListing, error){
		fail(Retryable(errors.New("503"))), fail(Retryable(errors.New("timeout"))), ok(listing("1"), listing("2")),
	}}
	r, _, listings := newRunner(adapter)
	run, err := r.Sync(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != domain.RunSucceeded || run.Attempts != 3 || run.FetchedCount != 2 || run.UpsertedCount != 2 {
		t.Fatalf("unexpected run: %+v", run)
	}
	if len(listings.stored) != 2 || listings.stored[0].SourceID != "src-1" {
		t.Fatalf("stored %+v", listings.stored)
	}
}

func TestSyncDoesNotRetryPermanentFailures(t *testing.T) {
	adapter := &fakeAdapter{results: []func() ([]RawListing, error){fail(errors.New("board not found"))}}
	r, sources, _ := newRunner(adapter)
	run, err := r.Sync(context.Background(), source)
	if err == nil || adapter.calls != 1 {
		t.Fatalf("err=%v calls=%d; want one failed attempt", err, adapter.calls)
	}
	if run.Status != domain.RunFailed || run.Error == nil || !strings.Contains(*run.Error, "board not found") {
		t.Fatalf("run must record the failure: %+v", run)
	}
	if len(sources.finished) != 1 {
		t.Fatal("failed runs must be recorded")
	}
}

func TestSyncGivesUpAfterMaxAttempts(t *testing.T) {
	adapter := &fakeAdapter{results: []func() ([]RawListing, error){fail(Retryable(errors.New("503")))}}
	r, _, _ := newRunner(adapter)
	r.MaxAttempts = 2
	run, err := r.Sync(context.Background(), source)
	if err == nil || run.Attempts != 2 || adapter.calls != 2 {
		t.Fatalf("err=%v attempts=%d calls=%d", err, run.Attempts, adapter.calls)
	}
}

func TestSyncSkipsBadListingsAndDuplicates(t *testing.T) {
	bad := listing("bad")
	bad.URL = "ftp://example.com"
	adapter := &fakeAdapter{results: []func() ([]RawListing, error){
		ok(listing("1"), listing("1"), bad, listing("store-fails"), listing("2")),
	}}
	r, _, listings := newRunner(adapter)
	listings.failOn = "store-fails"
	run, err := r.Sync(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	if run.FetchedCount != 4 || run.UpsertedCount != 2 || run.FailedCount != 2 || run.Status != domain.RunSucceeded {
		t.Fatalf("unexpected counts: %+v", run)
	}
	if run.Error == nil || !strings.Contains(*run.Error, "2 of 4 listings skipped") {
		t.Fatalf("partial failures must be summarized: %v", run.Error)
	}
}

func TestSyncFailsWhenEveryListingFails(t *testing.T) {
	bad := listing("bad")
	bad.Title = ""
	r, _, _ := newRunner(&fakeAdapter{results: []func() ([]RawListing, error){ok(bad)}})
	run, err := r.Sync(context.Background(), source)
	if err == nil || run.Status != domain.RunFailed {
		t.Fatalf("err=%v run=%+v", err, run)
	}
}

func TestSyncSkipsSourceThatIsAlreadyRunning(t *testing.T) {
	adapter := &fakeAdapter{results: []func() ([]RawListing, error){ok(listing("1"))}}
	r, sources, _ := newRunner(adapter)
	sources.running = true
	if _, err := r.Sync(context.Background(), source); err != nil {
		t.Fatal(err)
	}
	if adapter.calls != 0 {
		t.Fatal("a running source must not be fetched again")
	}
}

func TestSyncRecoversFromAdapterPanics(t *testing.T) {
	adapter := &fakeAdapter{results: []func() ([]RawListing, error){func() ([]RawListing, error) { panic("boom") }}}
	r, sources, _ := newRunner(adapter)
	run, err := r.Sync(context.Background(), source)
	if err == nil || run.Status != domain.RunFailed || len(sources.finished) != 1 {
		t.Fatalf("panic must fail the run: err=%v run=%+v", err, run)
	}
}

func TestSyncRecordsOutcomeWhenCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	adapter := &fakeAdapter{results: []func() ([]RawListing, error){func() ([]RawListing, error) {
		cancel()
		return nil, Retryable(errors.New("timeout"))
	}}}
	r, sources, _ := newRunner(adapter)
	run, err := r.Sync(ctx, source)
	if err == nil || run.Status != domain.RunFailed {
		t.Fatalf("err=%v run=%+v", err, run)
	}
	if len(sources.finished) != 1 {
		t.Fatal("a canceled run must still be recorded")
	}
}

func TestRunDueSyncsEachDueSourceAndFailsStaleRuns(t *testing.T) {
	adapter := &fakeAdapter{results: []func() ([]RawListing, error){ok(listing("1"))}}
	r, sources, _ := newRunner(adapter)
	other := source
	other.ID, other.Key, other.Adapter = "src-2", "unknown", "missing"
	sources.due = []domain.JobSource{source, other}
	if err := r.RunDue(context.Background()); err != nil {
		t.Fatal(err)
	}
	if sources.stale != 1 || len(sources.finished) != 2 {
		t.Fatalf("stale=%d finished=%d", sources.stale, len(sources.finished))
	}
	if sources.finished[1].Status != domain.RunFailed || !strings.Contains(*sources.finished[1].Error, "unknown source adapter") {
		t.Fatalf("unknown adapters must fail their run: %+v", sources.finished[1])
	}
}
