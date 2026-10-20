package enrich

import (
	"context"
	"errors"
	"sort"

	"example.com/rolescout/api/internal/config"
	"example.com/rolescout/api/internal/ingest"
	"example.com/rolescout/api/internal/llm"
)

// CVReading is what was read from a CV.
type CVReading struct {
	Skills    []string
	Titles    []string
	Seniority string
	Years     int
	Summary   string
	// AI is whether the model contributed; without it only the skills the
	// rules recognise are known.
	AI bool
}

// CVReader reads a CV: the rules find the technologies, the model (when AI is
// on) adds titles, seniority, experience and a summary.
type CVReader struct {
	LLM     llm.Completer
	Resolve func(context.Context) config.LLM
}

// Read never fails because of the model: a model that is off or in trouble
// leaves the rule-based reading, and AI false.
func (r *CVReader) Read(ctx context.Context, text string) CVReading {
	skills := map[string]bool{}
	for _, t := range ingest.ExtractTechnologies(text) {
		skills[t] = true
	}
	out := CVReading{}
	if r.Resolve != nil && r.Resolve(ctx).Enabled {
		p, err := llm.ParseCV(ctx, r.LLM, text)
		switch {
		case err == nil:
			out.AI = true
			out.Titles, out.Seniority, out.Years, out.Summary = p.Titles, p.Seniority, p.Years, p.Summary
			for _, s := range p.Skills {
				skills[s] = true
			}
		case errors.Is(err, context.Canceled):
		}
	}
	for s := range skills {
		out.Skills = append(out.Skills, s)
	}
	sort.Strings(out.Skills)
	if len(out.Skills) > 60 {
		out.Skills = out.Skills[:60]
	}
	return out
}
