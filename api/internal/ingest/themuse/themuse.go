// Package themuse reads The Muse's public jobs API
// (https://www.themuse.com/api/public/jobs), which needs no key.
// See docs/sources/themuse.md.
package themuse

import (
	"context"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"example.com/rolescout/api/internal/crawl"
	"example.com/rolescout/api/internal/ingest"
)

// DefaultMaxPages bounds the pages read (20 jobs each).
const DefaultMaxPages = 10

// Adapter reads one search. Config: {"category": "Software Engineering",
// "level": "Senior Level", "location": "London, United Kingdom",
// "max_pages": "10"}; every field is optional.
type Adapter struct {
	Client  *crawl.Client
	BaseURL string // overrides https://www.themuse.com (for tests)
}

func New(c *crawl.Client) *Adapter { return &Adapter{Client: c} }

var (
	text   = regexp.MustCompile(`^[A-Za-z0-9 &/,.'-]{1,60}$`)
	digits = regexp.MustCompile(`^[0-9]{1,2}$`)
)

func (a *Adapter) Validate(config map[string]any) error {
	return ingest.CheckConfig(config,
		ingest.Field{Name: "category", Pattern: text, Optional: true},
		ingest.Field{Name: "level", Pattern: text, Optional: true},
		ingest.Field{Name: "location", Pattern: text, Optional: true},
		ingest.Field{Name: "max_pages", Pattern: digits, Optional: true})
}

type response struct {
	PageCount int `json:"page_count"`
	Results   []struct {
		ID        int64  `json:"id"`
		Name      string `json:"name"`
		Contents  string `json:"contents"`
		Published string `json:"publication_date"`
		Locations []struct {
			Name string `json:"name"`
		} `json:"locations"`
		Levels []struct {
			Name string `json:"name"`
		} `json:"levels"`
		Refs struct {
			LandingPage string `json:"landing_page"`
		} `json:"refs"`
		Company struct {
			Name string `json:"name"`
		} `json:"company"`
	} `json:"results"`
}

func (a *Adapter) Fetch(ctx context.Context, config map[string]any) ([]ingest.RawListing, error) {
	if err := a.Validate(config); err != nil {
		return nil, err
	}
	base := a.BaseURL
	if base == "" {
		base = "https://www.themuse.com"
	}
	limit := DefaultMaxPages
	if n, err := strconv.Atoi(ingest.ConfigString(config, "max_pages")); err == nil && n > 0 {
		limit = n
	}
	var out []ingest.RawListing
	for p := 1; p <= limit; p++ {
		q := url.Values{"page": {strconv.Itoa(p)}}
		for _, k := range []string{"category", "level", "location"} {
			if v := ingest.ConfigString(config, k); v != "" {
				q.Set(k, v)
			}
		}
		var resp response
		if err := ingest.FetchJSON(ctx, a.Client, base+"/api/public/jobs?"+q.Encode(), &resp); err != nil {
			if p == 1 {
				return nil, err
			}
			return out, ingest.Incomplete("page %d could not be read: %v", p, err)
		}
		for _, j := range resp.Results {
			var locs, levels []string
			for _, l := range j.Locations {
				locs = append(locs, l.Name)
			}
			for _, l := range j.Levels {
				levels = append(levels, l.Name)
			}
			l := ingest.RawListing{
				ExternalID:  strconv.FormatInt(j.ID, 10),
				URL:         j.Refs.LandingPage,
				Title:       j.Name,
				Company:     j.Company.Name,
				Location:    strings.Join(locs, "; "),
				Description: j.Contents + " " + strings.Join(levels, " "),
			}
			if len(locs) == 1 && strings.Contains(strings.ToLower(locs[0]), "remote") {
				l.Arrangement = "remote"
			}
			if t, err := time.Parse(time.RFC3339, j.Published); err == nil {
				t = t.UTC()
				l.PostedAt = &t
			}
			out = append(out, l)
		}
		if p >= resp.PageCount || len(resp.Results) == 0 {
			break
		}
	}
	return out, ingest.Incomplete("search feed: read at most %d pages, other jobs are not closed", limit)
}
