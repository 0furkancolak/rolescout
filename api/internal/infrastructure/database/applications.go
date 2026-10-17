package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"example.com/rolescout/api/internal/domain"
)

// ApplicationRepository stores applications and their history. Every method
// is scoped to a workspace; rows from other workspaces are reported as
// domain.ErrNotFound.
type ApplicationRepository struct {
	pool *pgxpool.Pool
}

func NewApplicationRepository(pool *pgxpool.Pool) *ApplicationRepository {
	return &ApplicationRepository{pool: pool}
}

const applicationColumns = `id, workspace_id, job_id, status, created_by, created_at, updated_at`

func scanApplication(row pgx.Row) (domain.Application, error) {
	var a domain.Application
	err := row.Scan(&a.ID, &a.WorkspaceID, &a.JobID, &a.Status, &a.CreatedBy, &a.CreatedAt, &a.UpdatedAt)
	return a, err
}

const eventColumns = `id, workspace_id, application_id, attempt_id, type, from_status,
	to_status, occurred_at, details, created_by, created_at`

func scanEvent(row pgx.Row) (domain.ApplicationEvent, error) {
	var e domain.ApplicationEvent
	err := row.Scan(&e.ID, &e.WorkspaceID, &e.ApplicationID, &e.AttemptID, &e.Type,
		&e.FromStatus, &e.ToStatus, &e.OccurredAt, &e.Details, &e.CreatedBy, &e.CreatedAt)
	return e, err
}

type eventInsert struct {
	workspaceID   string
	applicationID string
	attemptID     *string
	eventType     domain.EventType
	fromStatus    *domain.ApplicationStatus
	toStatus      *domain.ApplicationStatus
	occurredAt    time.Time
	details       map[string]any
	createdBy     *string
}

func insertEvent(ctx context.Context, tx pgx.Tx, e eventInsert) (domain.ApplicationEvent, error) {
	details := e.details
	if details == nil {
		details = map[string]any{}
	}
	occurredAt := e.occurredAt
	if occurredAt.IsZero() {
		occurredAt = time.Now()
	}
	ev, err := scanEvent(tx.QueryRow(ctx, `
		INSERT INTO application_events (workspace_id, application_id, attempt_id, type,
			from_status, to_status, occurred_at, details, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING `+eventColumns,
		e.workspaceID, e.applicationID, e.attemptID, e.eventType, e.fromStatus,
		e.toStatus, occurredAt.UTC(), details, e.createdBy,
	))
	return ev, translate(err, "insert application event")
}

// lockApplication loads the application with a row lock so concurrent status
// changes and attempt numbering are serialized.
func lockApplication(ctx context.Context, tx pgx.Tx, workspaceID, id string) (domain.Application, error) {
	a, err := scanApplication(tx.QueryRow(ctx, `
		SELECT `+applicationColumns+` FROM applications
		WHERE workspace_id = $1 AND id = $2 FOR UPDATE`, workspaceID, id))
	return a, translate(err, "lock application")
}

// Create starts tracking a job in a workspace and records a "created" event.
func (r *ApplicationRepository) Create(ctx context.Context, workspaceID, jobID string, createdBy *string) (domain.Application, error) {
	var a domain.Application
	err := inTx(ctx, r.pool, func(tx pgx.Tx) error {
		var err error
		a, err = scanApplication(tx.QueryRow(ctx, `
			INSERT INTO applications (workspace_id, job_id, created_by) VALUES ($1, $2, $3)
			RETURNING `+applicationColumns, workspaceID, jobID, createdBy))
		if err != nil {
			return translate(err, "create application")
		}
		status := a.Status
		_, err = insertEvent(ctx, tx, eventInsert{
			workspaceID: workspaceID, applicationID: a.ID, eventType: domain.EventCreated,
			toStatus: &status, createdBy: createdBy,
		})
		return err
	})
	return a, err
}

func (r *ApplicationRepository) Get(ctx context.Context, workspaceID, id string) (domain.Application, error) {
	a, err := scanApplication(r.pool.QueryRow(ctx, `
		SELECT `+applicationColumns+` FROM applications
		WHERE workspace_id = $1 AND id = $2`, workspaceID, id))
	return a, translate(err, "get application")
}

// List returns the workspace's applications, most recently updated first,
// optionally filtered by status.
func (r *ApplicationRepository) List(ctx context.Context, workspaceID string, status *domain.ApplicationStatus) ([]domain.Application, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+applicationColumns+` FROM applications
		WHERE workspace_id = $1 AND ($2::text IS NULL OR status = $2)
		ORDER BY updated_at DESC, id`, workspaceID, status)
	if err != nil {
		return nil, translate(err, "list applications")
	}
	apps, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Application, error) {
		return scanApplication(row)
	})
	return apps, translate(err, "list applications")
}

