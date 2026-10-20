package discovery

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"example.com/rolescout/api/internal/crawl"
	"example.com/rolescout/api/internal/ingest"
)

//go:embed seeds/probe_names.txt
var probeNamesFile string

// Probe finds job boards by company name. For each name it tries the board
// addresses a company with that name would have on Lever, Greenhouse, Ashby,
// Workable and Recruitee, through the systems' public job APIs, and offers
// the boards that exist and list open jobs. It is how companies with a known
// name but no known career page, such as Turkish employers, enter the
// catalog.
type Probe struct {
	Client *crawl.Client
	// Names are company names; default is the embedded list.
	Names []string
	// Batch is how many names are tried per Discover call (default 250).
	Batch int
}

func (p *Probe) Name() string { return "probe" }

// ProbeNames returns the embedded company names and the names of the embedded
// seeds that are not job boards themselves (their career pages are found by
// detection; the probe also looks for their boards by name).
func ProbeNames() []string {
	names := parseNames(probeNamesFile)
	seen := map[string]bool{}
	for _, n := range names {
		seen[strings.ToLower(n)] = true
	}
	for _, file := range []string{turkeyEmployersFile, agenciesFile} {
		seeds, _ := ParseSeeds(file)
		for _, s := range seeds {
			n := probeNameOf(s.Name)
			if n != "" && !seen[strings.ToLower(n)] {
				seen[strings.ToLower(n)] = true
				names = append(names, n)
			}
		}
	}
	return names
}

// probeNameOf is the company name behind a seed's label: "Adecco Türkiye" is
// probed as "Adecco", and labels of career sites ("Turkcell Kariyer") are not
// names of companies.
func probeNameOf(label string) string {
	label = strings.TrimSpace(label)
	if strings.HasSuffix(label, " Kariyer") {
		return ""
	}
	for _, suffix := range []string{" Türkiye", " Turkey"} {
		label = strings.TrimSuffix(label, suffix)
	}
	return label
}

func parseNames(text string) []string {
	var out []string
	seen := map[string]bool{}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || seen[strings.ToLower(line)] {
			continue
		}
		seen[strings.ToLower(line)] = true
		out = append(out, line)
	}
	return out
}

// Slugs returns the board identifiers a company name may use: the name
// without separators and with hyphens, diacritics removed.
func Slugs(name string) []string {
	words := wordPattern.FindAllString(ingest.Fold(name), -1)
	if len(words) == 0 {
		return nil
	}
	out := []string{strings.Join(words, "")}
	if len(words) > 1 {
		out = append(out, strings.Join(words, "-"))
	}
	return out
}

var wordPattern = regexp.MustCompile(`[a-z0-9]+`)

// probeState is "<digest of the names>:<names done>".
func probeState(names []string, done int) string {
	sum := sha256.Sum256([]byte(strings.Join(names, "\n")))
	return hex.EncodeToString(sum[:6]) + ":" + strconv.Itoa(done)
}

// Pending is true until every name of the current list has been tried.
func (p *Probe) Pending(state string) bool {
	names := p.Names
	if names == nil {
		names = ProbeNames()
	}
	digest, n, ok := strings.Cut(state, ":")
	done, _ := strconv.Atoi(n)
	return !ok || digest != probeState(names, 0)[:12] || done < len(names)
}

func (p *Probe) Discover(ctx context.Context, state string, fn func(Candidate) error) (string, error) {
	names := p.Names
	if names == nil {
		names = ProbeNames()
	}
	start := 0
	if digest, n, ok := strings.Cut(state, ":"); ok && digest == probeState(names, 0)[:12] {
		start, _ = strconv.Atoi(n)
	}
	if start >= len(names) {
		return probeState(names, len(names)), nil
	}
	batch := p.Batch
	if batch <= 0 {
		batch = 250
	}
	end := min(start+batch, len(names))
	if i, err := p.probeNames(ctx, names[start:end], fn); err != nil {
		return probeState(names, start+i), err
	}
	return probeState(names, end), nil
}

// probeConcurrency is how many names are tried at the same time. The vendors'
// own limits (the crawler's per-host pacing) still apply to every request.
const probeConcurrency = 8

