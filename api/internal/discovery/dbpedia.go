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
var DefaultCompanyCountries = []string{"TR", "US", "GB", "DE", "NL", "FR", "ES", "IT", "PL", "SE", "CH", "IE", "CA", "AU", "IN"}

// dbpediaAgencies is the pseudo country of the worldwide recruitment and
// staffing agencies, software houses and consultancies; it is read after the
// countries.
const dbpediaAgencies = "agencies"

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
	return append(out, dbpediaAgencies)
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
				cand := Candidate{URL: site, Name: r.name, Origin: "dbpedia", Country: country}
				if country == dbpediaAgencies {
					cand.Country = "" // agencies work in many countries: the domain says more
				}
				if err := fn(cand); err != nil {
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

// dbpediaClasses are the kinds of record read for a country's domain: companies
// and the organisations that employ people (banks, airlines, publishers,
// broadcasters, universities, hospitals; agencies are filed as companies).
const dbpediaClasses = "dbo:Company dbo:Organisation dbo:Airline dbo:Bank dbo:Broadcaster dbo:Publisher dbo:RecordLabel dbo:University dbo:Hospital dbo:EducationalInstitution"

// dbpediaNotEmployers are kinds of record that a country's domain also holds
// but that are no employers: clubs and teams, events, people and bands, works.
const dbpediaNotEmployers = "dbo:SportsTeam dbo:SportsClub dbo:SportsLeague dbo:Band dbo:Event dbo:Person dbo:PoliticalParty dbo:Software dbo:VideoGame dbo:Work dbo:Place dbo:MusicalArtist dbo:Website"

// dbpediaCategory names DBpedia's "Companies of <country>" category of the
// countries it is known for; the whole tree below it is read.
var dbpediaCategory = map[string]string{
	"TR": "Companies_of_Turkey", "DE": "Companies_of_Germany", "GB": "Companies_of_the_United_Kingdom",
	"NL": "Companies_of_the_Netherlands", "FR": "Companies_of_France", "US": "Companies_of_the_United_States",
	"ES": "Companies_of_Spain", "IT": "Companies_of_Italy", "PL": "Companies_of_Poland", "SE": "Companies_of_Sweden",
	"CH": "Companies_of_Switzerland", "IE": "Companies_of_the_Republic_of_Ireland", "AT": "Companies_of_Austria",
	"BE": "Companies_of_Belgium", "DK": "Companies_of_Denmark", "NO": "Companies_of_Norway", "FI": "Companies_of_Finland",
	"PT": "Companies_of_Portugal", "CZ": "Companies_of_the_Czech_Republic", "RO": "Companies_of_Romania",
	"GR": "Companies_of_Greece", "CA": "Companies_of_Canada", "AU": "Companies_of_Australia", "IN": "Companies_of_India",
	"AE": "Companies_of_the_United_Arab_Emirates", "IL": "Companies_of_Israel", "SG": "Companies_of_Singapore",
	"JP": "Companies_of_Japan", "BR": "Companies_of_Brazil", "UA": "Companies_of_Ukraine",
}

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
	if country == dbpediaAgencies {
		return w.query(ctx, `# country agencies
SELECT DISTINCT ?c ?site ?name WHERE {
  VALUES ?cat { dbc:Recruitment dbc:Employment_agencies dbc:Staffing_companies dbc:Recruitment_agencies dbc:Executive_search_firms dbc:Human_resource_management_consulting dbc:Employment_agencies_of_the_United_Kingdom dbc:Employment_agencies_of_the_United_States dbc:Employment_agencies_of_the_Netherlands dbc:Employment_agencies_of_Germany dbc:Employment_agencies_of_Japan dbc:Employment_agencies_of_India dbc:Management_consulting_firms dbc:Information_technology_consulting_firms dbc:Information_technology_consulting_firms_of_India dbc:Information_technology_consulting_firms_of_the_United_Kingdom dbc:Information_technology_consulting_firms_of_the_United_States dbc:Business_process_outsourcing_companies dbc:Outsourcing_companies }
  ?c dct:subject ?cat .
  ?c foaf:homepage ?site .
  OPTIONAL { ?c rdfs:label ?name . FILTER(LANG(?name) = "en") }
} ORDER BY ?c LIMIT `+fmt.Sprint(size)+` OFFSET `+fmt.Sprint(offset))
	}
	if len(country) != 2 || strings.ToUpper(country) != country {
		return nil, fmt.Errorf("invalid country %q", country)
	}
	// A company belongs to a country by the domain of its website, or, where
	// that says nothing (the US), by the country DBpedia places it in. A country
	// with a category tree ("Companies of Germany") also offers every company
	// filed under it, whatever its domain.
	var where string
	if label, ok := dbpediaCountryLabel[country]; ok {
		where = fmt.Sprintf(`?c dbo:locationCountry ?co . ?co rdfs:label %q@en .`, label)
	} else if tld := countryTLD(country); tld != "" {
		where = fmt.Sprintf(`FILTER(REGEX(STR(?site), "\\.%s(/|$)"))`, tld)
	}
	var parts []string
	if where != "" {
		parts = append(parts, fmt.Sprintf(`{ VALUES ?t { %s } ?c a ?t . ?c foaf:homepage ?site . %s FILTER NOT EXISTS { VALUES ?no { %s } ?c a ?no } }`, dbpediaClasses, where, dbpediaNotEmployers))
	}
	if cat, ok := dbpediaCategory[country]; ok {
		// The country's category and the two levels below it, and the country's
		// software, video game and IT consulting companies.
		of := strings.TrimPrefix(cat, "Companies_of_")
		parts = append(parts, fmt.Sprintf(`{ ?c dct:subject/skos:broader?/skos:broader? dbc:%s . ?c foaf:homepage ?site }`, cat))
		parts = append(parts, fmt.Sprintf(`{ VALUES ?k { dbc:Software_companies_of_%[1]s dbc:Video_game_companies_of_%[1]s dbc:Information_technology_consulting_firms_of_%[1]s } ?c dct:subject ?k . ?c foaf:homepage ?site }`, of))
	}
	if len(parts) == 0 {
		return nil, fmt.Errorf("no way to tell the companies of %s", country)
	}
	query := fmt.Sprintf(`# country %s
SELECT DISTINCT ?c ?site ?name WHERE {
  %s
  OPTIONAL { ?c rdfs:label ?name . FILTER(LANG(?name) = "en") }
} ORDER BY ?c LIMIT %d OFFSET %d`, country, strings.Join(parts, " UNION "), size, offset)
	return w.query(ctx, query)
}

func (w *DBpedia) query(ctx context.Context, query string) ([]dbpediaRow, error) {
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
	"dbpedia.org", "github.com", "t.me", "bloomberg.com", "crunchbase.com", "google.com", "play.google.com", "archive.org", "web.archive.org", "wikimedia.org",
	"marketplace.atlassian.com", "apps.apple.com", "store.steampowered.com", "imdb.com", "reddit.com",
}

// companySite returns the company's own site as a clean http(s) address (path
// kept, query and fragment dropped), or "" when the value is not one.
func companySite(raw string) string {
	raw = strings.TrimSpace(raw)
	// Some records write two addresses as "a%7Cb"; the first is the site.
	for _, bar := range []string{"%7C", "%7c", "|"} {
		if i := strings.Index(raw, bar); i > 0 {
			raw = raw[:i]
		}
	}
	u, err := url.Parse(raw)
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
