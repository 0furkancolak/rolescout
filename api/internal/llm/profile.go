package llm

import (
	"context"
	"encoding/json"
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

const profileSystem = `You turn a job seeker's description of the job they want into a search profile. Use only what the description says or clearly implies; leave a list empty when unsure. Rules:
- name: a short label for the search (at most 60 characters), in the language of the description.
- title_keywords: job titles to match, e.g. "backend engineer".
- keywords: other words that should appear in a posting.
- technologies: lowercase technology names, e.g. "go", "postgresql", "react".
- countries: ISO 3166-1 alpha-2 codes such as "TR" or "DE". Only countries the description names.
- locations: cities or regions named in the description.
- excluded_keywords: things the person does not want, e.g. "intern", "php".
- remote: any, remote_only, remote_or_hybrid or onsite_ok.
- seniority: any of intern, junior, mid, senior, lead.
Reply with JSON only.`

var profileSchema = func() json.RawMessage {
	str := map[string]any{"type": "string"}
	list := func(max int, item any) map[string]any {
		return map[string]any{"type": "array", "items": item, "maxItems": max}
	}
	props := map[string]any{
		"name":              str,
		"title_keywords":    list(10, str),
		"keywords":          list(15, str),
		"technologies":      list(20, str),
		"countries":         list(8, str),
		"locations":         list(8, str),
		"excluded_keywords": list(10, str),
		"remote":            map[string]any{"type": "string", "enum": []string{"any", "remote_only", "remote_or_hybrid", "onsite_ok"}},
		"seniority":         list(5, map[string]any{"type": "string", "enum": []string{"intern", "junior", "mid", "senior", "lead"}}),
	}
	raw, _ := json.Marshal(map[string]any{
		"type": "object", "properties": props, "additionalProperties": false,
		"required": []string{"name", "title_keywords", "keywords", "technologies", "countries", "locations", "excluded_keywords", "remote", "seniority"},
	})
	return raw
}()

// SuggestProfile drafts a profile from the description. The result is cleaned
// (trimmed, de-duplicated, bounded); an unusable answer is ErrInvalidOutput.
func SuggestProfile(ctx context.Context, c Completer, description string) (ProfileSuggestion, error) {
	description = strings.TrimSpace(description)
	if utf8.RuneCountInString(description) > MaxProfileDescription {
		description = string([]rune(description)[:MaxProfileDescription])
	}
	resp, err := c.Complete(ctx, Request{System: profileSystem, User: "Description:\n" + description, Schema: profileSchema, MaxTokens: 700})
	if err != nil {
		return ProfileSuggestion{}, err
	}
	var s ProfileSuggestion
	if err := json.Unmarshal([]byte(strings.TrimSpace(resp.Content)), &s); err != nil {
		return ProfileSuggestion{}, ErrInvalidOutput
	}
	s.Name = clip(cleanText(s.Name), 60)
	s.TitleKeywords = tidy(s.TitleKeywords, 10, false)
	s.Keywords = tidy(s.Keywords, 15, false)
	s.Technologies = tidy(s.Technologies, 20, true)
	s.Locations = tidy(s.Locations, 8, false)
	s.ExcludedKeywords = tidy(s.ExcludedKeywords, 10, false)
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
