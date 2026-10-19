package database

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"example.com/rolescout/api/internal/domain"
)

// UserStateRepository keeps small JSON documents per user and workspace.
type UserStateRepository struct{ pool *pgxpool.Pool }

func NewUserStateRepository(pool *pgxpool.Pool) *UserStateRepository {
	return &UserStateRepository{pool: pool}
}

// Get returns the stored document, or domain.ErrNotFound.
func (r *UserStateRepository) Get(ctx context.Context, userID, workspaceID, key string) (json.RawMessage, error) {
	var v []byte
	err := r.pool.QueryRow(ctx, `SELECT value FROM user_state WHERE user_id = $1 AND workspace_id = $2 AND key = $3`,
		userID, workspaceID, key).Scan(&v)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	return v, translate(err, "get user state")
}

// Put stores the document, replacing any earlier one.
func (r *UserStateRepository) Put(ctx context.Context, userID, workspaceID, key string, value json.RawMessage) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO user_state (user_id, workspace_id, key, value) VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, workspace_id, key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()`,
		userID, workspaceID, key, []byte(value))
	return translate(err, "put user state")
}
