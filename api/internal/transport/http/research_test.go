package httptransport

import (
	"context"
	"strconv"
	"testing"
	"time"

	"example.com/rolescout/api/internal/domain"
	"example.com/rolescout/api/internal/infrastructure/database"
)

const (
	profileID = "44444444-4444-4444-4444-444444444444"
	runID     = "55555555-5555-5555-5555-555555555555"
	companyID = "66666666-6666-6666-6666-666666666666"
)

type fakeResearch struct {
	ResearchStore
	gotParams    database.ProfileParams
	profile      domain.ResearchProfile
	run          domain.ResearchRun
	enqueueErr   error
	cancelErr    error
	gotAfter     int64
	gotLevel     string
	events       []domain.RunEvent
	deleted      bool
	gotWorkspace string
}

func (f *fakeResearch) CreateProfile(_ context.Context, ws string, p database.ProfileParams) (domain.ResearchProfile, error) {
	f.gotParams, f.gotWorkspace = p, ws
	return domain.ResearchProfile{ID: profileID, WorkspaceID: ws, Name: p.Name, Remote: p.Remote, ScheduleInterval: p.ScheduleInterval,
		TitleKeywords: p.TitleKeywords, Technologies: p.Technologies, Countries: p.Countries}, nil
}
func (f *fakeResearch) GetProfile(context.Context, string, string) (domain.ResearchProfile, error) {
	if f.profile.ID == "" {
		return f.profile, domain.ErrNotFound
	}
	return f.profile, nil
}
func (f *fakeResearch) ListProfiles(context.Context, string) ([]domain.ResearchProfile, error) {
	return []domain.ResearchProfile{f.profile}, nil
}
func (f *fakeResearch) UpdateProfile(_ context.Context, _, _ string, p database.ProfileParams) (domain.ResearchProfile, error) {
	f.gotParams = p
	out := f.profile
	out.Name, out.Remote = p.Name, p.Remote
	return out, nil
}
func (f *fakeResearch) DeleteProfile(context.Context, string, string) error {
	f.deleted = true
	return nil
}
func (f *fakeResearch) EnqueueRun(_ context.Context, ws, prof, trigger string) (domain.ResearchRun, error) {
	return domain.ResearchRun{ID: runID, WorkspaceID: ws, ProfileID: prof, State: domain.RunQueued, Trigger: trigger,
		Stages: map[domain.Stage]domain.StageProgress{}}, f.enqueueErr
}
func (f *fakeResearch) GetRun(context.Context, string, string) (domain.ResearchRun, error) {
	return f.run, nil
}
func (f *fakeResearch) ListRuns(context.Context, string, string, int) ([]domain.ResearchRun, error) {
	return []domain.ResearchRun{f.run}, nil
}
func (f *fakeResearch) ListEvents(_ context.Context, _, _ string, after int64, level string, _ int) ([]domain.RunEvent, error) {
	f.gotAfter, f.gotLevel = after, level
	return f.events, nil
}
func (f *fakeResearch) RequestCancel(context.Context, string, string) error { return f.cancelErr }

func validProfileBody() map[string]any {
	return map[string]any{
		"name": "Backend", "title_keywords": []string{"backend"}, "technologies": []string{"Go", "PostgreSQL"},
		"countries": []string{"tr", "DE"}, "remote": "remote_or_hybrid", "seniority": []string{"senior"}, "schedule_hours": 6,
	}
}

