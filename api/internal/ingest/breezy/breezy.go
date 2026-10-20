// Package breezy reads public job lists through the JSON list every Breezy HR
// career site publishes at {subdomain}.breezy.hr/json. See docs/sources/breezy.md.
package breezy

import (
	"context"
	"fmt"
	"strings"
	"time"

	"example.com/rolescout/api/internal/crawl"
	"example.com/rolescout/api/internal/ingest"
)

// Adapter fetches one company per source. Config: {"subdomain": "acme"}.
type Adapter struct {
	Client  *crawl.Client
	BaseURL string // overrides https://{subdomain}.breezy.hr (for tests)
}

func New(c *crawl.Client) *Adapter { return &Adapter{Client: c} }

func (a *Adapter) Validate(config map[string]any) error {
	return ingest.CheckConfig(config, ingest.Field{Name: "subdomain", Pattern: ingest.Subdomain})
}

func (a *Adapter) base(subdomain string) string {
	if a.BaseURL != "" {
		return a.BaseURL
	}
	return "https://" + subdomain + ".breezy.hr"
}

type position struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
	Date string `json:"published_date"`
	Type struct {
		ID string `json:"id"`
	} `json:"type"`
	Location struct {
		Name     string `json:"name"`
		IsRemote bool   `json:"is_remote"`
		Country  struct {
			ID string `json:"id"`
		} `json:"country"`
	} `json:"location"`
	Department string `json:"department"`
	Salary     string `json:"salary"`
	Company    struct {
		Name string `json:"name"`
	} `json:"company"`
}

func (a *Adapter) Fetch(ctx context.Context, config map[string]any) ([]ingest.RawListing, error) {
	if err := a.Validate(config); err != nil {
		return nil, err
	}
	sub := ingest.ConfigString(config, "subdomain")
	var jobs []position
	if err := ingest.FetchJSON(ctx, a.Client, a.base(sub)+"/json", &jobs); err != nil {
		return nil, fmt.Errorf("list positions: %w", err)
	}
	out := make([]ingest.RawListing, 0, len(jobs))
	for _, p := range jobs {
		company := ingest.ConfigString(config, "company")
		if company == "" {
			company = p.Company.Name
		}
		// The list carries no description; the facts it does carry stand in.
		var desc []string
		for _, s := range []string{p.Department, p.Salary} {
			if s = strings.TrimSpace(s); s != "" {
				desc = append(desc, "<p>"+s+"</p>")
			}
		}
		l := ingest.RawListing{
			ExternalID:     p.ID,
			URL:            p.URL,
			Title:          p.Name,
			Company:        company,
			Location:       p.Location.Name,
			CountryCode:    p.Location.Country.ID,
			Description:    strings.Join(desc, ""),
			EmploymentType: p.Type.ID,
		}
		if p.Location.IsRemote {
			l.Arrangement = "remote"
		}
		if t, err := time.Parse(time.RFC3339, p.Date); err == nil {
			t = t.UTC()
			l.PostedAt = &t
		}
		out = append(out, l)
	}
	return out, nil
}
