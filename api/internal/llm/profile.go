package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

// ProfileSuggestion is a research profile drafted from a free-text description.
// It is a draft: the user reviews it before anything is saved.
type ProfileSuggestion struct {
	Name             string   `json:"name"`
	TitleKeywords    []string `json:"title_keywords"`
	Keywords         []string `json:"keywords"`
	Technologies     []string `json:"technologies"`
	Countries        []string `json:"countries"`
	Locations        []string `json:"locations"`
	ExcludedKeywords []string `json:"excluded_keywords"`
	Remote           string   `json:"remote"`
	Seniority        []string `json:"seniority"`
}

// MaxProfileDescription bounds the text sent to the model.
const MaxProfileDescription = 2000

// Background is what is already known about the person: their own note, what
// was read from their CV and the searches they already run. It lets a short
// description grow into a full profile.
type Background struct {
	About            string
	CVSummary        string
	CVSkills         []string
	CVTitles         []string
	CVSeniority      string
	CVYears          int
	ExistingProfiles []string // names and main titles of the saved searches
}

// Empty reports whether nothing is known.
func (b Background) Empty() bool {
	return strings.TrimSpace(b.About) == "" && b.CVSummary == "" && len(b.CVSkills) == 0 && len(b.CVTitles) == 0 && len(b.ExistingProfiles) == 0
}

func (b Background) prompt() string {
	var sb strings.Builder
	if a := strings.TrimSpace(b.About); a != "" {
		sb.WriteString("\nThe person's own note about the work they look for:\n" + clip(cleanText(a), 1500) + "\n")
	}
	if b.CVSummary != "" {
		sb.WriteString("\nFrom their CV: " + clip(cleanText(b.CVSummary), 600) + "\n")
	}
	if len(b.CVTitles) > 0 {
		sb.WriteString("CV job titles: " + strings.Join(b.CVTitles, ", ") + "\n")
	}
	if len(b.CVSkills) > 0 {
		sb.WriteString("CV skills: " + strings.Join(b.CVSkills, ", ") + "\n")
	}
	if b.CVSeniority != "" {
		sb.WriteString("CV seniority: " + b.CVSeniority)
		if b.CVYears > 0 {
			sb.WriteString(fmt.Sprintf(" (%d years of experience)", b.CVYears))
		}
		sb.WriteString("\n")
	}
	if len(b.ExistingProfiles) > 0 {
		sb.WriteString("Searches they already run (for context; do not repeat them): " + strings.Join(b.ExistingProfiles, "; ") + "\n")
	}
	return sb.String()
}

const profileSystem = `You turn a job seeker's description of the job they want into a job-search profile. The profile is matched against thousands of postings, so it decides how many fitting jobs are found: be generous and concrete, and fill every list as fully as the facts allow. Use the description first; use the background about the person (their note, CV and other searches) to fill what the description leaves open. Never invent facts about the person, but do add the obvious neighbours of what they asked for. Rules:
- name: a short label for the search (at most 60 characters), in the language of the description.
- title_keywords (up to 25): every job title this person would accept, written the way postings write them: the plain title, seniority variants ("senior", "lead", "junior"), common synonyms, spelling variants ("full stack", "fullstack", "full-stack") and the equivalents in the language of the target countries (for Turkey add Turkish titles such as "yazılım geliştirici", "yazılım mühendisi", "backend geliştirici").
- keywords (up to 30): other words that appear in fitting postings: domains, practices, product types, methodologies (for example "microservices", "rest api", "ci/cd", "agile"), in English and the local language.
- technologies (up to 30): lowercase technology names the person works with or clearly could: the language, its frameworks, databases, queues, cloud, tooling, front-end and test tools that normally go with the role. Prefer names postings use ("node.js", "postgresql", "kubernetes").
- countries: ISO 3166-1 alpha-2 codes such as "TR" or "DE". Only countries the description or the person's note names.
- locations (up to 12): cities or regions named, in the common spellings ("İstanbul" and "Istanbul").
- excluded_keywords (up to 20): things the person does not want: what they said, and obviously unrelated role families for this search (for example "sales", "accountant", "driver" for a software search; "intern" for a senior search). Never exclude a technology the person works with.
- remote: any, remote_only, remote_or_hybrid or onsite_ok, from the description or note; any when unsure.
- seniority: every level the person would take, from intern, junior, mid, senior, lead. Include the neighbouring level when the description allows it.
Reply with JSON only.`

