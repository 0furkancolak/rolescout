package httptransport

import (
	"regexp"
	"strings"

	"github.com/gofiber/fiber/v3"

	"example.com/rolescout/api/internal/domain"
)

// Setting is a read-only configuration value shown to administrators.
type Setting struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

var adapterPattern = regexp.MustCompile(`^[a-z0-9_]{1,40}$`)

// requireAdmin lets only the installation administrator through.
func (h *handlers) requireAdmin(c fiber.Ctx) error {
	if u, ok := currentUser(c); !ok || !u.IsAdmin {
		return domain.ErrForbidden
	}
	return c.Next()
}

func (h *handlers) listAdapterReviews(c fiber.Ctx) error {
	reviews, err := h.Companies.AdapterReviews(c.Context())
	if err != nil {
		return err
	}
	return c.JSON(dataResponse[[]adapterReviewDTO]{Data: mapSlice(reviews, toAdapterReview)})
}

type adapterReviewRequest struct {
	Enabled bool   `json:"enabled"`
	Note    string `json:"note"`
}

func (h *handlers) reviewAdapter(c fiber.Ctx) error {
	name := c.Params("adapter")
	if !adapterPattern.MatchString(name) {
		return domain.ErrNotFound
	}
	var req adapterReviewRequest
	if err := decodeJSON(c, &req); err != nil {
		return err
	}
	var v domain.ValidationError
	note := strings.TrimSpace(req.Note)
	if req.Enabled && note == "" {
		v.Add("note", "is required to enable an adapter: say what was reviewed and where the terms are")
	}
	if len([]rune(note)) > 2000 {
		v.Add("note", "must be at most 2000 characters")
	}
	if err := v.Err(); err != nil {
		return err
	}
	r, err := h.Companies.ReviewAdapter(c.Context(), name, note, req.Enabled)
	if err != nil {
		return err
	}
	return c.JSON(dataResponse[adapterReviewDTO]{Data: toAdapterReview(r)})
}

type disableCompanyRequest struct {
	Disabled bool `json:"disabled"`
}

func (h *handlers) setCompanyDisabled(c fiber.Ctx) error {
	id, err := pathID(c, "companyID")
	if err != nil {
		return err
	}
	var req disableCompanyRequest
	if err := decodeJSON(c, &req); err != nil {
		return err
	}
	co, err := h.Companies.SetDisabled(c.Context(), id, req.Disabled)
	if err != nil {
		return err
	}
	return c.JSON(dataResponse[companyDTO]{Data: toCompany(co)})
}

func (h *handlers) crawlSettings(c fiber.Ctx) error {
	settings := h.CrawlSettings
	if settings == nil {
		settings = []Setting{}
	}
	return c.JSON(dataResponse[[]Setting]{Data: settings})
}
