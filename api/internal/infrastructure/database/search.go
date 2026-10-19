package database

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"

	"example.com/rolescout/api/internal/domain"
)

// MaxRelevanceOffset bounds offset pagination of relevance-sorted results.
const MaxRelevanceOffset = 2000

// sqlQuery collects conditions and numbered arguments.
type sqlQuery struct {
	where []string
	args  []any
}

func (q *sqlQuery) arg(v any) string {
	q.args = append(q.args, v)
	return fmt.Sprintf("$%d", len(q.args))
}

func (q *sqlQuery) cond(format string, vals ...any) {
	refs := make([]any, len(vals))
	for i, v := range vals {
		refs[i] = q.arg(v)
	}
	q.where = append(q.where, fmt.Sprintf(format, refs...))
}

func (q *sqlQuery) whereSQL() string {
	if len(q.where) == 0 {
		return "TRUE"
	}
	return strings.Join(q.where, " AND ")
}

// foldedLocation is the job's location without diacritics, in lower case,
// matching ingest.Fold on the search text.
const foldedLocation = `lower(translate(COALESCE(j.location, ''), 'İIıçÇğĞöÖşŞüÜâÂîÎûÛ', 'iiicCgGoOsSuUaAiIuU'))`

// jobConditions adds the filter's conditions except the dimension named in
// skip ("posted", "arrangement", "seniority", "country", "technology",
// "company"), so facets can count a dimension without its own filter. It
// returns the FROM clause (jobs j, joined with the profile's matches as m).
func jobConditions(q *sqlQuery, f JobFilter, skip string) string {
	from := "jobs j" + aiJoin
	if f.ProfileID != "" {
		from += " JOIN research_matches m ON m.job_id = j.id AND m.profile_id = " + q.arg(f.ProfileID)
	}
	if !f.IncludeClosed {
		q.cond("j.closed_at IS NULL")
	}
	if f.WorkspaceID != "" {
		q.cond(`NOT EXISTS (SELECT 1 FROM workspace_company_blocks b WHERE b.workspace_id = %s AND b.company_id = j.company_id)`, f.WorkspaceID)
	}
	if s := strings.TrimSpace(f.Query); s != "" {
		q.cond("j.search @@ websearch_to_tsquery('simple', %s)", s)
	}
	if f.WorkArrangement != "" && skip != "arrangement" {
		q.cond(effArrangement+" = %s", f.WorkArrangement)
	}
	if f.EmploymentType != "" {
		q.cond("j.employment_type = %s", f.EmploymentType)
	}
	if f.Seniority != "" && skip != "seniority" {
		q.cond(effSeniority+" = %s", f.Seniority)
	}
	if len(f.Technologies) > 0 && skip != "technology" {
		q.cond(effTechnologies+" @> %s::text[]", normalizeTechnologies(f.Technologies))
	}
	if f.PostedWithin > 0 && skip != "posted" {
		q.cond("j.first_seen_at >= NOW() - make_interval(secs => %s)", f.PostedWithin.Seconds())
	}
	if len(f.Countries) > 0 && skip != "country" {
		q.cond(effCountry+" = ANY(%s::text[])", f.Countries)
	}
	if f.Location != "" {
		q.cond(foldedLocation+" LIKE %s", "%"+escapeLike(f.Location)+"%")
	}
	if f.ATS != "" {
		q.cond(`EXISTS (SELECT 1 FROM job_source_listings l JOIN job_sources s ON s.id = l.source_id
			WHERE l.job_id = j.id AND l.closed_at IS NULL AND s.adapter = %s)`, f.ATS)
	}
	if len(f.CompanyIDs) > 0 && skip != "company" {
		q.cond("j.company_id = ANY(%s::uuid[])", f.CompanyIDs)
	}
	return from
}