func TestCreateProfileValidatesAndNormalizes(t *testing.T) {
	res := &fakeResearch{}
	app := newTestApp(Deps{Research: res, Workspaces: allowAll})
	r := call(t, app, "POST", wsPath+"/research-profiles", validProfileBody())
	expectStatus(t, r, 201)
	p := res.gotParams
	if p.Name != "Backend" || p.Technologies[0] != "go" || p.Technologies[1] != "postgresql" || p.Countries[0] != "TR" ||
		p.Remote != domain.RemoteOrHybrid || p.ScheduleInterval != 6*time.Hour {
		t.Fatalf("params = %+v", p)
	}
	if r.data()["schedule_hours"].(float64) != 6 || r.data()["name"] != "Backend" {
		t.Errorf("response = %v", r.data())
	}

	bad := validProfileBody()
	bad["name"] = " "
	bad["countries"] = []string{"Turkey"}
	bad["remote"] = "sometimes"
	bad["seniority"] = []string{"wizard"}
	bad["schedule_hours"] = 0
	many := make([]string, 31)
	for i := range many {
		many[i] = "k" + strconv.Itoa(i)
	}
	bad["keywords"] = many
	r = call(t, app, "POST", wsPath+"/research-profiles", bad)
	expectError(t, r, 422, "invalid_input")
	fields := map[string]bool{}
	for _, f := range r.Body["error"].(map[string]any)["fields"].([]any) {
		fields[f.(map[string]any)["field"].(string)] = true
	}
	for _, want := range []string{"name", "countries", "remote", "seniority", "schedule_hours", "keywords"} {
		if !fields[want] {
			t.Errorf("missing validation error for %s: %v", want, fields)
		}
	}
}

func TestPatchProfileMergesFields(t *testing.T) {
	res := &fakeResearch{profile: domain.ResearchProfile{ID: profileID, Name: "Old", Remote: domain.RemoteAny,
		Technologies: []string{"react"}, ScheduleInterval: 12 * time.Hour}}
	app := newTestApp(Deps{Research: res, Workspaces: allowAll})
	r := call(t, app, "PATCH", wsPath+"/research-profiles/"+profileID, map[string]any{"remote": "remote_only"})
	expectStatus(t, r, 200)
	if res.gotParams.Name != "Old" || res.gotParams.Remote != domain.RemoteOnly || res.gotParams.Technologies[0] != "react" ||
		res.gotParams.ScheduleInterval != 12*time.Hour {
		t.Fatalf("merged params = %+v", res.gotParams)
	}
	expectStatus(t, call(t, app, "DELETE", wsPath+"/research-profiles/"+profileID, nil), 204)
	if !res.deleted {
		t.Error("profile not deleted")
	}
}

func TestStartRun(t *testing.T) {
	res := &fakeResearch{profile: domain.ResearchProfile{ID: profileID}}
	app := newTestApp(Deps{Research: res, Workspaces: allowAll})
	r := call(t, app, "POST", wsPath+"/research-profiles/"+profileID+"/runs", nil)
	expectStatus(t, r, 202)
	if r.data()["state"] != "queued" || r.data()["trigger"] != "manual" {
		t.Fatalf("run = %v", r.data())
	}
	res.enqueueErr = domain.ErrConflict
	expectError(t, call(t, app, "POST", wsPath+"/research-profiles/"+profileID+"/runs", nil), 409, "conflict")
	res.profile = domain.ResearchProfile{}
	expectStatus(t, call(t, app, "POST", wsPath+"/research-profiles/"+profileID+"/runs", nil), 404)
}

