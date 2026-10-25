package httptransport

import (
	"context"
	"testing"
	"time"

	"example.com/rolescout/api/internal/domain"
	"example.com/rolescout/api/internal/infrastructure/database"
)

type fakeWorkspaceJobs struct {
	fakeJobs
	gotFilter database.JobFilter
	result    []domain.WorkspaceJob
	more      bool
}

func (f *fakeWorkspaceJobs) WorkspaceJobs(_ context.Context, filter database.JobFilter) ([]domain.WorkspaceJob, bool, error) {
	f.gotFilter = filter
	return f.result, f.more, nil
}

func (f *fakeWorkspaceJobs) Facets(_ context.Context, filter database.JobFilter) (database.Facets, error) {
	f.gotFilter = filter
	return database.Facets{Posted: map[string]int{"24h": 3}, Arrangement: map[string]int{"remote": 2}, Seniority: map[string]int{},
		Country: map[string]int{"TR": 1}, Technologies: []database.FacetCount{{Value: "react", Label: "react", Count: 2}},
		Companies: []database.FacetCount{{Value: companyID, Label: "Acme", Count: 1}}}, nil
}

const jobID = "22222222-2222-2222-2222-222222222222"

func TestWorkspaceJobsParamsAndResponse(t *testing.T) {
	seen := time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)
	jobs := &fakeWorkspaceJobs{result: []domain.WorkspaceJob{{
		Job:   domain.Job{ID: jobID, Title: "Full Stack", FirstSeenAt: seen, CountryCode: strp("TR"), CompanyID: strp(companyID)},
		Saved: true,
		Match: &domain.Match{Score: 88, New: true, Reasons: []domain.MatchReason{{Code: "tech", Params: map[string]string{"matched": "3", "total": "5"}}}},
	}}, more: true}
	res := &fakeResearch{profile: domain.ResearchProfile{ID: profileID}}
	app := newTestApp(Deps{Jobs: jobs, Research: res, Workspaces: allowAll})

	r := call(t, app, "GET", wsPath+"/jobs?q=react&profile="+profileID+"&sort=score&posted=7d&arrangement=remote"+
		"&technology=React&technology=go&country=tr&company="+companyID+"&closed=true&limit=10", nil)
	expectStatus(t, r, 200)
	f := jobs.gotFilter
	if f.WorkspaceID == "" || f.ProfileID != profileID || f.Sort != "score" || f.PostedWithin != 7*24*time.Hour ||
		f.WorkArrangement != "remote" || len(f.Technologies) != 2 || f.Countries[0] != "TR" || f.CompanyIDs[0] != companyID ||
		!f.IncludeClosed || f.Limit != 10 || f.Query != "react" {
		t.Fatalf("filter = %+v", f)
	}
	j := r.list()[0].(map[string]any)
	m := j["match"].(map[string]any)
	if j["saved"] != true || j["country_code"] != "TR" || m["score"].(float64) != 88 || m["new"] != true ||
		m["reasons"].([]any)[0].(map[string]any)["code"] != "tech" {
		t.Fatalf("job = %v", j)
	}
	next := r.Body["next_cursor"].(string)
	call(t, app, "GET", wsPath+"/jobs?profile="+profileID+"&sort=score&cursor="+next, nil)
	if jobs.gotFilter.After == nil || jobs.gotFilter.After.Score != 88 || jobs.gotFilter.After.ID != jobID {
		t.Fatalf("cursor = %+v", jobs.gotFilter.After)
	}
}

func TestWorkspaceJobsValidation(t *testing.T) {
	jobs := &fakeWorkspaceJobs{}
	res := &fakeResearch{}
	app := newTestApp(Deps{Jobs: jobs, Research: res, Workspaces: allowAll})
	for _, q := range []string{"posted=1y", "sort=newest", "sort=score", "sort=relevance", "country=Turkey",
		"region=narnia", "ats=Lever!", "company=x", "profile=nope", "closed=maybe", "cursor=%%"} {
		expectError(t, call(t, app, "GET", wsPath+"/jobs?"+q, nil), 422, "invalid_input")
	}
	// A profile from another workspace is not found.
	expectStatus(t, call(t, app, "GET", wsPath+"/jobs?profile="+profileID, nil), 404)
}

func TestWorkspaceJobFacets(t *testing.T) {
	jobs := &fakeWorkspaceJobs{}
	app := newTestApp(Deps{Jobs: jobs, Research: &fakeResearch{}, Workspaces: allowAll})
	r := call(t, app, "GET", wsPath+"/jobs/facets?country=TR", nil)
	expectStatus(t, r, 200)
	d := r.data()
	if d["posted"].(map[string]any)["24h"].(float64) != 3 || d["companies"].([]any)[0].(map[string]any)["label"] != "Acme" {
		t.Fatalf("facets = %s", r.Raw)
	}
	if jobs.gotFilter.Countries[0] != "TR" {
		t.Fatalf("filter = %+v", jobs.gotFilter)
	}
}

func strp(s string) *string { return &s }

func TestWorkspaceJobsRegionalFilters(t *testing.T) {
	jobs := &fakeWorkspaceJobs{}
	app := newTestApp(Deps{Jobs: jobs, Research: &fakeResearch{}, Workspaces: allowAll})
	expectStatus(t, call(t, app, "GET", wsPath+"/jobs?region=emea&country=TR&location=%C4%B0zmir&ats=lever", nil), 200)
	f := jobs.gotFilter
	if f.Location != "izmir" || f.ATS != "lever" {
		t.Errorf("location = %q, ats = %q", f.Location, f.ATS)
	}
	// A region adds its countries to the ones asked for, once each.
	n := 0
	for _, c := range f.Countries {
		if c == "TR" {
			n++
		}
	}
	if n != 1 || len(f.Countries) < 30 {
		t.Errorf("countries = %v", f.Countries)
	}
}

func TestWorkspaceJobsScoreProfileRatesUnmatchedJobs(t *testing.T) {
	seen := time.Now()
	jobs := &fakeWorkspaceJobs{result: []domain.WorkspaceJob{
		{Job: domain.Job{ID: jobID, Title: "Full Stack Developer", Technologies: []string{"react"}, WorkArrangement: "remote", FirstSeenAt: seen}},
		{Job: domain.Job{ID: "33333333-3333-3333-3333-333333333333", Title: "Sales Manager", Description: "cold calls", FirstSeenAt: seen.AddDate(-1, 0, 0)}},
	}}
	res := &fakeResearch{profile: domain.ResearchProfile{ID: profileID, TitleKeywords: []string{"full stack"}, Technologies: []string{"react"}, ExcludedKeywords: []string{"sales"}}}
	app := newTestApp(Deps{Jobs: jobs, Research: res, Workspaces: allowAll})

	r := call(t, app, "GET", wsPath+"/jobs?score_profile="+profileID, nil)
	expectStatus(t, r, 200)
	if jobs.gotFilter.ProfileID != "" {
		t.Fatalf("score_profile must not restrict the list: %+v", jobs.gotFilter)
	}
	list := r.list()
	m, _ := list[0].(map[string]any)["match"].(map[string]any)
	if m == nil || m["score"].(float64) < 60 || m["new"] != false {
		t.Fatalf("scored job match = %v", m)
	}
	if list[1].(map[string]any)["match"] != nil {
		t.Fatalf("an excluded job must get no score: %v", list[1])
	}
	expectError(t, call(t, app, "GET", wsPath+"/jobs?score_profile=nope", nil), 422, "invalid_input")
}
