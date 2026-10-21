package httptransport

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v3"

	"example.com/rolescout/api/internal/domain"
	"example.com/rolescout/api/internal/infrastructure/database"
	"example.com/rolescout/api/internal/llm"
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
	detail := jobDetailDTO{jobDTO: toJob(job), Listings: mapSlice(listings, toListing)}
	if h.Enrichments != nil {
		if facts, err := h.Enrichments.AIFacts(c.Context(), []string{id}); err == nil {
			if a, ok := facts[id]; ok {
				detail.AI = toAI(a)
			}
		}
	}
	return c.JSON(dataResponse[jobDetailDTO]{Data: detail})
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

// JobSummarizer writes short job summaries with the local model.
type JobSummarizer interface {
	Summary(ctx context.Context, jobID, title, company, description, language string) (text string, ready bool, err error)
	Stream(ctx context.Context, jobID, title, company, description, language string, emit func(string)) (string, error)
}

// shortDescription is the length under which a posting counts as having no
// description worth sending to a model.
const shortDescription = 80

func factsSummary(job domain.Job, lang string) string {
	location := ""
	if job.Location != nil {
		location = *job.Location
	}
	return llm.FactsSummary(job.Title, job.CompanyName, location, job.WorkArrangement, job.EmploymentType, lang)
}

type summaryDTO struct {
	Summary *string `json:"summary"`
	// Source is "ai" with a model-written summary, "facts" with a line built from the listing's fields, "pending" while the model is still
	// writing it (ask again), or "unavailable" when the model is off,
	// unreachable or gave an unusable answer.
	Source string `json:"source"`
}

// jobSummary returns a short summary in the requested language (tr or en).
func (h *handlers) jobSummary(c fiber.Ctx) error {
	id, err := pathID(c, "jobID")
	if err != nil {
		return err
	}
	lang := c.Query("lang", "en")
	if lang != "tr" && lang != "en" {
		return (&domain.ValidationError{}).Add("lang", "must be tr or en")
	}
	job, err := h.Jobs.GetJob(c.Context(), id)
	if err != nil {
		return err
	}
	unavailable := dataResponse[summaryDTO]{Data: summaryDTO{Source: "unavailable"}}
	if len(strings.TrimSpace(job.Description)) < shortDescription {
		// List-only sources (kariyer.net) publish no description: say what the
		// source did give instead of nothing, with no model involved.
		if text := factsSummary(job, lang); text != "" {
			return c.JSON(dataResponse[summaryDTO]{Data: summaryDTO{Summary: &text, Source: "facts"}})
		}
		return c.JSON(unavailable)
	}
	if h.Summaries == nil {
		return c.JSON(unavailable)
	}
	text, ready, err := h.Summaries.Summary(c.Context(), id, job.Title, job.CompanyName, job.Description, lang)
	if err != nil || (ready && text == "") {
		return c.JSON(unavailable)
	}
	if !ready {
		return c.JSON(dataResponse[summaryDTO]{Data: summaryDTO{Source: "pending"}})
	}
	return c.JSON(dataResponse[summaryDTO]{Data: summaryDTO{Summary: &text, Source: "ai"}})
}

// jobSummaryStream is jobSummary as server-sent events: "delta" events carry
// the text as the model writes it, then one "done" event with the checked
// summary, or "unavailable" when there is none (the client drops what it showed).
func (h *handlers) jobSummaryStream(c fiber.Ctx) error {
	id, err := pathID(c, "jobID")
	if err != nil {
		return err
	}
	lang := c.Query("lang", "en")
	if lang != "tr" && lang != "en" {
		return (&domain.ValidationError{}).Add("lang", "must be tr or en")
	}
	job, err := h.Jobs.GetJob(c.Context(), id)
	if err != nil {
		return err
	}
	ctx := c.Context()
	c.Set(fiber.HeaderContentType, "text/event-stream")
	c.Set(fiber.HeaderCacheControl, "no-cache")
	c.Set("X-Accel-Buffering", "no")
	return c.SendStreamWriter(func(w *bufio.Writer) {
		send := func(event string, v any) bool {
			b, _ := json.Marshal(v)
			if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b); err != nil {
				return false
			}
			return w.Flush() == nil
		}
		if len(strings.TrimSpace(job.Description)) < shortDescription {
			if text := factsSummary(job, lang); text != "" {
				send("done", map[string]string{"summary": text})
				return
			}
			send("unavailable", map[string]string{})
			return
		}
		if h.Summaries == nil {
			send("unavailable", map[string]string{})
			return
		}
		alive := true
		text, err := h.Summaries.Stream(ctx, id, job.Title, job.CompanyName, job.Description, lang, func(piece string) {
			if alive {
				alive = send("delta", map[string]string{"text": piece})
			}
		})
		if err != nil || text == "" {
			send("unavailable", map[string]string{})
			return
		}
		send("done", map[string]string{"summary": text})
	})
}
