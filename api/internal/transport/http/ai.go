package httptransport

import (
	"context"
	"errors"
	"strings"

	"github.com/gofiber/fiber/v3"

	"example.com/rolescout/api/internal/config"
	"example.com/rolescout/api/internal/domain"
	"example.com/rolescout/api/internal/enrich"
	"example.com/rolescout/api/internal/llm"
	"example.com/rolescout/api/internal/llmconfig"
)

// MatchAnalyzer generates the AI text of one match on request.
type MatchAnalyzer interface {
	Analyze(ctx context.Context, workspaceID, profileID, jobID, language string) (string, error)
}

// ProfileSuggester drafts a research profile from a description.
type ProfileSuggester interface {
	SuggestProfile(ctx context.Context, description string) (llm.ProfileSuggestion, error)
}

// LLMSettingsStore reads and edits the AI connection saved in the web app.
type LLMSettingsStore interface {
	Current(ctx context.Context) config.LLM
	Stored(ctx context.Context) (map[string]bool, error)
	Save(ctx context.Context, u llmconfig.Update) error
}

type analysisDTO struct {
	Explanation string  `json:"explanation"`
	AI          *aiDTO  `json:"ai"`
	Model       *string `json:"model,omitempty"`
}

// analyzeJob generates the AI reading and the match explanation of one job on
// the user's request. It is how a weak match gets AI text; the call can take
// a while, so it is a POST the client waits for.
func (h *handlers) analyzeJob(c fiber.Ctx) error {
	jobID, err := pathID(c, "jobID")
	if err != nil {
		return err
	}
	profileID := c.Query("profile")
	if !isUUID(profileID) {
		return (&domain.ValidationError{}).Add("profile", "must be a profile ID")
	}
	lang := c.Query("lang")
	if lang != "" && lang != "tr" && lang != "en" {
		return (&domain.ValidationError{}).Add("lang", "must be tr or en")
	}
	if h.AI == nil {
		return fiber.NewError(fiber.StatusPreconditionFailed, "AI is switched off. An administrator can turn it on in Settings.")
	}
	text, err := h.AI.Analyze(c.Context(), workspaceID(c), strings.ToLower(profileID), jobID, lang)
	switch {
	case err == nil:
	case errors.Is(err, enrich.ErrDisabled):
		return fiber.NewError(fiber.StatusPreconditionFailed, "AI is switched off. An administrator can turn it on in Settings.")
	case errors.Is(err, enrich.ErrBusy):
		return fiber.NewError(fiber.StatusTooManyRequests, "This job is already being analysed.")
	case errors.Is(err, domain.ErrNotFound):
		return domain.ErrNotFound
	case errors.Is(err, llm.ErrUnavailable), errors.Is(err, llm.ErrTimeout):
		return fiber.NewError(fiber.StatusServiceUnavailable, "The AI service could not be reached. Check the connection and API key in Settings.")
	case errors.Is(err, context.Canceled):
		return err
	case errors.Is(err, llm.ErrInvalidOutput), errors.Is(err, llm.ErrBadResponse):
		return fiber.NewError(fiber.StatusBadGateway, "The AI answered with something unusable. Try again.")
	default:
		return err
	}
	out := analysisDTO{Explanation: text}
	if h.Enrichments != nil {
		if facts, err := h.Enrichments.AIFacts(c.Context(), []string{jobID}); err == nil {
			if a, ok := facts[jobID]; ok {
				out.AI = toAI(a)
			}
		}
	}
	return c.JSON(dataResponse[analysisDTO]{Data: out})
}

type llmConfigDTO struct {
	Enabled    bool   `json:"enabled"`
	BaseURL    string `json:"base_url"`
	Model      string `json:"model"`
	HasAPIKey  bool   `json:"has_api_key"`
	APIKeyHint string `json:"api_key_hint,omitempty"`
	// Background is whether the worker reads the whole catalog on its own;
	// BackgroundAuto says nobody chose, so it follows where the model runs.
	Background     bool `json:"background"`
	BackgroundAuto bool `json:"background_auto"`
	Local          bool `json:"local"`
	// Stored lists the values saved in the web app (the rest come from the environment).
	Stored map[string]bool `json:"stored"`
}

func (h *handlers) llmConfig(c fiber.Ctx) (llmConfigDTO, error) {
	cfg := h.LLMSettings.Current(c.Context())
	stored, err := h.LLMSettings.Stored(c.Context())
	if err != nil {
		return llmConfigDTO{}, err
	}
	return llmConfigDTO{
		Enabled: cfg.Enabled, BaseURL: cfg.BaseURL, Model: cfg.Model,
		HasAPIKey: cfg.APIKey != "", APIKeyHint: llmconfig.MaskKey(cfg.APIKey),
		Background: enrich.BackgroundOn(cfg), BackgroundAuto: cfg.Background == nil,
		Local:  enrich.IsLocalURL(cfg.BaseURL),
		Stored: stored,
	}, nil
}

