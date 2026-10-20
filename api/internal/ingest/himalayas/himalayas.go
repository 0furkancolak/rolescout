// Package himalayas reads Himalayas' public remote-jobs API
// (https://himalayas.app/jobs/api). See docs/sources/himalayas.md.
package himalayas

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

// DefaultMaxPages bounds the pages read (20 jobs each).
const DefaultMaxPages = 15

// Adapter reads the newest jobs. Config: {} or {"max_pages": "30"}.
type Adapter struct {
	Client  *crawl.Client
	BaseURL string // overrides https://himalayas.app (for tests)
}

func New(c *crawl.Client) *Adapter { return &Adapter{Client: c} }

var digits = regexp.MustCompile(`^[0-9]{1,3}$`)

func (a *Adapter) Validate(config map[string]any) error {
	return ingest.CheckConfig(config, ingest.Field{Name: "max_pages", Pattern: digits, Optional: true})
}

type response struct {
	NextCursor string `json:"nextCursor"`
	Jobs       []struct {
		Title       string   `json:"title"`
		Company     string   `json:"companyName"`
		Employment  string   `json:"employmentType"`
		Locations   []string `json:"locationRestrictions"`
		Description string   `json:"description"`
		Excerpt     string   `json:"excerpt"`
		PubDate     int64    `json:"pubDate"`
		GUID        string   `json:"guid"`
	} `json:"jobs"`
}

func (a *Adapter) Fetch(ctx context.Context, config map[string]any) ([]ingest.RawListing, error) {
	if err := a.Validate(config); err != nil {
		return nil, err
	}
	base := a.BaseURL
	if base == "" {
		base = "https://himalayas.app"
	}
	limit := DefaultMaxPages
	if n, err := strconv.Atoi(ingest.ConfigString(config, "max_pages")); err == nil && n > 0 {
		limit = n
	}
	var out []ingest.RawListing
	cursor := ""
	for p := 1; p <= limit; p++ {
		u := base + "/jobs/api?limit=20"
		if cursor != "" {
			u += "&cursor=" + url.QueryEscape(cursor)
		}
		var resp response
		if err := ingest.FetchJSON(ctx, a.Client, u, &resp); err != nil {
			if p == 1 {
				return nil, err
			}
			return out, ingest.Incomplete("page %d could not be read: %v", p, err)
		}
		for _, j := range resp.Jobs {
			if j.GUID == "" {
				continue
			}
			desc := j.Description
			if desc == "" {
				desc = j.Excerpt
			}
			loc := "Worldwide"
			if len(j.Locations) > 0 {
				loc = strings.Join(j.Locations, ", ")
			}
			l := ingest.RawListing{
				ExternalID:     j.GUID,
				URL:            j.GUID, // the plain job page; applicationLink carries tracking
				Title:          j.Title,
				Company:        j.Company,
				Location:       loc,
				Description:    desc,
				EmploymentType: j.Employment,
				Arrangement:    "remote",
			}
			if j.PubDate > 0 {
				t := time.Unix(j.PubDate, 0).UTC()
				l.PostedAt = &t
			}
			out = append(out, l)
		}
		cursor = resp.NextCursor
		if cursor == "" || len(resp.Jobs) == 0 {
			break
		}
	}
	return out, ingest.Incomplete("remote job feed: read the newest %d pages at most, other jobs are not closed", limit)
}
