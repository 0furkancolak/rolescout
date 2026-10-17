package jsonld

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// Posting is a schema.org JobPosting read from a page.
type Posting struct {
	ID, URL, Title, Company, Location, CountryCode, Arrangement, Description, EmploymentType string
	PostedAt                                                                                 *time.Time
}

// ExtractJobPostings reads the JobPosting objects in a page's JSON-LD
// scripts and the links that stay under the page's path on the same host.
// Scripts that are not valid JSON are skipped; expired postings are dropped.
func ExtractJobPostings(body []byte, pageURL string) ([]Posting, []string, error) {
	page, err := url.Parse(pageURL)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid page URL: %w", err)
	}
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, nil, fmt.Errorf("parse HTML: %w", err)
	}
	prefix := strings.TrimSuffix(page.Path, "/")
	var postings []Posting
	var links []string
	seen := map[string]bool{page.Scheme + "://" + page.Host + prefix: true, page.Scheme + "://" + page.Host + prefix + "/": true}
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "script":
				if strings.EqualFold(strings.TrimSpace(attr(n, "type")), "application/ld+json") && n.FirstChild != nil {
					postings = append(postings, parseScript(n.FirstChild.Data, page)...)
				}
			case "a":
				if link := sameSiteLink(page, prefix, attr(n, "href")); link != "" && !seen[link] {
					seen[link] = true
					links = append(links, link)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return postings, links, nil
}

func attr(n *html.Node, name string) string {
	for _, a := range n.Attr {
		if a.Key == name {
			return a.Val
		}
	}
	return ""
}

func sameSiteLink(page *url.URL, prefix, href string) string {
	href = strings.TrimSpace(href)
	if href == "" || strings.HasPrefix(href, "#") {
		return ""
	}
	u, err := page.Parse(href)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || !strings.EqualFold(u.Host, page.Host) {
		return ""
	}
	u.Fragment = ""
	if prefix != "" && u.Path != prefix && !strings.HasPrefix(u.Path, prefix+"/") {
		return ""
	}
	return u.String()
}

func parseScript(raw string, page *url.URL) []Posting {
	var v any
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &v); err != nil {
		return nil
	}
	var out []Posting
	var visit func(v any)
	visit = func(v any) {
		switch t := v.(type) {
		case []any:
			for _, item := range t {
				visit(item)
			}
		case map[string]any:
			if g, ok := t["@graph"]; ok {
				visit(g)
			}
			if isJobPosting(t["@type"]) {
				if p, ok := toPosting(t, page); ok {
					out = append(out, p)
				}
			}
		}
	}
	visit(v)
	return out
}

func isJobPosting(t any) bool {
	switch v := t.(type) {
	case string:
		return v == "JobPosting"
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok && s == "JobPosting" {
				return true
			}
		}
	}
	return false
}

func toPosting(m map[string]any, page *url.URL) (Posting, bool) {
	p := Posting{
		Title:       str(m["title"]),
		Description: str(m["description"]),
		Company:     name(m["hiringOrganization"]),
	}
	if p.Title == "" {
		return p, false
	}
	if until := parseTime(str(m["validThrough"])); until != nil && until.Before(time.Now()) {
		return p, false
	}
	p.URL = page.String()
	if u, err := page.Parse(str(m["url"])); err == nil && str(m["url"]) != "" && (u.Scheme == "http" || u.Scheme == "https") {
		p.URL = u.String()
	}
	p.ID = identifier(m["identifier"])
	if p.ID == "" && str(m["url"]) != "" {
		p.ID = p.URL
	}
	if p.ID == "" {
		sum := sha256.Sum256([]byte(p.Title))
		p.ID = page.String() + "#" + hex.EncodeToString(sum[:8])
	}
	switch v := m["employmentType"].(type) {
	case string:
		p.EmploymentType = v
	case []any:
		if len(v) > 0 {
			p.EmploymentType = str(v[0])
		}
	}
	if strings.EqualFold(str(m["jobLocationType"]), "TELECOMMUTE") {
		p.Arrangement = "remote"
	}
	p.PostedAt = parseTime(str(m["datePosted"]))
	p.Location, p.CountryCode = location(m["jobLocation"])
	return p, true
}

// str returns a string value, or the string form of a number.
func str(v any) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case float64:
		return fmt.Sprint(t)
	}
	return ""
}

func name(v any) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case map[string]any:
		return str(t["name"])
	}
	return ""
}

func identifier(v any) string {
	switch t := v.(type) {
	case map[string]any:
		return str(t["value"])
	default:
		return str(t)
	}
}

func location(v any) (string, string) {
	places, ok := v.([]any)
	if !ok {
		places = []any{v}
	}
	var names []string
	country := ""
	for _, pl := range places {
		m, ok := pl.(map[string]any)
		if !ok {
			continue
		}
		addr, _ := m["address"].(map[string]any)
		if addr == nil {
			if n := str(m["name"]); n != "" {
				names = append(names, n)
			}
			continue
		}
		c := name(addr["addressCountry"])
		parts := []string{}
		for _, k := range []string{"addressLocality", "addressRegion"} {
			if s := str(addr[k]); s != "" {
				parts = append(parts, s)
			}
		}
		if c != "" {
			parts = append(parts, c)
			if country == "" {
				country = c
			}
		}
		if len(parts) > 0 {
			names = append(names, strings.Join(parts, ", "))
		}
	}
	return strings.Join(names, "; "), country
}

func parseTime(s string) *time.Time {
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02T15:04", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			t = t.UTC()
			return &t
		}
	}
	return nil
}
