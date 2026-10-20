package database

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"

	"example.com/rolescout/api/internal/domain"
)

// MaxRelevanceOffset bounds offset pagination of relevance-sorted results.
const MaxRelevanceOffset = 2000

// Every statement below is written out in full. The thirteen filter arguments
// ($1..$13) are the same in all of them and an empty filter is passed as NULL,
// an empty list or an empty string, which the statement turns into "no
// condition". Values only ever travel as parameters, never as SQL text.
//
//	$1 include closed   $2 workspace   $3 profile   $4 text query   $5 arrangement
//	$6 employment type  $7 seniority   $8 technologies   $9 posted within (seconds)
//	$10 countries       $11 location pattern   $12 job board system   $13 companies
//
// Effective values (see docs): the rules' value wins, the AI fills gaps:
//	seniority    COALESCE(NULLIF(j.seniority, 'unknown'), ai.seniority, 'unknown')
//	arrangement  COALESCE(NULLIF(j.work_arrangement, 'unknown'), ai.work_arrangement, 'unknown')
//	country      COALESCE(j.country_code, ai.country_code)
//	technologies (j.technologies || COALESCE(ai.technologies, '{}'::text[]))

// filterArgs returns $1..$13 for f.
func filterArgs(f JobFilter) []any {
	var workspace, profile *string
	if f.WorkspaceID != "" {
		workspace = &f.WorkspaceID
	}
	if f.ProfileID != "" {
		profile = &f.ProfileID
	}
	location := ""
	if f.Location != "" {
		location = "%" + escapeLike(f.Location) + "%"
	}
	countries, companies := f.Countries, f.CompanyIDs
	if countries == nil {
		countries = []string{}
	}
	if companies == nil {
		companies = []string{}
	}
	return []any{
		f.IncludeClosed, workspace, profile, strings.TrimSpace(f.Query), f.WorkArrangement,
		f.EmploymentType, f.Seniority, normalizeTechnologies(f.Technologies), f.PostedWithin.Seconds(),
		countries, location, f.ATS, companies,
	}
}

const zeroUUID = "00000000-0000-0000-0000-000000000000"

