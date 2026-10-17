package domain

import (
	"fmt"
	"strings"
)

// FieldError describes one invalid input field.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationError reports invalid input. errors.Is(err, ErrInvalid) is true.
type ValidationError struct {
	Fields []FieldError
}

func (e *ValidationError) Error() string {
	parts := make([]string, len(e.Fields))
	for i, f := range e.Fields {
		parts[i] = f.Field + ": " + f.Message
	}
	return "invalid input: " + strings.Join(parts, "; ")
}

func (e *ValidationError) Is(target error) bool { return target == ErrInvalid }

// Add records a field error and returns the receiver for chaining.
func (e *ValidationError) Add(field, format string, args ...any) *ValidationError {
	e.Fields = append(e.Fields, FieldError{Field: field, Message: fmt.Sprintf(format, args...)})
	return e
}

// Err returns nil when no field errors were recorded.
func (e *ValidationError) Err() error {
	if len(e.Fields) == 0 {
		return nil
	}
	return e
}

var (
	ApplicationStatuses = []ApplicationStatus{
		StatusPlanned, StatusApplied, StatusInterviewing, StatusOffer,
		StatusAccepted, StatusRejected, StatusWithdrawn,
	}
	WorkArrangements = []string{"onsite", "hybrid", "remote", "unknown"}
	EmploymentTypes  = []string{"full_time", "part_time", "contract", "internship", "temporary", "unknown"}
	SeniorityLevels  = []string{"intern", "junior", "mid", "senior", "lead", "unknown"}
)

func (s ApplicationStatus) Valid() bool {
	for _, v := range ApplicationStatuses {
		if s == v {
			return true
		}
	}
	return false
}

// statusTransitions lists the statuses each status may move to. Rejected and
// withdrawn applications can be reopened because re-applying is a normal part
// of a job search; accepted is final except for withdrawing.
var statusTransitions = map[ApplicationStatus][]ApplicationStatus{
	StatusPlanned:      {StatusApplied, StatusWithdrawn},
	StatusApplied:      {StatusInterviewing, StatusOffer, StatusRejected, StatusWithdrawn},
	StatusInterviewing: {StatusOffer, StatusRejected, StatusWithdrawn},
	StatusOffer:        {StatusAccepted, StatusRejected, StatusWithdrawn},
	StatusAccepted:     {StatusWithdrawn},
	StatusRejected:     {StatusPlanned, StatusApplied},
	StatusWithdrawn:    {StatusPlanned, StatusApplied},
}

// ErrTransitionNotAllowed is returned for a status change the workflow forbids.
// errors.Is(err, ErrConflict) is true because it depends on the current state.
type ErrTransitionNotAllowed struct {
	From, To ApplicationStatus
}

func (e ErrTransitionNotAllowed) Error() string {
	return fmt.Sprintf("status cannot change from %s to %s", e.From, e.To)
}

func (e ErrTransitionNotAllowed) Is(target error) bool { return target == ErrConflict }

// ValidateTransition checks a status change. Keeping the same status is allowed.
func ValidateTransition(from, to ApplicationStatus) error {
	if !to.Valid() {
		return (&ValidationError{}).Add("status", "must be one of %s", joinStatuses())
	}
	if from == to {
		return nil
	}
	for _, next := range statusTransitions[from] {
		if next == to {
			return nil
		}
	}
	return ErrTransitionNotAllowed{From: from, To: to}
}

// StatusAfterAttempt returns the status an application moves to when a new
// application attempt is recorded, and whether it changes.
func StatusAfterAttempt(current ApplicationStatus) (ApplicationStatus, bool) {
	switch current {
	case StatusPlanned, StatusRejected, StatusWithdrawn:
		return StatusApplied, true
	default:
		return current, false
	}
}

func joinStatuses() string {
	parts := make([]string, len(ApplicationStatuses))
	for i, s := range ApplicationStatuses {
		parts[i] = string(s)
	}
	return strings.Join(parts, ", ")
}

// OneOf reports whether value is in allowed.
func OneOf(value string, allowed []string) bool {
	for _, a := range allowed {
		if value == a {
			return true
		}
	}
	return false
}
