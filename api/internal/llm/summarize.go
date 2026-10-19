package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

// SummaryPromptVersion is part of the cache key of stored summaries.
const SummaryPromptVersion = 1

const summarySystem = `You summarise a job posting for a job seeker in two or three short sentences: what the role does and what the company is. Use only what the posting says; do not add technologies, numbers, salary or promises that are not in it. Reply with JSON: {"summary": "..."}.`

var summarySchema = json.RawMessage(`{"type":"object","properties":{"summary":{"type":"string"}},"required":["summary"],"additionalProperties":false}`)

const maxSummaryInput = 6000

// Summarize writes a short summary of a posting in the given language ("tr"
// or "en") and rejects answers that name technologies or numbers the posting
// does not contain.
func Summarize(ctx context.Context, c Completer, title, company, description, language string) (string, error) {
	lang := "English"
	if language == "tr" {
		lang = "Turkish"
	}
	desc := truncateRunes(cleanText(description), maxSummaryInput)
	user := fmt.Sprintf("Write the summary in %s.\nJob: %s at %s\n\nPosting:\n%s", lang, cleanText(title), cleanText(company), desc)
	resp, err := c.Complete(ctx, Request{System: summarySystem, User: user, Schema: summarySchema, MaxTokens: 220})
	if err != nil {
		return "", err
	}
	var out struct {
		Summary string `json:"summary"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(resp.Content)), &out); err != nil {
		return "", ErrInvalidOutput
	}
	text := cleanText(out.Summary)
	if text == "" || urlPattern.MatchString(text) || utf8.RuneCountInString(text) > 450 || sentences(text) > 3 {
		return "", ErrInvalidOutput
	}
	source := strings.ToLower(title + "\n" + description)
	for n := range numbersIn(text) {
		if !strings.Contains(source, fmt.Sprint(n)) {
			return "", ErrInvalidOutput
		}
	}
	lower := strings.ToLower(text)
	for tech := range knownTechnologies {
		if containsWord(lower, tech) && !grounded(tech, source) {
			return "", ErrInvalidOutput
		}
	}
	return text, nil
}
