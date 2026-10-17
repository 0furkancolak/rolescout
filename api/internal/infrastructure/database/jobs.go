package database

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"example.com/rolescout/api/internal/domain"
)

type JobRepository struct {
	pool *pgxpool.Pool
}

func NewJobRepository(pool *pgxpool.Pool) *JobRepository {
	return &JobRepository{pool: pool}
}

const jobColumns = `id, dedupe_key, title, company_name, location, work_arrangement,
	employment_type, seniority, technologies, description, first_seen_at,
	last_seen_at, created_at, updated_at`

func scanJob(row pgx.Row) (domain.Job, error) {
	var j domain.Job
	err := row.Scan(&j.ID, &j.DedupeKey, &j.Title, &j.CompanyName, &j.Location,
		&j.WorkArrangement, &j.EmploymentType, &j.Seniority, &j.Technologies,
		&j.Description, &j.FirstSeenAt, &j.LastSeenAt, &j.CreatedAt, &j.UpdatedAt)
	return j, err
}

const listingColumns = `id, job_id, source_id, external_id, url, source_posted_at,
	source_updated_at, first_ingested_at, last_ingested_at`

func scanListing(row pgx.Row) (domain.JobListing, error) {
	var l domain.JobListing
	err := row.Scan(&l.ID, &l.JobID, &l.SourceID, &l.ExternalID, &l.URL,
		&l.SourcePostedAt, &l.SourceUpdatedAt, &l.FirstIngestedAt, &l.LastIngestedAt)
	return l, err
}

// CreateSource registers a job source. Sources start disabled until their
// access method and terms have been reviewed.
func (r *JobRepository) CreateSource(ctx context.Context, key, name, baseURL string) (domain.JobSource, error) {
	var s domain.JobSource
	err := r.pool.QueryRow(ctx, `
		INSERT INTO job_sources (key, name, base_url) VALUES ($1, $2, $3)
		RETURNING id, key, name, base_url, enabled, created_at, updated_at`,
		key, name, baseURL,
	).Scan(&s.ID, &s.Key, &s.Name, &s.BaseURL, &s.Enabled, &s.CreatedAt, &s.UpdatedAt)
	return s, translate(err, "create job source")
}

// IngestedListing is a normalized listing produced by a source adapter.
type IngestedListing struct {
	SourceID        string
	ExternalID      string
	URL             string
	SourcePostedAt  *time.Time
	SourceUpdatedAt *time.Time
	SeenAt          time.Time

	DedupeKey       string
	Title           string
	CompanyName     string
	Location        *string
	WorkArrangement string
	EmploymentType  string
	Seniority       string
	Technologies    []string
	Description     string
}

