// Package workingnomads reads Working Nomads' public job feed
// (https://www.workingnomads.com/api/exposed_jobs/). See
// docs/sources/workingnomads.md.
package workingnomads

import (
	"context"
	"strings"
	"time"

	"example.com/rolescout/api/internal/crawl"
	"example.com/rolescout/api/internal/ingest"
)

// Adapter reads the feed. Config: {}.
type Adapter struct {
	Client  *crawl.Client
	BaseURL string // overrides https://www.workingnomads.com (for tests)
}

func New(c *crawl.Client) *Adapter { return &Adapter{Client: c} }

func (a *Adapter) Validate(config map[string]any) error { return ingest.CheckConfig(config) }

type job struct {
	URL         string `json:"url"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Company     string `json:"company_name"`
	Tags        string `json:"tags"`
	Location    string `json:"location"`
	PubDate     string `json:"pub_date"`
}

func (a *Adapter) Fetch(ctx context.Context, config map[string]any) ([]ingest.RawListing, error) {
	if err := a.Validate(config); err != nil {
		return nil, err
	}
	base := a.BaseURL
	if base == "" {
		base = "https://www.workingnomads.com"
	}
	var jobs []job
	if err := ingest.FetchJSON(ctx, a.Client, base+"/api/exposed_jobs/", &jobs); err != nil {
		return nil, err
	}
	out := make([]ingest.RawListing, 0, len(jobs))
	for _, j := range jobs {
		if j.URL == "" || j.Title == "" {
			continue
		}
		l := ingest.RawListing{
			ExternalID:  j.URL,
			URL:         j.URL,
			Title:       j.Title,
			Company:     j.Company,
			Location:    j.Location,
			Description: j.Description + " " + strings.ReplaceAll(j.Tags, ",", " "),
			Arrangement: "remote",
		}
		if t, err := time.Parse(time.RFC3339, j.PubDate); err == nil {
			t = t.UTC()
			l.PostedAt = &t
		}
		out = append(out, l)
	}
	return out, ingest.Incomplete("remote job feed: jobs that left the feed are not closed")
}
