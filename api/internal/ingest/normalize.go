package ingest

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"

	xhtml "golang.org/x/net/html"

	"example.com/rolescout/api/internal/infrastructure/database"
)

// ErrInvalidListing marks listings that failed validation, so the runner can
// count them separately from storage errors.
var ErrInvalidListing = errors.New("invalid listing")

const (
	maxTitleLength       = 300
	maxCompanyLength     = 200
	maxLocationLength    = 200
	maxDescriptionLength = 50_000
)

// Normalize validates a raw listing and turns it into the stored
// representation. seenAt is when the listing was fetched.
func Normalize(sourceID string, raw RawListing, seenAt time.Time) (database.IngestedListing, error) {
	title := truncate(cleanLine(raw.Title), maxTitleLength)
	company := truncate(cleanLine(raw.Company), maxCompanyLength)
	location := truncate(cleanLine(raw.Location), maxLocationLength)
	externalID := strings.TrimSpace(raw.ExternalID)

	var problems []string
	if externalID == "" {
		problems = append(problems, "external id is empty")
	}
	if title == "" {
		problems = append(problems, "title is empty")
	}
	if company == "" {
		problems = append(problems, "company is empty")
	}
	if u, err := url.Parse(strings.TrimSpace(raw.URL)); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		problems = append(problems, "url must be an absolute http(s) URL")
	}
	if len(problems) > 0 {
		return database.IngestedListing{}, fmt.Errorf("%w %q: %s", ErrInvalidListing, externalID, strings.Join(problems, "; "))
	}

	description := truncate(HTMLToText(raw.Description), maxDescriptionLength)
	var locPtr *string
	if location != "" {
		locPtr = &location
	}
	searchText := strings.ToLower(title + "\n" + description)
	arrangement := workArrangement(title, location)
	switch raw.Arrangement {
	case "remote", "hybrid", "onsite":
		arrangement = raw.Arrangement
	}
	country := strings.ToUpper(strings.TrimSpace(raw.CountryCode))
	if !countryCode.MatchString(country) {
		country = InferCountry(raw.CountryCode)
	}
	if country == "" {
		country = InferCountry(location)
	}
	var countryPtr *string
	if country != "" {
		countryPtr = &country
	}

	return database.IngestedListing{
		SourceID:        sourceID,
		ExternalID:      externalID,
		URL:             strings.TrimSpace(raw.URL),
		SourcePostedAt:  raw.PostedAt,
		SourceUpdatedAt: raw.SourceUpdatedAt,
		SeenAt:          seenAt,
		DedupeKey:       DedupeKey(company, title, location),
		Title:           title,
		CompanyName:     company,
		Location:        locPtr,
		WorkArrangement: arrangement,
		CountryCode:     countryPtr,
		EmploymentType:  employmentType(raw.EmploymentType, title),
		Seniority:       seniority(title),
		Technologies:    technologies(strings.ToLower(title), searchText),
		Description:     description,
	}, nil
}

// DedupeKey identifies the same job across sources: same company, title and
// location after normalization. The "v1:" prefix allows a future change of the
// algorithm without colliding with keys already stored.
func DedupeKey(company, title, location string) string {
	sum := sha256.Sum256([]byte(keyPart(company) + "|" + keyPart(title) + "|" + keyPart(location)))
	return "v1:" + hex.EncodeToString(sum[:])
}

// keyPart lower-cases s and keeps only letters and digits separated by single
// spaces, so punctuation and spacing differences do not split a job.
func keyPart(s string) string {
	var b strings.Builder
	space := false
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if space && b.Len() > 0 {
				b.WriteByte(' ')
			}
			b.WriteRune(r)
			space = false
		} else {
			space = true
		}
	}
	return b.String()
}

func cleanLine(s string) string {
	return strings.Join(strings.Fields(html.UnescapeString(s)), " ")
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	// Cut on a rune boundary.
	for max > 0 && !utf8RuneStart(s[max]) {
		max--
	}
	return s[:max]
}