func TestRunDetailEventsAndCancel(t *testing.T) {
	started := time.Date(2026, 10, 17, 10, 0, 0, 0, time.UTC)
	res := &fakeResearch{
		run: domain.ResearchRun{ID: runID, ProfileID: profileID, State: domain.RunRunning, Stage: domain.StageCollection,
			StartedAt: &started, Summary: map[string]int{},
			Stages: map[domain.Stage]domain.StageProgress{domain.StageCollection: {State: "running", Total: 10, Done: 4, Detail: map[string]int{"jobs": 120}}}},
		events: []domain.RunEvent{{ID: 7, At: started, Level: "info", Stage: domain.StageCollection, Company: "Acme", Message: "greenhouse · 12 jobs"}},
	}
	app := newTestApp(Deps{Research: res, Workspaces: allowAll})
	r := call(t, app, "GET", wsPath+"/research-runs/"+runID, nil)
	expectStatus(t, r, 200)
	col := r.data()["stages"].(map[string]any)["collection"].(map[string]any)
	if col["done"].(float64) != 4 || col["detail"].(map[string]any)["jobs"].(float64) != 120 {
		t.Fatalf("stages = %v", r.data()["stages"])
	}
	r = call(t, app, "GET", wsPath+"/research-runs/"+runID+"/events?after=5&level=error&limit=50", nil)
	expectStatus(t, r, 200)
	if res.gotAfter != 5 || res.gotLevel != "error" || r.list()[0].(map[string]any)["company"] != "Acme" {
		t.Fatalf("events: after=%d level=%s body=%v", res.gotAfter, res.gotLevel, r.Raw)
	}
	expectError(t, call(t, app, "GET", wsPath+"/research-runs/"+runID+"/events?after=x&level=loud&limit=900", nil), 422, "invalid_input")
	expectStatus(t, call(t, app, "POST", wsPath+"/research-runs/"+runID+"/cancel", nil), 202)
	res.cancelErr = domain.ErrConflict
	expectError(t, call(t, app, "POST", wsPath+"/research-runs/"+runID+"/cancel", nil), 409, "conflict")
	expectStatus(t, call(t, app, "GET", wsPath+"/research-runs", nil), 200)
}

type fakeCatalog struct {
	CompanyCatalog
	gotFilter  database.CompanyFilter
	gotParams  database.CompanyParams
	blocked    bool
	reviewed   string
	disabled   bool
	reviewNote string
}

func (f *fakeCatalog) List(_ context.Context, filter database.CompanyFilter) ([]domain.Company, bool, error) {
	f.gotFilter = filter
	return []domain.Company{{ID: companyID, Name: "Acme", ATS: "lever", Status: domain.CompanyActive, OpenJobs: 3, Blocked: true}}, true, nil
}
func (f *fakeCatalog) UpsertDiscovered(_ context.Context, p database.CompanyParams) (domain.Company, bool, error) {
	f.gotParams = p
	return domain.Company{ID: companyID, Name: p.Name, CareersURL: p.CareersURL, ATS: p.ATS, Status: domain.CompanyPending}, true, nil
}
func (f *fakeCatalog) Block(context.Context, string, string) error   { f.blocked = true; return nil }
func (f *fakeCatalog) Unblock(context.Context, string, string) error { f.blocked = false; return nil }
func (f *fakeCatalog) SetDisabled(_ context.Context, id string, d bool) (domain.Company, error) {
	f.disabled = d
	return domain.Company{ID: id, Status: domain.CompanyDisabled}, nil
}
func (f *fakeCatalog) AdapterReviews(context.Context) ([]domain.AdapterReview, error) {
	return []domain.AdapterReview{{Adapter: "workday"}}, nil
}
func (f *fakeCatalog) ReviewAdapter(_ context.Context, adapter, note string, enabled bool) (domain.AdapterReview, error) {
	f.reviewed, f.reviewNote = adapter, note
	return domain.AdapterReview{Adapter: adapter, Enabled: enabled, Note: &note}, nil
}

