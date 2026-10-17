package httptransport

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"

	"example.com/rolescout/api/internal/auth"
	"example.com/rolescout/api/internal/domain"
	"example.com/rolescout/api/internal/infrastructure/database"
)

const testToken = "unit-test-session"

var testUser = domain.User{ID: "99999999-9999-9999-9999-999999999999", Email: "unit@example.com", DisplayName: "Unit"}

// fakeAuth accepts only testToken.
type fakeAuth struct{ AuthStore }

func (fakeAuth) UserBySession(_ context.Context, hash []byte) (domain.User, error) {
	if bytes.Equal(hash, auth.HashSessionToken(testToken)) {
		return testUser, nil
	}
	return domain.User{}, domain.ErrNotFound
}

// newTestApp builds the API with fakes filled in for missing auth dependencies.
func newTestApp(d Deps) *fiber.App {
	if d.Auth == nil {
		d.Auth = fakeAuth{}
	}
	if d.AuthConfig.SessionTTL == 0 {
		d.AuthConfig.SessionTTL = time.Hour
	}
	app := fiber.New(fiber.Config{ErrorHandler: ErrorHandler})
	RegisterAPIRoutes(app, d)
	return app
}

type testResponse struct {
	Status  int
	Body    map[string]any
	Raw     string
	Cookies []*http.Cookie
}

func (r testResponse) errorCode() string {
	e, _ := r.Body["error"].(map[string]any)
	code, _ := e["code"].(string)
	return code
}

func (r testResponse) data() map[string]any {
	d, _ := r.Body["data"].(map[string]any)
	return d
}

func (r testResponse) list() []any {
	d, _ := r.Body["data"].([]any)
	return d
}

func (r testResponse) sessionCookie() *http.Cookie {
	for _, c := range r.Cookies {
		if c.Name == sessionCookie {
			return c
		}
	}
	return nil
}

// client sends requests with a session cookie; an empty token sends none.
type client struct{ token string }

var anonymous = client{}

// call sends a request as the unit-test user.
func call(t *testing.T, app *fiber.App, method, path string, body any) testResponse {
	t.Helper()
	return client{token: testToken}.call(t, app, method, path, body)
}

// call sends a request; body may be nil, a string (sent verbatim) or a value
// encoded as JSON.
func (cl client) call(t *testing.T, app *fiber.App, method, path string, body any) testResponse {
	t.Helper()
	var reader io.Reader
	switch b := body.(type) {
	case nil:
	case string:
		reader = bytes.NewBufferString(b)
	default:
		raw, err := json.Marshal(b)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, reader)
	if reader != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if cl.token != "" {
		req.AddCookie(&http.Cookie{Name: sessionCookie, Value: cl.token})
	}
	// Password hashing is slow under -race; the default 1s timeout is too short.
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 30 * time.Second, FailOnTimeout: true})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	out := testResponse{Status: resp.StatusCode, Raw: string(raw), Cookies: resp.Cookies()}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &out.Body)
	}
	return out
}

func expectStatus(t *testing.T, r testResponse, want int) {
	t.Helper()
	if r.Status != want {
		t.Fatalf("status = %d, want %d; body: %s", r.Status, want, r.Raw)
	}
}

func expectError(t *testing.T, r testResponse, status int, code string) {
	t.Helper()
	expectStatus(t, r, status)
	if r.errorCode() != code {
		t.Fatalf("error code = %q, want %q; body: %s", r.errorCode(), code, r.Raw)
	}
}

// fakeAuthAs accepts testToken as the given user.
type fakeAuthAs domain.User

func (f fakeAuthAs) Register(context.Context, database.RegisterParams) (domain.User, domain.Workspace, error) {
	panic("unexpected")
}
func (f fakeAuthAs) Credentials(context.Context, string) (domain.User, string, error) {
	panic("unexpected")
}
func (f fakeAuthAs) CreateSession(context.Context, []byte, string, time.Time) error {
	panic("unexpected")
}
func (f fakeAuthAs) DeleteSession(context.Context, []byte) error { panic("unexpected") }
func (f fakeAuthAs) UserBySession(_ context.Context, hash []byte) (domain.User, error) {
	if bytes.Equal(hash, auth.HashSessionToken(testToken)) {
		return domain.User(f), nil
	}
	return domain.User{}, domain.ErrNotFound
}
