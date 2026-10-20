package httptransport

import (
	"context"
	"errors"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gofiber/fiber/v3"

	"example.com/rolescout/api/internal/cv"
	"example.com/rolescout/api/internal/domain"
	"example.com/rolescout/api/internal/enrich"
	"example.com/rolescout/api/internal/infrastructure/database"
)

// MemoryStore keeps what the AI knows about a workspace's person: a note and a CV.
type MemoryStore interface {
	Get(ctx context.Context, workspaceID string) (domain.WorkspaceContext, error)
	SaveAbout(ctx context.Context, workspaceID, about string) error
	SaveCV(ctx context.Context, workspaceID string, p database.CVParams, at time.Time) error
	DeleteCV(ctx context.Context, workspaceID string) error
}

// CVAnalyzer reads a CV's text.
type CVAnalyzer interface {
	Read(ctx context.Context, text string) enrich.CVReading
}

const maxAboutRunes = 4000

type cvDTO struct {
	Filename   string    `json:"filename"`
	UploadedAt time.Time `json:"uploaded_at"`
	Chars      int       `json:"chars"`
	Skills     []string  `json:"skills"`
	Titles     []string  `json:"titles"`
	Seniority  string    `json:"seniority"`
	Years      *int      `json:"years"`
	Summary    string    `json:"summary"`
}

type contextDTO struct {
	About string `json:"about"`
	CV    *cvDTO `json:"cv"`
	// AIRead is whether the model read the CV; without it only technologies are known.
	AIRead bool `json:"ai_read"`
}

func toContext(c domain.WorkspaceContext) contextDTO {
	out := contextDTO{About: c.About}
	if c.HasCV() {
		uploaded := time.Time{}
		if c.CVUploadedAt != nil {
			uploaded = c.CVUploadedAt.UTC()
		}
		out.CV = &cvDTO{
			Filename: c.CVFilename, UploadedAt: uploaded, Chars: utf8.RuneCountInString(c.CVText),
			Skills: orEmpty(c.CVSkills), Titles: orEmpty(c.CVTitles), Seniority: c.CVSeniority, Years: c.CVYears, Summary: c.CVSummary,
		}
		out.AIRead = c.CVSummary != ""
	}
	return out
}

func (h *handlers) getContext(c fiber.Ctx) error {
	if h.Memory == nil {
		return domain.ErrNotFound
	}
	m, err := h.Memory.Get(c.Context(), workspaceID(c))
	if err != nil {
		return err
	}
	return c.JSON(dataResponse[contextDTO]{Data: toContext(m)})
}

type contextRequest struct {
	About string `json:"about"`
}

func (h *handlers) putContext(c fiber.Ctx) error {
	if h.Memory == nil {
		return domain.ErrNotFound
	}
	var req contextRequest
	if err := decodeJSON(c, &req); err != nil {
		return err
	}
	about := strings.TrimSpace(req.About)
	if utf8.RuneCountInString(about) > maxAboutRunes {
		return (&domain.ValidationError{}).Add("about", "must be at most %d characters", maxAboutRunes)
	}
	ws := workspaceID(c)
	if err := h.Memory.SaveAbout(c.Context(), ws, about); err != nil {
		return err
	}
	m, err := h.Memory.Get(c.Context(), ws)
	if err != nil {
		return err
	}
	return c.JSON(dataResponse[contextDTO]{Data: toContext(m)})
}

// uploadCV reads an uploaded CV (PDF, Word or text), keeps its text and what
// was read from it, and returns the workspace's context.
func (h *handlers) uploadCV(c fiber.Ctx) error {
	if h.Memory == nil {
		return domain.ErrNotFound
	}
	fh, err := c.FormFile("file")
	if err != nil {
		return (&domain.ValidationError{}).Add("file", "choose a PDF, Word or text file")
	}
	if fh.Size > cv.MaxFileBytes {
		return (&domain.ValidationError{}).Add("file", "must be at most %d MB", cv.MaxFileBytes>>20)
	}
	f, err := fh.Open()
	if err != nil {
		return (&domain.ValidationError{}).Add("file", "the file could not be read")
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, cv.MaxFileBytes+1))
	if err != nil || len(data) > cv.MaxFileBytes {
		return (&domain.ValidationError{}).Add("file", "the file could not be read")
	}
	name := fh.Filename
	if r := []rune(name); len(r) > 200 {
		name = string(r[:200])
	}
	text, err := cv.Extract(name, data)
	switch {
	case err == nil:
	case errors.Is(err, cv.ErrUnsupported):
		return (&domain.ValidationError{}).Add("file", "use a PDF, Word (.docx) or text file")
	case errors.Is(err, cv.ErrNoText):
		return (&domain.ValidationError{}).Add("file", "no text could be read from the file; a scanned image needs to be saved as text first")
	default:
		return (&domain.ValidationError{}).Add("file", "%s", strings.TrimPrefix(err.Error(), "cv: "))
	}
	p := database.CVParams{Filename: name, Text: text}
	if h.CVReader != nil {
		r := h.CVReader.Read(c.Context(), text)
		p.Skills, p.Titles, p.Seniority, p.Summary = r.Skills, r.Titles, r.Seniority, r.Summary
		if r.Years > 0 {
			y := r.Years
			p.Years = &y
		}
	}
	ws := workspaceID(c)
	if err := h.Memory.SaveCV(c.Context(), ws, p, time.Now()); err != nil {
		return err
	}
	m, err := h.Memory.Get(c.Context(), ws)
	if err != nil {
		return err
	}
	return c.JSON(dataResponse[contextDTO]{Data: toContext(m)})
}

func (h *handlers) deleteCV(c fiber.Ctx) error {
	if h.Memory == nil {
		return domain.ErrNotFound
	}
	if err := h.Memory.DeleteCV(c.Context(), workspaceID(c)); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// techFitDTO compares a job's technologies with the person's (profile and CV).
// Missing ones are what the interface marks red.
type techFitDTO struct {
	Matched  []string `json:"matched"`
	Missing  []string `json:"missing"`
	Excluded []string `json:"excluded"`
	// FromCV is whether the CV counted in the comparison.
	FromCV bool `json:"from_cv"`
}

// attachFit adds the technology comparison to job DTOs, best effort. The
// comparison uses the job's technologies including those the model found.
func (h *handlers) attachFit(c fiber.Ctx, profileID string, jobs []workspaceJobDTO) {
	if len(jobs) == 0 {
		return
	}
	var have, excluded []string
	fromCV := false
	if profileID != "" {
		if p, err := h.Research.GetProfile(c.Context(), workspaceID(c), profileID); err == nil {
			have, excluded = append(have, p.Technologies...), p.ExcludedKeywords
		}
	}
	if h.Memory != nil {
		if m, err := h.Memory.Get(c.Context(), workspaceID(c)); err == nil && len(m.CVSkills) > 0 {
			have, fromCV = append(have, m.CVSkills...), true
		}
	}
	if len(have) == 0 {
		return
	}
	for i := range jobs {
		techs := jobs[i].Technologies
		if jobs[i].AI != nil {
			techs = append(append([]string{}, techs...), jobs[i].AI.ExtraTechnologies...)
		}
		fit := domain.FitTechnologies(techs, have, excluded)
		if len(fit.Matched)+len(fit.Missing) == 0 {
			continue
		}
		jobs[i].TechFit = &techFitDTO{Matched: fit.Matched, Missing: fit.Missing, Excluded: fit.Excluded, FromCV: fromCV}
	}
}
