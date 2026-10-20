package research

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"example.com/rolescout/api/internal/discovery"
	"example.com/rolescout/api/internal/domain"
)

type harness struct {
	p         *Pipeline
	runs      *fakeRuns
	companies *fakeCompanies
	ingest    *fakeIngest
}

func newHarness() harness {
	runs, companies := newFakeRuns(), newFakeCompanies()
	ing := &fakeIngest{companies: companies, fail: map[string]bool{}}
	lever := discovery.Board{ATS: "lever", ID: "beta", Config: map[string]any{"site": "beta"}}
	p := &Pipeline{
		Runs: runs, Companies: companies, Ingest: ing,
		Jobs: &fakeJobs{jobs: []domain.Job{
			job("Senior Full Stack Engineer", "remote", "react", "typescript"),
			job("Accountant", "onsite"),
		}},
		Detector: fakeDetector{"https://beta.example/careers": {Board: lever, FinalURL: "https://beta.example/careers", Name: "Beta"}},
		Sources: []discovery.Source{
			fakeSource{name: "seed", cands: []discovery.Candidate{
				{URL: "https://boards.greenhouse.io/acme", Name: "Acme", Origin: "seed"},
				{URL: "https://beta.example/careers", Name: "Beta", Origin: "seed"},
				{URL: "https://blocked.example/careers", Origin: "seed"},
				{URL: "https://nothing.example/careers", Origin: "seed"},
			}},
			fakeSource{name: "hackernews", err: errors.New("hn down")},
		},
		Config:        Config{DiscoveryInterval: 7 * 24 * time.Hour, Concurrency: 2, MatchMinScore: 40, CompanySyncInterval: 12 * time.Hour},
		Logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
		Now:           func() time.Time { return now },
		FlushInterval: 5 * time.Millisecond,
		LockWait:      5 * time.Millisecond,
	}
	return harness{p: p, runs: runs, companies: companies, ingest: ing}
}

var testRun = domain.ResearchRun{ID: "run-1", WorkspaceID: "ws", ProfileID: "prof", State: domain.RunRunning}

func lastSaved(r *fakeRuns) domain.ResearchRun {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.saved[len(r.saved)-1]
}

func TestExecuteRunsAllStages(t *testing.T) {
	h := newHarness()
	if err := h.p.Execute(context.Background(), testRun); err != nil {
		t.Fatal(err)
	}
	if h.runs.finished["run-1"] != domain.RunSucceeded {
		t.Fatalf("state = %s (%s)", h.runs.finished["run-1"], h.runs.errs["run-1"])
	}
	run := lastSaved(h.runs)
	for _, s := range domain.Stages {
		if st := run.Stages[s].State; st != "done" {
			t.Errorf("stage %s = %q, want done", s, st)
		}
	}
	disc := run.Stages[domain.StageDiscovery]
	if disc.Detail["seed"] != 4 || disc.Failed != 1 {
		t.Errorf("discovery = %+v", disc)
	}
	det := run.Stages[domain.StageDetection]
	if det.Done != 4 || det.Failed != 2 || det.Detail["greenhouse"] != 1 || det.Detail["lever"] != 1 {
		t.Errorf("detection = %+v", det)
	}
	if len(h.companies.attached) != 2 {
		t.Errorf("attached sources = %v", h.companies.attached)
	}
	col := run.Stages[domain.StageCollection]
	if col.Total != 2 || col.Done != 2 || col.Failed != 0 {
		t.Errorf("collection = %+v", col)
	}
	if len(h.runs.matches) != 1 || h.runs.matches[0].Score < 40 || h.companies.tiers != 1 {
		t.Errorf("matches = %+v tiers = %d", h.runs.matches, h.companies.tiers)
	}
	sum := h.runs.summaries["run-1"]
	if sum["matches"] != 1 || sum["jobs_open"] != 2 || sum["jobs_collected"] != 6 || sum["companies_known"] != 4 {
		t.Errorf("summary = %v", sum)
	}
	if h.runs.state["discovery.seed"] != "next-seed" || h.runs.state["discovery.last_full"] == "" {
		t.Errorf("discovery state = %v", h.runs.state)
	}
	var sawBlocked, sawHN bool
	for _, e := range h.runs.events {
		if strings.Contains(e.Message, "robots.txt") {
			sawBlocked = true
		}
		if e.Level == "error" && strings.Contains(e.Message, "hn down") {
			sawHN = true
		}
	}
	if !sawBlocked || !sawHN {
		t.Errorf("events missing robots or source failure: %+v", h.runs.events)
	}
	if h.runs.lockHeld {
		t.Error("stage lock still held")
	}
}

func TestExecuteSkipsFreshDiscovery(t *testing.T) {
	h := newHarness()
	h.runs.state["discovery.last_full"] = now.Add(-time.Hour).Format(time.RFC3339)
	if err := h.p.Execute(context.Background(), testRun); err != nil {
		t.Fatal(err)
	}
	disc := lastSaved(h.runs).Stages[domain.StageDiscovery]
	if disc.State != "skipped" || !strings.Contains(disc.Note, "fresh") {
		t.Fatalf("discovery = %+v", disc)
	}
}

func TestCancelDuringCollection(t *testing.T) {
	h := newHarness()
	h.ingest.block = make(chan struct{})
	done := make(chan error, 1)
	go func() { done <- h.p.Execute(context.Background(), testRun) }()
	time.Sleep(50 * time.Millisecond)
	h.runs.mu.Lock()
	h.runs.cancel = true
	h.runs.mu.Unlock()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("run did not stop after cancel")
	}
	if h.runs.finished["run-1"] != domain.RunCancelled {
		t.Fatalf("state = %s", h.runs.finished["run-1"])
	}
	if h.runs.lockHeld {
		t.Error("stage lock still held after cancel")
	}
}

