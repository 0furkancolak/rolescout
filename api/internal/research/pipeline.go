package research

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/url"
	"strings"
	"sync"
	"time"

	"example.com/rolescout/api/internal/crawl"
	"example.com/rolescout/api/internal/discovery"
	"example.com/rolescout/api/internal/domain"
	"example.com/rolescout/api/internal/focus"
	"example.com/rolescout/api/internal/infrastructure/database"
	"example.com/rolescout/api/internal/ingest"
)

// RunStore is the research repository as the pipeline uses it.
type RunStore interface {
	GetProfile(ctx context.Context, workspaceID, id string) (domain.ResearchProfile, error)
	SaveProgress(ctx context.Context, run domain.ResearchRun) error
	FinishRun(ctx context.Context, id string, state domain.RunStatus, errMsg *string, summary map[string]int) error
	CancelRequested(ctx context.Context, id string) (bool, error)
	AddEvents(ctx context.Context, events []domain.RunEvent) error
	ReplaceMatches(ctx context.Context, profileID string, ms []domain.Match, since *time.Time) (int, error)
	State(ctx context.Context, key string) (string, error)
	SetState(ctx context.Context, key, value string) error
	TryStageLock(ctx context.Context) (func(), bool, error)
	ClaimRun(ctx context.Context) (domain.ResearchRun, bool, error)
	FailStaleRuns(ctx context.Context, maxAge time.Duration) (int64, error)
	DueProfiles(ctx context.Context, now time.Time) ([]domain.ResearchProfile, error)
	EnqueueRun(ctx context.Context, workspaceID, profileID, trigger string) (domain.ResearchRun, error)
	PurgeClosedJobs(ctx context.Context, olderThan time.Duration) (int64, error)
	PurgeRunEvents(ctx context.Context, olderThan time.Duration) (int64, error)
}

// CompanyStore is the company catalog as the pipeline uses it.
type CompanyStore interface {
	UpsertDiscovered(ctx context.Context, p database.CompanyParams) (domain.Company, bool, error)
	PendingDetection(ctx context.Context, limit int) ([]domain.Company, error)
	RecordDetection(ctx context.Context, id, ats string, atsID *string, status domain.CompanyStatus, errMsg *string) (domain.Company, error)
	AttachSource(ctx context.Context, companyID string, p database.SourceParams) (domain.JobSource, error)
	SetDisabled(ctx context.Context, id string, disabled bool) (domain.Company, error)
	CountsByATS(ctx context.Context) (map[string]int, error)
	UpdateTiers(ctx context.Context, top, mid, low time.Duration) error
	RefreshCountries(ctx context.Context) (int64, error)
}

type JobStore interface {
	OpenJobsForScoring(ctx context.Context, workspaceID string, fn func(domain.Job) error) error
}

type Ingester interface {
	RunDue(ctx context.Context, o ingest.RunDueOptions) error
}

type Detector interface {
	Detect(ctx context.Context, careersURL string) (discovery.Detection, error)
}

type Config struct {
	DiscoveryInterval   time.Duration
	CompanySyncInterval time.Duration
	Concurrency         int
	MatchMinScore       int
	ClosedJobRetention  time.Duration
}

// Pipeline executes research runs: discovery, detection and collection are
// shared by all runs (one at a time across workers); scoring is per profile.
type Pipeline struct {
	Runs      RunStore
	Companies CompanyStore
	Jobs      JobStore
	Ingest    Ingester
	Detector  Detector
	Sources   []discovery.Source
	Config    Config
	Logger    *slog.Logger
	Now       func() time.Time
	// FlushInterval is how often progress is saved (default 1s); LockWait
	// how long to wait before retrying the shared-stage lock (default 5s).
	FlushInterval time.Duration
	LockWait      time.Duration

	lastPurge time.Time
}

const (
	detectionBatch = 200
	staleRunAfter  = 15 * time.Minute
	eventRetention = 30 * 24 * time.Hour
	lastFullKey    = "discovery.last_full"
)

