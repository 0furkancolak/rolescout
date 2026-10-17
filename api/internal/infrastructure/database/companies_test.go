package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"example.com/rolescout/api/internal/domain"
)

func strp(s string) *string { return &s }

func TestUpsertDiscoveredDedupesByBoard(t *testing.T) {
	pool := requireDB(t)
	ctx := context.Background()
	repo := NewCompanyRepository(pool)
	board := uniq("acme")

	c, created, err := repo.UpsertDiscovered(ctx, CompanyParams{
		Name: "Acme", CareersURL: "https://boards.greenhouse.io/" + board, ATS: "greenhouse",
		ATSID: strp(board), Origin: "manual",
	})
	if err != nil || !created || c.Status != domain.CompanyPending || c.Origin != "manual" {
		t.Fatalf("first upsert: %+v created=%v err=%v", c, created, err)
	}
	again, created, err := repo.UpsertDiscovered(ctx, CompanyParams{
		Name: "Acme Inc", CareersURL: "https://job-boards.greenhouse.io/" + board, ATS: "greenhouse",
		ATSID: strp(board), Origin: "commoncrawl",
	})
	if err != nil || created || again.ID != c.ID {
		t.Fatalf("second upsert: %+v created=%v err=%v", again, created, err)
	}
	if again.Origin != "manual" {
		t.Errorf("origin = %q, manual must not be downgraded", again.Origin)
	}

	// Without a board, the careers URL identifies the company.
	url := "https://" + uniq("site") + ".example.com/careers"
	a, created1, err := repo.UpsertDiscovered(ctx, CompanyParams{Name: "Site", CareersURL: url, ATS: "unknown", Origin: "seed"})
	if err != nil {
		t.Fatal(err)
	}
	b, created2, err := repo.UpsertDiscovered(ctx, CompanyParams{Name: "Site", CareersURL: url, ATS: "unknown", Origin: "link"})
	if err != nil || !created1 || created2 || a.ID != b.ID {
		t.Fatalf("url dedupe: %v %v %v %v", a.ID, b.ID, created1, created2)
	}
	if _, _, err := repo.UpsertDiscovered(ctx, CompanyParams{Name: "Bad", CareersURL: "ftp://x", ATS: "unknown", Origin: "seed"}); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("non-http URL: err = %v", err)
	}
}

