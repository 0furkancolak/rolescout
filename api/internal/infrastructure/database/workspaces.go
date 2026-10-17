package database

import (
	"context"
	"strings"

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

// Get returns a workspace or ErrNotFound.
func (r *WorkspaceRepository) Get(ctx context.Context, id string) (domain.Workspace, error) {
	var w domain.Workspace
	err := r.pool.QueryRow(ctx, `
		SELECT id, name, created_at, updated_at FROM workspaces WHERE id = $1`, id,
	).Scan(&w.ID, &w.Name, &w.CreatedAt, &w.UpdatedAt)
	return w, translate(err, "get workspace")
}

// ListMemberWorkspaces returns the user's workspaces with their role, oldest first.
func (r *WorkspaceRepository) ListMemberWorkspaces(ctx context.Context, userID string) ([]domain.MemberWorkspace, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT w.id, w.name, w.created_at, w.updated_at, m.role
		FROM workspaces w
		JOIN workspace_memberships m ON m.workspace_id = w.id
		WHERE m.user_id = $1
		ORDER BY w.created_at, w.id`, userID)
	if err != nil {
		return nil, translate(err, "list member workspaces")
	}
	ws, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.MemberWorkspace, error) {
		var mw domain.MemberWorkspace
		err := row.Scan(&mw.Workspace.ID, &mw.Workspace.Name, &mw.Workspace.CreatedAt,
			&mw.Workspace.UpdatedAt, &mw.Role)
		return mw, err
	})
	return ws, translate(err, "list member workspaces")
}

// ListMembers returns a workspace's members, owners first.
func (r *WorkspaceRepository) ListMembers(ctx context.Context, workspaceID string) ([]domain.WorkspaceMember, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+prefixed("u", userColumns)+`, m.role, m.created_at
		FROM workspace_memberships m JOIN users u ON u.id = m.user_id
		WHERE m.workspace_id = $1
		ORDER BY m.role = 'owner' DESC, m.created_at, u.id`, workspaceID)
	if err != nil {
		return nil, translate(err, "list workspace members")
	}
	members, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.WorkspaceMember, error) {
		var m domain.WorkspaceMember
		err := row.Scan(&m.User.ID, &m.User.Email, &m.User.DisplayName, &m.User.Locale,
			&m.User.CreatedAt, &m.User.UpdatedAt, &m.User.IsAdmin, &m.Role, &m.CreatedAt)
		return m, err
	})
	return members, translate(err, "list workspace members")
}

// AddMemberByEmail adds an existing user to a workspace. It returns
// ErrNotFound for an unknown email and ErrConflict if the user is already a member.
func (r *WorkspaceRepository) AddMemberByEmail(ctx context.Context, workspaceID, email string, role domain.MembershipRole) (domain.WorkspaceMember, error) {
	var m domain.WorkspaceMember
	err := r.pool.QueryRow(ctx, `
		WITH target AS (SELECT `+userColumns+` FROM users WHERE email = $2),
		inserted AS (
			INSERT INTO workspace_memberships (workspace_id, user_id, role)
			SELECT $1, id, $3 FROM target
			RETURNING role, created_at
		)
		SELECT `+prefixed("t", userColumns)+`, i.role, i.created_at FROM target t, inserted i`,
		workspaceID, strings.ToLower(strings.TrimSpace(email)), role,
	).Scan(&m.User.ID, &m.User.Email, &m.User.DisplayName, &m.User.Locale,
		&m.User.CreatedAt, &m.User.UpdatedAt, &m.User.IsAdmin, &m.Role, &m.CreatedAt)
	return m, translate(err, "add workspace member")
}
