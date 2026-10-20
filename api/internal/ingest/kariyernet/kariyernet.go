// Package kariyernet reads the public job lists of kariyer.net, a Turkish job
// site, one category list per source. The lists are server-rendered cards with
// the title, company, city, work model and employment type; the detail pages
// are rendered in the browser, so only the cards are read. See
// docs/sources/kariyernet.md.
package kariyernet

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"

	"example.com/rolescout/api/internal/crawl"
	"example.com/rolescout/api/internal/ingest"
)

// DefaultMaxPages bounds the pages read per category list (about 50 jobs each).
const DefaultMaxPages = 12

// Adapter reads one category list. Config:
// {"url": "https://www.kariyer.net/is-ilanlari/yazilim", "max_pages": "12"}.
type Adapter struct {
	Client *crawl.Client
	// Pause is the least time between two requests to the site from this
	// process, whichever category list they belong to: the site answers 403 to
	// clients that read too fast, and the lists are synced at the same time.
	Pause time.Duration
}

func New(c *crawl.Client) *Adapter { return &Adapter{Client: c, Pause: 3 * time.Second} }

var (
	gateMu   sync.Mutex
	gateNext time.Time
)

// wait blocks until this caller may make its request, one Pause after the
// previous caller's. It returns false when ctx ends first.
func (a *Adapter) wait(ctx context.Context) bool {
	if a.Pause <= 0 {
		return ctx.Err() == nil
	}
	gateMu.Lock()
	at := gateNext
	if now := time.Now(); at.Before(now) {
		at = now
	}
	gateNext = at.Add(a.Pause)
	gateMu.Unlock()
	select {
	case <-ctx.Done():
		return false
	case <-time.After(time.Until(at)):
		return true
	}
}

var (
	listURL  = regexp.MustCompile(`^https://www\.kariyer\.net/is-ilanlari/[a-z0-9-]+$`)
	digits   = regexp.MustCompile(`^[0-9]{1,3}$`)
	idInPath = regexp.MustCompile(`^/is-ilani/[a-z0-9-]+-([0-9]+)$`)
)

func (a *Adapter) Validate(config map[string]any) error {
	return ingest.CheckConfig(config,
		ingest.Field{Name: "url", Pattern: listURL},
		ingest.Field{Name: "max_pages", Pattern: digits, Optional: true})
}

// FetchTimeout leaves room to wait behind the other lists' requests.
func (a *Adapter) FetchTimeout(map[string]any) time.Duration { return time.Hour }

func maxPages(config map[string]any) int {
	if n, err := strconv.Atoi(ingest.ConfigString(config, "max_pages")); err == nil && n > 0 {
		return n
	}
	return DefaultMaxPages
}

func (a *Adapter) Fetch(ctx context.Context, config map[string]any) ([]ingest.RawListing, error) {
	if err := a.Validate(config); err != nil {
		return nil, err
	}
	start, _ := url.Parse(ingest.ConfigString(config, "url"))
	limit := maxPages(config)

	var out []ingest.RawListing
	seen := map[string]bool{}
	next := start.String()
	pages := 0
	for next != "" && pages < limit {
		if !a.wait(ctx) {
			break
		}
		res, err := a.Client.Get(ctx, next, "text/html,application/xhtml+xml", nil)
		if err != nil {
			if pages == 0 {
				return nil, classify(err)
			}
			return out, ingest.Incomplete("page %d of the list could not be read: %v", pages+1, err)
		}
		pages++
		cards, following := parsePage(res.Body, res.FinalURL, start.Path)
		added := 0
		for _, c := range cards {
			if !seen[c.ExternalID] {
				seen[c.ExternalID] = true
				out = append(out, c)
				added++
			}
		}
		if added == 0 {
			break
		}
		next = following
	}
	switch {
	case ctx.Err() != nil && len(out) == 0:
		return nil, ingest.Retryable(fmt.Errorf("kariyer.net list: %w", ctx.Err()))
	case ctx.Err() != nil:
		return out, ingest.Incomplete("time budget ran out after %d pages", pages)
	case len(out) == 0:
		// The list is never really empty; a layout change must not close every job.
		return nil, ingest.Incomplete("no job cards found on the list")
	case next != "" && pages >= limit:
		return out, ingest.Incomplete("page limit of %d reached", limit)
	}
	return out, nil
}

func classify(err error) error {
	var se *crawl.StatusError
	if errors.As(err, &se) {
		switch se.StatusCode {
		case 404, 410:
			return fmt.Errorf("%w: %w", ingest.ErrBoardNotFound, err)
		case 403, 429:
			return ingest.Retryable(fmt.Errorf("kariyer.net is limiting requests: %w", err))
		}
	}
	if crawl.IsTemporary(err) {
		return ingest.Retryable(err)
	}
	return err
}

var arrangements = map[string]string{"iş yerinde": "onsite", "hibrit": "hybrid", "uzaktan": "remote", "remote": "remote"}

// parsePage reads the cards of a list page and the address of the next page of
// the same list (empty when there is none).
func parsePage(body []byte, pageURL, listPath string) ([]ingest.RawListing, string) {
	page, err := url.Parse(pageURL)
	if err != nil {
		return nil, ""
	}
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, ""
	}
	var cards []ingest.RawListing
	nextURL := ""
	nextPath := regexp.MustCompile("^" + regexp.QuoteMeta(listPath) + `-[0-9]+$`)
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch {
			case n.Data == "link" && attr(n, "rel") == "next":
				if u, err := page.Parse(attr(n, "href")); err == nil && strings.EqualFold(u.Host, page.Host) && nextPath.MatchString(u.Path) {
					nextURL = u.Scheme + "://" + u.Host + u.Path
				}
			case n.Data == "div" && attr(n, "data-test") == "ad-card":
				if c, ok := parseCard(n, page); ok {
					cards = append(cards, c)
				}
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return cards, nextURL
}

func parseCard(n *html.Node, page *url.URL) (ingest.RawListing, bool) {
	var href, title, company, location, model string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			if n.Data == "a" && href == "" && strings.HasPrefix(attr(n, "href"), "/is-ilani/") {
				href = attr(n, "href")
			}
			switch attr(n, "data-test") {
			case "ad-card-title":
				title = clean(n)
			case "subtitle":
				company = clean(n)
			case "location":
				location = clean(n)
			case "work-model":
				model = clean(n)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	if title == "" {
		title = strings.TrimSpace(attr(n, "positionname"))
	}
	if location == "" {
		location = strings.TrimSpace(attr(n, "cityname"))
	}
	u, err := page.Parse(href)
	if href == "" || err != nil || title == "" || company == "" {
		return ingest.RawListing{}, false
	}
	m := idInPath.FindStringSubmatch(u.Path)
	if m == nil {
		return ingest.RawListing{}, false
	}
	if model == "" {
		model = attr(n, "workmodeltext") // the HTML parser lower-cases attribute names
	}
	raw := ingest.RawListing{
		ExternalID:     m[1],
		URL:            u.Scheme + "://" + u.Host + u.Path,
		Title:          title,
		Company:        company,
		Location:       location,
		EmploymentType: attr(n, "worktypetext"),
		Arrangement:    arrangements[strings.ToLower(model)],
	}
	if attr(n, "countryname") == "Türkiye" {
		raw.CountryCode = "TR"
	}
	return raw, true
}

func clean(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
			b.WriteByte(' ')
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.Join(strings.Fields(b.String()), " ")
}

func attr(n *html.Node, name string) string {
	for _, a := range n.Attr {
		if a.Key == name {
			return a.Val
		}
	}
	return ""
}
