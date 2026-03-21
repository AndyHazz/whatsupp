package hub

import (
	"testing"
	"time"

	"github.com/andyhazz/whatsupp/internal/store"
)

func TestDownsampler_HourlyAggregation(t *testing.T) {
	s := testStore(t)

	// Insert check results across two hours
	hour1 := int64(1711000000) // some epoch, truncated to hour
	hour1 = hour1 - (hour1 % 3600)
	hour2 := hour1 + 3600

	// Hour 1: 2 up, 1 down
	s.InsertCheckResult("Plex", hour1+10, "up", 40.0, "")
	s.InsertCheckResult("Plex", hour1+20, "up", 60.0, "")
	s.InsertCheckResult("Plex", hour1+30, "down", 0.0, "")

	// Hour 2: 3 up
	s.InsertCheckResult("Plex", hour2+10, "up", 30.0, "")
	s.InsertCheckResult("Plex", hour2+20, "up", 50.0, "")
	s.InsertCheckResult("Plex", hour2+30, "up", 70.0, "")

	d := NewDownsampler(s, DefaultRetentionConfig())

	// Aggregate hour 1
	err := d.AggregateHour(hour1)
	if err != nil {
		t.Fatalf("AggregateHour() error: %v", err)
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
	old := now - 86400*31 // 31 days ago

	s.InsertCheckResult("Plex", old, "up", 40.0, "")
	s.InsertCheckResult("Plex", now, "up", 50.0, "")

	d := NewDownsampler(s, DefaultRetentionConfig())
	n, err := d.CleanupRawCheckResults()
	if err != nil {
		t.Fatalf("CleanupRawCheckResults() error: %v", err)
	}
	if n != 1 {
		t.Errorf("cleaned up = %d, want 1", n)
	}
}

func TestDownsample5Min(t *testing.T) {
	s := testStore(t)

	base := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)

	// Insert raw metrics spanning 10 minutes
	for i := 0; i < 10; i++ {
		ts := base.Add(time.Duration(i) * time.Minute)
		s.InsertAgentMetricsBatch("host1", ts, []store.Metric{
			{Name: "cpu.usage_pct", Value: float64(10 + i*10)},
		})
	}

	d := NewDownsampler(s, DefaultRetentionConfig())
	err := d.AggregateAgentMetrics5Min(base, base.Add(10*time.Minute))
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

	d := NewDownsampler(s, DefaultRetentionConfig())
	d.AggregateAgentMetrics5Min(base, base.Add(5*time.Minute))

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

	d := NewDownsampler(s, DefaultRetentionConfig())
	d.AggregateAgentMetrics5Min(base, base.Add(5*time.Minute))

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

	d := NewDownsampler(s, DefaultRetentionConfig())
	d.AggregateAgentMetrics5Min(base, base.Add(5*time.Minute))
	d.AggregateAgentMetrics5Min(base, base.Add(5*time.Minute)) // second run

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

	d := NewDownsampler(s, DefaultRetentionConfig())
	n, err := d.PurgeRawAgentMetrics(now.Add(-48 * time.Hour))
	if err != nil {
		t.Fatalf("PurgeRawAgentMetrics error: %v", err)
	}
	if n != 1 {
		t.Errorf("purged = %d, want 1", n)
	}

	remaining, _ := s.QueryAgentMetricsRaw("h", now.Add(-time.Hour), now.Add(time.Hour), nil)
	if len(remaining) != 1 {
		t.Errorf("remaining = %d, want 1", len(remaining))
	}
}

func TestDefaultRetentionConfig(t *testing.T) {
	rc := DefaultRetentionConfig()
	if rc.CheckResultsRaw != 30*24*time.Hour {
		t.Errorf("CheckResultsRaw = %v, want 720h", rc.CheckResultsRaw)
	}
	if rc.Hourly != 180*24*time.Hour {
		t.Errorf("Hourly = %v, want 4320h", rc.Hourly)
	}
}
