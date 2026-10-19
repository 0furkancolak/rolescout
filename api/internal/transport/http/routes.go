package httptransport

import (
	"context"

	"github.com/gofiber/fiber/v3"

	"example.com/rolescout/api/internal/domain"
	"example.com/rolescout/api/internal/infrastructure/database"
)

// The handlers depend on these narrow interfaces rather than on concrete
// repositories so they can be tested without a database.

type JobStore interface {
	SearchJobs(ctx context.Context, f database.JobFilter) ([]domain.Job, bool, error)
	GetJob(ctx context.Context, id string) (domain.Job, error)
	ListListings(ctx context.Context, jobID string) ([]domain.JobListing, error)
	SaveJob(ctx context.Context, workspaceID, jobID string, savedBy *string) error
	UnsaveJob(ctx context.Context, workspaceID, jobID string) error
	ListSavedJobs(ctx context.Context, workspaceID string) ([]domain.SavedJob, error)
	WorkspaceJobs(ctx context.Context, f database.JobFilter) ([]domain.WorkspaceJob, bool, error)
	Facets(ctx context.Context, f database.JobFilter) (database.Facets, error)
}

type ApplicationStore interface {
	Create(ctx context.Context, workspaceID, jobID string, createdBy *string) (domain.Application, error)
	Get(ctx context.Context, workspaceID, id string) (domain.Application, error)
	List(ctx context.Context, workspaceID string, status *domain.ApplicationStatus) ([]domain.Application, error)
	ChangeStatus(ctx context.Context, workspaceID, id string, to domain.ApplicationStatus, changedBy *string) (domain.Application, error)
	Delete(ctx context.Context, workspaceID, id string) error
	AddAttempt(ctx context.Context, workspaceID, applicationID string, p database.AddAttemptParams) (domain.ApplicationAttempt, error)
	ListAttempts(ctx context.Context, workspaceID, applicationID string) ([]domain.ApplicationAttempt, error)
	AddEvent(ctx context.Context, workspaceID, applicationID string, p database.ScheduleEventParams) (domain.ApplicationEvent, error)
	ListEvents(ctx context.Context, workspaceID, applicationID string) ([]domain.ApplicationEvent, error)
}

type AnnotationStore interface {
	CreateTag(ctx context.Context, workspaceID, name string) (domain.Tag, error)
	ListTags(ctx context.Context, workspaceID string) ([]domain.Tag, error)
	TagApplication(ctx context.Context, workspaceID, applicationID, tagID string) error
	UntagApplication(ctx context.Context, workspaceID, applicationID, tagID string) error
	ListApplicationTags(ctx context.Context, workspaceID, applicationID string) ([]domain.Tag, error)
	AddNote(ctx context.Context, workspaceID, applicationID string, authorID *string, body string) (domain.Note, error)
	UpdateNote(ctx context.Context, workspaceID, applicationID, noteID, body string) (domain.Note, error)
	ListNotes(ctx context.Context, workspaceID, applicationID string) ([]domain.Note, error)
}

// WorkspaceAuthorizer decides whether the authenticated user may act in a
// workspace and returns their role. It must return domain.ErrNotFound when
// they may not, so callers cannot probe for other workspaces.
type WorkspaceAuthorizer interface {
	AuthorizeWorkspace(c fiber.Ctx, workspaceID string) (domain.MembershipRole, error)
}

type Deps struct {
	Auth           AuthStore
	AuthConfig     AuthConfig
	RateLimits     RateLimits
	WorkspaceStore WorkspaceStore
	Jobs           JobStore
	Applications   ApplicationStore
	Annotations    AnnotationStore
	Research       ResearchStore
	Companies      CompanyCatalog
	Activity       ActivityStore
	UserState      UserStateStore
	Summaries      JobSummarizer // nil when the local model is off
	LLM            LLMStatusProvider
	Enrichments    EnrichmentReader
	// CrawlSettings are shown read-only to administrators.
	CrawlSettings []Setting
	Workspaces    WorkspaceAuthorizer
}

const (
	workspaceIDLocal   = "workspaceID"
	workspaceRoleLocal = "workspaceRole"
)

