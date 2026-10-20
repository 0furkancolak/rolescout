// Package focus carries the countries a research run concentrates on. Work for
// those countries (detecting their companies' job boards, reading their jobs)
// is done first; the rest follows. It travels in the context so the stages and
// the repositories they call do not each need a new parameter.
package focus

import (
	"context"
	"slices"
	"strings"
)

type key struct{}

// With returns ctx with the countries (ISO 3166-1 alpha-2) to focus on.
func With(ctx context.Context, countries []string) context.Context {
	return context.WithValue(ctx, key{}, normalize(countries))
}

// Countries returns the focus countries in upper case; never nil, so it can be
// passed to the database as an empty array.
func Countries(ctx context.Context) []string {
	if v, ok := ctx.Value(key{}).([]string); ok {
		return v
	}
	return []string{}
}

func normalize(in []string) []string {
	out := make([]string, 0, len(in))
	for _, c := range in {
		if c = strings.ToUpper(strings.TrimSpace(c)); len(c) == 2 && !slices.Contains(out, c) {
			out = append(out, c)
		}
	}
	return out
}
