package discovery

import (
	"context"
	_ "embed"
	"fmt"
	"net/url"
	"strings"
)

//go:embed seeds/companies.txt
var seedsFile string

//go:embed seeds/turkey.txt
var turkeySeedsFile string

// Seed is a career page shipped with RoleScout.
type Seed struct {
	URL, Name string
}

// Seeds returns the embedded seed lists (worldwide and Türkiye): one "URL [| Name]" per line, with
// "#" comments and blank lines.
func Seeds() ([]Seed, error) { return ParseSeeds(seedsFile + "\n" + turkeySeedsFile) }

// ParseSeeds parses a seed list in the embedded format.
func ParseSeeds(text string) ([]Seed, error) {
	var out []Seed
	for i, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		raw, name, _ := strings.Cut(line, "|")
		raw = strings.TrimSpace(raw)
		u, err := url.Parse(raw)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return nil, fmt.Errorf("seeds line %d: %q is not an absolute http(s) URL", i+1, raw)
		}
		out = append(out, Seed{URL: raw, Name: strings.TrimSpace(name)})
	}
	return out, nil
}

// SeedSource offers the embedded seeds as candidates.
type SeedSource struct{}

func (SeedSource) Name() string { return "seed" }

func (SeedSource) Discover(_ context.Context, state string, fn func(Candidate) error) (string, error) {
	seeds, err := Seeds()
	if err != nil {
		return state, err
	}
	for _, s := range seeds {
		if err := fn(Candidate{URL: s.URL, Name: s.Name, Origin: "seed"}); err != nil {
			return state, err
		}
	}
	return state, nil
}
