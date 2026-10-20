package httptransport

import (
	"context"

	"github.com/gofiber/fiber/v3"

	"example.com/rolescout/api/internal/sysstat"
)

// SystemReporter measures the installation's memory use and ceilings.
type SystemReporter interface {
	Report(ctx context.Context) (sysstat.Report, error)
}

// systemStatus shows what each service uses and may use (administrator).
func (h *handlers) systemStatus(c fiber.Ctx) error {
	if h.System == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "System statistics are not available.")
	}
	r, err := h.System.Report(c.Context())
	if err != nil {
		return err
	}
	return c.JSON(dataResponse[sysstat.Report]{Data: r})
}
