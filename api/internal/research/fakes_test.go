package research

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"example.com/rolescout/api/internal/crawl"
	"example.com/rolescout/api/internal/discovery"
	"example.com/rolescout/api/internal/domain"
	"example.com/rolescout/api/internal/infrastructure/database"
	"example.com/rolescout/api/internal/ingest"
)

type fakeRuns struct {
	mu        sync.Mutex
	profile   domain.ResearchProfile
	saved     []domain.ResearchRun
	finished  map[string]domain.RunStatus
	errs      map[string]string
	summaries map[string]map[string]int
	events    []domain.RunEvent
	matches   []domain.Match
	since     *time.Time
	state     map[string]string
	cancel    bool
	lockHeld  bool
	lockWaits int
	queued    []domain.ResearchRun
	enqueued  []string
	stale     int
	purged    int
}

func newFakeRuns() *fakeRuns {
	return &fakeRuns{profile: defaultProfile(), finished: map[string]domain.RunStatus{}, errs: map[string]string{},
		summaries: map[string]map[string]int{}, state: map[string]string{}}
}

func (f *fakeRuns) GetProfile(context.Context, string, string) (domain.ResearchProfile, error) {
	return f.profile, nil
}
func (f *fakeRuns) SaveProgress(_ context.Context, run domain.ResearchRun) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.saved = append(f.saved, run)
	return nil
}
func (f *fakeRuns) FinishRun(_ context.Context, id string, state domain.RunStatus, msg *string, summary map[string]int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.finished[id] = state
	if msg != nil {
		f.errs[id] = *msg
	}
	f.summaries[id] = summary
	return nil
}
func (f *fakeRuns) CancelRequested(context.Context, string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.cancel, nil
}
func (f *fakeRuns) AddEvents(_ context.Context, es []domain.RunEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, es...)
	return nil
}
func (f *fakeRuns) ReplaceMatches(_ context.Context, _ string, ms []domain.Match, since *time.Time) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.matches, f.since = ms, since
	return len(ms), nil
}
func (f *fakeRuns) State(_ context.Context, key string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.state[key], nil
}
func (f *fakeRuns) SetState(_ context.Context, key, value string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.state[key] = value
	return nil
}
func (f *fakeRuns) TryStageLock(context.Context) (func(), bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.lockHeld {
		f.lockWaits++
		return nil, false, nil
	}
	f.lockHeld = true
	return func() {
		f.mu.Lock()
		f.lockHeld = false
		f.mu.Unlock()
	}, true, nil
}
func (f *fakeRuns) ClaimRun(context.Context) (domain.ResearchRun, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.queued) == 0 {
		return domain.ResearchRun{}, false, nil
	}
	r := f.queued[0]
	f.queued = f.queued[1:]
	return r, true, nil
}
func (f *fakeRuns) FailStaleRuns(context.Context, time.Duration) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stale++
	return 0, nil
}
func (f *fakeRuns) DueProfiles(context.Context, time.Time) ([]domain.ResearchProfile, error) {
	return []domain.ResearchProfile{f.profile}, nil
}
func (f *fakeRuns) EnqueueRun(_ context.Context, ws, profileID, trigger string) (domain.ResearchRun, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.enqueued = append(f.enqueued, trigger)
	r := domain.ResearchRun{ID: fmt.Sprintf("run-%d", len(f.enqueued)), WorkspaceID: ws, ProfileID: profileID, State: domain.RunRunning}
	f.queued = append(f.queued, r)
	return r, nil
}
func (f *fakeRuns) PurgeClosedJobs(context.Context, time.Duration) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.purged++
	return 0, nil
}
func (f *fakeRuns) PurgeRunEvents(context.Context, time.Duration) (int64, error) { return 0, nil }

type fakeCompanies struct {
	mu        sync.Mutex
	byURL     map[string]*domain.Company
	attached  map[string]database.SourceParams
	disabled  map[string]bool
	tiers     int
	detected  map[string]domain.CompanyStatus
	conflicts map[string]bool
}

func newFakeCompanies() *fakeCompanies {
	return &fakeCompanies{byURL: map[string]*domain.Company{}, attached: map[string]database.SourceParams{},
		disabled: map[string]bool{}, detected: map[string]domain.CompanyStatus{}, conflicts: map[string]bool{}}
}

