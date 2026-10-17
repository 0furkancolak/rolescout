package httptransport

import (
	"context"
	"errors"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"example.com/rolescout/api/internal/domain"
	"example.com/rolescout/api/internal/infrastructure/database"
)

type ResearchStore interface {
	CreateProfile(ctx context.Context, workspaceID string, p database.ProfileParams) (domain.ResearchProfile, error)
	GetProfile(ctx context.Context, workspaceID, id string) (domain.ResearchProfile, error)
	ListProfiles(ctx context.Context, workspaceID string) ([]domain.ResearchProfile, error)
	UpdateProfile(ctx context.Context, workspaceID, id string, p database.ProfileParams) (domain.ResearchProfile, error)
	DeleteProfile(ctx context.Context, workspaceID, id string) error
	EnqueueRun(ctx context.Context, workspaceID, profileID, trigger string) (domain.ResearchRun, error)
	GetRun(ctx context.Context, workspaceID, id string) (domain.ResearchRun, error)
	ListRuns(ctx context.Context, workspaceID, profileID string, limit int) ([]domain.ResearchRun, error)
	ListEvents(ctx context.Context, workspaceID, runID string, afterID int64, level string, limit int) ([]domain.RunEvent, error)
	RequestCancel(ctx context.Context, workspaceID, id string) error
}

const (
	maxProfileName   = 100
	maxProfileList   = 30
	maxProfileItem   = 60
	maxScheduleHours = 24 * 14
	maxEventPage     = 500
)

var countryPattern = regexp.MustCompile(`^[A-Z]{2}$`)

// profileRequest is the body of POST and PATCH; PATCH leaves absent fields
// unchanged.
type profileRequest struct {
	Name             *string   `json:"name"`
	TitleKeywords    *[]string `json:"title_keywords"`
	Keywords         *[]string `json:"keywords"`
	Technologies     *[]string `json:"technologies"`
	Countries        *[]string `json:"countries"`
	Locations        *[]string `json:"locations"`
	ExcludedKeywords *[]string `json:"excluded_keywords"`
	Remote           *string   `json:"remote"`
	Seniority        *[]string `json:"seniority"`
	ScheduleHours    *int      `json:"schedule_hours"`
}

// apply merges the request onto base and validates the result.
func (req profileRequest) apply(base database.ProfileParams) (database.ProfileParams, error) {
	var v domain.ValidationError
	p := base
	if req.Name != nil {
		p.Name = *req.Name
	}
	p.Name = requiredText(&v, "name", p.Name, maxProfileName)
	list := func(field string, in *[]string, cur []string, transform func(string) string) []string {
		if in == nil {
			return cur
		}
		var out []string
		for _, s := range *in {
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}
			if transform != nil {
				s = transform(s)
			}
			if len([]rune(s)) > maxProfileItem {
				v.Add(field, "items must be at most %d characters", maxProfileItem)
				return nil
			}
			if !slices.Contains(out, s) {
				out = append(out, s)
			}
		}
		if len(out) > maxProfileList {
			v.Add(field, "must have at most %d items", maxProfileList)
		}
		return out
	}
	p.TitleKeywords = list("title_keywords", req.TitleKeywords, p.TitleKeywords, nil)
	p.Keywords = list("keywords", req.Keywords, p.Keywords, nil)
	p.Technologies = list("technologies", req.Technologies, p.Technologies, strings.ToLower)
	p.Countries = list("countries", req.Countries, p.Countries, strings.ToUpper)
	p.Locations = list("locations", req.Locations, p.Locations, nil)
	p.ExcludedKeywords = list("excluded_keywords", req.ExcludedKeywords, p.ExcludedKeywords, nil)
	p.Seniority = list("seniority", req.Seniority, p.Seniority, strings.ToLower)
	for _, c := range p.Countries {
		if !countryPattern.MatchString(c) {
			v.Add("countries", "must be ISO 3166-1 alpha-2 codes such as TR or DE")
			break
		}
	}
	for _, s := range p.Seniority {
		if s == "unknown" || !domain.OneOf(s, domain.SeniorityLevels) {
			v.Add("seniority", "must contain only intern, junior, mid, senior or lead")
			break
		}
	}
	if req.Remote != nil {
		p.Remote = domain.RemotePreference(*req.Remote)
	}
	if p.Remote == "" {
		p.Remote = domain.RemoteAny
	}
	switch p.Remote {
	case domain.RemoteAny, domain.RemoteOnly, domain.RemoteOrHybrid, domain.OnsiteOK:
	default:
		v.Add("remote", "must be any, remote_only, remote_or_hybrid or onsite_ok")
	}
	if req.ScheduleHours != nil {
		if *req.ScheduleHours < 1 || *req.ScheduleHours > maxScheduleHours {
			v.Add("schedule_hours", "must be between 1 and %d", maxScheduleHours)
		}
		p.ScheduleInterval = time.Duration(*req.ScheduleHours) * time.Hour
	}
	if p.ScheduleInterval <= 0 {
		p.ScheduleInterval = 12 * time.Hour
	}
	return p, v.Err()
}

func paramsOf(p domain.ResearchProfile) database.ProfileParams {
	return database.ProfileParams{
		Name: p.Name, TitleKeywords: p.TitleKeywords, Keywords: p.Keywords, Technologies: p.Technologies,
		Countries: p.Countries, Locations: p.Locations, ExcludedKeywords: p.ExcludedKeywords,
		Remote: p.Remote, Seniority: p.Seniority, ScheduleInterval: p.ScheduleInterval,
	}
}

