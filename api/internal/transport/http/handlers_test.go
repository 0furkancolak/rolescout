package httptransport

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"

	"example.com/rolescout/api/internal/domain"
	"example.com/rolescout/api/internal/infrastructure/database"
)

// The fakes embed the interfaces so unimplemented methods panic if a test
// reaches them unexpectedly.

type fakeJobs struct {
	JobStore
	gotFilter database.JobFilter
	jobs      []domain.Job
	hasMore   bool
	err       error
}

func (f *fakeJobs) SearchJobs(_ context.Context, filter database.JobFilter) ([]domain.Job, bool, error) {
	f.gotFilter = filter
	return f.jobs, f.hasMore, f.err
}

type fakeApplications struct {
	ApplicationStore
	changeErr error
}

func (f *fakeApplications) ChangeStatus(_ context.Context, _, _ string, _ domain.ApplicationStatus, _ *string) (domain.Application, error) {
	return domain.Application{}, f.changeErr
}

type authorizer func(workspaceID string) (domain.MembershipRole, error)

func (a authorizer) AuthorizeWorkspace(_ fiber.Ctx, id string) (domain.MembershipRole, error) {
	return a(id)
}

var allowAll = authorizer(func(string) (domain.MembershipRole, error) { return domain.RoleOwner, nil })

const wsPath = "/api/v1/workspaces/11111111-1111-1111-1111-111111111111"

func TestUnknownRouteUsesErrorFormat(t *testing.T) {
	r := call(t, newTestApp(Deps{Workspaces: allowAll}), "GET", "/api/v1/nope", nil)
	expectStatus(t, r, 404)
	if r.errorCode() != "not_found" {
		t.Fatalf("code = %q", r.errorCode())
	}
}

func TestSearchJobsValidation(t *testing.T) {
	jobs := &fakeJobs{}
	app := newTestApp(Deps{Jobs: jobs, Workspaces: allowAll})

	r := call(t, app, "GET", "/api/v1/jobs?limit=0&work_arrangement=moon&cursor=%25%25", nil)
	expectStatus(t, r, 422)
	fields := r.Body["error"].(map[string]any)["fields"].([]any)
	var names []string
	for _, f := range fields {
		names = append(names, f.(map[string]any)["field"].(string))
	}
	if got := strings.Join(names, ","); got != "limit,work_arrangement,cursor" {
		t.Fatalf("invalid fields = %s", got)
	}
}

func TestSearchJobsPassesFiltersAndPaginates(t *testing.T) {
	seen := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	jobs := &fakeJobs{
		jobs:    []domain.Job{{ID: "22222222-2222-2222-2222-222222222222", Title: "Go", LastSeenAt: seen}},
		hasMore: true,
	}
	app := newTestApp(Deps{Jobs: jobs, Workspaces: allowAll})

	r := call(t, app, "GET", "/api/v1/jobs?q=+go+&seniority=senior&technology=Go,%20postgresql,&limit=1", nil)
	expectStatus(t, r, 200)
	f := jobs.gotFilter
	if f.Query != "go" || f.Seniority != "senior" || f.Limit != 1 ||
		strings.Join(f.Technologies, "|") != "Go|postgresql" {
		t.Fatalf("filter = %+v", f)
	}

	next, _ := r.Body["next_cursor"].(string)
	cursor, err := decodeCursor(next)
	if err != nil {
		t.Fatalf("next_cursor %q: %v", next, err)
	}
	if !cursor.LastSeenAt.Equal(seen) || cursor.ID != jobs.jobs[0].ID {
		t.Fatalf("cursor = %+v", cursor)
	}

	r = call(t, app, "GET", "/api/v1/jobs?cursor="+next, nil)
	expectStatus(t, r, 200)
	if jobs.gotFilter.After == nil || jobs.gotFilter.After.ID != cursor.ID {
		t.Fatal("cursor was not passed to the store")
	}
}

func TestMalformedIDsAreNotFound(t *testing.T) {
	app := newTestApp(Deps{Workspaces: allowAll})
	for _, path := range []string{
		"/api/v1/jobs/not-a-uuid",
		"/api/v1/workspaces/123/applications",
		wsPath + "/applications/abc",
	} {
		r := call(t, app, "GET", path, nil)
		expectStatus(t, r, 404)
	}
}

