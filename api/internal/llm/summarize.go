package llm

import (
	"context"
	"fmt"
	"strings"
)

// SummaryPromptVersion is part of the cache key of stored summaries.
const SummaryPromptVersion = 3

const summarySystem = `You summarise a job posting for a job seeker in at most two short sentences (under 220 characters in total): what the role does and what the company is. Start directly with the content, with no introduction. Use only what the posting says; do not add technologies, numbers, salary or promises that are not in it. Reply with the summary text only, no heading, no list, no quotes.`

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
	req := Request{System: summarySystem, User: user, MaxTokens: 110, Temperature: 0.2}
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
	// A model that runs on is cut at its second sentence, not rejected: the
	// reader sees a short summary either way.
	text = firstSentences(text, 2, 300)
	if text == "" || urlPattern.MatchString(text) {
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

// firstSentences keeps at most n sentences within max characters, ending at a
// sentence end; text without one that fits is cut at a word.
func firstSentences(text string, n, max int) string {
	runes := []rune(text)
	count, cut := 0, 0
	for i, r := range runes {
		if r != '.' && r != '!' && r != '?' {
			continue
		}
		if i+1 > max {
			break
		}
		count++
		cut = i + 1
		if count == n {
			break
		}
	}
	switch {
	case len(runes) <= max && count <= n && (cut == 0 || cut == len(runes) || sentences(text) <= n):
		return text
	case cut > 0:
		return strings.TrimSpace(string(runes[:cut]))
	default:
		return truncateRunes(text, max)
	}
}
