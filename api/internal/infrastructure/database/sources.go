package database

import (
	"context"
	"errors"
	"example.com/rolescout/api/internal/focus"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"example.com/rolescout/api/internal/domain"
)

// SourceRepository manages job sources and their ingestion runs.
type SourceRepository struct {
	pool *pgxpool.Pool
}

func NewSourceRepository(pool *pgxpool.Pool) *SourceRepository {
	return &SourceRepository{pool: pool}
}

func scanSource(row pgx.Row) (domain.JobSource, error) {
	var s domain.JobSource
	var seconds int64
	err := row.Scan(&s.ID, &s.Key, &s.Name, &s.BaseURL, &s.Enabled, &s.Adapter, &s.Config,
		&seconds, &s.ReviewedAt, &s.ReviewNote, &s.CompanyID, &s.CreatedAt, &s.UpdatedAt)
	s.SyncInterval = time.Duration(seconds) * time.Second
	return s, err
}

// SourceParams describes a new source. Empty Adapter means "manual" and a
// zero SyncInterval uses the database default.
type SourceParams struct {
	Key          string
	Name         string
	BaseURL      string
	Adapter      string
	Config       map[string]any
	SyncInterval time.Duration
}

func (r *SourceRepository) Create(ctx context.Context, p SourceParams) (domain.JobSource, error) {
	adapter := p.Adapter
	if adapter == "" {
		adapter = "manual"
	}
	config := p.Config
	if config == nil {
		config = map[string]any{}
	}
	var interval *int64
	if p.SyncInterval > 0 {
		secs := int64(p.SyncInterval / time.Second)
		interval = &secs
	}
	s, err := scanSource(r.pool.QueryRow(ctx, `
		INSERT INTO job_sources (key, name, base_url, adapter, config, sync_interval)
		VALUES ($1, $2, $3, $4, $5, COALESCE(make_interval(secs => $6), INTERVAL '6 hours'))
		RETURNING id, key, name, base_url, enabled, adapter, config,
	EXTRACT(EPOCH FROM sync_interval)::bigint, reviewed_at, review_note, company_id, created_at, updated_at`, p.Key, p.Name, p.BaseURL, adapter, config, interval))
	return s, translate(err, "create job source")
}

func (r *SourceRepository) Get(ctx context.Context, key string) (domain.JobSource, error) {
	s, err := scanSource(r.pool.QueryRow(ctx, `SELECT id, key, name, base_url, enabled, adapter, config,
	EXTRACT(EPOCH FROM sync_interval)::bigint, reviewed_at, review_note, company_id, created_at, updated_at FROM job_sources WHERE key = $1`, key))
	return s, translate(err, "get job source")
}

func (r *SourceRepository) List(ctx context.Context) ([]domain.JobSource, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, key, name, base_url, enabled, adapter, config,
	EXTRACT(EPOCH FROM sync_interval)::bigint, reviewed_at, review_note, company_id, created_at, updated_at FROM job_sources ORDER BY key`)
	if err != nil {
		return nil, translate(err, "list job sources")
	}
	sources, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.JobSource, error) {
		return scanSource(row)
	})
	return sources, translate(err, "list job sources")
}

// Review records that the source's access method, permissions and terms were
// checked. The note should say what was checked and where the terms live.
func (r *SourceRepository) Review(ctx context.Context, key, note string) (domain.JobSource, error) {
	s, err := scanSource(r.pool.QueryRow(ctx, `
		UPDATE job_sources SET reviewed_at = NOW(), review_note = $2
		WHERE key = $1 RETURNING id, key, name, base_url, enabled, adapter, config,
	EXTRACT(EPOCH FROM sync_interval)::bigint, reviewed_at, review_note, company_id, created_at, updated_at`, key, note))
	return s, translate(err, "review job source")
}

// ErrSourceNotReviewed is returned when enabling a source without a review.
var ErrSourceNotReviewed = fmt.Errorf("source must be reviewed before it is enabled: %w", domain.ErrInvalid)

func (r *SourceRepository) SetEnabled(ctx context.Context, key string, enabled bool) (domain.JobSource, error) {
	s, err := scanSource(r.pool.QueryRow(ctx, `
		UPDATE job_sources SET enabled = $2 WHERE key = $1 RETURNING id, key, name, base_url, enabled, adapter, config,
	EXTRACT(EPOCH FROM sync_interval)::bigint, reviewed_at, review_note, company_id, created_at, updated_at`, key, enabled))
	err = translate(err, "set job source enabled")
	if enabled && errors.Is(err, domain.ErrInvalid) {
		return s, ErrSourceNotReviewed
	}
	return s, err
}

// Due returns enabled sources whose next synchronization is due at now. A
// source is due when it has no run yet, or its last run started at least one
// sync interval ago. After a failed run the source is retried sooner, after a
// quarter of the interval but no less than five minutes.
func (r *SourceRepository) Due(ctx context.Context, now time.Time) ([]domain.JobSource, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT s.id, s.key, s.name, s.base_url, s.enabled, s.adapter, s.config,
			EXTRACT(EPOCH FROM s.sync_interval)::bigint, s.reviewed_at, s.review_note,
			s.company_id, s.created_at, s.updated_at
		FROM job_sources s
		LEFT JOIN LATERAL (
			SELECT status, started_at FROM ingestion_runs
			WHERE source_id = s.id ORDER BY started_at DESC LIMIT 1
		) last ON TRUE
		WHERE s.enabled
		  -- A company source waits out its company's failure backoff.
		  AND NOT EXISTS (SELECT 1 FROM companies c WHERE c.id = s.company_id AND c.next_check_at > $1)
		  AND (last.status IS NULL OR (last.status <> 'running' AND last.started_at +
		       CASE WHEN last.status = 'failed'
		            THEN GREATEST(s.sync_interval / 4, INTERVAL '5 minutes')
		            ELSE s.sync_interval END <= $1))
		-- Sources of the research profile's countries are read first.
		ORDER BY COALESCE((SELECT c.country = ANY($2::text[]) FROM companies c WHERE c.id = s.company_id), FALSE) DESC,
		         last.started_at NULLS FIRST, s.key`, now, focus.Countries(ctx))
	if err != nil {
		return nil, translate(err, "list due job sources")
	}
	sources, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.JobSource, error) {
		return scanSource(row)
	})
	return sources, translate(err, "list due job sources")
}

