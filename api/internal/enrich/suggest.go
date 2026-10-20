package enrich

import (
	"context"
	"strings"

	"example.com/rolescout/api/internal/config"
	"example.com/rolescout/api/internal/domain"
	"example.com/rolescout/api/internal/llm"
)

// MemoryReader reads what the AI knows about a workspace's person.
type MemoryReader interface {
	Get(ctx context.Context, workspaceID string) (domain.WorkspaceContext, error)
}

// ProfileLister lists a workspace's saved searches.
type ProfileLister interface {
	ListProfiles(ctx context.Context, workspaceID string) ([]domain.ResearchProfile, error)
}

// ProfileSuggester drafts research profiles with the live AI connection. Its
// memory (the person's note, their CV and their other searches) is read for
// every draft, so a short description still grows into a full profile.
type ProfileSuggester struct {
	LLM      llm.Completer
	Resolve  func(context.Context) config.LLM
	Memory   MemoryReader
	Profiles ProfileLister
}

// SuggestProfile returns ErrDisabled while AI is switched off.
func (s *ProfileSuggester) SuggestProfile(ctx context.Context, workspaceID, description string) (llm.ProfileSuggestion, error) {
	if !s.Resolve(ctx).Enabled {
		return llm.ProfileSuggestion{}, ErrDisabled
	}
	return llm.SuggestProfile(ctx, s.LLM, description, s.background(ctx, workspaceID))
}

// HasBackground reports whether the workspace has a note, a CV or searches the
// AI can draw on, so a draft may be asked for without a description.
func (s *ProfileSuggester) HasBackground(ctx context.Context, workspaceID string) bool {
	return !s.background(ctx, workspaceID).Empty()
}

func (s *ProfileSuggester) background(ctx context.Context, workspaceID string) llm.Background {
	var bg llm.Background
	if s.Memory != nil {
		if m, err := s.Memory.Get(ctx, workspaceID); err == nil {
			bg.About, bg.CVSummary, bg.CVSkills, bg.CVTitles, bg.CVSeniority = m.About, m.CVSummary, m.CVSkills, m.CVTitles, m.CVSeniority
			if m.CVYears != nil {
				bg.CVYears = *m.CVYears
			}
		}
	}
	if s.Profiles != nil {
		if ps, err := s.Profiles.ListProfiles(ctx, workspaceID); err == nil {
			for _, p := range ps {
				line := p.Name
				if len(p.TitleKeywords) > 0 {
					line += " (" + strings.Join(p.TitleKeywords[:min(len(p.TitleKeywords), 4)], ", ") + ")"
				}
				bg.ExistingProfiles = append(bg.ExistingProfiles, line)
			}
		}
	}
	return bg
}
