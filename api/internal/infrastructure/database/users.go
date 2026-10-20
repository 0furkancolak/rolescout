package database

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"example.com/rolescout/api/internal/domain"
)

type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

func scanUser(row pgx.Row) (domain.User, error) {
	var u domain.User
	err := row.Scan(&u.ID, &u.Email, &u.DisplayName, &u.Locale, &u.CreatedAt, &u.UpdatedAt, &u.IsAdmin)
	return u, err
}

type CreateUserParams struct {
	Email       string
	DisplayName string
	Locale      string
}

// Create stores a user. Emails are normalized to lower case.
func (r *UserRepository) Create(ctx context.Context, p CreateUserParams) (domain.User, error) {
	locale := p.Locale
	if locale == "" {
		locale = "tr"
	}
	u, err := scanUser(r.pool.QueryRow(ctx, `
		INSERT INTO users (email, display_name, locale)
		VALUES ($1, $2, $3)
		RETURNING id, email, display_name, locale, created_at, updated_at, is_admin`,
		strings.ToLower(strings.TrimSpace(p.Email)), p.DisplayName, locale,
	))
	return u, translate(err, "create user")
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (domain.User, error) {
	u, err := scanUser(r.pool.QueryRow(ctx,
		`SELECT id, email, display_name, locale, created_at, updated_at, is_admin FROM users WHERE id = $1`, id))
	return u, translate(err, "get user")
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (domain.User, error) {
	u, err := scanUser(r.pool.QueryRow(ctx,
		`SELECT id, email, display_name, locale, created_at, updated_at, is_admin FROM users WHERE email = $1`,
		strings.ToLower(strings.TrimSpace(email))))
	return u, translate(err, "get user by email")
}
