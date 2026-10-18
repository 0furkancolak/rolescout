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

	"example.com/rolescout/api/internal/crawl"
	"example.com/rolescout/api/internal/domain"
	"example.com/rolescout/api/internal/infrastructure/database"
)

type fakeSources struct {
	mu       sync.Mutex
	running  map[string]bool
	finished []domain.IngestionRun
	due      []domain.JobSource
	stale    int
	purged   int
}

func (f *fakeSources) Due(context.Context, time.Time) ([]domain.JobSource, error) { return f.due, nil }

func (f *fakeSources) StartRun(_ context.Context, sourceID string) (domain.IngestionRun, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.running == nil {
		f.running = map[string]bool{}
	}
	if f.running[sourceID] {
		return domain.IngestionRun{}, domain.ErrConflict
	}
	f.running[sourceID] = true
	return domain.IngestionRun{ID: "run-" + sourceID, SourceID: sourceID, Status: domain.RunRunning}, nil
}

func (f *fakeSources) FinishRun(ctx context.Context, run domain.IngestionRun) (domain.IngestionRun, error) {
	if ctx.Err() != nil {
		return run, ctx.Err()
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.running, run.SourceID)
	f.finished = append(f.finished, run)
	return run, nil
}

func (f *fakeSources) PurgeRuns(context.Context, time.Duration) (int64, error) {
	f.purged++
	return 0, nil
}

func (f *fakeSources) FailStaleRuns(context.Context, time.Duration) (int64, error) {
	f.stale++
	return 0, nil
}

type fakeListings struct {
	mu          sync.Mutex
	stored      []database.IngestedListing
	failOn      string
	closedCalls [][]string
	closeFailed int
}

func (f *fakeListings) UpsertListing(_ context.Context, in database.IngestedListing) (domain.Job, domain.JobListing, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if in.ExternalID == f.failOn {
		return domain.Job{}, domain.JobListing{}, fmt.Errorf("store: %w", domain.ErrInvalid)
	}
	f.stored = append(f.stored, in)
	return domain.Job{}, domain.JobListing{}, nil
}

func (f *fakeListings) CloseMissing(_ context.Context, _ string, seen []string, _ time.Time) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closedCalls = append(f.closedCalls, seen)
	return 0, nil
}

func (f *fakeListings) CloseFailedSources(context.Context, time.Duration) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closeFailed++
	return 0, nil
}

type fakeCompanies struct {
	mu       sync.Mutex
	outcomes map[string]domain.RunStatus
}

func (f *fakeCompanies) RecordSyncOutcome(_ context.Context, companyID string, run domain.IngestionRun) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.outcomes == nil {
		f.outcomes = map[string]domain.RunStatus{}
	}
	f.outcomes[companyID] = run.Status
	return nil
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
	sources.running = map[string]bool{source.ID: true}
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
	if err := r.RunDue(context.Background(), RunDueOptions{}); err != nil {
		t.Fatal(err)
	}
	if sources.stale != 1 || len(sources.finished) != 2 {
		t.Fatalf("stale=%d finished=%d", sources.stale, len(sources.finished))
	}
	if sources.finished[1].Status != domain.RunFailed || !strings.Contains(*sources.finished[1].Error, "unknown source adapter") {
		t.Fatalf("unknown adapters must fail their run: %+v", sources.finished[1])
	}
}

func TestSyncClosesMissingListingsAfterSuccess(t *testing.T) {
	bad := listing("bad")
	bad.Title = ""
	adapter := &fakeAdapter{results: []func() ([]RawListing, error){ok(listing("1"), bad, listing("2"))}}
	r, _, listings := newRunner(adapter)
	if _, err := r.Sync(context.Background(), source); err != nil {
		t.Fatal(err)
	}
	if len(listings.closedCalls) != 1 {
		t.Fatalf("CloseMissing calls = %d, want 1", len(listings.closedCalls))
	}
	// A listing that failed validation is still in the snapshot: not closed.
	got := strings.Join(listings.closedCalls[0], ",")
	if got != "1,bad,2" {
		t.Errorf("seen IDs = %s, want 1,bad,2", got)
	}
}

func TestFailedFetchDoesNotClose(t *testing.T) {
	adapter := &fakeAdapter{results: []func() ([]RawListing, error){fail(errors.New("board not found"))}}
	r, _, listings := newRunner(adapter)
	r.Sync(context.Background(), source)
	if len(listings.closedCalls) != 0 {
		t.Fatal("a failed fetch must not close listings")
	}
}

func TestAllListingsFailingDoesNotClose(t *testing.T) {
	adapter := &fakeAdapter{results: []func() ([]RawListing, error){ok(listing("1"))}}
	r, _, listings := newRunner(adapter)
	listings.failOn = "1"
	r.Sync(context.Background(), source)
	if len(listings.closedCalls) != 0 {
		t.Fatal("a failed run must not close listings")
	}
}

type slowAdapter struct{ delay time.Duration }

func (a slowAdapter) Validate(map[string]any) error { return nil }
func (a slowAdapter) Fetch(ctx context.Context, cfg map[string]any) ([]RawListing, error) {
	time.Sleep(a.delay)
	return []RawListing{listing(cfg["id"].(string))}, nil
}

