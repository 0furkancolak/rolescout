package discovery

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"example.com/rolescout/api/internal/crawl"
	"example.com/rolescout/api/internal/ingest"
)

// DefaultLists are public, community-maintained Markdown lists of companies
// with links to their career pages.
var DefaultLists = []string{
	"https://raw.githubusercontent.com/poteto/hiring-without-whiteboards/main/README.md",
}

// MarkdownList reads company links from list items and table rows of
// Markdown documents, such as "- [Acme](https://acme.example/careers) | …".
type MarkdownList struct {
	Client *crawl.Client
	URLs   []string
}

func (l *MarkdownList) Name() string { return "list" }

var listLink = regexp.MustCompile(`^\s*(?:[-*]|\|)\s*\[([^\]]+)\]\((https?://[^)\s]+)\)`)

func (l *MarkdownList) Discover(ctx context.Context, state string, fn func(Candidate) error) (string, error) {
	for _, u := range l.URLs {
		body, err := ingest.Fetch(ctx, l.Client, u, "text/plain, text/markdown")
		if err != nil {
			return state, fmt.Errorf("read list %s: %w", u, err)
		}
		for _, line := range strings.Split(string(body), "\n") {
			m := listLink.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			if err := fn(Candidate{URL: m[2], Name: strings.TrimSpace(m[1]), Origin: "list"}); err != nil {
				return state, err
			}
		}
	}
	return state, nil
}
