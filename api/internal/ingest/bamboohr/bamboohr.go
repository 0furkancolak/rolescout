// Package bamboohr reads the public job list of a BambooHR career page at
// {subdomain}.bamboohr.com/careers/list. See docs/sources/bamboohr.md.
package bamboohr

import (
	"context"
	"fmt"
	"strings"

	"example.com/rolescout/api/internal/crawl"
	"example.com/rolescout/api/internal/ingest"
)

// Adapter fetches one company per source. Config: {"subdomain": "acme", "company": "Acme"}.
// The list has no descriptions; listings link to the career page.
type Adapter struct {
	Client  *crawl.Client
	BaseURL string // overrides https://{subdomain}.bamboohr.com (for tests)
}

func New(c *crawl.Client) *Adapter { return &Adapter{Client: c} }

func (a *Adapter) Validate(config map[string]any) error {
	return ingest.CheckConfig(config, ingest.Field{Name: "subdomain", Pattern: ingest.Subdomain})
}

type listResponse struct {
	Result []struct {
		ID           string `json:"id"`
		Name         string `json:"jobOpeningName"`
		Status       string `json:"employmentStatusLabel"`
		LocationType string `json:"locationType"`
		IsRemote     *bool  `json:"isRemote"`
		Location     struct {
			City  *string `json:"city"`
			State *string `json:"state"`
		} `json:"location"`
		ATSLocation struct {
			Country  *string `json:"country"`
			State    *string `json:"state"`
			Province *string `json:"province"`
			City     *string `json:"city"`
		} `json:"atsLocation"`
	} `json:"result"`
}

func (a *Adapter) Fetch(ctx context.Context, config map[string]any) ([]ingest.RawListing, error) {
	if err := a.Validate(config); err != nil {
		return nil, err
	}
	sub := ingest.ConfigString(config, "subdomain")
	base := a.BaseURL
	if base == "" {
		base = "https://" + sub + ".bamboohr.com"
	}
	company := ingest.ConfigString(config, "company")
	if company == "" {
		company = sub
	}
	var resp listResponse
	// A company without a BambooHR career page redirects to the marketing
	// site, which is not JSON; that fails decoding and the run.
	if err := ingest.FetchJSON(ctx, a.Client, base+"/careers/list", &resp); err != nil {
		return nil, fmt.Errorf("list jobs: %w", err)
	}
	out := make([]ingest.RawListing, 0, len(resp.Result))
	for _, j := range resp.Result {
		loc := j.ATSLocation
		parts := nonEmpty(loc.City, loc.State, loc.Province, loc.Country)
		if len(parts) == 0 {
			parts = nonEmpty(j.Location.City, j.Location.State)
		}
		l := ingest.RawListing{
			ExternalID:     j.ID,
			URL:            base + "/careers/" + j.ID,
			Title:          j.Name,
			Company:        company,
			Location:       strings.Join(parts, ", "),
			EmploymentType: j.Status,
		}
		if loc.Country != nil {
			l.CountryCode = *loc.Country
		}
		switch {
		case j.LocationType == "1" || (j.IsRemote != nil && *j.IsRemote):
			l.Arrangement = "remote"
		case j.LocationType == "2":
			l.Arrangement = "hybrid"
		case j.LocationType == "0":
			l.Arrangement = "onsite"
		}
		out = append(out, l)
	}
	return out, nil
}

func nonEmpty(ss ...*string) []string {
	var out []string
	for _, s := range ss {
		if s != nil && strings.TrimSpace(*s) != "" {
			out = append(out, strings.TrimSpace(*s))
		}
	}
	return out
}
