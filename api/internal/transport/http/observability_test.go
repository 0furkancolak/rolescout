package httptransport

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"

	"example.com/rolescout/api/internal/auth"
	"example.com/rolescout/api/internal/domain"
)

func observedApp(t *testing.T, logs *bytes.Buffer) (*fiber.App, *Metrics) {
	t.Helper()
	logger := slog.New(slog.NewJSONHandler(logs, nil))
	metrics := NewMetrics()
	app := fiber.New(fiber.Config{ErrorHandler: ErrorHandler})
	app.Use(RequestID(), Observe(logger, metrics))
	RegisterHealthRoutes(app, okPinger{})
	RegisterMetricsRoute(app, metrics, nil)
	RegisterAPIRoutes(app, Deps{Auth: fakeAuth{}, AuthConfig: AuthConfig{SessionTTL: time.Hour}, Jobs: &notFoundJobs{}})
	return app, metrics
}

type okPinger struct{}

func (okPinger) Ping(context.Context) error { return nil }

type notFoundJobs struct{ JobStore }

func (*notFoundJobs) GetJob(context.Context, string) (domain.Job, error) {
	return domain.Job{}, domain.ErrNotFound
}

func logLines(t *testing.T, logs *bytes.Buffer) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(logs.String()), "\n") {
		if line == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("log line is not JSON: %q", line)
		}
		out = append(out, m)
	}
	return out
}

func TestRequestsAreLoggedWithIDAndRouteTemplate(t *testing.T) {
	var logs bytes.Buffer
	app, _ := observedApp(t, &logs)

	r := call(t, app, "GET", "/api/v1/jobs/aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", nil)
	expectError(t, r, 404, "not_found")

	lines := logLines(t, &logs)
	if len(lines) != 1 {
		t.Fatalf("want one log line, got %d: %s", len(lines), logs.String())
	}
	l := lines[0]
	if l["msg"] != "request" || l["route"] != "/api/v1/jobs/:jobID" || l["status"] != float64(404) || l["method"] != "GET" {
		t.Fatalf("unexpected log line: %v", l)
	}
	if id, _ := l["request_id"].(string); id == "" {
		t.Fatal("request_id missing from log")
	}
	if strings.Contains(logs.String(), "aaaaaaaa") {
		t.Fatal("logs must use the route template, not IDs")
	}
}

func TestRequestIDHeader(t *testing.T) {
	var logs bytes.Buffer
	app, _ := observedApp(t, &logs)
	send := func(id string) string {
		req := httptest.NewRequest("GET", "/healthz", nil)
		if id != "" {
			req.Header.Set("X-Request-ID", id)
		}
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		return resp.Header.Get("X-Request-ID")
	}
	if got := send(""); len(got) < 16 {
		t.Fatalf("generated request id %q is too short", got)
	}
	if got := send("trace-123"); got != "trace-123" {
		t.Fatalf("valid incoming id must be kept, got %q", got)
	}
	if got := send("bad id\"\n"); got == "bad id\"\n" || got == "" {
		t.Fatalf("invalid incoming id must be replaced, got %q", got)
	}
	if logs.Len() != 0 {
		t.Fatalf("health checks must not be logged: %s", logs.String())
	}
}

func TestMetricsCountRequestsByRoute(t *testing.T) {
	var logs bytes.Buffer
	app, _ := observedApp(t, &logs)
	for range 2 {
		call(t, app, "GET", "/api/v1/jobs/aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", nil)
	}
	anonymous.call(t, app, "GET", "/api/v1/jobs", nil)
	call(t, app, "GET", "/nope", nil)

	r := anonymous.call(t, app, "GET", "/metrics", nil)
	expectStatus(t, r, 200)
	for _, want := range []string{
		`rolescout_http_requests_total{method="GET",route="/api/v1/jobs/:jobID",status="404"} 2`,
		`rolescout_http_requests_total{method="GET",route="unmatched",status="404"} 1`,
		`rolescout_http_request_duration_seconds_count{route="/api/v1/jobs/:jobID"} 2`,
		`rolescout_http_request_duration_seconds_bucket{route="/api/v1/jobs/:jobID",le="+Inf"} 2`,
		`rolescout_http_requests_in_flight`,
	} {
		if !strings.Contains(r.Raw, want) {
			t.Errorf("metrics missing %q\n%s", want, r.Raw)
		}
	}
	if strings.Contains(r.Raw, "/nope") {
		t.Error("unmatched paths must not become labels")
	}
	if !strings.Contains(r.Raw, `status="401"`) {
		t.Error("requests rejected by middleware must be counted")
	}
}

// loginAuth accepts one account with a real password hash.
type loginAuth struct {
	fakeAuth
	hash string
}

func (a loginAuth) Credentials(_ context.Context, email string) (domain.User, string, error) {
	if email == testUser.Email {
		return testUser, a.hash, nil
	}
	return domain.User{}, "", domain.ErrNotFound
}

func (loginAuth) CreateSession(context.Context, []byte, string, time.Time) error { return nil }

func TestLoginRateLimitCountsOnlyFailures(t *testing.T) {
	hash, err := auth.HashPassword("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	app := fiber.New(fiber.Config{ErrorHandler: ErrorHandler})
	RegisterAPIRoutes(app, Deps{
		Auth:       loginAuth{hash: hash},
		AuthConfig: AuthConfig{SessionTTL: time.Hour},
		RateLimits: RateLimits{Login: Limit{Max: 2, Window: time.Minute}},
	})
	good := map[string]string{"email": testUser.Email, "password": "correct horse battery"}
	bad := map[string]string{"email": testUser.Email, "password": ""}

	expectStatus(t, anonymous.call(t, app, "POST", "/api/v1/auth/login", good), 200)
	expectError(t, anonymous.call(t, app, "POST", "/api/v1/auth/login", bad), 401, "invalid_credentials")
	expectStatus(t, anonymous.call(t, app, "POST", "/api/v1/auth/login", good), 200)
	expectError(t, anonymous.call(t, app, "POST", "/api/v1/auth/login", bad), 401, "invalid_credentials")
	// Two failures used the budget; even a correct password is now refused.
	expectError(t, anonymous.call(t, app, "POST", "/api/v1/auth/login", good), 429, "rate_limited")
}

func TestRegisterRateLimitIgnoresSpoofedClientIP(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: ErrorHandler, ProxyHeader: "X-Real-IP"})
	RegisterAPIRoutes(app, Deps{
		Auth:       fakeAuth{},
		AuthConfig: AuthConfig{SessionTTL: time.Hour},
		RateLimits: RateLimits{Register: Limit{Max: 1, Window: time.Minute}},
	})
	send := func(ip string) int {
		req := httptest.NewRequest("POST", "/api/v1/auth/register", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Real-IP", ip)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		return resp.StatusCode
	}
	if got := send("203.0.113.1"); got != 422 {
		t.Fatalf("first request: %d", got)
	}
	// Without a trusted proxy, X-Real-IP must not create a fresh budget.
	if got := send("203.0.113.2"); got != 429 {
		t.Fatalf("spoofed IP bypassed the limit: %d", got)
	}
}
