package httptransport

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"example.com/rolescout/api/internal/auth"
)

func TestAuthRegisterSetsSafeSessionCookie(t *testing.T) {
	app := newIntegrationApp(t, AuthConfig{AllowRegistration: true, CookieSecure: true})
	email := uniqueEmail("cookie")

	r := anonymous.call(t, app, "POST", "/api/v1/auth/register", map[string]string{
		"email": strings.ToUpper(email), "password": "correct horse battery", "display_name": "Cookie", "locale": "en",
	})
	expectStatus(t, r, 201)
	if r.data()["email"] != email {
		t.Fatalf("email = %v, want normalized %s", r.data()["email"], email)
	}
	if _, leaked := r.data()["password_hash"]; leaked {
		t.Fatal("password hash returned to client")
	}

	c := r.sessionCookie()
	if c == nil {
		t.Fatal("no session cookie")
	}
	if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode || c.Path != "/" || c.MaxAge != 3600 {
		t.Fatalf("unsafe cookie attributes: %+v", c)
	}

	// Only the hash of the token is stored.
	var rawStored, hashStored bool
	err := testPool.QueryRow(context.Background(), `
		SELECT EXISTS (SELECT 1 FROM sessions WHERE token_hash = $1),
		       EXISTS (SELECT 1 FROM sessions WHERE token_hash = $2)`,
		[]byte(c.Value), auth.HashSessionToken(c.Value)).Scan(&rawStored, &hashStored)
	if err != nil {
		t.Fatal(err)
	}
	if rawStored || !hashStored {
		t.Fatalf("raw token stored=%v, hash stored=%v", rawStored, hashStored)
	}

	r = client{token: c.Value}.call(t, app, "GET", "/api/v1/workspaces", nil)
	if len(r.list()) != 1 || r.list()[0].(map[string]any)["name"] != "Personal" {
		t.Fatalf("personal workspace = %s", r.Raw)
	}
}

func TestAuthRegisterValidation(t *testing.T) {
	app := newIntegrationApp(t, AuthConfig{AllowRegistration: true})
	email := uniqueEmail("dup")
	registerUser(t, app, email)

	tests := []struct {
		name   string
		body   map[string]string
		status int
		code   string
	}{
		{"short password", map[string]string{"email": uniqueEmail("x"), "password": "short", "display_name": "X"}, 422, "invalid_input"},
		{"bad email", map[string]string{"email": "not-an-email", "password": "correct horse battery", "display_name": "X"}, 422, "invalid_input"},
		{"display name with address", map[string]string{"email": "X <x@example.com>", "password": "correct horse battery", "display_name": "X"}, 422, "invalid_input"},
		{"bad locale", map[string]string{"email": uniqueEmail("x"), "password": "correct horse battery", "display_name": "X", "locale": "de"}, 422, "invalid_input"},
		{"duplicate email", map[string]string{"email": strings.ToUpper(email), "password": "correct horse battery", "display_name": "X"}, 409, "conflict"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := anonymous.call(t, app, "POST", "/api/v1/auth/register", tt.body)
			expectError(t, r, tt.status, tt.code)
			if r.sessionCookie() != nil {
				t.Fatal("failed registration set a session cookie")
			}
		})
	}
}

func TestAuthRegistrationClosedAfterFirstUser(t *testing.T) {
	app := newIntegrationApp(t, AuthConfig{AllowRegistration: false})
	// Other tests have created users, so this database is not empty.
	r := anonymous.call(t, app, "POST", "/api/v1/auth/register", map[string]string{
		"email": uniqueEmail("closed"), "password": "correct horse battery", "display_name": "Closed",
	})
	expectError(t, r, 403, "registration_closed")
}

func TestAuthLoginLogoutAndSessions(t *testing.T) {
	app := newIntegrationApp(t, AuthConfig{AllowRegistration: true})
	email := uniqueEmail("login")
	registered, userID, _ := registerUser(t, app, email)

	wrong := anonymous.call(t, app, "POST", "/api/v1/auth/login", map[string]string{"email": email, "password": "wrong password!!"})
	unknown := anonymous.call(t, app, "POST", "/api/v1/auth/login", map[string]string{"email": uniqueEmail("nobody"), "password": "correct horse battery"})
	for _, r := range []testResponse{wrong, unknown} {
		expectError(t, r, 401, "invalid_credentials")
		if r.sessionCookie() != nil {
			t.Fatal("failed login set a session cookie")
		}
	}
	if wrong.Raw != unknown.Raw {
		t.Fatalf("login errors differ for known and unknown emails: %s vs %s", wrong.Raw, unknown.Raw)
	}

	r := anonymous.call(t, app, "POST", "/api/v1/auth/login", map[string]string{"email": strings.ToUpper(email), "password": "correct horse battery"})
	expectStatus(t, r, 200)
	loggedIn := client{token: r.sessionCookie().Value}
	if loggedIn.token == registered.token {
		t.Fatal("login reused the existing session token")
	}

	me := loggedIn.call(t, app, "GET", "/api/v1/auth/me", nil)
	expectStatus(t, me, 200)
	if me.data()["id"] != userID {
		t.Fatalf("me = %s", me.Raw)
	}

	// Logging out revokes only that session.
	out := loggedIn.call(t, app, "POST", "/api/v1/auth/logout", nil)
	expectStatus(t, out, 204)
	if c := out.sessionCookie(); c == nil || c.Value != "" || c.MaxAge >= 0 {
		t.Fatalf("logout did not clear the cookie: %+v", c)
	}
	expectError(t, loggedIn.call(t, app, "GET", "/api/v1/auth/me", nil), 401, "unauthenticated")
	expectStatus(t, registered.call(t, app, "GET", "/api/v1/auth/me", nil), 200)

	expectStatus(t, anonymous.call(t, app, "POST", "/api/v1/auth/logout", nil), 204)
}