func (h *handlers) getLLMConfig(c fiber.Ctx) error {
	if h.LLMSettings == nil {
		return domain.ErrNotFound
	}
	dto, err := h.llmConfig(c)
	if err != nil {
		return err
	}
	return c.JSON(dataResponse[llmConfigDTO]{Data: dto})
}

// llmConfigRequest changes the AI connection. A field left out stays as it
// is; an empty string brings back the environment default. The key is never
// returned: send a new one to replace it, or "" to remove it.
type llmConfigRequest struct {
	Enabled *bool   `json:"enabled"`
	BaseURL *string `json:"base_url"`
	Model   *string `json:"model"`
	APIKey  *string `json:"api_key"`
	// Background: true or false chooses, null with ResetBackground returns to automatic.
	Background      *bool `json:"background"`
	ResetBackground bool  `json:"reset_background"`
}

func (h *handlers) putLLMConfig(c fiber.Ctx) error {
	if h.LLMSettings == nil {
		return domain.ErrNotFound
	}
	var req llmConfigRequest
	if err := decodeJSON(c, &req); err != nil {
		return err
	}
	err := h.LLMSettings.Save(c.Context(), llmconfig.Update{
		Enabled: req.Enabled, BaseURL: req.BaseURL, Model: req.Model, APIKey: req.APIKey,
		Background: req.Background, ClearBackground: req.ResetBackground,
	})
	var invalid *llmconfig.InvalidError
	if errors.As(err, &invalid) {
		return (&domain.ValidationError{}).Add(invalid.Field, "%s", invalid.Message)
	}
	if err != nil {
		return err
	}
	dto, err := h.llmConfig(c)
	if err != nil {
		return err
	}
	return c.JSON(dataResponse[llmConfigDTO]{Data: dto})
}

type suggestRequest struct {
	Description string `json:"description"`
}

type profileDraftDTO struct {
	Name             string   `json:"name"`
	TitleKeywords    []string `json:"title_keywords"`
	Keywords         []string `json:"keywords"`
	Technologies     []string `json:"technologies"`
	Countries        []string `json:"countries"`
	Locations        []string `json:"locations"`
	ExcludedKeywords []string `json:"excluded_keywords"`
	Remote           string   `json:"remote"`
	Seniority        []string `json:"seniority"`
	ScheduleHours    int      `json:"schedule_hours"`
}

// suggestProfile drafts a research profile from a description. Nothing is
// saved: the web form shows the draft and the user saves it as a normal profile.
func (h *handlers) suggestProfile(c fiber.Ctx) error {
	var req suggestRequest
	if err := decodeJSON(c, &req); err != nil {
		return err
	}
	var v domain.ValidationError
	desc := strings.TrimSpace(req.Description)
	if len([]rune(desc)) < 10 {
		v.Add("description", "describe the job you want in a sentence or two")
	} else if len([]rune(desc)) > llm.MaxProfileDescription {
		v.Add("description", "must be at most %d characters", llm.MaxProfileDescription)
	}
	if err := v.Err(); err != nil {
		return err
	}
	if h.Suggester == nil {
		return fiber.NewError(fiber.StatusPreconditionFailed, "AI is switched off. An administrator can turn it on in Settings.")
	}
	s, err := h.Suggester.SuggestProfile(c.Context(), desc)
	switch {
	case err == nil:
	case errors.Is(err, enrich.ErrDisabled):
		return fiber.NewError(fiber.StatusPreconditionFailed, "AI is switched off. An administrator can turn it on in Settings.")
	case errors.Is(err, llm.ErrUnavailable), errors.Is(err, llm.ErrTimeout):
		return fiber.NewError(fiber.StatusServiceUnavailable, "The AI service could not be reached. Check the connection and API key in Settings.")
	case errors.Is(err, context.Canceled):
		return err
	case errors.Is(err, llm.ErrInvalidOutput), errors.Is(err, llm.ErrBadResponse):
		return fiber.NewError(fiber.StatusBadGateway, "The AI answered with something unusable. Try again or describe the job in more detail.")
	default:
		return err
	}
	return c.JSON(dataResponse[profileDraftDTO]{Data: profileDraftDTO{
		Name: s.Name, TitleKeywords: nz(s.TitleKeywords), Keywords: nz(s.Keywords), Technologies: nz(s.Technologies),
		Countries: nz(s.Countries), Locations: nz(s.Locations), ExcludedKeywords: nz(s.ExcludedKeywords),
		Remote: s.Remote, Seniority: nz(s.Seniority), ScheduleHours: 12,
	}})
}

// nz makes a nil list encode as [] rather than null.
func nz(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}
