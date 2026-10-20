package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"strings"

	"example.com/rolescout/api/internal/crawl"
	"example.com/rolescout/api/internal/focus"
	"example.com/rolescout/api/internal/ingest"
)

// DBpediaEndpoint is DBpedia's public SPARQL service.
const DBpediaEndpoint = "https://dbpedia.org/sparql"

// DefaultCompanyCountries are read when a run has no country focus.
var DefaultCompanyCountries = []string{"TR", "US", "GB", "DE", "NL", "FR"}

// DBpedia lists the companies of a country that have an official website:
// the structured, community-kept company data of DBpedia. It is how a country's
// employers enter the catalog without a list somebody wrote for it. Their
// websites become candidates; the detector then finds each one's career page.
// The countries of the run's focus come first, then the defaults.
type DBpedia struct {
	Client   *crawl.Client
	Endpoint string // default DBpediaEndpoint
	// PageSize is rows per request (default 2000); MaxRows caps a country
	// (default 20000); PagesPerRun caps requests per Discover call (default 5).
	PageSize, MaxRows, PagesPerRun int
	// Countries replaces the default list; the run's focus still comes first.
	Countries []string
}

func (w *DBpedia) Name() string { return "dbpedia" }

// dbpediaState maps a country to the rows read so far, or finished.
type dbpediaState map[string]int

const dbpediaFinished = -1

func parseDBpediaState(s string) dbpediaState {
	st := dbpediaState{}
	if s != "" {
		_ = json.Unmarshal([]byte(s), &st)
	}
	return st
}

func (w *DBpedia) countries(ctx context.Context) []string {
	base := w.Countries
	if len(base) == 0 {
		base = DefaultCompanyCountries
	}
	out := slices.Clone(focus.Countries(ctx))
	for _, c := range base {
		if !slices.Contains(out, c) {
			out = append(out, c)
		}
	}
	return out
}

// PendingCtx is true while a country of this run (the focus and the defaults)
// has not been read to the end.
func (w *DBpedia) PendingCtx(ctx context.Context, state string) bool {
	st := parseDBpediaState(state)
	for _, c := range w.countries(ctx) {
		if st[c] != dbpediaFinished {
			return true
		}
	}
	return false
}

func (w *DBpedia) Discover(ctx context.Context, state string, fn func(Candidate) error) (string, error) {
	st := parseDBpediaState(state)
	countries := w.countries(ctx)
	if !slices.ContainsFunc(countries, func(c string) bool { return st[c] != dbpediaFinished }) {
		// Everything was read: the normal discovery schedule reads it again.
		for _, c := range countries {
			delete(st, c)
		}
	}
	pageSize, maxRows, pages := w.PageSize, w.MaxRows, w.PagesPerRun
	if pageSize <= 0 {
		pageSize = 2000
	}
	if maxRows <= 0 {
		maxRows = 20000
	}
	if pages <= 0 {
		pages = 5
	}
	save := func() string { b, _ := json.Marshal(st); return string(b) }
	for _, country := range countries {
		for st[country] != dbpediaFinished {
			if pages == 0 {
				return save(), nil
			}
			pages--
			rows, err := w.page(ctx, country, st[country], pageSize)
			if err != nil {
				return save(), fmt.Errorf("dbpedia %s: %w", country, err)
			}
			for _, r := range rows {
				site := companySite(r.site)
				if site == "" {
					continue
				}
				if err := fn(Candidate{URL: site, Name: r.name, Origin: "dbpedia", Country: country}); err != nil {
					return save(), err
				}
			}
			st[country] += len(rows)
			if len(rows) < pageSize || st[country] >= maxRows {
				st[country] = dbpediaFinished
			}
		}
	}
	return save(), nil
}

type dbpediaRow struct{ site, name string }

// dbpediaCountryLabel names the countries whose companies do not use their own
// top-level domain.
var dbpediaCountryLabel = map[string]string{"US": "United States"}

// countryTLD is the top-level domain of a country, such as "tr" or "uk".
func countryTLD(country string) string {
	for tld, code := range countryDomains {
		if code == country {
			return tld
		}
	}
	return ""
}

func (w *DBpedia) page(ctx context.Context, country string, offset, size int) ([]dbpediaRow, error) {
	if len(country) != 2 || strings.ToUpper(country) != country {
		return nil, fmt.Errorf("invalid country %q", country)
	}
	// A company belongs to a country by the domain of its website, or, where
	// that says nothing (the US), by the country DBpedia places it in.
	var where string
	if label, ok := dbpediaCountryLabel[country]; ok {
		where = fmt.Sprintf(`?c dbo:locationCountry ?co . ?co rdfs:label %q@en .`, label)
	} else if tld := countryTLD(country); tld != "" {
		where = fmt.Sprintf(`FILTER(REGEX(STR(?site), "\\.%s(/|$)"))`, tld)
	} else {
		return nil, fmt.Errorf("no way to tell the companies of %s", country)
	}
	query := fmt.Sprintf(`# country %s
SELECT DISTINCT ?c ?site ?name WHERE {
  ?c a dbo:Company .
  ?c foaf:homepage ?site .
  %s
  OPTIONAL { ?c rdfs:label ?name . FILTER(LANG(?name) = "en") }
} ORDER BY ?c LIMIT %d OFFSET %d`, country, where, size, offset)
	endpoint := w.Endpoint
	if endpoint == "" {
		endpoint = DBpediaEndpoint
	}
	var res struct {
		Results struct {
			Bindings []map[string]struct {
				Value string `json:"value"`
			} `json:"bindings"`
		} `json:"results"`
	}
	body, err := ingest.Fetch(ctx, w.Client, endpoint+"?query="+url.QueryEscape(query), "application/sparql-results+json")
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, fmt.Errorf("unreadable answer: %w", err)
	}
	rows := make([]dbpediaRow, 0, len(res.Results.Bindings))
	for _, b := range res.Results.Bindings {
		rows = append(rows, dbpediaRow{site: b["site"].Value, name: strings.TrimSpace(b["name"].Value)})
	}
	return rows, nil
}

// notCompanySites are hosts a company record may name as its "website" that
// are a profile somewhere else, not the company's own site.
var notCompanySites = []string{
	"facebook.com", "linkedin.com", "twitter.com", "x.com", "instagram.com", "youtube.com", "wikipedia.org",
	"dbpedia.org", "github.com", "t.me", "bloomberg.com", "crunchbase.com", "google.com", "play.google.com",
}

// companySite returns the company's own site as a clean http(s) address (path
// kept, query and fragment dropped), or "" when the value is not one.
func companySite(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || !strings.Contains(u.Hostname(), ".") {
		return ""
	}
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	for _, bad := range notCompanySites {
		if host == bad || strings.HasSuffix(host, "."+bad) {
			return ""
		}
	}
	u.RawQuery, u.Fragment = "", ""
	return u.String()
}