// WorkspaceJobs returns one page of jobs as a workspace sees them, with the
// saved flag and, when ProfileID is set, each job's match.
func (r *JobRepository) WorkspaceJobs(ctx context.Context, f JobFilter) ([]domain.WorkspaceJob, bool, error) {
	var q sqlQuery
	from := jobConditions(&q, f, "")
	matchCols := "NULL::smallint, NULL::jsonb, NULL::boolean, NULL::timestamptz"
	if f.ProfileID != "" {
		matchCols = "m.score, m.reasons, m.is_new, m.first_matched_at"
	}
	savedCol := "FALSE"
	if f.WorkspaceID != "" {
		savedCol = "EXISTS (SELECT 1 FROM saved_jobs s WHERE s.workspace_id = " + q.arg(f.WorkspaceID) + " AND s.job_id = j.id)"
	}

	var order, offset string
	switch f.Sort {
	case "score":
		order = "m.score DESC, j.id DESC"
		if f.After != nil {
			q.cond("(m.score, j.id) < (%s, %s::uuid)", f.After.Score, f.After.ID)
		}
	case "relevance":
		order = "ts_rank_cd(j.search, websearch_to_tsquery('simple', " + q.arg(strings.TrimSpace(f.Query)) + ")) DESC, j.id DESC"
		if f.After != nil {
			offset = fmt.Sprintf(" OFFSET %d", min(f.After.Offset, MaxRelevanceOffset))
		}
	default:
		order = "j.first_seen_at DESC, j.id DESC"
		if f.After != nil {
			q.cond("(j.first_seen_at, j.id) < (%s, %s::uuid)", f.After.FirstSeenAt, f.After.ID)
		}
	}
	sql := `SELECT ` + effectiveJobColumns() + `, ` + savedCol + `, ` + matchCols + `
		FROM ` + from + ` WHERE ` + q.whereSQL() + ` ORDER BY ` + order +
		` LIMIT ` + q.arg(f.Limit+1) + offset
	rows, err := r.pool.Query(ctx, sql, q.args...)
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
	key := fmt.Sprintf("%s|%s|%s|%s|%v|%s|%s|%v|%s|%s|%v|%v", f.Query, f.WorkArrangement, f.EmploymentType, f.Seniority, f.Technologies,
		f.WorkspaceID, f.ProfileID, f.PostedWithin, f.Location, f.ATS, f.Countries, f.CompanyIDs) + fmt.Sprint(f.IncludeClosed)
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

	run(func() error {
		var q sqlQuery
		from := jobConditions(&q, f, "posted")
		var d1, d7, d30 int
		if err := r.pool.QueryRow(ctx, `
			SELECT count(*) FILTER (WHERE j.first_seen_at >= NOW() - INTERVAL '1 day')::int,
			       count(*) FILTER (WHERE j.first_seen_at >= NOW() - INTERVAL '7 days')::int,
			       count(*) FILTER (WHERE j.first_seen_at >= NOW() - INTERVAL '30 days')::int
			FROM `+from+` WHERE `+q.whereSQL(), q.args...).Scan(&d1, &d7, &d30); err != nil {
			return translate(err, "count posted facet")
		}
		mu.Lock()
		fc.Posted["24h"], fc.Posted["7d"], fc.Posted["30d"] = d1, d7, d30
		mu.Unlock()
		return nil
	})
	for _, dim := range []struct {
		skip, column string
		into         map[string]int
	}{
		{"arrangement", effArrangement, fc.Arrangement},
		{"seniority", effSeniority, fc.Seniority},
		{"country", effCountry, fc.Country},
	} {
		run(func() error {
			var q sqlQuery
			from := jobConditions(&q, f, dim.skip)
			rows, err := r.pool.Query(ctx, `SELECT `+dim.column+`, count(*)::int FROM `+from+
				` WHERE `+q.whereSQL()+` AND `+dim.column+` IS NOT NULL GROUP BY 1`, q.args...)
			if err != nil {
				return translate(err, "count "+dim.skip+" facet")
			}
			defer rows.Close()
			counts := map[string]int{}
			for rows.Next() {
				var v string
				var n int
				if err := rows.Scan(&v, &n); err != nil {
					return translate(err, "count "+dim.skip+" facet")
				}
				counts[v] = n
			}
			mu.Lock()
			maps.Copy(dim.into, counts)
			mu.Unlock()
			return translate(rows.Err(), "count "+dim.skip+" facet")
		})
	}
	run(func() error {
		list, err := r.topFacet(ctx, f, "technology",
			`SELECT t, t, count(DISTINCT j.id)::int FROM %s CROSS JOIN LATERAL unnest(`+effTechnologies+`) AS t WHERE %s GROUP BY t ORDER BY 3 DESC, 1 LIMIT 100`)
		mu.Lock()
		fc.Technologies = list
		mu.Unlock()
		return err
	})
	run(func() error {
		list, err := r.topFacet(ctx, f, "company",
			`SELECT j.company_id::text, min(j.company_name), count(*)::int FROM %s WHERE %s AND j.company_id IS NOT NULL GROUP BY j.company_id ORDER BY 3 DESC, 2 LIMIT 100`)
		mu.Lock()
		fc.Companies = list
		mu.Unlock()
		return err
	})
	wg.Wait()
	return fc, firstErr
}

func (r *JobRepository) topFacet(ctx context.Context, f JobFilter, skip, format string) ([]FacetCount, error) {
	var q sqlQuery
	from := jobConditions(&q, f, skip)
	rows, err := r.pool.Query(ctx, fmt.Sprintf(format, from, q.whereSQL()), q.args...)
	if err != nil {
		return nil, translate(err, "count "+skip+" facet")
	}
	out, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (FacetCount, error) {
		var c FacetCount
		err := row.Scan(&c.Value, &c.Label, &c.Count)
		return c, err
	})
	if out == nil {
		out = []FacetCount{}
	}
	return out, translate(err, "count "+skip+" facet")
}
