package database

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"example.com/rolescout/api/internal/domain"
)

func profileParams(name string) ProfileParams {
	return ProfileParams{
		Name: name, TitleKeywords: []string{"full stack"}, Technologies: []string{"react", "go"},
		Countries: []string{"TR"}, Remote: domain.RemoteAny, ScheduleInterval: 12 * time.Hour,
	}
}

func TestNewWorkspaceGetsDefaultProfile(t *testing.T) {
	f := newFixture(t)
	repo := NewResearchRepository(requireDB(t))
	ps, err := repo.ListProfiles(context.Background(), f.workspace.ID)
	if err != nil || len(ps) != 1 {
		t.Fatalf("profiles = %+v, err = %v", ps, err)
	}
	p := ps[0]
	if p.Name != "Full Stack Developer" || len(p.TitleKeywords) != 3 || p.Remote != domain.RemoteAny ||
		p.ScheduleInterval != 12*time.Hour || len(p.Technologies) == 0 {
		t.Errorf("default profile = %+v", p)
	}
}

func TestProfileCRUDAndIsolation(t *testing.T) {
	f1, f2 := newFixture(t), newFixture(t)
	ctx := context.Background()
	repo := NewResearchRepository(requireDB(t))

	p, err := repo.CreateProfile(ctx, f1.workspace.ID, profileParams("Backend"))
	if err != nil || p.Name != "Backend" || p.Countries[0] != "TR" {
		t.Fatalf("create: %+v %v", p, err)
	}
	if _, err := repo.CreateProfile(ctx, f1.workspace.ID, profileParams("Backend")); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("duplicate name: err = %v", err)
	}
	if _, err := repo.GetProfile(ctx, f2.workspace.ID, p.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("other workspace get: err = %v", err)
	}
	upd := profileParams("Backend Go")
	upd.Remote = domain.RemoteOnly
	p, err = repo.UpdateProfile(ctx, f1.workspace.ID, p.ID, upd)
	if err != nil || p.Name != "Backend Go" || p.Remote != domain.RemoteOnly {
		t.Fatalf("update: %+v %v", p, err)
	}
	if _, err := repo.UpdateProfile(ctx, f2.workspace.ID, p.ID, upd); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("other workspace update: err = %v", err)
	}
	if err := repo.DeleteProfile(ctx, f2.workspace.ID, p.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("other workspace delete: err = %v", err)
	}
	if err := repo.DeleteProfile(ctx, f1.workspace.ID, p.ID); err != nil {
		t.Fatal(err)
	}
}

func TestEnqueueOnePerProfileAndCancel(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	repo := NewResearchRepository(requireDB(t))
	p, _ := repo.CreateProfile(ctx, f.workspace.ID, profileParams(uniq("p")))

	run, err := repo.EnqueueRun(ctx, f.workspace.ID, p.ID, "manual")
	if err != nil || run.State != domain.RunQueued || run.Trigger != "manual" {
		t.Fatalf("enqueue: %+v %v", run, err)
	}
	if _, err := repo.EnqueueRun(ctx, f.workspace.ID, p.ID, "manual"); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("second enqueue: err = %v", err)
	}
	// Cancelling a queued run finishes it at once.
	if err := repo.RequestCancel(ctx, f.workspace.ID, run.ID); err != nil {
		t.Fatal(err)
	}
	got, _ := repo.GetRun(ctx, f.workspace.ID, run.ID)
	if got.State != domain.RunCancelled || got.FinishedAt == nil {
		t.Fatalf("cancelled queued run: %+v", got)
	}
	if err := repo.RequestCancel(ctx, f.workspace.ID, run.ID); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("cancel finished run: err = %v", err)
	}
	if _, err := repo.EnqueueRun(ctx, f.workspace.ID, p.ID, "manual"); err != nil {
		t.Fatalf("enqueue after cancel: %v", err)
	}
}