// UpsertListing idempotently records a listing and its canonical job. Ingesting
// the same listing twice updates the existing rows instead of duplicating them,
// and listings from different sources with the same dedupe key share one job.
func (r *JobRepository) UpsertListing(ctx context.Context, in IngestedListing) (domain.Job, domain.JobListing, error) {
	seenAt := in.SeenAt
	if seenAt.IsZero() {
		seenAt = time.Now()
	}
	seenAt = seenAt.UTC()
	technologies := in.Technologies
	if technologies == nil {
		technologies = []string{}
	}

	var job domain.Job
	var listing domain.JobListing
	err := inTx(ctx, r.pool, func(tx pgx.Tx) error {
		var err error
		job, err = scanJob(tx.QueryRow(ctx, `
			INSERT INTO jobs (dedupe_key, title, company_name, location, work_arrangement,
				employment_type, seniority, technologies, description, first_seen_at, last_seen_at)
			VALUES ($1, $2, $3, $4, COALESCE(NULLIF($5, ''), 'unknown'),
				COALESCE(NULLIF($6, ''), 'unknown'), COALESCE(NULLIF($7, ''), 'unknown'),
				$8, $9, $10, $10)
			ON CONFLICT (dedupe_key) DO UPDATE SET
				title = EXCLUDED.title,
				company_name = EXCLUDED.company_name,
				location = EXCLUDED.location,
				work_arrangement = EXCLUDED.work_arrangement,
				employment_type = EXCLUDED.employment_type,
				seniority = EXCLUDED.seniority,
				technologies = EXCLUDED.technologies,
				description = EXCLUDED.description,
				first_seen_at = LEAST(jobs.first_seen_at, EXCLUDED.first_seen_at),
				last_seen_at = GREATEST(jobs.last_seen_at, EXCLUDED.last_seen_at)
			RETURNING `+jobColumns,
			in.DedupeKey, in.Title, in.CompanyName, in.Location, in.WorkArrangement,
			in.EmploymentType, in.Seniority, technologies, in.Description, seenAt,
		))
		if err != nil {
			return translate(err, "upsert job")
		}

		listing, err = scanListing(tx.QueryRow(ctx, `
			INSERT INTO job_source_listings (job_id, source_id, external_id, url,
				source_posted_at, source_updated_at, first_ingested_at, last_ingested_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $7)
			ON CONFLICT (source_id, external_id) DO UPDATE SET
				job_id = EXCLUDED.job_id,
				url = EXCLUDED.url,
				source_posted_at = EXCLUDED.source_posted_at,
				source_updated_at = EXCLUDED.source_updated_at,
				last_ingested_at = GREATEST(job_source_listings.last_ingested_at, EXCLUDED.last_ingested_at)
			RETURNING `+listingColumns,
			job.ID, in.SourceID, in.ExternalID, in.URL, in.SourcePostedAt,
			in.SourceUpdatedAt, seenAt,
		))
		return translate(err, "upsert job listing")
	})
	return job, listing, err
}

func (r *JobRepository) GetJob(ctx context.Context, id string) (domain.Job, error) {
	j, err := scanJob(r.pool.QueryRow(ctx, `SELECT `+jobColumns+` FROM jobs WHERE id = $1`, id))
	return j, translate(err, "get job")
}

func (r *JobRepository) ListListings(ctx context.Context, jobID string) ([]domain.JobListing, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+listingColumns+` FROM job_source_listings
		WHERE job_id = $1 ORDER BY first_ingested_at, id`, jobID)
	if err != nil {
		return nil, translate(err, "list job listings")
	}
	ls, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.JobListing, error) {
		return scanListing(row)
	})
	return ls, translate(err, "list job listings")
}

// SaveJob bookmarks a job for a workspace. Saving an already saved job is a no-op.
func (r *JobRepository) SaveJob(ctx context.Context, workspaceID, jobID string, savedBy *string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO saved_jobs (workspace_id, job_id, saved_by) VALUES ($1, $2, $3)
		ON CONFLICT (workspace_id, job_id) DO NOTHING`, workspaceID, jobID, savedBy)
	return translate(err, "save job")
}

func (r *JobRepository) UnsaveJob(ctx context.Context, workspaceID, jobID string) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM saved_jobs WHERE workspace_id = $1 AND job_id = $2`, workspaceID, jobID)
	if err != nil {
		return translate(err, "unsave job")
	}
	if tag.RowsAffected() == 0 {
		return translate(pgx.ErrNoRows, "unsave job")
	}
	return nil
}

// ListSavedJobs returns the workspace's saved jobs, most recently saved first.
func (r *JobRepository) ListSavedJobs(ctx context.Context, workspaceID string) ([]domain.SavedJob, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT workspace_id, job_id, saved_by, created_at FROM saved_jobs
		WHERE workspace_id = $1 ORDER BY created_at DESC, job_id`, workspaceID)
	if err != nil {
		return nil, translate(err, "list saved jobs")
	}
	saved, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.SavedJob, error) {
		var s domain.SavedJob
		err := row.Scan(&s.WorkspaceID, &s.JobID, &s.SavedBy, &s.CreatedAt)
		return s, err
	})
	return saved, translate(err, "list saved jobs")
}
