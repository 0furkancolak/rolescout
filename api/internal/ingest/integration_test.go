package ingest_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"example.com/rolescout/api/internal/crawl"
	"example.com/rolescout/api/internal/domain"
	"example.com/rolescout/api/internal/infrastructure/database"
	"example.com/rolescout/api/internal/infrastructure/database/dbtest"
	"example.com/rolescout/api/internal/ingest"
	"example.com/rolescout/api/internal/ingest/greenhouse"
)

var testPool *pgxpool.Pool

func TestMain(m *testing.M) {
	adminURL := os.Getenv(dbtest.EnvVar)
	if adminURL == "" {
		os.Exit(m.Run())
	}
	dbURL, drop, err := dbtest.Create(adminURL)
	if err != nil {
		log.Fatalf("database test setup: %v", err)
	}
	code := func() int {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		testPool, err = database.Open(ctx, dbURL)
		if err != nil {
			log.Printf("open test database: %v", err)
			return 1
		}
		defer testPool.Close()
		if err := database.RunMigrations(ctx, testPool, "../../migrations"); err != nil {
			log.Printf("migrate test database: %v", err)
			return 1
		}
		return m.Run()
	}()
	drop()
	os.Exit(code)
}

func testCrawler() *crawl.Client {
	return crawl.New(crawl.Options{UserAgent: "RoleScout/test", HostInterval: time.Millisecond})
}

func uniq(prefix string) string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return prefix + hex.EncodeToString(b)
}

// board serves a Greenhouse board whose jobs can change between requests.
type board struct {
	body  atomic.Value
	fails atomic.Int32
}

func (b *board) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/robots.txt" {
		http.NotFound(w, r)
		return
	}
	if b.fails.Load() > 0 {
		b.fails.Add(-1)
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	w.Write([]byte(b.body.Load().(string)))
}

