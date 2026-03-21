package hub

import (
	"testing"

	"github.com/andyhazz/whatsupp/internal/checks"
)

func TestMonitorState_TransitionsToDown(t *testing.T) {
	ms := NewMonitorState("Test", 3)

	for i := 0; i < 3; i++ {
		transition := ms.RecordResult(checks.Result{
			Monitor: "Test",
			Status:  "down",
			Error:   "connection refused",
		})
		if i < 2 {
			if transition != TransitionNone {
				t.Errorf("iteration %d: transition = %v, want None", i, transition)
			}
		}
	}
	if ms.Status != StatusDown {
		t.Errorf("Status = %v, want DOWN", ms.Status)
	}
}

func TestMonitorState_TransitionsToUp(t *testing.T) {
	ms := NewMonitorState("Test", 3)
	for i := 0; i < 3; i++ {
		ms.RecordResult(checks.Result{Monitor: "Test", Status: "down", Error: "timeout"})
	}
	if ms.Status != StatusDown {
		t.Fatalf("Status = %v, want DOWN after 3 failures", ms.Status)
	}

	transition := ms.RecordResult(checks.Result{Monitor: "Test", Status: "up"})
	if transition != TransitionToUp {
		t.Errorf("transition = %v, want TransitionToUp", transition)
	}
	if ms.Status != StatusUp {
		t.Errorf("Status = %v, want UP", ms.Status)
	}
}

func TestMonitorState_NoTransitionOnIntermittentFailure(t *testing.T) {
	ms := NewMonitorState("Test", 3)

	ms.RecordResult(checks.Result{Monitor: "Test", Status: "down", Error: "timeout"})
	ms.RecordResult(checks.Result{Monitor: "Test", Status: "down", Error: "timeout"})
	ms.RecordResult(checks.Result{Monitor: "Test", Status: "up"})
	transition := ms.RecordResult(checks.Result{Monitor: "Test", Status: "down", Error: "timeout"})

	if ms.Status != StatusUp {
		t.Errorf("Status = %v, want UP (intermittent failures shouldn't trigger DOWN)", ms.Status)
	}
	if transition != TransitionNone {
		t.Errorf("transition = %v, want None", transition)
	}
}

func TestMonitorState_ConsecutiveFailureCount(t *testing.T) {
	ms := NewMonitorState("Test", 5)

	ms.RecordResult(checks.Result{Monitor: "Test", Status: "down", Error: "err"})
	ms.RecordResult(checks.Result{Monitor: "Test", Status: "down", Error: "err"})
	if ms.ConsecutiveFailures != 2 {
		t.Errorf("ConsecutiveFailures = %d, want 2", ms.ConsecutiveFailures)
	}

	ms.RecordResult(checks.Result{Monitor: "Test", Status: "up"})
	if ms.ConsecutiveFailures != 0 {
		t.Errorf("ConsecutiveFailures = %d after success, want 0", ms.ConsecutiveFailures)
	}
}

func TestMonitorState_TransitionToDown_ReturnsTransition(t *testing.T) {
	ms := NewMonitorState("Test", 2)
	ms.RecordResult(checks.Result{Monitor: "Test", Status: "down", Error: "err"})
	transition := ms.RecordResult(checks.Result{Monitor: "Test", Status: "down", Error: "err"})

	if transition != TransitionToDown {
		t.Errorf("transition = %v, want TransitionToDown", transition)
	}
}
