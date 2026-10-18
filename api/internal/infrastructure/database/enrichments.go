package database

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"example.com/rolescout/api/internal/domain"
)

// EnrichmentRepository is the queue and cache of the optional local model.
type EnrichmentRepository struct {
	pool *pgxpool.Pool
}

func NewEnrichmentRepository(pool *pgxpool.Pool) *EnrichmentRepository {
	return &EnrichmentRepository{pool: pool}
}

// maxEnrichmentAttempts is how often a failing job is tried before it stays
// failed until the prompt version changes.
const maxEnrichmentAttempts = 5

// Next returns up to limit open jobs that still need the model for this cache
// key (model, prompt version, schema version), most useful first: jobs that
// were never enriched before jobs being redone for a new prompt; within each,
// matched jobs by best score, then saved or applied jobs, then the newest.
// A description that is already enriched under another job id is copied here
// instead of being returned, so the model never reads the same text twice.
func (r *EnrichmentRepository) Next(ctx context.Context, model string, promptVersion, schemaVersion, limit int, now time.Time) ([]domain.EnrichmentTask, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	_, err = tx.Exec(ctx, `
INSERT INTO job_enrichments (job_id, description_hash, model, prompt_version, schema_version, status, data, attempts)
SELECT DISTINCT ON (j.id) j.id, j.description_hash, $1, $2, $3, 'ok', e.data, 1
FROM jobs j
JOIN job_enrichments e ON e.description_hash = j.description_hash AND e.status = 'ok'
    AND e.model = $1 AND e.prompt_version = $2 AND e.schema_version = $3
WHERE j.closed_at IS NULL AND btrim(j.description) <> ''
  AND NOT EXISTS (SELECT 1 FROM job_enrichments x WHERE x.job_id = j.id AND x.description_hash = j.description_hash
      AND x.model = $1 AND x.prompt_version = $2 AND x.schema_version = $3)
ORDER BY j.id, e.created_at DESC
ON CONFLICT (job_id, description_hash, model, prompt_version, schema_version) DO NOTHING`,
		model, promptVersion, schemaVersion)
	if err != nil {
		return nil, err
	}

	rows, err := tx.Query(ctx, `
SELECT j.id, j.title, j.company_name, j.description, j.description_hash, COALESCE(j.location, '')
FROM jobs j
LEFT JOIN LATERAL (SELECT max(score) AS best FROM research_matches m WHERE m.job_id = j.id) m ON TRUE
WHERE j.closed_at IS NULL AND btrim(j.description) <> ''
  AND NOT EXISTS (SELECT 1 FROM job_enrichments e WHERE e.job_id = j.id AND e.description_hash = j.description_hash
      AND e.model = $1 AND e.prompt_version = $2 AND e.schema_version = $3
      AND (e.status = 'ok' OR e.attempts >= $5 OR e.next_attempt_at > $4))
ORDER BY EXISTS (SELECT 1 FROM job_enrichments o WHERE o.job_id = j.id AND o.status = 'ok') ASC,
         (m.best IS NULL) ASC, m.best DESC NULLS LAST,
         (EXISTS (SELECT 1 FROM saved_jobs s WHERE s.job_id = j.id)
          OR EXISTS (SELECT 1 FROM applications a WHERE a.job_id = j.id)) DESC,
         j.first_seen_at DESC, j.id
LIMIT $6`,
		model, promptVersion, schemaVersion, now, maxEnrichmentAttempts, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.EnrichmentTask
	for rows.Next() {
		var t domain.EnrichmentTask
		if err := rows.Scan(&t.JobID, &t.Title, &t.CompanyName, &t.Description, &t.DescriptionHash, &t.Location); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	return out, tx.Commit(ctx)
}

// Save records a result. A failure backs off for 1 h doubling per attempt,
// capped at 24 h; a success is never overwritten by a later failure.
func (r *EnrichmentRepository) Save(ctx context.Context, res domain.EnrichmentResult) error {
	status := "failed"
	var data any
	if res.OK {
		status = "ok"
		data = []byte(res.Data)
	}
	errText := res.Error
	if len(errText) > 200 {
		errText = errText[:200]
	}
	_, err := r.pool.Exec(ctx, `
INSERT INTO job_enrichments (job_id, description_hash, model, prompt_version, schema_version, status, data, error,
    prompt_tokens, completion_tokens, latency_ms, attempts, next_attempt_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8, ''), $9, $10, $11, 1, CASE WHEN $6 = 'failed' THEN $12::timestamptz + INTERVAL '1 hour' END)
ON CONFLICT (job_id, description_hash, model, prompt_version, schema_version) DO UPDATE SET
    status = EXCLUDED.status, data = EXCLUDED.data, error = EXCLUDED.error,
    prompt_tokens = EXCLUDED.prompt_tokens, completion_tokens = EXCLUDED.completion_tokens, latency_ms = EXCLUDED.latency_ms,
    attempts = job_enrichments.attempts + 1, updated_at = NOW(),
    next_attempt_at = CASE WHEN EXCLUDED.status = 'failed'
        THEN $12::timestamptz + LEAST(INTERVAL '24 hours', INTERVAL '1 hour' * power(2, job_enrichments.attempts)) END
WHERE job_enrichments.status <> 'ok'`,
		res.JobID, res.DescriptionHash, res.Model, res.PromptVersion, res.SchemaVersion, status, data, errText,
		res.PromptTokens, res.CompletionTokens, int(res.Latency/time.Millisecond), res.Now)
	return err
}

// Latest returns the newest successful enrichment of each job that still
// matches the job's current description.
func (r *EnrichmentRepository) Latest(ctx context.Context, jobIDs []string) (map[string]domain.Enrichment, error) {
	out := map[string]domain.Enrichment{}
	if len(jobIDs) == 0 {
		return out, nil
	}
	rows, err := r.pool.Query(ctx, `
SELECT DISTINCT ON (e.job_id) e.job_id, e.model, e.data, e.created_at
FROM job_enrichments e
JOIN jobs j ON j.id = e.job_id AND j.description_hash = e.description_hash
WHERE e.job_id = ANY($1) AND e.status = 'ok'
ORDER BY e.job_id, e.created_at DESC`, jobIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var en domain.Enrichment
		var data []byte
		if err := rows.Scan(&en.JobID, &en.Model, &data, &en.CreatedAt); err != nil {
			return nil, err
		}
		en.Data = json.RawMessage(data)
		out[en.JobID] = en
	}
	return out, rows.Err()
}

// Stats summarises the queue and recent behaviour for one model.
func (r *EnrichmentRepository) Stats(ctx context.Context, model string) (domain.EnrichmentStats, error) {
	var s domain.EnrichmentStats
	err := r.pool.QueryRow(ctx, `
SELECT
  (SELECT count(*) FROM jobs j WHERE j.closed_at IS NULL AND btrim(j.description) <> ''
     AND NOT EXISTS (SELECT 1 FROM job_enrichments e WHERE e.job_id = j.id AND e.description_hash = j.description_hash
                     AND e.model = $1 AND e.status = 'ok')),
  (SELECT count(*) FROM job_enrichments WHERE model = $1 AND status = 'ok'),
  (SELECT count(*) FROM job_enrichments WHERE model = $1 AND status = 'failed'),
  COALESCE((SELECT avg(latency_ms)::int FROM (SELECT latency_ms FROM job_enrichments
            WHERE model = $1 AND status = 'ok' AND latency_ms > 0 ORDER BY updated_at DESC LIMIT 200) l), 0)`,
		model).Scan(&s.Queue, &s.Enriched, &s.Failed, &s.AvgLatencyMs)
	if err != nil {
		return s, err
	}
	var msg *string
	var at *time.Time
	err = r.pool.QueryRow(ctx, `SELECT error, updated_at FROM job_enrichments WHERE model = $1 AND status = 'failed'
		ORDER BY updated_at DESC LIMIT 1`, model).Scan(&msg, &at)
	if err != nil && err != pgx.ErrNoRows {
		return s, err
	}
	if msg != nil {
		s.LastError, s.LastErrorAt = *msg, at
	}
	return s, nil
}

// SaveExplanation stores the model's reason for one match.
func (r *EnrichmentRepository) SaveExplanation(ctx context.Context, workspaceID, profileID, jobID, descriptionHash, model string, promptVersion int, text string) error {
	_, err := r.pool.Exec(ctx, `
INSERT INTO match_explanations (workspace_id, profile_id, job_id, description_hash, model, prompt_version, text)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (workspace_id, profile_id, job_id, description_hash, model, prompt_version) DO UPDATE SET text = EXCLUDED.text, created_at = NOW()`,
		workspaceID, profileID, jobID, descriptionHash, model, promptVersion, text)
	return err
}

// Explanations returns the newest explanation per job for one workspace's
// profile, skipping those written for an older description.
func (r *EnrichmentRepository) Explanations(ctx context.Context, workspaceID, profileID string, jobIDs []string) (map[string]string, error) {
	out := map[string]string{}
	if len(jobIDs) == 0 {
		return out, nil
	}
	rows, err := r.pool.Query(ctx, `
SELECT DISTINCT ON (x.job_id) x.job_id, x.text
FROM match_explanations x
JOIN jobs j ON j.id = x.job_id AND j.description_hash = x.description_hash
WHERE x.workspace_id = $1 AND x.profile_id = $2 AND x.job_id = ANY($3)
ORDER BY x.job_id, x.created_at DESC`, workspaceID, profileID, jobIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, text string
		if err := rows.Scan(&id, &text); err != nil {
			return nil, err
		}
		out[id] = text
	}
	return out, rows.Err()
}