func utf8RuneStart(b byte) bool { return b&0xC0 != 0x80 }

var blockTags = map[string]bool{
	"p": true, "br": true, "div": true, "li": true, "ul": true, "ol": true, "tr": true,
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true, "section": true,
}

// HTMLToText converts listing HTML to plain text with paragraph breaks. Input
// that was HTML-escaped once more (as the Greenhouse API returns it) is
// unescaped first. Script and style contents are dropped.
func HTMLToText(s string) string {
	if strings.Contains(s, "&lt;") {
		s = html.UnescapeString(s)
	}
	z := xhtml.NewTokenizer(strings.NewReader(s))
	var b strings.Builder
	skip := 0
	for {
		tt := z.Next()
		switch tt {
		case xhtml.ErrorToken:
			return tidyText(b.String())
		case xhtml.TextToken:
			if skip == 0 {
				b.Write(z.Text())
			}
		case xhtml.StartTagToken, xhtml.EndTagToken, xhtml.SelfClosingTagToken:
			name, _ := z.TagName()
			tag := string(name)
			if tag == "script" || tag == "style" {
				if tt == xhtml.StartTagToken {
					skip++
				} else if tt == xhtml.EndTagToken && skip > 0 {
					skip--
				}
				continue
			}
			if blockTags[tag] {
				b.WriteByte('\n')
				if tag == "li" && tt == xhtml.StartTagToken {
					b.WriteString("• ")
				}
			}
		}
	}
}

