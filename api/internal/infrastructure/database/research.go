package database

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"example.com/rolescout/api/internal/domain"
)

// ResearchRepository stores research profiles, runs, run logs, matches and
// discovery progress.
type ResearchRepository struct {
	pool *pgxpool.Pool
}

func NewResearchRepository(pool *pgxpool.Pool) *ResearchRepository {
	return &ResearchRepository{pool: pool}
}

// maxRunEvents is how many log lines are kept per run.
const maxRunEvents = 2000

func scanProfile(row pgx.Row) (domain.ResearchProfile, error) {
	var p domain.ResearchProfile
	var secs int64
	err := row.Scan(&p.ID, &p.WorkspaceID, &p.Name, &p.TitleKeywords, &p.Keywords, &p.Technologies,
		&p.Countries, &p.Locations, &p.ExcludedKeywords, &p.Remote, &p.Seniority, &secs, &p.SchedulePaused,
		&p.LastRunAt, &p.CreatedAt, &p.UpdatedAt)
	p.ScheduleInterval = time.Duration(secs) * time.Second
	return p, err
}

// ProfileParams are the editable fields of a research profile. A zero
// ScheduleInterval means 12 hours.
type ProfileParams struct {
	Name             string
	TitleKeywords    []string
	Keywords         []string
	Technologies     []string
	Countries        []string
	Locations        []string
	ExcludedKeywords []string
	Remote           domain.RemotePreference
	Seniority        []string
	ScheduleInterval time.Duration
	// SchedulePaused stops automatic runs; manual runs still work.
	SchedulePaused bool
}

func (p ProfileParams) args() []any {
	nz := func(s []string) []string {
		if s == nil {
			return []string{}
		}
		return s
	}
	remote := p.Remote
	if remote == "" {
		remote = domain.RemoteAny
	}
	interval := p.ScheduleInterval
	if interval <= 0 {
		interval = 12 * time.Hour
	}
	return []any{p.Name, nz(p.TitleKeywords), nz(p.Keywords), nz(p.Technologies), nz(p.Countries),
		nz(p.Locations), nz(p.ExcludedKeywords), remote, nz(p.Seniority), interval.Seconds(), p.SchedulePaused}
}

func (r *ResearchRepository) CreateProfile(ctx context.Context, workspaceID string, p ProfileParams) (domain.ResearchProfile, error) {
	prof, err := scanProfile(r.pool.QueryRow(ctx, `
		INSERT INTO research_profiles (workspace_id, name, title_keywords, keywords, technologies, countries,
			locations, excluded_keywords, remote, seniority, schedule_interval, schedule_paused)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, make_interval(secs => $11), $12)
		RETURNING id, workspace_id, name, title_keywords, keywords, technologies, countries,
	locations, excluded_keywords, remote, seniority, EXTRACT(EPOCH FROM schedule_interval)::bigint, schedule_paused,
	last_run_at, created_at, updated_at`, append([]any{workspaceID}, p.args()...)...))
	return prof, translate(err, "create research profile")
}

func (r *ResearchRepository) GetProfile(ctx context.Context, workspaceID, id string) (domain.ResearchProfile, error) {
	p, err := scanProfile(r.pool.QueryRow(ctx, `
		SELECT id, workspace_id, name, title_keywords, keywords, technologies, countries,
	locations, excluded_keywords, remote, seniority, EXTRACT(EPOCH FROM schedule_interval)::bigint, schedule_paused,
	last_run_at, created_at, updated_at FROM research_profiles WHERE workspace_id = $1 AND id = $2`, workspaceID, id))
	return p, translate(err, "get research profile")
}

