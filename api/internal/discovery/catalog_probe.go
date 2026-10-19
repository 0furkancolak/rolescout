package discovery

import (
	"context"
	"strings"
	"time"
)

// NameLister returns up to limit names of catalog companies that have no
// usable job board, in a stable order, starting after the given name.
type NameLister func(ctx context.Context, after string, limit int) ([]string, error)

// CatalogProbe looks for the job boards of companies the catalog already knows
// by name but could not read: most of the thousands of employers found in
// "Who is hiring?" threads have no career page of their own, yet many keep
// their jobs on Lever, Greenhouse, Ashby, Workable or Recruitee. It walks the
// names in order, one batch per discovery run, and starts over after Every.
type CatalogProbe struct {
	Probe *Probe
	Names NameLister
	// Batch is how many names are tried per Discover call (default 1000).
	Batch int
	// Every is how long a finished pass waits before the next (default 30 days).
	Every time.Duration
}

func (c *CatalogProbe) Name() string { return "catalog_probe" }

const catalogDone = "done:"

// Pending is true until a pass has finished; a finished pass is repeated by
// the normal discovery schedule, not forced.
func (c *CatalogProbe) Pending(state string) bool { return !strings.HasPrefix(state, catalogDone) }

func (c *CatalogProbe) Discover(ctx context.Context, state string, fn func(Candidate) error) (string, error) {
	after := ""
	if rest, ok := strings.CutPrefix(state, catalogDone); ok {
		every := c.Every
		if every <= 0 {
			every = 30 * 24 * time.Hour
		}
		if at, err := time.Parse(time.RFC3339, rest); err == nil && time.Since(at) < every {
			return state, nil
		}
	} else {
		after = state
	}
	batch := c.Batch
	if batch <= 0 {
		batch = 1000
	}
	names, err := c.Names(ctx, after, batch)
	if err != nil {
		return state, err
	}
	usable := names[:0:0]
	for _, n := range names {
		if probable(n) {
			usable = append(usable, n)
		}
	}
	done, err := c.Probe.probeNames(ctx, usable, fn)
	if err != nil {
		// Resume at the first name not finished.
		if done > 0 && done <= len(usable) {
			return usable[done-1], err
		}
		return state, err
	}
	if len(names) < batch {
		return catalogDone + time.Now().UTC().Format(time.RFC3339), nil
	}
	return names[len(names)-1], nil
}

// probable reports whether a catalog name looks like a company name rather
// than a sentence from a post.
func probable(name string) bool {
	return len(name) >= 3 && len(name) <= 40 && len(strings.Fields(name)) <= 4
}