// RegisterAPIRoutes mounts the v1 REST API under /api/v1.
func RegisterAPIRoutes(app *fiber.App, d Deps) {
	h := &handlers{Deps: d}
	v1 := app.Group("/api/v1")

	// Public routes are registered before the authentication middleware.
	v1.Post("/auth/register", rateLimit(d.RateLimits.Register, false), renderErrors, h.register)
	// Only failed sign-ins count, so a user who signs in often is not locked out.
	v1.Post("/auth/login", rateLimit(d.RateLimits.Login, true), renderErrors, h.login)
	v1.Post("/auth/logout", h.logout)

	private := v1.Group("", h.requireUser)
	private.Get("/auth/me", h.me)

	private.Get("/jobs", h.searchJobs)
	private.Get("/jobs/:jobID", h.getJob)

	private.Get("/workspaces", h.listWorkspaces)
	private.Post("/workspaces", h.createWorkspace)
	private.Post("/companies", h.addCompany)

	admin := private.Group("/admin", h.requireAdmin)
	admin.Get("/adapters", h.listAdapterReviews)
	admin.Patch("/adapters/:adapter", h.reviewAdapter)
	admin.Patch("/companies/:companyID", h.setCompanyDisabled)
	admin.Get("/settings", h.crawlSettings)
	admin.Get("/llm", h.llmStatus)
	admin.Post("/llm/check", h.llmCheck)

	ws := private.Group("/workspaces/:workspaceID", h.requireWorkspace)
	ws.Get("/members", h.listMembers)
	ws.Post("/members", h.addMember)
	ws.Get("/jobs", h.listWorkspaceJobs)
	ws.Get("/jobs/facets", h.workspaceJobFacets)
	ws.Get("/jobs/:jobID/summary", h.jobSummary)
	ws.Get("/saved-jobs", h.listSavedJobs)
	ws.Put("/saved-jobs/:jobID", h.saveJob)
	ws.Delete("/saved-jobs/:jobID", h.unsaveJob)

	ws.Get("/state/:key", h.getUserState)
	ws.Put("/state/:key", h.putUserState)

	ws.Get("/tags", h.listTags)
	ws.Post("/tags", h.createTag)

	ws.Get("/applications", h.listApplications)
	ws.Post("/applications", h.createApplication)
	ws.Get("/applications/:applicationID", h.getApplication)
	ws.Patch("/applications/:applicationID", h.updateApplication)
	ws.Delete("/applications/:applicationID", h.deleteApplication)
	ws.Get("/applications/:applicationID/attempts", h.listAttempts)
	ws.Post("/applications/:applicationID/attempts", h.addAttempt)
	ws.Get("/applications/:applicationID/events", h.listEvents)
	ws.Post("/applications/:applicationID/events", h.addEvent)
	ws.Get("/applications/:applicationID/tags", h.listApplicationTags)
	ws.Put("/applications/:applicationID/tags/:tagID", h.tagApplication)
	ws.Delete("/applications/:applicationID/tags/:tagID", h.untagApplication)
	ws.Get("/applications/:applicationID/notes", h.listNotes)
	ws.Post("/applications/:applicationID/notes", h.addNote)
	ws.Patch("/applications/:applicationID/notes/:noteID", h.updateNote)

	ws.Get("/research-profiles", h.listProfiles)
	ws.Post("/research-profiles", h.createProfile)
	ws.Get("/research-profiles/:profileID", h.getProfile)
	ws.Patch("/research-profiles/:profileID", h.updateProfile)
	ws.Delete("/research-profiles/:profileID", h.deleteProfile)
	ws.Post("/research-profiles/:profileID/runs", h.startRun)
	ws.Get("/research-runs", h.listRuns)
	ws.Get("/activity", h.getActivity)
	ws.Get("/research-runs/:runID", h.getRun)
	ws.Get("/research-runs/:runID/events", h.listRunEvents)
	ws.Post("/research-runs/:runID/cancel", h.cancelRun)

	ws.Get("/companies", h.listCompanies)
	ws.Put("/company-blocks/:companyID", h.blockCompany)
	ws.Delete("/company-blocks/:companyID", h.unblockCompany)
}

type handlers struct {
	Deps
}

func (h *handlers) requireWorkspace(c fiber.Ctx) error {
	id, err := pathID(c, "workspaceID")
	if err != nil {
		return err
	}
	role, err := h.Workspaces.AuthorizeWorkspace(c, id)
	if err != nil {
		return err
	}
	c.Locals(workspaceIDLocal, id)
	c.Locals(workspaceRoleLocal, role)
	return c.Next()
}

func workspaceID(c fiber.Ctx) string {
	id, _ := c.Locals(workspaceIDLocal).(string)
	return id
}