func newCompany(t *testing.T, repo *CompanyRepository, ats string) domain.Company {
	t.Helper()
	id := uniq("co")
	c, _, err := repo.UpsertDiscovered(context.Background(), CompanyParams{
		Name: "Co " + id, CareersURL: "https://" + id + ".example.com/jobs", ATS: ats, ATSID: strp(id), Origin: "seed",
	})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestAttachSourceIsIdempotent(t *testing.T) {
	pool := requireDB(t)
	ctx := context.Background()
	repo := NewCompanyRepository(pool)
	c := newCompany(t, repo, "greenhouse")

	s1, err := repo.AttachSource(ctx, c.ID, SourceParams{
		Name: c.Name, BaseURL: c.CareersURL, Adapter: "greenhouse", Config: map[string]any{"board_token": "a"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !s1.Enabled || s1.CompanyID == nil || *s1.CompanyID != c.ID {
		t.Fatalf("attached source: %+v", s1)
	}
	s2, err := repo.AttachSource(ctx, c.ID, SourceParams{
		Name: c.Name, BaseURL: c.CareersURL, Adapter: "greenhouse", Config: map[string]any{"board_token": "b"},
	})
	if err != nil || s2.ID != s1.ID || s2.Config["board_token"] != "b" {
		t.Fatalf("second attach: %+v %v", s2, err)
	}
	got, err := repo.Get(ctx, c.ID)
	if err != nil || got.SourceID == nil || *got.SourceID != s1.ID {
		t.Fatalf("company source link: %+v %v", got, err)
	}
}

func TestSourceEnableNeedsAdapterOrSourceReview(t *testing.T) {
	pool := requireDB(t)
	ctx := context.Background()
	companies := NewCompanyRepository(pool)
	sources := NewSourceRepository(pool)
	adapter := uniq("adapter_")

	s, err := sources.Create(ctx, SourceParams{Key: uniq("src-"), Name: "X", BaseURL: "https://x.example.com", Adapter: adapter})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sources.SetEnabled(ctx, s.Key, true); !errors.Is(err, ErrSourceNotReviewed) {
		t.Fatalf("unreviewed adapter: err = %v", err)
	}
	if _, err := companies.ReviewAdapter(ctx, adapter, "checked terms", true); err != nil {
		t.Fatal(err)
	}
	if s, err = sources.SetEnabled(ctx, s.Key, true); err != nil || !s.Enabled {
		t.Fatalf("reviewed adapter: %+v %v", s, err)
	}
	// Disabling the adapter type blocks enabling its sources again.
	if _, err := companies.ReviewAdapter(ctx, adapter, "checked terms", false); err != nil {
		t.Fatal(err)
	}
	sources.SetEnabled(ctx, s.Key, false)
	if _, err := sources.SetEnabled(ctx, s.Key, true); !errors.Is(err, ErrSourceNotReviewed) {
		t.Fatalf("disabled adapter: err = %v", err)
	}

	// Adapters reviewed by the migration enable without a source review.
	g, err := sources.Create(ctx, SourceParams{Key: uniq("gh-"), Name: "G", BaseURL: "https://g.example.com", Adapter: "greenhouse"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sources.SetEnabled(ctx, g.Key, true); err != nil {
		t.Fatalf("greenhouse: %v", err)
	}

	reviews, err := companies.AdapterReviews(ctx)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]domain.AdapterReview{}
	for _, r := range reviews {
		byName[r.Adapter] = r
	}
	for _, name := range []string{"greenhouse", "lever", "ashby", "workable", "recruitee", "personio", "bamboohr", "jsonld"} {
		if r := byName[name]; !r.Enabled || r.ReviewedAt == nil || r.Note == nil {
			t.Errorf("%s review = %+v, want enabled with note", name, r)
		}
	}
	if r, ok := byName["workday"]; !ok || r.Enabled || r.ReviewedAt != nil {
		t.Errorf("workday review = %+v, want present and not reviewed", r)
	}
	if _, err := companies.ReviewAdapter(ctx, adapter, "  ", true); !errors.Is(err, domain.ErrInvalid) {
		t.Errorf("enable with blank note: err = %v", err)
	}
	reviews, _ = companies.AdapterReviews(ctx)
	for _, r := range reviews {
		if r.Adapter == adapter && r.Enabled {
			t.Error("a rejected review must not enable the adapter")
		}
	}
}

func TestRecordSyncOutcomeBackoff(t *testing.T) {
	pool := requireDB(t)
	ctx := context.Background()
	repo := NewCompanyRepository(pool)
	c := newCompany(t, repo, "greenhouse")
	msg := "greenhouse responded 503"

	for i := 1; i <= 3; i++ {
		if err := repo.RecordSyncOutcome(ctx, c.ID, domain.IngestionRun{Status: domain.RunFailed, Error: &msg}); err != nil {
			t.Fatal(err)
		}
	}
	got, err := repo.Get(ctx, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != domain.CompanyUnreachable || got.FailureCount != 3 || got.LastError == nil || *got.LastError != msg {
		t.Fatalf("after failures: %+v", got)
	}
	if d := time.Until(got.NextCheckAt); d < 3*time.Hour+50*time.Minute || d > 4*time.Hour+10*time.Minute {
		t.Errorf("next check in %v, want about 4h", d)
	}
	for i := 0; i < 10; i++ {
		repo.RecordSyncOutcome(ctx, c.ID, domain.IngestionRun{Status: domain.RunFailed, Error: &msg})
	}
	got, _ = repo.Get(ctx, c.ID)
	if d := time.Until(got.NextCheckAt); d > 168*time.Hour+time.Minute {
		t.Errorf("backoff %v exceeds 168h", d)
	}

	robots := "fetch: https://x/jobs: disallowed by robots.txt"
	repo.RecordSyncOutcome(ctx, c.ID, domain.IngestionRun{Status: domain.RunFailed, Error: &robots})
	if got, _ = repo.Get(ctx, c.ID); got.Status != domain.CompanyBlockedByRobots {
		t.Errorf("robots failure status = %s", got.Status)
	}

	if err := repo.RecordSyncOutcome(ctx, c.ID, domain.IngestionRun{Status: domain.RunSucceeded, FetchedCount: 0}); err != nil {
		t.Fatal(err)
	}
	if got, _ = repo.Get(ctx, c.ID); got.Status != domain.CompanyEmpty || got.FailureCount != 0 || got.LastError != nil {
		t.Fatalf("empty: %+v", got)
	}
	repo.RecordSyncOutcome(ctx, c.ID, domain.IngestionRun{Status: domain.RunSucceeded, FetchedCount: 4})
	if got, _ = repo.Get(ctx, c.ID); got.Status != domain.CompanyActive || got.LastCheckedAt == nil {
		t.Fatalf("active: %+v", got)
	}

	// A disabled company keeps its status whatever the sync says.
	repo.SetDisabled(ctx, c.ID, true)
	repo.RecordSyncOutcome(ctx, c.ID, domain.IngestionRun{Status: domain.RunSucceeded, FetchedCount: 4})
	if got, _ = repo.Get(ctx, c.ID); got.Status != domain.CompanyDisabled {
		t.Fatalf("disabled: %+v", got)
	}
}

func TestDetectionQueueAndRecord(t *testing.T) {
	pool := requireDB(t)
	ctx := context.Background()
	repo := NewCompanyRepository(pool)
	url := "https://" + uniq("pending") + ".example.com/careers"
	c, _, err := repo.UpsertDiscovered(ctx, CompanyParams{Name: "Pending", CareersURL: url, ATS: "unknown", Origin: "manual"})
	if err != nil {
		t.Fatal(err)
	}
	pending, err := repo.PendingDetection(ctx, 10_000)
	if err != nil || !containsCompany(pending, c.ID) {
		t.Fatalf("pending list misses company: %v", err)
	}
	got, err := repo.RecordDetection(ctx, c.ID, "lever", strp("acme"), domain.CompanyActive, nil)
	if err != nil || got.ATS != "lever" || *got.ATSID != "acme" || got.Status != domain.CompanyActive {
		t.Fatalf("record detection: %+v %v", got, err)
	}
	pending, _ = repo.PendingDetection(ctx, 10_000)
	if containsCompany(pending, c.ID) {
		t.Fatal("detected company still pending")
	}
}

func containsCompany(cs []domain.Company, id string) bool {
	for _, c := range cs {
		if c.ID == id {
			return true
		}
	}
	return false
}

func TestBlocksAreWorkspacePrivate(t *testing.T) {
	f1, f2 := newFixture(t), newFixture(t)
	pool := requireDB(t)
	ctx := context.Background()
	repo := NewCompanyRepository(pool)
	c := newCompany(t, repo, "lever")

	if err := repo.Block(ctx, f1.workspace.ID, c.ID); err != nil {
		t.Fatal(err)
	}
	if err := repo.Block(ctx, f1.workspace.ID, c.ID); err != nil {
		t.Fatalf("blocking twice: %v", err)
	}
	blocked := func(ws string) bool {
		list, _, err := repo.List(ctx, CompanyFilter{Query: c.Name, WorkspaceID: ws, Limit: 10})
		if err != nil || len(list) != 1 {
			t.Fatalf("list: %v %v", list, err)
		}
		return list[0].Blocked
	}
	if !blocked(f1.workspace.ID) || blocked(f2.workspace.ID) {
		t.Fatal("block leaked across workspaces")
	}
	if err := repo.Unblock(ctx, f1.workspace.ID, c.ID); err != nil || blocked(f1.workspace.ID) {
		t.Fatalf("unblock: %v", err)
	}
}

func TestListCompaniesFiltersAndPages(t *testing.T) {
	pool := requireDB(t)
	ctx := context.Background()
	repo := NewCompanyRepository(pool)
	tag := uniq("page")
	for i := 0; i < 5; i++ {
		repo.UpsertDiscovered(ctx, CompanyParams{
			Name: tag + " " + string(rune('a'+i)), CareersURL: "https://" + uniq("p") + ".example.com", ATS: "ashby",
			ATSID: strp(uniq("b")), Origin: "seed",
		})
	}
	first, more, err := repo.List(ctx, CompanyFilter{Query: tag, ATS: "ashby", Limit: 3})
	if err != nil || len(first) != 3 || !more {
		t.Fatalf("first page: %d %v %v", len(first), more, err)
	}
	last := first[2]
	second, more, err := repo.List(ctx, CompanyFilter{Query: tag, ATS: "ashby", Limit: 3, AfterName: last.Name, AfterID: last.ID})
	if err != nil || len(second) != 2 || more || second[0].Name <= last.Name {
		t.Fatalf("second page: %+v %v %v", second, more, err)
	}
	counts, err := repo.CountsByATS(ctx)
	if err != nil || counts["ashby"] < 5 {
		t.Fatalf("counts: %v %v", counts, err)
	}
}
