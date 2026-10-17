package httptransport

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v3"
)

const readinessTimeout = 2 * time.Second

// Pinger reports whether a dependency is reachable.
type Pinger interface {
	Ping(ctx context.Context) error
}

func RegisterHealthRoutes(app *fiber.App, db Pinger) {
	app.Get("/healthz", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	app.Get("/readyz", func(c fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(c.Context(), readinessTimeout)
		defer cancel()

		if err := db.Ping(ctx); err != nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(
				fiber.Map{"status": "not_ready"},
			)
		}

		return c.JSON(fiber.Map{"status": "ready", "database": "ok"})
	})
}