func (f *fakeCompanies) UpsertDiscovered(_ context.Context, p database.CompanyParams) (domain.Company, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if c, ok := f.byURL[p.CareersURL]; ok {
		return *c, false, nil
	}
	c := &domain.Company{ID: fmt.Sprintf("co-%d", len(f.byURL)+1), Name: p.Name, CareersURL: p.CareersURL, ATS: p.ATS,
		ATSID: p.ATSID, Origin: p.Origin, Status: domain.CompanyPending}
	f.byURL[p.CareersURL] = c
	return *c, true, nil
}
func (f *fakeCompanies) PendingDetection(_ context.Context, limit int) ([]domain.Company, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []domain.Company
	for _, c := range f.byURL {
		if c.Status == domain.CompanyPending && len(out) < limit {
			out = append(out, *c)
		}
	}
	return out, nil
}
func (f *fakeCompanies) find(id string) *domain.Company {
	for _, c := range f.byURL {
		if c.ID == id {
			return c
		}
	}
	return nil
}
func (f *fakeCompanies) RecordDetection(_ context.Context, id, ats string, atsID *string, status domain.CompanyStatus, _ *string) (domain.Company, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c := f.find(id)
	if atsID != nil && f.conflicts[*atsID] {
		return *c, fmt.Errorf("record: %w", domain.ErrConflict)
	}
	c.ATS, c.ATSID, c.Status = ats, atsID, status
	f.detected[id] = status
	return *c, nil
}
func (f *fakeCompanies) AttachSource(_ context.Context, companyID string, p database.SourceParams) (domain.JobSource, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.attached[companyID] = p
	return domain.JobSource{ID: "src-" + companyID, Key: p.Key, CompanyID: &companyID}, nil
}
func (f *fakeCompanies) SetDisabled(_ context.Context, id string, disabled bool) (domain.Company, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.disabled[id] = disabled
	c := f.find(id)
	c.Status = domain.CompanyDisabled
	return *c, nil
}
func (f *fakeCompanies) CountsByATS(context.Context) (map[string]int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	m := map[string]int{}
	for _, c := range f.byURL {
		m[c.ATS]++
	}
	return m, nil
}
func (f *fakeCompanies) UpdateTiers(context.Context, time.Duration, time.Duration, time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tiers++
	return nil
}

type fakeJobs struct{ jobs []domain.Job }

func (f *fakeJobs) OpenJobsForScoring(_ context.Context, _ string, fn func(domain.Job) error) error {
	for _, j := range f.jobs {
		if err := fn(j); err != nil {
			return err
		}
	}
	return nil
}

// fakeIngest reports one result per attached source.
type fakeIngest struct {
	companies *fakeCompanies
	block     chan struct{} // when set, RunDue waits on it or on ctx
	fail      map[string]bool
}

func (f *fakeIngest) RunDue(ctx context.Context, o ingest.RunDueOptions) error {
	f.companies.mu.Lock()
	var ids []string
	for id := range f.companies.attached {
		ids = append(ids, id)
	}
	f.companies.mu.Unlock()
	if o.OnStart != nil {
		o.OnStart(len(ids))
	}
	if f.block != nil {
		select {
		case <-f.block:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	for _, id := range ids {
		id := id
		src := domain.JobSource{ID: "src-" + id, Name: id, Adapter: "greenhouse", CompanyID: &id}
		run := domain.IngestionRun{Status: domain.RunSucceeded, FetchedCount: 3, UpsertedCount: 3}
		var err error
		if f.fail[id] {
			msg := "board not found"
			run = domain.IngestionRun{Status: domain.RunFailed, Error: &msg}
			err = errors.New(msg)
		}
		o.OnResult(src, run, err)
	}
	return nil
}

type fakeDetector map[string]discovery.Detection

func (f fakeDetector) Detect(_ context.Context, url string) (discovery.Detection, error) {
	if d, ok := f[url]; ok {
		return d, nil
	}
	if url == "https://blocked.example/careers" {
		return discovery.Detection{}, fmt.Errorf("read: %w", crawl.ErrDisallowed)
	}
	if url == "https://panic.example/careers" {
		panic("detector bug")
	}
	return discovery.Detection{Board: discovery.Board{ATS: "unknown"}}, nil
}

type fakeSource struct {
	name  string
	cands []discovery.Candidate
	err   error
}

func (f fakeSource) Name() string { return f.name }
func (f fakeSource) Discover(_ context.Context, state string, fn func(discovery.Candidate) error) (string, error) {
	for _, c := range f.cands {
		if err := fn(c); err != nil {
			return state, err
		}
	}
	return "next-" + f.name, f.err
}
