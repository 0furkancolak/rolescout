package database

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"example.com/rolescout/api/internal/domain"
)

type WorkspaceRepository struct {
	pool *pgxpool.Pool
}

func NewWorkspaceRepository(pool *pgxpool.Pool) *WorkspaceRepository {
	return &WorkspaceRepository{pool: pool}
}

// Create stores a workspace and makes ownerID its owner in one transaction.
func (r *WorkspaceRepository) Create(ctx context.Context, name, ownerID string) (domain.Workspace, error) {
	var w domain.Workspace
	err := inTx(ctx, r.pool, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `
			INSERT INTO workspaces (name) VALUES ($1)
			RETURNING id, name, created_at, updated_at`, name,
		).Scan(&w.ID, &w.Name, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return translate(err, "create workspace")
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO workspace_memberships (workspace_id, user_id, role)
			VALUES ($1, $2, $3)`, w.ID, ownerID, domain.RoleOwner)
		return translate(err, "create owner membership")
	})
	return w, err
}

func (r *WorkspaceRepository) AddMember(ctx context.Context, workspaceID, userID string, role domain.MembershipRole) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO workspace_memberships (workspace_id, user_id, role)
		VALUES ($1, $2, $3)`, workspaceID, userID, role)
	return translate(err, "add workspace member")
}

// Membership returns the user's membership, or ErrNotFound if the user does not
// belong to the workspace.
func (r *WorkspaceRepository) Membership(ctx context.Context, workspaceID, userID string) (domain.WorkspaceMembership, error) {
	var m domain.WorkspaceMembership
	err := r.pool.QueryRow(ctx, `
		SELECT workspace_id, user_id, role, created_at
		FROM workspace_memberships WHERE workspace_id = $1 AND user_id = $2`,
		workspaceID, userID,
	).Scan(&m.WorkspaceID, &m.UserID, &m.Role, &m.CreatedAt)
	return m, translate(err, "get workspace membership")
}

// ListForUser returns the workspaces the user belongs to, oldest first.
func (r *WorkspaceRepository) ListForUser(ctx context.Context, userID string) ([]domain.Workspace, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT w.id, w.name, w.created_at, w.updated_at
		FROM workspaces w
		JOIN workspace_memberships m ON m.workspace_id = w.id
		WHERE m.user_id = $1
		ORDER BY w.created_at, w.id`, userID)
	if err != nil {
		return nil, translate(err, "list workspaces")
	}
	ws, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Workspace, error) {
		var w domain.Workspace
		err := row.Scan(&w.ID, &w.Name, &w.CreatedAt, &w.UpdatedAt)
		return w, err
	})
	return ws, translate(err, "list workspaces")
}