func TestRunDueParallel(t *testing.T) {
	r, sources, _ := newRunner(slowAdapter{delay: 50 * time.Millisecond})
	r.Adapters = Registry{"slow": slowAdapter{delay: 50 * time.Millisecond}}
	companies := &fakeCompanies{}
	r.Companies = companies
	for i := range 10 {
		id := fmt.Sprintf("s%d", i)
		s := domain.JobSource{ID: id, Key: id, Adapter: "slow", Config: map[string]any{"id": id}}
		if i%2 == 0 {
			co := "co-" + id
			s.CompanyID = &co
		}
		sources.due = append(sources.due, s)
	}
	var mu sync.Mutex
	results, total := 0, 0
	start := time.Now()
	err := r.RunDue(context.Background(), RunDueOptions{Concurrency: 5, OnStart: func(n int) { total = n }, OnResult: func(_ domain.JobSource, run domain.IngestionRun, err error) {
		mu.Lock()
		defer mu.Unlock()
		if err != nil || run.Status != domain.RunSucceeded {
			t.Errorf("run %+v err %v", run, err)
		}
		results++
	}})
	if err != nil {
		t.Fatal(err)
	}
	// Serially this takes 500ms; allow slack for a busy machine.
	if d := time.Since(start); d > 400*time.Millisecond {
		t.Errorf("10 syncs of 50ms with concurrency 5 took %v", d)
	}
	if results != 10 || total != 10 || len(sources.finished) != 10 {
		t.Fatalf("results=%d finished=%d", results, len(sources.finished))
	}
	if len(companies.outcomes) != 5 || companies.outcomes["co-s0"] != domain.RunSucceeded {
		t.Fatalf("company outcomes = %v", companies.outcomes)
	}
}

func TestRunDueClosesFailedSources(t *testing.T) {
	r, sources, listings := newRunner(&fakeAdapter{results: []func() ([]RawListing, error){ok()}})
	for range 3 {
		if err := r.RunDue(context.Background(), RunDueOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	if listings.closeFailed != 1 {
		t.Fatalf("CloseFailedSources calls = %d, want 1 a day", listings.closeFailed)
	}
	if sources.purged != 1 {
		t.Fatalf("PurgeRuns calls = %d, want 1 a day", sources.purged)
	}
}

type noteAdapter struct {
	err     error
	timeout time.Duration
	gotDL   time.Duration
}

func (a *noteAdapter) Validate(map[string]any) error { return nil }
func (a *noteAdapter) Fetch(ctx context.Context, _ map[string]any) ([]RawListing, error) {
	if dl, ok := ctx.Deadline(); ok {
		a.gotDL = time.Until(dl)
	}
	return []RawListing{listing("1"), listing("2")}, a.err
}
func (a *noteAdapter) FetchTimeout(map[string]any) time.Duration { return a.timeout }

func TestIncompleteSnapshotIsStoredButClosesNothing(t *testing.T) {
	a := &noteAdapter{err: Incomplete("3 pages could not be read")}
	r, sources, listings := newRunner(a)
	r.Adapters = Registry{"fake": a}
	run, err := r.Sync(context.Background(), source)
	if err != nil || run.Status != domain.RunSucceeded || run.UpsertedCount != 2 {
		t.Fatalf("run = %+v err = %v", run, err)
	}
	if len(listings.closedCalls) != 0 {
		t.Fatal("an incomplete snapshot must not close listings")
	}
	if run.Error == nil || !strings.Contains(*run.Error, "3 pages could not be read") || len(sources.finished) != 1 {
		t.Fatalf("run error = %v", run.Error)
	}
}

func TestSnapshotNoteIsRecordedAndStillCloses(t *testing.T) {
	a := &noteAdapter{err: Note("1 page was not HTML")}
	r, _, listings := newRunner(a)
	r.Adapters = Registry{"fake": a}
	run, err := r.Sync(context.Background(), source)
	if err != nil || run.Status != domain.RunSucceeded || run.Error == nil || !strings.Contains(*run.Error, "not HTML") {
		t.Fatalf("run = %+v err = %v", run, err)
	}
	if len(listings.closedCalls) != 1 {
		t.Fatal("a complete snapshot with notes still closes missing listings")
	}
}

func TestAdapterCanAskForALongerFetchTimeout(t *testing.T) {
	a := &noteAdapter{timeout: time.Hour}
	r, _, _ := newRunner(a)
	r.Adapters = Registry{"fake": a}
	r.FetchTimeout = time.Minute
	r.Sync(context.Background(), source)
	if a.gotDL < 59*time.Minute {
		t.Fatalf("fetch deadline = %v, want the adapter's hour", a.gotDL)
	}
}

func TestRetryAfterIsHonoured(t *testing.T) {
	busy := Retryable(fmt.Errorf("list: %w", &crawl.StatusError{StatusCode: 429, RetryAfter: 80 * time.Millisecond}))
	adapter := &fakeAdapter{results: []func() ([]RawListing, error){fail(busy), ok(listing("1"))}}
	r, _, _ := newRunner(adapter)
	start := time.Now()
	run, err := r.Sync(context.Background(), source)
	if err != nil || run.Attempts != 2 {
		t.Fatalf("run = %+v err = %v", run, err)
	}
	if d := time.Since(start); d < 80*time.Millisecond {
		t.Errorf("retried after %v, want at least the 80ms Retry-After", d)
	}
}

func TestLongRetryAfterFailsTheRunNow(t *testing.T) {
	busy := Retryable(&crawl.StatusError{StatusCode: 429, RetryAfter: 10 * time.Minute})
	adapter := &fakeAdapter{results: []func() ([]RawListing, error){fail(busy)}}
	r, _, _ := newRunner(adapter)
	run, err := r.Sync(context.Background(), source)
	if err == nil || run.Attempts != 1 {
		t.Fatalf("run = %+v err = %v; want one attempt and a failed run", run, err)
	}
}
