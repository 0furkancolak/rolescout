// Package ashby reads public job boards through Ashby's Job Postings API
// (https://developers.ashbyhq.com/docs/public-job-posting-api), which Ashby
// offers for publishing a company's open jobs. See docs/sources/ashby.md.
package ashby

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"example.com/rolescout/api/internal/crawl"
	"example.com/rolescout/api/internal/ingest"
)

// Adapter fetches one board per source. Config: {"board": "acme", "company": "Acme"}.
type Adapter struct {
	Client  *crawl.Client
	BaseURL string // overrides https://api.ashbyhq.com (for tests)
}

func New(c *crawl.Client) *Adapter { return &Adapter{Client: c} }

func (a *Adapter) Validate(config map[string]any) error {
	return ingest.CheckConfig(config, ingest.Field{Name: "board", Pattern: ingest.BoardID})
}

type boardResponse struct {
	Jobs []struct {
		ID             string     `json:"id"`
		Title          string     `json:"title"`
		Location       string     `json:"location"`
		EmploymentType string     `json:"employmentType"`
		PublishedAt    *time.Time `json:"publishedAt"`
		IsListed       bool       `json:"isListed"`
		IsRemote       bool       `json:"isRemote"`
		WorkplaceType  string     `json:"workplaceType"`
		JobURL         string     `json:"jobUrl"`
		Description    string     `json:"descriptionHtml"`
		Address        struct {
			PostalAddress struct {
				Country string `json:"addressCountry"`
			} `json:"postalAddress"`
		} `json:"address"`
	} `json:"jobs"`
}

var employmentTypes = map[string]string{
	"FullTime": "Full-time", "PartTime": "Part-time", "Contract": "Contract",
	"Intern": "Internship", "Temporary": "Temporary",
}

func (a *Adapter) Fetch(ctx context.Context, config map[string]any) ([]ingest.RawListing, error) {
	if err := a.Validate(config); err != nil {
		return nil, err
	}
	board := ingest.ConfigString(config, "board")
	base := a.BaseURL
	if base == "" {
		base = "https://api.ashbyhq.com"
	}
	company := ingest.ConfigString(config, "company")
	if company == "" {
		company = board
	}
	var resp boardResponse
	if err := ingest.FetchJSON(ctx, a.Client, base+"/posting-api/job-board/"+url.PathEscape(board)+"?includeCompensation=false", &resp); err != nil {
		return nil, fmt.Errorf("list jobs: %w", err)
	}
	out := make([]ingest.RawListing, 0, len(resp.Jobs))
	for _, j := range resp.Jobs {
		if !j.IsListed {
			continue
		}
		arrangement := ""
		switch strings.ToLower(j.WorkplaceType) {
		case "remote":
			arrangement = "remote"
		case "hybrid":
			arrangement = "hybrid"
		case "onsite":
			arrangement = "onsite"
		}
		if arrangement == "" && j.IsRemote {
			arrangement = "remote"
		}
		var posted *time.Time
		if j.PublishedAt != nil {
			t := j.PublishedAt.UTC()
			posted = &t
		}
		out = append(out, ingest.RawListing{
			ExternalID:     j.ID,
			URL:            j.JobURL,
			Title:          j.Title,
			Company:        company,
			Location:       j.Location,
			Description:    j.Description,
			EmploymentType: employmentTypes[j.EmploymentType],
			PostedAt:       posted,
			CountryCode:    j.Address.PostalAddress.Country,
			Arrangement:    arrangement,
		})
	}
	return out, nil
}