func (r *ResearchRepository) ListProfiles(ctx context.Context, workspaceID string) ([]domain.ResearchProfile, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, workspace_id, name, title_keywords, keywords, technologies, countries,
	locations, excluded_keywords, remote, seniority, EXTRACT(EPOCH FROM schedule_interval)::bigint, schedule_paused,
	last_run_at, created_at, updated_at FROM research_profiles WHERE workspace_id = $1 ORDER BY created_at, id`, workspaceID)
	return collectProfiles(rows, err, "list research profiles")
}

func collectProfiles(rows pgx.Rows, err error, op string) ([]domain.ResearchProfile, error) {
	if err != nil {
		return nil, translate(err, op)
	}
	ps, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.ResearchProfile, error) {
		return scanProfile(row)
	})
	return ps, translate(err, op)
}

func (r *ResearchRepository) UpdateProfile(ctx context.Context, workspaceID, id string, p ProfileParams) (domain.ResearchProfile, error) {
	prof, err := scanProfile(r.pool.QueryRow(ctx, `
		UPDATE research_profiles SET name = $3, title_keywords = $4, keywords = $5, technologies = $6,
			countries = $7, locations = $8, excluded_keywords = $9, remote = $10, seniority = $11,
			schedule_interval = make_interval(secs => $12), schedule_paused = $13
		WHERE workspace_id = $1 AND id = $2
		RETURNING id, workspace_id, name, title_keywords, keywords, technologies, countries,
	locations, excluded_keywords, remote, seniority, EXTRACT(EPOCH FROM schedule_interval)::bigint, schedule_paused,
	last_run_at, created_at, updated_at`, append([]any{workspaceID, id}, p.args()...)...))
	return prof, translate(err, "update research profile")
}

func (r *ResearchRepository) DeleteProfile(ctx context.Context, workspaceID, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM research_profiles WHERE workspace_id = $1 AND id = $2`, workspaceID, id)
	if err != nil {
		return translate(err, "delete research profile")
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("delete research profile: %w", domain.ErrNotFound)
	}
	return nil
}

