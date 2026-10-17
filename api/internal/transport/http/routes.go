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
}

type ApplicationStore interface {
	Create(ctx context.Context, workspaceID, jobID string, createdBy *string) (domain.Application, error)
	Get(ctx context.Context, workspaceID, id string) (domain.Application, error)
	List(ctx context.Context, workspaceID string, status *domain.ApplicationStatus) ([]domain.Application, error)
	ChangeStatus(ctx context.Context, workspaceID, id string, to domain.ApplicationStatus, changedBy *string) (domain.Application, error)
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

// WorkspaceAuthorizer decides whether the current request may act in a
// workspace. It must return domain.ErrNotFound when it may not, so callers
// cannot probe for other workspaces. Phase 4 replaces the default
// existence check with a membership check for the authenticated user.
type WorkspaceAuthorizer interface {
	AuthorizeWorkspace(c fiber.Ctx, workspaceID string) error
}

type Deps struct {
	Jobs         JobStore
	Applications ApplicationStore
	Annotations  AnnotationStore
	Workspaces   WorkspaceAuthorizer
}

const workspaceIDLocal = "workspaceID"

// RegisterAPIRoutes mounts the v1 REST API under /api/v1.
func RegisterAPIRoutes(app *fiber.App, d Deps) {
	h := &handlers{Deps: d}
	v1 := app.Group("/api/v1")

	v1.Get("/jobs", h.searchJobs)
	v1.Get("/jobs/:jobID", h.getJob)

	ws := v1.Group("/workspaces/:workspaceID", h.requireWorkspace)
	ws.Get("/saved-jobs", h.listSavedJobs)
	ws.Put("/saved-jobs/:jobID", h.saveJob)
	ws.Delete("/saved-jobs/:jobID", h.unsaveJob)

	ws.Get("/tags", h.listTags)
	ws.Post("/tags", h.createTag)

	ws.Get("/applications", h.listApplications)
	ws.Post("/applications", h.createApplication)
	ws.Get("/applications/:applicationID", h.getApplication)
	ws.Patch("/applications/:applicationID", h.updateApplication)
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
}

type handlers struct {
	Deps
}

func (h *handlers) requireWorkspace(c fiber.Ctx) error {
	id, err := pathID(c, "workspaceID")
	if err != nil {
		return err
	}
	if err := h.Workspaces.AuthorizeWorkspace(c, id); err != nil {
		return err
	}
	c.Locals(workspaceIDLocal, id)
	return c.Next()
}

func workspaceID(c fiber.Ctx) string {
	id, _ := c.Locals(workspaceIDLocal).(string)
	return id
}

// actorID identifies the user performing the request. It is nil until
// authentication is added in Phase 4.
func actorID(fiber.Ctx) *string { return nil }

// ExistingWorkspace is the pre-authentication WorkspaceAuthorizer: any existing
// workspace may be used.
type ExistingWorkspace struct {
	Workspaces interface {
		Get(ctx context.Context, id string) (domain.Workspace, error)
	}
}

func (e ExistingWorkspace) AuthorizeWorkspace(c fiber.Ctx, workspaceID string) error {
	_, err := e.Workspaces.Get(c.Context(), workspaceID)
	return err
}
