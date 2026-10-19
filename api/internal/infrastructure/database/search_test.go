package database

import (
	"context"
	"testing"
	"time"

	"example.com/rolescout/api/internal/domain"
)

// searchFixture creates jobs that only this test can find through a unique
// company, and returns the company name for filtering.
type searchFixture struct {
	f       fixture
	company domain.Company
	jobs    map[string]domain.Job
}

func newSearchFixture(t *testing.T) searchFixture {
	t.Helper()
	f := newFixture(t)
	ctx := context.Background()
	pool := requireDB(t)
	repo := NewJobRepository(pool)
	co := newCompany(t, NewCompanyRepository(pool), "greenhouse")
	sf := searchFixture{f: f, company: co, jobs: map[string]domain.Job{}}
	add := func(key, title, desc, arrangement, seniority, country string, techs []string, age time.Duration) {
		c := country
		j, _, err := repo.UpsertListing(ctx, IngestedListing{
			SourceID: f.source.ID, ExternalID: uniq(key), URL: "https://x.example/" + key, DedupeKey: uniq("dk"),
			Title: title, CompanyName: co.Name, Description: desc, WorkArrangement: arrangement, Seniority: seniority,
			Technologies: techs, CompanyID: &co.ID, CountryCode: &c, SeenAt: time.Now().Add(-age),
		})
		if err != nil {
			t.Fatal(err)
		}
		sf.jobs[key] = j
	}
	add("title", "Full Stack Developer", "We build things.", "remote", "senior", "TR", []string{"react", "go"}, time.Hour)
	add("desc", "Platform Engineer", "Work across the full stack with React.", "hybrid", "mid", "DE", []string{"react"}, 3*24*time.Hour)
	add("tr", "Kıdemli Yazılım Geliştirici", "Türkçe ilan, geliştirici arıyoruz.", "onsite", "senior", "TR", []string{"vue"}, 20*24*time.Hour)
	add("old", "Accountant", "Numbers.", "onsite", "junior", "DE", nil, 40*24*time.Hour)
	return sf
}

func (sf searchFixture) filter() JobFilter {
	return JobFilter{WorkspaceID: sf.f.workspace.ID, CompanyIDs: []string{sf.company.ID}, Limit: 50}
}

func ids(jobs []domain.WorkspaceJob) []string {
	var out []string
	for _, j := range jobs {
		out = append(out, j.Job.ID)
	}
	return out
}

func TestSearchRanksTitleAboveDescription(t *testing.T) {
	sf := newSearchFixture(t)
	repo := NewJobRepository(requireDB(t))
	f := sf.filter()
	f.Query, f.Sort = "full stack", "relevance"
	got, _, err := repo.WorkspaceJobs(context.Background(), f)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Job.ID != sf.jobs["title"].ID || got[1].Job.ID != sf.jobs["desc"].ID {
		t.Fatalf("results = %v", ids(got))
	}
}

func TestSearchTurkishCharacters(t *testing.T) {
	sf := newSearchFixture(t)
	repo := NewJobRepository(requireDB(t))
	f := sf.filter()
	f.Query = "geliştirici"
	got, _, err := repo.WorkspaceJobs(context.Background(), f)
	if err != nil || len(got) != 1 || got[0].Job.ID != sf.jobs["tr"].ID {
		t.Fatalf("results = %v, %v", ids(got), err)
	}
}

func TestWorkspaceJobsFilters(t *testing.T) {
	sf := newSearchFixture(t)
	ctx := context.Background()
	repo := NewJobRepository(requireDB(t))
	for name, tc := range map[string]struct {
		mod  func(*JobFilter)
		want []string
	}{
		"posted 7d":  {func(f *JobFilter) { f.PostedWithin = 7 * 24 * time.Hour }, []string{"title", "desc"}},
		"country TR": {func(f *JobFilter) { f.Countries = []string{"TR"} }, []string{"title", "tr"}},
		"remote":     {func(f *JobFilter) { f.WorkArrangement = "remote" }, []string{"title"}},
		"react":      {func(f *JobFilter) { f.Technologies = []string{"react"} }, []string{"title", "desc"}},
		"senior":     {func(f *JobFilter) { f.Seniority = "senior" }, []string{"title", "tr"}},
	} {
		f := sf.filter()
		tc.mod(&f)
		got, _, err := repo.WorkspaceJobs(ctx, f)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		want := map[string]bool{}
		for _, k := range tc.want {
			want[sf.jobs[k].ID] = true
		}
		if len(got) != len(want) {
			t.Errorf("%s: got %d jobs, want %d", name, len(got), len(want))
		}
		for _, j := range got {
			if !want[j.Job.ID] {
				t.Errorf("%s: unexpected job %s", name, j.Job.Title)
			}
		}
	}
}