func TestStoppingWorkerPutsTheRunBackInTheQueue(t *testing.T) {
	h := newHarness()
	h.ingest.block = make(chan struct{})
	ctx, stop := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- h.p.Execute(ctx, testRun) }()
	time.Sleep(50 * time.Millisecond)
	stop()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("run did not stop")
	}
	h.runs.mu.Lock()
	defer h.runs.mu.Unlock()
	if len(h.runs.requeued) != 1 || h.runs.requeued[0] != "run-1" {
		t.Fatalf("requeued = %v", h.runs.requeued)
	}
	if _, finished := h.runs.finished["run-1"]; finished {
		t.Fatal("an interrupted run must not be finished as failed")
	}
}

func TestDetectorPanicFailsOnlyThatCompany(t *testing.T) {
	h := newHarness()
	src := h.p.Sources[0].(fakeSource)
	src.cands = append(src.cands, discovery.Candidate{URL: "https://panic.example/careers", Origin: "seed"})
	h.p.Sources[0] = src
	if err := h.p.Execute(context.Background(), testRun); err != nil {
		t.Fatal(err)
	}
	if h.runs.finished["run-1"] != domain.RunSucceeded {
		t.Fatalf("state = %s (%s)", h.runs.finished["run-1"], h.runs.errs["run-1"])
	}
	if det := lastSaved(h.runs).Stages[domain.StageDetection]; det.Failed != 3 {
		t.Errorf("detection failures = %d, want 3", det.Failed)
	}
}

func TestWaitsForStageLock(t *testing.T) {
	h := newHarness()
	h.runs.lockHeld = true
	done := make(chan error, 1)
	go func() { done <- h.p.Execute(context.Background(), testRun) }()
	time.Sleep(40 * time.Millisecond)
	h.runs.mu.Lock()
	waits := h.runs.lockWaits
	h.runs.lockHeld = false
	h.runs.mu.Unlock()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if waits < 2 || h.runs.finished["run-1"] != domain.RunSucceeded {
		t.Fatalf("waits = %d state = %s", waits, h.runs.finished["run-1"])
	}
}

func TestDuplicateBoardDisablesCompany(t *testing.T) {
	h := newHarness()
	h.companies.conflicts["beta"] = true
	if err := h.p.Execute(context.Background(), testRun); err != nil {
		t.Fatal(err)
	}
	var beta string
	for _, c := range h.companies.byURL {
		if c.CareersURL == "https://beta.example/careers" {
			beta = c.ID
		}
	}
	if !h.companies.disabled[beta] {
		t.Fatal("a company whose board belongs to another company must be disabled")
	}
}

func TestTickEnqueuesDueProfilesAndRunsThem(t *testing.T) {
	h := newHarness()
	h.p.Config.ClosedJobRetention = 90 * 24 * time.Hour
	h.p.Tick(context.Background())
	if len(h.runs.enqueued) != 1 || h.runs.enqueued[0] != "schedule" {
		t.Fatalf("enqueued = %v", h.runs.enqueued)
	}
	if h.runs.finished["run-1"] != domain.RunSucceeded || h.runs.stale != 1 || h.runs.purged != 1 {
		t.Fatalf("finished = %v stale = %d purged = %d", h.runs.finished, h.runs.stale, h.runs.purged)
	}
	h.p.Tick(context.Background())
	if h.runs.purged != 1 {
		t.Error("purge must run at most once a day")
	}
}

func TestDiscoveryLogsProgressEveryHundredCompanies(t *testing.T) {
	h := newHarness()
	var cands []discovery.Candidate
	for i := range 250 {
		cands = append(cands, discovery.Candidate{URL: fmt.Sprintf("https://c%d.example/careers", i), Origin: "seed"})
	}
	h.p.Sources = []discovery.Source{fakeSource{name: "seed", cands: cands}}
	if err := h.p.Execute(context.Background(), testRun); err != nil {
		t.Fatal(err)
	}
	var progress []string
	for _, e := range h.runs.events {
		if strings.Contains(e.Message, "so far") {
			progress = append(progress, e.Message)
		}
	}
	if strings.Join(progress, "|") != "seed: 100 new companies so far|seed: 200 new companies so far" {
		t.Fatalf("progress events = %v", progress)
	}
}

// gateSource waits until every other gateSource has started, which only
// happens when the sources run at the same time.
type gateSource struct {
	name    string
	started chan<- string
	all     <-chan struct{}
}

func (g gateSource) Name() string { return g.name }
func (g gateSource) Discover(ctx context.Context, state string, fn func(discovery.Candidate) error) (string, error) {
	g.started <- g.name
	select {
	case <-g.all:
		return "ok", fn(discovery.Candidate{URL: "https://" + g.name + ".example/careers", Name: g.name, Origin: "seed"})
	case <-time.After(3 * time.Second):
		return state, errors.New("the other source never started: sources run one after another")
	}
}

func TestDiscoverySourcesRunAtTheSameTime(t *testing.T) {
	h := newHarness()
	started := make(chan string, 3)
	all := make(chan struct{})
	h.p.Sources = []discovery.Source{
		gateSource{"alpha", started, all}, gateSource{"beta", started, all}, gateSource{"gamma", started, all},
	}
	go func() {
		for range 3 {
			<-started
		}
		close(all)
	}()
	if err := h.p.Execute(context.Background(), testRun); err != nil {
		t.Fatal(err)
	}
	disc := lastSaved(h.runs).Stages[domain.StageDiscovery]
	if disc.Failed != 0 || disc.Detail["alpha"] != 1 || disc.Detail["beta"] != 1 || disc.Detail["gamma"] != 1 {
		t.Fatalf("discovery = %+v", disc)
	}
}
