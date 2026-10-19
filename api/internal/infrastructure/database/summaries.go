package database

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SummaryRepository stores model-written job summaries.
type SummaryRepository struct{ pool *pgxpool.Pool }

func NewSummaryRepository(pool *pgxpool.Pool) *SummaryRepository {
	return &SummaryRepository{pool: pool}
}

// Get returns the stored summary when it was written from the same text with
// the same prompt version.
func (r *SummaryRepository) Get(ctx context.Context, jobID, language, hash string, promptVersion int) (string, bool, error) {
	var s string
	err := r.pool.QueryRow(ctx, `SELECT summary FROM job_summaries
		WHERE job_id = $1 AND language = $2 AND description_hash = $3 AND prompt_version = $4`,
		jobID, language, hash, promptVersion).Scan(&s)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	return s, err == nil, translate(err, "get summary")
}

func (r *SummaryRepository) Put(ctx context.Context, jobID, language, hash string, promptVersion int, model, summary string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO job_summaries (job_id, language, description_hash, prompt_version, model, summary)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (job_id, language) DO UPDATE SET description_hash = EXCLUDED.description_hash,
			prompt_version = EXCLUDED.prompt_version, model = EXCLUDED.model, summary = EXCLUDED.summary, created_at = NOW()`,
		jobID, language, hash, promptVersion, model, summary)
	return translate(err, "put summary")
}
