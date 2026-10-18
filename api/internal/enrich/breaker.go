// Package enrich runs the optional local-model enrichment as its own worker
// loop. Nothing in the research pipeline waits for it.
package enrich

import (
	"sync"
	"time"
)

// Breaker stops sending requests to a model server that keeps failing. After
// threshold consecutive failures it opens for the cool-down; the first request
// after that is a probe, and one more failure reopens it immediately.
type Breaker struct {
	mu        sync.Mutex
	threshold int
	cooldown  time.Duration
	now       func() time.Time
	failures  int
	openUntil time.Time
}

func NewBreaker(threshold int, cooldown time.Duration, now func() time.Time) *Breaker {
	if now == nil {
		now = time.Now
	}
	return &Breaker{threshold: threshold, cooldown: cooldown, now: now}
}

// Allow reports whether a request may be sent now.
func (b *Breaker) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return !b.now().Before(b.openUntil)
}

// Open reports whether the breaker is currently refusing requests.
func (b *Breaker) Open() bool { return !b.Allow() }

// Success closes the breaker and forgets earlier failures.
func (b *Breaker) Success() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures = 0
	b.openUntil = time.Time{}
}

// Failure records a failed request and reports whether it opened the breaker.
func (b *Breaker) Failure() (opened bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures++
	if b.failures >= b.threshold {
		b.openUntil = b.now().Add(b.cooldown)
		return true
	}
	return false
}