// probeNames tries each name and reports the boards found. Names are tried
// several at a time; on an error it returns how many names, counted from the
// start, were finished before it.
func (p *Probe) probeNames(ctx context.Context, names []string, fn func(Candidate) error) (int, error) {
	for start := 0; start < len(names); start += probeConcurrency {
		chunk := names[start:min(start+probeConcurrency, len(names))]
		found := make([][]Candidate, len(chunk))
		var wg sync.WaitGroup
		for i, name := range chunk {
			wg.Add(1)
			go func() {
				defer wg.Done()
				found[i] = p.probeName(ctx, name)
			}()
		}
		wg.Wait()
		if ctx.Err() != nil {
			return start, ctx.Err()
		}
		for _, cands := range found {
			for _, c := range cands {
				if err := fn(c); err != nil {
					return start, err
				}
			}
		}
	}
	return len(names), nil
}

// probeName asks every board address a name could have, at the same time (they
// live on different hosts, which the crawler paces separately), and returns
// the boards that exist. An unreachable vendor counts as "no board" so it
// cannot stop the others.
func (p *Probe) probeName(ctx context.Context, name string) []Candidate {
	var targets []probeTarget
	for _, slug := range Slugs(name) {
		targets = append(targets, probeTargets(slug)...)
	}
	found := make([]bool, len(targets))
	var wg sync.WaitGroup
	for j, tg := range targets {
		wg.Add(1)
		go func() {
			defer wg.Done()
			found[j], _ = p.exists(ctx, tg)
		}()
	}
	wg.Wait()
	var out []Candidate
	for j, tg := range targets {
		if found[j] {
			out = append(out, Candidate{URL: tg.url, Name: name, Origin: "probe"})
		}
	}
	return out
}

type probeTarget struct{ url, api, kind string }

// probeTargets lists the candidate boards for a slug: the board URL RoleScout
// would store, the API address that proves it exists, and the response shape.
func probeTargets(slug string) []probeTarget {
	esc := url.PathEscape(slug)
	t := []probeTarget{
		{"https://jobs.lever.co/" + esc, "https://api.lever.co/v0/postings/" + esc + "?mode=json&limit=1", "array"},
		{"https://jobs.eu.lever.co/" + esc, "https://api.eu.lever.co/v0/postings/" + esc + "?mode=json&limit=1", "array"},
		{"https://boards.greenhouse.io/" + esc, "https://boards-api.greenhouse.io/v1/boards/" + esc + "/jobs", "jobs"},
		{"https://jobs.ashbyhq.com/" + esc, "https://api.ashbyhq.com/posting-api/job-board/" + esc, "jobs"},
		{"https://apply.workable.com/" + esc, "https://apply.workable.com/api/v1/widget/accounts/" + esc, "jobs"},
		{"https://ats.rippling.com/" + esc, "https://api.rippling.com/platform/api/ats/v1/board/" + esc + "/jobs", "array"},
		{"https://jobs.gem.com/" + esc, "https://api.gem.com/job_board/v0/" + esc + "/job_posts/", "array"},
	}
	if labelPattern.MatchString(slug) {
		t = append(t, probeTarget{"https://" + slug + ".recruitee.com", "https://" + slug + ".recruitee.com/api/offers/", "offers"})
	}
	return t
}

func (p *Probe) exists(ctx context.Context, t probeTarget) (bool, error) {
	var raw struct {
		Jobs   []any `json:"jobs"`
		Offers []any `json:"offers"`
	}
	if t.kind == "array" {
		var list []any
		err := ingest.FetchJSON(ctx, p.Client, t.api, &list)
		return err == nil && len(list) > 0, ignoreMissing(err)
	}
	err := ingest.FetchJSON(ctx, p.Client, t.api, &raw)
	if t.kind == "offers" {
		return err == nil && len(raw.Offers) > 0, ignoreMissing(err)
	}
	return err == nil && len(raw.Jobs) > 0, ignoreMissing(err)
}

// ignoreMissing turns "no such board" into no error: it is the normal answer.
func ignoreMissing(err error) error {
	if errors.Is(err, ingest.ErrBoardNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("probe: %w", err)
	}
	return nil
}
