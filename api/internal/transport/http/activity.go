package httptransport

import (
	"context"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"

	"example.com/rolescout/api/internal/domain"
)

type ActivityStore interface {
	Activity(ctx context.Context, workspaceID string, days int, now time.Time) (domain.Activity, error)
}

const (
	defaultActivityDays = 14
	maxActivityDays     = 90
)

type activityDTO struct {
	Days                []string `json:"days"`
	NewMatches          []int    `json:"new_matches"`
	JobsCollected       []int    `json:"jobs_collected"`
	ApplicationsCreated []int    `json:"applications_created"`
	UpcomingInterviews  int      `json:"upcoming_interviews"`
}

// getActivity returns the dashboard's daily series for the workspace.
func (h *handlers) getActivity(c fiber.Ctx) error {
	days := defaultActivityDays
	if raw := c.Query("days"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > maxActivityDays {
			var v domain.ValidationError
			v.Add("days", "must be an integer between 1 and %d", maxActivityDays)
			return v.Err()
		}
		days = n
	}
	a, err := h.Activity.Activity(c.Context(), workspaceID(c), days, time.Now().UTC())
	if err != nil {
		return err
	}
	return c.JSON(dataResponse[activityDTO]{Data: activityDTO{
		Days: a.Days, NewMatches: a.NewMatches, JobsCollected: a.JobsCollected,
		ApplicationsCreated: a.ApplicationsCreated, UpcomingInterviews: a.UpcomingInterviews,
	}})
}
