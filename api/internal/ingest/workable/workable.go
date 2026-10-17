// Package workable reads public job lists through Workable's jobs widget API,
// which Workable offers for embedding a company's open jobs on its own site.
// See docs/sources/workable.md.
package workable

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"example.com/rolescout/api/internal/crawl"
	"example.com/rolescout/api/internal/ingest"
)

// Adapter fetches one account per source. Config: {"account": "acme"}.
type Adapter struct {
	Client  *crawl.Client
	BaseURL string // overrides https://apply.workable.com (for tests)
}

func New(c *crawl.Client) *Adapter { return &Adapter{Client: c} }

func (a *Adapter) Validate(config map[string]any) error {
	return ingest.CheckConfig(config, ingest.Field{Name: "account", Pattern: ingest.BoardID})
}

type accountResponse struct {
	Name string `json:"name"`
	Jobs []struct {
		Title          string `json:"title"`
		Shortcode      string `json:"shortcode"`
		EmploymentType string `json:"employment_type"`
		Telecommuting  bool   `json:"telecommuting"`
		URL            string `json:"url"`
		PublishedOn    string `json:"published_on"`
		Country        string `json:"country"`
		City           string `json:"city"`
		State          string `json:"state"`
		Description    string `json:"description"`
		Locations      []struct {
			CountryCode string `json:"countryCode"`
		} `json:"locations"`
	} `json:"jobs"`
}

func (a *Adapter) Fetch(ctx context.Context, config map[string]any) ([]ingest.RawListing, error) {
	if err := a.Validate(config); err != nil {
		return nil, err
	}
	account := ingest.ConfigString(config, "account")
	base := a.BaseURL
	if base == "" {
		base = "https://apply.workable.com"
	}
	var resp accountResponse
	if err := ingest.FetchJSON(ctx, a.Client, base+"/api/v1/widget/accounts/"+url.PathEscape(account)+"?details=true", &resp); err != nil {
		return nil, fmt.Errorf("list jobs: %w", err)
	}
	company := ingest.ConfigString(config, "company")
	if company == "" {
		company = resp.Name
	}
	out := make([]ingest.RawListing, 0, len(resp.Jobs))
	for _, j := range resp.Jobs {
		var parts []string
		for _, p := range []string{j.City, j.State, j.Country} {
			if p != "" {
				parts = append(parts, p)
			}
		}
		l := ingest.RawListing{
			ExternalID:     j.Shortcode,
			URL:            j.URL,
			Title:          j.Title,
			Company:        company,
			Location:       strings.Join(parts, ", "),
			Description:    j.Description,
			EmploymentType: j.EmploymentType,
			CountryCode:    j.Country,
		}
		if len(j.Locations) > 0 && j.Locations[0].CountryCode != "" {
			l.CountryCode = j.Locations[0].CountryCode
		}
		if j.Telecommuting {
			l.Arrangement = "remote"
		}
		if t, err := time.Parse("2006-01-02", j.PublishedOn); err == nil {
			l.PostedAt = &t
		}
		out = append(out, l)
	}
	return out, nil
}
