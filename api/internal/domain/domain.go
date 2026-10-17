// Package domain holds RoleScout's core entities and the errors repositories
// report. It has no infrastructure dependencies.
package domain

import (
	"errors"
	"time"
)

var (
	// ErrNotFound means the entity, or an entity it references, does not exist
	// in the requested workspace.
	ErrNotFound = errors.New("not found")
	// ErrConflict means a uniqueness rule was violated.
	ErrConflict = errors.New("conflict")
	// ErrInvalid means the input broke a data constraint.
	ErrInvalid = errors.New("invalid input")
	// ErrForbidden means the user may see the resource but not perform the action.
	ErrForbidden = errors.New("forbidden")
	// ErrRegistrationClosed means new accounts are disabled on this installation.
	ErrRegistrationClosed = errors.New("registration closed")
)

type MembershipRole string

const (
	RoleOwner  MembershipRole = "owner"
	RoleMember MembershipRole = "member"
)

type User struct {
	ID          string
	Email       string
	DisplayName string
	Locale      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Workspace struct {
	ID        string
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type WorkspaceMembership struct {
	WorkspaceID string
	UserID      string
	Role        MembershipRole
	CreatedAt   time.Time
}

// WorkspaceMember is a membership together with the member's profile.
type WorkspaceMember struct {
	User      User
	Role      MembershipRole
	CreatedAt time.Time
}

// MemberWorkspace is a workspace as seen by one of its members.
type MemberWorkspace struct {
	Workspace Workspace
	Role      MembershipRole
}

type JobSource struct {
	ID        string
	Key       string
	Name      string
	BaseURL   string
	Enabled   bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Job struct {
	ID              string
	DedupeKey       string
	Title           string
	CompanyName     string
	Location        *string
	WorkArrangement string
	EmploymentType  string
	Seniority       string
	Technologies    []string
	Description     string
	FirstSeenAt     time.Time
	LastSeenAt      time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// JobListing is one occurrence of a job on a source.
type JobListing struct {
	ID              string
	JobID           string
	SourceID        string
	ExternalID      string
	URL             string
	SourcePostedAt  *time.Time
	SourceUpdatedAt *time.Time
	FirstIngestedAt time.Time
	LastIngestedAt  time.Time
}

type SavedJob struct {
	WorkspaceID string
	JobID       string
	SavedBy     *string
	CreatedAt   time.Time
	// Job is populated by list queries.
	Job *Job
}

type ApplicationStatus string

const (
	StatusPlanned      ApplicationStatus = "planned"
	StatusApplied      ApplicationStatus = "applied"
	StatusInterviewing ApplicationStatus = "interviewing"
	StatusOffer        ApplicationStatus = "offer"
	StatusAccepted     ApplicationStatus = "accepted"
	StatusRejected     ApplicationStatus = "rejected"
	StatusWithdrawn    ApplicationStatus = "withdrawn"
)

type Application struct {
	ID          string
	WorkspaceID string
	JobID       string
	Status      ApplicationStatus
	CreatedBy   *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type ApplicationAttempt struct {
	ID            string
	WorkspaceID   string
	ApplicationID string
	AttemptNumber int
	AppliedAt     time.Time
	Channel       string
	CreatedBy     *string
	CreatedAt     time.Time
}

type EventType string

const (
	EventCreated       EventType = "created"
	EventStatusChanged EventType = "status_changed"
	EventAttemptAdded  EventType = "attempt_added"
	EventInterview     EventType = "interview"
	EventFollowUp      EventType = "follow_up"
	EventDeadline      EventType = "deadline"
)

// ApplicationEvent is an append-only history entry for an application.
type ApplicationEvent struct {
	ID            string
	WorkspaceID   string
	ApplicationID string
	AttemptID     *string
	Type          EventType
	FromStatus    *ApplicationStatus
	ToStatus      *ApplicationStatus
	OccurredAt    time.Time
	Details       map[string]any
	CreatedBy     *string
	CreatedAt     time.Time
}

type Tag struct {
	ID          string
	WorkspaceID string
	Name        string
	CreatedAt   time.Time
}

type Note struct {
	ID            string
	WorkspaceID   string
	ApplicationID string
	AuthorID      *string
	Body          string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
