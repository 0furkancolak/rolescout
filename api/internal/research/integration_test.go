package research_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"example.com/rolescout/api/internal/crawl"
	"example.com/rolescout/api/internal/discovery"
	"example.com/rolescout/api/internal/domain"
	"example.com/rolescout/api/internal/infrastructure/database"
	"example.com/rolescout/api/internal/infrastructure/database/dbtest"
	"example.com/rolescout/api/internal/ingest"
	"example.com/rolescout/api/internal/ingest/adapters"
	"example.com/rolescout/api/internal/research"
)

type rewrite struct{ target *url.URL }

func (r rewrite) RoundTrip(req *http.Request) (*http.Response, error) {
	out := req.Clone(req.Context())
	out.Host = req.URL.Host
	out.URL.Scheme, out.URL.Host = r.target.Scheme, r.target.Host
	return http.DefaultTransport.RoundTrip(out)
}

type staticSource []discovery.Candidate

func (s staticSource) Name() string { return "seed" }
func (s staticSource) Discover(_ context.Context, state string, fn func(discovery.Candidate) error) (string, error) {
	for _, c := range s {
		if err := fn(c); err != nil {
			return state, err
		}
	}
	return state, nil
}

// pipelineFixture is a pipeline over a fake internet and a throwaway database.
type pipelineFixture struct {
	ctx      context.Context
	pool     *pgxpool.Pool
	p        *research.Pipeline
	runs     *database.ResearchRepository
	ws       domain.Workspace
	profiles []domain.ResearchProfile
}

