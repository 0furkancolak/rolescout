// Package gem reads public job boards hosted by Gem (jobs.gem.com/{board})
// through its Greenhouse-compatible job board API at api.gem.com/job_board/v0.
// See docs/sources/gem.md.
package gem

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"example.com/rolescout/api/internal/crawl"
	"example.com/rolescout/api/internal/ingest"
)

const DefaultBaseURL = "https://api.gem.com"

var boardPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,99}$`)

// Adapter fetches one board per source. Config: {"board": "acme"}.
type Adapter struct {
	Client  *crawl.Client
	BaseURL string
}

func New(c *crawl.Client) *Adapter { return &Adapter{Client: c, BaseURL: DefaultBaseURL} }

func (a *Adapter) Validate(config map[string]any) error {
	return ingest.CheckConfig(config, ingest.Field{Name: "board", Pattern: boardPattern})
}

type post struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	URL          string `json:"absolute_url"`
	Content      string `json:"content"`
	Employment   string `json:"employment_type"`
	LocationType string `json:"location_type"`
	Published    string `json:"first_published_at"`
	Updated      string `json:"updated_at"`
	Location     struct {
		Name string `json:"name"`
	} `json:"location"`
}

func (a *Adapter) Fetch(ctx context.Context, config map[string]any) ([]ingest.RawListing, error) {
	if err := a.Validate(config); err != nil {
		return nil, err
	}
	board := ingest.ConfigString(config, "board")
	var posts []post
	if err := ingest.FetchJSON(ctx, a.Client, a.BaseURL+"/job_board/v0/"+board+"/job_posts/", &posts); err != nil {
		return nil, fmt.Errorf("list job posts: %w", err)
	}
	company := ingest.ConfigString(config, "company")
	if company == "" {
		company = board
	}
	out := make([]ingest.RawListing, 0, len(posts))
	for _, p := range posts {
		l := ingest.RawListing{
			ExternalID:     p.ID,
			URL:            p.URL,
			Title:          p.Title,
			Company:        company,
			Location:       p.Location.Name,
			Description:    p.Content,
			EmploymentType: strings.ReplaceAll(p.Employment, "_", " "),
		}
		switch strings.ToLower(p.LocationType) {
		case "remote":
			l.Arrangement = "remote"
		case "hybrid":
			l.Arrangement = "hybrid"
		case "in_office", "onsite", "on_site":
			l.Arrangement = "onsite"
		}
		if t, err := time.Parse(time.RFC3339, p.Published); err == nil {
			t = t.UTC()
			l.PostedAt = &t
		}
		if t, err := time.Parse(time.RFC3339, p.Updated); err == nil {
			t = t.UTC()
			l.SourceUpdatedAt = &t
		}
		out = append(out, l)
	}
	return out, nil
}
