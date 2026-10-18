package enrich

import (
	"testing"
	"time"
)

type clock struct{ t time.Time }

func (c *clock) now() time.Time          { return c.t }
func (c *clock) advance(d time.Duration) { c.t = c.t.Add(d) }

func TestBreakerOpensAfterThreeFailuresAndRecovers(t *testing.T) {
	c := &clock{t: time.Date(2031, 1, 1, 0, 0, 0, 0, time.UTC)}
	b := NewBreaker(3, 10*time.Minute, c.now)

	if !b.Allow() {
		t.Fatal("a new breaker must allow requests")
	}
	if b.Failure() || b.Failure() {
		t.Fatal("opened before the threshold")
	}
	if !b.Failure() {
		t.Fatal("the third failure must open the breaker (and say so)")
	}
	if b.Allow() {
		t.Fatal("open breaker allowed a request")
	}
	c.advance(9 * time.Minute)
	if b.Allow() {
		t.Fatal("still cooling down")
	}
	c.advance(2 * time.Minute)
	if !b.Allow() {
		t.Fatal("after the cool-down one probe must be allowed")
	}
	// The probe fails: it reopens at once, without needing three more failures.
	if !b.Failure() {
		t.Fatal("a failed probe must reopen the breaker")
	}
	if b.Allow() {
		t.Fatal("reopened breaker allowed a request")
	}
	c.advance(11 * time.Minute)
	b.Success()
	if !b.Allow() || b.Open() {
		t.Fatal("a success must close the breaker")
	}
	// And the failure count restarts from zero.
	b.Failure()
	b.Failure()
	if !b.Allow() {
		t.Fatal("two failures after a success must not open it")
	}
}

func TestBreakerSuccessResetsCount(t *testing.T) {
	c := &clock{t: time.Now()}
	b := NewBreaker(3, time.Minute, c.now)
	b.Failure()
	b.Failure()
	b.Success()
	if b.Failure() {
		t.Fatal("count was not reset by the success")
	}
}
