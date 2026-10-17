// Package research runs research profiles: it grows the company catalog,
// collects jobs and scores them against what a workspace is looking for.
package research

import (
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"example.com/rolescout/api/internal/domain"
)

// Score weights. They add up to 100.
const (
	titleWeight     = 40
	techWeight      = 30
	locationWeight  = 15
	seniorityWeight = 10
	freshWeight     = 5
)

// fullStackSynonyms are titles that mean full stack work without saying so.
var fullStackSynonyms = []string{"product engineer"}

// devRoleWords give partial title credit when no profile keyword matches.
var devRoleWords = []string{"engineer", "developer", "programmer", "geliştirici", "yazılım", "mühendis"}

// Score rates how well j fits p, from 0 to 100, and explains the score.
// ok is false when a hard filter removes the job.
func Score(p domain.ResearchProfile, j domain.Job, now time.Time) (int, []domain.MatchReason, bool) {
	title := normalize(j.Title)
	text := title + " " + normalize(j.Description)
	for _, kw := range p.ExcludedKeywords {
		if k := normalize(kw); k != "" && containsPhrase(text, k) {
			return 0, nil, false
		}
	}
	inCountry := j.CountryCode != nil && slices.Contains(p.Countries, *j.CountryCode)
	inLocation := j.Location != nil && matchesAny(strings.ToLower(*j.Location), p.Locations)
	if p.Remote == domain.RemoteOnly && j.WorkArrangement == "onsite" && !inCountry && !inLocation {
		return 0, nil, false
	}

	var reasons []domain.MatchReason
	add := func(code string, params map[string]string) {
		reasons = append(reasons, domain.MatchReason{Code: code, Params: params})
	}
	score := 0

	// Title.
	switch kw, strength := titleMatch(p, title); strength {
	case 3:
		score += titleWeight
		add("title", map[string]string{"value": kw})
	case 2:
		score += titleWeight / 2
		add("keyword", map[string]string{"value": kw})
	case 1:
		score += titleWeight / 4
		add("role", map[string]string{"value": kw})
	}

	// Technologies.
	if len(p.Technologies) == 0 {
		score += techWeight / 2
	} else {
		matched := 0
		for _, t := range p.Technologies {
			if slices.Contains(j.Technologies, strings.ToLower(t)) {
				matched++
			}
		}
		if matched > 0 {
			score += techWeight * matched / len(p.Technologies)
			add("tech", map[string]string{"matched": strconv.Itoa(matched), "total": strconv.Itoa(len(p.Technologies))})
		}
	}

	// Location and arrangement.
	switch {
	case j.WorkArrangement == "remote" && p.Remote != domain.OnsiteOK:
		score += locationWeight
		add("remote", nil)
	case inCountry:
		score += locationWeight
		add("country", map[string]string{"value": *j.CountryCode})
	case inLocation:
		score += locationWeight
		add("location", map[string]string{"value": *j.Location})
	case j.WorkArrangement == "hybrid" && p.Remote == domain.RemoteOrHybrid:
		score += locationWeight * 2 / 3
		add("hybrid", nil)
	case len(p.Countries) == 0 && len(p.Locations) == 0 && p.Remote != domain.RemoteOnly:
		score += locationWeight / 2
	}

	// Seniority.
	switch {
	case len(p.Seniority) == 0:
		score += seniorityWeight
	case slices.Contains(p.Seniority, j.Seniority):
		score += seniorityWeight
		add("seniority", map[string]string{"value": j.Seniority})
	case j.Seniority == "unknown" || j.Seniority == "":
		score += seniorityWeight / 2
	}

	// Freshness.
	if now.Sub(j.FirstSeenAt) <= 7*24*time.Hour {
		score += freshWeight
		add("fresh", nil)
	}
	return min(score, 100), reasons, score > 0
}

// titleMatch returns the matching keyword and how strongly the title
// matches: 3 for a profile title keyword (or its synonyms), 2 for another
// profile keyword, 1 for a generic developer role, 0 for none.
func titleMatch(p domain.ResearchProfile, title string) (string, int) {
	compactTitle := strings.ReplaceAll(title, " ", "")
	for _, raw := range p.TitleKeywords {
		kw := normalize(raw)
		if kw == "" {
			continue
		}
		compact := strings.ReplaceAll(kw, " ", "")
		if containsPhrase(title, kw) || (len(compact) > 4 && strings.Contains(compactTitle, compact)) {
			return canonicalKeyword(p, kw), 3
		}
		if compact == "fullstack" {
			for _, syn := range fullStackSynonyms {
				if containsPhrase(title, syn) {
					return canonicalKeyword(p, kw), 3
				}
			}
		}
	}
	for _, raw := range p.Keywords {
		if kw := normalize(raw); kw != "" && containsPhrase(title, kw) {
			return kw, 2
		}
	}
	for _, w := range devRoleWords {
		if containsPhrase(title, w) {
			return w, 1
		}
	}
	return "", 0
}

// canonicalKeyword reports spelling variants ("fullstack", "full-stack") as
// the profile's first variant of the same keyword.
func canonicalKeyword(p domain.ResearchProfile, kw string) string {
	compact := strings.ReplaceAll(kw, " ", "")
	for _, raw := range p.TitleKeywords {
		if n := normalize(raw); strings.ReplaceAll(n, " ", "") == compact {
			return n
		}
	}
	return kw
}

// normalize lower-cases s and turns punctuation into single spaces.
func normalize(s string) string {
	var b strings.Builder
	space := true
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '+' || r == '#' || r == '.' {
			b.WriteRune(r)
			space = false
		} else if !space {
			b.WriteByte(' ')
			space = true
		}
	}
	return strings.TrimSpace(b.String())
}

// containsPhrase reports whether phrase occurs in s on word boundaries.
func containsPhrase(s, phrase string) bool {
	for i := 0; ; {
		j := strings.Index(s[i:], phrase)
		if j < 0 {
			return false
		}
		start, end := i+j, i+j+len(phrase)
		if (start == 0 || s[start-1] == ' ') && (end == len(s) || s[end] == ' ') {
			return true
		}
		i = start + 1
	}
}

func matchesAny(s string, needles []string) bool {
	for _, n := range needles {
		if n = strings.ToLower(strings.TrimSpace(n)); n != "" && strings.Contains(s, n) {
			return true
		}
	}
	return false
}
