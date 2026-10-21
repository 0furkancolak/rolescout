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
	a, b       client
	userA      string
	userB      string
	workspaceA string
	workspaceB string
	jobID      string
	otherJobID string
	company    string
}

func newIntegrationApp(t *testing.T, cfg AuthConfig) *fiber.App {
	t.Helper()
	if testPool == nil {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	if cfg.SessionTTL == 0 {
		cfg.SessionTTL = time.Hour
	}
	workspaces := database.NewWorkspaceRepository(testPool)
	return newTestApp(Deps{
		Auth:           database.NewAuthRepository(testPool),
		AuthConfig:     cfg,
		WorkspaceStore: workspaces,
		Jobs:           database.NewJobRepository(testPool),
		Applications:   database.NewApplicationRepository(testPool),
		UserState:      database.NewUserStateRepository(testPool),
		Annotations:    database.NewAnnotationRepository(testPool),
		Workspaces:     MembershipAuthorizer{Workspaces: workspaces},
	})
}

// registerUser signs up through the API and returns a client carrying the
// session cookie, the user ID and the personal workspace ID.
func registerUser(t *testing.T, app *fiber.App, email string) (client, string, string) {
	t.Helper()
	r := anonymous.call(t, app, "POST", "/api/v1/auth/register", map[string]string{
		"email": email, "password": "correct horse battery", "display_name": "Test User",
	})
	expectStatus(t, r, 201)
	cookie := r.sessionCookie()
	if cookie == nil {
		t.Fatal("register did not set a session cookie")
	}
	cl := client{token: cookie.Value}
	ws := cl.call(t, app, "GET", "/api/v1/workspaces", nil)
	expectStatus(t, ws, 200)
	if len(ws.list()) != 1 {
		t.Fatalf("new user has %d workspaces", len(ws.list()))
	}
	return cl, r.data()["id"].(string), ws.list()[0].(map[string]any)["id"].(string)
}

func uniqueEmail(prefix string) string {
	return fmt.Sprintf("%s-%d@example.com", prefix, time.Now().UnixNano())
}

func newAPIFixture(t *testing.T) apiFixture {
	t.Helper()
	app := newIntegrationApp(t, AuthConfig{AllowRegistration: true})
	ctx := context.Background()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	f := apiFixture{app: app}
	f.a, f.userA, f.workspaceA = registerUser(t, app, uniqueEmail("a"))
	f.b, f.userB, f.workspaceB = registerUser(t, app, uniqueEmail("b"))

	jobs := database.NewJobRepository(testPool)
	source, err := jobs.CreateSource(ctx, "api-"+suffix, "API Source", "https://jobs.example.com")
	if err != nil {
		t.Fatal(err)
	}
	f.company = "Integration Co " + suffix
	var jobIDs []string
	for i, title := range []string{"Backend Engineer", "Data Engineer"} {
		job, _, err := jobs.UpsertListing(ctx, database.IngestedListing{
			SourceID: source.ID, ExternalID: fmt.Sprintf("%s-%d", suffix, i),
			URL: "https://jobs.example.com/" + suffix, DedupeKey: fmt.Sprintf("%s-%d", suffix, i),
			Title: title, CompanyName: f.company, WorkArrangement: "remote",
			Technologies: []string{"go"},
		})
		if err != nil {
			t.Fatal(err)
		}
		jobIDs = append(jobIDs, job.ID)
	}
	f.jobID, f.otherJobID = jobIDs[0], jobIDs[1]
	return f
}

func TestAPIApplicationTrackingFlow(t *testing.T) {
	f := newAPIFixture(t)
	ws := "/api/v1/workspaces/" + f.workspaceA

	// Job discovery.
	r := f.a.call(t, f.app, "GET", "/api/v1/jobs?work_arrangement=remote&technology=GO&q="+url.QueryEscape(f.company), nil)
	expectStatus(t, r, 200)
	if len(r.list()) != 2 {
		t.Fatalf("search returned %d jobs", len(r.list()))
	}
	r = f.a.call(t, f.app, "GET", "/api/v1/jobs/"+f.jobID, nil)
	expectStatus(t, r, 200)
	if listings := r.data()["listings"].([]any); len(listings) != 1 {
		t.Fatalf("listings = %d", len(listings))
	}

	// Saved jobs are idempotent.
	for range 2 {
		expectStatus(t, f.a.call(t, f.app, "PUT", ws+"/saved-jobs/"+f.jobID, nil), 204)
	}
	r = f.a.call(t, f.app, "GET", ws+"/saved-jobs", nil)
	expectStatus(t, r, 200)
	if saved := r.list(); len(saved) != 1 || saved[0].(map[string]any)["job"].(map[string]any)["title"] != "Backend Engineer" {
		t.Fatalf("saved jobs = %s", r.Raw)
	}
	expectStatus(t, f.a.call(t, f.app, "DELETE", ws+"/saved-jobs/"+f.jobID, nil), 204)
	expectStatus(t, f.a.call(t, f.app, "DELETE", ws+"/saved-jobs/"+f.jobID, nil), 404)

	// Application lifecycle.
	r = f.a.call(t, f.app, "POST", ws+"/applications", map[string]string{"job_id": f.jobID})
	expectStatus(t, r, 201)
	appID := r.data()["id"].(string)
	appPath := ws + "/applications/" + appID
	expectStatus(t, f.a.call(t, f.app, "POST", ws+"/applications", map[string]string{"job_id": f.jobID}), 409)
	expectStatus(t, f.a.call(t, f.app, "POST", ws+"/applications",
		map[string]string{"job_id": "00000000-0000-0000-0000-000000000000"}), 404)

	r = f.a.call(t, f.app, "POST", appPath+"/attempts", map[string]string{
		"applied_at": "2026-10-10T09:30:00+03:00", "channel": "Company site",
	})
	expectStatus(t, r, 201)
	if r.data()["attempt_number"].(float64) != 1 || r.data()["applied_at"] != "2026-10-10T06:30:00Z" {
		t.Fatalf("attempt = %s", r.Raw)
	}
	r = f.a.call(t, f.app, "GET", appPath, nil)
	if r.data()["status"] != "applied" {
		t.Fatalf("status after attempt = %v", r.data()["status"])
	}

	r = f.a.call(t, f.app, "PATCH", appPath, map[string]string{"status": "interviewing"})
	expectStatus(t, r, 200)
	// A status can be corrected to any other (a mistaken "applied" goes back
	// to planned and forward again); unknown statuses are refused.
	expectStatus(t, f.a.call(t, f.app, "PATCH", appPath, map[string]string{"status": "planned"}), 200)
	expectStatus(t, f.a.call(t, f.app, "PATCH", appPath, map[string]string{"status": "interviewing"}), 200)
	expectStatus(t, f.a.call(t, f.app, "PATCH", appPath, map[string]string{"status": "ghosted"}), 422)

	r = f.a.call(t, f.app, "POST", appPath+"/events", map[string]any{
		"type": "interview", "occurred_at": "2031-10-20T14:00:00Z", "details": map[string]string{"round": "technical"},
	})
	expectStatus(t, r, 201)
	expectStatus(t, f.a.call(t, f.app, "POST", appPath+"/events", map[string]any{
		"type": "status_changed", "occurred_at": "2031-10-20T14:00:00Z",
	}), 422)

	r = f.a.call(t, f.app, "GET", appPath+"/events", nil)
	expectStatus(t, r, 200)
	var types []string
	for _, e := range r.list() {
		types = append(types, e.(map[string]any)["type"].(string))
	}
	// History is ordered by when things happened: the attempt on Oct 10 comes
	// before the application was added to RoleScout.
	if fmt.Sprint(types) != "[attempt_added created status_changed status_changed status_changed status_changed interview]" {
		t.Fatalf("event types = %v", types)
	}

	r = f.a.call(t, f.app, "GET", ws+"/applications?status=interviewing", nil)
	expectStatus(t, r, 200)
	if len(r.list()) != 1 {
		t.Fatalf("interviewing applications = %d", len(r.list()))
	}

	// Tags.
	r = f.a.call(t, f.app, "POST", ws+"/tags", map[string]string{"name": "Remote"})
	expectStatus(t, r, 201)
	tagID := r.data()["id"].(string)
	expectStatus(t, f.a.call(t, f.app, "POST", ws+"/tags", map[string]string{"name": "remote"}), 409)
	expectStatus(t, f.a.call(t, f.app, "PUT", appPath+"/tags/"+tagID, nil), 204)
	r = f.a.call(t, f.app, "GET", appPath+"/tags", nil)
	if len(r.list()) != 1 {
		t.Fatalf("application tags = %s", r.Raw)
	}
	expectStatus(t, f.a.call(t, f.app, "DELETE", appPath+"/tags/"+tagID, nil), 204)
	expectStatus(t, f.a.call(t, f.app, "DELETE", appPath+"/tags/"+tagID, nil), 404)

	// Notes are scoped to their application.
	r = f.a.call(t, f.app, "POST", appPath+"/notes", map[string]string{"body": "Recruiter called"})
	expectStatus(t, r, 201)
	noteID := r.data()["id"].(string)
	expectStatus(t, f.a.call(t, f.app, "POST", appPath+"/notes", map[string]string{"body": "  "}), 422)
	r = f.a.call(t, f.app, "PATCH", appPath+"/notes/"+noteID, map[string]string{"body": "Recruiter called twice"})
	expectStatus(t, r, 200)

	r = f.a.call(t, f.app, "POST", ws+"/applications", map[string]string{"job_id": f.otherJobID})
	expectStatus(t, r, 201)
	otherAppPath := ws + "/applications/" + r.data()["id"].(string)
	expectStatus(t, f.a.call(t, f.app, "PATCH", otherAppPath+"/notes/"+noteID, map[string]string{"body": "moved"}), 404)

	// An application recorded by mistake can be deleted with everything under
	// it, but not by another workspace, and only once.
	expectStatus(t, f.b.call(t, f.app, "DELETE", "/api/v1/workspaces/"+f.workspaceB+"/applications/"+r.data()["id"].(string), nil), 404)
	expectStatus(t, f.a.call(t, f.app, "DELETE", otherAppPath, nil), 204)
	expectStatus(t, f.a.call(t, f.app, "DELETE", otherAppPath, nil), 404)
	expectStatus(t, f.a.call(t, f.app, "GET", otherAppPath, nil), 404)
	expectStatus(t, f.a.call(t, f.app, "DELETE", appPath, nil), 204)
	expectStatus(t, f.a.call(t, f.app, "GET", appPath+"/notes", nil), 404)
}

func TestAPIUserState(t *testing.T) {
	f := newAPIFixture(t)
	ws := "/api/v1/workspaces/" + f.workspaceA
	expectStatus(t, f.a.call(t, f.app, "GET", ws+"/state/jobs_view", nil), 404)
	expectStatus(t, f.a.call(t, f.app, "PUT", ws+"/state/jobs_view", map[string]string{"query": "technology=go&job=" + f.jobID}), 204)
	r := f.a.call(t, f.app, "GET", ws+"/state/jobs_view", nil)
	expectStatus(t, r, 200)
	if r.data()["query"] != "technology=go&job="+f.jobID {
		t.Fatalf("state = %s", r.Raw)
	}
	// Replaced, not merged; unknown keys do not exist; another user sees nothing.
	expectStatus(t, f.a.call(t, f.app, "PUT", ws+"/state/jobs_view", map[string]string{"other": "x"}), 204)
	if r = f.a.call(t, f.app, "GET", ws+"/state/jobs_view", nil); r.data()["query"] != nil || r.data()["other"] != "x" {
		t.Fatalf("state after replace = %s", r.Raw)
	}
	expectStatus(t, f.a.call(t, f.app, "PUT", ws+"/state/secrets", map[string]string{"a": "b"}), 404)
	expectStatus(t, f.b.call(t, f.app, "GET", "/api/v1/workspaces/"+f.workspaceB+"/state/jobs_view", nil), 404)
	expectStatus(t, f.b.call(t, f.app, "GET", ws+"/state/jobs_view", nil), 404)
}

func TestAPIJobSummaryFallsBackToTheListingFactsWithoutTheModel(t *testing.T) {
	f := newAPIFixture(t)
	ws := "/api/v1/workspaces/" + f.workspaceA
	r := f.a.call(t, f.app, "GET", ws+"/jobs/"+f.jobID+"/summary?lang=tr", nil)
	expectStatus(t, r, 200)
	// The fixture job has no description worth summarising, so the line comes
	// from the listing's fields, with no model.
	if r.data()["source"] != "facts" || r.data()["summary"] == nil {
		t.Fatalf("summary = %s", r.Raw)
	}
	expectStatus(t, f.a.call(t, f.app, "GET", ws+"/jobs/"+f.jobID+"/summary?lang=de", nil), 422)
}

func TestAPIWorkspaceIsolation(t *testing.T) {
	f := newAPIFixture(t)

	r := f.a.call(t, f.app, "POST", "/api/v1/workspaces/"+f.workspaceA+"/applications", map[string]string{"job_id": f.jobID})
	expectStatus(t, r, 201)
	appID := r.data()["id"].(string)

	// Cross-user: B uses A's workspace directly.
	ownedByA := "/api/v1/workspaces/" + f.workspaceA + "/applications/" + appID
	// Cross-workspace: B uses its own workspace with A's application ID.
	viaB := "/api/v1/workspaces/" + f.workspaceB + "/applications/" + appID

	for _, base := range []string{ownedByA, viaB} {
		for _, path := range []string{base, base + "/attempts", base + "/events", base + "/notes", base + "/tags"} {
			expectError(t, f.b.call(t, f.app, "GET", path, nil), 404, "not_found")
		}
		expectStatus(t, f.b.call(t, f.app, "PATCH", base, map[string]string{"status": "withdrawn"}), 404)
		expectStatus(t, f.b.call(t, f.app, "POST", base+"/attempts", map[string]string{"applied_at": "2026-10-10T09:00:00Z"}), 404)
		expectStatus(t, f.b.call(t, f.app, "POST", base+"/notes", map[string]string{"body": "x"}), 404)
	}
	for _, path := range []string{"/applications", "/saved-jobs", "/tags", "/members"} {
		expectStatus(t, f.b.call(t, f.app, "GET", "/api/v1/workspaces/"+f.workspaceA+path, nil), 404)
	}

	r = f.b.call(t, f.app, "GET", "/api/v1/workspaces/"+f.workspaceB+"/applications", nil)
	expectStatus(t, r, 200)
	if len(r.list()) != 0 {
		t.Fatalf("workspace B sees %d applications", len(r.list()))
	}

	r = f.b.call(t, f.app, "GET", "/api/v1/workspaces", nil)
	for _, w := range r.list() {
		if w.(map[string]any)["id"] == f.workspaceA {
			t.Fatal("B can list A's workspace")
		}
	}

	expectStatus(t, f.a.call(t, f.app, "GET", "/api/v1/workspaces/00000000-0000-0000-0000-000000000000/applications", nil), 404)

	// A's application still unchanged.
	r = f.a.call(t, f.app, "GET", ownedByA, nil)
	if r.data()["status"] != "planned" {
		t.Fatalf("A's application changed to %v", r.data()["status"])
	}
}
