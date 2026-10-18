// Package ingest imports job listings from external sources. Each source type
// has an Adapter that fetches a snapshot of its current listings; the Runner
// normalizes them and stores them idempotently.
package ingest

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"
)

// RawListing is a listing as an adapter read it from a source, before
// normalization. Adapters fill what the source provides and leave the rest
// empty; the normalizer infers work arrangement, seniority and technologies.
type RawListing struct {
	// ExternalID is the source's own stable identifier for the listing.
	ExternalID string
	URL        string
	Title      string
	Company    string
	Location   string
	// Description may contain HTML; it is converted to plain text.
	Description     string
	EmploymentType  string
	PostedAt        *time.Time
	SourceUpdatedAt *time.Time
	// CountryCode is the ISO 3166-1 alpha-2 country when the source states
	// it; otherwise it is inferred from Location.
	CountryCode string
	// Arrangement is "remote", "hybrid" or "onsite" when the source states
	// it; it takes precedence over inference.
	Arrangement string
}

// Adapter fetches the current listings of one configured source.
type Adapter interface {
	// Validate checks the source configuration before it is stored.
	Validate(config map[string]any) error
	// Fetch returns every listing the source currently publishes. Errors that
	// may succeed on a later attempt should be wrapped with Retryable.
	Fetch(ctx context.Context, config map[string]any) ([]RawListing, error)
}

// Registry maps adapter names, as stored in job_sources.adapter, to adapters.
type Registry map[string]Adapter

func (r Registry) Get(name string) (Adapter, error) {
	a, ok := r[name]
	if !ok {
		return nil, fmt.Errorf("unknown source adapter %q (available: %v)", name, r.Names())
	}
	return a, nil
}

func (r Registry) Names() []string {
	names := make([]string, 0, len(r))
	for name := range r {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

type retryableError struct{ err error }

func (e retryableError) Error() string { return e.err.Error() }
func (e retryableError) Unwrap() error { return e.err }

// Retryable marks a temporary failure, such as a timeout or a 5xx response.
func Retryable(err error) error {
	if err == nil {
		return nil
	}
	return retryableError{err: err}
}

// IsRetryable reports whether err was marked with Retryable.
func IsRetryable(err error) bool {
	var r retryableError
	return errors.As(err, &r)
}

// StringConfig reads a required, non-empty string from an adapter config.
func StringConfig(config map[string]any, key string) (string, error) {
	v, ok := config[key].(string)
	if !ok || v == "" {
		return "", fmt.Errorf("config field %q must be a non-empty string", key)
	}
	return v, nil
}

// snapshotNote accompanies listings an adapter did read. complete is false
// when the snapshot misses listings (pages that failed, a page limit or time
// budget), so listings it lacks must not be closed.
type snapshotNote struct {
	msg      string
	complete bool
}

func (n snapshotNote) Error() string { return n.msg }

// Incomplete returns listings together with a reason why the snapshot is
// missing some; the runner stores them but closes nothing.
func Incomplete(format string, args ...any) error {
	return snapshotNote{msg: fmt.Sprintf(format, args...)}
}

// Note returns listings of a complete snapshot together with something the
// operator should know, such as pages skipped as irrelevant.
func Note(format string, args ...any) error {
	return snapshotNote{msg: fmt.Sprintf(format, args...), complete: true}
}

func asNote(err error) (snapshotNote, bool) {
	var n snapshotNote
	ok := errors.As(err, &n)
	return n, ok
}

// FetchTimeouter is implemented by adapters that need longer than the
// runner's default time for one fetch, such as page-by-page crawls.
type FetchTimeouter interface {
	FetchTimeout(config map[string]any) time.Duration
}

// IsIncompleteSnapshot reports whether err carries listings of a snapshot
// that misses some (see Incomplete).
func IsIncompleteSnapshot(err error) bool {
	n, ok := asNote(err)
	return ok && !n.complete
}