func newPipelineFixture(t *testing.T) pipelineFixture {
	admin := os.Getenv(dbtest.EnvVar)
	if admin == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	dbURL, drop, err := dbtest.Create(admin)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(drop)
	ctx := context.Background()
	pool, err := database.Open(ctx, dbURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := database.RunMigrations(ctx, pool, "../../migrations"); err != nil {
		t.Fatal(err)
	}

	// The shipped kariyer.net sources would read a site this fake internet does not have.
	if _, err := pool.Exec(ctx, `UPDATE job_sources SET enabled = FALSE WHERE adapter = 'kariyernet'`); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Host + r.URL.Path {
		case "boards-api.greenhouse.io/v1/boards/acme/jobs":
			fmt.Fprint(w, `{"jobs":[{"id":1,"title":"Senior Full Stack Engineer","absolute_url":"https://boards.greenhouse.io/acme/jobs/1",
				"content":"React, TypeScript and Node.js","location":{"name":"Remote"}},
				{"id":2,"title":"Office Manager","absolute_url":"https://boards.greenhouse.io/acme/jobs/2","content":"","location":{"name":"Berlin, Germany"}}]}`)
		case "beta.example/careers":
			fmt.Fprint(w, `<html><head><title>Beta</title></head><script type="application/ld+json">{"@type":"JobPosting","title":"Fullstack Developer",
				"url":"https://beta.example/careers/7","hiringOrganization":{"name":"Beta"},"description":"React and PostgreSQL",
				"jobLocation":{"address":{"addressLocality":"İstanbul","addressCountry":"TR"}}}</script></html>`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	target, _ := url.Parse(srv.URL)
	crawler := crawl.New(crawl.Options{AllowPrivate: true, UserAgent: "RoleScout/test", HostInterval: time.Millisecond,
		VendorHosts: crawl.DefaultVendorHosts, Transport: rewrite{target}})

	auth := database.NewAuthRepository(pool)
	_, ws, err := auth.Register(ctx, database.RegisterParams{Email: "a@example.com", DisplayName: "A", PasswordHash: "$argon2id$v=19$m=65536,t=3,p=2$c2FsdHNhbHRzYWx0$aGFzaGhhc2hoYXNoaGFzaGhhc2hoYXNoaGFzaGhhc2g", WorkspaceName: "W"})
	if err != nil {
		t.Fatal(err)
	}
	runs := database.NewResearchRepository(pool)
	companies := database.NewCompanyRepository(pool)
	jobs := database.NewJobRepository(pool)
	profiles, err := runs.ListProfiles(ctx, ws.ID)
	if err != nil || len(profiles) != 1 {
		t.Fatalf("profiles: %v %v", profiles, err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	p := &research.Pipeline{
		Runs: runs, Companies: companies, Jobs: jobs,
		Ingest: &ingest.Runner{Sources: database.NewSourceRepository(pool), Listings: jobs, Companies: companies,
			Adapters: adapters.Registry(crawler, adapters.Options{JSONLDMaxPages: 5}), Logger: logger},
		Detector: &discovery.Detector{Client: crawler},
		Sources: []discovery.Source{staticSource{
			{URL: "https://boards.greenhouse.io/acme", Name: "Acme", Origin: "seed"},
			{URL: "http://beta.example/careers", Name: "Beta", Origin: "probe"},
			{URL: "http://gone.example/careers", Origin: "seed"},
		}},
		Config:        research.Config{DiscoveryInterval: time.Hour, Concurrency: 4, MatchMinScore: 40, CompanySyncInterval: 12 * time.Hour},
		Logger:        logger,
		FlushInterval: 20 * time.Millisecond,
	}

	return pipelineFixture{ctx: ctx, pool: pool, p: p, runs: runs, ws: ws, profiles: profiles}
}

func TestPipelineEndToEnd(t *testing.T) {
	f := newPipelineFixture(t)
	ctx, p, runs, ws, profiles := f.ctx, f.p, f.runs, f.ws, f.profiles
	run, err := runs.EnqueueRun(ctx, ws.ID, profiles[0].ID, "manual")
	if err != nil {
		t.Fatal(err)
	}
	p.Tick(ctx)
	got, err := runs.GetRun(ctx, ws.ID, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != domain.RunSucceeded {
		t.Fatalf("run = %+v (error %v)", got, got.Error)
	}
	if got.Summary["companies_known"] != 3 || got.Summary["jobs_collected"] != 3 || got.Summary["matches"] != 2 ||
		got.Summary["companies_failed"] != 1 {
		t.Errorf("summary = %v", got.Summary)
	}
	for _, s := range domain.Stages {
		if st := got.Stages[s].State; st != "done" {
			t.Errorf("stage %s = %s", s, st)
		}
	}
	events, err := runs.ListEvents(ctx, ws.ID, run.ID, 0, "", 500)
	if err != nil || len(events) == 0 {
		t.Fatalf("events: %d %v", len(events), err)
	}
	var log strings.Builder
	for _, e := range events {
		log.WriteString(e.Level + " " + e.Company + " " + e.Message + "\n")
	}
	for _, want := range []string{"greenhouse · 2 jobs", "jsonld · 1 jobs", "responded 404"} {
		if !strings.Contains(log.String(), want) {
			t.Errorf("log misses %q:\n%s", want, log.String())
		}
	}

	// A second run reuses the fresh catalog and finds nothing new.
	run2, _ := runs.EnqueueRun(ctx, ws.ID, profiles[0].ID, "manual")
	p.Tick(ctx)
	got2, _ := runs.GetRun(ctx, ws.ID, run2.ID)
	if got2.State != domain.RunSucceeded || got2.Stages[domain.StageDiscovery].State != "skipped" || got2.Summary["new_matches"] != 0 {
		t.Errorf("second run = %+v", got2)
	}
}

// growingSource is a source with new work on every call, like a seed list a
// release extended.
type growingSource struct{ staticSource }

func (growingSource) Pending(string) bool { return true }

func TestPendingSourceRunsDiscoveryDespiteFreshCatalog(t *testing.T) {
	f := newPipelineFixture(t)
	ctx, p, runs := f.ctx, f.p, f.runs
	if _, err := runs.EnqueueRun(ctx, f.ws.ID, f.profiles[0].ID, "manual"); err != nil {
		t.Fatal(err)
	}
	p.Tick(ctx)
	// Same catalog, fresh: the next run skips discovery...
	run2, _ := runs.EnqueueRun(ctx, f.ws.ID, f.profiles[0].ID, "manual")
	p.Tick(ctx)
	if got, _ := runs.GetRun(ctx, f.ws.ID, run2.ID); got.Stages[domain.StageDiscovery].State != "skipped" {
		t.Fatalf("fresh catalog: discovery = %s", got.Stages[domain.StageDiscovery].State)
	}
	// ...unless a source reports pending work.
	p.Sources = []discovery.Source{growingSource{p.Sources[0].(staticSource)}}
	run3, _ := runs.EnqueueRun(ctx, f.ws.ID, f.profiles[0].ID, "manual")
	p.Tick(ctx)
	if got, _ := runs.GetRun(ctx, f.ws.ID, run3.ID); got.Stages[domain.StageDiscovery].State != "done" {
		t.Fatalf("pending source: discovery = %s", got.Stages[domain.StageDiscovery].State)
	}
}
