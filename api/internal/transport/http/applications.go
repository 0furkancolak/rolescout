package httptransport

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"example.com/rolescout/api/internal/domain"
	"example.com/rolescout/api/internal/infrastructure/database"
)

const (
	maxChannelLength = 100
	maxTagLength     = 50
	maxNoteLength    = 10_000
	maxDetailsBytes  = 4_096
)

// applicationID validates the path ID and confirms the application belongs to
// the request's workspace, so sub-resource lists return 404 instead of an
// empty list for someone else's application.
func (h *handlers) applicationID(c fiber.Ctx) (string, error) {
	id, err := pathID(c, "applicationID")
	if err != nil {
		return "", err
	}
	if _, err := h.Applications.Get(c.Context(), workspaceID(c), id); err != nil {
		return "", err
	}
	return id, nil
}

func (h *handlers) listApplications(c fiber.Ctx) error {
	var status *domain.ApplicationStatus
	if raw := c.Query("status"); raw != "" {
		s := domain.ApplicationStatus(raw)
		if !s.Valid() {
			return (&domain.ValidationError{}).Add("status", "is not a known application status")
		}
		status = &s
	}
	apps, err := h.Applications.List(c.Context(), workspaceID(c), status)
	if err != nil {
		return err
	}
	return c.JSON(dataResponse[[]applicationDTO]{Data: mapSlice(apps, toApplication)})
}

type createApplicationRequest struct {
	JobID string `json:"job_id"`
}

func (h *handlers) createApplication(c fiber.Ctx) error {
	var req createApplicationRequest
	if err := decodeJSON(c, &req); err != nil {
		return err
	}
	if !isUUID(req.JobID) {
		return (&domain.ValidationError{}).Add("job_id", "must be a UUID")
	}
	app, err := h.Applications.Create(c.Context(), workspaceID(c), strings.ToLower(req.JobID), actorID(c))
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(dataResponse[applicationDTO]{Data: toApplication(app)})
}

func (h *handlers) getApplication(c fiber.Ctx) error {
	id, err := pathID(c, "applicationID")
	if err != nil {
		return err
	}
	app, err := h.Applications.Get(c.Context(), workspaceID(c), id)
	if err != nil {
		return err
	}
	return c.JSON(dataResponse[applicationDTO]{Data: toApplication(app)})
}

type updateApplicationRequest struct {
	Status *string `json:"status"`
}

func (h *handlers) updateApplication(c fiber.Ctx) error {
	id, err := pathID(c, "applicationID")
	if err != nil {
		return err
	}
	var req updateApplicationRequest
	if err := decodeJSON(c, &req); err != nil {
		return err
	}
	if req.Status == nil {
		return (&domain.ValidationError{}).Add("status", "is required")
	}
	to := domain.ApplicationStatus(*req.Status)
	if !to.Valid() {
		return (&domain.ValidationError{}).Add("status", "is not a known application status")
	}
	app, err := h.Applications.ChangeStatus(c.Context(), workspaceID(c), id, to, actorID(c))
	if err != nil {
		return err
	}
	return c.JSON(dataResponse[applicationDTO]{Data: toApplication(app)})
}

func (h *handlers) listAttempts(c fiber.Ctx) error {
	id, err := h.applicationID(c)
	if err != nil {
		return err
	}
	attempts, err := h.Applications.ListAttempts(c.Context(), workspaceID(c), id)
	if err != nil {
		return err
	}
	return c.JSON(dataResponse[[]attemptDTO]{Data: mapSlice(attempts, toAttempt)})
}

type addAttemptRequest struct {
	AppliedAt *time.Time `json:"applied_at"`
	Channel   string     `json:"channel"`
}

func (h *handlers) addAttempt(c fiber.Ctx) error {
	id, err := pathID(c, "applicationID")
	if err != nil {
		return err
	}
	var req addAttemptRequest
	if err := decodeJSON(c, &req); err != nil {
		return err
	}
	var v domain.ValidationError
	if req.AppliedAt == nil {
		v.Add("applied_at", "is required")
	} else if req.AppliedAt.After(time.Now().Add(24 * time.Hour)) {
		v.Add("applied_at", "cannot be in the future")
	}
	channel := strings.TrimSpace(req.Channel)
	if len([]rune(channel)) > maxChannelLength {
		v.Add("channel", "must be at most %d characters", maxChannelLength)
	}
	if err := v.Err(); err != nil {
		return err
	}
	attempt, err := h.Applications.AddAttempt(c.Context(), workspaceID(c), id, database.AddAttemptParams{
		AppliedAt: *req.AppliedAt, Channel: channel, CreatedBy: actorID(c),
	})
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(dataResponse[attemptDTO]{Data: toAttempt(attempt)})
}

func (h *handlers) listEvents(c fiber.Ctx) error {
	id, err := h.applicationID(c)
	if err != nil {
		return err
	}
	events, err := h.Applications.ListEvents(c.Context(), workspaceID(c), id)
	if err != nil {
		return err
	}
	return c.JSON(dataResponse[[]eventDTO]{Data: mapSlice(events, toEvent)})
}

type addEventRequest struct {
	Type       string         `json:"type"`
	OccurredAt *time.Time     `json:"occurred_at"`
	AttemptID  *string        `json:"attempt_id"`
	Details    map[string]any `json:"details"`
}