func (p *Pipeline) defaults() {
	if p.Now == nil {
		p.Now = time.Now
	}
	if p.Logger == nil {
		p.Logger = slog.Default()
	}
	if p.FlushInterval <= 0 {
		p.FlushInterval = time.Second
	}
	if p.LockWait <= 0 {
		p.LockWait = 5 * time.Second
	}
	if p.Config.Concurrency <= 0 {
		p.Config.Concurrency = 1
	}
	if p.Config.CompanySyncInterval <= 0 {
		p.Config.CompanySyncInterval = 12 * time.Hour
	}
}

// Loop runs Tick every interval until ctx is cancelled.
func (p *Pipeline) Loop(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		p.Tick(ctx)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// Tick recovers abandoned runs, queues scheduled runs, executes queued runs
// one after another and purges old data once a day.
func (p *Pipeline) Tick(ctx context.Context) {
	p.defaults()
	if n, err := p.Runs.FailStaleRuns(ctx, staleRunAfter); err != nil {
		p.Logger.Error("fail stale research runs", "error", err)
	} else if n > 0 {
		p.Logger.Warn("marked abandoned research runs as failed", "count", n)
	}
	due, err := p.Runs.DueProfiles(ctx, p.Now())
	if err != nil {
		p.Logger.Error("list due research profiles", "error", err)
	}
	for _, prof := range due {
		if _, err := p.Runs.EnqueueRun(ctx, prof.WorkspaceID, prof.ID, "schedule"); err != nil && !errors.Is(err, domain.ErrConflict) {
			p.Logger.Error("queue scheduled research run", "profile", prof.ID, "error", err)
		}
	}
	for ctx.Err() == nil {
		run, ok, err := p.Runs.ClaimRun(ctx)
		if err != nil {
			p.Logger.Error("claim research run", "error", err)
			break
		}
		if !ok {
			break
		}
		if err := p.Execute(ctx, run); err != nil {
			p.Logger.Error("research run failed", "run", run.ID, "error", err)
		}
	}
	if p.Now().Sub(p.lastPurge) >= 24*time.Hour && ctx.Err() == nil {
		p.lastPurge = p.Now()
		if p.Config.ClosedJobRetention > 0 {
			if n, err := p.Runs.PurgeClosedJobs(ctx, p.Config.ClosedJobRetention); err != nil {
				p.Logger.Error("purge closed jobs", "error", err)
			} else if n > 0 {
				p.Logger.Info("purged closed jobs", "count", n)
			}
		}
		if _, err := p.Runs.PurgeRunEvents(ctx, eventRetention); err != nil {
			p.Logger.Error("purge research run events", "error", err)
		}
	}
}

// Execute runs every stage of a claimed run and records its outcome.
func (p *Pipeline) Execute(ctx context.Context, run domain.ResearchRun) error {
	p.defaults()
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	rep := newReporter(p.Runs, run, cancel, p.Now)
	stop := rep.start(context.WithoutCancel(ctx), p.FlushInterval)
	log := p.Logger.With("run", run.ID, "profile", run.ProfileID)
	log.Info("research run started")

	summary, err := p.execute(runCtx, run, rep)
	stop()

	state, msg := domain.RunSucceeded, (*string)(nil)
	switch {
	case rep.isCancelled():
		state = domain.RunCancelled
	case err != nil && ctx.Err() != nil:
		state, msg = domain.RunFailed, strp("the worker stopped during the run")
	case err != nil:
		state, msg = domain.RunFailed, strp(err.Error())
	}
	if ferr := p.Runs.FinishRun(context.WithoutCancel(ctx), run.ID, state, msg, summary); ferr != nil {
		return errors.Join(err, ferr)
	}
	log.Info("research run finished", "state", state, "summary", summary)
	if state == domain.RunFailed {
		return err
	}
	return nil
}

func strp(s string) *string { return &s }

func (p *Pipeline) execute(ctx context.Context, run domain.ResearchRun, rep *reporter) (summary map[string]int, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("research run panicked: %v", r)
		}
	}()
	profile, err := p.Runs.GetProfile(ctx, run.WorkspaceID, run.ProfileID)
	if err != nil {
		return nil, fmt.Errorf("load profile: %w", err)
	}
	// Companies of the profile's countries are looked at first in every stage.
	ctx = focus.With(ctx, profile.Countries)
	if len(profile.Countries) > 0 {
		rep.event("info", domain.StageDiscovery, "", "focus on "+strings.Join(profile.Countries, ", ")+": their companies are read first")
	}

	release, err := p.lockStages(ctx, rep)
	if err != nil {
		return nil, err
	}
	collected, err := func() (int, error) {
		defer release()
		if err := p.discover(ctx, rep); err != nil {
			return 0, err
		}
		if err := p.detect(ctx, rep); err != nil {
			return 0, err
		}
		return p.collect(ctx, rep)
	}()
	if err != nil {
		return nil, err
	}
	// What the jobs say about where each company hires lets the next run read
	// the profile's countries first.
	if n, err := p.Companies.RefreshCountries(ctx); err != nil {
		p.Logger.Warn("refresh company countries", "error", err)
	} else if n > 0 {
		rep.event("info", domain.StageCollection, "", fmt.Sprintf("country known for %d more companies", n))
	}

	open, matches, fresh, err := p.score(ctx, run, profile, rep)
	if err != nil {
		return nil, err
	}

	rep.begin(domain.StageResult)
	counts, err := p.Companies.CountsByATS(ctx)
	if err != nil {
		return nil, err
	}
	known := 0
	for _, n := range counts {
		known += n
	}
	det, col := rep.stage(domain.StageDetection), rep.stage(domain.StageCollection)
	summary = map[string]int{
		"companies_known":   known,
		"companies_checked": det.Done,
		"companies_failed":  det.Failed + col.Failed,
		"jobs_collected":    collected,
		"jobs_open":         open,
		"matches":           matches,
		"new_matches":       fresh,
	}
	rep.update(domain.StageResult, func(sp *domain.StageProgress) {
		sp.Total, sp.Done = 1, 1
		sp.Detail = maps.Clone(summary)
	})
	rep.end(domain.StageResult, "done", "")
	return summary, nil
}

