package llm

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"
)

// SummaryPromptVersion is part of the cache key of stored summaries.
const SummaryPromptVersion = 2

const summarySystem = `You summarise a job posting for a job seeker in two or three short sentences: what the role does and what the company is. Use only what the posting says; do not add technologies, numbers, salary or promises that are not in it. Reply with the summary text only, no heading, no list, no quotes.`

const maxSummaryInput = 6000

// Summarize writes a short summary of a posting in the given language ("tr"
// or "en") and rejects answers that name technologies or numbers the posting
// does not contain. When onDelta is set and the client can stream, the text is
// handed over as it is written; it is checked only once complete, so a caller
// must discard what it showed if an error comes back.
func Summarize(ctx context.Context, c Completer, title, company, description, language string, onDelta func(string)) (string, error) {
	lang := "English"
	if language == "tr" {
		lang = "Turkish"
	}
	desc := truncateRunes(cleanText(description), maxSummaryInput)
	user := fmt.Sprintf("Write the summary in %s.\nJob: %s at %s\n\nPosting:\n%s", lang, cleanText(title), cleanText(company), desc)
	req := Request{System: summarySystem, User: user, MaxTokens: 220}
	var resp Response
	var err error
	if st, ok := c.(Streamer); ok && onDelta != nil {
		resp, err = st.Stream(ctx, req, onDelta)
	} else {
		resp, err = c.Complete(ctx, req)
	}
	if err != nil {
		return "", err
	}
	text := cleanText(strings.Trim(strings.TrimSpace(resp.Content), "\"“”"))
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
