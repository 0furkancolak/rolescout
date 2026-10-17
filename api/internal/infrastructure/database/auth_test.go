package database

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"example.com/rolescout/api/internal/auth"
	"example.com/rolescout/api/internal/domain"
	"example.com/rolescout/api/internal/infrastructure/database/dbtest"
)

// TestRegisterOnlyFirstUserWhenClosed needs an empty users table, so it uses
// its own database.
func TestRegisterOnlyFirstUserWhenClosed(t *testing.T) {
	requireDB(t)
	ctx := context.Background()
	dbURL, drop, err := dbtest.Create(os.Getenv(dbtest.EnvVar))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(drop)
	pool, err := Open(ctx, dbURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := RunMigrations(ctx, pool, "../../../migrations"); err != nil {
		t.Fatal(err)
	}

	repo := NewAuthRepository(pool)
	hash, err := auth.HashPassword("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	params := func(i int, allow bool) RegisterParams {
		return RegisterParams{
			Email: fmt.Sprintf("user%d@example.com", i), DisplayName: "U", PasswordHash: hash,
			WorkspaceName: "Personal", AllowWhenUsersExist: allow,
		}
	}

	// Concurrent "first" registrations: exactly one may win.
	const n = 6
	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, errs[i] = repo.Register(ctx, params(i, false))
		}()
	}
	wg.Wait()
	succeeded := 0
	for _, err := range errs {
		switch {
		case err == nil:
			succeeded++
		case !errors.Is(err, domain.ErrRegistrationClosed):
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if succeeded != 1 {
		t.Fatalf("%d concurrent first registrations succeeded, want 1", succeeded)
	}

	u, w, err := repo.Register(ctx, params(100, true))
	if err != nil {
		t.Fatalf("registration with AllowWhenUsersExist failed: %v", err)
	}
	m, err := NewWorkspaceRepository(pool).Membership(ctx, w.ID, u.ID)
	if err != nil || m.Role != domain.RoleOwner {
		t.Fatalf("personal workspace membership = %+v, %v", m, err)
	}
}

func TestSessions(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	repo := NewAuthRepository(testPool)

	_, hash, err := auth.NewSessionToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateSession(ctx, hash, f.user.ID, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	u, err := repo.UserBySession(ctx, hash)
	if err != nil || u.ID != f.user.ID {
		t.Fatalf("session user = %+v, %v", u, err)
	}

	if err := repo.DeleteSession(ctx, hash); err != nil {
		t.Fatal(err)
	}
	_, err = repo.UserBySession(ctx, hash)
	requireErr(t, err, domain.ErrNotFound)

	// Expired sessions are rejected and cleaned up on the next login.
	_, expired, _ := auth.NewSessionToken()
	if err := repo.CreateSession(ctx, expired, f.user.ID, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := testPool.Exec(ctx, `
		UPDATE sessions SET created_at = NOW() - INTERVAL '2 hours', expires_at = NOW() - INTERVAL '1 hour'
		WHERE token_hash = $1`, expired); err != nil {
		t.Fatal(err)
	}
	_, err = repo.UserBySession(ctx, expired)
	requireErr(t, err, domain.ErrNotFound)

	_, fresh, _ := auth.NewSessionToken()
	if err := repo.CreateSession(ctx, fresh, f.user.ID, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	var remaining int
	if err := testPool.QueryRow(ctx, `SELECT COUNT(*) FROM sessions WHERE token_hash = $1`, expired).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != 0 {
		t.Fatal("expired session was not cleaned up")
	}
}