// DueProfiles returns profiles whose schedule says they should run at now:
// automatic runs are not paused, they have no active run and their last run was queued at least one
// schedule interval ago (or never).
func (r *ResearchRepository) DueProfiles(ctx context.Context, now time.Time) ([]domain.ResearchProfile, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, workspace_id, name, title_keywords, keywords, technologies, countries,
	locations, excluded_keywords, remote, seniority, EXTRACT(EPOCH FROM schedule_interval)::bigint, schedule_paused,
	last_run_at, created_at, updated_at FROM research_profiles p
		WHERE NOT p.schedule_paused
		  AND NOT EXISTS (SELECT 1 FROM research_runs r WHERE r.profile_id = p.id AND r.state IN ('queued', 'running'))
		  AND COALESCE((SELECT max(queued_at) FROM research_runs r WHERE r.profile_id = p.id), '-infinity')
		      + p.schedule_interval <= $1
		ORDER BY p.created_at, p.id`, now)
	return collectProfiles(rows, err, "list due research profiles")
}

func scanResearchRun(row pgx.Row) (domain.ResearchRun, error) {
	var run domain.ResearchRun
	var stages, summary []byte
	err := row.Scan(&run.ID, &run.WorkspaceID, &run.ProfileID, &run.State, &run.Stage, &stages, &summary,
		&run.Trigger, &run.Error, &run.CancelRequested, &run.QueuedAt, &run.StartedAt, &run.FinishedAt)
	if err != nil {
		return run, err
	}
	if err := json.Unmarshal(stages, &run.Stages); err != nil {
		return run, fmt.Errorf("decode run stages: %w", err)
	}
	if err := json.Unmarshal(summary, &run.Summary); err != nil {
		return run, fmt.Errorf("decode run summary: %w", err)
	}
	return run, nil
}

// EnqueueRun queues a run. It returns ErrConflict when the profile already
// has a queued or running run.
func (r *ResearchRepository) EnqueueRun(ctx context.Context, workspaceID, profileID, trigger string) (domain.ResearchRun, error) {
	run, err := scanResearchRun(r.pool.QueryRow(ctx, `
		INSERT INTO research_runs (workspace_id, profile_id, trigger) VALUES ($1, $2, $3)
		RETURNING id, workspace_id, profile_id, state, stage, stages, summary, trigger, error,
	cancel_requested, queued_at, started_at, finished_at`, workspaceID, profileID, trigger))
	return run, translate(err, "enqueue research run")
}

// ClaimRun moves the oldest queued run to running. ok is false when no run
// is queued. Concurrent workers never claim the same run.
func (r *ResearchRepository) ClaimRun(ctx context.Context) (domain.ResearchRun, bool, error) {
	run, err := scanResearchRun(r.pool.QueryRow(ctx, `
		UPDATE research_runs SET state = 'running', started_at = NOW(), updated_at = NOW()
		WHERE id = (
			SELECT id FROM research_runs WHERE state = 'queued'
			ORDER BY queued_at, id FOR UPDATE SKIP LOCKED LIMIT 1
		)
		RETURNING id, workspace_id, profile_id, state, stage, stages, summary, trigger, error,
	cancel_requested, queued_at, started_at, finished_at`))
	if err == pgx.ErrNoRows {
		return run, false, nil
	}
	if err != nil {
		return run, false, translate(err, "claim research run")
	}
	return run, true, nil
}

// SaveProgress stores a running run's stage and stage progress and marks it
// alive (see FailStaleRuns).
func (r *ResearchRepository) SaveProgress(ctx context.Context, run domain.ResearchRun) error {
	stages, err := json.Marshal(nonNilStages(run.Stages))
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `
		UPDATE research_runs SET stage = $2, stages = $3, updated_at = NOW()
		WHERE id = $1 AND state = 'running'`, run.ID, run.Stage, stages)
	return translate(err, "save research run progress")
}

func nonNilStages(s map[domain.Stage]domain.StageProgress) map[domain.Stage]domain.StageProgress {
	if s == nil {
		return map[domain.Stage]domain.StageProgress{}
	}
	return s
}

// FinishRun records a run's final state. A succeeded run also becomes its
// profile's last run, so jobs it collected are not new for the next run.
func (r *ResearchRepository) FinishRun(ctx context.Context, id string, state domain.RunStatus, errMsg *string, summary map[string]int) error {
	if summary == nil {
		summary = map[string]int{}
	}
	sum, err := json.Marshal(summary)
	if err != nil {
		return err
	}
	return inTx(ctx, r.pool, func(tx pgx.Tx) error {
		var profileID string
		err := tx.QueryRow(ctx, `
			UPDATE research_runs SET state = $2, error = $3, summary = $4, finished_at = NOW(), updated_at = NOW(),
				-- A stage still "running" when the run ends did not finish: it becomes
				-- failed (or skipped when the run was cancelled), not a spinner forever.
				stages = CASE WHEN $2::text = 'succeeded' THEN stages ELSE COALESCE((
					SELECT jsonb_object_agg(k, CASE WHEN v->>'state' = 'running'
						THEN jsonb_set(v, '{state}', to_jsonb(CASE WHEN $2::text = 'failed' THEN 'failed' ELSE 'skipped' END::text))
						ELSE v END)
					FROM jsonb_each(stages) AS s(k, v)), stages) END
			WHERE id = $1 AND state IN ('queued', 'running')
			RETURNING profile_id`, id, state, errMsg, sum).Scan(&profileID)
		if err != nil {
			return translate(err, "finish research run")
		}
		if state == domain.RunSucceeded {
			_, err = tx.Exec(ctx, `UPDATE research_profiles SET last_run_at = NOW() WHERE id = $1`, profileID)
		}
		return translate(err, "update profile last run")
	})
}

// RequestCancel cancels a queued run at once and asks a running run to stop.
// It returns ErrConflict for a finished run.
func (r *ResearchRepository) RequestCancel(ctx context.Context, workspaceID, id string) error {
	var state string
	err := r.pool.QueryRow(ctx, `
		WITH target AS (SELECT id, state FROM research_runs WHERE workspace_id = $1 AND id = $2 FOR UPDATE),
		updated AS (
			UPDATE research_runs r SET
				state = CASE WHEN t.state = 'queued' THEN 'cancelled' ELSE r.state END,
				finished_at = CASE WHEN t.state = 'queued' THEN NOW() ELSE r.finished_at END,
				cancel_requested = TRUE, updated_at = NOW()
			FROM target t WHERE r.id = t.id AND t.state IN ('queued', 'running')
			RETURNING r.id
		)
		SELECT t.state FROM target t`, workspaceID, id).Scan(&state)
	if err != nil {
		return translate(err, "cancel research run")
	}
	if state != string(domain.RunQueued) && state != string(domain.RunRunning) {
		return fmt.Errorf("cancel research run: run is %s: %w", state, domain.ErrConflict)
	}
	return nil
}

func (r *ResearchRepository) CancelRequested(ctx context.Context, id string) (bool, error) {
	var c bool
	err := r.pool.QueryRow(ctx, `SELECT cancel_requested FROM research_runs WHERE id = $1`, id).Scan(&c)
	return c, translate(err, "read research run cancel flag")
}

func (r *ResearchRepository) GetRun(ctx context.Context, workspaceID, id string) (domain.ResearchRun, error) {
	run, err := scanResearchRun(r.pool.QueryRow(ctx, `
		SELECT id, workspace_id, profile_id, state, stage, stages, summary, trigger, error,
	cancel_requested, queued_at, started_at, finished_at FROM research_runs WHERE workspace_id = $1 AND id = $2`, workspaceID, id))
	return run, translate(err, "get research run")
}

// ListRuns returns a workspace's runs, newest first; profileID filters when set.
func (r *ResearchRepository) ListRuns(ctx context.Context, workspaceID, profileID string, limit int) ([]domain.ResearchRun, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, workspace_id, profile_id, state, stage, stages, summary, trigger, error,
	cancel_requested, queued_at, started_at, finished_at FROM research_runs
		WHERE workspace_id = $1 AND ($2 = '' OR profile_id::text = $2)
		ORDER BY queued_at DESC, id DESC LIMIT $3`, workspaceID, profileID, limit)
	if err != nil {
		return nil, translate(err, "list research runs")
	}
	runs, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.ResearchRun, error) {
		return scanResearchRun(row)
	})
	return runs, translate(err, "list research runs")
}

