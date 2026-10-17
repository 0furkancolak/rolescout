package database

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"example.com/rolescout/api/internal/domain"
)

// registrationLockID serializes registrations so the "first user may always
// register" rule cannot be raced.
const registrationLockID int64 = 7_202_610_171_400

type AuthRepository struct {
	pool *pgxpool.Pool
}

func NewAuthRepository(pool *pgxpool.Pool) *AuthRepository {
	return &AuthRepository{pool: pool}
}

type RegisterParams struct {
	Email         string
	DisplayName   string
	Locale        string
	PasswordHash  string
	WorkspaceName string
	// AllowWhenUsersExist permits registration after the first account.
	AllowWhenUsersExist bool
}

// Register creates a user with a password and a personal workspace they own.
// It returns domain.ErrRegistrationClosed when users already exist and
// AllowWhenUsersExist is false.
func (r *AuthRepository) Register(ctx context.Context, p RegisterParams) (domain.User, domain.Workspace, error) {
	locale := p.Locale
	if locale == "" {
		locale = "tr"
	}
	var u domain.User
	var w domain.Workspace
	err := inTx(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, registrationLockID); err != nil {
			return fmt.Errorf("acquire registration lock: %w", err)
		}
		if !p.AllowWhenUsersExist {
			var exists bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users)`).Scan(&exists); err != nil {
				return translate(err, "check existing users")
			}
			if exists {
				return fmt.Errorf("register: %w", domain.ErrRegistrationClosed)
			}
		}

		var err error
		u, err = scanUser(tx.QueryRow(ctx, `
			INSERT INTO users (email, display_name, locale, is_admin)
			VALUES ($1, $2, $3, NOT EXISTS (SELECT 1 FROM users))
			RETURNING `+userColumns,
			strings.ToLower(strings.TrimSpace(p.Email)), p.DisplayName, locale))
		if err != nil {
			return translate(err, "create user")
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO user_credentials (user_id, password_hash) VALUES ($1, $2)`,
			u.ID, p.PasswordHash); err != nil {
			return translate(err, "store credentials")
		}
		if err := tx.QueryRow(ctx, `
			INSERT INTO workspaces (name) VALUES ($1)
			RETURNING id, name, created_at, updated_at`, p.WorkspaceName,
		).Scan(&w.ID, &w.Name, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return translate(err, "create personal workspace")
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO workspace_memberships (workspace_id, user_id, role) VALUES ($1, $2, $3)`,
			w.ID, u.ID, domain.RoleOwner)
		return translate(err, "create owner membership")
	})
	return u, w, err
}

// Credentials returns the user and password hash for an email, or ErrNotFound.
func (r *AuthRepository) Credentials(ctx context.Context, email string) (domain.User, string, error) {
	var u domain.User
	var hash string
	err := r.pool.QueryRow(ctx, `
		SELECT u.id, u.email, u.display_name, u.locale, u.created_at, u.updated_at, u.is_admin, c.password_hash
		FROM users u JOIN user_credentials c ON c.user_id = u.id
		WHERE u.email = $1`, strings.ToLower(strings.TrimSpace(email)),
	).Scan(&u.ID, &u.Email, &u.DisplayName, &u.Locale, &u.CreatedAt, &u.UpdatedAt, &u.IsAdmin, &hash)
	return u, hash, translate(err, "get credentials")
}

// CreateSession stores a session and removes the user's expired sessions.
func (r *AuthRepository) CreateSession(ctx context.Context, tokenHash []byte, userID string, expiresAt time.Time) error {
	return inTx(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx,
			`DELETE FROM sessions WHERE user_id = $1 AND expires_at <= NOW()`, userID); err != nil {
			return translate(err, "delete expired sessions")
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO sessions (token_hash, user_id, expires_at) VALUES ($1, $2, $3)`,
			tokenHash, userID, expiresAt.UTC())
		return translate(err, "create session")
	})
}

// UserBySession returns the user of an unexpired session, or ErrNotFound.
func (r *AuthRepository) UserBySession(ctx context.Context, tokenHash []byte) (domain.User, error) {
	u, err := scanUser(r.pool.QueryRow(ctx, `
		SELECT `+prefixed("u", userColumns)+`
		FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = $1 AND s.expires_at > NOW()`, tokenHash))
	return u, translate(err, "get session user")
}

// DeleteSession revokes a session. Deleting an unknown session is a no-op.
func (r *AuthRepository) DeleteSession(ctx context.Context, tokenHash []byte) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM sessions WHERE token_hash = $1`, tokenHash)
	return translate(err, "delete session")
}