func TestIngestionIsIdempotentAndDeduplicatesAcrossSources(t *testing.T) {
	if testPool == nil {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	sources := database.NewSourceRepository(testPool)
	jobs := database.NewJobRepository(testPool)

	title := uniq("Platform Engineer ")
	gh := &board{}
	gh.body.Store(`{"jobs":[{"id":1,"title":"` + title + `","updated_at":"2026-10-15T10:00:00Z",
		"absolute_url":"https://boards.greenhouse.io/acme/jobs/1","content":"&lt;p&gt;Golang and Kafka&lt;/p&gt;",
		"location":{"name":"Remote"}}]}`)
	srv := httptest.NewServer(gh)
	defer srv.Close()

	adapter := &greenhouse.Adapter{BaseURL: srv.URL, Client: testCrawler()}
	runner := &ingest.Runner{
		Sources: sources, Listings: jobs,
		Adapters: ingest.Registry{"greenhouse": adapter},
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		Backoff:  func(int) time.Duration { return time.Millisecond },
	}

	newSource := func(company string) domain.JobSource {
		s, err := sources.Create(ctx, database.SourceParams{
			Key: uniq("gh-"), Name: company, BaseURL: "https://boards.greenhouse.io/acme", Adapter: "greenhouse",
			Config: map[string]any{"board_token": "acme", "company": company},
		})
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	first := newSource("Acme")

	gh.fails.Store(1) // the first request fails and is retried
	run, err := runner.Sync(ctx, first)
	if err != nil || run.Attempts != 2 || run.UpsertedCount != 1 {
		t.Fatalf("first sync: err=%v run=%+v", err, run)
	}

	found, _, err := jobs.SearchJobs(ctx, database.JobFilter{Query: title, Limit: 10})
	if err != nil || len(found) != 1 {
		t.Fatalf("search after first sync: err=%v jobs=%d", err, len(found))
	}
	job := found[0]
	if job.WorkArrangement != "remote" || job.Description != "Golang and Kafka" || strings.Join(job.Technologies, ",") != "go,kafka" {
		t.Fatalf("job not normalized: %+v", job)
	}

	// Syncing again updates the same rows.
	if _, err := runner.Sync(ctx, first); err != nil {
		t.Fatal(err)
	}
	// A second source publishing the same job maps to the same canonical job.
	second := newSource("ACME")
	if _, err := runner.Sync(ctx, second); err != nil {
		t.Fatal(err)
	}

	found, _, err = jobs.SearchJobs(ctx, database.JobFilter{Query: title, Limit: 10})
	if err != nil || len(found) != 1 || found[0].ID != job.ID {
		t.Fatalf("expected one deduplicated job, got %d (err=%v)", len(found), err)
	}
	listings, err := jobs.ListListings(ctx, job.ID)
	if err != nil || len(listings) != 2 {
		t.Fatalf("expected one listing per source, got %d (err=%v)", len(listings), err)
	}
	if !found[0].LastSeenAt.After(job.LastSeenAt) && !found[0].LastSeenAt.Equal(job.LastSeenAt) {
		t.Fatal("last_seen_at must not move backwards")
	}

	runs, err := sources.ListRuns(ctx, first.ID, 10)
	if err != nil || len(runs) != 2 || runs[0].Status != domain.RunSucceeded {
		t.Fatalf("runs: err=%v %+v", err, runs)
	}
}

func TestSyncClosesAndReopensJobs(t *testing.T) {
	if testPool == nil {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	sources := database.NewSourceRepository(testPool)
	jobs := database.NewJobRepository(testPool)
	companies := database.NewCompanyRepository(testPool)

	a, b := uniq("Closing A "), uniq("Closing B ")
	jobJSON := func(id int, title string) string {
		return fmt.Sprintf(`{"id":%d,"title":%q,"absolute_url":"https://boards.greenhouse.io/acme/jobs/%d","content":"","location":{"name":"Berlin, Germany"}}`, id, title, id)
	}
	gh := &board{}
	gh.body.Store(`{"jobs":[` + jobJSON(1, a) + `,` + jobJSON(2, b) + `]}`)
	srv := httptest.NewServer(gh)
	defer srv.Close()

	company, _, err := companies.UpsertDiscovered(ctx, database.CompanyParams{
		Name: "Acme", CareersURL: "https://boards.greenhouse.io/" + uniq("acme"), ATS: "greenhouse", Origin: "seed",
	})
	if err != nil {
		t.Fatal(err)
	}
	src, err := companies.AttachSource(ctx, company.ID, database.SourceParams{
		Name: "Acme", BaseURL: company.CareersURL, Adapter: "greenhouse",
		Config: map[string]any{"board_token": "acme", "company": "Acme"},
	})
	if err != nil {
		t.Fatal(err)
	}
	runner := &ingest.Runner{
		Sources: sources, Listings: jobs, Companies: companies,
		Adapters: ingest.Registry{"greenhouse": &greenhouse.Adapter{BaseURL: srv.URL, Client: testCrawler()}},
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		Backoff:  func(int) time.Duration { return time.Millisecond },
	}
	find := func(title string) domain.Job {
		t.Helper()
		found, _, err := jobs.SearchJobs(ctx, database.JobFilter{Query: title, Limit: 10})
		if err != nil || len(found) != 1 {
			t.Fatalf("search %q: %v %d", title, err, len(found))
		}
		return found[0]
	}

	if _, err := runner.Sync(ctx, src); err != nil {
		t.Fatal(err)
	}
	jobA := find(a)
	if jobA.CompanyID == nil || *jobA.CompanyID != company.ID || jobA.CountryCode == nil || *jobA.CountryCode != "DE" || jobA.ClosedAt != nil {
		t.Fatalf("job after first sync: %+v", jobA)
	}
	co, _ := companies.Get(ctx, company.ID)
	if co.Status != domain.CompanyActive || co.OpenJobs != 2 {
		t.Fatalf("company after sync: %+v", co)
	}

	gh.body.Store(`{"jobs":[` + jobJSON(1, a) + `]}`)
	if _, err := runner.Sync(ctx, src); err != nil {
		t.Fatal(err)
	}
	if find(b).ClosedAt == nil || find(a).ClosedAt != nil {
		t.Fatal("B must be closed and A open after B disappeared")
	}
	if co, _ = companies.Get(ctx, company.ID); co.OpenJobs != 1 {
		t.Fatalf("open jobs = %d, want 1", co.OpenJobs)
	}

	// A failed fetch keeps everything as it is.
	gh.fails.Store(10)
	runner.Sync(ctx, src)
	gh.fails.Store(0)
	if find(a).ClosedAt != nil {
		t.Fatal("a failed sync must not close jobs")
	}

	gh.body.Store(`{"jobs":[` + jobJSON(1, a) + `,` + jobJSON(2, b) + `]}`)
	if _, err := runner.Sync(ctx, src); err != nil {
		t.Fatal(err)
	}
	if find(b).ClosedAt != nil {
		t.Fatal("B must reopen when listed again")
	}
}

func TestCloseFailedSourcesAfterSevenDays(t *testing.T) {
	if testPool == nil {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	sources := database.NewSourceRepository(testPool)
	jobs := database.NewJobRepository(testPool)

	setup := func(lastSuccessDaysAgo int) domain.Job {
		s, err := sources.Create(ctx, database.SourceParams{Key: uniq("dead-"), Name: "Dead", BaseURL: "https://x.example", Adapter: "greenhouse"})
		if err != nil {
			t.Fatal(err)
		}
		job, _, err := jobs.UpsertListing(ctx, database.IngestedListing{
			SourceID: s.ID, ExternalID: "1", URL: "https://x.example/1", DedupeKey: uniq("dk"),
			Title: uniq("Dead job "), CompanyName: "Dead",
		})
		if err != nil {
			t.Fatal(err)
		}
		_, err = testPool.Exec(ctx, `
			INSERT INTO ingestion_runs (source_id, status, started_at, finished_at, error) VALUES
			($1, 'succeeded', NOW() - make_interval(days => $2), NOW() - make_interval(days => $2), NULL),
			($1, 'failed', NOW() - INTERVAL '1 hour', NOW() - INTERVAL '1 hour', 'board not found')`,
			s.ID, lastSuccessDaysAgo)
		if err != nil {
			t.Fatal(err)
		}
		return job
	}
	old, recent := setup(8), setup(6)
	if _, err := jobs.CloseFailedSources(ctx, 7*24*time.Hour); err != nil {
		t.Fatal(err)
	}
	get := func(id string) domain.Job {
		j, err := jobs.GetJob(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		return j
	}
	if get(old.ID).ClosedAt == nil {
		t.Error("job of a source failing for 8 days must be closed")
	}
	if get(recent.ID).ClosedAt != nil {
		t.Error("job of a source failing for 6 days must stay open")
	}
}