// Only scheduled events can be created directly; created, status_changed and
// attempt_added events are written by the other endpoints.
var userEventTypes = []string{
	string(domain.EventInterview), string(domain.EventFollowUp), string(domain.EventDeadline),
}

func (h *handlers) addEvent(c fiber.Ctx) error {
	id, err := pathID(c, "applicationID")
	if err != nil {
		return err
	}
	var req addEventRequest
	if err := decodeJSON(c, &req); err != nil {
		return err
	}
	var v domain.ValidationError
	if !domain.OneOf(req.Type, userEventTypes) {
		v.Add("type", "must be one of %s", strings.Join(userEventTypes, ", "))
	}
	if req.OccurredAt == nil {
		v.Add("occurred_at", "is required")
	}
	if req.AttemptID != nil && !isUUID(*req.AttemptID) {
		v.Add("attempt_id", "must be a UUID")
	}
	if req.Details != nil {
		if raw, err := json.Marshal(req.Details); err != nil || len(raw) > maxDetailsBytes {
			v.Add("details", "must be a JSON object of at most %d bytes", maxDetailsBytes)
		}
	}
	if err := v.Err(); err != nil {
		return err
	}
	event, err := h.Applications.AddEvent(c.Context(), workspaceID(c), id, database.ScheduleEventParams{
		Type: domain.EventType(req.Type), OccurredAt: *req.OccurredAt, AttemptID: req.AttemptID,
		Details: req.Details, CreatedBy: actorID(c),
	})
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(dataResponse[eventDTO]{Data: toEvent(event)})
}

func (h *handlers) listTags(c fiber.Ctx) error {
	tags, err := h.Annotations.ListTags(c.Context(), workspaceID(c))
	if err != nil {
		return err
	}
	return c.JSON(dataResponse[[]tagDTO]{Data: mapSlice(tags, toTag)})
}

type createTagRequest struct {
	Name string `json:"name"`
}

func (h *handlers) createTag(c fiber.Ctx) error {
	var req createTagRequest
	if err := decodeJSON(c, &req); err != nil {
		return err
	}
	var v domain.ValidationError
	name := requiredText(&v, "name", req.Name, maxTagLength)
	if err := v.Err(); err != nil {
		return err
	}
	tag, err := h.Annotations.CreateTag(c.Context(), workspaceID(c), name)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(dataResponse[tagDTO]{Data: toTag(tag)})
}

func (h *handlers) listApplicationTags(c fiber.Ctx) error {
	id, err := h.applicationID(c)
	if err != nil {
		return err
	}
	tags, err := h.Annotations.ListApplicationTags(c.Context(), workspaceID(c), id)
	if err != nil {
		return err
	}
	return c.JSON(dataResponse[[]tagDTO]{Data: mapSlice(tags, toTag)})
}

func (h *handlers) tagApplication(c fiber.Ctx) error {
	appID, tagID, err := applicationAndTag(c)
	if err != nil {
		return err
	}
	if err := h.Annotations.TagApplication(c.Context(), workspaceID(c), appID, tagID); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *handlers) untagApplication(c fiber.Ctx) error {
	appID, tagID, err := applicationAndTag(c)
	if err != nil {
		return err
	}
	if err := h.Annotations.UntagApplication(c.Context(), workspaceID(c), appID, tagID); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func applicationAndTag(c fiber.Ctx) (string, string, error) {
	appID, err := pathID(c, "applicationID")
	if err != nil {
		return "", "", err
	}
	tagID, err := pathID(c, "tagID")
	return appID, tagID, err
}

func (h *handlers) listNotes(c fiber.Ctx) error {
	id, err := h.applicationID(c)
	if err != nil {
		return err
	}
	notes, err := h.Annotations.ListNotes(c.Context(), workspaceID(c), id)
	if err != nil {
		return err
	}
	return c.JSON(dataResponse[[]noteDTO]{Data: mapSlice(notes, toNote)})
}

type noteRequest struct {
	Body string `json:"body"`
}

func decodeNote(c fiber.Ctx) (string, error) {
	var req noteRequest
	if err := decodeJSON(c, &req); err != nil {
		return "", err
	}
	var v domain.ValidationError
	body := requiredText(&v, "body", req.Body, maxNoteLength)
	return body, v.Err()
}

func (h *handlers) addNote(c fiber.Ctx) error {
	id, err := pathID(c, "applicationID")
	if err != nil {
		return err
	}
	body, err := decodeNote(c)
	if err != nil {
		return err
	}
	note, err := h.Annotations.AddNote(c.Context(), workspaceID(c), id, actorID(c), body)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(dataResponse[noteDTO]{Data: toNote(note)})
}

func (h *handlers) updateNote(c fiber.Ctx) error {
	appID, err := pathID(c, "applicationID")
	if err != nil {
		return err
	}
	noteID, err := pathID(c, "noteID")
	if err != nil {
		return err
	}
	body, err := decodeNote(c)
	if err != nil {
		return err
	}
	note, err := h.Annotations.UpdateNote(c.Context(), workspaceID(c), appID, noteID, body)
	if err != nil {
		return err
	}
	return c.JSON(dataResponse[noteDTO]{Data: toNote(note)})
}
