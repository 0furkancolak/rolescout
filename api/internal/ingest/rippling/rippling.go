// Package rippling reads public job boards through Rippling's ATS board API
// (api.rippling.com/platform/api/ats/v1/board/{board}/jobs), the list behind
// ats.rippling.com/{board}/jobs. See docs/sources/rippling.md.
package rippling

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"example.com/rolescout/api/internal/crawl"
	"example.com/rolescout/api/internal/ingest"
)

const DefaultBaseURL = "https://api.rippling.com"

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

type job struct {
	UUID       string `json:"uuid"`
	Name       string `json:"name"`
	URL        string `json:"url"`
	Department struct {
		Label string `json:"label"`
	} `json:"department"`
	WorkLocation struct {
		Label string `json:"label"`
	} `json:"workLocation"`
}

func (a *Adapter) Fetch(ctx context.Context, config map[string]any) ([]ingest.RawListing, error) {
	if err := a.Validate(config); err != nil {
		return nil, err
	}
	board := ingest.ConfigString(config, "board")
	var jobs []job
	if err := ingest.FetchJSON(ctx, a.Client, a.BaseURL+"/platform/api/ats/v1/board/"+board+"/jobs", &jobs); err != nil {
		return nil, fmt.Errorf("list jobs: %w", err)
	}
	company := ingest.ConfigString(config, "company")
	if company == "" {
		company = board
	}
	out := make([]ingest.RawListing, 0, len(jobs))
	for _, j := range jobs {
		l := ingest.RawListing{
			ExternalID: j.UUID,
			URL:        j.URL,
			Title:      strings.TrimSpace(j.Name),
			Company:    company,
			Location:   strings.TrimSpace(j.WorkLocation.Label),
		}
		// The list carries no description; the department stands in.
		if d := strings.TrimSpace(j.Department.Label); d != "" {
			l.Description = "<p>" + d + "</p>"
		}
		out = append(out, l)
	}
	return out, nil
}