// WorkspaceJobs returns one page of jobs as a workspace sees them, with the
// saved flag and, when ProfileID is set, each job's match.
func (r *JobRepository) WorkspaceJobs(ctx context.Context, f JobFilter) ([]domain.WorkspaceJob, bool, error) {
	args := append(filterArgs(f), f.Limit+1) // $14
	var rows pgx.Rows
	var err error
	switch f.Sort {
	case "score":
		after, score, id := false, 0, zeroUUID
		if f.After != nil {
			after, score, id = true, f.After.Score, f.After.ID
		}
		rows, err = r.pool.Query(ctx, `
		SELECT j.id,
			j.dedupe_key,
			j.title,
			j.company_name,
			j.location,
			COALESCE(NULLIF(j.work_arrangement, 'unknown'), ai.work_arrangement, 'unknown'),
			j.employment_type,
			COALESCE(NULLIF(j.seniority, 'unknown'), ai.seniority, 'unknown'),
			ARRAY(SELECT DISTINCT t FROM unnest((j.technologies || COALESCE(ai.technologies, '{}'::text[]))) AS t ORDER BY t),
			j.description,
			j.company_id,
			COALESCE(j.country_code, ai.country_code),
			j.closed_at,
			j.first_seen_at,
			j.last_seen_at,
			j.created_at,
			j.updated_at,
			EXISTS (SELECT 1 FROM saved_jobs sj WHERE sj.workspace_id = $2::uuid AND sj.job_id = j.id),
			m.score, m.reasons, m.is_new, m.first_matched_at
		FROM jobs j
		LEFT JOIN job_ai ai ON ai.job_id = j.id
		LEFT JOIN research_matches m ON m.job_id = j.id AND m.profile_id = $3::uuid
		WHERE ($1::bool OR j.closed_at IS NULL)
		  AND ($3::uuid IS NULL OR m.job_id IS NOT NULL)
		  AND ($2::uuid IS NULL OR NOT EXISTS (SELECT 1 FROM workspace_company_blocks b
			WHERE b.workspace_id = $2::uuid AND b.company_id = j.company_id))
		  AND ($4::text = '' OR j.search @@ websearch_to_tsquery('simple', $4::text))
		  AND ($5::text = '' OR COALESCE(NULLIF(j.work_arrangement, 'unknown'), ai.work_arrangement, 'unknown') = $5::text)
		  AND ($6::text = '' OR j.employment_type = $6::text)
		  AND ($7::text = '' OR COALESCE(NULLIF(j.seniority, 'unknown'), ai.seniority, 'unknown') = $7::text)
		  AND (cardinality($8::text[]) = 0 OR (j.technologies || COALESCE(ai.technologies, '{}'::text[])) @> $8::text[])
		  AND ($9::float8 <= 0 OR j.first_seen_at >= NOW() - make_interval(secs => $9::float8))
		  AND (cardinality($10::text[]) = 0 OR COALESCE(j.country_code, ai.country_code) = ANY($10::text[]))
		  AND ($11::text = '' OR lower(translate(COALESCE(j.location, ''), 'İIıçÇğĞöÖşŞüÜâÂîÎûÛ', 'iiicCgGoOsSuUaAiIuU')) LIKE $11::text)
		  AND ($12::text = '' OR EXISTS (SELECT 1 FROM job_source_listings l JOIN job_sources s ON s.id = l.source_id
			WHERE l.job_id = j.id AND l.closed_at IS NULL AND s.adapter = $12::text))
		  AND (cardinality($13::uuid[]) = 0 OR j.company_id = ANY($13::uuid[]))
		  AND ($15::bool = false OR (m.score, j.id) < ($16::int, $17::uuid))
		ORDER BY m.score DESC, j.id DESC
		LIMIT $14`, append(args, after, score, id)...)
	case "relevance":
		offset := 0
		if f.After != nil {
			offset = min(f.After.Offset, MaxRelevanceOffset)
		}
		rows, err = r.pool.Query(ctx, `
		SELECT j.id,
			j.dedupe_key,
			j.title,
			j.company_name,
			j.location,
			COALESCE(NULLIF(j.work_arrangement, 'unknown'), ai.work_arrangement, 'unknown'),
			j.employment_type,
			COALESCE(NULLIF(j.seniority, 'unknown'), ai.seniority, 'unknown'),
			ARRAY(SELECT DISTINCT t FROM unnest((j.technologies || COALESCE(ai.technologies, '{}'::text[]))) AS t ORDER BY t),
			j.description,
			j.company_id,
			COALESCE(j.country_code, ai.country_code),
			j.closed_at,
			j.first_seen_at,
			j.last_seen_at,
			j.created_at,
			j.updated_at,
			EXISTS (SELECT 1 FROM saved_jobs sj WHERE sj.workspace_id = $2::uuid AND sj.job_id = j.id),
			m.score, m.reasons, m.is_new, m.first_matched_at
		FROM jobs j
		LEFT JOIN job_ai ai ON ai.job_id = j.id
		LEFT JOIN research_matches m ON m.job_id = j.id AND m.profile_id = $3::uuid
		WHERE ($1::bool OR j.closed_at IS NULL)
		  AND ($3::uuid IS NULL OR m.job_id IS NOT NULL)
		  AND ($2::uuid IS NULL OR NOT EXISTS (SELECT 1 FROM workspace_company_blocks b
			WHERE b.workspace_id = $2::uuid AND b.company_id = j.company_id))
		  AND ($4::text = '' OR j.search @@ websearch_to_tsquery('simple', $4::text))
		  AND ($5::text = '' OR COALESCE(NULLIF(j.work_arrangement, 'unknown'), ai.work_arrangement, 'unknown') = $5::text)
		  AND ($6::text = '' OR j.employment_type = $6::text)
		  AND ($7::text = '' OR COALESCE(NULLIF(j.seniority, 'unknown'), ai.seniority, 'unknown') = $7::text)
		  AND (cardinality($8::text[]) = 0 OR (j.technologies || COALESCE(ai.technologies, '{}'::text[])) @> $8::text[])
		  AND ($9::float8 <= 0 OR j.first_seen_at >= NOW() - make_interval(secs => $9::float8))
		  AND (cardinality($10::text[]) = 0 OR COALESCE(j.country_code, ai.country_code) = ANY($10::text[]))
		  AND ($11::text = '' OR lower(translate(COALESCE(j.location, ''), 'İIıçÇğĞöÖşŞüÜâÂîÎûÛ', 'iiicCgGoOsSuUaAiIuU')) LIKE $11::text)
		  AND ($12::text = '' OR EXISTS (SELECT 1 FROM job_source_listings l JOIN job_sources s ON s.id = l.source_id
			WHERE l.job_id = j.id AND l.closed_at IS NULL AND s.adapter = $12::text))
		  AND (cardinality($13::uuid[]) = 0 OR j.company_id = ANY($13::uuid[]))
		ORDER BY ts_rank_cd(j.search, websearch_to_tsquery('simple', $4::text)) DESC, j.id DESC
		LIMIT $14 OFFSET $15::int`, append(args, offset)...)
	default:
		after, seen, id := false, time.Time{}, zeroUUID
		if f.After != nil {
			after, seen, id = true, f.After.FirstSeenAt, f.After.ID
		}
		rows, err = r.pool.Query(ctx, `
		SELECT j.id,
			j.dedupe_key,
			j.title,
			j.company_name,
			j.location,
			COALESCE(NULLIF(j.work_arrangement, 'unknown'), ai.work_arrangement, 'unknown'),
			j.employment_type,
			COALESCE(NULLIF(j.seniority, 'unknown'), ai.seniority, 'unknown'),
			ARRAY(SELECT DISTINCT t FROM unnest((j.technologies || COALESCE(ai.technologies, '{}'::text[]))) AS t ORDER BY t),
			j.description,
			j.company_id,
			COALESCE(j.country_code, ai.country_code),
			j.closed_at,
			j.first_seen_at,
			j.last_seen_at,
			j.created_at,
			j.updated_at,
			EXISTS (SELECT 1 FROM saved_jobs sj WHERE sj.workspace_id = $2::uuid AND sj.job_id = j.id),
			m.score, m.reasons, m.is_new, m.first_matched_at
		FROM jobs j
		LEFT JOIN job_ai ai ON ai.job_id = j.id
		LEFT JOIN research_matches m ON m.job_id = j.id AND m.profile_id = $3::uuid
		WHERE ($1::bool OR j.closed_at IS NULL)
		  AND ($3::uuid IS NULL OR m.job_id IS NOT NULL)
		  AND ($2::uuid IS NULL OR NOT EXISTS (SELECT 1 FROM workspace_company_blocks b
			WHERE b.workspace_id = $2::uuid AND b.company_id = j.company_id))
		  AND ($4::text = '' OR j.search @@ websearch_to_tsquery('simple', $4::text))
		  AND ($5::text = '' OR COALESCE(NULLIF(j.work_arrangement, 'unknown'), ai.work_arrangement, 'unknown') = $5::text)
		  AND ($6::text = '' OR j.employment_type = $6::text)
		  AND ($7::text = '' OR COALESCE(NULLIF(j.seniority, 'unknown'), ai.seniority, 'unknown') = $7::text)
		  AND (cardinality($8::text[]) = 0 OR (j.technologies || COALESCE(ai.technologies, '{}'::text[])) @> $8::text[])
		  AND ($9::float8 <= 0 OR j.first_seen_at >= NOW() - make_interval(secs => $9::float8))
		  AND (cardinality($10::text[]) = 0 OR COALESCE(j.country_code, ai.country_code) = ANY($10::text[]))
		  AND ($11::text = '' OR lower(translate(COALESCE(j.location, ''), 'İIıçÇğĞöÖşŞüÜâÂîÎûÛ', 'iiicCgGoOsSuUaAiIuU')) LIKE $11::text)
		  AND ($12::text = '' OR EXISTS (SELECT 1 FROM job_source_listings l JOIN job_sources s ON s.id = l.source_id
			WHERE l.job_id = j.id AND l.closed_at IS NULL AND s.adapter = $12::text))
		  AND (cardinality($13::uuid[]) = 0 OR j.company_id = ANY($13::uuid[]))
		  AND ($15::bool = false OR (j.first_seen_at, j.id) < ($16::timestamptz, $17::uuid))
		ORDER BY j.first_seen_at DESC, j.id DESC
		LIMIT $14`, append(args, after, seen, id)...)
	}
	if err != nil {
		return nil, false, translate(err, "list workspace jobs")
	}
	out, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.WorkspaceJob, error) {
		var wj domain.WorkspaceJob
		var score *int16
		var reasons []byte
		var isNew *bool
		var matched *time.Time
		dest := append(jobDest(&wj.Job), &wj.Saved, &score, &reasons, &isNew, &matched)
		if err := row.Scan(dest...); err != nil {
			return wj, err
		}
		if score != nil {
			m := &domain.Match{ProfileID: f.ProfileID, JobID: wj.Job.ID, Score: int(*score), New: isNew != nil && *isNew}
			if matched != nil {
				m.FirstMatchedAt = *matched
			}
			if err := json.Unmarshal(reasons, &m.Reasons); err != nil {
				return wj, fmt.Errorf("decode match reasons: %w", err)
			}
			wj.Match = m
		}
		return wj, nil
	})
	if err != nil {
		return nil, false, translate(err, "list workspace jobs")
	}
	if len(out) > f.Limit {
		return out[:f.Limit], true, nil
	}
	return out, false, nil
}

