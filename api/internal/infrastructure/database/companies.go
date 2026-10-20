package database

import (
	"context"
	"example.com/rolescout/api/internal/focus"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"example.com/rolescout/api/internal/domain"
)

// CompanyRepository manages the global company catalog, per-workspace company
// blocks and adapter-type reviews.
type CompanyRepository struct {
	pool *pgxpool.Pool
}

func NewCompanyRepository(pool *pgxpool.Pool) *CompanyRepository {
	return &CompanyRepository{pool: pool}
}

func scanCompany(row pgx.Row, extra ...any) (domain.Company, error) {
	var c domain.Company
	dst := []any{&c.ID, &c.Name, &c.Domain, &c.CareersURL, &c.ATS, &c.ATSID, &c.Origin, &c.Status,
		&c.LastCheckedAt, &c.LastError, &c.FailureCount, &c.NextCheckAt, &c.SourceID, &c.OpenJobs,
		&c.CreatedAt, &c.UpdatedAt}
	err := row.Scan(append(dst, extra...)...)
	return c, err
}

// CompanyParams describes a discovered company. ATSID identifies the board
// within the ATS; when it is nil the careers URL identifies the company.
type CompanyParams struct {
	Name       string
	CareersURL string
	ATS        string
	Origin     string
	ATSID      *string
	Domain     *string
	// Country is the ISO 3166-1 alpha-2 code the company hires in, when known.
	Country string
	// SameSite lists other spellings of a bare home page (www, http, trailing
	// slash); a company stored under one of them is the same company.
	SameSite []string
}

// UpsertDiscovered adds a company or returns the existing one. A known board
// (ats, ats_id) or careers URL is not duplicated, and a manually added
// company keeps its origin.
func (r *CompanyRepository) UpsertDiscovered(ctx context.Context, p CompanyParams) (domain.Company, bool, error) {
	ats := p.ATS
	if ats == "" {
		ats = "unknown"
	}
	var c domain.Company
	var created bool
	err := inTx(ctx, r.pool, func(tx pgx.Tx) error {
		var id string
		err := tx.QueryRow(ctx, `
			SELECT id FROM companies
			WHERE ($1::text IS NOT NULL AND ats = $2 AND ats_id = $1) OR careers_url = $3 OR careers_url = ANY($4::text[])
			ORDER BY (ats_id = $1) DESC NULLS LAST, (careers_url = $3) DESC LIMIT 1 FOR UPDATE`,
			p.ATSID, ats, p.CareersURL, p.SameSite).Scan(&id)
		switch {
		case err == pgx.ErrNoRows:
			err = tx.QueryRow(ctx, `
				INSERT INTO companies (name, domain, careers_url, ats, ats_id, origin, country)
				VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7::text, ''))
				ON CONFLICT DO NOTHING RETURNING id`,
				p.Name, p.Domain, p.CareersURL, ats, p.ATSID, p.Origin, p.Country).Scan(&id)
			if err == pgx.ErrNoRows {
				// Inserted concurrently by another discoverer.
				return tx.QueryRow(ctx, `SELECT id FROM companies WHERE careers_url = $1 OR (ats = $2 AND ats_id = $3)`,
					p.CareersURL, ats, p.ATSID).Scan(&id)
			}
			if err != nil {
				return translate(err, "insert company")
			}
			created = true
		case err != nil:
			return translate(err, "find company")
		default:
			// A known company learns its country from a later sighting.
			if _, err := tx.Exec(ctx, `UPDATE companies SET country = $2 WHERE id = $1 AND country IS NULL AND $2::text <> ''`, id, p.Country); err != nil {
				return translate(err, "set company country")
			}
		}
		c, err = scanCompany(tx.QueryRow(ctx, `SELECT c.id, c.name, c.domain, c.careers_url, c.ats, c.ats_id, c.origin, c.status,
	c.last_checked_at, c.last_error, c.failure_count, c.next_check_at,
	(SELECT s.id FROM job_sources s WHERE s.company_id = c.id),
	(SELECT count(*) FROM jobs j WHERE j.company_id = c.id AND j.closed_at IS NULL)::int,
	c.created_at, c.updated_at FROM companies c WHERE c.id = $1`, id))
		return translate(err, "read company")
	})
	return c, created, err
}