// RequeueRun puts a running run back in the queue, as when the worker is
// stopped for an update: the next worker starts it again from the beginning.
func (r *ResearchRepository) RequeueRun(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE research_runs SET state = 'queued', started_at = NULL, stage = 'discovery', stages = '{}',
			error = NULL, updated_at = NOW()
		WHERE id = $1 AND state = 'running' AND NOT cancel_requested`, id)
	return translate(err, "requeue research run")
}

// FailStaleRuns fails running runs that have not reported progress for
// maxAge, such as after a worker crash.
func (r *ResearchRepository) FailStaleRuns(ctx context.Context, maxAge time.Duration) (int64, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE research_runs SET state = 'failed', finished_at = NOW(), updated_at = NOW(),
			stages = COALESCE((
				SELECT jsonb_object_agg(k, CASE WHEN v->>'state' = 'running' THEN jsonb_set(v, '{state}', '"failed"') ELSE v END)
				FROM jsonb_each(stages) AS s(k, v)), stages),
			error = 'the worker stopped while the run was in progress'
		WHERE state = 'running' AND updated_at < NOW() - make_interval(secs => $1)`, maxAge.Seconds())
	if err != nil {
		return 0, translate(err, "fail stale research runs")
	}
	return tag.RowsAffected(), nil
}