// CursorAfter returns the cursor that continues f after job last.
func CursorAfter(f JobFilter, last domain.WorkspaceJob) JobCursor {
	c := JobCursor{ID: last.Job.ID, FirstSeenAt: last.Job.FirstSeenAt}
	switch f.Sort {
	case "score":
		if last.Match != nil {
			c.Score = last.Match.Score
		}
	case "relevance":
		prev := 0
		if f.After != nil {
			prev = f.After.Offset
		}
		c.Offset = prev + f.Limit
	}
	return c
}

// FacetCount is one value of a facet with how many jobs have it.
type FacetCount struct {
	Value string
	Label string
	Count int
}

// Facets counts the jobs matching a filter by posting age, arrangement,
// seniority, country, technology and company. Each dimension is counted
// with every filter except its own.
type Facets struct {
	Posted       map[string]int
	Arrangement  map[string]int
	Seniority    map[string]int
	Country      map[string]int
	Technologies []FacetCount
	Companies    []FacetCount
}

// facetTTL is how long counts are reused; they scan the whole catalog, and a
// few seconds of staleness does not matter for filter badges.
const facetTTL = 60 * time.Second

func (r *JobRepository) Facets(ctx context.Context, f JobFilter) (Facets, error) {
	key := strings.Join([]string{
		f.Query, f.WorkArrangement, f.EmploymentType, f.Seniority, strings.Join(f.Technologies, ","),
		f.WorkspaceID, f.ProfileID, f.PostedWithin.String(), f.Location, f.ATS,
		strings.Join(f.Countries, ","), strings.Join(f.CompanyIDs, ","), strconv.FormatBool(f.IncludeClosed),
	}, "|")
	r.facetMu.Lock()
	if e, ok := r.facetCache[key]; ok && time.Since(e.at) < facetTTL {
		r.facetMu.Unlock()
		return e.facets, nil
	}
	r.facetMu.Unlock()
	fc, err := r.computeFacets(ctx, f)
	if err != nil {
		return fc, err
	}
	r.facetMu.Lock()
	if len(r.facetCache) > 200 {
		clear(r.facetCache)
	}
	r.facetCache[key] = facetEntry{at: time.Now(), facets: fc}
	r.facetMu.Unlock()
	return fc, nil
}

