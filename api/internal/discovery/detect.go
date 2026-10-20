package discovery

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"golang.org/x/net/html"

	"example.com/rolescout/api/internal/crawl"
	"example.com/rolescout/api/internal/ingest/htmlpage"
	"example.com/rolescout/api/internal/ingest/jsonld"
)

// jsonLDProbeLinks is how many linked job pages are checked for JSON-LD when
// the career page itself has none.
const jsonLDProbeLinks = 3

// Detector identifies the job board behind a career page.
type Detector struct {
	Client *crawl.Client
	// Classifier, when set, is asked to pick a link on a page the rules could
	// not place. It is nil in the research pipeline (a run must never wait for
	// a model); it exists for a background re-detection pass.
	Classifier Classifier
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
	return d.detect(ctx, careersURL, true)
}

func (d *Detector) detect(ctx context.Context, careersURL string, followCareersLink bool) (Detection, error) {
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
		return det, nil
	}
	// The page itself may be the list: job cards printed in plain HTML.
	if htmlpage.LooksLikeJobList(res.Body, res.FinalURL) {
		det.Board = Board{ATS: "htmlpage", ID: res.FinalURL, Config: map[string]any{"url": res.FinalURL}}
		return det, nil
	}
	// A company homepage usually links to its career page; failing that, the
	// addresses career pages usually have are tried.
	if followCareersLink {
		for _, target := range d.careerTargets(ctx, res.FinalURL, page.careerLinks) {
			next, err := d.detect(ctx, target, false)
			if err != nil || next.Board.ATS == "unknown" {
				continue
			}
			if next.Name == "" {
				next.Name = det.Name
			}
			return next, nil
		}
	}
	// The rules found nothing. A classifier may still recognise the link to the
	// job listings; its pick is fetched like any other page and has to detect as
	// a known board or JSON-LD, so a wrong pick costs one request and nothing else.
	if followCareersLink && d.Classifier != nil && len(page.anchors) > 0 {
		if i, ok, err := d.Classifier.PickLink(ctx, det.Name, page.anchors); err == nil && ok && i >= 0 && i < len(page.anchors) {
			if next, err := d.detect(ctx, page.anchors[i].URL, false); err == nil && next.Board.ATS != "unknown" {
				if next.Name == "" {
					next.Name = det.Name
				}
				return next, nil
			}
		}
	}
	return det, nil
}

// careerPaths are the addresses company sites commonly put their job list at.
var careerPaths = []string{
	"/kariyer", "/careers", "/career", "/jobs", "/is-ilanlari", "/insan-kaynaklari", "/ik", "/bize-katilin",
	"/kariyer-firsatlari", "/en/career", "/en/careers", "/tr/kariyer", "/is-firsatlari", "/acik-pozisyonlar",
}

// maxCareerTargets bounds how many addresses one company page may cost.
const maxCareerTargets = 12

var (
	sitemapLoc = regexp.MustCompile(`(?i)<loc>\s*([^<\s]+)\s*</loc>`)
	localeOnly = regexp.MustCompile(`^[a-z]{2}(-[A-Za-z]{2})?$`)
)

// careerTargets lists the pages to try for a site whose given page held no
// board: the career links the page showed, the career pages named in its
// sitemap, then the usual paths. It only guesses on a site's front page.
func (d *Detector) careerTargets(ctx context.Context, pageURL string, links []string) []string {
	base, err := url.Parse(pageURL)
	if err != nil {
		return nil
	}
	seen := map[string]bool{pageURL: true}
	var out []string
	add := func(raw string) {
		if raw == "" || seen[raw] || len(out) >= maxCareerTargets {
			return
		}
		seen[raw] = true
		out = append(out, raw)
	}
	for i, l := range links {
		if i < 3 {
			add(l)
		}
	}
	if p := strings.Trim(base.Path, "/"); p != "" && !localeOnly.MatchString(p) {
		return out // a deep page: only what it links to
	}
	origin := base.Scheme + "://" + base.Host
	if res, err := d.Client.Get(ctx, origin+"/sitemap.xml", "application/xml,text/xml", nil); err == nil {
		n := 0
		for _, m := range sitemapLoc.FindAllStringSubmatch(string(res.Body), 2000) {
			u, err := url.Parse(m[1])
			if err != nil || !strings.EqualFold(u.Host, base.Host) || !careersText.MatchString(u.Path) || strings.Count(strings.Trim(u.Path, "/"), "/") > 1 {
				continue
			}
			add(u.String())
			if n++; n == 3 {
				break
			}
		}
	}
	for _, p := range careerPaths {
		add(origin + p)
	}
	return out
}

var careersText = regexp.MustCompile(`(?i)\b(careers?|jobs|kariyer|karriere|join[- ]us|work[- ]with[- ]us|open[- ](positions|roles)|we.?re[- ]hiring|stellenangebote|emplois?)\b`)

type pageInfo struct {
	name   string
	boards []Board
	// careerLinks are same-site links that look like a careers page.
	careerLinks []string
	// anchors are the other same-site links, in page order, for a classifier.
	anchors []LinkCandidate
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
					} else if n.Data == "a" && strings.EqualFold(u.Host, base.Host) && u.Path != base.Path &&
						(careersText.MatchString(u.Path) || careersText.MatchString(text(n))) {
						u.Fragment = ""
						info.careerLinks = append(info.careerLinks, u.String())
					} else if n.Data == "a" && strings.EqualFold(u.Host, base.Host) && u.Path != base.Path &&
						(u.Scheme == "http" || u.Scheme == "https") && len(info.anchors) < maxClassifierCandidates {
						u.Fragment = ""
						if label := strings.Join(strings.Fields(text(n)), " "); label != "" {
							info.anchors = append(info.anchors, LinkCandidate{URL: u.String(), Text: label})
						}
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

// text returns the text inside n.
func text(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}

func attr(n *html.Node, name string) string {
	for _, a := range n.Attr {
		if a.Key == name {
			return a.Val
		}
	}
	return ""
}
