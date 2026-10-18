package httptransport

import (
	"context"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"

	"example.com/rolescout/api/internal/domain"
	"example.com/rolescout/api/internal/enrich"
)

// LLMStatusProvider reports on the optional local model. With probe it also
// runs the model once.
type LLMStatusProvider interface {
	Status(ctx context.Context, probe bool) (enrich.Status, error)
}

// EnrichmentReader gives the job endpoints what the model added.
type EnrichmentReader interface {
	AIFacts(ctx context.Context, jobIDs []string) (map[string]domain.JobAI, error)
	Explanations(ctx context.Context, workspaceID, profileID string, jobIDs []string) (map[string]string, error)
}

type llmStatusDTO struct {
	Enabled       bool       `json:"enabled"`
	BaseHost      string     `json:"base_host,omitempty"`
	Model         string     `json:"model,omitempty"`
	FallbackModel string     `json:"fallback_model,omitempty"`
	Reachable     bool       `json:"reachable"`
	ModelPresent  bool       `json:"model_present"`
	Degraded      bool       `json:"degraded"`
	Local         bool       `json:"local"`
	Queue         int        `json:"queue"`
	Enriched      int        `json:"enriched"`
	Failed        int        `json:"failed"`
	AvgLatencyMs  int        `json:"avg_latency_ms"`
	LastError     string     `json:"last_error,omitempty"`
	LastCheckedAt *time.Time `json:"last_checked_at"`
	ProbeOK       bool       `json:"probe_ok"`
	ProbeMs       int        `json:"probe_ms"`
}

func toLLMStatus(s enrich.Status) llmStatusDTO {
	return llmStatusDTO{
		Enabled: s.Enabled, BaseHost: s.BaseHost, Model: s.Model, FallbackModel: s.FallbackModel, Reachable: s.Reachable,
		ModelPresent: s.ModelPresent, Degraded: s.Degraded, Local: s.Local, Queue: s.Queue, Enriched: s.Enriched, Failed: s.Failed,
		AvgLatencyMs: s.AvgLatencyMs, LastError: s.LastError, LastCheckedAt: utcPtr(s.LastCheckedAt), ProbeOK: s.ProbeOK, ProbeMs: s.ProbeMs,
	}
}

func (h *handlers) llmStatus(c fiber.Ctx) error {
	return h.respondLLM(c, false)
}

// checkCooldown spaces connection checks, so the button cannot hammer a laptop's model.
const checkCooldown = 5 * time.Second

var (
	checkMu   sync.Mutex
	lastCheck time.Time
)

func (h *handlers) llmCheck(c fiber.Ctx) error {
	if h.LLM != nil {
		checkMu.Lock()
		now := time.Now()
		if now.Sub(lastCheck) < checkCooldown {
			checkMu.Unlock()
			return fiber.NewError(fiber.StatusTooManyRequests, "Wait a few seconds before checking again.")
		}
		lastCheck = now
		checkMu.Unlock()
	}
	return h.respondLLM(c, true)
}

func (h *handlers) respondLLM(c fiber.Ctx, probe bool) error {
	if h.LLM == nil {
		return c.JSON(dataResponse[llmStatusDTO]{Data: llmStatusDTO{}})
	}
	s, err := h.LLM.Status(c.Context(), probe)
	if err != nil {
		return err
	}
	return c.JSON(dataResponse[llmStatusDTO]{Data: toLLMStatus(s)})
}

type salaryDTO struct {
	Min      *int   `json:"min"`
	Max      *int   `json:"max"`
	Currency string `json:"currency"`
	Period   string `json:"period"`
}

// aiDTO is what the local model added to a job. Every value in it is
// model-derived and the interface marks it as such.
type aiDTO struct {
	Model             string     `json:"model"`
	Summary           *string    `json:"summary"`
	RemotePolicy      *string    `json:"remote_policy"`
	Salary            *salaryDTO `json:"salary"`
	Fields            []string   `json:"fields"`
	ExtraTechnologies []string   `json:"extra_technologies"`
}

func toAI(a domain.JobAI) *aiDTO {
	if a.Empty() {
		return nil
	}
	out := &aiDTO{Model: a.Model, Fields: orEmpty(a.Fields), ExtraTechnologies: orEmpty(a.ExtraTechnologies)}
	if a.Summary != "" {
		out.Summary = &a.Summary
	}
	if a.RemotePolicy != "" {
		out.RemotePolicy = &a.RemotePolicy
	}
	if a.SalaryMin != nil || a.SalaryMax != nil {
		out.Salary = &salaryDTO{Min: a.SalaryMin, Max: a.SalaryMax, Currency: a.SalaryCurrency, Period: a.SalaryPeriod}
	}
	return out
}

func orEmpty(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// attachAI adds the model's contribution to job DTOs. It is best effort: a
// failure to read it must never break the job list.
func (h *handlers) attachAI(c fiber.Ctx, profileID string, jobs []workspaceJobDTO) {
	if h.Enrichments == nil || len(jobs) == 0 {
		return
	}
	ids := make([]string, len(jobs))
	for i, j := range jobs {
		ids[i] = j.ID
	}
	facts, err := h.Enrichments.AIFacts(c.Context(), ids)
	if err != nil {
		return
	}
	var explanations map[string]string
	if profileID != "" {
		explanations, _ = h.Enrichments.Explanations(c.Context(), workspaceID(c), profileID, ids)
	}
	for i := range jobs {
		if a, ok := facts[jobs[i].ID]; ok {
			jobs[i].AI = toAI(a)
		}
		if text := explanations[jobs[i].ID]; text != "" && jobs[i].Match != nil {
			t := text
			jobs[i].Match.Explanation = &t
		}
	}
}