// ChangeStatus updates the current status and appends a status_changed event in
// the same transaction. Setting the current status again is a no-op.
func (r *ApplicationRepository) ChangeStatus(ctx context.Context, workspaceID, id string, to domain.ApplicationStatus, changedBy *string) (domain.Application, error) {
	var a domain.Application
	err := inTx(ctx, r.pool, func(tx pgx.Tx) error {
		current, err := lockApplication(ctx, tx, workspaceID, id)
		if err != nil {
			return err
		}
		if current.Status == to {
			a = current
			return nil
		}
		a, err = scanApplication(tx.QueryRow(ctx, `
			UPDATE applications SET status = $3
			WHERE workspace_id = $1 AND id = $2
			RETURNING `+applicationColumns, workspaceID, id, to))
		if err != nil {
			return translate(err, "update application status")
		}
		from := current.Status
		_, err = insertEvent(ctx, tx, eventInsert{
			workspaceID: workspaceID, applicationID: id, eventType: domain.EventStatusChanged,
			fromStatus: &from, toStatus: &to, createdBy: changedBy,
		})
		return err
	})
	return a, err
}

type AddAttemptParams struct {
	AppliedAt time.Time
	Channel   string
	CreatedBy *string
}

// AddAttempt records a new submission with the next attempt number. Earlier
// attempts are never modified.
func (r *ApplicationRepository) AddAttempt(ctx context.Context, workspaceID, applicationID string, p AddAttemptParams) (domain.ApplicationAttempt, error) {
	if p.AppliedAt.IsZero() {
		return domain.ApplicationAttempt{}, fmt.Errorf("add attempt: applied_at is required: %w", domain.ErrInvalid)
	}
	var at domain.ApplicationAttempt
	err := inTx(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := lockApplication(ctx, tx, workspaceID, applicationID); err != nil {
			return err
		}
		err := tx.QueryRow(ctx, `
			INSERT INTO application_attempts (workspace_id, application_id, attempt_number,
				applied_at, channel, created_by)
			SELECT $1, $2, COALESCE(MAX(attempt_number), 0) + 1, $3, $4, $5
			FROM application_attempts WHERE application_id = $2
			RETURNING id, workspace_id, application_id, attempt_number, applied_at,
				channel, created_by, created_at`,
			workspaceID, applicationID, p.AppliedAt.UTC(), p.Channel, p.CreatedBy,
		).Scan(&at.ID, &at.WorkspaceID, &at.ApplicationID, &at.AttemptNumber,
			&at.AppliedAt, &at.Channel, &at.CreatedBy, &at.CreatedAt)
		if err != nil {
			return translate(err, "add application attempt")
		}
		_, err = insertEvent(ctx, tx, eventInsert{
			workspaceID: workspaceID, applicationID: applicationID, attemptID: &at.ID,
			eventType: domain.EventAttemptAdded, occurredAt: at.AppliedAt,
			details: map[string]any{"attempt_number": at.AttemptNumber}, createdBy: p.CreatedBy,
		})
		return err
	})
	return at, err
}

func (r *ApplicationRepository) ListAttempts(ctx context.Context, workspaceID, applicationID string) ([]domain.ApplicationAttempt, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, workspace_id, application_id, attempt_number, applied_at, channel,
			created_by, created_at
		FROM application_attempts
		WHERE workspace_id = $1 AND application_id = $2
		ORDER BY attempt_number`, workspaceID, applicationID)
	if err != nil {
		return nil, translate(err, "list application attempts")
	}
	attempts, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.ApplicationAttempt, error) {
		var at domain.ApplicationAttempt
		err := row.Scan(&at.ID, &at.WorkspaceID, &at.ApplicationID, &at.AttemptNumber,
			&at.AppliedAt, &at.Channel, &at.CreatedBy, &at.CreatedAt)
		return at, err
	})
	return attempts, translate(err, "list application attempts")
}

type ScheduleEventParams struct {
	Type       domain.EventType
	OccurredAt time.Time
	AttemptID  *string
	Details    map[string]any
	CreatedBy  *string
}

// AddEvent records an interview, follow-up or deadline.
func (r *ApplicationRepository) AddEvent(ctx context.Context, workspaceID, applicationID string, p ScheduleEventParams) (domain.ApplicationEvent, error) {
	switch p.Type {
	case domain.EventInterview, domain.EventFollowUp, domain.EventDeadline:
	default:
		return domain.ApplicationEvent{}, fmt.Errorf("add event: type %q is managed by the repository: %w", p.Type, domain.ErrInvalid)
	}
	if p.OccurredAt.IsZero() {
		return domain.ApplicationEvent{}, fmt.Errorf("add event: occurred_at is required: %w", domain.ErrInvalid)
	}
	var ev domain.ApplicationEvent
	err := inTx(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := lockApplication(ctx, tx, workspaceID, applicationID); err != nil {
			return err
		}
		var err error
		ev, err = insertEvent(ctx, tx, eventInsert{
			workspaceID: workspaceID, applicationID: applicationID, attemptID: p.AttemptID,
			eventType: p.Type, occurredAt: p.OccurredAt, details: p.Details, createdBy: p.CreatedBy,
		})
		return err
	})
	return ev, err
}

// ListEvents returns the application's history in chronological order.
func (r *ApplicationRepository) ListEvents(ctx context.Context, workspaceID, applicationID string) ([]domain.ApplicationEvent, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+eventColumns+` FROM application_events
		WHERE workspace_id = $1 AND application_id = $2
		ORDER BY occurred_at, created_at, id`, workspaceID, applicationID)
	if err != nil {
		return nil, translate(err, "list application events")
	}
	events, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.ApplicationEvent, error) {
		return scanEvent(row)
	})
	return events, translate(err, "list application events")
}
