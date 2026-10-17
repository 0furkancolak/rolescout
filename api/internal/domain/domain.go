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
	// IsAdmin marks the installation administrator (the first account).
	IsAdmin   bool
	CreatedAt time.Time
	UpdatedAt time.Time
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
	ID      string
	Key     string
	Name    string
	BaseURL string
	Enabled bool
	// Adapter names the source adapter; Config is its JSON configuration.
	Adapter      string
	Config       map[string]any
	SyncInterval time.Duration
	// ReviewedAt and ReviewNote record the permission and terms review that
	// is required before a source can be enabled.
	ReviewedAt *time.Time
	ReviewNote *string
	// CompanyID links a source created for a discovered company.
	CompanyID *string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type CompanyStatus string

const (
	CompanyPending         CompanyStatus = "pending"
	CompanyActive          CompanyStatus = "active"
	CompanyEmpty           CompanyStatus = "empty"
	CompanyUnreachable     CompanyStatus = "unreachable"
	CompanyBlockedByRobots CompanyStatus = "blocked_by_robots"
	CompanyDisabled        CompanyStatus = "disabled"
)

// Company is an employer with a career page, shared by all workspaces.
type Company struct {
	ID         string
	Name       string
	Domain     *string
	CareersURL string
	// ATS is the detected job board system ("unknown" until detected) and
	// ATSID its board identifier.
	ATS           string
	ATSID         *string
	Origin        string // seed, hackernews, list, link or manual
	Status        CompanyStatus
	LastCheckedAt *time.Time
	LastError     *string
	FailureCount  int
	NextCheckAt   time.Time
	SourceID      *string
	OpenJobs      int
	// Blocked is set when listing for a workspace that blocked the company.
	Blocked   bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// AdapterReview records the access and terms review of a source adapter type.
// Sources of an enabled, reviewed adapter type can be enabled.
type AdapterReview struct {
	Adapter    string
	Enabled    bool
	ReviewedAt *time.Time
	Note       *string
}

type RunStatus string

const (
	RunRunning   RunStatus = "running"
	RunSucceeded RunStatus = "succeeded"
	RunFailed    RunStatus = "failed"
)

// IngestionRun records one synchronization of a source.
type IngestionRun struct {
	ID            string
	SourceID      string
	Status        RunStatus
	StartedAt     time.Time
	FinishedAt    *time.Time
	Attempts      int
	FetchedCount  int
	UpsertedCount int
	FailedCount   int
	Error         *string
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
	CompanyID       *string
	CountryCode     *string
	// ClosedAt is set when no source lists the job any more.
	ClosedAt    *time.Time
	FirstSeenAt time.Time
	LastSeenAt  time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
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
	// ClosedAt is set when the listing was missing from its source's latest
	// successful snapshot.
	ClosedAt *time.Time
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
