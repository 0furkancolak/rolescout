// Package lever reads public job postings through Lever's Postings API
// (https://github.com/lever/postings-api), which Lever offers for publishing a
// company's open jobs. See docs/sources/lever.md.
package lever

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"example.com/rolescout/api/internal/crawl"
	"example.com/rolescout/api/internal/ingest"
)

// Adapter fetches one Lever site per source. Config:
//
//	{"site": "acme", "region": "eu", "company": "Acme"}
//
// region "eu" uses Lever's EU data centre; company defaults to the site name.
type Adapter struct {
	Client *crawl.Client
	// BaseURL overrides the API origin (for tests).
	BaseURL string
}

func New(c *crawl.Client) *Adapter { return &Adapter{Client: c} }

func (a *Adapter) Validate(config map[string]any) error {
	return ingest.CheckConfig(config,
		ingest.Field{Name: "site", Pattern: ingest.BoardID},
		ingest.Field{Name: "region", OneOf: []string{"eu"}, Optional: true})
}

type posting struct {
	ID          string `json:"id"`
	Text        string `json:"text"`
	HostedURL   string `json:"hostedUrl"`
	CreatedAt   int64  `json:"createdAt"`
	Country     string `json:"country"`
	Workplace   string `json:"workplaceType"`
	Description string `json:"description"`
	Additional  string `json:"additional"`
	Categories  struct {
		Commitment string `json:"commitment"`
		Location   string `json:"location"`
	} `json:"categories"`
	Lists []struct {
		Text    string `json:"text"`
		Content string `json:"content"`
	} `json:"lists"`
}

func (a *Adapter) Fetch(ctx context.Context, config map[string]any) ([]ingest.RawListing, error) {
	if err := a.Validate(config); err != nil {
		return nil, err
	}
	site := ingest.ConfigString(config, "site")
	base := a.BaseURL
	if base == "" {
		base = "https://api.lever.co"
		if ingest.ConfigString(config, "region") == "eu" {
			base = "https://api.eu.lever.co"
		}
	}
	company := ingest.ConfigString(config, "company")
	if company == "" {
		company = site
	}
	var postings []posting
	if err := ingest.FetchJSON(ctx, a.Client, base+"/v0/postings/"+url.PathEscape(site)+"?mode=json", &postings); err != nil {
		return nil, fmt.Errorf("list postings: %w", err)
	}
	out := make([]ingest.RawListing, 0, len(postings))
	for _, p := range postings {
		var b strings.Builder
		b.WriteString(p.Description)
		for _, l := range p.Lists {
			fmt.Fprintf(&b, "<h3>%s</h3><ul>%s</ul>", l.Text, l.Content)
		}
		b.WriteString(p.Additional)
		l := ingest.RawListing{
			ExternalID:     p.ID,
			URL:            p.HostedURL,
			Title:          p.Text,
			Company:        company,
			Location:       p.Categories.Location,
			Description:    b.String(),
			EmploymentType: p.Categories.Commitment,
			CountryCode:    p.Country,
			Arrangement:    arrangement(p.Workplace),
		}
		if p.CreatedAt > 0 {
			t := time.UnixMilli(p.CreatedAt).UTC()
			l.PostedAt = &t
		}
		out = append(out, l)
	}
	return out, nil
}

func arrangement(workplace string) string {
	switch strings.ToLower(workplace) {
	case "remote":
		return "remote"
	case "hybrid":
		return "hybrid"
	case "onsite", "on-site":
		return "onsite"
	}
	return ""
}
