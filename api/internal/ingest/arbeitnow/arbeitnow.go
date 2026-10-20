// Package arbeitnow reads Arbeitnow's free job board API
// (https://www.arbeitnow.com/api/job-board-api), mostly German-speaking
// countries. See docs/sources/arbeitnow.md.
package arbeitnow

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"example.com/rolescout/api/internal/crawl"
	"example.com/rolescout/api/internal/ingest"
)

// DefaultMaxPages bounds the pages read (300 jobs each).
const DefaultMaxPages = 3

// Adapter reads the feed. Config: {} or {"max_pages": "5"}.
type Adapter struct {
	Client  *crawl.Client
	BaseURL string // overrides https://www.arbeitnow.com (for tests)
}

func New(c *crawl.Client) *Adapter { return &Adapter{Client: c} }

var digits = regexp.MustCompile(`^[0-9]{1,2}$`)

func (a *Adapter) Validate(config map[string]any) error {
	return ingest.CheckConfig(config, ingest.Field{Name: "max_pages", Pattern: digits, Optional: true})
}

type page struct {
	Data []struct {
		Slug        string   `json:"slug"`
		Company     string   `json:"company_name"`
		Title       string   `json:"title"`
		Description string   `json:"description"`
		Remote      bool     `json:"remote"`
		URL         string   `json:"url"`
		JobTypes    []string `json:"job_types"`
		Location    string   `json:"location"`
		CreatedAt   int64    `json:"created_at"`
	} `json:"data"`
	Links struct {
		Next string `json:"next"`
	} `json:"links"`
}

func (a *Adapter) Fetch(ctx context.Context, config map[string]any) ([]ingest.RawListing, error) {
	if err := a.Validate(config); err != nil {
		return nil, err
	}
	base := a.BaseURL
	if base == "" {
		base = "https://www.arbeitnow.com"
	}
	limit := DefaultMaxPages
	if n, err := strconv.Atoi(ingest.ConfigString(config, "max_pages")); err == nil && n > 0 {
		limit = n
	}
	var out []ingest.RawListing
	for p := 1; p <= limit; p++ {
		var resp page
		if err := ingest.FetchJSON(ctx, a.Client, fmt.Sprintf("%s/api/job-board-api?page=%d", base, p), &resp); err != nil {
			if p == 1 {
				return nil, err
			}
			return out, ingest.Incomplete("page %d could not be read: %v", p, err)
		}
		for _, j := range resp.Data {
			l := ingest.RawListing{
				ExternalID:     j.Slug,
				URL:            j.URL,
				Title:          j.Title,
				Company:        j.Company,
				Location:       j.Location,
				Description:    j.Description,
				EmploymentType: strings.Join(j.JobTypes, " "),
			}
			if j.Remote {
				l.Arrangement = "remote"
			}
			if j.CreatedAt > 0 {
				t := time.Unix(j.CreatedAt, 0).UTC()
				l.PostedAt = &t
			}
			out = append(out, l)
		}
		if resp.Links.Next == "" || len(resp.Data) == 0 {
			break
		}
	}
	return out, ingest.Incomplete("job board feed: jobs that left the newest %d pages are not closed", limit)
}
