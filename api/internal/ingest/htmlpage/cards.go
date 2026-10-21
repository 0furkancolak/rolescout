// Package htmlpage reads job lists that a company's own career page prints in
// its HTML without any structured data or job board system behind it: a page
// of cards, each with a title, a few words about the job and an apply button.
// The reading is rule based and needs no AI; see docs/sources/htmlpage.md.
package htmlpage

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/net/html"
)

// Card is one job found on a page.
type Card struct {
	ID             string
	Title          string
	URL            string
	Description    string
	EmploymentType string
	Arrangement    string // remote, hybrid or onsite
	Location       string
}

var (
	applyText = regexp.MustCompile(`(?i)^(hemen\s+)?başvur(u\s+yap|un)?\.?$|^(apply|apply\s+now|apply\s+here|apply\s+for\s+this\s+(job|role|position)|başvuru\s+yap|bewerben|postuler)\.?$`)
	typeWords = map[string]string{
		"tam zamanlı": "full-time", "tam zamanli": "full-time", "full time": "full-time", "full-time": "full-time",
		"yarı zamanlı": "part-time", "yari zamanli": "part-time", "part time": "part-time", "part-time": "part-time",
		"stajyer": "internship", "staj": "internship", "internship": "internship", "intern": "internship",
		"sözleşmeli": "contract", "contract": "contract", "freelance": "contract", "geçici": "temporary", "temporary": "temporary",
	}
	arrangementWords = map[string]string{
		"ofis": "onsite", "ofiste": "onsite", "office": "onsite", "on-site": "onsite", "onsite": "onsite", "iş yerinde": "onsite",
		"remote": "remote", "uzaktan": "remote", "evden": "remote", "hibrit": "hybrid", "hybrid": "hybrid", "karma": "hybrid",
	}
	jobPath  = regexp.MustCompile(`(?i)(career|kariyer|karriere|job|ilan|pozisyon|position|opening|vacanc|insan-kaynak|ise-alim|işe-alım|bize-katil|join)`)
	jobHref  = regexp.MustCompile(`(?i)/(ilan|ilanlar|is-ilani|job|jobs|pozisyon|position|positions|opening|openings|vacancy|vacancies|kariyer|career|careers)/[^/?#]+`)
	skipTags = map[string]bool{"script": true, "style": true, "noscript": true, "svg": true, "template": true, "head": true}
)

// Extract finds the job cards on a page. A card is the largest block with
// exactly one apply button and a heading; its heading is the job title.
// Without apply buttons, a list of links to job pages under a career path is
// read instead. Cards with the same title are listed once.
func Extract(body []byte, pageURL string) []Card {
	page, err := url.Parse(pageURL)
	if err != nil {
		return nil
	}
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil
	}
	cards := applyCards(doc, page)
	if len(cards) == 0 && jobPath.MatchString(page.Path) {
		cards = linkCards(doc, page)
	}
	seen := map[string]bool{}
	out := cards[:0]
	for _, c := range cards {
		if seen[c.ID] {
			continue
		}
		seen[c.ID] = true
		out = append(out, c)
	}
	return out
}

// LooksLikeJobList reports whether a page is a list of jobs rather than any
// other page with a button that says "apply": the page's address talks about
// careers, or a card names an employment type or a place of work.
func LooksLikeJobList(body []byte, pageURL string) bool {
	page, err := url.Parse(pageURL)
	if err != nil {
		return false
	}
	cards := Extract(body, pageURL)
	if len(cards) == 0 {
		return false
	}
	if jobPath.MatchString(page.Path) {
		return true
	}
	for _, c := range cards {
		if c.EmploymentType != "" || c.Arrangement != "" {
			return true
		}
	}
	return false
}

func applyCards(doc *html.Node, page *url.URL) []Card {
	var markers []*html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			if skipTags[n.Data] {
				return
			}
			if (n.Data == "a" || n.Data == "button") && applyText.MatchString(collapse(text(n))) {
				markers = append(markers, n)
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	if len(markers) == 0 {
		return nil
	}
	isMarker := map[*html.Node]bool{}
	for _, m := range markers {
		isMarker[m] = true
	}
	count := func(n *html.Node) int {
		k := 0
		var w func(*html.Node)
		w = func(n *html.Node) {
			if isMarker[n] {
				k++
				return
			}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				w(c)
			}
		}
		w(n)
		return k
	}
	var out []Card
	for _, m := range markers {
		// A button inside an apply link is one marker: climb out of the link first.
		start := m
		if p := m.Parent; p != nil && p.Type == html.ElementNode && p.Data == "a" && count(p) == 1 {
			start = p
		}
		card := start
		for p := start.Parent; p != nil && p.Type == html.ElementNode && p.Data != "body" && p.Data != "html" && count(p) == 1; p = p.Parent {
			card = p
		}
		if c, ok := buildCard(card, m, page); ok {
			out = append(out, c)
		}
	}
	return out
}

