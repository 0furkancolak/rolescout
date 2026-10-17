package discovery

import (
	"context"
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"
	"time"

	"example.com/rolescout/api/internal/crawl"
	"example.com/rolescout/api/internal/ingest"
)

// HackerNews reads the monthly "Ask HN: Who is hiring?" threads through the
// official Hacker News API (https://github.com/HackerNews/API). Companies post
// one comment each, usually with a link to their job board or career page.
type HackerNews struct {
	Client  *crawl.Client
	BaseURL string // default https://hacker-news.firebaseio.com/v0
	// Months is how many monthly threads are read on the first run (default 12).
	Months int
	Now    func() time.Time
}

func (h *HackerNews) Name() string { return "hackernews" }

type hnItem struct {
	ID      int64   `json:"id"`
	Title   string  `json:"title"`
	Text    string  `json:"text"`
	Time    int64   `json:"time"`
	Kids    []int64 `json:"kids"`
	Deleted bool    `json:"deleted"`
	Dead    bool    `json:"dead"`
}

var (
	hrefPattern = regexp.MustCompile(`href="([^"]+)"`)
	tagPattern  = regexp.MustCompile(`<[^>]+>`)
)

// Discover reads threads newer than state, a story ID. Threads younger than
// 35 days are read again next time, because companies keep posting during
// the month; the returned state is the newest older thread.
func (h *HackerNews) Discover(ctx context.Context, state string, fn func(Candidate) error) (string, error) {
	base := h.BaseURL
	if base == "" {
		base = "https://hacker-news.firebaseio.com/v0"
	}
	months := h.Months
	if months <= 0 {
		months = 12
	}
	now := time.Now
	if h.Now != nil {
		now = h.Now
	}
	done, _ := strconv.ParseInt(state, 10, 64)

	var user struct {
		Submitted []int64 `json:"submitted"`
	}
	if err := ingest.FetchJSON(ctx, h.Client, base+"/user/whoishiring.json", &user); err != nil {
		return state, fmt.Errorf("read whoishiring: %w", err)
	}
	// The account posts three threads a month; only "Who is hiring?" counts.
	var stories []hnItem
	for i, id := range user.Submitted {
		if i >= months*3 || id <= done {
			break
		}
		var it hnItem
		if err := ingest.FetchJSON(ctx, h.Client, fmt.Sprintf("%s/item/%d.json", base, id), &it); err != nil {
			return state, fmt.Errorf("read story %d: %w", id, err)
		}
		if strings.Contains(strings.ToLower(it.Title), "who is hiring") {
			stories = append(stories, it)
		}
	}

	next := done
	cutoff := now().Add(-35 * 24 * time.Hour).Unix()
	for i := len(stories) - 1; i >= 0; i-- {
		story := stories[i]
		for _, kid := range story.Kids {
			var c hnItem
			if err := ingest.FetchJSON(ctx, h.Client, fmt.Sprintf("%s/item/%d.json", base, kid), &c); err != nil {
				if ctx.Err() != nil {
					return strconv.FormatInt(next, 10), ctx.Err()
				}
				continue
			}
			if c.Deleted || c.Dead || c.Text == "" {
				continue
			}
			for _, cand := range commentCandidates(c.Text) {
				if err := fn(cand); err != nil {
					return strconv.FormatInt(next, 10), err
				}
			}
		}
		if story.Time < cutoff && story.ID > next {
			next = story.ID
		}
	}
	if next == 0 {
		return state, nil
	}
	return strconv.FormatInt(next, 10), nil
}

// commentCandidates returns the job board links of a "Who is hiring?"
// comment, or its first link when it has none. The company name is the text
// before the first "|".
func commentCandidates(text string) []Candidate {
	plain := html.UnescapeString(tagPattern.ReplaceAllString(strings.SplitN(text, "<p>", 2)[0], " "))
	name, _, _ := strings.Cut(plain, "|")
	name = strings.Join(strings.Fields(name), " ")
	if len(name) > 100 {
		name = name[:100]
	}
	var boards, others []string
	for _, m := range hrefPattern.FindAllStringSubmatch(text, -1) {
		u := html.UnescapeString(m[1])
		if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
			continue
		}
		if _, ok := ParseBoardURL(u); ok {
			boards = append(boards, u)
		} else {
			others = append(others, u)
		}
	}
	links := boards
	if len(links) == 0 && len(others) > 0 {
		links = others[:1]
	}
	out := make([]Candidate, 0, len(links))
	for _, u := range links {
		out = append(out, Candidate{URL: u, Name: name, Origin: "hackernews"})
	}
	return out
}
