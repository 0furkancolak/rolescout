// Package jobicy reads Jobicy's public remote-jobs API
// (https://jobicy.com/api/v2/remote-jobs). See docs/sources/jobicy.md.
package jobicy

import (
	"context"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"example.com/rolescout/api/internal/crawl"
	"example.com/rolescout/api/internal/ingest"
)

// Adapter reads the newest jobs. Config: {} or {"industry": "engineering",
// "geo": "turkey", "tag": "go"}.
type Adapter struct {
	Client  *crawl.Client
	BaseURL string // overrides https://jobicy.com (for tests)
}

func New(c *crawl.Client) *Adapter { return &Adapter{Client: c} }

var word = regexp.MustCompile(`^[a-z0-9+#.-]{1,40}$`)

func (a *Adapter) Validate(config map[string]any) error {
	return ingest.CheckConfig(config,
		ingest.Field{Name: "industry", Pattern: word, Optional: true},
		ingest.Field{Name: "geo", Pattern: word, Optional: true},
		ingest.Field{Name: "tag", Pattern: word, Optional: true})
}

type response struct {
	Jobs []struct {
		ID          int64    `json:"id"`
		URL         string   `json:"url"`
		Title       string   `json:"jobTitle"`
		Company     string   `json:"companyName"`
		Type        []string `json:"jobType"`
		Geo         string   `json:"jobGeo"`
		Level       string   `json:"jobLevel"`
		Excerpt     string   `json:"jobExcerpt"`
		Description string   `json:"jobDescription"`
		PubDate     string   `json:"pubDate"`
	} `json:"jobs"`
}

func (a *Adapter) Fetch(ctx context.Context, config map[string]any) ([]ingest.RawListing, error) {
	if err := a.Validate(config); err != nil {
		return nil, err
	}
	base := a.BaseURL
	if base == "" {
		base = "https://jobicy.com"
	}
	q := url.Values{"count": {"100"}}
	for _, k := range []string{"industry", "geo", "tag"} {
		if v := ingest.ConfigString(config, k); v != "" {
			q.Set(k, v)
		}
	}
	var resp response
	if err := ingest.FetchJSON(ctx, a.Client, base+"/api/v2/remote-jobs?"+q.Encode(), &resp); err != nil {
		return nil, err
	}
	out := make([]ingest.RawListing, 0, len(resp.Jobs))
	for _, j := range resp.Jobs {
		desc := j.Description
		if desc == "" {
			desc = j.Excerpt
		}
		l := ingest.RawListing{
			ExternalID:     strconv.FormatInt(j.ID, 10),
			URL:            j.URL,
			Title:          j.Title,
			Company:        j.Company,
			Location:       strings.Join(strings.Fields(strings.ReplaceAll(j.Geo, ",", ", ")), " "),
			Description:    desc + " " + j.Level,
			EmploymentType: strings.Join(j.Type, " "),
			Arrangement:    "remote",
		}
		if t, err := time.Parse(time.RFC3339, j.PubDate); err == nil {
			t = t.UTC()
			l.PostedAt = &t
		}
		out = append(out, l)
	}
	return out, ingest.Incomplete("remote job feed: jobs that left the feed are not closed")
}
