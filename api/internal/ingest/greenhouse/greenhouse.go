// Package greenhouse reads public job boards through the Greenhouse Job Board
// API (https://docs.greenhouse.io/job-board.html). Its GET endpoints are
// public and meant for publishing a company's open jobs; no credentials are
// used. See docs/sources/greenhouse.md for the access review.
package greenhouse

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"time"

	"example.com/rolescout/api/internal/crawl"
	"example.com/rolescout/api/internal/ingest"
)

const DefaultBaseURL = "https://boards-api.greenhouse.io"

var boardTokenPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,100}$`)

// Adapter fetches one board per source. Source config:
//
//	{"board_token": "acme", "company": "Acme Inc."}
//
// company is optional; the board's own name is used when it is missing.
type Adapter struct {
	BaseURL string
	Client  *crawl.Client
}

func New(c *crawl.Client) *Adapter {
	return &Adapter{BaseURL: DefaultBaseURL, Client: c}
}

func (a *Adapter) Validate(config map[string]any) error {
	token, err := ingest.StringConfig(config, "board_token")
	if err != nil {
		return err
	}
	if !boardTokenPattern.MatchString(token) {
		return errors.New(`config field "board_token" may only contain letters, digits, "-" and "_"`)
	}
	if v, ok := config["company"]; ok {
		if s, ok := v.(string); !ok || s == "" {
			return errors.New(`config field "company" must be a non-empty string when set`)
		}
	}
	for key := range config {
		if key != "board_token" && key != "company" {
			return fmt.Errorf("unknown config field %q", key)
		}
	}
	return nil
}

type board struct {
	Name string `json:"name"`
}

type jobsResponse struct {
	Jobs []job `json:"jobs"`
}

type job struct {
	ID             int64      `json:"id"`
	Title          string     `json:"title"`
	UpdatedAt      *time.Time `json:"updated_at"`
	FirstPublished *time.Time `json:"first_published"`
	AbsoluteURL    string     `json:"absolute_url"`
	Content        string     `json:"content"`
	Location       struct {
		Name string `json:"name"`
	} `json:"location"`
}

func (a *Adapter) Fetch(ctx context.Context, config map[string]any) ([]ingest.RawListing, error) {
	if err := a.Validate(config); err != nil {
		return nil, err
	}
	token := config["board_token"].(string)
	company, _ := config["company"].(string)
	if company == "" {
		var b board
		if err := a.get(ctx, "/v1/boards/"+url.PathEscape(token), &b); err != nil {
			return nil, fmt.Errorf("read board: %w", err)
		}
		company = b.Name
	}

	var resp jobsResponse
	if err := a.get(ctx, "/v1/boards/"+url.PathEscape(token)+"/jobs?content=true", &resp); err != nil {
		return nil, fmt.Errorf("list jobs: %w", err)
	}
	out := make([]ingest.RawListing, 0, len(resp.Jobs))
	for _, j := range resp.Jobs {
		out = append(out, ingest.RawListing{
			ExternalID:      strconv.FormatInt(j.ID, 10),
			URL:             j.AbsoluteURL,
			Title:           j.Title,
			Company:         company,
			Location:        j.Location.Name,
			Description:     j.Content,
			PostedAt:        utc(j.FirstPublished),
			SourceUpdatedAt: utc(j.UpdatedAt),
		})
	}
	return out, nil
}

func (a *Adapter) get(ctx context.Context, path string, dst any) error {
	return ingest.FetchJSON(ctx, a.Client, a.BaseURL+path, dst)
}

func utc(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}