var profileSchema = func() json.RawMessage {
	str := map[string]any{"type": "string"}
	list := func(max int, item any) map[string]any {
		return map[string]any{"type": "array", "items": item, "maxItems": max}
	}
	props := map[string]any{
		"name":              str,
		"title_keywords":    list(25, str),
		"keywords":          list(30, str),
		"technologies":      list(30, str),
		"countries":         list(8, str),
		"locations":         list(12, str),
		"excluded_keywords": list(20, str),
		"remote":            map[string]any{"type": "string", "enum": []string{"any", "remote_only", "remote_or_hybrid", "onsite_ok"}},
		"seniority":         list(5, map[string]any{"type": "string", "enum": []string{"intern", "junior", "mid", "senior", "lead"}}),
	}
	raw, _ := json.Marshal(map[string]any{
		"type": "object", "properties": props, "additionalProperties": false,
		"required": []string{"name", "title_keywords", "keywords", "technologies", "countries", "locations", "excluded_keywords", "remote", "seniority"},
	})
	return raw
}()

// SuggestProfile drafts a profile from the description and what is known about
// the person. The result is cleaned (trimmed, de-duplicated, bounded); an
// unusable answer is ErrInvalidOutput.
func SuggestProfile(ctx context.Context, c Completer, description string, bg Background) (ProfileSuggestion, error) {
	description = strings.TrimSpace(description)
	if utf8.RuneCountInString(description) > MaxProfileDescription {
		description = string([]rune(description)[:MaxProfileDescription])
	}
	user := "Description:\n" + description
	if description == "" {
		user = "Description: (none: build the profile from the background alone)"
	}
	if b := bg.prompt(); b != "" {
		user += "\n\nBackground about the person:" + b
	}
	resp, err := c.Complete(ctx, Request{System: profileSystem, User: user, Schema: profileSchema, MaxTokens: 2200})
	if err != nil {
		return ProfileSuggestion{}, err
	}
	var s ProfileSuggestion
	if err := json.Unmarshal([]byte(strings.TrimSpace(resp.Content)), &s); err != nil {
		return ProfileSuggestion{}, ErrInvalidOutput
	}
	s.Name = clip(cleanText(s.Name), 60)
	s.TitleKeywords = tidy(s.TitleKeywords, 25, false)
	s.Keywords = tidy(s.Keywords, 30, false)
	s.Technologies = tidy(s.Technologies, 30, true)
	s.Locations = tidy(s.Locations, 12, false)
	s.ExcludedKeywords = tidy(s.ExcludedKeywords, 20, false)
	// A technology the profile asks for is never also excluded.
	var excluded []string
	for _, e := range s.ExcludedKeywords {
		if !containsString(s.Technologies, strings.ToLower(e)) {
			excluded = append(excluded, e)
		}
	}
	s.ExcludedKeywords = excluded
	var countries []string
	for _, code := range tidy(s.Countries, 8, false) {
		if code = strings.ToUpper(code); len(code) == 2 && code[0] >= 'A' && code[0] <= 'Z' && code[1] >= 'A' && code[1] <= 'Z' {
			countries = append(countries, code)
		}
	}
	s.Countries = countries
	var levels []string
	for _, l := range s.Seniority {
		if l = strings.ToLower(strings.TrimSpace(l)); containsString([]string{"intern", "junior", "mid", "senior", "lead"}, l) && !containsString(levels, l) {
			levels = append(levels, l)
		}
	}
	s.Seniority = levels
	if !containsString([]string{"any", "remote_only", "remote_or_hybrid", "onsite_ok"}, s.Remote) {
		s.Remote = "any"
	}
	if s.Name == "" || (len(s.TitleKeywords) == 0 && len(s.Keywords) == 0 && len(s.Technologies) == 0) {
		return ProfileSuggestion{}, ErrInvalidOutput
	}
	return s, nil
}

func clip(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

func containsString(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

// tidy trims, drops empty, over-long and repeated entries and caps the list.
func tidy(in []string, max int, lower bool) []string {
	var out []string
	for _, s := range in {
		s = cleanText(s)
		if lower {
			s = strings.ToLower(s)
		}
		if s == "" || utf8.RuneCountInString(s) > 50 || containsString(out, s) {
			continue
		}
		out = append(out, s)
		if len(out) == max {
			break
		}
	}
	return out
}
