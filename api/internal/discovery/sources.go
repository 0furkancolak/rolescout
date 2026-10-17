package discovery

import "context"

// Candidate is a URL that may lead to a company's job board: a board URL, a
// career page or a company website.
type Candidate struct {
	URL    string
	Name   string
	Origin string // seed, hackernews, list, link or manual
}

// Source finds candidates. state is what the source returned last time (""
// at first); Discover returns the state to keep for the next run.
type Source interface {
	Name() string
	Discover(ctx context.Context, state string, fn func(Candidate) error) (string, error)
}
