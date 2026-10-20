// Package remoteok reads Remote OK's public API (https://remoteok.com/api).
// See docs/sources/remoteok.md.
package remoteok

import (
	"context"
	"time"

	"example.com/rolescout/api/internal/crawl"
	"example.com/rolescout/api/internal/ingest"
)

// Adapter reads the feed. Config: {}.
type Adapter struct {
	Client  *crawl.Client
	BaseURL string // overrides https://remoteok.com (for tests)
}

func New(c *crawl.Client) *Adapter { return &Adapter{Client: c} }

func (a *Adapter) Validate(config map[string]any) error { return ingest.CheckConfig(config) }

type job struct {
	ID          string `json:"id"`
	Epoch       int64  `json:"epoch"`
	Company     string `json:"company"`
	Position    string `json:"position"`
	Description string `json:"description"`
	Location    string `json:"location"`
	URL         string `json:"url"`
}

func (a *Adapter) Fetch(ctx context.Context, config map[string]any) ([]ingest.RawListing, error) {
	if err := a.Validate(config); err != nil {
		return nil, err
	}
	base := a.BaseURL
	if base == "" {
		base = "https://remoteok.com"
	}
	// The first element is the legal notice, the rest are jobs; the notice has
	// no id or position and is skipped below.
	var items []job
	if err := ingest.FetchJSON(ctx, a.Client, base+"/api", &items); err != nil {
		return nil, err
	}
	out := make([]ingest.RawListing, 0, len(items))
	for _, j := range items {
		if j.ID == "" || j.Position == "" {
			continue
		}
		l := ingest.RawListing{
			ExternalID:  j.ID,
			URL:         j.URL,
			Title:       j.Position,
			Company:     j.Company,
			Location:    j.Location,
			Description: j.Description,
			Arrangement: "remote",
		}
		if j.Epoch > 0 {
			t := time.Unix(j.Epoch, 0).UTC()
			l.PostedAt = &t
		}
		out = append(out, l)
	}
	return out, ingest.Incomplete("remote job feed: jobs that left the feed are not closed")
}