func TestWorkspaceAuthorizerGuardsRoutes(t *testing.T) {
	denied := authorizer(func(string) (domain.MembershipRole, error) { return "", domain.ErrNotFound })
	// Nil stores: reaching a handler would panic.
	app := newTestApp(Deps{Workspaces: denied})
	r := call(t, app, "POST", wsPath+"/applications", map[string]string{"job_id": "x"})
	expectStatus(t, r, 404)
}

func TestRequestBodyErrors(t *testing.T) {
	app := newTestApp(Deps{Workspaces: allowAll})
	tests := []struct {
		name, body, code string
		status           int
	}{
		{"empty", "", "bad_request", 400},
		{"malformed", "{", "bad_request", 400},
		{"unknown field", `{"job_id":"x","extra":1}`, "bad_request", 400},
		{"trailing data", `{"job_id":"x"} {}`, "bad_request", 400},
		{"invalid id", `{"job_id":"x"}`, "invalid_input", 422},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := call(t, app, "POST", wsPath+"/applications", tt.body)
			expectStatus(t, r, tt.status)
			if r.errorCode() != tt.code {
				t.Fatalf("code = %q, want %q", r.errorCode(), tt.code)
			}
		})
	}
}

func TestStoreErrorsMapToStatusCodes(t *testing.T) {
	tests := []struct {
		err    error
		status int
		code   string
	}{
		{domain.ErrTransitionNotAllowed{From: "planned", To: "offer"}, 409, "invalid_transition"},
		{domain.ErrNotFound, 404, "not_found"},
		{domain.ErrConflict, 409, "conflict"},
		{domain.ErrInvalid, 422, "invalid_input"},
		{errors.New("connection refused to 10.0.0.5"), 500, "internal"},
	}
	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			app := newTestApp(Deps{Workspaces: allowAll, Applications: &fakeApplications{changeErr: tt.err}})
			r := call(t, app, "PATCH", wsPath+"/applications/33333333-3333-3333-3333-333333333333",
				map[string]string{"status": "offer"})
			expectStatus(t, r, tt.status)
			if r.errorCode() != tt.code {
				t.Fatalf("code = %q, want %q", r.errorCode(), tt.code)
			}
			if strings.Contains(r.Raw, "10.0.0.5") {
				t.Fatal("internal error details leaked to the client")
			}
		})
	}
}

func TestPrivateRoutesRequireSession(t *testing.T) {
	app := newTestApp(Deps{Workspaces: allowAll})
	for _, tc := range []struct {
		name string
		cl   client
	}{
		{"no cookie", anonymous},
		{"unknown token", client{token: "forged"}},
	} {
		for _, path := range []string{"/api/v1/auth/me", "/api/v1/jobs", "/api/v1/workspaces", wsPath + "/applications", "/api/v1/unknown"} {
			r := tc.cl.call(t, app, "GET", path, nil)
			if r.Status != 401 || r.errorCode() != "unauthenticated" {
				t.Errorf("%s GET %s = %d %s, want 401 unauthenticated", tc.name, path, r.Status, r.errorCode())
			}
		}
	}
}

func TestMembersCannotManageMembers(t *testing.T) {
	member := authorizer(func(string) (domain.MembershipRole, error) { return domain.RoleMember, nil })
	app := newTestApp(Deps{Workspaces: member})
	r := call(t, app, "POST", wsPath+"/members", map[string]string{"email": "x@example.com"})
	expectError(t, r, 403, "forbidden")
}

func TestRequestBodiesMustBeJSON(t *testing.T) {
	app := newTestApp(Deps{Workspaces: allowAll})
	for _, ct := range []string{"text/plain", "application/x-www-form-urlencoded", ""} {
		req := httptest.NewRequest("POST", wsPath+"/tags", strings.NewReader(`{"name":"go"}`))
		if ct != "" {
			req.Header.Set("Content-Type", ct)
		}
		req.AddCookie(&http.Cookie{Name: sessionCookie, Value: testToken})
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != fiber.StatusUnsupportedMediaType {
			t.Errorf("Content-Type %q: status %d, want 415", ct, resp.StatusCode)
		}
	}
}
