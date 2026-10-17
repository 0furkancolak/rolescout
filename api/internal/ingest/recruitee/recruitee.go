// Package recruitee reads public job offers through the careers site API that
// every Recruitee company exposes at {subdomain}.recruitee.com/api/offers/.
// See docs/sources/recruitee.md.
package recruitee

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"example.com/rolescout/api/internal/crawl"
	"example.com/rolescout/api/internal/ingest"
)

// Adapter fetches one company per source. Config: {"subdomain": "acme"}.
type Adapter struct {
	Client  *crawl.Client
	BaseURL string // overrides https://{subdomain}.recruitee.com (for tests)
}

func New(c *crawl.Client) *Adapter { return &Adapter{Client: c} }

func (a *Adapter) Validate(config map[string]any) error {
	return ingest.CheckConfig(config, ingest.Field{Name: "subdomain", Pattern: ingest.Subdomain})
}

func (a *Adapter) base(subdomain string) string {
	if a.BaseURL != "" {
		return a.BaseURL
	}
	return "https://" + subdomain + ".recruitee.com"
}

type offersResponse struct {
	Offers []struct {
		ID           int64  `json:"id"`
		Title        string `json:"title"`
		CareersURL   string `json:"careers_url"`
		Location     string `json:"location"`
		CountryCode  string `json:"country_code"`
		Remote       bool   `json:"remote"`
		Hybrid       bool   `json:"hybrid"`
		OnSite       bool   `json:"on_site"`
		PublishedAt  string `json:"published_at"`
		Description  string `json:"description"`
		Requirements string `json:"requirements"`
		CompanyName  string `json:"company_name"`
		Employment   string `json:"employment_type_code"`
		Status       string `json:"status"`
	} `json:"offers"`
}

func (a *Adapter) Fetch(ctx context.Context, config map[string]any) ([]ingest.RawListing, error) {
	if err := a.Validate(config); err != nil {
		return nil, err
	}
	sub := ingest.ConfigString(config, "subdomain")
	var resp offersResponse
	if err := ingest.FetchJSON(ctx, a.Client, a.base(sub)+"/api/offers/", &resp); err != nil {
		return nil, fmt.Errorf("list offers: %w", err)
	}
	out := make([]ingest.RawListing, 0, len(resp.Offers))
	for _, o := range resp.Offers {
		if o.Status != "" && o.Status != "published" {
			continue
		}
		company := ingest.ConfigString(config, "company")
		if company == "" {
			company = o.CompanyName
		}
		l := ingest.RawListing{
			ExternalID:     strconv.FormatInt(o.ID, 10),
			URL:            o.CareersURL,
			Title:          o.Title,
			Company:        company,
			Location:       o.Location,
			Description:    o.Description + o.Requirements,
			EmploymentType: o.Employment,
			CountryCode:    o.CountryCode,
		}
		switch {
		case o.Remote:
			l.Arrangement = "remote"
		case o.Hybrid:
			l.Arrangement = "hybrid"
		case o.OnSite:
			l.Arrangement = "onsite"
		}
		if t, err := time.Parse("2006-01-02 15:04:05 MST", o.PublishedAt); err == nil {
			t = t.UTC()
			l.PostedAt = &t
		}
		out = append(out, l)
	}
	return out, nil
}
