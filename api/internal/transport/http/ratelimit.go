package httptransport

import (
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/limiter"
)

// Limit allows Max requests per client IP within Window. Max 0 disables it.
type Limit struct {
	Max    int
	Window time.Duration
}

// RateLimits protects the public authentication endpoints against password
// guessing and mass sign-ups. Counters are kept in memory per API process.
type RateLimits struct {
	Login    Limit
	Register Limit
	// Analyze caps on-demand AI generations per client: each one is a paid
	// request when a hosted service is configured.
	Analyze Limit
}

// DefaultRateLimits allows 10 failed sign-ins per 15 minutes and 5 sign-ups
// per hour from one IP address.
var DefaultRateLimits = RateLimits{
	Login:    Limit{Max: 10, Window: 15 * time.Minute},
	Register: Limit{Max: 5, Window: time.Hour},
	Analyze:  Limit{Max: 60, Window: time.Hour},
}

func rateLimit(l Limit, onlyFailures bool) fiber.Handler {
	if l.Max <= 0 {
		return func(c fiber.Ctx) error { return c.Next() }
	}
	return limiter.New(limiter.Config{
		Max:        l.Max,
		Expiration: l.Window,
		// Decided from the response status; routes use renderErrors after the
		// limiter so handler errors already set it.
		SkipSuccessfulRequests: onlyFailures,
		// c.IP() honours X-Real-IP only from trusted proxies (TRUSTED_PROXIES).
		KeyGenerator: func(c fiber.Ctx) string { return c.IP() },
		LimitReached: func(c fiber.Ctx) error {
			return fiber.NewError(fiber.StatusTooManyRequests, "Too many attempts. Try again later.")
		},
	})
}

// renderErrors writes a handler's error response immediately instead of
// leaving it to the app's error handler, so middleware that runs before it
// (the rate limiter) sees the final status code.
func renderErrors(c fiber.Ctx) error {
	if err := c.Next(); err != nil {
		return c.App().Config().ErrorHandler(c, err)
	}
	return nil
}
