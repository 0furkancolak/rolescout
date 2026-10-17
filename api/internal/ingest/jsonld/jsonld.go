// Package jsonld reads job postings that career sites publish as schema.org
// JobPosting structured data (the format search engines use for job
// listings). It starts at the configured career page and follows links under
// the same path on the same host, up to a page limit. See docs/sources/jsonld.md.
package jsonld

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"example.com/rolescout/api/internal/crawl"
	"example.com/rolescout/api/internal/ingest"
)

// Adapter fetches one career site per source. Config: {"url": "https://acme.example/careers", "company": "Acme"}.
type Adapter struct {
	Client   *crawl.Client
	MaxPages int
}

func New(c *crawl.Client, maxPages int) *Adapter {
	return &Adapter{Client: c, MaxPages: maxPages}
}

var httpURL = regexp.MustCompile(`^https?://[^/\s]+`)

func (a *Adapter) Validate(config map[string]any) error {
	return ingest.CheckConfig(config, ingest.Field{Name: "url", Pattern: httpURL})
}

func (a *Adapter) Fetch(ctx context.Context, config map[string]any) ([]ingest.RawListing, error) {
	if err := a.Validate(config); err != nil {
		return nil, err
	}
	start := ingest.ConfigString(config, "url")
	company := ingest.ConfigString(config, "company")
	maxPages := max(a.MaxPages, 1)

	queue := []string{start}
	visited := map[string]bool{start: true}
	seenIDs := map[string]bool{}
	var out []ingest.RawListing
	for pages := 0; len(queue) > 0 && pages < maxPages; pages++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		pageURL := queue[0]
		queue = queue[1:]
		res, err := a.Client.Get(ctx, pageURL, "text/html,application/xhtml+xml", nil)
		if err != nil {
			if pages == 0 {
				return nil, fmt.Errorf("read career page: %w", classify(err))
			}
			// Linked pages may be missing or disallowed; skip them.
			continue
		}
		if ct := res.Header.Get("Content-Type"); ct != "" && !strings.Contains(ct, "html") {
			continue
		}
		postings, links, err := ExtractJobPostings(res.Body, res.FinalURL)
		if err != nil {
			continue
		}
		for _, p := range postings {
			if seenIDs[p.ID] {
				continue
			}
			seenIDs[p.ID] = true
			name := company
			if name == "" {
				name = p.Company
			}
			if name == "" {
				if u, err := url.Parse(start); err == nil {
					name = strings.TrimPrefix(u.Hostname(), "www.")
				}
			}
			out = append(out, ingest.RawListing{
				ExternalID:     p.ID,
				URL:            p.URL,
				Title:          p.Title,
				Company:        name,
				Location:       p.Location,
				Description:    p.Description,
				EmploymentType: p.EmploymentType,
				PostedAt:       p.PostedAt,
				CountryCode:    p.CountryCode,
				Arrangement:    p.Arrangement,
			})
		}
		for _, l := range links {
			if !visited[l] {
				visited[l] = true
				queue = append(queue, l)
			}
		}
	}
	return out, nil
}

func classify(err error) error {
	var se *crawl.StatusError
	if errors.As(err, &se) && (se.StatusCode == 404 || se.StatusCode == 410) {
		return fmt.Errorf("%w: %w", ingest.ErrBoardNotFound, err)
	}
	if crawl.IsTemporary(err) {
		return ingest.Retryable(err)
	}
	return err
}