func (p *Pipeline) lockStages(ctx context.Context, rep *reporter) (func(), error) {
	waiting := false
	for {
		release, ok, err := p.Runs.TryStageLock(ctx)
		if err != nil {
			return nil, err
		}
		if ok {
			return release, nil
		}
		if !waiting {
			rep.event("info", domain.StageDiscovery, "", "waiting for another research run to finish its shared stages")
			waiting = true
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(p.LockWait):
		}
	}
}

// discover grows the company catalog unless it was refreshed recently.
func (p *Pipeline) discover(ctx context.Context, rep *reporter) error {
	stage := domain.StageDiscovery
	rep.begin(stage)
	last, err := p.Runs.State(ctx, lastFullKey)
	if err != nil {
		return err
	}
	pending, err := p.pendingSources(ctx)
	if err != nil {
		return err
	}
	if t, err := time.Parse(time.RFC3339, last); err == nil && p.Now().Sub(t) < p.Config.DiscoveryInterval && !pending {
		known, err := p.knownCompanies(ctx)
		if err != nil {
			return err
		}
		rep.update(stage, func(sp *domain.StageProgress) { sp.Total, sp.Done = known, known })
		rep.end(stage, "skipped", fmt.Sprintf("catalog is fresh, %d companies", known))
		return nil
	}
	// The sources are independent (each keeps its own state), and most of the time
	// is spent waiting for other servers, so they all run at the same time: a
	// slow one no longer holds up the rest. progress counts finished sources
	// until the whole catalog size replaces it below.
	rep.update(stage, func(sp *domain.StageProgress) { sp.Total, sp.Done = len(p.Sources), 0 })
	var wg sync.WaitGroup
	for _, src := range p.Sources {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p.discoverFrom(ctx, rep, src)
			rep.update(stage, func(sp *domain.StageProgress) { sp.Done++ })
		}()
	}
	wg.Wait()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err := p.Runs.SetState(ctx, lastFullKey, p.Now().UTC().Format(time.RFC3339)); err != nil {
		return err
	}
	known, err := p.knownCompanies(ctx)
	if err != nil {
		return err
	}
	rep.update(stage, func(sp *domain.StageProgress) { sp.Total, sp.Done = known, known })
	rep.end(stage, "done", "")
	return nil
}