// tidyText collapses spaces within lines and keeps at most one empty line
// between paragraphs.
func tidyText(s string) string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	blank := false
	for _, line := range lines {
		line = strings.Join(strings.Fields(line), " ")
		if line == "" || line == "•" {
			if !blank && len(out) > 0 {
				out = append(out, "")
			}
			blank = true
			continue
		}
		out = append(out, line)
		blank = false
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

func word(pattern string) *regexp.Regexp {
	return regexp.MustCompile(`(^|[^\pL\pN+#])(` + pattern + `)($|[^\pL\pN+#])`)
}

var (
	remotePattern = word(`remote|uzaktan|fully remote|work from home`)
	hybridPattern = word(`hybrid|hibrit`)
	onsitePattern = word(`on-?site|in-office|ofisten`)
)

func workArrangement(title, location string) string {
	s := strings.ToLower(title + " " + location)
	switch {
	case hybridPattern.MatchString(s):
		return "hybrid"
	case remotePattern.MatchString(s):
		return "remote"
	case onsitePattern.MatchString(s):
		return "onsite"
	default:
		return "unknown"
	}
}

var seniorityPatterns = []struct {
	level   string
	pattern *regexp.Regexp
}{
	{"intern", word(`intern|internship|stajyer|trainee`)},
	{"lead", word(`lead|principal|staff|head of|tech lead|team lead`)},
	{"senior", word(`senior|sr\.?|kıdemli`)},
	{"junior", word(`junior|jr\.?|entry[- ]level|graduate`)},
	{"mid", word(`mid|mid-level|intermediate`)},
}

func seniority(title string) string {
	s := strings.ToLower(title)
	for _, p := range seniorityPatterns {
		if p.pattern.MatchString(s) {
			return p.level
		}
	}
	return "unknown"
}

var employmentPatterns = []struct {
	kind    string
	pattern *regexp.Regexp
}{
	{"internship", word(`intern|internship|staj`)},
	{"contract", word(`contract|contractor|freelance|sözleşmeli`)},
	{"part_time", word(`part[- ]time|yarı zamanlı`)},
	{"temporary", word(`temporary|temp|geçici`)},
	{"full_time", word(`full[- ]time|tam zamanlı|permanent`)},
}

func employmentType(given, title string) string {
	s := strings.ToLower(given + " " + title)
	for _, p := range employmentPatterns {
		if p.pattern.MatchString(s) {
			return p.kind
		}
	}
	return "unknown"
}

// technologyAliases maps a canonical technology name to the spellings that
// identify it. Short or ambiguous names (like "go") are only matched in the
// title, where they rarely mean something else.
var technologyAliases = []struct {
	name      string
	anywhere  *regexp.Regexp
	titleOnly *regexp.Regexp
}{
	{"go", word(`golang`), word(`go`)},
	{"python", word(`python`), nil},
	{"java", word(`java`), nil},
	{"kotlin", word(`kotlin`), nil},
	{"javascript", word(`javascript`), word(`js`)},
	{"typescript", word(`typescript`), word(`ts`)},
	{"react", word(`react|react\.js|reactjs`), nil},
	{"vue", word(`vue|vue\.js|vuejs`), nil},
	{"angular", word(`angular`), nil},
	{"node.js", word(`node\.js|nodejs`), word(`node`)},
	{"ruby", word(`ruby|rails|ruby on rails`), nil},
	{"php", word(`php|laravel|symfony`), nil},
	{"c#", word(`c#|\.net|dotnet`), nil},
	{"c++", word(`c\+\+`), nil},
	{"rust", word(`rust`), nil},
	{"scala", word(`scala`), nil},
	{"elixir", word(`elixir`), nil},
	{"swift", word(`swift`), nil},
	{"flutter", word(`flutter`), nil},
	{"postgresql", word(`postgres|postgresql`), nil},
	{"mysql", word(`mysql`), nil},
	{"mongodb", word(`mongodb|mongo`), nil},
	{"redis", word(`redis`), nil},
	{"kafka", word(`kafka`), nil},
	{"kubernetes", word(`kubernetes|k8s`), nil},
	{"docker", word(`docker`), nil},
	{"terraform", word(`terraform`), nil},
	{"aws", word(`aws|amazon web services`), nil},
	{"gcp", word(`gcp|google cloud`), nil},
	{"azure", word(`azure`), nil},
	{"graphql", word(`graphql`), nil},
	{"django", word(`django`), nil},
	{"spring", word(`spring boot|spring framework`), word(`spring`)},
}

func technologies(title, text string) []string {
	var out []string
	for _, t := range technologyAliases {
		if t.anywhere.MatchString(text) || (t.titleOnly != nil && t.titleOnly.MatchString(title)) {
			out = append(out, t.name)
		}
	}
	return out
}

// cvShortNames are technologies whose short spelling is also an everyday word
// or an abbreviation. A CV names them as skills, usually capitalised or in a
// list, so they count only in that spelling ("Go", "JS", "TS", "Node").
var cvShortNames = []struct {
	name string
	re   *regexp.Regexp
}{
	{"go", regexp.MustCompile(`(^|[\s,;:/(•·|*-])Go($|[\s,;:/).•·|*-])`)},
	{"javascript", regexp.MustCompile(`(^|[\s,;:/(•·|*-])(JS|JavaScript)($|[\s,;:/).•·|*-])`)},
	{"typescript", regexp.MustCompile(`(^|[\s,;:/(•·|*-])(TS|TypeScript)($|[\s,;:/).•·|*-])`)},
	{"node.js", regexp.MustCompile(`(^|[\s,;:/(•·|*-])Node($|[\s,;:/).•·|*-])`)},
	{"spring", regexp.MustCompile(`(^|[\s,;:/(•·|*-])Spring($|[\s,;:/).•·|*-])`)},
}

// ExtractTechnologies lists the technologies a free text names, in the same
// canonical spelling the job normalizer uses (a CV is read the way a posting
// is, plus the short spellings a CV lists its skills with).
func ExtractTechnologies(text string) []string {
	out := technologies("", strings.ToLower(text))
	for _, c := range cvShortNames {
		if !slices.Contains(out, c.name) && c.re.MatchString(text) {
			out = append(out, c.name)
		}
	}
	return out
}
