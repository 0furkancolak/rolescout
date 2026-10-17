package discovery

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"strings"

	"golang.org/x/net/html"

	"example.com/rolescout/api/internal/crawl"
	"example.com/rolescout/api/internal/ingest/jsonld"
)

// jsonLDProbeLinks is how many linked job pages are checked for JSON-LD when
// the career page itself has none.
const jsonLDProbeLinks = 3

// Detector identifies the job board behind a career page.
type Detector struct {
	Client *crawl.Client
}

// Detection is what a career page turned out to be. Board.ATS is "unknown"
// when no supported board or JSON-LD was found.
type Detection struct {
	Board    Board
	FinalURL string
	// Name is the site's og:site_name or <title>, if any.
	Name string
	// Links are other boards the page links to, such as a subsidiary's.
	Links []Board
}

// Detect reads a career page and finds its job board: a known board URL, a
// redirect to one, an embedded board, or schema.org JobPosting data.
// crawl.ErrDisallowed is returned when robots.txt forbids the page.
func (d *Detector) Detect(ctx context.Context, careersURL string) (Detection, error) {
	if b, ok := ParseBoardURL(careersURL); ok {
		return Detection{Board: b, FinalURL: careersURL}, nil
	}
	res, err := d.Client.Get(ctx, careersURL, "text/html,application/xhtml+xml", nil)
	if err != nil {
		return Detection{}, fmt.Errorf("read career page: %w", err)
	}
	det := Detection{Board: Board{ATS: "unknown"}, FinalURL: res.FinalURL}
	if b, ok := ParseBoardURL(res.FinalURL); ok {
		det.Board = b
		return det, nil
	}

	page := scanPage(res.Body, res.FinalURL)
	det.Name = page.name
	seen := map[string]bool{}
	for _, b := range page.boards {
		key := b.ATS + "/" + b.ID
		if seen[key] {
			continue
		}
		seen[key] = true
		if det.Board.ATS == "unknown" {
			det.Board = b
		} else {
			det.Links = append(det.Links, b)
		}
	}
	if det.Board.ATS != "unknown" {
		return det, nil
	}

	postings, links, _ := jsonld.ExtractJobPostings(res.Body, res.FinalURL)
	found := len(postings) > 0
	for i := 0; !found && i < len(links) && i < jsonLDProbeLinks; i++ {
		r, err := d.Client.Get(ctx, links[i], "text/html,application/xhtml+xml", nil)
		if err != nil {
			continue
		}
		ps, _, _ := jsonld.ExtractJobPostings(r.Body, r.FinalURL)
		found = len(ps) > 0
	}
	if found {
		det.Board = Board{ATS: "jsonld", ID: res.FinalURL, Config: map[string]any{"url": res.FinalURL}}
	}
	return det, nil
}

type pageInfo struct {
	name   string
	boards []Board
}

// scanPage collects board URLs from script, iframe, link and anchor
// elements, and the site name.
func scanPage(body []byte, pageURL string) pageInfo {
	var info pageInfo
	base, err := url.Parse(pageURL)
	doc, perr := html.Parse(bytes.NewReader(body))
	if err != nil || perr != nil {
		return info
	}
	var title, siteName string
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			var ref string
			switch n.Data {
			case "script", "iframe":
				ref = attr(n, "src")
			case "a", "link":
				ref = attr(n, "href")
			case "title":
				if n.FirstChild != nil && title == "" {
					title = strings.TrimSpace(n.FirstChild.Data)
				}
			case "meta":
				if attr(n, "property") == "og:site_name" {
					siteName = strings.TrimSpace(attr(n, "content"))
				}
			}
			if ref != "" {
				if u, err := base.Parse(strings.TrimSpace(ref)); err == nil {
					if b, ok := ParseBoardURL(u.String()); ok {
						info.boards = append(info.boards, b)
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	info.name = siteName
	if info.name == "" {
		info.name = title
	}
	return info
}

func attr(n *html.Node, name string) string {
	for _, a := range n.Attr {
		if a.Key == name {
			return a.Val
		}
	}
	return ""
}