func TestClaimRunSkipLockedAndProgress(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	repo := NewResearchRepository(requireDB(t))
	// Drain runs left by other tests so the claims below see only ours.
	for {
		r, ok, err := repo.ClaimRun(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			break
		}
		repo.FinishRun(ctx, r.ID, domain.RunFailed, strp("drained"), nil)
	}
	p1, _ := repo.CreateProfile(ctx, f.workspace.ID, profileParams(uniq("a")))
	p2, _ := repo.CreateProfile(ctx, f.workspace.ID, profileParams(uniq("b")))
	repo.EnqueueRun(ctx, f.workspace.ID, p1.ID, "manual")
	repo.EnqueueRun(ctx, f.workspace.ID, p2.ID, "schedule")

	var mu sync.Mutex
	claimed := map[string]bool{}
	var wg sync.WaitGroup
	for range 3 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, ok, err := repo.ClaimRun(ctx)
			if err != nil {
				t.Error(err)
			}
			if ok {
				mu.Lock()
				claimed[r.ID] = true
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if len(claimed) != 2 {
		t.Fatalf("claimed %d runs, want 2 distinct", len(claimed))
	}
	var id string
	for k := range claimed {
		id = k
	}
	run, _ := repo.GetRun(ctx, f.workspace.ID, id)
	if run.State != domain.RunRunning || run.StartedAt == nil {
		t.Fatalf("claimed run: %+v", run)
	}
	run.Stage = domain.StageCollection
	run.Stages = map[domain.Stage]domain.StageProgress{
		domain.StageDiscovery:  {State: "done", Total: 10, Done: 10, Detail: map[string]int{"greenhouse": 4}},
		domain.StageCollection: {State: "running", Total: 8, Done: 3, Failed: 1},
	}
	if err := repo.SaveProgress(ctx, run); err != nil {
		t.Fatal(err)
	}
	if err := repo.RequestCancel(ctx, f.workspace.ID, id); err != nil {
		t.Fatal(err)
	}
	if c, err := repo.CancelRequested(ctx, id); err != nil || !c {
		t.Fatalf("cancel requested = %v, %v", c, err)
	}
	if err := repo.FinishRun(ctx, id, domain.RunSucceeded, nil, map[string]int{"matches": 5}); err != nil {
		t.Fatal(err)
	}
	got, _ := repo.GetRun(ctx, f.workspace.ID, id)
	if got.State != domain.RunSucceeded || got.Stages[domain.StageDiscovery].Detail["greenhouse"] != 4 ||
		got.Stages[domain.StageCollection].Failed != 1 || got.Summary["matches"] != 5 || got.FinishedAt == nil {
		t.Fatalf("finished run: %+v", got)
	}
	prof, _ := repo.GetProfile(ctx, f.workspace.ID, got.ProfileID)
	if prof.LastRunAt == nil {
		t.Error("a succeeded run must set the profile's last run time")
	}
	runs, err := repo.ListRuns(ctx, f.workspace.ID, got.ProfileID, 10)
	if err != nil || len(runs) != 1 {
		t.Fatalf("list runs: %d %v", len(runs), err)
	}
}

func TestFailStaleResearchRuns(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	pool := requireDB(t)
	repo := NewResearchRepository(pool)
	p, _ := repo.CreateProfile(ctx, f.workspace.ID, profileParams(uniq("s")))
	run, _ := repo.EnqueueRun(ctx, f.workspace.ID, p.ID, "manual")
	pool.Exec(ctx, `UPDATE research_runs SET state = 'running', started_at = NOW(), updated_at = NOW() - INTERVAL '1 hour' WHERE id = $1`, run.ID)
	if n, err := repo.FailStaleRuns(ctx, 15*time.Minute); err != nil || n < 1 {
		t.Fatalf("stale runs = %d, %v", n, err)
	}
	got, _ := repo.GetRun(ctx, f.workspace.ID, run.ID)
	if got.State != domain.RunFailed || got.Error == nil {
		t.Fatalf("stale run: %+v", got)
	}
}

func TestEndedRunsDoNotKeepARunningStage(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	pool := requireDB(t)
	repo := NewResearchRepository(pool)
	for _, tc := range []struct {
		end  domain.RunStatus
		want string
	}{{domain.RunFailed, "failed"}, {domain.RunCancelled, "skipped"}, {domain.RunSucceeded, "running"}} {
		p, _ := repo.CreateProfile(ctx, f.workspace.ID, profileParams(uniq("settle")))
		run, _ := repo.EnqueueRun(ctx, f.workspace.ID, p.ID, "manual")
		pool.Exec(ctx, `UPDATE research_runs SET state = 'running', started_at = NOW() WHERE id = $1`, run.ID)
		run.Stages = map[domain.Stage]domain.StageProgress{
			domain.StageDiscovery: {State: "done"}, domain.StageDetection: {State: "running", Total: 4, Done: 1},
		}
		if err := repo.SaveProgress(ctx, run); err != nil {
			t.Fatal(err)
		}
		if err := repo.FinishRun(ctx, run.ID, tc.end, nil, nil); err != nil {
			t.Fatal(err)
		}
		got, _ := repo.GetRun(ctx, f.workspace.ID, run.ID)
		if got.Stages[domain.StageDetection].State != tc.want || got.Stages[domain.StageDiscovery].State != "done" ||
			got.Stages[domain.StageDetection].Done != 1 {
			t.Errorf("%s: stages = %+v", tc.end, got.Stages)
		}
	}
}

func TestDueProfiles(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	repo := NewResearchRepository(requireDB(t))
	p, _ := repo.CreateProfile(ctx, f.workspace.ID, profileParams(uniq("due")))
	due := func() bool {
		ps, err := repo.DueProfiles(ctx, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range ps {
			if d.ID == p.ID {
				return true
			}
		}
		return false
	}
	if !due() {
		t.Fatal("a profile that never ran is due")
	}
	run, _ := repo.EnqueueRun(ctx, f.workspace.ID, p.ID, "schedule")
	if due() {
		t.Fatal("a profile with an active run is not due")
	}
	repo.RequestCancel(ctx, f.workspace.ID, run.ID)
	if due() {
		t.Fatal("a profile whose last run was queued just now is not due")
	}
}

func TestPausedProfileIsNeverDueAndRunsComeBack(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	repo := NewResearchRepository(requireDB(t))
	params := profileParams(uniq("pause"))
	params.SchedulePaused = true
	p, err := repo.CreateProfile(ctx, f.workspace.ID, params)
	if err != nil || !p.SchedulePaused {
		t.Fatalf("paused = %v, err = %v", p.SchedulePaused, err)
	}
	ps, _ := repo.DueProfiles(ctx, time.Now())
	for _, d := range ps {
		if d.ID == p.ID {
			t.Fatal("a paused profile is due")
		}
	}
	// A run interrupted by a stopping worker goes back to the queue.
	run, _ := repo.EnqueueRun(ctx, f.workspace.ID, p.ID, "manual")
	if _, ok, err := repo.ClaimRun(ctx); err != nil || !ok {
		t.Fatalf("claim: %v %v", ok, err)
	}
	if err := repo.RequeueRun(ctx, run.ID); err != nil {
		t.Fatal(err)
	}
	got, _ := repo.GetRun(ctx, f.workspace.ID, run.ID)
	if got.State != domain.RunQueued || got.StartedAt != nil {
		t.Fatalf("run after requeue = %s, started %v", got.State, got.StartedAt)
	}
}

func TestEventsCappedAndFiltered(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	repo := NewResearchRepository(requireDB(t))
	p, _ := repo.CreateProfile(ctx, f.workspace.ID, profileParams(uniq("e")))
	run, _ := repo.EnqueueRun(ctx, f.workspace.ID, p.ID, "manual")
	var events []domain.RunEvent
	for i := range 2100 {
		level := "info"
		if i%100 == 0 {
			level = "error"
		}
		events = append(events, domain.RunEvent{RunID: run.ID, At: time.Now(), Level: level, Stage: domain.StageCollection, Company: "Acme", Message: "x"})
	}
	if err := repo.AddEvents(ctx, events); err != nil {
		t.Fatal(err)
	}
	all, err := repo.ListEvents(ctx, f.workspace.ID, run.ID, 0, "", 5000)
	if err != nil || len(all) != 2000 {
		t.Fatalf("events kept = %d, %v; want 2000", len(all), err)
	}
	after, _ := repo.ListEvents(ctx, f.workspace.ID, run.ID, all[1989].ID, "", 500)
	if len(after) != 10 {
		t.Errorf("events after cursor = %d, want 10", len(after))
	}
	errs, _ := repo.ListEvents(ctx, f.workspace.ID, run.ID, 0, "error", 500)
	if len(errs) != 20 {
		t.Errorf("error events = %d, want 20", len(errs))
	}
	other := newFixture(t)
	if got, err := repo.ListEvents(ctx, other.workspace.ID, run.ID, 0, "", 10); err == nil && len(got) > 0 {
		t.Error("events leaked to another workspace")
	}
}

func TestReplaceMatchesNewFlagAndRemoval(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	pool := requireDB(t)
	repo := NewResearchRepository(pool)
	jobs := NewJobRepository(pool)
	p, _ := repo.CreateProfile(ctx, f.workspace.ID, profileParams(uniq("m")))
	old := f.job
	fresh, _, err := jobs.UpsertListing(ctx, IngestedListing{SourceID: f.source.ID, ExternalID: uniq("e"), URL: "https://x.example/2",
		DedupeKey: uniq("dk"), Title: "Fresh", CompanyName: "Acme", SeenAt: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	pool.Exec(ctx, `UPDATE jobs SET first_seen_at = NOW() - INTERVAL '10 days' WHERE id = $1`, old.ID)
	since := time.Now().Add(-24 * time.Hour)
	reasons := []domain.MatchReason{{Code: "title", Params: map[string]string{"value": "full stack"}}}
	n, err := repo.ReplaceMatches(ctx, p.ID, []domain.Match{
		{JobID: old.ID, Score: 70, Reasons: reasons}, {JobID: fresh.ID, Score: 90, Reasons: reasons},
	}, &since)
	if err != nil || n != 1 {
		t.Fatalf("new matches = %d, %v; want 1", n, err)
	}
	got, _ := repo.MatchesByJob(ctx, p.ID, []string{old.ID, fresh.ID})
	if got[fresh.ID].Score != 90 || !got[fresh.ID].New || got[old.ID].New || got[old.ID].Reasons[0].Code != "title" {
		t.Fatalf("matches = %+v", got)
	}
	if _, err := repo.ReplaceMatches(ctx, p.ID, []domain.Match{{JobID: fresh.ID, Score: 50}}, &since); err != nil {
		t.Fatal(err)
	}
	got, _ = repo.MatchesByJob(ctx, p.ID, []string{old.ID, fresh.ID})
	if _, ok := got[old.ID]; ok || got[fresh.ID].Score != 50 {
		t.Fatalf("after replace: %+v", got)
	}
}

func TestPurgeClosedJobsKeepsSavedAndApplied(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	pool := requireDB(t)
	jobs := NewJobRepository(pool)
	repo := NewResearchRepository(pool)
	mk := func() domain.Job {
		j, _, err := jobs.UpsertListing(ctx, IngestedListing{SourceID: f.source.ID, ExternalID: uniq("e"), URL: "https://x.example/p",
			DedupeKey: uniq("dk"), Title: "Closed", CompanyName: "Acme"})
		if err != nil {
			t.Fatal(err)
		}
		pool.Exec(ctx, `UPDATE jobs SET closed_at = NOW() - INTERVAL '100 days' WHERE id = $1`, j.ID)
		return j
	}
	plain, saved, applied, recent := mk(), mk(), mk(), mk()
	pool.Exec(ctx, `UPDATE jobs SET closed_at = NOW() - INTERVAL '1 day' WHERE id = $1`, recent.ID)
	jobs.SaveJob(ctx, f.workspace.ID, saved.ID, nil)
	NewApplicationRepository(pool).Create(ctx, f.workspace.ID, applied.ID, nil)

	if _, err := repo.PurgeClosedJobs(ctx, 90*24*time.Hour); err != nil {
		t.Fatal(err)
	}
	if _, err := jobs.GetJob(ctx, plain.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("old closed job not purged: %v", err)
	}
	for _, j := range []domain.Job{saved, applied, recent} {
		if _, err := jobs.GetJob(ctx, j.ID); err != nil {
			t.Errorf("job %s purged: %v", j.ID, err)
		}
	}
}

func TestDiscoveryState(t *testing.T) {
	repo := NewResearchRepository(requireDB(t))
	ctx := context.Background()
	key := uniq("state-")
	if v, err := repo.State(ctx, key); err != nil || v != "" {
		t.Fatalf("missing state = %q, %v", v, err)
	}
	repo.SetState(ctx, key, "42")
	repo.SetState(ctx, key, "43")
	if v, _ := repo.State(ctx, key); v != "43" {
		t.Fatalf("state = %q", v)
	}
}

func TestOpenJobsForScoringSkipsClosedAndBlocked(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	pool := requireDB(t)
	jobs := NewJobRepository(pool)
	companies := NewCompanyRepository(pool)
	blocked := newCompany(t, companies, "lever")
	companies.Block(ctx, f.workspace.ID, blocked.ID)
	mk := func(title string, company *string) domain.Job {
		j, _, err := jobs.UpsertListing(ctx, IngestedListing{SourceID: f.source.ID, ExternalID: uniq("e"), URL: "https://x.example/s",
			DedupeKey: uniq("dk"), Title: title, CompanyName: "Acme", CompanyID: company})
		if err != nil {
			t.Fatal(err)
		}
		return j
	}
	open, closed, fromBlocked := mk(uniq("Open "), nil), mk(uniq("Closed "), nil), mk(uniq("Blocked "), &blocked.ID)
	pool.Exec(ctx, `UPDATE jobs SET closed_at = NOW() WHERE id = $1`, closed.ID)
	seen := map[string]bool{}
	if err := jobs.OpenJobsForScoring(ctx, f.workspace.ID, func(j domain.Job) error {
		seen[j.ID] = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !seen[open.ID] || seen[closed.ID] || seen[fromBlocked.ID] {
		t.Fatalf("open=%v closed=%v blocked=%v", seen[open.ID], seen[closed.ID], seen[fromBlocked.ID])
	}
}

func TestUpdateTiers(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	pool := requireDB(t)
	companies := NewCompanyRepository(pool)
	jobs := NewJobRepository(pool)
	research := NewResearchRepository(pool)
	profiles, _ := research.ListProfiles(ctx, f.workspace.ID)

	tier := func(withJob, withMatch bool) string {
		c := newCompany(t, companies, "greenhouse")
		s, err := companies.AttachSource(ctx, c.ID, SourceParams{Name: c.Name, BaseURL: c.CareersURL, Adapter: "greenhouse"})
		if err != nil {
			t.Fatal(err)
		}
		if withJob {
			j, _, err := jobs.UpsertListing(ctx, IngestedListing{SourceID: s.ID, ExternalID: "1", URL: "https://x.example/t",
				DedupeKey: uniq("dk"), Title: "T", CompanyName: c.Name, CompanyID: &c.ID})
			if err != nil {
				t.Fatal(err)
			}
			if withMatch {
				research.ReplaceMatches(ctx, profiles[0].ID, []domain.Match{{JobID: j.ID, Score: 80}}, nil)
			}
		}
		return s.ID
	}
	top, mid, low := tier(true, true), tier(true, false), tier(false, false)
	if err := companies.UpdateTiers(ctx, 12*time.Hour, 72*time.Hour, 168*time.Hour); err != nil {
		t.Fatal(err)
	}
	interval := func(id string) time.Duration {
		var secs int64
		pool.QueryRow(ctx, `SELECT EXTRACT(EPOCH FROM sync_interval)::bigint FROM job_sources WHERE id = $1`, id).Scan(&secs)
		return time.Duration(secs) * time.Second
	}
	if interval(top) != 12*time.Hour || interval(mid) != 72*time.Hour || interval(low) != 168*time.Hour {
		t.Fatalf("tiers = %v %v %v", interval(top), interval(mid), interval(low))
	}
}

func TestStageLockIsExclusive(t *testing.T) {
	repo := NewResearchRepository(requireDB(t))
	ctx := context.Background()
	release, ok, err := repo.TryStageLock(ctx)
	if err != nil || !ok {
		t.Fatalf("first lock: %v %v", ok, err)
	}
	if _, ok2, err := repo.TryStageLock(ctx); err != nil || ok2 {
		t.Fatalf("second lock acquired while held: %v %v", ok2, err)
	}
	release()
	release2, ok, err := repo.TryStageLock(ctx)
	if err != nil || !ok {
		t.Fatalf("lock after release: %v %v", ok, err)
	}
	release2()
}

func TestStarterProfileDoesNotRunByItself(t *testing.T) {
	f := newFixture(t)
	ps, err := NewResearchRepository(requireDB(t)).ListProfiles(context.Background(), f.workspace.ID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, p := range ps {
		if p.Name == "Full Stack Developer" {
			found = true
			if !p.SchedulePaused {
				t.Error("the starter profile must start with auto-run off")
			}
		}
	}
	if !found {
		t.Fatal("a new workspace has no starter profile")
	}
}