func (r *CompanyRepository) Get(ctx context.Context, id string) (domain.Company, error) {
	c, err := scanCompany(r.pool.QueryRow(ctx, `SELECT c.id, c.name, c.domain, c.careers_url, c.ats, c.ats_id, c.origin, c.status,
	c.last_checked_at, c.last_error, c.failure_count, c.next_check_at,
	(SELECT s.id FROM job_sources s WHERE s.company_id = c.id),
	(SELECT count(*) FROM jobs j WHERE j.company_id = c.id AND j.closed_at IS NULL)::int,
	c.created_at, c.updated_at FROM companies c WHERE c.id = $1`, id))
	return c, translate(err, "get company")
}

// PendingDetection returns companies whose career page still has to be
// identified: new ones and unknown ones whose next check is due.
func (r *CompanyRepository) PendingDetection(ctx context.Context, limit int) ([]domain.Company, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT c.id, c.name, c.domain, c.careers_url, c.ats, c.ats_id, c.origin, c.status,
	c.last_checked_at, c.last_error, c.failure_count, c.next_check_at,
	(SELECT s.id FROM job_sources s WHERE s.company_id = c.id),
	(SELECT count(*) FROM jobs j WHERE j.company_id = c.id AND j.closed_at IS NULL)::int,
	c.created_at, c.updated_at FROM companies c
		WHERE c.status = 'pending'
		   OR (c.ats = 'unknown' AND c.status NOT IN ('disabled') AND c.next_check_at <= NOW())
		-- Companies of the research profile's countries are identified first.
		ORDER BY COALESCE(c.country = ANY($2::text[]), FALSE) DESC, c.next_check_at, c.id LIMIT $1`, limit, focus.Countries(ctx))
	return collectCompanies(rows, err, "list companies pending detection")
}

func collectCompanies(rows pgx.Rows, err error, op string) ([]domain.Company, error) {
	if err != nil {
		return nil, translate(err, op)
	}
	cs, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Company, error) {
		return scanCompany(row)
	})
	return cs, translate(err, op)
}

// RecordDetection stores the outcome of identifying a company's career page.
// Failed detections are retried with the same backoff as failed syncs.
func (r *CompanyRepository) RecordDetection(ctx context.Context, id, ats string, atsID *string, status domain.CompanyStatus, errMsg *string) (domain.Company, error) {
	failed := status == domain.CompanyUnreachable || status == domain.CompanyBlockedByRobots || ats == "unknown"
	c, err := scanCompany(r.pool.QueryRow(ctx, `
		WITH updated AS (
			UPDATE companies SET
				ats = $2, ats_id = $3,
				status = CASE WHEN status = 'disabled' THEN status ELSE $4 END,
				last_error = $5, last_checked_at = NOW(),
				failure_count = CASE WHEN $6 THEN failure_count + 1 ELSE 0 END,
				next_check_at = CASE WHEN $6 THEN NOW() + LEAST(INTERVAL '1 hour' * power(2, GREATEST(failure_count + 1, 1) - 1), INTERVAL '168 hours') ELSE NOW() END
			WHERE id = $1 RETURNING *
		)
		SELECT c.id, c.name, c.domain, c.careers_url, c.ats, c.ats_id, c.origin, c.status,
	c.last_checked_at, c.last_error, c.failure_count, c.next_check_at,
	(SELECT s.id FROM job_sources s WHERE s.company_id = c.id),
	(SELECT count(*) FROM jobs j WHERE j.company_id = c.id AND j.closed_at IS NULL)::int,
	c.created_at, c.updated_at FROM updated c`, id, ats, atsID, status, errMsg, failed))
	return c, translate(err, "record company detection")
}

var sourceKeyUnsafe = regexp.MustCompile(`[^a-z0-9_-]+`)

// AttachSource creates or updates the enabled job source of a company. The
// source key is derived from the adapter and the company's board.
func (r *CompanyRepository) AttachSource(ctx context.Context, companyID string, p SourceParams) (domain.JobSource, error) {
	config := p.Config
	if config == nil {
		config = map[string]any{}
	}
	var interval *int64
	if p.SyncInterval > 0 {
		secs := int64(p.SyncInterval / time.Second)
		interval = &secs
	}
	key := p.Key
	if key == "" {
		key = "company-" + p.Adapter + "-" + strings.Trim(sourceKeyUnsafe.ReplaceAllString(strings.ToLower(companyID), "-"), "-")
	}
	s, err := scanSource(r.pool.QueryRow(ctx, `
		INSERT INTO job_sources (key, name, base_url, adapter, config, sync_interval, company_id, enabled)
		VALUES ($1, $2, $3, $4, $5, COALESCE(make_interval(secs => $6), INTERVAL '12 hours'), $7, TRUE)
		ON CONFLICT (company_id) DO UPDATE SET
			name = EXCLUDED.name, base_url = EXCLUDED.base_url, adapter = EXCLUDED.adapter,
			config = EXCLUDED.config, enabled = TRUE
		RETURNING id, key, name, base_url, enabled, adapter, config,
	EXTRACT(EPOCH FROM sync_interval)::bigint, reviewed_at, review_note, company_id, created_at, updated_at`, key, p.Name, p.BaseURL, p.Adapter, config, interval, companyID))
	return s, translate(err, "attach company source")
}

// RecordSyncOutcome updates a company's status after a sync of its source.
func (r *CompanyRepository) RecordSyncOutcome(ctx context.Context, companyID string, run domain.IngestionRun) error {
	var status domain.CompanyStatus
	failed := run.Status == domain.RunFailed
	switch {
	case failed && run.Error != nil && strings.Contains(*run.Error, "disallowed by robots.txt"):
		status = domain.CompanyBlockedByRobots
	case failed:
		status = domain.CompanyUnreachable
	case run.FetchedCount == 0:
		status = domain.CompanyEmpty
	default:
		status = domain.CompanyActive
	}
	var errMsg *string
	if failed {
		errMsg = run.Error
	}
	_, err := r.pool.Exec(ctx, `
		UPDATE companies SET
			status = CASE WHEN status = 'disabled' THEN status ELSE $2 END,
			last_error = $3, last_checked_at = NOW(),
			failure_count = CASE WHEN $4 THEN failure_count + 1 ELSE 0 END,
			next_check_at = CASE WHEN $4 THEN NOW() + LEAST(INTERVAL '1 hour' * power(2, GREATEST(failure_count + 1, 1) - 1), INTERVAL '168 hours') ELSE NOW() END
		WHERE id = $1`, companyID, status, errMsg, failed)
	return translate(err, "record company sync outcome")
}

// CompanyFilter selects companies. WorkspaceID marks companies that
// workspace blocked; it does not filter.
type CompanyFilter struct {
	Query       string
	ATS         string
	Status      domain.CompanyStatus
	WorkspaceID string
	AfterName   string
	AfterID     string
	Limit       int
}

// List returns companies by name. It fetches one extra row to report
// whether another page exists.
func (r *CompanyRepository) List(ctx context.Context, f CompanyFilter) ([]domain.Company, bool, error) {
	var pattern string
	if q := strings.TrimSpace(f.Query); q != "" {
		pattern = "%" + escapeLike(q) + "%"
	}
	var ws *string
	if f.WorkspaceID != "" {
		ws = &f.WorkspaceID
	}
	rows, err := r.pool.Query(ctx, `
		SELECT c.id, c.name, c.domain, c.careers_url, c.ats, c.ats_id, c.origin, c.status,
	c.last_checked_at, c.last_error, c.failure_count, c.next_check_at,
	(SELECT s.id FROM job_sources s WHERE s.company_id = c.id),
	(SELECT count(*) FROM jobs j WHERE j.company_id = c.id AND j.closed_at IS NULL)::int,
	c.created_at, c.updated_at,
			($4::uuid IS NOT NULL AND EXISTS (
				SELECT 1 FROM workspace_company_blocks b WHERE b.workspace_id = $4 AND b.company_id = c.id))
		FROM companies c
		WHERE ($1 = '' OR c.name ILIKE $1 OR c.careers_url ILIKE $1)
		  AND ($2 = '' OR c.ats = $2)
		  AND ($3 = '' OR c.status = $3)
		  AND ($5 = '' OR (lower(c.name), c.id) > (lower($5), $6::uuid))
		ORDER BY lower(c.name), c.id
		LIMIT $7`, pattern, f.ATS, string(f.Status), ws, f.AfterName, nullUUID(f.AfterID), f.Limit+1)
	if err != nil {
		return nil, false, translate(err, "list companies")
	}
	cs, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Company, error) {
		var blocked bool
		c, err := scanCompany(row, &blocked)
		c.Blocked = blocked
		return c, err
	})
	if err != nil {
		return nil, false, translate(err, "list companies")
	}
	if len(cs) > f.Limit {
		return cs[:f.Limit], true, nil
	}
	return cs, false, nil
}

func nullUUID(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// SetDisabled turns a company off for everyone, or back on for a new check.
func (r *CompanyRepository) SetDisabled(ctx context.Context, id string, disabled bool) (domain.Company, error) {
	var c domain.Company
	err := inTx(ctx, r.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE companies SET
				status = CASE WHEN $2 THEN 'disabled' ELSE 'pending' END,
				failure_count = 0, next_check_at = NOW()
			WHERE id = $1`, id, disabled)
		if err != nil {
			return translate(err, "set company disabled")
		}
		if tag.RowsAffected() == 0 {
			return fmt.Errorf("set company disabled: %w", domain.ErrNotFound)
		}
		if _, err := tx.Exec(ctx, `UPDATE job_sources SET enabled = NOT $2 WHERE company_id = $1`, id, disabled); err != nil {
			return translate(err, "set company source enabled")
		}
		c, err = scanCompany(tx.QueryRow(ctx, `SELECT c.id, c.name, c.domain, c.careers_url, c.ats, c.ats_id, c.origin, c.status,
	c.last_checked_at, c.last_error, c.failure_count, c.next_check_at,
	(SELECT s.id FROM job_sources s WHERE s.company_id = c.id),
	(SELECT count(*) FROM jobs j WHERE j.company_id = c.id AND j.closed_at IS NULL)::int,
	c.created_at, c.updated_at FROM companies c WHERE c.id = $1`, id))
		return translate(err, "read company")
	})
	return c, err
}

