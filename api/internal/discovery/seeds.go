package discovery

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
)

//go:embed seeds/companies.txt
var seedsFile string

//go:embed seeds/turkey.txt
var turkeySeedsFile string

//go:embed seeds/turkey_employers.txt
var turkeyEmployersFile string

//go:embed seeds/agencies_and_employers.txt
var agenciesFile string

// Seed is a career page shipped with RoleScout.
type Seed struct {
	URL, Name string
	Country   string // ISO 3166-1 code of the list the seed came from, if it is a country's list
}

// Seeds returns the embedded seed lists (worldwide and Türkiye): one "URL [| Name]" per line, with
// "#" comments and blank lines.
func Seeds() ([]Seed, error) {
	world, err := ParseSeeds(seedsFile)
	if err != nil {
		return nil, err
	}
	turkey, err := ParseSeeds(turkeySeedsFile)
	if err != nil {
		return nil, err
	}
	for i := range turkey {
		turkey[i].Country = "TR"
	}
	employers, err := ParseSeeds(turkeyEmployersFile)
	if err != nil {
		return nil, err
	}
	for i := range employers {
		employers[i].Country = "TR"
	}
	agencies, err := ParseSeeds(agenciesFile)
	if err != nil {
		return nil, err
	}
	all := append(append(append(world, turkey...), employers...), agencies...)
	// A site listed twice is offered once: the first list wins.
	seen := map[string]bool{}
	out := all[:0]
	for _, s := range all {
		key := strings.ToLower(strings.TrimRight(s.URL, "/"))
		if !seen[key] {
			seen[key] = true
			out = append(out, s)
		}
	}
	return out, nil
}

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

// seedDigest identifies the embedded lists; it is the source's state, so a
// release with new seeds is read at once.
func seedDigest() string {
	sum := sha256.Sum256([]byte(seedsFile + "\x00" + turkeySeedsFile + "\x00" + turkeyEmployersFile + "\x00" + agenciesFile))
	return hex.EncodeToString(sum[:8])
}

func (SeedSource) Pending(state string) bool { return state != seedDigest() }

func (SeedSource) Discover(_ context.Context, state string, fn func(Candidate) error) (string, error) {
	seeds, err := Seeds()
	if err != nil {
		return state, err
	}
	for _, s := range seeds {
		if err := fn(Candidate{URL: s.URL, Name: s.Name, Origin: "seed", Country: s.Country}); err != nil {
			return state, err
		}
	}
	return seedDigest(), nil
}