// discoverFrom reads one discovery source and stores what it finds. A failing
// source is reported and does not stop the others.
func (p *Pipeline) discoverFrom(ctx context.Context, rep *reporter, src discovery.Source) {
	stage := domain.StageDiscovery
	key := "discovery." + src.Name()
	state, err := p.Runs.State(ctx, key)
	if err != nil {
		rep.update(stage, func(sp *domain.StageProgress) { sp.Failed++ })
		rep.event("error", stage, "", fmt.Sprintf("source %s: %v", src.Name(), err))
		return
	}
	var mu sync.Mutex
	added := 0
	rep.event("info", stage, "", fmt.Sprintf("%s: started", src.Name()))
	next, err := src.Discover(ctx, state, func(c discovery.Candidate) error {
		created, err := p.addCandidate(ctx, c)
		if err != nil {
			rep.event("warn", stage, c.Name, fmt.Sprintf("skipped %s: %v", c.URL, err))
			return ctx.Err()
		}
		if created {
			mu.Lock()
			added++
			n := added
			mu.Unlock()
			rep.update(stage, func(sp *domain.StageProgress) { sp.Detail[src.Name()]++ })
			if n%100 == 0 {
				rep.event("info", stage, "", fmt.Sprintf("%s: %d new companies so far", src.Name(), n))
			}
		}
		return nil
	})
	if ctx.Err() != nil {
		return
	}
	if err != nil {
		rep.update(stage, func(sp *domain.StageProgress) { sp.Failed++ })
		rep.event("error", stage, "", fmt.Sprintf("source %s failed: %v", src.Name(), err))
	} else if err := p.Runs.SetState(ctx, key, next); err != nil {
		rep.event("error", stage, "", fmt.Sprintf("source %s: save state: %v", src.Name(), err))
	}
	rep.event("info", stage, "", fmt.Sprintf("%s: %d new companies", src.Name(), added))
}

func (p *Pipeline) knownCompanies(ctx context.Context) (int, error) {
	counts, err := p.Companies.CountsByATS(ctx)
	n := 0
	for _, c := range counts {
		n += c
	}
	return n, err
}

// addCandidate stores a discovered URL as a company: with its board when the
// URL is a board, else as a career page to detect.
func (p *Pipeline) addCandidate(ctx context.Context, c discovery.Candidate) (bool, error) {
	params := database.CompanyParams{Name: c.Name, CareersURL: c.URL, ATS: "unknown", Origin: c.Origin, Country: c.Country}
	if params.Country == "" {
		params.Country = discovery.CountryOfURL(c.URL)
	}
	if b, ok := discovery.ParseBoardURL(c.URL); ok {
		params.ATS, params.ATSID = b.ATS, &b.ID
		if params.Name == "" {
			params.Name = b.ID
		}
	}
	if params.Name == "" {
		if u, err := url.Parse(c.URL); err == nil {
			params.Name = strings.TrimPrefix(u.Hostname(), "www.")
		}
	}
	_, created, err := p.Companies.UpsertDiscovered(ctx, params)
	return created, err
}

// detect identifies the boards of pending companies and attaches sources.
func (p *Pipeline) detect(ctx context.Context, rep *reporter) error {
	stage := domain.StageDetection
	rep.begin(stage)
	seen := map[string]bool{}
	for {
		batch, err := p.Companies.PendingDetection(ctx, detectionBatch)
		if err != nil {
			return err
		}
		var todo []domain.Company
		for _, c := range batch {
			if !seen[c.ID] {
				seen[c.ID] = true
				todo = append(todo, c)
			}
		}
		if len(todo) == 0 {
			break
		}
		rep.update(stage, func(sp *domain.StageProgress) { sp.Total += len(todo) })
		work := make(chan domain.Company)
		var wg sync.WaitGroup
		for range min(p.Config.Concurrency, len(todo)) {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for c := range work {
					p.detectOne(ctx, rep, c)
				}
			}()
		}
	feed:
		for _, c := range todo {
			select {
			case work <- c:
			case <-ctx.Done():
				break feed
			}
		}
		close(work)
		wg.Wait()
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
	rep.end(stage, "done", "")
	return nil
}

