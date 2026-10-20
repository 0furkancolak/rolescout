package httptransport

import (
	"context"
	"testing"
	"time"

	"example.com/rolescout/api/internal/domain"
	"example.com/rolescout/api/internal/infrastructure/database"
)

// stubUsers records what the admin endpoint passes to the store.
type stubUsers struct {
	fakeAuthAs
	has    bool
	got    database.RegisterParams
	listed []domain.User
}

func (s *stubUsers) HasUsers(context.Context) (bool, error) { return s.has, nil }
func (s *stubUsers) ListUsers(context.Context) ([]domain.User, error) {
	return s.listed, nil
}
func (s *stubUsers) Register(_ context.Context, p database.RegisterParams) (domain.User, domain.Workspace, error) {
	s.got = p
	return domain.User{ID: "u2", Email: p.Email, DisplayName: p.DisplayName, Locale: p.Locale, CreatedAt: time.Now()}, domain.Workspace{}, nil
}

func TestSetupStatusFollowsWhetherAnAccountExists(t *testing.T) {
	for _, tc := range []struct {
		has, allow, open bool
	}{{false, false, true}, {true, false, false}, {true, true, true}} {
		app := newTestApp(Deps{Auth: &stubUsers{has: tc.has}, AuthConfig: AuthConfig{AllowRegistration: tc.allow}})
		r := anonymous.call(t, app, "GET", "/api/v1/auth/setup", nil)
		expectStatus(t, r, 200)
		if got := r.data()["registration_open"]; got != tc.open {
			t.Errorf("has=%v allow=%v: registration_open = %v, want %v", tc.has, tc.allow, got, tc.open)
		}
	}
}

func TestAdminAddsAccountsAndMembersCannot(t *testing.T) {
	body := map[string]string{"email": "Grace@Example.com", "password": "correct horse battery", "display_name": "Grace", "locale": "en"}

	store := &stubUsers{fakeAuthAs: fakeAuthAs(domain.User{ID: "a", Email: "a@example.com", IsAdmin: true})}
	app := newTestApp(Deps{Auth: store, AuthConfig: AuthConfig{}})
	r := client{token: testToken}.call(t, app, "POST", "/api/v1/admin/users", body)
	expectStatus(t, r, 201)
	if !store.got.AllowWhenUsersExist || store.got.Email != "grace@example.com" || store.got.WorkspaceName != "Personal" {
		t.Fatalf("store got %+v", store.got)
	}
	if r.sessionCookie() != nil {
		t.Error("adding an account must not sign the new user in")
	}
	expectStatus(t, client{token: testToken}.call(t, app, "GET", "/api/v1/admin/users", nil), 200)

	member := &stubUsers{fakeAuthAs: fakeAuthAs(domain.User{ID: "m", Email: "m@example.com"})}
	app = newTestApp(Deps{Auth: member, AuthConfig: AuthConfig{}})
	expectStatus(t, client{token: testToken}.call(t, app, "POST", "/api/v1/admin/users", body), 403)
	expectStatus(t, client{token: testToken}.call(t, app, "GET", "/api/v1/admin/users", nil), 403)
	if member.got.Email != "" {
		t.Error("a member created an account")
	}
}

func TestAdminUserValidation(t *testing.T) {
	store := &stubUsers{fakeAuthAs: fakeAuthAs(domain.User{ID: "a", IsAdmin: true})}
	app := newTestApp(Deps{Auth: store})
	r := client{token: testToken}.call(t, app, "POST", "/api/v1/admin/users", map[string]string{"email": "nope", "password": "short", "display_name": ""})
	expectStatus(t, r, 422)
}
