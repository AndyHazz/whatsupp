package hub

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andyhazz/whatsupp/internal/checks"
	"github.com/andyhazz/whatsupp/internal/config"
)

// mockChecker records how many times Check was called.
type mockChecker struct {
	callCount atomic.Int32
	result    checks.Result
}

func (m *mockChecker) Check(name string) checks.Result {
	m.callCount.Add(1)
	return m.result
}

func TestScheduler_RunsChecks(t *testing.T) {
	resultCh := make(chan checks.Result, 100)
	mock := &mockChecker{result: checks.Result{Status: "up", LatencyMs: 10}}

	monitors := []config.Monitor{
		{Name: "Test", Type: "http", URL: "http://example.com", Interval: 100 * time.Millisecond},
	}

	s := NewScheduler(monitors, resultCh)
	s.RegisterChecker("Test", mock)
	s.Start()

	// Wait for at least 2 checks to fire
	time.Sleep(250 * time.Millisecond)
	s.Stop()

	count := mock.callCount.Load()
	if count < 2 {
		t.Errorf("check ran %d times, want >= 2", count)
	}

	// Drain results channel
	close(resultCh)
	var results []checks.Result
	for r := range resultCh {
		results = append(results, r)
	}
	if len(results) < 2 {
		t.Errorf("received %d results, want >= 2", len(results))
	}
}

func TestScheduler_StopsCleanly(t *testing.T) {
	resultCh := make(chan checks.Result, 100)
	mock := &mockChecker{result: checks.Result{Status: "up"}}

	monitors := []config.Monitor{
		{Name: "Test", Type: "http", URL: "http://example.com", Interval: 50 * time.Millisecond},
	}

	s := NewScheduler(monitors, resultCh)
	s.RegisterChecker("Test", mock)
	s.Start()
	time.Sleep(100 * time.Millisecond)
	s.Stop()

	countAtStop := mock.callCount.Load()
	time.Sleep(150 * time.Millisecond)
	countAfter := mock.callCount.Load()

	if countAfter != countAtStop {
		t.Errorf("checks continued after Stop(): %d -> %d", countAtStop, countAfter)
	}
}

func TestScheduler_MultipleMonitors(t *testing.T) {
	resultCh := make(chan checks.Result, 100)

	var mu sync.Mutex
	seen := make(map[string]int)

	monitors := []config.Monitor{
		{Name: "A", Type: "http", URL: "http://a.com", Interval: 100 * time.Millisecond},
		{Name: "B", Type: "http", URL: "http://b.com", Interval: 100 * time.Millisecond},
	}

	mockA := &mockChecker{result: checks.Result{Monitor: "A", Status: "up"}}
	mockB := &mockChecker{result: checks.Result{Monitor: "B", Status: "up"}}

	s := NewScheduler(monitors, resultCh)
	s.RegisterChecker("A", mockA)
	s.RegisterChecker("B", mockB)
	s.Start()

	time.Sleep(250 * time.Millisecond)
	s.Stop()
	close(resultCh)

	for r := range resultCh {
		mu.Lock()
		seen[r.Monitor]++
		mu.Unlock()
	}

	if seen["A"] < 1 {
		t.Errorf("monitor A ran %d times, want >= 1", seen["A"])
	}
	if seen["B"] < 1 {
		t.Errorf("monitor B ran %d times, want >= 1", seen["B"])
	}
}