func scanRun(row pgx.Row) (domain.IngestionRun, error) {
	var run domain.IngestionRun
	err := row.Scan(&run.ID, &run.SourceID, &run.Status, &run.StartedAt, &run.FinishedAt,
		&run.Attempts, &run.FetchedCount, &run.UpsertedCount, &run.FailedCount, &run.Error)
	return run, err
}

// StartRun records a running synchronization. It returns domain.ErrConflict
// when the source already has a run in progress, which keeps concurrent
// workers from syncing the same source twice.
func (r *SourceRepository) StartRun(ctx context.Context, sourceID string) (domain.IngestionRun, error) {
	run, err := scanRun(r.pool.QueryRow(ctx, `
		INSERT INTO ingestion_runs (source_id) VALUES ($1) RETURNING id, source_id, status, started_at, finished_at, attempts, fetched_count,
	upserted_count, failed_count, error`, sourceID))
	return run, translate(err, "start ingestion run")
}

// FinishRun stores the outcome of a run. Status must be succeeded or failed.
func (r *SourceRepository) FinishRun(ctx context.Context, run domain.IngestionRun) (domain.IngestionRun, error) {
	out, err := scanRun(r.pool.QueryRow(ctx, `
		UPDATE ingestion_runs SET status = $2, finished_at = NOW(), attempts = $3,
			fetched_count = $4, upserted_count = $5, failed_count = $6, error = $7
		WHERE id = $1 AND status = 'running'
		RETURNING id, source_id, status, started_at, finished_at, attempts, fetched_count,
	upserted_count, failed_count, error`, run.ID, run.Status, run.Attempts, run.FetchedCount,
		run.UpsertedCount, run.FailedCount, run.Error))
	return out, translate(err, "finish ingestion run")
}

// FailStaleRuns marks runs that have been running for longer than maxAge as
// failed, for example after a worker crashed. It returns how many it marked.
func (r *SourceRepository) FailStaleRuns(ctx context.Context, maxAge time.Duration) (int64, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE ingestion_runs SET status = 'failed', finished_at = NOW(),
			error = 'interrupted: the worker stopped before the run finished'
		WHERE status = 'running' AND started_at < NOW() - make_interval(secs => $1)`,
		maxAge.Seconds())
	if err != nil {
		return 0, translate(err, "fail stale ingestion runs")
	}
	return tag.RowsAffected(), nil
}

// ListRuns returns the most recent runs of a source, newest first.
func (r *SourceRepository) ListRuns(ctx context.Context, sourceID string, limit int) ([]domain.IngestionRun, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, source_id, status, started_at, finished_at, attempts, fetched_count,
	upserted_count, failed_count, error FROM ingestion_runs WHERE source_id = $1
		ORDER BY started_at DESC, id LIMIT $2`, sourceID, limit)
	if err != nil {
		return nil, translate(err, "list ingestion runs")
	}
	runs, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.IngestionRun, error) {
		return scanRun(row)
	})
	return runs, translate(err, "list ingestion runs")
}

// PurgeRuns deletes finished runs older than olderThan, keeping each
// source's latest successful run so failure tracking still has a baseline.
func (r *SourceRepository) PurgeRuns(ctx context.Context, olderThan time.Duration) (int64, error) {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM ingestion_runs r
		WHERE r.status <> 'running'
		  AND r.started_at < NOW() - make_interval(secs => $1)
		  AND r.id <> COALESCE((
			SELECT k.id FROM ingestion_runs k
			WHERE k.source_id = r.source_id AND k.status = 'succeeded'
			ORDER BY k.started_at DESC LIMIT 1), '00000000-0000-0000-0000-000000000000')`, olderThan.Seconds())
	if err != nil {
		return 0, translate(err, "purge ingestion runs")
	}
	return tag.RowsAffected(), nil
}
