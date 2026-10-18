package database

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"example.com/rolescout/api/internal/domain"
)

// ActivityRepository reads the small daily series shown on the dashboard.
type ActivityRepository struct {
	pool *pgxpool.Pool
}

func NewActivityRepository(pool *pgxpool.Pool) *ActivityRepository {
	return &ActivityRepository{pool: pool}
}

// Activity returns the last `days` UTC days ending on the day of now. Matches
// and applications belong to the workspace; jobs are the shared catalog, so
// the jobs series counts jobs first seen that are still open (which lets it
// use the open-jobs index).
func (r *ActivityRepository) Activity(ctx context.Context, workspaceID string, days int, now time.Time) (domain.Activity, error) {
	const q = `
WITH b AS (SELECT (($2::timestamptz AT TIME ZONE 'UTC')::date) AS today),
rng AS (
    SELECT (b.today - ($3::int - 1))::timestamp AT TIME ZONE 'UTC' AS lo,
           (b.today + 1)::timestamp AT TIME ZONE 'UTC' AS hi
    FROM b
),
d AS (
    SELECT g::date AS day
    FROM b, generate_series((b.today - ($3::int - 1))::timestamp, b.today::timestamp, interval '1 day') g
),
nm AS (
    SELECT (m.first_matched_at AT TIME ZONE 'UTC')::date AS day, count(*)::int AS n
    FROM research_matches m
    JOIN research_profiles p ON p.id = m.profile_id, rng
    WHERE p.workspace_id = $1 AND m.first_matched_at >= rng.lo AND m.first_matched_at < rng.hi
    GROUP BY 1
),
jc AS (
    SELECT (j.first_seen_at AT TIME ZONE 'UTC')::date AS day, count(*)::int AS n
    FROM jobs j, rng
    WHERE j.closed_at IS NULL AND j.first_seen_at >= rng.lo AND j.first_seen_at < rng.hi
    GROUP BY 1
),
ac AS (
    SELECT (a.created_at AT TIME ZONE 'UTC')::date AS day, count(*)::int AS n
    FROM applications a, rng
    WHERE a.workspace_id = $1 AND a.created_at >= rng.lo AND a.created_at < rng.hi
    GROUP BY 1
)
SELECT to_char(d.day, 'YYYY-MM-DD'), COALESCE(nm.n, 0), COALESCE(jc.n, 0), COALESCE(ac.n, 0)
FROM d
LEFT JOIN nm USING (day)
LEFT JOIN jc USING (day)
LEFT JOIN ac USING (day)
ORDER BY d.day`
	rows, err := r.pool.Query(ctx, q, workspaceID, now, days)
	if err != nil {
		return domain.Activity{}, err
	}
	defer rows.Close()
	out := domain.Activity{
		Days: make([]string, 0, days), NewMatches: make([]int, 0, days),
		JobsCollected: make([]int, 0, days), ApplicationsCreated: make([]int, 0, days),
	}
	for rows.Next() {
		var day string
		var nm, jc, ac int
		if err := rows.Scan(&day, &nm, &jc, &ac); err != nil {
			return domain.Activity{}, err
		}
		out.Days = append(out.Days, day)
		out.NewMatches = append(out.NewMatches, nm)
		out.JobsCollected = append(out.JobsCollected, jc)
		out.ApplicationsCreated = append(out.ApplicationsCreated, ac)
	}
	if err := rows.Err(); err != nil {
		return domain.Activity{}, err
	}
	err = r.pool.QueryRow(ctx,
		`SELECT count(*)::int FROM application_events WHERE workspace_id = $1 AND type = 'interview' AND occurred_at > $2`,
		workspaceID, now).Scan(&out.UpcomingInterviews)
	return out, err
}
