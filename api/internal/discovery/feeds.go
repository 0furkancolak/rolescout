package discovery

import (
	"context"
	"strings"
)

// NameFeed produces company names from somewhere other than a career page: a
// stock exchange listing, a sector, a list a person keeps. Names are not boards;
// a FeedSource turns them into boards by probing the job board systems.
type NameFeed interface {
	Name() string
	// Next returns the next batch of names after state ("" at first), the state
	// to keep, and whether the feed has more to give after this batch.
	Next(ctx context.Context, state string) (names []string, next string, more bool, err error)
}

// Gate is implemented by feeds that can be switched off (the AI sector feed).
// A gated-off feed is skipped and keeps its state, so it resumes when switched on.
type Gate interface {
	Ready(ctx context.Context) bool
}

// FeedSource is a discovery Source made of a NameFeed and the name probe. Each
// Discover call handles one batch, so a long feed is read over several runs
// instead of blocking one; Pending keeps the schedule going until it is done.
type FeedSource struct {
	Feed  NameFeed
	Probe *Probe
}

func (f *FeedSource) Name() string { return f.Feed.Name() }

const feedDone = "done"

// Pending is true until the feed has been read to the end.
func (f *FeedSource) Pending(state string) bool {
	if g, ok := f.Feed.(Gate); ok && !g.Ready(context.Background()) {
		return false
	}
	return state != feedDone
}

func (f *FeedSource) Discover(ctx context.Context, state string, fn func(Candidate) error) (string, error) {
	if g, ok := f.Feed.(Gate); ok && !g.Ready(ctx) {
		return state, nil
	}
	if state == feedDone {
		// A finished feed is read again with the normal discovery schedule.
		state = ""
	}
	names, next, more, err := f.Feed.Next(ctx, state)
	if err != nil {
		return state, err
	}
	usable := names[:0:0]
	for _, n := range names {
		if probable(n) {
			usable = append(usable, n)
		}
	}
	if done, err := f.Probe.probeNames(ctx, usable, fn); err != nil {
		// The batch is tried again next time; boards already found are known.
		_ = done
		return state, err
	}
	if !more {
		return feedDone, nil
	}
	return next, nil
}

// cleanCompanyName turns a listing's security name ("Acme Holdings, Inc. -
// Common Stock") into the name people use ("Acme").
func cleanCompanyName(security string) string {
	name, _, _ := strings.Cut(security, " - ")
	name = strings.TrimSpace(name)
	for {
		trimmed := name
		for _, suffix := range companySuffixes {
			low := strings.ToLower(trimmed)
			if strings.HasSuffix(low, suffix) {
				trimmed = strings.TrimRight(strings.TrimSpace(trimmed[:len(trimmed)-len(suffix)]), ",. ")
			}
		}
		if trimmed == name {
			return name
		}
		name = trimmed
	}
}

var companySuffixes = []string{
	" inc.", " inc", " incorporated", " corporation", " corp.", " corp", " co.", " company", " ltd.", " ltd", " limited",
	" plc", " n.v.", " nv", " s.a.", " sa", " ag", " se", " holdings", " holding", " group", " technologies", " technology",
	" l.p.", " lp", " llc", " ordinary shares", " common stock", " class a", " class b",
}
