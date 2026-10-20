package enrich

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"example.com/rolescout/api/internal/config"
	"example.com/rolescout/api/internal/domain"
	"example.com/rolescout/api/internal/llm"
)

// ErrDisabled means AI is switched off, so nothing was generated.
var ErrDisabled = errors.New("enrich: AI is switched off")

// AnalysisRepository is what an on-demand analysis reads and writes.
type AnalysisRepository interface {
	Repository
	EnrichmentTaskFor(ctx context.Context, jobID string) (domain.EnrichmentTask, error)
	HasEnrichment(ctx context.Context, jobID string) (bool, error)
	ExplanationTaskFor(ctx context.Context, workspaceID, profileID, jobID string) (domain.ExplanationTask, error)
	SaveExplanation(ctx context.Context, workspaceID, profileID, jobID, descriptionHash, model string, promptVersion int, text string) error
}

// Analyzer runs the model for one job a user asked about: it reads the posting
// if that has not happened yet and writes the reason the job matches. This is
// how weak matches get AI text, which the background worker only writes for
// strong ones (and, with a hosted service, not at all).
type Analyzer struct {
	Repo           AnalysisRepository
	LLM            llm.Completer
	Resolve        func(ctx context.Context) config.LLM
	ResolveCountry func(place string) string
	Now            func() time.Time

	mu     sync.Mutex
	flight map[string]struct{}
}

// ErrBusy means the same analysis is already running.
var ErrBusy = errors.New("enrich: analysis already running")

func (a *Analyzer) now() time.Time {
	if a.Now != nil {
		return a.Now()
	}
	return time.Now()
}

// Analyze generates (or returns the stored) explanation of a match. language
// is "tr" or "en"; empty keeps the workspace owner's language.
func (a *Analyzer) Analyze(ctx context.Context, workspaceID, profileID, jobID, language string) (string, error) {
	cfg := a.Resolve(ctx)
	if !cfg.Enabled {
		return "", ErrDisabled
	}
	key := workspaceID + "/" + profileID + "/" + jobID
	a.mu.Lock()
	if a.flight == nil {
		a.flight = map[string]struct{}{}
	}
	if _, busy := a.flight[key]; busy {
		a.mu.Unlock()
		return "", ErrBusy
	}
	a.flight[key] = struct{}{}
	a.mu.Unlock()
	defer func() {
		a.mu.Lock()
		delete(a.flight, key)
		a.mu.Unlock()
	}()

	read, err := a.Repo.HasEnrichment(ctx, jobID)
	if err != nil {
		return "", err
	}
	if !read {
		if err := a.read(ctx, cfg, jobID); err != nil {
			return "", err
		}
	}
	task, err := a.Repo.ExplanationTaskFor(ctx, workspaceID, profileID, jobID)
	if err != nil {
		return "", err
	}
	if language == "tr" || language == "en" {
		task.Locale = language
	}
	text, err := llm.Explain(ctx, a.LLM, llm.JobFacts{
		Title: task.Title, Company: task.CompanyName, Score: task.Score, Reasons: reasonFacts(task.Reasons),
		Technologies: task.Technologies, MatchedTechnologies: task.MatchedTechnologies,
		MissingTechnologies: task.MissingTechnologies, CVSkills: task.CVSkills, About: task.About,
		Seniority: task.Seniority, Arrangement: task.Arrangement, Country: task.Country, Language: task.Locale,
	})
	if err != nil {
		return "", err
	}
	if err := a.Repo.SaveExplanation(ctx, workspaceID, profileID, jobID, task.DescriptionHash, cfg.Model, llm.ExplainPromptVersion, text); err != nil {
		return "", err
	}
	return text, nil
}

func (a *Analyzer) read(ctx context.Context, cfg config.LLM, jobID string) error {
	task, err := a.Repo.EnrichmentTaskFor(ctx, jobID)
	if err != nil {
		return err
	}
	e, stats, err := llm.Extract(ctx, a.LLM, llm.Input{
		Model: cfg.Model, Title: task.Title, Company: task.CompanyName, Description: task.Description,
		MaxChars: cfg.MaxInputChars, MaxTokens: cfg.MaxOutputTokens, ResolveCountry: a.ResolveCountry,
	})
	if err != nil {
		return err
	}
	data, err := json.Marshal(e)
	if err != nil {
		return err
	}
	return a.Repo.Save(ctx, domain.EnrichmentResult{
		JobID: task.JobID, DescriptionHash: task.DescriptionHash, Model: cfg.Model,
		PromptVersion: llm.PromptVersion, SchemaVersion: llm.SchemaVersion, Now: a.now(),
		OK: true, Data: data,
		PromptTokens: stats.PromptTokens, CompletionTokens: stats.CompletionTokens, Latency: stats.Latency,
	})
}
