package agent

import (
	"sync"
	"time"
)

// Backoff throttles collectors that keep failing.
//
// A collector that cannot work on a given host - the Docker collector on a host
// with no Docker, say - fails identically every interval, forever. Retrying it
// every 15s and logging each time produces thousands of identical lines a day
// and buries anything real. Backing off keeps the retries (so the collector
// recovers by itself if Docker later appears) while making both the attempts and
// the log lines rare.
//
// Callers report outcomes with Failure and Success, which return whether the
// outcome is worth logging: the first failure of a run and the recovery that
// ends it, but not the repeats in between.
//
// Time is passed in rather than read from the clock so this is testable.
type Backoff struct {
	base time.Duration
	max  time.Duration

	mu    sync.Mutex
	state map[string]*backoffState
}

type backoffState struct {
	failures int
	retryAt  time.Time
}

// NewBackoff returns a Backoff that waits base after the first failure and
// doubles on each consecutive failure, never exceeding max.
func NewBackoff(base, max time.Duration) *Backoff {
	return &Backoff{
		base:  base,
		max:   max,
		state: make(map[string]*backoffState),
	}
}

// Allow reports whether name should be attempted now.
func (b *Backoff) Allow(name string, now time.Time) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	s, ok := b.state[name]
	if !ok || s.failures == 0 {
		return true
	}
	return !now.Before(s.retryAt)
}

// Failure records a failed attempt and reports whether it is worth logging,
// which is true only for the first failure of a run.
func (b *Backoff) Failure(name string, now time.Time) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	s, ok := b.state[name]
	if !ok {
		s = &backoffState{}
		b.state[name] = s
	}

	first := s.failures == 0
	s.failures++
	s.retryAt = now.Add(b.delay(s.failures))
	return first
}

// Success clears any backoff for name and reports whether it is worth logging,
// which is true only when it ends a run of failures.
func (b *Backoff) Success(name string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	s, ok := b.state[name]
	if !ok || s.failures == 0 {
		return false
	}
	s.failures = 0
	s.retryAt = time.Time{}
	return true
}

// delay returns base doubled once per failure after the first, capped at max.
func (b *Backoff) delay(failures int) time.Duration {
	d := b.base
	for i := 1; i < failures; i++ {
		d *= 2
		if d >= b.max {
			return b.max
		}
	}
	return d
}
