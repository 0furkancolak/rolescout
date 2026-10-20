package discovery

import "context"

// Candidate is a URL that may lead to a company's job board: a board URL, a
// career page or a company website.
type Candidate struct {
	URL    string
	Name   string
	Origin string // seed, hackernews, list, link, manual or probe
	// Country is the ISO 3166-1 code the company hires in, when the source knows
	// it (a list of Turkish employers); otherwise it comes from the URL's domain.
	Country string
}

// Pender is implemented by sources that know they have new work whatever the
// discovery schedule says, such as a seed list that grew in a new release.
type Pender interface {
	Pending(state string) bool
}

// CtxPender is a Pender whose work depends on the run, such as the countries
// it focuses on (the context carries them, see package focus).
type CtxPender interface {
	PendingCtx(ctx context.Context, state string) bool
}

// Source finds candidates. state is what the source returned last time (""
// at first); Discover returns the state to keep for the next run.
type Source interface {
	Name() string
	Discover(ctx context.Context, state string, fn func(Candidate) error) (string, error)
}
