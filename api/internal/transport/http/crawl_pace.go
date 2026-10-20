package httptransport

import (
	"context"
	"errors"

	"github.com/gofiber/fiber/v3"

	"example.com/rolescout/api/internal/crawl"
	"example.com/rolescout/api/internal/crawlconfig"
	"example.com/rolescout/api/internal/domain"
)

// CrawlPaceStore reads and edits how fast the crawl runs.
type CrawlPaceStore interface {
	Current(ctx context.Context) crawl.Tuning
	Defaults() crawl.Tuning
	Stored(ctx context.Context) (map[string]bool, error)
	Save(ctx context.Context, u crawlconfig.Update) error
}

type crawlPaceValues struct {
	Concurrency       int `json:"concurrency"`
	HostIntervalMS    int `json:"host_interval_ms"`
	VendorConcurrency int `json:"vendor_concurrency"`
	VendorIntervalMS  int `json:"vendor_interval_ms"`
}

type crawlPaceDTO struct {
	crawlPaceValues
	// Defaults are what the environment asks for; clearing a field returns to them.
	Defaults crawlPaceValues `json:"defaults"`
	// Stored lists the values saved in the web app.
	Stored map[string]bool `json:"stored"`
}

func paceValues(t crawl.Tuning) crawlPaceValues {
	return crawlPaceValues{
		Concurrency: t.Concurrency, HostIntervalMS: int(t.HostInterval.Milliseconds()),
		VendorConcurrency: t.VendorConcurrency, VendorIntervalMS: int(t.VendorInterval.Milliseconds()),
	}
}

func (h *handlers) crawlPace(c fiber.Ctx) (crawlPaceDTO, error) {
	stored, err := h.CrawlPace.Stored(c.Context())
	if err != nil {
		return crawlPaceDTO{}, err
	}
	return crawlPaceDTO{
		crawlPaceValues: paceValues(h.CrawlPace.Current(c.Context())),
		Defaults:        paceValues(h.CrawlPace.Defaults()),
		Stored:          stored,
	}, nil
}

func (h *handlers) getCrawlPace(c fiber.Ctx) error {
	if h.CrawlPace == nil {
		return domain.ErrNotFound
	}
	dto, err := h.crawlPace(c)
	if err != nil {
		return err
	}
	return c.JSON(dataResponse[crawlPaceDTO]{Data: dto})
}

// crawlPaceRequest changes the pace. A field left out stays as it is; 0 brings
// back the environment default.
type crawlPaceRequest struct {
	Concurrency       *int `json:"concurrency"`
	HostIntervalMS    *int `json:"host_interval_ms"`
	VendorConcurrency *int `json:"vendor_concurrency"`
	VendorIntervalMS  *int `json:"vendor_interval_ms"`
}

func (h *handlers) putCrawlPace(c fiber.Ctx) error {
	if h.CrawlPace == nil {
		return domain.ErrNotFound
	}
	var req crawlPaceRequest
	if err := decodeJSON(c, &req); err != nil {
		return err
	}
	err := h.CrawlPace.Save(c.Context(), crawlconfig.Update{
		Concurrency: req.Concurrency, HostIntervalMS: req.HostIntervalMS,
		VendorConcurrency: req.VendorConcurrency, VendorIntervalMS: req.VendorIntervalMS,
	})
	var invalid *crawlconfig.InvalidError
	if errors.As(err, &invalid) {
		return (&domain.ValidationError{}).Add(invalid.Field, "%s", invalid.Message)
	}
	if err != nil {
		return err
	}
	dto, err := h.crawlPace(c)
	if err != nil {
		return err
	}
	return c.JSON(dataResponse[crawlPaceDTO]{Data: dto})
}