func TestAuthExpiredSessionIsRejected(t *testing.T) {
	app := newIntegrationApp(t, AuthConfig{AllowRegistration: true})
	cl, _, _ := registerUser(t, app, uniqueEmail("expiry"))
	expectStatus(t, cl.call(t, app, "GET", "/api/v1/auth/me", nil), 200)

	if _, err := testPool.Exec(context.Background(), `
		UPDATE sessions SET created_at = NOW() - INTERVAL '2 hours', expires_at = NOW() - INTERVAL '1 second'
		WHERE token_hash = $1`, auth.HashSessionToken(cl.token)); err != nil {
		t.Fatal(err)
	}
	expectError(t, cl.call(t, app, "GET", "/api/v1/auth/me", nil), 401, "unauthenticated")
}

func TestWorkspaceMembershipRules(t *testing.T) {
	f := newAPIFixture(t)
	wsA := "/api/v1/workspaces/" + f.workspaceA
	c, _, _ := registerUser(t, f.app, uniqueEmail("c"))

	// B is not a member yet.
	expectStatus(t, f.b.call(t, f.app, "GET", wsA+"/applications", nil), 404)

	r := f.a.call(t, f.app, "GET", "/api/v1/workspaces/"+f.workspaceB+"/members", nil)
	expectStatus(t, r, 404)

	// The owner adds B by email.
	var bEmail string
	me := f.b.call(t, f.app, "GET", "/api/v1/auth/me", nil)
	bEmail = me.data()["email"].(string)
	r = f.a.call(t, f.app, "POST", wsA+"/members", map[string]string{"email": bEmail})
	expectStatus(t, r, 201)
	if r.data()["role"] != "member" {
		t.Fatalf("default role = %v", r.data()["role"])
	}
	expectError(t, f.a.call(t, f.app, "POST", wsA+"/members", map[string]string{"email": bEmail}), 409, "conflict")
	expectError(t, f.a.call(t, f.app, "POST", wsA+"/members", map[string]string{"email": uniqueEmail("ghost")}), 404, "not_found")
	expectError(t, f.a.call(t, f.app, "POST", wsA+"/members", map[string]string{"email": bEmail, "role": "admin"}), 422, "invalid_input")

	// B can now work in A's workspace, and authorship is recorded.
	r = f.b.call(t, f.app, "POST", wsA+"/applications", map[string]string{"job_id": f.jobID})
	expectStatus(t, r, 201)
	if r.data()["created_by"] != f.userB {
		t.Fatalf("created_by = %v, want %s", r.data()["created_by"], f.userB)
	}
	appPath := wsA + "/applications/" + r.data()["id"].(string)
	r = f.a.call(t, f.app, "POST", appPath+"/notes", map[string]string{"body": "Owner note"})
	expectStatus(t, r, 201)
	if r.data()["author_id"] != f.userA {
		t.Fatalf("author_id = %v, want %s", r.data()["author_id"], f.userA)
	}

	// Members cannot manage members; non-members cannot see them.
	expectError(t, f.b.call(t, f.app, "POST", wsA+"/members", map[string]string{"email": bEmail}), 403, "forbidden")
	expectStatus(t, f.b.call(t, f.app, "GET", wsA+"/members", nil), 200)
	expectStatus(t, c.call(t, f.app, "GET", wsA+"/members", nil), 404)
	expectStatus(t, c.call(t, f.app, "POST", wsA+"/members", map[string]string{"email": bEmail}), 404)

	r = f.a.call(t, f.app, "GET", wsA+"/members", nil)
	if len(r.list()) != 2 || r.list()[0].(map[string]any)["role"] != "owner" {
		t.Fatalf("members = %s", r.Raw)
	}
}
