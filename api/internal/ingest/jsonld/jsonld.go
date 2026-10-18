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
	"time"

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

// FetchTimeout gives a crawl time for its page limit at a polite pace.
func (a *Adapter) FetchTimeout(map[string]any) time.Duration {
	return min(max(time.Duration(max(a.MaxPages, 1))*5*time.Second, 2*time.Minute), time.Hour)
}

// skips counts pages and blocks that were left out, by reason.
type skips struct {
	unreadable, missing, notHTML, malformed int
}

func (s skips) note() string {
	var parts []string
	add := func(n int, one, many string) {
		switch {
		case n == 1:
			parts = append(parts, "1 "+one)
		case n > 1:
			parts = append(parts, fmt.Sprintf("%d %s", n, many))
		}
	}
	add(s.missing, "page not found", "pages not found")
	add(s.notHTML, "page not HTML", "pages not HTML")
	add(s.malformed, "malformed JSON-LD block", "malformed JSON-LD blocks")
	return strings.Join(parts, ", ")
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
	var skipped skips
	pages := 0
	for ; len(queue) > 0 && pages < maxPages; pages++ {
		if ctx.Err() != nil {
			break
		}
		pageURL := queue[0]
		queue = queue[1:]
		res, err := a.Client.Get(ctx, pageURL, "text/html,application/xhtml+xml", nil)
		if err != nil {
			if pages == 0 {
				return nil, fmt.Errorf("read career page: %w", classify(err))
			}
			var se *crawl.StatusError
			switch {
			case ctx.Err() != nil:
			case errors.As(err, &se) && (se.StatusCode == 404 || se.StatusCode == 410), errors.Is(err, crawl.ErrDisallowed):
				skipped.missing++
			default:
				skipped.unreadable++
			}
			continue
		}
		if ct := res.Header.Get("Content-Type"); ct != "" && !strings.Contains(ct, "html") {
			if pages == 0 {
				return nil, fmt.Errorf("career page is not an HTML page (%s)", ct)
			}
			skipped.notHTML++
			continue
		}
		postings, links, malformed, err := extract(res.Body, res.FinalURL)
		skipped.malformed += malformed
		if err != nil {
			skipped.notHTML++
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

	note := skipped.note()
	withNote := func(reason string) string {
		if note == "" {
			return reason
		}
		return reason + "; " + note
	}
	switch {
	case ctx.Err() != nil && len(out) == 0:
		return nil, ingest.Retryable(fmt.Errorf("career site crawl: %w", ctx.Err()))
	case ctx.Err() != nil:
		return out, ingest.Incomplete("%s", withNote(fmt.Sprintf("time budget ran out after %d pages", pages)))
	case skipped.unreadable > 0:
		n := skipped.unreadable
		return out, ingest.Incomplete("%s", withNote(fmt.Sprintf("%d %s could not be read", n, map[bool]string{true: "page", false: "pages"}[n == 1])))
	case len(queue) > 0:
		return out, ingest.Incomplete("%s", withNote(fmt.Sprintf("page limit of %d reached", maxPages)))
	case note != "":
		return out, ingest.Note("%s", note)
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