func (r *CompanyRepository) Block(ctx context.Context, workspaceID, companyID string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO workspace_company_blocks (workspace_id, company_id) VALUES ($1, $2)
		ON CONFLICT DO NOTHING`, workspaceID, companyID)
	return translate(err, "block company")
}

func (r *CompanyRepository) Unblock(ctx context.Context, workspaceID, companyID string) error {
	_, err := r.pool.Exec(ctx, `
		DELETE FROM workspace_company_blocks WHERE workspace_id = $1 AND company_id = $2`, workspaceID, companyID)
	return translate(err, "unblock company")
}

// CountsByATS returns how many companies use each ATS.
func (r *CompanyRepository) CountsByATS(ctx context.Context) (map[string]int, error) {
	rows, err := r.pool.Query(ctx, `SELECT ats, count(*)::int FROM companies GROUP BY ats`)
	if err != nil {
		return nil, translate(err, "count companies")
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var ats string
		var n int
		if err := rows.Scan(&ats, &n); err != nil {
			return nil, translate(err, "count companies")
		}
		out[ats] = n
	}
	return out, translate(rows.Err(), "count companies")
}

func (r *CompanyRepository) AdapterReviews(ctx context.Context) ([]domain.AdapterReview, error) {
	rows, err := r.pool.Query(ctx, `SELECT adapter, enabled, reviewed_at, note FROM source_adapter_reviews ORDER BY adapter`)
	if err != nil {
		return nil, translate(err, "list adapter reviews")
	}
	reviews, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.AdapterReview, error) {
		var a domain.AdapterReview
		err := row.Scan(&a.Adapter, &a.Enabled, &a.ReviewedAt, &a.Note)
		return a, err
	})
	return reviews, translate(err, "list adapter reviews")
}

// ReviewAdapter records a review of an adapter type and enables or disables
// it. Enabling needs a non-blank note.
func (r *CompanyRepository) ReviewAdapter(ctx context.Context, adapter, note string, enabled bool) (domain.AdapterReview, error) {
	note = strings.TrimSpace(note)
	var a domain.AdapterReview
	if enabled && note == "" {
		return a, fmt.Errorf("review adapter: a note is required to enable it: %w", domain.ErrInvalid)
	}
	err := r.pool.QueryRow(ctx, `
		INSERT INTO source_adapter_reviews (adapter, enabled, reviewed_at, note)
		VALUES ($1, $2, CASE WHEN $3::text = '' THEN NULL ELSE NOW() END, NULLIF($3::text, ''))
		ON CONFLICT (adapter) DO UPDATE SET
			enabled = EXCLUDED.enabled,
			reviewed_at = COALESCE(EXCLUDED.reviewed_at, source_adapter_reviews.reviewed_at),
			note = COALESCE(EXCLUDED.note, source_adapter_reviews.note)
		RETURNING adapter, enabled, reviewed_at, note`, adapter, enabled, note,
	).Scan(&a.Adapter, &a.Enabled, &a.ReviewedAt, &a.Note)
	return a, translate(err, "review adapter")
}

// UpdateTiers sets how often company sources sync: top when the company has
// an open job matching any profile, mid when it has open jobs, low otherwise.
func (r *CompanyRepository) UpdateTiers(ctx context.Context, top, mid, low time.Duration) error {
	_, err := r.pool.Exec(ctx, `
		WITH tiers AS (
			SELECT s.id, CASE
				WHEN EXISTS (SELECT 1 FROM jobs j JOIN research_matches m ON m.job_id = j.id
				             WHERE j.company_id = s.company_id AND j.closed_at IS NULL) THEN make_interval(secs => $1)
				WHEN EXISTS (SELECT 1 FROM jobs j WHERE j.company_id = s.company_id AND j.closed_at IS NULL)
				     THEN make_interval(secs => $2)
				ELSE make_interval(secs => $3) END AS interval
			FROM job_sources s WHERE s.company_id IS NOT NULL
		)
		UPDATE job_sources s SET sync_interval = t.interval
		FROM tiers t WHERE s.id = t.id AND s.sync_interval <> t.interval`,
		top.Seconds(), mid.Seconds(), low.Seconds())
	return translate(err, "update company sync tiers")
}

// NamesWithoutBoard lists names (in name order, after the given one) of
// companies whose career page could not be read, for CatalogProbe.
func (r *CompanyRepository) NamesWithoutBoard(ctx context.Context, after string, limit int) ([]string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT name FROM (
			SELECT DISTINCT ON (lower(name)) name FROM companies
			WHERE ats = 'unknown' AND status IN ('unreachable', 'empty', 'pending')
			ORDER BY lower(name), id
		) n
		WHERE lower(name) > lower($1)
		ORDER BY lower(name)
		LIMIT $2`, after, limit)
	if err != nil {
		return nil, translate(err, "list names without board")
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, translate(err, "scan name")
		}
		out = append(out, n)
	}
	return out, translate(rows.Err(), "list names without board")
}

// RefreshCountries sets each company's country to the country most of its jobs
// are in, so the next research run can read the companies of a profile's
// countries first. It returns how many companies changed.
func (r *CompanyRepository) RefreshCountries(ctx context.Context) (int64, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE companies c SET country = j.country
		FROM (
			SELECT company_id, mode() WITHIN GROUP (ORDER BY country_code) AS country
			FROM jobs
			WHERE company_id IS NOT NULL AND country_code IS NOT NULL
			GROUP BY company_id
		) j
		WHERE c.id = j.company_id AND c.country IS DISTINCT FROM j.country`)
	return tag.RowsAffected(), translate(err, "refresh company countries")
}
