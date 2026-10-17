// Package personio reads a company's public job feed, the XML export every
// Personio career page publishes at {subdomain}.jobs.personio.{de,com}/xml.
// See docs/sources/personio.md.
package personio

import (
	"context"
	"encoding/xml"
	"fmt"
	"strings"
	"time"

	"example.com/rolescout/api/internal/crawl"
	"example.com/rolescout/api/internal/ingest"
)

// Adapter fetches one company per source. Config: {"subdomain": "acme", "tld": "de"};
// tld is "de" (default) or "com".
type Adapter struct {
	Client  *crawl.Client
	BaseURL string // overrides https://{subdomain}.jobs.personio.{tld} (for tests)
}

func New(c *crawl.Client) *Adapter { return &Adapter{Client: c} }

func (a *Adapter) Validate(config map[string]any) error {
	return ingest.CheckConfig(config,
		ingest.Field{Name: "subdomain", Pattern: ingest.Subdomain},
		ingest.Field{Name: "tld", OneOf: []string{"de", "com"}, Optional: true})
}

func (a *Adapter) base(subdomain, tld string) string {
	if a.BaseURL != "" {
		return a.BaseURL
	}
	if tld == "" {
		tld = "de"
	}
	return "https://" + subdomain + ".jobs.personio." + tld
}

type feed struct {
	Positions []struct {
		ID           string   `xml:"id"`
		Subcompany   string   `xml:"subcompany"`
		Office       string   `xml:"office"`
		Additional   []string `xml:"additionalOffices>office"`
		Name         string   `xml:"name"`
		Schedule     string   `xml:"schedule"`
		CreatedAt    string   `xml:"createdAt"`
		Descriptions []struct {
			Name  string `xml:"name"`
			Value string `xml:"value"`
		} `xml:"jobDescriptions>jobDescription"`
	} `xml:"position"`
}

func (a *Adapter) Fetch(ctx context.Context, config map[string]any) ([]ingest.RawListing, error) {
	if err := a.Validate(config); err != nil {
		return nil, err
	}
	sub := ingest.ConfigString(config, "subdomain")
	base := a.base(sub, ingest.ConfigString(config, "tld"))
	body, err := ingest.Fetch(ctx, a.Client, base+"/xml?language=en", "application/xml")
	if err != nil {
		return nil, fmt.Errorf("read job feed: %w", err)
	}
	var f feed
	if err := xml.Unmarshal(body, &f); err != nil {
		return nil, fmt.Errorf("decode job feed: %w", err)
	}
	out := make([]ingest.RawListing, 0, len(f.Positions))
	for _, p := range f.Positions {
		company := ingest.ConfigString(config, "company")
		if company == "" {
			company = p.Subcompany
		}
		if company == "" {
			company = sub
		}
		var desc strings.Builder
		for _, d := range p.Descriptions {
			fmt.Fprintf(&desc, "<h3>%s</h3>%s", d.Name, d.Value)
		}
		offices := append([]string{p.Office}, p.Additional...)
		l := ingest.RawListing{
			ExternalID:     strings.TrimSpace(p.ID),
			URL:            base + "/job/" + strings.TrimSpace(p.ID),
			Title:          p.Name,
			Company:        company,
			Location:       strings.Join(nonEmpty(offices), ", "),
			Description:    desc.String(),
			EmploymentType: p.Schedule,
		}
		if t, err := time.Parse(time.RFC3339, strings.TrimSpace(p.CreatedAt)); err == nil {
			t = t.UTC()
			l.PostedAt = &t
		}
		out = append(out, l)
	}
	return out, nil
}

func nonEmpty(ss []string) []string {
	var out []string
	for _, s := range ss {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}