func (h *handlers) listProfiles(c fiber.Ctx) error {
	ps, err := h.Research.ListProfiles(c.Context(), workspaceID(c))
	if err != nil {
		return err
	}
	return c.JSON(dataResponse[[]profileDTO]{Data: mapSlice(ps, toProfile)})
}

func (h *handlers) createProfile(c fiber.Ctx) error {
	var req profileRequest
	if err := decodeJSON(c, &req); err != nil {
		return err
	}
	params, err := req.apply(database.ProfileParams{})
	if err != nil {
		return err
	}
	p, err := h.Research.CreateProfile(c.Context(), workspaceID(c), params)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(dataResponse[profileDTO]{Data: toProfile(p)})
}

func (h *handlers) getProfile(c fiber.Ctx) error {
	id, err := pathID(c, "profileID")
	if err != nil {
		return err
	}
	p, err := h.Research.GetProfile(c.Context(), workspaceID(c), id)
	if err != nil {
		return err
	}
	return c.JSON(dataResponse[profileDTO]{Data: toProfile(p)})
}

func (h *handlers) updateProfile(c fiber.Ctx) error {
	id, err := pathID(c, "profileID")
	if err != nil {
		return err
	}
	var req profileRequest
	if err := decodeJSON(c, &req); err != nil {
		return err
	}
	current, err := h.Research.GetProfile(c.Context(), workspaceID(c), id)
	if err != nil {
		return err
	}
	params, err := req.apply(paramsOf(current))
	if err != nil {
		return err
	}
	p, err := h.Research.UpdateProfile(c.Context(), workspaceID(c), id, params)
	if err != nil {
		return err
	}
	return c.JSON(dataResponse[profileDTO]{Data: toProfile(p)})
}

func (h *handlers) deleteProfile(c fiber.Ctx) error {
	id, err := pathID(c, "profileID")
	if err != nil {
		return err
	}
	if err := h.Research.DeleteProfile(c.Context(), workspaceID(c), id); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *handlers) startRun(c fiber.Ctx) error {
	id, err := pathID(c, "profileID")
	if err != nil {
		return err
	}
	if _, err := h.Research.GetProfile(c.Context(), workspaceID(c), id); err != nil {
		return err
	}
	run, err := h.Research.EnqueueRun(c.Context(), workspaceID(c), id, "manual")
	if err != nil {
		if errors.Is(err, domain.ErrConflict) {
			return &conflictError{message: "This profile already has a queued or running research run."}
		}
		return err
	}
	return c.Status(fiber.StatusAccepted).JSON(dataResponse[runDTO]{Data: toRun(run)})
}

func (h *handlers) listRuns(c fiber.Ctx) error {
	var v domain.ValidationError
	profile := c.Query("profile")
	if profile != "" && !isUUID(profile) {
		v.Add("profile", "must be a profile ID")
	}
	limit := parseLimit(c.Query("limit"), &v)
	if err := v.Err(); err != nil {
		return err
	}
	runs, err := h.Research.ListRuns(c.Context(), workspaceID(c), profile, limit)
	if err != nil {
		return err
	}
	return c.JSON(dataResponse[[]runDTO]{Data: mapSlice(runs, toRun)})
}

func (h *handlers) getRun(c fiber.Ctx) error {
	id, err := pathID(c, "runID")
	if err != nil {
		return err
	}
	run, err := h.Research.GetRun(c.Context(), workspaceID(c), id)
	if err != nil {
		return err
	}
	return c.JSON(dataResponse[runDTO]{Data: toRun(run)})
}

func (h *handlers) listRunEvents(c fiber.Ctx) error {
	id, err := pathID(c, "runID")
	if err != nil {
		return err
	}
	var v domain.ValidationError
	var after int64
	if raw := c.Query("after"); raw != "" {
		if after, err = strconv.ParseInt(raw, 10, 64); err != nil || after < 0 {
			v.Add("after", "must be an event ID")
		}
	}
	level := c.Query("level")
	if level != "" && level != "warn" && level != "error" {
		v.Add("level", "must be warn or error")
	}
	limit := 200
	if raw := c.Query("limit"); raw != "" {
		if limit, err = strconv.Atoi(raw); err != nil || limit < 1 || limit > maxEventPage {
			v.Add("limit", "must be an integer between 1 and %d", maxEventPage)
		}
	}
	if err := v.Err(); err != nil {
		return err
	}
	events, err := h.Research.ListEvents(c.Context(), workspaceID(c), id, after, level, limit)
	if err != nil {
		return err
	}
	return c.JSON(dataResponse[[]runEventDTO]{Data: mapSlice(events, toRunEvent)})
}

func (h *handlers) cancelRun(c fiber.Ctx) error {
	id, err := pathID(c, "runID")
	if err != nil {
		return err
	}
	if err := h.Research.RequestCancel(c.Context(), workspaceID(c), id); err != nil {
		if errors.Is(err, domain.ErrConflict) {
			return &conflictError{message: "The run has already finished."}
		}
		return err
	}
	run, err := h.Research.GetRun(c.Context(), workspaceID(c), id)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusAccepted).JSON(dataResponse[runDTO]{Data: toRun(run)})
}
