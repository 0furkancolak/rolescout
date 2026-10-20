package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"example.com/rolescout/api/internal/crawl"
	"example.com/rolescout/api/internal/ingest"
)

// YCombinatorURL is the community-kept public JSON of Y Combinator companies
// that say they are hiring (yc-oss, built from the public company directory).
const YCombinatorURL = "https://yc-oss.github.io/api/companies/hiring.json"

// YCombinator offers the websites of the Y Combinator companies that are
// hiring. The detector then finds each one's job board.
type YCombinator struct {
	Client *crawl.Client
	URL    string // default YCombinatorURL
}

func (y *YCombinator) Name() string { return "ycombinator" }

type ycCompany struct {
	Name         string `json:"name"`
	Website      string `json:"website"`
	AllLocations string `json:"all_locations"`
	Status       string `json:"status"`
}

func (y *YCombinator) Discover(ctx context.Context, state string, fn func(Candidate) error) (string, error) {
	u := y.URL
	if u == "" {
		u = YCombinatorURL
	}
	body, err := ingest.Fetch(ctx, y.Client, u, "application/json")
	if err != nil {
		return state, fmt.Errorf("read Y Combinator companies: %w", err)
	}
	var companies []ycCompany
	if err := json.Unmarshal(body, &companies); err != nil {
		return state, fmt.Errorf("unreadable Y Combinator list: %w", err)
	}
	for _, c := range companies {
		if strings.EqualFold(c.Status, "Inactive") || strings.EqualFold(c.Status, "Acquired") {
			continue
		}
		site := companySite(c.Website)
		if site == "" {
			continue
		}
		cand := Candidate{URL: site, Name: strings.TrimSpace(c.Name), Origin: "ycombinator"}
		// The first location names the office; the domain says the rest.
		if first, _, _ := strings.Cut(c.AllLocations, ";"); first != "" {
			cand.Country = ingest.InferCountry(first)
		}
		if err := fn(cand); err != nil {
			return state, err
		}
	}
	return state, nil
}
