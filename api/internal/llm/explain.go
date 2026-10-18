package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// ExplainPromptVersion is part of the cache key of stored explanations.
const ExplainPromptVersion = 1

// JobFacts are everything an explanation may be built from. Nothing else is
// given to the model and nothing outside it may appear in the answer.
type JobFacts struct {
	Title               string
	Company             string
	Score               int
	Reasons             []string // the match reasons, already worded
	Technologies        []string // the job's technologies
	MatchedTechnologies []string // those that are also in the profile
	Seniority           string
	Arrangement         string
	Country             string
	Language            string // "en" or "tr"
}

const explainSystem = `You explain in at most two short sentences why a job fits a job seeker's search. Use only the facts you are given; do not add any technology, number, salary, company detail or claim that is not in them. Reply with JSON: {"explanation": "..."}.`

var explainSchema = json.RawMessage(`{"type":"object","properties":{"explanation":{"type":"string"}},"required":["explanation"],"additionalProperties":false}`)

// Explain asks the model for a short reason for a match and rejects answers
// that mention technologies or numbers outside the facts.
func Explain(ctx context.Context, c Completer, f JobFacts) (string, error) {
	lang := "English"
	if f.Language == "tr" {
		lang = "Turkish"
	}
	user := fmt.Sprintf("Write the explanation in %s.\nJob: %s at %s\nScore: %d of 100\nMatch reasons: %s\nJob technologies: %s\nTechnologies that match the search: %s\nSeniority: %s\nWork arrangement: %s\nCountry: %s",
		lang, cleanText(f.Title), cleanText(f.Company), f.Score, strings.Join(f.Reasons, "; "), strings.Join(f.Technologies, ", "),
		strings.Join(f.MatchedTechnologies, ", "), orNone(f.Seniority), orNone(f.Arrangement), orNone(f.Country))
	resp, err := c.Complete(ctx, Request{System: explainSystem, User: user, Schema: explainSchema, MaxTokens: 160})
	if err != nil {
		return "", err
	}
	var out struct {
		Explanation string `json:"explanation"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(resp.Content)), &out); err != nil {
		return "", ErrInvalidOutput
	}
	text := cleanText(out.Explanation)
	if text == "" || utf8.RuneCountInString(text) > 300 || sentences(text) > 2 || !explainGrounded(text, f, user) {
		return "", ErrInvalidOutput
	}
	return text, nil
}

func orNone(s string) string {
	if s == "" || s == "unknown" {
		return "not stated"
	}
	return s
}

var sentenceEnd = regexp.MustCompile(`[.!?]+(\s|$)`)

func sentences(s string) int { return len(sentenceEnd.FindAllStringIndex(s, -1)) }

// explainGrounded checks the answer against the facts: every number must be in
// the prompt facts, and no technology name may appear unless the facts list it.
func explainGrounded(text string, f JobFacts, prompt string) bool {
	facts := strings.ToLower(prompt)
	lower := strings.ToLower(text)
	for n := range numbersIn(text) {
		if !strings.Contains(facts, fmt.Sprint(n)) {
			return false
		}
	}
	for _, m := range numberPattern.FindAllString(text, -1) {
		digits := strings.TrimSpace(m)
		if !strings.Contains(facts, digits) && !strings.Contains(facts, strings.NewReplacer(",", "", ".", "", " ", "").Replace(digits)) {
			return false
		}
	}
	listed := map[string]bool{}
	for _, t := range append(append([]string{}, f.Technologies...), f.MatchedTechnologies...) {
		listed[strings.ToLower(t)] = true
	}
	for tech := range knownTechnologies {
		if containsWord(lower, tech) && !listed[tech] && !groundedAlias(tech, listed) {
			return false
		}
	}
	return true
}

func groundedAlias(tech string, listed map[string]bool) bool {
	for _, a := range technologyAliases[tech] {
		if listed[a] {
			return true
		}
	}
	for canon, aliases := range technologyAliases {
		if listed[canon] {
			for _, a := range aliases {
				if a == tech {
					return true
				}
			}
		}
	}
	return false
}

// knownTechnologies is a vocabulary of common technology names. An explanation
// that names one of them must have been given it.
var knownTechnologies = func() map[string]bool {
	m := map[string]bool{}
	for _, t := range strings.Fields(`go rust python java kotlin swift ruby php scala elixir clojure haskell typescript javascript
		react vue angular svelte next.js nuxt node.js deno express django flask rails laravel spring .net c# c++ dotnet
		postgresql postgres mysql mongodb redis elasticsearch kafka rabbitmq cassandra dynamodb sqlite snowflake bigquery
		kubernetes docker terraform ansible aws gcp azure graphql grpc rest linux git jenkins airflow spark hadoop tensorflow pytorch`) {
		m[t] = true
	}
	return m
}()
