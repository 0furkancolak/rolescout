// Package workday reads job lists from Workday career sites through the JSON
// endpoint those sites call themselves ({host}/wday/cxs/{tenant}/{site}/jobs).
// It is not a documented public API, so the adapter type ships without a
// review; an administrator must review and enable it. See docs/sources/workday.md.
package workday

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"example.com/rolescout/api/internal/crawl"
	"example.com/rolescout/api/internal/ingest"
)

var (
	hostPattern = regexp.MustCompile(`^[a-z0-9-]+\.wd[0-9]+\.myworkdayjobs\.com$`)
	namePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,100}$`)
	daysAgo     = regexp.MustCompile(`(\d+)\+? Days? Ago`)
)

// Adapter fetches one career site per source. Config:
//
//	{"host": "acme.wd5.myworkdayjobs.com", "tenant": "acme", "site": "External"}
//
// The list has no descriptions; listings link to the career site.
type Adapter struct {
	Client   *crawl.Client
	BaseURL  string // overrides https://{host} (for tests)
	PageSize int    // default 20, Workday's maximum
	MaxPages int    // 0 reads every page
	Now      func() time.Time
}

func New(c *crawl.Client) *Adapter { return &Adapter{Client: c} }

func (a *Adapter) Validate(config map[string]any) error {
	return ingest.CheckConfig(config,
		ingest.Field{Name: "host", Pattern: hostPattern},
		ingest.Field{Name: "tenant", Pattern: namePattern},
		ingest.Field{Name: "site", Pattern: namePattern})
}

type jobsResponse struct {
	Total    int `json:"total"`
	Postings []struct {
		Title         string   `json:"title"`
		ExternalPath  string   `json:"externalPath"`
		LocationsText string   `json:"locationsText"`
		PostedOn      string   `json:"postedOn"`
		BulletFields  []string `json:"bulletFields"`
	} `json:"jobPostings"`
}

var multiLocation = regexp.MustCompile(`^\d+ Locations$`)

func (a *Adapter) Fetch(ctx context.Context, config map[string]any) ([]ingest.RawListing, error) {
	if err := a.Validate(config); err != nil {
		return nil, err
	}
	host, tenant, site := ingest.ConfigString(config, "host"), ingest.ConfigString(config, "tenant"), ingest.ConfigString(config, "site")
	base := a.BaseURL
	if base == "" {
		base = "https://" + host
	}
	company := ingest.ConfigString(config, "company")
	if company == "" {
		company = tenant
	}
	size := a.PageSize
	if size <= 0 {
		size = 20
	}
	now := time.Now
	if a.Now != nil {
		now = a.Now
	}
	endpoint := base + "/wday/cxs/" + url.PathEscape(tenant) + "/" + url.PathEscape(site) + "/jobs"
	var out []ingest.RawListing
	total := 0
	for page := 0; a.MaxPages <= 0 || page < a.MaxPages; page++ {
		var resp jobsResponse
		body := map[string]any{"limit": size, "offset": page * size, "searchText": "", "appliedFacets": map[string]any{}}
		if err := ingest.PostJSON(ctx, a.Client, endpoint, body, &resp); err != nil {
			return nil, fmt.Errorf("list jobs (offset %d): %w", page*size, err)
		}
		for _, p := range resp.Postings {
			id := p.ExternalPath
			if len(p.BulletFields) > 0 && p.BulletFields[0] != "" {
				id = p.BulletFields[0]
			}
			location := p.LocationsText
			if multiLocation.MatchString(location) || location == "" {
				location = pathLocation(p.ExternalPath)
			}
			out = append(out, ingest.RawListing{
				ExternalID: id,
				URL:        base + "/" + site + p.ExternalPath,
				Title:      p.Title,
				Company:    company,
				Location:   location,
				PostedAt:   postedOn(p.PostedOn, now()),
			})
		}
		// Workday reports the total on the first page only.
		if page == 0 {
			total = resp.Total
		}
		if len(resp.Postings) == 0 || (page+1)*size >= total {
			return out, nil
		}
	}
	return out, ingest.Incomplete("page limit of %d reached (%d of %d jobs read)", a.MaxPages, len(out), total)
}

// pathLocation reads the primary location from a job path such as
// "/job/US-CA-Santa-Clara/Title_JR1".
func pathLocation(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 2 {
		return ""
	}
	return strings.ReplaceAll(parts[1], "-", " ")
}

// postedOn turns Workday's "Posted Today", "Posted Yesterday" or "Posted N
// Days Ago" into a date.
func postedOn(s string, now time.Time) *time.Time {
	day := now.UTC().Truncate(24 * time.Hour)
	var t time.Time
	switch {
	case strings.Contains(s, "Today"):
		t = day
	case strings.Contains(s, "Yesterday"):
		t = day.AddDate(0, 0, -1)
	default:
		m := daysAgo.FindStringSubmatch(s)
		if m == nil {
			return nil
		}
		n, _ := strconv.Atoi(m[1])
		t = day.AddDate(0, 0, -n)
	}
	return &t
}