// computeFacets runs the counts side by side; each is its own scan.
func (r *JobRepository) computeFacets(ctx context.Context, f JobFilter) (Facets, error) {
	fc := Facets{Posted: map[string]int{}, Arrangement: map[string]int{}, Seniority: map[string]int{}, Country: map[string]int{}}
	args := filterArgs(f)
	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		firstErr error
	)
	run := func(fn func() error) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := fn(); err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
			}
		}()
	}
	// counts reads (value, count) rows into dst.
	counts := func(rows pgx.Rows, err error, dst map[string]int, op string) error {
		if err != nil {
			return translate(err, op)
		}
		defer rows.Close()
		found := map[string]int{}
		for rows.Next() {
			var v string
			var n int
			if err := rows.Scan(&v, &n); err != nil {
				return translate(err, op)
			}
			found[v] = n
		}
		mu.Lock()
		maps.Copy(dst, found)
		mu.Unlock()
		return translate(rows.Err(), op)
	}

	run(func() error {
		var d1, d7, d30 int
		if err := r.pool.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE j.first_seen_at >= NOW() - INTERVAL '1 day')::int,
		       count(*) FILTER (WHERE j.first_seen_at >= NOW() - INTERVAL '7 days')::int,
		       count(*) FILTER (WHERE j.first_seen_at >= NOW() - INTERVAL '30 days')::int
		FROM jobs j
		LEFT JOIN job_ai ai ON ai.job_id = j.id
		LEFT JOIN research_matches m ON m.job_id = j.id AND m.profile_id = $3::uuid
		WHERE ($1::bool OR j.closed_at IS NULL)
		  AND ($3::uuid IS NULL OR m.job_id IS NOT NULL)
		  AND ($2::uuid IS NULL OR NOT EXISTS (SELECT 1 FROM workspace_company_blocks b
			WHERE b.workspace_id = $2::uuid AND b.company_id = j.company_id))
		  AND ($4::text = '' OR j.search @@ websearch_to_tsquery('simple', $4::text))
		  AND ($5::text = '' OR COALESCE(NULLIF(j.work_arrangement, 'unknown'), ai.work_arrangement, 'unknown') = $5::text)
		  AND ($6::text = '' OR j.employment_type = $6::text)
		  AND ($7::text = '' OR COALESCE(NULLIF(j.seniority, 'unknown'), ai.seniority, 'unknown') = $7::text)
		  AND (cardinality($8::text[]) = 0 OR (j.technologies || COALESCE(ai.technologies, '{}'::text[])) @> $8::text[])
		  AND $9::float8 IS NOT NULL
		  AND (cardinality($10::text[]) = 0 OR COALESCE(j.country_code, ai.country_code) = ANY($10::text[]))
		  AND ($11::text = '' OR lower(translate(COALESCE(j.location, ''), 'İIıçÇğĞöÖşŞüÜâÂîÎûÛ', 'iiicCgGoOsSuUaAiIuU')) LIKE $11::text)
		  AND ($12::text = '' OR EXISTS (SELECT 1 FROM job_source_listings l JOIN job_sources s ON s.id = l.source_id
			WHERE l.job_id = j.id AND l.closed_at IS NULL AND s.adapter = $12::text))
		  AND (cardinality($13::uuid[]) = 0 OR j.company_id = ANY($13::uuid[]))`, args...).Scan(&d1, &d7, &d30); err != nil {
			return translate(err, "count posted facet")
		}
		mu.Lock()
		fc.Posted["24h"], fc.Posted["7d"], fc.Posted["30d"] = d1, d7, d30
		mu.Unlock()
		return nil
	})
	run(func() error {
		rows, err := r.pool.Query(ctx, `
		SELECT COALESCE(NULLIF(j.work_arrangement, 'unknown'), ai.work_arrangement, 'unknown'), count(*)::int
		FROM jobs j
		LEFT JOIN job_ai ai ON ai.job_id = j.id
		LEFT JOIN research_matches m ON m.job_id = j.id AND m.profile_id = $3::uuid
		WHERE ($1::bool OR j.closed_at IS NULL)
		  AND ($3::uuid IS NULL OR m.job_id IS NOT NULL)
		  AND ($2::uuid IS NULL OR NOT EXISTS (SELECT 1 FROM workspace_company_blocks b
			WHERE b.workspace_id = $2::uuid AND b.company_id = j.company_id))
		  AND ($4::text = '' OR j.search @@ websearch_to_tsquery('simple', $4::text))
		  AND $5::text IS NOT NULL
		  AND ($6::text = '' OR j.employment_type = $6::text)
		  AND ($7::text = '' OR COALESCE(NULLIF(j.seniority, 'unknown'), ai.seniority, 'unknown') = $7::text)
		  AND (cardinality($8::text[]) = 0 OR (j.technologies || COALESCE(ai.technologies, '{}'::text[])) @> $8::text[])
		  AND ($9::float8 <= 0 OR j.first_seen_at >= NOW() - make_interval(secs => $9::float8))
		  AND (cardinality($10::text[]) = 0 OR COALESCE(j.country_code, ai.country_code) = ANY($10::text[]))
		  AND ($11::text = '' OR lower(translate(COALESCE(j.location, ''), 'İIıçÇğĞöÖşŞüÜâÂîÎûÛ', 'iiicCgGoOsSuUaAiIuU')) LIKE $11::text)
		  AND ($12::text = '' OR EXISTS (SELECT 1 FROM job_source_listings l JOIN job_sources s ON s.id = l.source_id
			WHERE l.job_id = j.id AND l.closed_at IS NULL AND s.adapter = $12::text))
		  AND (cardinality($13::uuid[]) = 0 OR j.company_id = ANY($13::uuid[]))
		  AND COALESCE(NULLIF(j.work_arrangement, 'unknown'), ai.work_arrangement, 'unknown') IS NOT NULL
		GROUP BY 1`, args...)
		return counts(rows, err, fc.Arrangement, "count arrangement facet")
	})
	run(func() error {
		rows, err := r.pool.Query(ctx, `
		SELECT COALESCE(NULLIF(j.seniority, 'unknown'), ai.seniority, 'unknown'), count(*)::int
		FROM jobs j
		LEFT JOIN job_ai ai ON ai.job_id = j.id
		LEFT JOIN research_matches m ON m.job_id = j.id AND m.profile_id = $3::uuid
		WHERE ($1::bool OR j.closed_at IS NULL)
		  AND ($3::uuid IS NULL OR m.job_id IS NOT NULL)
		  AND ($2::uuid IS NULL OR NOT EXISTS (SELECT 1 FROM workspace_company_blocks b
			WHERE b.workspace_id = $2::uuid AND b.company_id = j.company_id))
		  AND ($4::text = '' OR j.search @@ websearch_to_tsquery('simple', $4::text))
		  AND ($5::text = '' OR COALESCE(NULLIF(j.work_arrangement, 'unknown'), ai.work_arrangement, 'unknown') = $5::text)
		  AND ($6::text = '' OR j.employment_type = $6::text)
		  AND $7::text IS NOT NULL
		  AND (cardinality($8::text[]) = 0 OR (j.technologies || COALESCE(ai.technologies, '{}'::text[])) @> $8::text[])
		  AND ($9::float8 <= 0 OR j.first_seen_at >= NOW() - make_interval(secs => $9::float8))
		  AND (cardinality($10::text[]) = 0 OR COALESCE(j.country_code, ai.country_code) = ANY($10::text[]))
		  AND ($11::text = '' OR lower(translate(COALESCE(j.location, ''), 'İIıçÇğĞöÖşŞüÜâÂîÎûÛ', 'iiicCgGoOsSuUaAiIuU')) LIKE $11::text)
		  AND ($12::text = '' OR EXISTS (SELECT 1 FROM job_source_listings l JOIN job_sources s ON s.id = l.source_id
			WHERE l.job_id = j.id AND l.closed_at IS NULL AND s.adapter = $12::text))
		  AND (cardinality($13::uuid[]) = 0 OR j.company_id = ANY($13::uuid[]))
		  AND COALESCE(NULLIF(j.seniority, 'unknown'), ai.seniority, 'unknown') IS NOT NULL
		GROUP BY 1`, args...)
		return counts(rows, err, fc.Seniority, "count seniority facet")
	})
	run(func() error {
		rows, err := r.pool.Query(ctx, `
		SELECT COALESCE(j.country_code, ai.country_code), count(*)::int
		FROM jobs j
		LEFT JOIN job_ai ai ON ai.job_id = j.id
		LEFT JOIN research_matches m ON m.job_id = j.id AND m.profile_id = $3::uuid
		WHERE ($1::bool OR j.closed_at IS NULL)
		  AND ($3::uuid IS NULL OR m.job_id IS NOT NULL)
		  AND ($2::uuid IS NULL OR NOT EXISTS (SELECT 1 FROM workspace_company_blocks b
			WHERE b.workspace_id = $2::uuid AND b.company_id = j.company_id))
		  AND ($4::text = '' OR j.search @@ websearch_to_tsquery('simple', $4::text))
		  AND ($5::text = '' OR COALESCE(NULLIF(j.work_arrangement, 'unknown'), ai.work_arrangement, 'unknown') = $5::text)
		  AND ($6::text = '' OR j.employment_type = $6::text)
		  AND ($7::text = '' OR COALESCE(NULLIF(j.seniority, 'unknown'), ai.seniority, 'unknown') = $7::text)
		  AND (cardinality($8::text[]) = 0 OR (j.technologies || COALESCE(ai.technologies, '{}'::text[])) @> $8::text[])
		  AND ($9::float8 <= 0 OR j.first_seen_at >= NOW() - make_interval(secs => $9::float8))
		  AND $10::text[] IS NOT NULL
		  AND ($11::text = '' OR lower(translate(COALESCE(j.location, ''), 'İIıçÇğĞöÖşŞüÜâÂîÎûÛ', 'iiicCgGoOsSuUaAiIuU')) LIKE $11::text)
		  AND ($12::text = '' OR EXISTS (SELECT 1 FROM job_source_listings l JOIN job_sources s ON s.id = l.source_id
			WHERE l.job_id = j.id AND l.closed_at IS NULL AND s.adapter = $12::text))
		  AND (cardinality($13::uuid[]) = 0 OR j.company_id = ANY($13::uuid[]))
		  AND COALESCE(j.country_code, ai.country_code) IS NOT NULL
		GROUP BY 1`, args...)
		return counts(rows, err, fc.Country, "count country facet")
	})
	run(func() error {
		rows, err := r.pool.Query(ctx, `
		SELECT t, t, count(DISTINCT j.id)::int
		FROM jobs j
		LEFT JOIN job_ai ai ON ai.job_id = j.id
		LEFT JOIN research_matches m ON m.job_id = j.id AND m.profile_id = $3::uuid
		CROSS JOIN LATERAL unnest((j.technologies || COALESCE(ai.technologies, '{}'::text[]))) AS t
		WHERE ($1::bool OR j.closed_at IS NULL)
		  AND ($3::uuid IS NULL OR m.job_id IS NOT NULL)
		  AND ($2::uuid IS NULL OR NOT EXISTS (SELECT 1 FROM workspace_company_blocks b
			WHERE b.workspace_id = $2::uuid AND b.company_id = j.company_id))
		  AND ($4::text = '' OR j.search @@ websearch_to_tsquery('simple', $4::text))
		  AND ($5::text = '' OR COALESCE(NULLIF(j.work_arrangement, 'unknown'), ai.work_arrangement, 'unknown') = $5::text)
		  AND ($6::text = '' OR j.employment_type = $6::text)
		  AND ($7::text = '' OR COALESCE(NULLIF(j.seniority, 'unknown'), ai.seniority, 'unknown') = $7::text)
		  AND $8::text[] IS NOT NULL
		  AND ($9::float8 <= 0 OR j.first_seen_at >= NOW() - make_interval(secs => $9::float8))
		  AND (cardinality($10::text[]) = 0 OR COALESCE(j.country_code, ai.country_code) = ANY($10::text[]))
		  AND ($11::text = '' OR lower(translate(COALESCE(j.location, ''), 'İIıçÇğĞöÖşŞüÜâÂîÎûÛ', 'iiicCgGoOsSuUaAiIuU')) LIKE $11::text)
		  AND ($12::text = '' OR EXISTS (SELECT 1 FROM job_source_listings l JOIN job_sources s ON s.id = l.source_id
			WHERE l.job_id = j.id AND l.closed_at IS NULL AND s.adapter = $12::text))
		  AND (cardinality($13::uuid[]) = 0 OR j.company_id = ANY($13::uuid[]))
		GROUP BY t ORDER BY 3 DESC, 1 LIMIT 100`, args...)
		list, err := collectFacet(rows, err, "count technology facet")
		mu.Lock()
		fc.Technologies = list
		mu.Unlock()
		return err
	})
	run(func() error {
		rows, err := r.pool.Query(ctx, `
		SELECT j.company_id::text, min(j.company_name), count(*)::int
		FROM jobs j
		LEFT JOIN job_ai ai ON ai.job_id = j.id
		LEFT JOIN research_matches m ON m.job_id = j.id AND m.profile_id = $3::uuid
		WHERE ($1::bool OR j.closed_at IS NULL)
		  AND ($3::uuid IS NULL OR m.job_id IS NOT NULL)
		  AND ($2::uuid IS NULL OR NOT EXISTS (SELECT 1 FROM workspace_company_blocks b
			WHERE b.workspace_id = $2::uuid AND b.company_id = j.company_id))
		  AND ($4::text = '' OR j.search @@ websearch_to_tsquery('simple', $4::text))
		  AND ($5::text = '' OR COALESCE(NULLIF(j.work_arrangement, 'unknown'), ai.work_arrangement, 'unknown') = $5::text)
		  AND ($6::text = '' OR j.employment_type = $6::text)
		  AND ($7::text = '' OR COALESCE(NULLIF(j.seniority, 'unknown'), ai.seniority, 'unknown') = $7::text)
		  AND (cardinality($8::text[]) = 0 OR (j.technologies || COALESCE(ai.technologies, '{}'::text[])) @> $8::text[])
		  AND ($9::float8 <= 0 OR j.first_seen_at >= NOW() - make_interval(secs => $9::float8))
		  AND (cardinality($10::text[]) = 0 OR COALESCE(j.country_code, ai.country_code) = ANY($10::text[]))
		  AND ($11::text = '' OR lower(translate(COALESCE(j.location, ''), 'İIıçÇğĞöÖşŞüÜâÂîÎûÛ', 'iiicCgGoOsSuUaAiIuU')) LIKE $11::text)
		  AND ($12::text = '' OR EXISTS (SELECT 1 FROM job_source_listings l JOIN job_sources s ON s.id = l.source_id
			WHERE l.job_id = j.id AND l.closed_at IS NULL AND s.adapter = $12::text))
		  AND $13::uuid[] IS NOT NULL
		  AND j.company_id IS NOT NULL
		GROUP BY j.company_id ORDER BY 3 DESC, 2 LIMIT 100`, args...)
		list, err := collectFacet(rows, err, "count company facet")
		mu.Lock()
		fc.Companies = list
		mu.Unlock()
		return err
	})
	wg.Wait()
	return fc, firstErr
}

func collectFacet(rows pgx.Rows, err error, op string) ([]FacetCount, error) {
	if err != nil {
		return nil, translate(err, op)
	}
	out, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (FacetCount, error) {
		var c FacetCount
		err := row.Scan(&c.Value, &c.Label, &c.Count)
		return c, err
	})
	if out == nil {
		out = []FacetCount{}
	}
	return out, translate(err, op)
}