func TestRecentSortPagesWithoutRepeats(t *testing.T) {
	sf := newSearchFixture(t)
	repo := NewJobRepository(requireDB(t))
	f := sf.filter()
	f.Limit = 3
	first, more, err := repo.WorkspaceJobs(context.Background(), f)
	if err != nil || len(first) != 3 || !more || first[0].Job.ID != sf.jobs["title"].ID {
		t.Fatalf("first page = %v more=%v err=%v", ids(first), more, err)
	}
	c := CursorAfter(f, first[2])
	f.After = &c
	second, more, err := repo.WorkspaceJobs(context.Background(), f)
	if err != nil || len(second) != 1 || more || second[0].Job.ID != sf.jobs["old"].ID {
		t.Fatalf("second page = %v more=%v err=%v", ids(second), more, err)
	}
}

func TestScoreSortKeysetAndSavedFlag(t *testing.T) {
	sf := newSearchFixture(t)
	ctx := context.Background()
	pool := requireDB(t)
	repo := NewJobRepository(pool)
	research := NewResearchRepository(pool)
	profiles, _ := research.ListProfiles(ctx, sf.f.workspace.ID)
	prof := profiles[0].ID
	research.ReplaceMatches(ctx, prof, []domain.Match{
		{JobID: sf.jobs["title"].ID, Score: 90}, {JobID: sf.jobs["desc"].ID, Score: 70}, {JobID: sf.jobs["tr"].ID, Score: 70},
	}, nil)
	repo.SaveJob(ctx, sf.f.workspace.ID, sf.jobs["desc"].ID, nil)

	f := sf.filter()
	f.ProfileID, f.Sort, f.Limit = prof, "score", 2
	var all []domain.WorkspaceJob
	for page := 0; page < 3; page++ {
		got, more, err := repo.WorkspaceJobs(ctx, f)
		if err != nil {
			t.Fatal(err)
		}
		all = append(all, got...)
		if !more {
			break
		}
		c := CursorAfter(f, got[len(got)-1])
		f.After = &c
	}
	if len(all) != 3 || all[0].Job.ID != sf.jobs["title"].ID || all[0].Match == nil || all[0].Match.Score != 90 {
		t.Fatalf("pages = %v", ids(all))
	}
	seen := map[string]bool{}
	for _, j := range all {
		if seen[j.Job.ID] {
			t.Fatal("a job was repeated across pages")
		}
		seen[j.Job.ID] = true
		if j.Saved != (j.Job.ID == sf.jobs["desc"].ID) {
			t.Errorf("saved flag of %s = %v", j.Job.Title, j.Saved)
		}
	}
}

func TestBlockedAndClosedHidden(t *testing.T) {
	sf := newSearchFixture(t)
	ctx := context.Background()
	pool := requireDB(t)
	repo := NewJobRepository(pool)
	pool.Exec(ctx, `UPDATE jobs SET closed_at = NOW() WHERE id = $1`, sf.jobs["old"].ID)
	got, _, _ := repo.WorkspaceJobs(ctx, sf.filter())
	if len(got) != 3 {
		t.Fatalf("closed job shown: %d jobs", len(got))
	}
	f := sf.filter()
	f.IncludeClosed = true
	if got, _, _ := repo.WorkspaceJobs(ctx, f); len(got) != 4 {
		t.Fatalf("closed=true: %d jobs", len(got))
	}
	NewCompanyRepository(pool).Block(ctx, sf.f.workspace.ID, sf.company.ID)
	if got, _, _ := repo.WorkspaceJobs(ctx, sf.filter()); len(got) != 0 {
		t.Fatalf("blocked company's jobs shown: %d", len(got))
	}
}

