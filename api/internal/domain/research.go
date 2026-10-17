package domain

import "time"

// Research run states extend RunStatus (running, succeeded, failed).
const (
	RunQueued    RunStatus = "queued"
	RunCancelled RunStatus = "cancelled"
)

type RemotePreference string

const (
	RemoteAny      RemotePreference = "any"
	RemoteOnly     RemotePreference = "remote_only"
	RemoteOrHybrid RemotePreference = "remote_or_hybrid"
	OnsiteOK       RemotePreference = "onsite_ok"
)

// ResearchProfile describes what a workspace is looking for.
type ResearchProfile struct {
	ID               string
	WorkspaceID      string
	Name             string
	TitleKeywords    []string
	Keywords         []string
	Technologies     []string
	Countries        []string // ISO 3166-1 alpha-2
	Locations        []string
	ExcludedKeywords []string
	Remote           RemotePreference
	Seniority        []string
	ScheduleInterval time.Duration
	// LastRunAt is when the last successful run started.
	LastRunAt *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Stage string

const (
	StageDiscovery  Stage = "discovery"
	StageDetection  Stage = "detection"
	StageCollection Stage = "collection"
	StageScoring    Stage = "scoring"
	StageResult     Stage = "result"
)

// Stages lists research stages in order.
var Stages = []Stage{StageDiscovery, StageDetection, StageCollection, StageScoring, StageResult}

// StageProgress is the state of one stage of a run. State is pending,
// running, done, skipped or failed.
type StageProgress struct {
	State      string         `json:"state"`
	Total      int            `json:"total"`
	Done       int            `json:"done"`
	Failed     int            `json:"failed"`
	Detail     map[string]int `json:"detail,omitempty"`
	StartedAt  *time.Time     `json:"startedAt,omitempty"`
	FinishedAt *time.Time     `json:"finishedAt,omitempty"`
	Note       string         `json:"note,omitempty"`
}

// ResearchRun is one execution of the research pipeline for a profile.
type ResearchRun struct {
	ID              string
	WorkspaceID     string
	ProfileID       string
	State           RunStatus
	Stage           Stage
	Stages          map[Stage]StageProgress
	Trigger         string // manual or schedule
	Error           *string
	CancelRequested bool
	QueuedAt        time.Time
	StartedAt       *time.Time
	FinishedAt      *time.Time
	Summary         map[string]int
}

// RunEvent is one line of a run's log.
type RunEvent struct {
	ID      int64
	RunID   string
	At      time.Time
	Level   string // info, warn or error
	Stage   Stage
	Company string
	Message string
}

// MatchReason explains part of a match score, such as {Code: "tech",
// Params: {"matched": "5", "total": "6"}}.
type MatchReason struct {
	Code   string            `json:"code"`
	Params map[string]string `json:"params,omitempty"`
}

// Match is a job's score for a research profile.
type Match struct {
	ProfileID      string
	JobID          string
	Score          int
	Reasons        []MatchReason
	New            bool
	FirstMatchedAt time.Time
}
