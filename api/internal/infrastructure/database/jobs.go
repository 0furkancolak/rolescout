package database

import (
	"context"
	"strings"
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

// CreateSource registers a manually managed job source. Sources start
// disabled until their access method and terms have been reviewed.
func (r *JobRepository) CreateSource(ctx context.Context, key, name, baseURL string) (domain.JobSource, error) {
	return NewSourceRepository(r.pool).Create(ctx, SourceParams{Key: key, Name: name, BaseURL: baseURL})
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
	technologies := normalizeTechnologies(in.Technologies)

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

// normalizeTechnologies lower-cases, trims and de-duplicates technology names
// so filters match regardless of how a source spelled them.
func normalizeTechnologies(in []string) []string {
	out := make([]string, 0, len(in))
	seen := make(map[string]bool, len(in))
	for _, t := range in {
		t = strings.ToLower(strings.TrimSpace(t))
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	return out
}

// JobCursor marks the last job of a page in (last_seen_at, id) order.
type JobCursor struct {
	LastSeenAt time.Time
	ID         string
}

// JobFilter selects jobs. Empty fields do not filter. Technologies must all be
// present on a job.
type JobFilter struct {
	Query           string
	WorkArrangement string
	EmploymentType  string
	Seniority       string
	Technologies    []string
	After           *JobCursor
	Limit           int
}

// escapeLike escapes LIKE wildcards so user input matches literally.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

// SearchJobs returns jobs most recently seen first. It fetches one row more
// than Limit so callers can tell whether another page exists.
func (r *JobRepository) SearchJobs(ctx context.Context, f JobFilter) (jobs []domain.Job, hasMore bool, err error) {
	var pattern string
	if q := strings.TrimSpace(f.Query); q != "" {
		pattern = "%" + escapeLike(q) + "%"
	}
	var afterTime *time.Time
	var afterID *string
	if f.After != nil {
		afterTime, afterID = &f.After.LastSeenAt, &f.After.ID
	}

	rows, err := r.pool.Query(ctx, `
		SELECT `+jobColumns+` FROM jobs
		WHERE ($1 = '' OR title ILIKE $1 OR company_name ILIKE $1 OR location ILIKE $1)
		  AND ($2 = '' OR work_arrangement = $2)
		  AND ($3 = '' OR employment_type = $3)
		  AND ($4 = '' OR seniority = $4)
		  AND (cardinality($5::text[]) = 0 OR technologies @> $5::text[])
		  AND ($6::timestamptz IS NULL OR (last_seen_at, id) < ($6, $7::uuid))
		ORDER BY last_seen_at DESC, id DESC
		LIMIT $8`,
		pattern, f.WorkArrangement, f.EmploymentType, f.Seniority,
		normalizeTechnologies(f.Technologies), afterTime, afterID, f.Limit+1)
	if err != nil {
		return nil, false, translate(err, "search jobs")
	}
	jobs, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Job, error) {
		return scanJob(row)
	})
	if err != nil {
		return nil, false, translate(err, "search jobs")
	}
	if len(jobs) > f.Limit {
		return jobs[:f.Limit], true, nil
	}
	return jobs, false, nil
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

// ListSavedJobs returns the workspace's saved jobs with their job details,
// most recently saved first.
func (r *JobRepository) ListSavedJobs(ctx context.Context, workspaceID string) ([]domain.SavedJob, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT s.workspace_id, s.saved_by, s.created_at, `+prefixed("j", jobColumns)+`
		FROM saved_jobs s JOIN jobs j ON j.id = s.job_id
		WHERE s.workspace_id = $1 ORDER BY s.created_at DESC, s.job_id`, workspaceID)
	if err != nil {
		return nil, translate(err, "list saved jobs")
	}
	saved, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.SavedJob, error) {
		var s domain.SavedJob
		var j domain.Job
		err := row.Scan(&s.WorkspaceID, &s.SavedBy, &s.CreatedAt,
			&j.ID, &j.DedupeKey, &j.Title, &j.CompanyName, &j.Location,
			&j.WorkArrangement, &j.EmploymentType, &j.Seniority, &j.Technologies,
			&j.Description, &j.FirstSeenAt, &j.LastSeenAt, &j.CreatedAt, &j.UpdatedAt)
		s.JobID, s.Job = j.ID, &j
		return s, err
	})
	return saved, translate(err, "list saved jobs")
}

// prefixed qualifies a comma-separated column list with a table alias.
func prefixed(alias, columns string) string {
	parts := strings.Split(columns, ",")
	for i, c := range parts {
		parts[i] = alias + "." + strings.TrimSpace(c)
	}
	return strings.Join(parts, ", ")
}
