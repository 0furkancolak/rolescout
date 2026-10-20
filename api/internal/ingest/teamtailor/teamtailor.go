// Package teamtailor reads public job lists through the RSS feed every
// Teamtailor career site publishes at {subdomain}.teamtailor.com/jobs.rss.
// See docs/sources/teamtailor.md.
package teamtailor

import (
	"context"
	"encoding/xml"
	"fmt"
	"strings"
	"time"

	"example.com/rolescout/api/internal/crawl"
	"example.com/rolescout/api/internal/ingest"
)

// Adapter fetches one company per source. Config: {"subdomain": "acme"}.
type Adapter struct {
	Client  *crawl.Client
	BaseURL string // overrides https://{subdomain}.teamtailor.com (for tests)
}

func New(c *crawl.Client) *Adapter { return &Adapter{Client: c} }

func (a *Adapter) Validate(config map[string]any) error {
	return ingest.CheckConfig(config, ingest.Field{Name: "subdomain", Pattern: ingest.Subdomain})
}

func (a *Adapter) base(subdomain string) string {
	if a.BaseURL != "" {
		return a.BaseURL
	}
	return "https://" + subdomain + ".teamtailor.com"
}

type feed struct {
	Channel struct {
		Title string `xml:"title"`
		Items []struct {
			Title        string `xml:"title"`
			Description  string `xml:"description"`
			PubDate      string `xml:"pubDate"`
			Link         string `xml:"link"`
			GUID         string `xml:"guid"`
			RemoteStatus string `xml:"remoteStatus"`
			Locations    []struct {
				City    string `xml:"city"`
				Name    string `xml:"name"`
				Country string `xml:"country"`
			} `xml:"locations>location"`
		} `xml:"item"`
	} `xml:"channel"`
}

func (a *Adapter) Fetch(ctx context.Context, config map[string]any) ([]ingest.RawListing, error) {
	if err := a.Validate(config); err != nil {
		return nil, err
	}
	sub := ingest.ConfigString(config, "subdomain")
	body, err := ingest.Fetch(ctx, a.Client, a.base(sub)+"/jobs.rss", "application/rss+xml, application/xml")
	if err != nil {
		return nil, fmt.Errorf("fetch feed: %w", err)
	}
	var f feed
	if err := xml.Unmarshal(body, &f); err != nil {
		return nil, fmt.Errorf("decode feed: %w", err)
	}
	company := ingest.ConfigString(config, "company")
	if company == "" {
		company = strings.TrimSpace(f.Channel.Title)
	}
	out := make([]ingest.RawListing, 0, len(f.Channel.Items))
	for _, it := range f.Channel.Items {
		l := ingest.RawListing{
			ExternalID:  strings.TrimSpace(it.GUID),
			URL:         strings.TrimSpace(it.Link),
			Title:       strings.TrimSpace(it.Title),
			Company:     company,
			Description: it.Description,
		}
		if l.ExternalID == "" {
			l.ExternalID = l.URL
		}
		var places []string
		for _, loc := range it.Locations {
			place := strings.TrimSpace(loc.City)
			if place == "" {
				place = strings.TrimSpace(loc.Name)
			}
			if c := strings.TrimSpace(loc.Country); c != "" {
				if place != "" {
					place += ", "
				}
				place += c
			}
			if place != "" {
				places = append(places, place)
			}
		}
		l.Location = strings.Join(places, "; ")
		switch strings.ToLower(strings.TrimSpace(it.RemoteStatus)) {
		case "fully", "full", "remote":
			l.Arrangement = "remote"
		case "hybrid", "temporary":
			l.Arrangement = "hybrid"
		case "none":
			l.Arrangement = "onsite"
		}
		if t, err := time.Parse(time.RFC1123Z, strings.TrimSpace(it.PubDate)); err == nil {
			t = t.UTC()
			l.PostedAt = &t
		}
		out = append(out, l)
	}
	return out, nil
}