func (p *Pipeline) detectOne(ctx context.Context, rep *reporter, c domain.Company) {
	stage := domain.StageDetection
	fail := func(status domain.CompanyStatus, msg string) {
		rep.update(stage, func(sp *domain.StageProgress) { sp.Done++; sp.Failed++ })
		rep.event("warn", stage, c.Name, msg)
		if _, err := p.Companies.RecordDetection(ctx, c.ID, "unknown", nil, status, &msg); err != nil && ctx.Err() == nil {
			p.Logger.Error("record detection", "company", c.ID, "error", err)
		}
	}
	defer func() {
		if r := recover(); r != nil {
			fail(domain.CompanyUnreachable, fmt.Sprintf("detection failed unexpectedly: %v", r))
		}
	}()

	board, ok := discovery.ParseBoardURL(c.CareersURL)
	var links []discovery.Board
	if !ok {
		det, err := p.Detector.Detect(ctx, c.CareersURL)
		switch {
		case ctx.Err() != nil:
			return
		case errors.Is(err, crawl.ErrDisallowed):
			fail(domain.CompanyBlockedByRobots, "career page is disallowed by robots.txt")
			return
		case err != nil:
			fail(domain.CompanyUnreachable, err.Error())
			return
		case det.Board.ATS == "unknown":
			fail(domain.CompanyUnreachable, "no supported job board found on "+c.CareersURL)
			return
		}
		board, links = det.Board, det.Links
	}

	if _, err := p.Companies.RecordDetection(ctx, c.ID, board.ATS, &board.ID, domain.CompanyActive, nil); err != nil {
		if errors.Is(err, domain.ErrConflict) {
			// Another company already has this board; keep that one.
			_, _ = p.Companies.SetDisabled(ctx, c.ID, true)
			rep.update(stage, func(sp *domain.StageProgress) { sp.Done++ })
			rep.event("info", stage, c.Name, fmt.Sprintf("same %s board as another company; kept the other one", board.ATS))
			return
		}
		fail(domain.CompanyUnreachable, err.Error())
		return
	}
	config := maps.Clone(board.Config)
	if c.Name != "" && c.Name != board.ID {
		config["company"] = c.Name
	}
	if _, err := p.Companies.AttachSource(ctx, c.ID, database.SourceParams{
		Key: board.SourceKey(), Name: c.Name, BaseURL: c.CareersURL, Adapter: board.ATS, Config: config,
		SyncInterval: p.Config.CompanySyncInterval,
	}); err != nil {
		fail(domain.CompanyUnreachable, "attach source: "+err.Error())
		return
	}
	for _, l := range links {
		if _, err := p.addCandidate(ctx, discovery.Candidate{URL: boardURL(l), Origin: "link"}); err != nil {
			rep.event("warn", stage, c.Name, "linked board skipped: "+err.Error())
		}
	}
	rep.update(stage, func(sp *domain.StageProgress) { sp.Done++; sp.Detail[board.ATS]++ })
	rep.event("info", stage, c.Name, board.ATS+" board found")
}

// boardURL is a canonical URL for a linked board, used as its careers URL.
func boardURL(b discovery.Board) string {
	switch b.ATS {
	case "greenhouse":
		return "https://boards.greenhouse.io/" + b.ID
	case "lever":
		if b.Config["region"] == "eu" {
			return "https://jobs.eu.lever.co/" + b.ID
		}
		return "https://jobs.lever.co/" + b.ID
	case "ashby":
		return "https://jobs.ashbyhq.com/" + b.ID
	case "workable":
		return "https://apply.workable.com/" + b.ID + "/"
	case "recruitee":
		return "https://" + b.ID + ".recruitee.com/"
	case "personio":
		return fmt.Sprintf("https://%s.jobs.personio.%s/", b.ID, b.Config["tld"])
	case "bamboohr":
		return "https://" + b.ID + ".bamboohr.com/careers"
	case "workday":
		return fmt.Sprintf("https://%s/%s", b.Config["host"], b.Config["site"])
	}
	return ""
}