func buildCard(card, marker *html.Node, page *url.URL) (Card, bool) {
	heading := firstHeading(card)
	if heading == nil {
		return Card{}, false
	}
	title := collapse(text(heading))
	if n := len([]rune(title)); n < 2 || n > 120 {
		return Card{}, false
	}
	c := Card{Title: title}
	var short []string
	var paragraph string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && (skipTags[n.Data] || n == heading || n == marker) {
			return
		}
		if n.Type == html.ElementNode && (n.Data == "p" || n.Data == "span" || n.Data == "li" || n.Data == "div") && onlyText(n) {
			t := collapse(text(n))
			switch {
			case t == "":
			case len([]rune(t)) <= 30:
				short = append(short, t)
			case paragraph == "":
				paragraph = t
			}
			return
		}
		for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
			walk(ch)
		}
	}
	walk(card)
	c.Description = paragraph
	for _, s := range short {
		low := strings.ToLower(s)
		if v, ok := typeWords[low]; ok && c.EmploymentType == "" {
			c.EmploymentType = v
		} else if v, ok := arrangementWords[low]; ok && c.Arrangement == "" {
			c.Arrangement = v
		} else if c.Location == "" && !strings.ContainsAny(s, ".!?") && hasLetter(s) {
			c.Location = s
		}
	}
	c.URL = jobLink(card, heading, marker, page)
	c.ID = hash(title)
	return c, true
}

// linkCards reads a plain list of links to job pages.
func linkCards(doc *html.Node, page *url.URL) []Card {
	var out []Card
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			if skipTags[n.Data] {
				return
			}
			if n.Data == "a" {
				if u, ok := resolve(page, attr(n, "href")); ok && sameHost(page, u) && u.Path != page.Path && jobHref.MatchString(u.Path) {
					title := collapse(firstLine(n))
					if l := len([]rune(title)); l >= 4 && l <= 100 && !applyText.MatchString(title) {
						out = append(out, Card{ID: hash(u.Path), Title: title, URL: u.String()})
					}
				}
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	if len(out) < 2 {
		return nil
	}
	return out
}

func jobLink(card, heading, marker *html.Node, page *url.URL) string {
	// The heading's own link, else the first link of the card to a page of the
	// site, else the apply button's link, else the list page itself.
	try := func(n *html.Node) string {
		for a := n; a != nil; a = a.Parent {
			if a.Type == html.ElementNode && a.Data == "a" {
				if u, ok := resolve(page, attr(a, "href")); ok {
					return u.String()
				}
			}
			if a == card {
				break
			}
		}
		return ""
	}
	if u := try(heading); u != "" {
		return u
	}
	var found string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if found != "" || (n.Type == html.ElementNode && skipTags[n.Data]) {
			return
		}
		if n.Type == html.ElementNode && n.Data == "a" && !within(marker, n) && !within(n, marker) {
			if u, ok := resolve(page, attr(n, "href")); ok && sameHost(page, u) {
				found = u.String()
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(card)
	if found != "" {
		return found
	}
	if u := try(marker); u != "" {
		return u
	}
	u := *page
	u.Fragment = ""
	return u.String() + "#" + hash(text(heading))
}

// within reports whether n is inside ancestor (or is it).
func within(n, ancestor *html.Node) bool {
	for p := n; p != nil; p = p.Parent {
		if p == ancestor {
			return true
		}
	}
	return false
}

// resolve returns an absolute http(s) link; mailto, javascript and protected
// e-mail links are not job pages.
func resolve(page *url.URL, href string) (*url.URL, bool) {
	href = strings.TrimSpace(href)
	if href == "" || strings.HasPrefix(href, "#") || strings.Contains(href, "/cdn-cgi/") {
		return nil, false
	}
	u, err := page.Parse(href)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, false
	}
	u.Fragment = ""
	return u, true
}

func sameHost(a, b *url.URL) bool {
	return strings.EqualFold(strings.TrimPrefix(a.Hostname(), "www."), strings.TrimPrefix(b.Hostname(), "www."))
}

func firstHeading(n *html.Node) *html.Node {
	var found *html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if found != nil || (n.Type == html.ElementNode && skipTags[n.Data]) {
			return
		}
		if n.Type == html.ElementNode {
			switch n.Data {
			case "h1", "h2", "h3", "h4", "h5", "h6":
				if collapse(text(n)) != "" {
					found = n
					return
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return found
}

// onlyText is true for an element with no block children, so it is one line of text.
func onlyText(n *html.Node) bool {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode {
			continue
		}
		switch c.Data {
		case "div", "p", "ul", "ol", "li", "section", "article", "h1", "h2", "h3", "h4", "h5", "h6", "a", "button":
			return false
		}
		if !onlyText(c) {
			return false
		}
	}
	return true
}

func firstLine(n *html.Node) string {
	if h := firstHeading(n); h != nil {
		return text(h)
	}
	return text(n)
}

func text(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && skipTags[n.Data] {
			return
		}
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
			b.WriteByte(' ')
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}

func collapse(s string) string { return strings.Join(strings.Fields(s), " ") }

func attr(n *html.Node, name string) string {
	for _, a := range n.Attr {
		if a.Key == name {
			return a.Val
		}
	}
	return ""
}

// hash is a short stable identifier for a job without a page of its own.
func hash(s string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(collapse(s))))
	return hex.EncodeToString(sum[:8])
}

// hasLetter is false for icon-only text such as an emoji printed beside a
// card, which is decoration and not a place.
func hasLetter(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) {
			return true
		}
	}
	return false
}
