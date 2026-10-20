package enrich

import (
	"context"

	"example.com/rolescout/api/internal/config"
	"example.com/rolescout/api/internal/llm"
)

// ProfileSuggester drafts research profiles with the live AI connection.
type ProfileSuggester struct {
	LLM     llm.Completer
	Resolve func(context.Context) config.LLM
}

// SuggestProfile returns ErrDisabled while AI is switched off.
func (s *ProfileSuggester) SuggestProfile(ctx context.Context, description string) (llm.ProfileSuggestion, error) {
	if !s.Resolve(ctx).Enabled {
		return llm.ProfileSuggestion{}, ErrDisabled
	}
	return llm.SuggestProfile(ctx, s.LLM, description)
}
