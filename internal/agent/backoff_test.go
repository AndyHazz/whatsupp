package agent

import (
	"testing"
	"time"
)

func TestBackoffAllowsFirstAttempt(t *testing.T) {
	b := NewBackoff(15*time.Second, 30*time.Minute)
	now := time.Date(2026, 8, 17, 20, 0, 0, 0, time.UTC)

	if !b.Allow("docker", now) {
		t.Error("Allow = false on first attempt, want true")
	}
}

func TestBackoffSkipsUntilTheDelayHasPassed(t *testing.T) {
	b := NewBackoff(15*time.Second, 30*time.Minute)
	now := time.Date(2026, 8, 17, 20, 0, 0, 0, time.UTC)

	b.Failure("docker", now) // first failure: retry after 15s

	if b.Allow("docker", now.Add(10*time.Second)) {
		t.Error("Allow = true 10s after failure, want false (delay is 15s)")
	}
	if !b.Allow("docker", now.Add(15*time.Second)) {
		t.Error("Allow = false 15s after failure, want true")
	}
}

func TestBackoffDoublesTheDelayOnEachConsecutiveFailure(t *testing.T) {
	b := NewBackoff(15*time.Second, 30*time.Minute)
	now := time.Date(2026, 8, 17, 20, 0, 0, 0, time.UTC)

	for _, want := range []time.Duration{15 * time.Second, 30 * time.Second, 60 * time.Second} {
		b.Failure("docker", now)
		if b.Allow("docker", now.Add(want-time.Second)) {
			t.Errorf("Allow = true just before %v delay, want false", want)
		}
		if !b.Allow("docker", now.Add(want)) {
			t.Errorf("Allow = false at %v delay, want true", want)
		}
	}
}

func TestBackoffCapsTheDelay(t *testing.T) {
	b := NewBackoff(15*time.Second, 60*time.Second)
	now := time.Date(2026, 8, 17, 20, 0, 0, 0, time.UTC)

	for i := 0; i < 20; i++ {
		b.Failure("docker", now)
	}

	if !b.Allow("docker", now.Add(60*time.Second)) {
		t.Error("Allow = false at the cap, want true - delay grew beyond max")
	}
}

func TestBackoffSuccessClearsTheDelay(t *testing.T) {
	b := NewBackoff(15*time.Second, 30*time.Minute)
	now := time.Date(2026, 8, 17, 20, 0, 0, 0, time.UTC)

	b.Failure("docker", now)
	b.Failure("docker", now)
	b.Success("docker")

	if !b.Allow("docker", now) {
		t.Error("Allow = false immediately after success, want true")
	}
}

func TestBackoffReportsOnlyTheFirstOfARunOfFailures(t *testing.T) {
	b := NewBackoff(15*time.Second, 30*time.Minute)
	now := time.Date(2026, 8, 17, 20, 0, 0, 0, time.UTC)

	if !b.Failure("docker", now) {
		t.Error("Failure = false on first failure, want true (worth logging)")
	}
	if b.Failure("docker", now) {
		t.Error("Failure = true on second consecutive failure, want false (already logged)")
	}
	if b.Failure("docker", now) {
		t.Error("Failure = true on third consecutive failure, want false")
	}
}

func TestBackoffReportsRecoveryOnlyAfterFailures(t *testing.T) {
	b := NewBackoff(15*time.Second, 30*time.Minute)
	now := time.Date(2026, 8, 17, 20, 0, 0, 0, time.UTC)

	if b.Success("docker") {
		t.Error("Success = true with no prior failure, want false (nothing to recover from)")
	}

	b.Failure("docker", now)

	if !b.Success("docker") {
		t.Error("Success = false after a failure, want true (recovery is worth logging)")
	}
	if b.Success("docker") {
		t.Error("Success = true on a second consecutive success, want false")
	}
}

func TestBackoffTracksEachCollectorSeparately(t *testing.T) {
	b := NewBackoff(15*time.Second, 30*time.Minute)
	now := time.Date(2026, 8, 17, 20, 0, 0, 0, time.UTC)

	b.Failure("docker", now)

	if !b.Allow("cpu", now) {
		t.Error("Allow(cpu) = false after docker failed, want true")
	}
}
