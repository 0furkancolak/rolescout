// Package weworkremotely reads the RSS feeds of We Work Remotely
// (https://weworkremotely.com/remote-jobs.rss and its category feeds).
// See docs/sources/weworkremotely.md.
package weworkremotely

import (
	"context"
	"encoding/xml"
	"fmt"
	"regexp"
	"strings"
	"time"

	"example.com/rolescout/api/internal/crawl"
	"example.com/rolescout/api/internal/ingest"
)

// Adapter reads one feed. Config: {} for all jobs, or {"category":
// "remote-programming-jobs"} for a category feed.
type Adapter struct {
	Client  *crawl.Client
	BaseURL string // overrides https://weworkremotely.com (for tests)
}

func New(c *crawl.Client) *Adapter { return &Adapter{Client: c} }

var category = regexp.MustCompile(`^[a-z0-9-]{1,60}$`)

func (a *Adapter) Validate(config map[string]any) error {
	return ingest.CheckConfig(config, ingest.Field{Name: "category", Pattern: category, Optional: true})
}

type feed struct {
	Items []struct {
		Title       string `xml:"title"`
		Region      string `xml:"region"`
		Country     string `xml:"country"`
		State       string `xml:"state"`
		Skills      string `xml:"skills"`
		Type        string `xml:"type"`
		Description string `xml:"description"`
		PubDate     string `xml:"pubDate"`
		GUID        string `xml:"guid"`
		Link        string `xml:"link"`
	} `xml:"channel>item"`
}

func (a *Adapter) Fetch(ctx context.Context, config map[string]any) ([]ingest.RawListing, error) {
	if err := a.Validate(config); err != nil {
		return nil, err
	}
	base := a.BaseURL
	if base == "" {
		base = "https://weworkremotely.com"
	}
	u := base + "/remote-jobs.rss"
	if c := ingest.ConfigString(config, "category"); c != "" {
		u = base + "/categories/" + c + ".rss"
	}
	body, err := ingest.Fetch(ctx, a.Client, u, "application/rss+xml, application/xml")
	if err != nil {
		return nil, err
	}
	var f feed
	if err := xml.Unmarshal(body, &f); err != nil {
		return nil, fmt.Errorf("decode feed: %w", err)
	}
	out := make([]ingest.RawListing, 0, len(f.Items))
	for _, it := range f.Items {
		id := it.GUID
		if id == "" {
			id = it.Link
		}
		// Titles read "Company: Job title".
		company, title, ok := strings.Cut(it.Title, ": ")
		if !ok {
			company, title = "", it.Title
		}
		l := ingest.RawListing{
			ExternalID:     id,
			URL:            it.Link,
			Title:          strings.TrimSpace(title),
			Company:        strings.TrimSpace(company),
			Location:       strings.Join(nonEmpty(it.Region, it.Country, it.State), ", "),
			Description:    it.Description + " " + it.Skills,
			EmploymentType: it.Type,
			Arrangement:    "remote",
		}
		if t, err := time.Parse(time.RFC1123Z, strings.TrimSpace(it.PubDate)); err == nil {
			t = t.UTC()
			l.PostedAt = &t
		}
		out = append(out, l)
	}
	return out, ingest.Incomplete("remote job feed: jobs that left the feed are not closed")
}

func nonEmpty(s ...string) []string {
	var out []string
	for _, v := range s {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}
