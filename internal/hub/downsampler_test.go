package hub

import (
	"testing"
	"time"

	"github.com/andyhazz/whatsupp/internal/store"
)

func TestDownsampler_HourlyAggregation(t *testing.T) {
	s := testStore(t)

	hour1 := int64(1711000000)
	hour1 = hour1 - (hour1 % 3600)
	hour2 := hour1 + 3600

	s.InsertCheckResult("Plex", hour1+10, "up", 40.0, "")
	s.InsertCheckResult("Plex", hour1+20, "up", 60.0, "")
	s.InsertCheckResult("Plex", hour1+30, "down", 0.0, "")

	s.InsertCheckResult("Plex", hour2+10, "up", 30.0, "")
	s.InsertCheckResult("Plex", hour2+20, "up", 50.0, "")
	s.InsertCheckResult("Plex", hour2+30, "up", 70.0, "")

	// Aggregate hour 1 directly via store
	err := s.AggregateCheckResultsHourly(hour1, hour1+3600)
	if err != nil {
		t.Fatalf("AggregateCheckResultsHourly() error: %v", err)
	}

	rows, err := s.GetCheckResultsHourly("Plex", hour1, hour1+3600)
	if err != nil {
		t.Fatalf("GetCheckResultsHourly() error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("hourly rows = %d, want 1", len(rows))
	}
	if rows[0].SuccessCount != 2 {
		t.Errorf("SuccessCount = %d, want 2", rows[0].SuccessCount)
	}
	if rows[0].FailCount != 1 {
		t.Errorf("FailCount = %d, want 1", rows[0].FailCount)
	}
}

func TestDownsampler_Cleanup(t *testing.T) {
	s := testStore(t)

	now := time.Now().Unix()
	old := now - 86400 // 1 day ago (well past 12h retention)

	s.InsertCheckResult("Plex", old, "up", 40.0, "")
	s.InsertCheckResult("Plex", now, "up", 50.0, "")

	cutoff := time.Now().Add(-retainCheckResultsRaw).Unix()
	n, err := s.DeleteOldCheckResults(cutoff)
	if err != nil {
		t.Fatalf("DeleteOldCheckResults() error: %v", err)
	}
	if n != 1 {
		t.Errorf("cleaned up = %d, want 1", n)
	}
}

func TestDownsample5Min(t *testing.T) {
	s := testStore(t)

	base := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)

	for i := 0; i < 10; i++ {
		ts := base.Add(time.Duration(i) * time.Minute)
		s.InsertAgentMetricsBatch("host1", ts, []store.Metric{
			{Name: "cpu.usage_pct", Value: float64(10 + i*10)},
		})
	}

	err := s.AggregateAgentMetrics5Min(base, base.Add(10*time.Minute))
	if err != nil {
		t.Fatalf("AggregateAgentMetrics5Min error: %v", err)
	}

	results, _ := s.QueryAgentMetrics5Min("host1", base, base.Add(10*time.Minute), nil)
	if len(results) != 2 {
		t.Fatalf("got %d 5-min buckets, want 2", len(results))
	}
}

func TestDownsample5Min_MultipleMetrics(t *testing.T) {
	s := testStore(t)

	base := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		ts := base.Add(time.Duration(i) * time.Minute)
		s.InsertAgentMetricsBatch("host1", ts, []store.Metric{
			{Name: "cpu.usage_pct", Value: float64(50 + i)},
			{Name: "mem.usage_pct", Value: float64(60 + i)},
		})
	}

	s.AggregateAgentMetrics5Min(base, base.Add(5*time.Minute))

	results, _ := s.QueryAgentMetrics5Min("host1", base, base.Add(5*time.Minute), nil)
	if len(results) != 2 { // cpu and mem in one bucket
		t.Fatalf("got %d results, want 2", len(results))
	}
}

func TestDownsample5Min_MultipleHosts(t *testing.T) {
	s := testStore(t)

	base := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		ts := base.Add(time.Duration(i) * time.Minute)
		s.InsertAgentMetricsBatch("host1", ts, []store.Metric{{Name: "cpu.usage_pct", Value: 50}})
		s.InsertAgentMetricsBatch("host2", ts, []store.Metric{{Name: "cpu.usage_pct", Value: 70}})
	}

	s.AggregateAgentMetrics5Min(base, base.Add(5*time.Minute))

	r1, _ := s.QueryAgentMetrics5Min("host1", base, base.Add(5*time.Minute), nil)
	r2, _ := s.QueryAgentMetrics5Min("host2", base, base.Add(5*time.Minute), nil)
	if len(r1) != 1 || len(r2) != 1 {
		t.Fatalf("expected 1 result per host, got %d and %d", len(r1), len(r2))
	}
}

func TestDownsample5Min_Idempotent(t *testing.T) {
	s := testStore(t)

	base := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		ts := base.Add(time.Duration(i) * time.Minute)
		s.InsertAgentMetricsBatch("host1", ts, []store.Metric{{Name: "cpu.usage_pct", Value: 50}})
	}

	s.AggregateAgentMetrics5Min(base, base.Add(5*time.Minute))
	s.AggregateAgentMetrics5Min(base, base.Add(5*time.Minute)) // second run

	results, _ := s.QueryAgentMetrics5Min("host1", base, base.Add(5*time.Minute), nil)
	if len(results) != 1 {
		t.Fatalf("got %d results after second run, want 1 (idempotent)", len(results))
	}
}

func TestPurgeRawAgentMetrics(t *testing.T) {
	s := testStore(t)

	now := time.Now()
	old := now.Add(-72 * time.Hour)

	s.InsertAgentMetricsBatch("h", old, []store.Metric{{Name: "cpu.usage_pct", Value: 50}})
	s.InsertAgentMetricsBatch("h", now, []store.Metric{{Name: "cpu.usage_pct", Value: 60}})

	n, err := s.DeleteOldAgentMetrics(now.Add(-48 * time.Hour))
	if err != nil {
		t.Fatalf("DeleteOldAgentMetrics error: %v", err)
	}
	if n != 1 {
		t.Errorf("purged = %d, want 1", n)
	}

	remaining, _ := s.QueryAgentMetricsRaw("h", now.Add(-time.Hour), now.Add(time.Hour), nil)
	if len(remaining) != 1 {
		t.Errorf("remaining = %d, want 1", len(remaining))
	}
}
