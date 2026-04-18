// Package clock is a tiny time abstraction so polling loops can be driven
// deterministically in tests.
package clock

import (
	"sync"
	"time"
)

// Clock is the minimal time interface the CLI consumes.
type Clock interface {
	Now() time.Time
	After(d time.Duration) <-chan time.Time
}

// Real is the production clock backed by the time package.
type Real struct{}

func (Real) Now() time.Time                         { return time.Now() }
func (Real) After(d time.Duration) <-chan time.Time { return time.After(d) }

// Fake advances only when Advance is called. After returns a channel that
// fires the first time Advance pushes time past the requested duration.
type Fake struct {
	mu      sync.Mutex
	now     time.Time
	waiters []*waiter
}

type waiter struct {
	deadline time.Time
	ch       chan time.Time
	fired    bool
}

// NewFake constructs a Fake clock starting at t.
func NewFake(t time.Time) *Fake { return &Fake{now: t} }

func (f *Fake) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

func (f *Fake) After(d time.Duration) <-chan time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	ch := make(chan time.Time, 1)
	f.waiters = append(f.waiters, &waiter{
		deadline: f.now.Add(d),
		ch:       ch,
	})
	return ch
}

// Advance moves the fake clock forward by d and fires any After channels
// whose deadline has passed.
func (f *Fake) Advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = f.now.Add(d)
	for _, w := range f.waiters {
		if !w.fired && !f.now.Before(w.deadline) {
			w.fired = true
			w.ch <- f.now
		}
	}
}
