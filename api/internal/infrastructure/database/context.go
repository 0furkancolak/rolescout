package database

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"example.com/rolescout/api/internal/domain"
)

// ContextRepository stores what the AI knows about a workspace's person.
type ContextRepository struct{ pool *pgxpool.Pool }

func NewContextRepository(pool *pgxpool.Pool) *ContextRepository {
	return &ContextRepository{pool: pool}
}

// CVParams is a CV as it is stored.
type CVParams struct {
	Filename  string
	Text      string
	Skills    []string
	Titles    []string
	Seniority string
	Years     *int
	Summary   string
}

// Get returns the workspace's context; a workspace that has none yet gets an
// empty one.
func (r *ContextRepository) Get(ctx context.Context, workspaceID string) (domain.WorkspaceContext, error) {
	c := domain.WorkspaceContext{WorkspaceID: workspaceID, CVSkills: []string{}, CVTitles: []string{}}
	err := r.pool.QueryRow(ctx, `
		SELECT about, cv_filename, cv_text, cv_skills, cv_titles, cv_seniority, cv_years, cv_summary, cv_uploaded_at, updated_at
		FROM workspace_context WHERE workspace_id = $1`, workspaceID).
		Scan(&c.About, &c.CVFilename, &c.CVText, &c.CVSkills, &c.CVTitles, &c.CVSeniority, &c.CVYears, &c.CVSummary, &c.CVUploadedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return c, nil
	}
	return c, translate(err, "get workspace context")
}

// SaveAbout stores the person's note.
func (r *ContextRepository) SaveAbout(ctx context.Context, workspaceID, about string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO workspace_context (workspace_id, about) VALUES ($1, $2)
		ON CONFLICT (workspace_id) DO UPDATE SET about = EXCLUDED.about`, workspaceID, about)
	if err != nil {
		return translate(err, "save workspace note")
	}
	return r.forgetExplanations(ctx, workspaceID)
}

// forgetExplanations drops the stored match explanations of the workspace: they
// were written from the previous note or CV and are written again from the new one.
func (r *ContextRepository) forgetExplanations(ctx context.Context, workspaceID string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM match_explanations WHERE workspace_id = $1`, workspaceID)
	return translate(err, "reset match explanations")
}

// SaveCV stores the CV, replacing the previous one.
func (r *ContextRepository) SaveCV(ctx context.Context, workspaceID string, p CVParams, at time.Time) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO workspace_context (workspace_id, cv_filename, cv_text, cv_skills, cv_titles, cv_seniority, cv_years, cv_summary, cv_uploaded_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (workspace_id) DO UPDATE SET cv_filename = EXCLUDED.cv_filename, cv_text = EXCLUDED.cv_text,
			cv_skills = EXCLUDED.cv_skills, cv_titles = EXCLUDED.cv_titles, cv_seniority = EXCLUDED.cv_seniority,
			cv_years = EXCLUDED.cv_years, cv_summary = EXCLUDED.cv_summary, cv_uploaded_at = EXCLUDED.cv_uploaded_at`,
		workspaceID, p.Filename, p.Text, nonNilStrings(p.Skills), nonNilStrings(p.Titles), p.Seniority, p.Years, p.Summary, at)
	if err != nil {
		return translate(err, "save cv")
	}
	return r.forgetExplanations(ctx, workspaceID)
}

// DeleteCV removes the CV and what was read from it; the note stays.
func (r *ContextRepository) DeleteCV(ctx context.Context, workspaceID string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE workspace_context SET cv_filename = '', cv_text = '', cv_skills = '{}', cv_titles = '{}',
			cv_seniority = '', cv_years = NULL, cv_summary = '', cv_uploaded_at = NULL
		WHERE workspace_id = $1`, workspaceID)
	if err != nil {
		return translate(err, "delete cv")
	}
	return r.forgetExplanations(ctx, workspaceID)
}

func nonNilStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
