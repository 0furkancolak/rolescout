package crawl

import (
	"sync/atomic"
	"time"
)

// Tuning is the pace of the crawl. It can change while the worker runs.
type Tuning struct {
	// Concurrency is how many companies are detected and synced at once.
	Concurrency int
	// HostInterval spaces requests to one ordinary host.
	HostInterval time.Duration
	// VendorConcurrency and VendorInterval pace a job board API shared by
	// many companies.
	VendorConcurrency int
	VendorInterval    time.Duration
}

// Live holds the Tuning in effect and hands out changes to the crawl client
// and the pipeline without a restart.
type Live struct {
	cur atomic.Pointer[Tuning]
	ver atomic.Uint64
}

// NewLive starts with t.
func NewLive(t Tuning) *Live {
	l := &Live{}
	l.cur.Store(&t)
	return l
}

// Get returns the tuning in effect.
func (l *Live) Get() Tuning { return *l.cur.Load() }

// Version changes whenever Set stores a different tuning.
func (l *Live) Version() uint64 { return l.ver.Load() }

// Set stores t; it reports whether anything changed.
func (l *Live) Set(t Tuning) bool {
	if *l.cur.Load() == t {
		return false
	}
	l.cur.Store(&t)
	l.ver.Add(1)
	return true
}