// AddEvents appends log lines and keeps the newest 2000 per run.
func (r *ResearchRepository) AddEvents(ctx context.Context, events []domain.RunEvent) error {
	if len(events) == 0 {
		return nil
	}
	return inTx(ctx, r.pool, func(tx pgx.Tx) error {
		rows := make([][]any, len(events))
		runs := map[string]bool{}
		for i, e := range events {
			at := e.At
			if at.IsZero() {
				at = time.Now()
			}
			rows[i] = []any{e.RunID, at, e.Level, string(e.Stage), e.Company, e.Message}
			runs[e.RunID] = true
		}
		if _, err := tx.CopyFrom(ctx, pgx.Identifier{"research_run_events"},
			[]string{"run_id", "at", "level", "stage", "company", "message"}, pgx.CopyFromRows(rows)); err != nil {
			return translate(err, "add research run events")
		}
		for id := range runs {
			if _, err := tx.Exec(ctx, `
				DELETE FROM research_run_events WHERE run_id = $1 AND id < (
					SELECT id FROM research_run_events WHERE run_id = $1 ORDER BY id DESC OFFSET $2 LIMIT 1)`,
				id, maxRunEvents-1); err != nil {
				return translate(err, "trim research run events")
			}
		}
		return nil
	})
}

// ListEvents returns a run's log lines after afterID, oldest first. level
// "warn" returns warnings and errors, "error" only errors.
func (r *ResearchRepository) ListEvents(ctx context.Context, workspaceID, runID string, afterID int64, level string, limit int) ([]domain.RunEvent, error) {
	levels := []string{"info", "warn", "error"}
	switch level {
	case "warn":
		levels = []string{"warn", "error"}
	case "error":
		levels = []string{"error"}
	}
	rows, err := r.pool.Query(ctx, `
		SELECT e.id, e.run_id, e.at, e.level, e.stage, e.company, e.message
		FROM research_run_events e JOIN research_runs r ON r.id = e.run_id
		WHERE r.workspace_id = $1 AND e.run_id = $2 AND e.id > $3 AND e.level = ANY($4)
		ORDER BY e.id LIMIT $5`, workspaceID, runID, afterID, levels, limit)
	if err != nil {
		return nil, translate(err, "list research run events")
	}
	events, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.RunEvent, error) {
		var e domain.RunEvent
		err := row.Scan(&e.ID, &e.RunID, &e.At, &e.Level, &e.Stage, &e.Company, &e.Message)
		return e, err
	})
	return events, translate(err, "list research run events")
}

// matchBatch bounds how many matches one statement writes.
const matchBatch = 5000

// ReplaceMatches stores a profile's matches and removes those not in ms. A
// match is new when its job was first seen after since (always when since
// is nil). It returns how many matches are new.
func (r *ResearchRepository) ReplaceMatches(ctx context.Context, profileID string, ms []domain.Match, since *time.Time) (int, error) {
	newCount := 0
	err := inTx(ctx, r.pool, func(tx pgx.Tx) error {
		ids := make([]string, 0, len(ms))
		for start := 0; start < len(ms); start += matchBatch {
			batch := ms[start:min(start+matchBatch, len(ms))]
			jobIDs := make([]string, len(batch))
			scores := make([]int32, len(batch))
			reasons := make([]string, len(batch))
			for i, m := range batch {
				jobIDs[i], scores[i] = m.JobID, int32(m.Score)
				rs := m.Reasons
				if rs == nil {
					rs = []domain.MatchReason{}
				}
				b, err := json.Marshal(rs)
				if err != nil {
					return err
				}
				reasons[i] = string(b)
			}
			ids = append(ids, jobIDs...)
			var n int
			err := tx.QueryRow(ctx, `
				WITH input AS (
					SELECT * FROM unnest($2::uuid[], $3::int[], $4::jsonb[]) AS t(job_id, score, reasons)
				), upserted AS (
					INSERT INTO research_matches (profile_id, job_id, score, reasons, is_new)
					SELECT $1, i.job_id, i.score, i.reasons, ($5::timestamptz IS NULL OR j.first_seen_at > $5)
					FROM input i JOIN jobs j ON j.id = i.job_id
					ON CONFLICT (profile_id, job_id) DO UPDATE SET
						score = EXCLUDED.score, reasons = EXCLUDED.reasons, is_new = EXCLUDED.is_new
					RETURNING is_new
				)
				SELECT count(*) FILTER (WHERE is_new)::int FROM upserted`,
				profileID, jobIDs, scores, reasons, since).Scan(&n)
			if err != nil {
				return translate(err, "store research matches")
			}
			newCount += n
		}
		_, err := tx.Exec(ctx, `
			DELETE FROM research_matches WHERE profile_id = $1 AND NOT (job_id = ANY($2::uuid[]))`, profileID, ids)
		return translate(err, "remove old research matches")
	})
	return newCount, err
}

