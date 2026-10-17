package httptransport

import (
	"strings"

	"github.com/gofiber/fiber/v3"

	"example.com/rolescout/api/internal/domain"
	"example.com/rolescout/api/internal/infrastructure/database"
)

const (
	maxQueryLength   = 200
	maxTechnologies  = 10
	technologyLength = 50
)

func (h *handlers) searchJobs(c fiber.Ctx) error {
	var v domain.ValidationError
	f := database.JobFilter{
		Query:           strings.TrimSpace(c.Query("q")),
		WorkArrangement: c.Query("work_arrangement"),
		EmploymentType:  c.Query("employment_type"),
		Seniority:       c.Query("seniority"),
		Limit:           parseLimit(c.Query("limit"), &v),
	}
	if len([]rune(f.Query)) > maxQueryLength {
		v.Add("q", "must be at most %d characters", maxQueryLength)
	}
	for _, p := range []struct {
		field, value string
		allowed      []string
	}{
		{"work_arrangement", f.WorkArrangement, domain.WorkArrangements},
		{"employment_type", f.EmploymentType, domain.EmploymentTypes},
		{"seniority", f.Seniority, domain.SeniorityLevels},
	} {
		if p.value != "" && !domain.OneOf(p.value, p.allowed) {
			v.Add(p.field, "must be one of %s", strings.Join(p.allowed, ", "))
		}
	}
	if raw := c.Query("technology"); raw != "" {
		for _, t := range strings.Split(raw, ",") {
			if t = strings.TrimSpace(t); t != "" {
				f.Technologies = append(f.Technologies, t)
			}
		}
		if len(f.Technologies) > maxTechnologies {
			v.Add("technology", "must list at most %d technologies", maxTechnologies)
		}
		for _, t := range f.Technologies {
			if len(t) > technologyLength {
				v.Add("technology", "each technology must be at most %d characters", technologyLength)
				break
			}
		}
	}
	if raw := c.Query("cursor"); raw != "" {
		cursor, err := decodeCursor(raw)
		if err != nil {
			v.Add("cursor", "is not a valid cursor")
		}
		f.After = cursor
	}
	if err := v.Err(); err != nil {
		return err
	}

	jobs, hasMore, err := h.Jobs.SearchJobs(c.Context(), f)
	if err != nil {
		return err
	}
	resp := pageResponse[jobDTO]{Data: mapSlice(jobs, toJob)}
	if hasMore {
		last := jobs[len(jobs)-1]
		next := encodeCursor(database.JobCursor{LastSeenAt: last.LastSeenAt, ID: last.ID})
		resp.NextCursor = &next
	}
	return c.JSON(resp)
}

func (h *handlers) getJob(c fiber.Ctx) error {
	id, err := pathID(c, "jobID")
	if err != nil {
		return err
	}
	job, err := h.Jobs.GetJob(c.Context(), id)
	if err != nil {
		return err
	}
	listings, err := h.Jobs.ListListings(c.Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(dataResponse[jobDetailDTO]{Data: jobDetailDTO{
		jobDTO: toJob(job), Listings: mapSlice(listings, toListing),
	}})
}

func (h *handlers) listSavedJobs(c fiber.Ctx) error {
	saved, err := h.Jobs.ListSavedJobs(c.Context(), workspaceID(c))
	if err != nil {
		return err
	}
	return c.JSON(dataResponse[[]savedJobDTO]{Data: mapSlice(saved, func(s domain.SavedJob) savedJobDTO {
		return savedJobDTO{SavedAt: s.CreatedAt.UTC(), SavedBy: s.SavedBy, Job: toJob(*s.Job)}
	})})
}

// saveJob is idempotent: saving an already saved job succeeds.
func (h *handlers) saveJob(c fiber.Ctx) error {
	jobID, err := pathID(c, "jobID")
	if err != nil {
		return err
	}
	if err := h.Jobs.SaveJob(c.Context(), workspaceID(c), jobID, actorID(c)); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *handlers) unsaveJob(c fiber.Ctx) error {
	jobID, err := pathID(c, "jobID")
	if err != nil {
		return err
	}
	if err := h.Jobs.UnsaveJob(c.Context(), workspaceID(c), jobID); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}
