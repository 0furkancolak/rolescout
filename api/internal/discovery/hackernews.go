package discovery

import (
	"context"
	"errors"
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"
	"sync"
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
	// ThreadURL, when set (the HN Search API, https://hn.algolia.com/api/v1),
	// returns a whole thread in one request instead of one request per comment:
	// a year of threads in a dozen requests rather than thousands. If it fails,
	// the per-comment API is used.
	ThreadURL string
	// Months is how many monthly threads are read on the first run (default 12).
	Months int
	// Concurrency is how many comments are read at once (default 4, the
	// crawler's limit for the API host).
	Concurrency int
	Now         func() time.Time
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
	workers := h.Concurrency
	if workers <= 0 {
		workers = 4
	}
	for i := len(stories) - 1; i >= 0; i-- {
		story := stories[i]
		if h.ThreadURL != "" {
			switch err := h.readThread(ctx, story.ID, fn); {
			case err == nil:
				if story.Time < cutoff && story.ID > next {
					next = story.ID
				}
				continue
			case ctx.Err() != nil:
				return strconv.FormatInt(next, 10), ctx.Err()
			case errors.Is(err, errCallback):
				return strconv.FormatInt(next, 10), errors.Unwrap(err)
			}
		}
		if err := h.readComments(ctx, base, story.Kids, workers, fn); err != nil {
			return strconv.FormatInt(next, 10), err
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

var errCallback = errors.New("candidate callback failed")

type searchThread struct {
	Children []struct {
		Text string `json:"text"`
	} `json:"children"`
}

// readThread reads one thread's top-level comments (the job posts) from the
// HN Search API. An error from fn is wrapped in errCallback.
func (h *HackerNews) readThread(ctx context.Context, id int64, fn func(Candidate) error) error {
	var t searchThread
	if err := ingest.FetchJSON(ctx, h.Client, fmt.Sprintf("%s/items/%d", strings.TrimRight(h.ThreadURL, "/"), id), &t); err != nil {
		return err
	}
	if len(t.Children) == 0 {
		return fmt.Errorf("thread %d has no comments", id) // let the other API try
	}
	for _, c := range t.Children {
		if c.Text == "" {
			continue
		}
		for _, cand := range commentCandidates(c.Text) {
			if err := fn(cand); err != nil {
				return fmt.Errorf("%w: %w", errCallback, err)
			}
		}
	}
	return nil
}

// readComments reads a thread's comments with a few requests at a time and
// passes their candidates to fn one at a time, in comment order per worker.
func (h *HackerNews) readComments(ctx context.Context, base string, kids []int64, workers int, fn func(Candidate) error) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	work := make(chan int64)
	var mu sync.Mutex
	var firstErr error
	var wg sync.WaitGroup
	for range min(workers, max(len(kids), 1)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for kid := range work {
				var c hnItem
				if err := ingest.FetchJSON(ctx, h.Client, fmt.Sprintf("%s/item/%d.json", base, kid), &c); err != nil {
					continue
				}
				if c.Deleted || c.Dead || c.Text == "" {
					continue
				}
				mu.Lock()
				for _, cand := range commentCandidates(c.Text) {
					if firstErr == nil {
						if err := fn(cand); err != nil {
							firstErr = err
							cancel()
						}
					}
				}
				mu.Unlock()
			}
		}()
	}
feed:
	for _, kid := range kids {
		select {
		case work <- kid:
		case <-ctx.Done():
			break feed
		}
	}
	close(work)
	wg.Wait()
	if firstErr != nil {
		return firstErr
	}
	return ctx.Err()
}

// commentCandidates returns the job board links of a "Who is hiring?"
// comment, or its first link when it has none. The company name is the text
// before the first "|".
func commentCandidates(text string) []Candidate {
	plain := html.UnescapeString(tagPattern.ReplaceAllString(strings.SplitN(text, "<p>", 2)[0], " "))
	name, _, _ := strings.Cut(plain, "|")
	name = strings.Join(strings.Fields(name), " ")
	if r := []rune(name); len(r) > 100 {
		name = string(r[:100])
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
