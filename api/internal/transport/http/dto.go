package httptransport

import (
	"time"

	"example.com/rolescout/api/internal/domain"
)

// Response bodies use explicit DTOs so storage fields never leak by accident.

type dataResponse[T any] struct {
	Data T `json:"data"`
}

type pageResponse[T any] struct {
	Data       []T     `json:"data"`
	NextCursor *string `json:"next_cursor"`
}

type jobDTO struct {
	ID              string    `json:"id"`
	Title           string    `json:"title"`
	CompanyName     string    `json:"company_name"`
	Location        *string   `json:"location"`
	WorkArrangement string    `json:"work_arrangement"`
	EmploymentType  string    `json:"employment_type"`
	Seniority       string    `json:"seniority"`
	Technologies    []string  `json:"technologies"`
	Description     string    `json:"description"`
	FirstSeenAt     time.Time `json:"first_seen_at"`
	LastSeenAt      time.Time `json:"last_seen_at"`
}

func toJob(j domain.Job) jobDTO {
	tech := j.Technologies
	if tech == nil {
		tech = []string{}
	}
	return jobDTO{
		ID: j.ID, Title: j.Title, CompanyName: j.CompanyName, Location: j.Location,
		WorkArrangement: j.WorkArrangement, EmploymentType: j.EmploymentType,
		Seniority: j.Seniority, Technologies: tech, Description: j.Description,
		FirstSeenAt: j.FirstSeenAt.UTC(), LastSeenAt: j.LastSeenAt.UTC(),
	}
}

type listingDTO struct {
	ID              string     `json:"id"`
	SourceID        string     `json:"source_id"`
	URL             string     `json:"url"`
	SourcePostedAt  *time.Time `json:"source_posted_at"`
	SourceUpdatedAt *time.Time `json:"source_updated_at"`
	LastIngestedAt  time.Time  `json:"last_ingested_at"`
}

type jobDetailDTO struct {
	jobDTO
	Listings []listingDTO `json:"listings"`
}

func toListing(l domain.JobListing) listingDTO {
	return listingDTO{
		ID: l.ID, SourceID: l.SourceID, URL: l.URL, SourcePostedAt: utcPtr(l.SourcePostedAt),
		SourceUpdatedAt: utcPtr(l.SourceUpdatedAt), LastIngestedAt: l.LastIngestedAt.UTC(),
	}
}

type savedJobDTO struct {
	SavedAt time.Time `json:"saved_at"`
	SavedBy *string   `json:"saved_by"`
	Job     jobDTO    `json:"job"`
}

type applicationDTO struct {
	ID        string    `json:"id"`
	JobID     string    `json:"job_id"`
	Status    string    `json:"status"`
	CreatedBy *string   `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func toApplication(a domain.Application) applicationDTO {
	return applicationDTO{
		ID: a.ID, JobID: a.JobID, Status: string(a.Status), CreatedBy: a.CreatedBy,
		CreatedAt: a.CreatedAt.UTC(), UpdatedAt: a.UpdatedAt.UTC(),
	}
}

type attemptDTO struct {
	ID            string    `json:"id"`
	AttemptNumber int       `json:"attempt_number"`
	AppliedAt     time.Time `json:"applied_at"`
	Channel       string    `json:"channel"`
	CreatedBy     *string   `json:"created_by"`
	CreatedAt     time.Time `json:"created_at"`
}

func toAttempt(a domain.ApplicationAttempt) attemptDTO {
	return attemptDTO{
		ID: a.ID, AttemptNumber: a.AttemptNumber, AppliedAt: a.AppliedAt.UTC(),
		Channel: a.Channel, CreatedBy: a.CreatedBy, CreatedAt: a.CreatedAt.UTC(),
	}
}

type eventDTO struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	AttemptID  *string        `json:"attempt_id"`
	FromStatus *string        `json:"from_status"`
	ToStatus   *string        `json:"to_status"`
	OccurredAt time.Time      `json:"occurred_at"`
	Details    map[string]any `json:"details"`
	CreatedBy  *string        `json:"created_by"`
	CreatedAt  time.Time      `json:"created_at"`
}

func toEvent(e domain.ApplicationEvent) eventDTO {
	details := e.Details
	if details == nil {
		details = map[string]any{}
	}
	return eventDTO{
		ID: e.ID, Type: string(e.Type), AttemptID: e.AttemptID,
		FromStatus: statusPtr(e.FromStatus), ToStatus: statusPtr(e.ToStatus),
		OccurredAt: e.OccurredAt.UTC(), Details: details, CreatedBy: e.CreatedBy,
		CreatedAt: e.CreatedAt.UTC(),
	}
}

type tagDTO struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

func toTag(t domain.Tag) tagDTO {
	return tagDTO{ID: t.ID, Name: t.Name, CreatedAt: t.CreatedAt.UTC()}
}

type noteDTO struct {
	ID        string    `json:"id"`
	AuthorID  *string   `json:"author_id"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func toNote(n domain.Note) noteDTO {
	return noteDTO{
		ID: n.ID, AuthorID: n.AuthorID, Body: n.Body,
		CreatedAt: n.CreatedAt.UTC(), UpdatedAt: n.UpdatedAt.UTC(),
	}
}

func mapSlice[T, U any](in []T, f func(T) U) []U {
	out := make([]U, len(in))
	for i, v := range in {
		out[i] = f(v)
	}
	return out
}

func utcPtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}

func statusPtr(s *domain.ApplicationStatus) *string {
	if s == nil {
		return nil
	}
	v := string(*s)
	return &v
}