// MatchesByJob returns the profile's matches for the given jobs.
func (r *ResearchRepository) MatchesByJob(ctx context.Context, profileID string, jobIDs []string) (map[string]domain.Match, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT profile_id, job_id, score, reasons, is_new, first_matched_at FROM research_matches
		WHERE profile_id = $1 AND job_id = ANY($2::uuid[])`, profileID, jobIDs)
	if err != nil {
		return nil, translate(err, "list research matches")
	}
	defer rows.Close()
	out := map[string]domain.Match{}
	for rows.Next() {
		var m domain.Match
		var reasons []byte
		if err := rows.Scan(&m.ProfileID, &m.JobID, &m.Score, &reasons, &m.New, &m.FirstMatchedAt); err != nil {
			return nil, translate(err, "list research matches")
		}
		if err := json.Unmarshal(reasons, &m.Reasons); err != nil {
			return nil, fmt.Errorf("decode match reasons: %w", err)
		}
		out[m.JobID] = m
	}
	return out, translate(rows.Err(), "list research matches")
}

// PurgeClosedJobs deletes jobs closed longer than olderThan that no
// workspace saved or applied to.
func (r *ResearchRepository) PurgeClosedJobs(ctx context.Context, olderThan time.Duration) (int64, error) {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM jobs j
		WHERE j.closed_at < NOW() - make_interval(secs => $1)
		  AND NOT EXISTS (SELECT 1 FROM saved_jobs s WHERE s.job_id = j.id)
		  AND NOT EXISTS (SELECT 1 FROM applications a WHERE a.job_id = j.id)`, olderThan.Seconds())
	if err != nil {
		return 0, translate(err, "purge closed jobs")
	}
	return tag.RowsAffected(), nil
}

// PurgeRunEvents deletes the logs of runs that finished before olderThan.
func (r *ResearchRepository) PurgeRunEvents(ctx context.Context, olderThan time.Duration) (int64, error) {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM research_run_events e USING research_runs r
		WHERE e.run_id = r.id AND r.finished_at < NOW() - make_interval(secs => $1)`, olderThan.Seconds())
	if err != nil {
		return 0, translate(err, "purge research run events")
	}
	return tag.RowsAffected(), nil
}

// State returns a discovery source's progress marker, or "".
func (r *ResearchRepository) State(ctx context.Context, key string) (string, error) {
	var v string
	err := r.pool.QueryRow(ctx, `SELECT value FROM discovery_state WHERE key = $1`, key).Scan(&v)
	if err == pgx.ErrNoRows {
		return "", nil
	}
	return v, translate(err, "read discovery state")
}

func (r *ResearchRepository) SetState(ctx context.Context, key, value string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO discovery_state (key, value) VALUES ($1, $2)
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()`, key, value)
	return translate(err, "write discovery state")
}

// stageLockKey identifies the advisory lock that serializes the shared
// research stages (discovery, detection, collection) across workers.
const stageLockKey = 7_311_204_118

// TryStageLock takes the shared-stage lock on a dedicated connection. ok is
// false when another run holds it. release unlocks and returns the
// connection; the lock also ends if the connection drops.
func (r *ResearchRepository) TryStageLock(ctx context.Context) (release func(), ok bool, err error) {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return nil, false, translate(err, "acquire connection for stage lock")
	}
	if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock($1)`, stageLockKey).Scan(&ok); err != nil || !ok {
		conn.Release()
		return nil, false, translate(err, "take stage lock")
	}
	return func() {
		_, _ = conn.Exec(context.Background(), `SELECT pg_advisory_unlock($1)`, stageLockKey)
		conn.Release()
	}, true, nil
}
