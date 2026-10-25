package ingest

import (
	"regexp"
	"strings"
	"unicode"
)

// Some sources publish things that are not jobs ("General Application",
// "Join our talent community") or names that are whole sentences (a "Who is
// hiring?" comment's first line). The checks below keep both out of the
// catalog.

var nonJobTitle = regexp.MustCompile(`(?i)\b(general|open|spontaneous|speculative|unsolicited)\s+(application|applications|apply)\b|\b(talent|candidate)\s+(community|pool|network|pipeline)\b|\bfuture\s+(opportunit|openings|roles|vacanc)|\bdo not apply\b|\binitiativbewerbung\b|\bcandidature\s+(spontan|libre)|\bgenel\s+başvuru|\baçık\s+başvuru|\bbolsa de talento\b`)

var nonJobTitleExact = map[string]bool{
	"remote": true, "hybrid": true, "onsite": true, "on-site": true, "full-time": true, "full time": true,
	"part-time": true, "contract": true, "hiring": true, "we are hiring": true, "test": true, "n/a": true,
}

// IsJunkListing reports whether a raw listing is not a real job: a general
// application or talent-pool entry, or a title that is a link, a single
// symbol or only a place or contract word. The runner leaves such listings
// out of a snapshot, so ones stored earlier close with the next sync.
func IsJunkListing(raw RawListing) bool {
	title := cleanLine(raw.Title)
	if title == "" {
		return false // reported as invalid by Normalize
	}
	lower := strings.ToLower(title)
	if nonJobTitleExact[lower] || strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "www.") {
		return true
	}
	letters := 0
	for _, r := range title {
		if unicode.IsLetter(r) {
			letters++
		}
	}
	return letters < 2 || nonJobTitle.MatchString(title)
}

var (
	companyURL    = regexp.MustCompile(`(?i)\(?\s*(https?://|www\.)\S*\s*\)?`)
	companyParens = regexp.MustCompile(`\s*\([^)]*\)?`)
	// Where a company name ends and the rest of a sentence begins.
	companyCut = regexp.MustCompile(`\s+[-–—]\s+|\s*[:|]\s+|\s*/\s+|,\s+|\s+(is|are|we|was|has|have|hiring|looking|building)\s+`)
)

const maxCleanCompany = 60

// sentenceStart begins things that are not names: greetings, "Hiring: …",
// "Location: …" and role titles from a "Who is hiring?" first line.
var sentenceStart = regexp.MustCompile(`(?i)^(hi|hello|hey|we|we.re|i|i.m|how|looking|hiring|please|are you|do you|join|our|at|location|company|senior|staff|jobs?)\b`)

// PlausibleCompanyName reports whether name reads as a company's name rather
// than a sentence or a fragment of a post.
func PlausibleCompanyName(name string) bool {
	name = strings.TrimSpace(name)
	return name != "" && len([]rune(name)) <= maxCleanCompany && len(strings.Fields(name)) <= 7 &&
		!sentenceStart.MatchString(name) && !strings.Contains(name, ":")
}

// CleanCompanyName turns a name that is really a sentence or carries extras
// into the company's name: "xAI (Yes, we just got acquired…)" becomes "xAI",
// "Docker Inc, Remote (62 positions…" "Docker Inc". A short, plain name is
// returned as it is.
func CleanCompanyName(name string) string {
	name = strings.Join(strings.Fields(name), " ")
	if len([]rune(name)) <= 24 && !strings.ContainsAny(name, "()|") && !companyURL.MatchString(name) {
		return name
	}
	cleaned := strings.TrimSpace(companyURL.ReplaceAllString(name, " "))
	cleaned = strings.Join(strings.Fields(companyParens.ReplaceAllString(cleaned, " ")), " ")
	if loc := companyCut.FindStringIndex(cleaned); loc != nil && loc[0] > 0 {
		cleaned = cleaned[:loc[0]]
	}
	cleaned = strings.Trim(strings.TrimSpace(cleaned), " -–—:|,.;/")
	if !PlausibleCompanyName(cleaned) {
		return name
	}
	return cleaned
}