func TestFacetsExcludeOwnDimension(t *testing.T) {
	sf := newSearchFixture(t)
	repo := NewJobRepository(requireDB(t))
	f := sf.filter()
	f.Countries = []string{"TR"}
	fc, err := repo.Facets(context.Background(), f)
	if err != nil {
		t.Fatal(err)
	}
	// Country counts ignore the country filter, so DE is still offered.
	if fc.Country["TR"] != 2 || fc.Country["DE"] != 2 {
		t.Errorf("country facet = %v", fc.Country)
	}
	// Other facets respect it: TR jobs are remote and onsite.
	if fc.Arrangement["remote"] != 1 || fc.Arrangement["onsite"] != 1 || fc.Arrangement["hybrid"] != 0 {
		t.Errorf("arrangement facet = %v", fc.Arrangement)
	}
	if fc.Posted["24h"] != 1 || fc.Posted["30d"] != 2 {
		t.Errorf("posted facet = %v", fc.Posted)
	}
	var react int
	for _, tc := range fc.Technologies {
		if tc.Value == "react" {
			react = tc.Count
		}
	}
	// The company facet ignores the company filter, so other tests'
	// companies appear too; ours has its two TR jobs.
	var ours *FacetCount
	for i, c := range fc.Companies {
		if c.Value == sf.company.ID {
			ours = &fc.Companies[i]
		}
	}
	if react != 1 || ours == nil || ours.Label != sf.company.Name || ours.Count != 2 {
		t.Errorf("tech react = %d company = %+v", react, ours)
	}
}

func TestWorkspaceJobsFilterByPlaceAndJobBoardSystem(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	pool := requireDB(t)
	repo := NewJobRepository(pool)
	co := newCompany(t, NewCompanyRepository(pool), "lever")
	add := func(key, loc string) {
		l := loc
		if _, _, err := repo.UpsertListing(ctx, IngestedListing{
			SourceID: f.source.ID, ExternalID: uniq(key), URL: "https://x.example/" + key, DedupeKey: uniq("dk"),
			Title: "Engineer " + key, CompanyName: co.Name, Location: &l, CompanyID: &co.ID, SeenAt: time.Now(),
		}); err != nil {
			t.Fatal(err)
		}
	}
	add("a", "İstanbul, Türkiye")
	add("b", "Şişli / ISTANBUL")
	add("c", "Berlin, Germany")
	base := JobFilter{WorkspaceID: f.workspace.ID, CompanyIDs: []string{co.ID}, Limit: 50}
	count := func(mod func(*JobFilter)) int {
		fl := base
		mod(&fl)
		got, _, err := repo.WorkspaceJobs(ctx, fl)
		if err != nil {
			t.Fatal(err)
		}
		return len(got)
	}
	for in, want := range map[string]int{"istanbul": 2, "turkiye": 1, "sisli": 1, "100%": 0, "berlin": 1} {
		if got := count(func(fl *JobFilter) { fl.Location = in }); got != want {
			t.Errorf("location %q matched %d jobs, want %d", in, got, want)
		}
	}
	if got := count(func(fl *JobFilter) { fl.ATS = "lever" }); got != 0 {
		t.Errorf("ats lever before the source is a Lever source: %d", got)
	}
	if _, err := pool.Exec(ctx, `UPDATE job_sources SET adapter = 'lever' WHERE id = $1`, f.source.ID); err != nil {
		t.Fatal(err)
	}
	if got := count(func(fl *JobFilter) { fl.ATS = "lever" }); got != 3 {
		t.Errorf("ats lever matched %d jobs, want 3", got)
	}
	if got := count(func(fl *JobFilter) { fl.ATS = "greenhouse" }); got != 0 {
		t.Errorf("ats greenhouse matched %d jobs", got)
	}
}
