package httptransport

import (
	"context"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"

	"example.com/rolescout/api/internal/infrastructure/database"
)

type apiFixture struct {
	app        *fiber.App
	workspaceA string
	workspaceB string
	jobID      string
	otherJobID string
	company    string
}

func newAPIFixture(t *testing.T) apiFixture {
	t.Helper()
	if testPool == nil {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	users := database.NewUserRepository(testPool)
	workspaces := database.NewWorkspaceRepository(testPool)
	jobs := database.NewJobRepository(testPool)

	var wsIDs []string
	for _, name := range []string{"a", "b"} {
		u, err := users.Create(ctx, database.CreateUserParams{
			Email: name + suffix + "@example.com", DisplayName: name,
		})
		if err != nil {
			t.Fatal(err)
		}
		w, err := workspaces.Create(ctx, "Workspace "+name, u.ID)
		if err != nil {
			t.Fatal(err)
		}
		wsIDs = append(wsIDs, w.ID)
	}

	source, err := jobs.CreateSource(ctx, "api-"+suffix, "API Source", "https://jobs.example.com")
	if err != nil {
		t.Fatal(err)
	}
	company := "Integration Co " + suffix
	var jobIDs []string
	for i, title := range []string{"Backend Engineer", "Data Engineer"} {
		job, _, err := jobs.UpsertListing(ctx, database.IngestedListing{
			SourceID: source.ID, ExternalID: fmt.Sprintf("%s-%d", suffix, i),
			URL: "https://jobs.example.com/" + suffix, DedupeKey: fmt.Sprintf("%s-%d", suffix, i),
			Title: title, CompanyName: company, WorkArrangement: "remote",
			Technologies: []string{"go"},
		})
		if err != nil {
			t.Fatal(err)
		}
		jobIDs = append(jobIDs, job.ID)
	}

	app := newTestApp(Deps{
		Jobs:         jobs,
		Applications: database.NewApplicationRepository(testPool),
		Annotations:  database.NewAnnotationRepository(testPool),
		Workspaces:   ExistingWorkspace{Workspaces: workspaces},
	})
	return apiFixture{
		app: app, workspaceA: wsIDs[0], workspaceB: wsIDs[1],
		jobID: jobIDs[0], otherJobID: jobIDs[1], company: company,
	}
}

func TestAPIApplicationTrackingFlow(t *testing.T) {
	f := newAPIFixture(t)
	ws := "/api/v1/workspaces/" + f.workspaceA

	// Job discovery.
	r := call(t, f.app, "GET", "/api/v1/jobs?work_arrangement=remote&technology=GO&q="+url.QueryEscape(f.company), nil)
	expectStatus(t, r, 200)
	if len(r.list()) != 2 {
		t.Fatalf("search returned %d jobs", len(r.list()))
	}
	r = call(t, f.app, "GET", "/api/v1/jobs/"+f.jobID, nil)
	expectStatus(t, r, 200)
	if listings := r.data()["listings"].([]any); len(listings) != 1 {
		t.Fatalf("listings = %d", len(listings))
	}

	// Saved jobs are idempotent.
	for range 2 {
		expectStatus(t, call(t, f.app, "PUT", ws+"/saved-jobs/"+f.jobID, nil), 204)
	}
	r = call(t, f.app, "GET", ws+"/saved-jobs", nil)
	expectStatus(t, r, 200)
	if saved := r.list(); len(saved) != 1 || saved[0].(map[string]any)["job"].(map[string]any)["title"] != "Backend Engineer" {
		t.Fatalf("saved jobs = %s", r.Raw)
	}
	expectStatus(t, call(t, f.app, "DELETE", ws+"/saved-jobs/"+f.jobID, nil), 204)
	expectStatus(t, call(t, f.app, "DELETE", ws+"/saved-jobs/"+f.jobID, nil), 404)

	// Application lifecycle.
	r = call(t, f.app, "POST", ws+"/applications", map[string]string{"job_id": f.jobID})
	expectStatus(t, r, 201)
	appID := r.data()["id"].(string)
	appPath := ws + "/applications/" + appID
	expectStatus(t, call(t, f.app, "POST", ws+"/applications", map[string]string{"job_id": f.jobID}), 409)
	expectStatus(t, call(t, f.app, "POST", ws+"/applications",
		map[string]string{"job_id": "00000000-0000-0000-0000-000000000000"}), 404)

	r = call(t, f.app, "POST", appPath+"/attempts", map[string]string{
		"applied_at": "2026-10-10T09:30:00+03:00", "channel": "Company site",
	})
	expectStatus(t, r, 201)
	if r.data()["attempt_number"].(float64) != 1 || r.data()["applied_at"] != "2026-10-10T06:30:00Z" {
		t.Fatalf("attempt = %s", r.Raw)
	}
	r = call(t, f.app, "GET", appPath, nil)
	if r.data()["status"] != "applied" {
		t.Fatalf("status after attempt = %v", r.data()["status"])
	}

	r = call(t, f.app, "PATCH", appPath, map[string]string{"status": "interviewing"})
	expectStatus(t, r, 200)
	r = call(t, f.app, "PATCH", appPath, map[string]string{"status": "planned"})
	expectStatus(t, r, 409)
	if r.errorCode() != "invalid_transition" {
		t.Fatalf("code = %q", r.errorCode())
	}

	r = call(t, f.app, "POST", appPath+"/events", map[string]any{
		"type": "interview", "occurred_at": "2026-10-20T14:00:00Z", "details": map[string]string{"round": "technical"},
	})
	expectStatus(t, r, 201)
	expectStatus(t, call(t, f.app, "POST", appPath+"/events", map[string]any{
		"type": "status_changed", "occurred_at": "2026-10-20T14:00:00Z",
	}), 422)

	r = call(t, f.app, "GET", appPath+"/events", nil)
	expectStatus(t, r, 200)
	var types []string
	for _, e := range r.list() {
		types = append(types, e.(map[string]any)["type"].(string))
	}
	// History is ordered by when things happened: the attempt on Oct 10 comes
	// before the application was added to RoleScout.
	if fmt.Sprint(types) != "[attempt_added created status_changed status_changed interview]" {
		t.Fatalf("event types = %v", types)
	}

	r = call(t, f.app, "GET", ws+"/applications?status=interviewing", nil)
	expectStatus(t, r, 200)
	if len(r.list()) != 1 {
		t.Fatalf("interviewing applications = %d", len(r.list()))
	}

	// Tags.
	r = call(t, f.app, "POST", ws+"/tags", map[string]string{"name": "Remote"})
	expectStatus(t, r, 201)
	tagID := r.data()["id"].(string)
	expectStatus(t, call(t, f.app, "POST", ws+"/tags", map[string]string{"name": "remote"}), 409)
	expectStatus(t, call(t, f.app, "PUT", appPath+"/tags/"+tagID, nil), 204)
	r = call(t, f.app, "GET", appPath+"/tags", nil)
	if len(r.list()) != 1 {
		t.Fatalf("application tags = %s", r.Raw)
	}
	expectStatus(t, call(t, f.app, "DELETE", appPath+"/tags/"+tagID, nil), 204)
	expectStatus(t, call(t, f.app, "DELETE", appPath+"/tags/"+tagID, nil), 404)

	// Notes are scoped to their application.
	r = call(t, f.app, "POST", appPath+"/notes", map[string]string{"body": "Recruiter called"})
	expectStatus(t, r, 201)
	noteID := r.data()["id"].(string)
	expectStatus(t, call(t, f.app, "POST", appPath+"/notes", map[string]string{"body": "  "}), 422)
	r = call(t, f.app, "PATCH", appPath+"/notes/"+noteID, map[string]string{"body": "Recruiter called twice"})
	expectStatus(t, r, 200)

	r = call(t, f.app, "POST", ws+"/applications", map[string]string{"job_id": f.otherJobID})
	expectStatus(t, r, 201)
	otherAppPath := ws + "/applications/" + r.data()["id"].(string)
	expectStatus(t, call(t, f.app, "PATCH", otherAppPath+"/notes/"+noteID, map[string]string{"body": "moved"}), 404)
}

func TestAPIWorkspaceIsolation(t *testing.T) {
	f := newAPIFixture(t)

	r := call(t, f.app, "POST", "/api/v1/workspaces/"+f.workspaceA+"/applications", map[string]string{"job_id": f.jobID})
	expectStatus(t, r, 201)
	appID := r.data()["id"].(string)

	other := "/api/v1/workspaces/" + f.workspaceB + "/applications/" + appID
	for _, req := range []struct{ method, path string }{
		{"GET", other},
		{"GET", other + "/attempts"},
		{"GET", other + "/events"},
		{"GET", other + "/notes"},
		{"GET", other + "/tags"},
	} {
		r := call(t, f.app, req.method, req.path, nil)
		expectStatus(t, r, 404)
	}
	expectStatus(t, call(t, f.app, "PATCH", other, map[string]string{"status": "withdrawn"}), 404)
	expectStatus(t, call(t, f.app, "POST", other+"/attempts", map[string]string{"applied_at": "2026-10-10T09:00:00Z"}), 404)
	expectStatus(t, call(t, f.app, "POST", other+"/notes", map[string]string{"body": "x"}), 404)

	r = call(t, f.app, "GET", "/api/v1/workspaces/"+f.workspaceB+"/applications", nil)
	expectStatus(t, r, 200)
	if len(r.list()) != 0 {
		t.Fatalf("workspace B sees %d applications", len(r.list()))
	}

	expectStatus(t, call(t, f.app, "GET", "/api/v1/workspaces/00000000-0000-0000-0000-000000000000/applications", nil), 404)
}
