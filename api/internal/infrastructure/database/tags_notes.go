package database

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"example.com/rolescout/api/internal/domain"
)

// AnnotationRepository stores workspace tags and application notes.
type AnnotationRepository struct {
	pool *pgxpool.Pool
}

func NewAnnotationRepository(pool *pgxpool.Pool) *AnnotationRepository {
	return &AnnotationRepository{pool: pool}
}

// CreateTag adds a tag. Names are unique per workspace, ignoring case.
func (r *AnnotationRepository) CreateTag(ctx context.Context, workspaceID, name string) (domain.Tag, error) {
	var t domain.Tag
	err := r.pool.QueryRow(ctx, `
		INSERT INTO tags (workspace_id, name) VALUES ($1, $2)
		RETURNING id, workspace_id, name, created_at`, workspaceID, strings.TrimSpace(name),
	).Scan(&t.ID, &t.WorkspaceID, &t.Name, &t.CreatedAt)
	return t, translate(err, "create tag")
}

// TagApplication attaches a tag. Both must belong to workspaceID; attaching an
// already attached tag is a no-op.
func (r *AnnotationRepository) TagApplication(ctx context.Context, workspaceID, applicationID, tagID string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO application_tags (workspace_id, application_id, tag_id) VALUES ($1, $2, $3)
		ON CONFLICT (application_id, tag_id) DO NOTHING`, workspaceID, applicationID, tagID)
	return translate(err, "tag application")
}

// ListTags returns the workspace's tags ordered by name.
func (r *AnnotationRepository) ListTags(ctx context.Context, workspaceID string) ([]domain.Tag, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, workspace_id, name, created_at FROM tags
		WHERE workspace_id = $1 ORDER BY lower(name)`, workspaceID)
	if err != nil {
		return nil, translate(err, "list tags")
	}
	tags, err := pgx.CollectRows(rows, scanTag)
	return tags, translate(err, "list tags")
}

func scanTag(row pgx.CollectableRow) (domain.Tag, error) {
	var t domain.Tag
	err := row.Scan(&t.ID, &t.WorkspaceID, &t.Name, &t.CreatedAt)
	return t, err
}

// UntagApplication detaches a tag, or returns ErrNotFound if it was not attached.
func (r *AnnotationRepository) UntagApplication(ctx context.Context, workspaceID, applicationID, tagID string) error {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM application_tags
		WHERE workspace_id = $1 AND application_id = $2 AND tag_id = $3`,
		workspaceID, applicationID, tagID)
	if err != nil {
		return translate(err, "untag application")
	}
	if tag.RowsAffected() == 0 {
		return translate(pgx.ErrNoRows, "untag application")
	}
	return nil
}

func (r *AnnotationRepository) ListApplicationTags(ctx context.Context, workspaceID, applicationID string) ([]domain.Tag, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT t.id, t.workspace_id, t.name, t.created_at
		FROM application_tags at JOIN tags t ON t.id = at.tag_id
		WHERE at.workspace_id = $1 AND at.application_id = $2
		ORDER BY lower(t.name)`, workspaceID, applicationID)
	if err != nil {
		return nil, translate(err, "list application tags")
	}
	tags, err := pgx.CollectRows(rows, scanTag)
	return tags, translate(err, "list application tags")
}

func scanNote(row pgx.Row) (domain.Note, error) {
	var n domain.Note
	err := row.Scan(&n.ID, &n.WorkspaceID, &n.ApplicationID, &n.AuthorID, &n.Body, &n.CreatedAt, &n.UpdatedAt)
	return n, err
}

func (r *AnnotationRepository) AddNote(ctx context.Context, workspaceID, applicationID string, authorID *string, body string) (domain.Note, error) {
	n, err := scanNote(r.pool.QueryRow(ctx, `
		INSERT INTO notes (workspace_id, application_id, author_id, body) VALUES ($1, $2, $3, $4)
		RETURNING id, workspace_id, application_id, author_id, body, created_at, updated_at`, workspaceID, applicationID, authorID, body))
	return n, translate(err, "add note")
}

func (r *AnnotationRepository) UpdateNote(ctx context.Context, workspaceID, applicationID, noteID, body string) (domain.Note, error) {
	n, err := scanNote(r.pool.QueryRow(ctx, `
		UPDATE notes SET body = $4
		WHERE workspace_id = $1 AND application_id = $2 AND id = $3
		RETURNING id, workspace_id, application_id, author_id, body, created_at, updated_at`, workspaceID, applicationID, noteID, body))
	return n, translate(err, "update note")
}

func (r *AnnotationRepository) ListNotes(ctx context.Context, workspaceID, applicationID string) ([]domain.Note, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, workspace_id, application_id, author_id, body, created_at, updated_at FROM notes
		WHERE workspace_id = $1 AND application_id = $2
		ORDER BY created_at, id`, workspaceID, applicationID)
	if err != nil {
		return nil, translate(err, "list notes")
	}
	notes, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Note, error) {
		return scanNote(row)
	})
	return notes, translate(err, "list notes")
}
