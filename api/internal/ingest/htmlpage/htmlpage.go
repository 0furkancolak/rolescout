package htmlpage

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

// Adapter reads the job cards of one career page per source.
// Config: {"url": "https://acme.example/kariyer", "company": "Acme"}.
type Adapter struct {
	Client *crawl.Client
}

func New(c *crawl.Client) *Adapter { return &Adapter{Client: c} }

var httpURL = regexp.MustCompile(`^https?://[^/\s]+`)

func (a *Adapter) Validate(config map[string]any) error {
	return ingest.CheckConfig(config, ingest.Field{Name: "url", Pattern: httpURL})
}

func (a *Adapter) FetchTimeout(map[string]any) time.Duration { return 2 * time.Minute }

func (a *Adapter) Fetch(ctx context.Context, config map[string]any) ([]ingest.RawListing, error) {
	if err := a.Validate(config); err != nil {
		return nil, err
	}
	start := ingest.ConfigString(config, "url")
	res, err := a.Client.Get(ctx, start, "text/html,application/xhtml+xml", nil)
	if err != nil {
		var se *crawl.StatusError
		if errors.As(err, &se) && (se.StatusCode == 404 || se.StatusCode == 410) {
			return nil, fmt.Errorf("%w: %w", ingest.ErrBoardNotFound, err)
		}
		if crawl.IsTemporary(err) {
			return nil, ingest.Retryable(err)
		}
		return nil, fmt.Errorf("read career page: %w", err)
	}
	if ct := res.Header.Get("Content-Type"); ct != "" && !strings.Contains(ct, "html") {
		return nil, fmt.Errorf("career page is not an HTML page (%s)", ct)
	}
	company := ingest.ConfigString(config, "company")
	if company == "" {
		if u, err := url.Parse(start); err == nil {
			company = strings.TrimPrefix(u.Hostname(), "www.")
		}
	}
	cards := Extract(res.Body, res.FinalURL)
	if len(cards) == 0 {
		// A page that lost its cards may have changed its layout; closing every
		// job because of that would be wrong.
		return nil, ingest.Incomplete("no job cards found on the page")
	}
	out := make([]ingest.RawListing, 0, len(cards))
	for _, c := range cards {
		out = append(out, ingest.RawListing{
			ExternalID: c.ID, URL: c.URL, Title: c.Title, Company: company, Location: c.Location,
			Description: c.Description, EmploymentType: c.EmploymentType, Arrangement: c.Arrangement,
		})
	}
	return out, nil
}
