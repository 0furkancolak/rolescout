package llm

import (
	"context"
	"encoding/json"
	"strings"
)

// CVProfile is what the model read from a CV.
type CVProfile struct {
	Titles    []string `json:"titles"`
	Skills    []string `json:"skills"`
	Seniority string   `json:"seniority"`
	Years     int      `json:"years_experience"`
	Summary   string   `json:"summary"`
}

const maxCVInput = 12000

const cvSystem = `You read a CV and report what it says about the person, for matching them to job postings. Use only what the CV states. Rules:
- titles (up to 8): job titles the person has held or aims for, as written.
- skills (up to 40): technologies, languages, frameworks, databases, cloud and tools named in the CV, lowercase, in the spelling postings use ("node.js", "postgresql"). Only skills that appear in the CV.
- seniority: one of intern, junior, mid, senior, lead, or "" when the CV does not allow a guess.
- years_experience: total years of professional experience as a whole number, 0 when unclear.
- summary: two or three sentences, in the language of the CV, on who this person is professionally (role, strengths, domains). No personal data such as name, address, phone or email.
Reply with JSON only.`

var cvSchema = func() json.RawMessage {
	str := map[string]any{"type": "string"}
	list := func(max int) map[string]any { return map[string]any{"type": "array", "items": str, "maxItems": max} }
	raw, _ := json.Marshal(map[string]any{
		"type": "object", "additionalProperties": false,
		"properties": map[string]any{
			"titles": list(8), "skills": list(40),
			"seniority":        map[string]any{"type": "string", "enum": []string{"", "intern", "junior", "mid", "senior", "lead"}},
			"years_experience": map[string]any{"type": "integer", "minimum": 0, "maximum": 60},
			"summary":          str,
		},
		"required": []string{"titles", "skills", "seniority", "years_experience", "summary"},
	})
	return raw
}()

// ParseCV asks the model what a CV says. Skills the CV text does not contain
// are dropped, so a guess never becomes a fact about the person.
func ParseCV(ctx context.Context, c Completer, text string) (CVProfile, error) {
	text = truncateRunes(strings.TrimSpace(text), maxCVInput)
	resp, err := c.Complete(ctx, Request{System: cvSystem, User: "CV:\n" + text, Schema: cvSchema, MaxTokens: 1200, Temperature: 0.1})
	if err != nil {
		return CVProfile{}, err
	}
	var p CVProfile
	if err := json.Unmarshal([]byte(strings.TrimSpace(resp.Content)), &p); err != nil {
		return CVProfile{}, ErrInvalidOutput
	}
	lower := strings.ToLower(text)
	var skills []string
	for _, s := range tidy(p.Skills, 40, true) {
		if strings.Contains(lower, s) {
			skills = append(skills, s)
		}
	}
	p.Skills = skills
	p.Titles = tidy(p.Titles, 8, false)
	p.Summary = clip(cleanText(p.Summary), 600)
	p.Seniority = strings.ToLower(strings.TrimSpace(p.Seniority))
	if !containsString([]string{"", "intern", "junior", "mid", "senior", "lead"}, p.Seniority) {
		p.Seniority = ""
	}
	if p.Years < 0 || p.Years > 60 {
		p.Years = 0
	}
	if p.Summary == "" && len(p.Skills) == 0 && len(p.Titles) == 0 {
		return CVProfile{}, ErrInvalidOutput
	}
	return p, nil
}
