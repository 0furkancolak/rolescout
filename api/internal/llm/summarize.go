package llm

import (
	"context"
	"fmt"
	"strings"
)

// SummaryPromptVersion is part of the cache key of stored summaries.
const SummaryPromptVersion = 4

const summarySystem = `You summarise a job posting for a job seeker in two or three short sentences (under 330 characters in total). Say what the role does day to day, the main requirements, and what kind of company it is when the posting says. Skip boilerplate: equal-opportunity statements, benefits lists, application instructions, company slogans. Start directly with the content, with no introduction. Use only what the posting says; do not add technologies, numbers, salary or promises that are not in it. Write complete sentences that end with a full stop. Reply with the summary text only, no heading, no list, no quotes.`

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
	req := Request{System: summarySystem, User: user, MaxTokens: 260, Temperature: 0.2}
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
	// A model that runs on is cut at its third sentence, not rejected: the
	// reader sees a short summary either way.
	text = firstSentences(text, 3, 380)
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

// ExtractiveSummary is the fallback when the model's answer was rejected twice:
// the first sentences of the posting itself, which cannot say anything the
// posting does not. It returns "" when the posting has no usable text.
func ExtractiveSummary(description string) string {
	text := cleanText(description)
	if len([]rune(text)) < 80 {
		return ""
	}
	return firstSentences(text, 2, 320)
}

// FactsSummary describes a posting that has no description (list-only sources
// such as kariyer.net) from the facts the source did give, without a model:
// there is nothing to summarise and nothing a model could add truthfully.
func FactsSummary(title, company, location, arrangement, employmentType, language string) string {
	title, company = cleanText(title), cleanText(company)
	if title == "" {
		return ""
	}
	tr := language == "tr"
	arr := map[string][2]string{
		"remote": {"remote", "uzaktan"}, "hybrid": {"hybrid", "hibrit"}, "onsite": {"on-site", "iş yerinde"},
	}[arrangement]
	emp := map[string][2]string{
		"full_time": {"full-time", "tam zamanlı"}, "part_time": {"part-time", "yarı zamanlı"},
		"contract": {"contract", "sözleşmeli"}, "internship": {"internship", "staj"}, "temporary": {"temporary", "geçici"},
	}[employmentType]
	pick := func(p [2]string) string {
		if tr {
			return p[1]
		}
		return p[0]
	}
	var details []string
	for _, v := range []string{cleanText(location), pick(arr), pick(emp)} {
		if v != "" {
			details = append(details, v)
		}
	}
	d := strings.Join(details, ", ")
	if tr {
		s := title + " pozisyonu"
		if company != "" {
			s = company + " şirketinin " + s
		}
		if d != "" {
			s += " (" + d + ")"
		}
		return s + ". İlan kaynağında ayrıntılı açıklama yayımlanmıyor; ayrıntılar için ilan bağlantısına bakın."
	}
	s := title + " role"
	if company != "" {
		s += " at " + company
	}
	if d != "" {
		s += " (" + d + ")"
	}
	return s + ". The source publishes no detailed description; see the listing link for details."
}
