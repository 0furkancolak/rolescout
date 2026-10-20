package httptransport

import (
	"time"

	"example.com/rolescout/api/internal/domain"
	"example.com/rolescout/api/internal/infrastructure/database"
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
	ID              string     `json:"id"`
	Title           string     `json:"title"`
	CompanyName     string     `json:"company_name"`
	CompanyID       *string    `json:"company_id"`
	CountryCode     *string    `json:"country_code"`
	ClosedAt        *time.Time `json:"closed_at"`
	Location        *string    `json:"location"`
	WorkArrangement string     `json:"work_arrangement"`
	EmploymentType  string     `json:"employment_type"`
	Seniority       string     `json:"seniority"`
	Technologies    []string   `json:"technologies"`
	Description     string     `json:"description"`
	FirstSeenAt     time.Time  `json:"first_seen_at"`
	LastSeenAt      time.Time  `json:"last_seen_at"`
}

func toJob(j domain.Job) jobDTO {
	tech := j.Technologies
	if tech == nil {
		tech = []string{}
	}
	return jobDTO{
		ID: j.ID, Title: j.Title, CompanyName: j.CompanyName, CompanyID: j.CompanyID, CountryCode: j.CountryCode,
		ClosedAt: utcPtr(j.ClosedAt), Location: j.Location,
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
	AI       *aiDTO       `json:"ai"`
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

type setupDTO struct {
	RegistrationOpen bool `json:"registration_open"`
}

type userDTO struct {
	ID          string    `json:"id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	Locale      string    `json:"locale"`
	IsAdmin     bool      `json:"is_admin"`
	CreatedAt   time.Time `json:"created_at"`
}

func toUser(u domain.User) userDTO {
	return userDTO{ID: u.ID, Email: u.Email, DisplayName: u.DisplayName, Locale: u.Locale, IsAdmin: u.IsAdmin, CreatedAt: u.CreatedAt.UTC()}
}

type workspaceDTO struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

func toWorkspace(w domain.MemberWorkspace) workspaceDTO {
	return workspaceDTO{ID: w.Workspace.ID, Name: w.Workspace.Name, Role: string(w.Role), CreatedAt: w.Workspace.CreatedAt.UTC()}
}

type memberDTO struct {
	UserID      string    `json:"user_id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	Role        string    `json:"role"`
	JoinedAt    time.Time `json:"joined_at"`
}

func toMember(m domain.WorkspaceMember) memberDTO {
	return memberDTO{
		UserID: m.User.ID, Email: m.User.Email, DisplayName: m.User.DisplayName,
		Role: string(m.Role), JoinedAt: m.CreatedAt.UTC(),
	}
}

type profileDTO struct {
	ID               string     `json:"id"`
	Name             string     `json:"name"`
	TitleKeywords    []string   `json:"title_keywords"`
	Keywords         []string   `json:"keywords"`
	Technologies     []string   `json:"technologies"`
	Countries        []string   `json:"countries"`
	Locations        []string   `json:"locations"`
	ExcludedKeywords []string   `json:"excluded_keywords"`
	Remote           string     `json:"remote"`
	Seniority        []string   `json:"seniority"`
	ScheduleHours    int        `json:"schedule_hours"`
	ScheduleEnabled  bool       `json:"schedule_enabled"`
	LastRunAt        *time.Time `json:"last_run_at"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func toProfile(p domain.ResearchProfile) profileDTO {
	return profileDTO{
		ID: p.ID, Name: p.Name, TitleKeywords: nonNil(p.TitleKeywords), Keywords: nonNil(p.Keywords),
		Technologies: nonNil(p.Technologies), Countries: nonNil(p.Countries), Locations: nonNil(p.Locations),
		ExcludedKeywords: nonNil(p.ExcludedKeywords), Remote: string(p.Remote), Seniority: nonNil(p.Seniority),
		ScheduleHours: int(p.ScheduleInterval.Hours()), ScheduleEnabled: !p.SchedulePaused, LastRunAt: utcPtr(p.LastRunAt),
		CreatedAt: p.CreatedAt.UTC(), UpdatedAt: p.UpdatedAt.UTC(),
	}
}

type stageDTO struct {
	State      string         `json:"state"`
	Total      int            `json:"total"`
	Done       int            `json:"done"`
	Failed     int            `json:"failed"`
	Detail     map[string]int `json:"detail"`
	Note       string         `json:"note"`
	StartedAt  *time.Time     `json:"started_at"`
	FinishedAt *time.Time     `json:"finished_at"`
}

type runDTO struct {
	ID              string              `json:"id"`
	ProfileID       string              `json:"profile_id"`
	State           string              `json:"state"`
	Stage           string              `json:"stage"`
	Stages          map[string]stageDTO `json:"stages"`
	Trigger         string              `json:"trigger"`
	Error           *string             `json:"error"`
	CancelRequested bool                `json:"cancel_requested"`
	Summary         map[string]int      `json:"summary"`
	QueuedAt        time.Time           `json:"queued_at"`
	StartedAt       *time.Time          `json:"started_at"`
	FinishedAt      *time.Time          `json:"finished_at"`
}

func toRun(r domain.ResearchRun) runDTO {
	stages := make(map[string]stageDTO, len(domain.Stages))
	for _, s := range domain.Stages {
		p, ok := r.Stages[s]
		if !ok {
			p = domain.StageProgress{State: "pending"}
		}
		detail := p.Detail
		if detail == nil {
			detail = map[string]int{}
		}
		stages[string(s)] = stageDTO{State: p.State, Total: p.Total, Done: p.Done, Failed: p.Failed, Detail: detail,
			Note: p.Note, StartedAt: utcPtr(p.StartedAt), FinishedAt: utcPtr(p.FinishedAt)}
	}
	summary := r.Summary
	if summary == nil {
		summary = map[string]int{}
	}
	return runDTO{
		ID: r.ID, ProfileID: r.ProfileID, State: string(r.State), Stage: string(r.Stage), Stages: stages,
		Trigger: r.Trigger, Error: r.Error, CancelRequested: r.CancelRequested, Summary: summary,
		QueuedAt: r.QueuedAt.UTC(), StartedAt: utcPtr(r.StartedAt), FinishedAt: utcPtr(r.FinishedAt),
	}
}

type runEventDTO struct {
	ID      int64     `json:"id"`
	At      time.Time `json:"at"`
	Level   string    `json:"level"`
	Stage   string    `json:"stage"`
	Company string    `json:"company"`
	Message string    `json:"message"`
}

func toRunEvent(e domain.RunEvent) runEventDTO {
	return runEventDTO{ID: e.ID, At: e.At.UTC(), Level: e.Level, Stage: string(e.Stage), Company: e.Company, Message: e.Message}
}

type companyDTO struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	CareersURL    string     `json:"careers_url"`
	ATS           string     `json:"ats"`
	Status        string     `json:"status"`
	Origin        string     `json:"origin"`
	OpenJobs      int        `json:"open_jobs"`
	LastCheckedAt *time.Time `json:"last_checked_at"`
	LastError     *string    `json:"last_error"`
	Blocked       bool       `json:"blocked"`
}

func toCompany(c domain.Company) companyDTO {
	return companyDTO{
		ID: c.ID, Name: c.Name, CareersURL: c.CareersURL, ATS: c.ATS, Status: string(c.Status), Origin: c.Origin,
		OpenJobs: c.OpenJobs, LastCheckedAt: utcPtr(c.LastCheckedAt), LastError: c.LastError, Blocked: c.Blocked,
	}
}

type adapterReviewDTO struct {
	Adapter    string     `json:"adapter"`
	Enabled    bool       `json:"enabled"`
	ReviewedAt *time.Time `json:"reviewed_at"`
	Note       *string    `json:"note"`
}

func toAdapterReview(a domain.AdapterReview) adapterReviewDTO {
	return adapterReviewDTO{Adapter: a.Adapter, Enabled: a.Enabled, ReviewedAt: utcPtr(a.ReviewedAt), Note: a.Note}
}

type matchDTO struct {
	Score   int                  `json:"score"`
	New     bool                 `json:"new"`
	Reasons []domain.MatchReason `json:"reasons"`
	// Explanation is a short reason written by the local model, when there is one.
	Explanation *string `json:"explanation"`
}

type workspaceJobDTO struct {
	jobDTO
	Saved bool      `json:"saved"`
	Match *matchDTO `json:"match"`
	AI    *aiDTO    `json:"ai"`
	// TechFit compares the job's technologies with the profile's and the CV's.
	TechFit *techFitDTO `json:"tech_fit"`
}

func toWorkspaceJob(w domain.WorkspaceJob) workspaceJobDTO {
	out := workspaceJobDTO{jobDTO: toJob(w.Job), Saved: w.Saved}
	if w.Match != nil {
		reasons := w.Match.Reasons
		if reasons == nil {
			reasons = []domain.MatchReason{}
		}
		out.Match = &matchDTO{Score: w.Match.Score, New: w.Match.New, Reasons: reasons}
	}
	return out
}

type facetCountDTO struct {
	Value string `json:"value"`
	Label string `json:"label"`
	Count int    `json:"count"`
}

type facetsDTO struct {
	Posted       map[string]int  `json:"posted"`
	Arrangement  map[string]int  `json:"arrangement"`
	Seniority    map[string]int  `json:"seniority"`
	Country      map[string]int  `json:"country"`
	Technologies []facetCountDTO `json:"technologies"`
	Companies    []facetCountDTO `json:"companies"`
}

func toFacets(f database.Facets) facetsDTO {
	conv := func(in []database.FacetCount) []facetCountDTO {
		out := make([]facetCountDTO, len(in))
		for i, c := range in {
			out[i] = facetCountDTO{Value: c.Value, Label: c.Label, Count: c.Count}
		}
		return out
	}
	return facetsDTO{Posted: f.Posted, Arrangement: f.Arrangement, Seniority: f.Seniority, Country: f.Country,
		Technologies: conv(f.Technologies), Companies: conv(f.Companies)}
}