// collect syncs every due company source.
func (p *Pipeline) collect(ctx context.Context, rep *reporter) (int, error) {
	stage := domain.StageCollection
	rep.begin(stage)
	var mu sync.Mutex
	collected := 0
	err := p.Ingest.RunDue(ctx, ingest.RunDueOptions{
		Concurrency: p.Config.Concurrency,
		OnStart: func(total int) {
			rep.update(stage, func(sp *domain.StageProgress) { sp.Total = total })
		},
		OnResult: func(src domain.JobSource, run domain.IngestionRun, err error) {
			if err != nil || run.Status == domain.RunFailed {
				msg := "sync failed"
				if run.Error != nil {
					msg = *run.Error
				} else if err != nil {
					msg = err.Error()
				}
				rep.update(stage, func(sp *domain.StageProgress) { sp.Done++; sp.Failed++ })
				rep.event("error", stage, src.Name, src.Adapter+" · "+msg)
				return
			}
			mu.Lock()
			collected += run.UpsertedCount
			mu.Unlock()
			rep.update(stage, func(sp *domain.StageProgress) {
				sp.Done++
				sp.Detail["jobs"] += run.UpsertedCount
			})
			rep.event("info", stage, src.Name, fmt.Sprintf("%s · %d jobs", src.Adapter, run.UpsertedCount))
		},
	})
	if ctx.Err() != nil {
		return 0, ctx.Err()
	}
	if err != nil {
		return 0, err
	}
	rep.end(stage, "done", "")
	return collected, nil
}

// score rates every open job for the profile and stores the matches.
func (p *Pipeline) score(ctx context.Context, run domain.ResearchRun, profile domain.ResearchProfile, rep *reporter) (open, matches, fresh int, err error) {
	stage := domain.StageScoring
	rep.begin(stage)
	now := p.Now()
	var ms []domain.Match
	err = p.Jobs.OpenJobsForScoring(ctx, run.WorkspaceID, func(j domain.Job) error {
		open++
		if score, reasons, ok := Score(profile, j, now); ok && score >= p.Config.MatchMinScore {
			ms = append(ms, domain.Match{ProfileID: profile.ID, JobID: j.ID, Score: score, Reasons: reasons})
		}
		if open%1000 == 0 {
			n := open
			rep.update(stage, func(sp *domain.StageProgress) { sp.Done, sp.Total = n, n })
		}
		return ctx.Err()
	})
	if err != nil {
		return 0, 0, 0, err
	}
	fresh, err = p.Runs.ReplaceMatches(ctx, profile.ID, ms, profile.LastRunAt)
	if err != nil {
		return 0, 0, 0, err
	}
	top := p.Config.CompanySyncInterval
	if err := p.Companies.UpdateTiers(ctx, top, 72*time.Hour, 168*time.Hour); err != nil {
		return 0, 0, 0, err
	}
	rep.update(stage, func(sp *domain.StageProgress) {
		sp.Total, sp.Done = open, open
		sp.Detail["matches"], sp.Detail["new"] = len(ms), fresh
	})
	rep.event("info", stage, "", fmt.Sprintf("%d of %d open jobs match, %d new", len(ms), open, fresh))
	rep.end(stage, "done", "")
	return open, len(ms), fresh, nil
}

// pendingSources reports whether a source has new work now, such as seeds
// added by a release, although the catalog was refreshed recently.
func (p *Pipeline) pendingSources(ctx context.Context) (bool, error) {
	for _, src := range p.Sources {
		pd, ok := src.(discovery.Pender)
		if !ok {
			continue
		}
		state, err := p.Runs.State(ctx, "discovery."+src.Name())
		if err != nil {
			return false, err
		}
		if pd.Pending(state) {
			return true, nil
		}
	}
	return false, nil
}