func TestCompaniesListAddAndBlock(t *testing.T) {
	cat := &fakeCatalog{}
	app := newTestApp(Deps{Companies: cat, Workspaces: allowAll})
	r := call(t, app, "GET", wsPath+"/companies?q=acme&ats=lever&status=active&limit=1", nil)
	expectStatus(t, r, 200)
	if cat.gotFilter.Query != "acme" || cat.gotFilter.ATS != "lever" || cat.gotFilter.WorkspaceID == "" || cat.gotFilter.Limit != 1 {
		t.Fatalf("filter = %+v", cat.gotFilter)
	}
	if r.list()[0].(map[string]any)["blocked"] != true || r.Body["next_cursor"] == nil {
		t.Fatalf("body = %s", r.Raw)
	}
	next := r.Body["next_cursor"].(string)
	call(t, app, "GET", wsPath+"/companies?cursor="+next, nil)
	if cat.gotFilter.AfterID != companyID || cat.gotFilter.AfterName != "Acme" {
		t.Fatalf("cursor not decoded: %+v", cat.gotFilter)
	}
	expectError(t, call(t, app, "GET", wsPath+"/companies?status=sleepy", nil), 422, "invalid_input")

	expectError(t, call(t, app, "POST", "/api/v1/companies", map[string]any{"careers_url": "javascript:alert(1)"}), 422, "invalid_input")
	for _, internal := range []string{"http://localhost/careers", "http://127.0.0.1:8080/x", "http://10.1.2.3/jobs", "http://postgres:5432/", "http://api.internal/"} {
		expectError(t, call(t, app, "POST", "/api/v1/companies", map[string]any{"careers_url": internal}), 422, "invalid_input")
	}
	r = call(t, app, "POST", "/api/v1/companies", map[string]any{"careers_url": "https://jobs.lever.co/acme"})
	expectStatus(t, r, 202)
	if cat.gotParams.ATS != "lever" || *cat.gotParams.ATSID != "acme" || cat.gotParams.Origin != "manual" || cat.gotParams.Name != "acme" {
		t.Fatalf("params = %+v", cat.gotParams)
	}
	r = call(t, app, "POST", "/api/v1/companies", map[string]any{"careers_url": "https://www.acme.example/careers", "name": "Acme"})
	expectStatus(t, r, 202)
	if cat.gotParams.ATS != "unknown" || cat.gotParams.Name != "Acme" {
		t.Fatalf("params = %+v", cat.gotParams)
	}

	expectStatus(t, call(t, app, "PUT", wsPath+"/company-blocks/"+companyID, nil), 204)
	if !cat.blocked {
		t.Fatal("not blocked")
	}
	expectStatus(t, call(t, app, "DELETE", wsPath+"/company-blocks/"+companyID, nil), 204)
}

func TestAdminRoutesNeedAdmin(t *testing.T) {
	cat := &fakeCatalog{}
	app := newTestApp(Deps{Companies: cat, Workspaces: allowAll, CrawlSettings: []Setting{{Name: "CRAWL_CONCURRENCY", Value: "16"}}})
	for _, req := range []struct{ method, path string }{
		{"GET", "/api/v1/admin/adapters"}, {"PATCH", "/api/v1/admin/adapters/workday"},
		{"PATCH", "/api/v1/admin/companies/" + companyID}, {"GET", "/api/v1/admin/settings"},
	} {
		expectError(t, call(t, app, req.method, req.path, map[string]any{}), 403, "forbidden")
	}

	admin := testUser
	admin.IsAdmin = true
	app = newTestApp(Deps{Companies: cat, Workspaces: allowAll, Auth: fakeAuthAs(admin), CrawlSettings: []Setting{{Name: "CRAWL_CONCURRENCY", Value: "16"}}})
	expectStatus(t, call(t, app, "GET", "/api/v1/admin/adapters", nil), 200)
	expectError(t, call(t, app, "PATCH", "/api/v1/admin/adapters/workday", map[string]any{"enabled": true, "note": " "}), 422, "invalid_input")
	r := call(t, app, "PATCH", "/api/v1/admin/adapters/workday", map[string]any{"enabled": true, "note": "terms checked"})
	expectStatus(t, r, 200)
	if cat.reviewed != "workday" || cat.reviewNote != "terms checked" {
		t.Fatalf("review = %s %s", cat.reviewed, cat.reviewNote)
	}
	expectStatus(t, call(t, app, "PATCH", "/api/v1/admin/companies/"+companyID, map[string]any{"disabled": true}), 200)
	if !cat.disabled {
		t.Fatal("company not disabled")
	}
	r = call(t, app, "GET", "/api/v1/admin/settings", nil)
	expectStatus(t, r, 200)
	if r.list()[0].(map[string]any)["value"] != "16" {
		t.Fatalf("settings = %s", r.Raw)
	}
	r = call(t, app, "GET", "/api/v1/auth/me", nil)
	if r.data()["is_admin"] != true {
		t.Fatalf("me = %s", r.Raw)
	}
}
