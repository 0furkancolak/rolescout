package crawl

import (
	"context"
	"sync"
	"time"
)

// hostLimiter bounds concurrency and spaces request starts for one host (or
// one vendor, for vendor hosts that serve many companies).
type hostLimiter struct {
	slots    chan struct{}
	interval time.Duration

	mu   sync.Mutex
	next time.Time
}

func newHostLimiter(concurrency int, interval time.Duration) *hostLimiter {
	if concurrency < 1 {
		concurrency = 1
	}
	return &hostLimiter{slots: make(chan struct{}, concurrency), interval: interval}
}

// acquire waits for a free slot and the host's next start time. The returned
// function releases the slot.
func (l *hostLimiter) acquire(ctx context.Context) (func(), error) {
	select {
	case l.slots <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	release := func() { <-l.slots }

	l.mu.Lock()
	now := time.Now()
	start := l.next
	if start.Before(now) {
		start = now
	}
	l.next = start.Add(l.interval)
	l.mu.Unlock()

	if wait := time.Until(start); wait > 0 {
		t := time.NewTimer(wait)
		defer t.Stop()
		select {
		case <-t.C:
		case <-ctx.Done():
			release()
			return nil, ctx.Err()
		}
	}
	return release, nil
}
