package store

import (
	"path/filepath"
	"testing"
	"time"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestInsertAgentMetrics(t *testing.T) {
	s := openTestStore(t)
	ts := time.Now()

	metrics := []Metric{
		{Name: "cpu.usage_pct", Value: 42.5},
		{Name: "mem.usage_pct", Value: 65.0},
	}

	err := s.InsertAgentMetricsBatch("testhost", ts, metrics)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	results, err := s.QueryAgentMetricsRaw("testhost", ts.Add(-time.Minute), ts.Add(time.Minute), nil)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
}

func TestInsertAgentMetrics_BulkInsert(t *testing.T) {
	s := openTestStore(t)
	ts := time.Now()

	metrics := make([]Metric, 50)
	for i := range metrics {
		metrics[i] = Metric{Name: "test.metric", Value: float64(i)}
	}

	err := s.InsertAgentMetricsBatch("testhost", ts, metrics)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	results, err := s.QueryAgentMetricsRaw("testhost", ts.Add(-time.Minute), ts.Add(time.Minute), nil)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(results) != 50 {
		t.Fatalf("got %d results, want 50", len(results))
	}
}

func TestQueryAgentMetrics_TimeRange(t *testing.T) {
	s := openTestStore(t)

	base := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)
	s.InsertAgentMetricsBatch("h", base, []Metric{{Name: "m1", Value: 1}})
	s.InsertAgentMetricsBatch("h", base.Add(time.Hour), []Metric{{Name: "m2", Value: 2}})
	s.InsertAgentMetricsBatch("h", base.Add(2*time.Hour), []Metric{{Name: "m3", Value: 3}})

	// Query only the middle hour
	results, err := s.QueryAgentMetricsRaw("h", base.Add(30*time.Minute), base.Add(90*time.Minute), nil)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].MetricName != "m2" {
		t.Errorf("metric name = %q, want %q", results[0].MetricName, "m2")
	}
}

func TestQueryAgentMetrics_NameFilter(t *testing.T) {
	s := openTestStore(t)
	ts := time.Now()

	metrics := []Metric{
		{Name: "cpu.usage_pct", Value: 42},
		{Name: "cpu.load_1m", Value: 1.5},
		{Name: "mem.usage_pct", Value: 65},
	}
	s.InsertAgentMetricsBatch("h", ts, metrics)

	results, err := s.QueryAgentMetricsRaw("h", ts.Add(-time.Minute), ts.Add(time.Minute), []string{"cpu"})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2 (cpu only)", len(results))
	}
}

func TestUpsertHeartbeat(t *testing.T) {
	s := openTestStore(t)

	ts1 := time.Now().Add(-time.Hour)
	err := s.UpsertHeartbeat("testhost", ts1)
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}

	hb, err := s.GetHeartbeat("testhost")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if hb == nil {
		t.Fatal("heartbeat should exist")
	}
	if hb.LastSeenAt != ts1.Unix() {
		t.Errorf("last_seen_at = %d, want %d", hb.LastSeenAt, ts1.Unix())
	}

	// Update
	ts2 := time.Now()
	err = s.UpsertHeartbeat("testhost", ts2)
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	hb, err = s.GetHeartbeat("testhost")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if hb.LastSeenAt != ts2.Unix() {
		t.Errorf("last_seen_at = %d, want %d", hb.LastSeenAt, ts2.Unix())
	}
}

func TestGetStaleAgents(t *testing.T) {
	s := openTestStore(t)

	now := time.Now()
	s.UpsertHeartbeat("fresh", now)
	s.UpsertHeartbeat("stale", now.Add(-10*time.Minute))

	threshold := now.Add(-5 * time.Minute)
	stale, err := s.GetStaleAgents(threshold)
	if err != nil {
		t.Fatalf("get stale: %v", err)
	}
	if len(stale) != 1 {
		t.Fatalf("got %d stale agents, want 1", len(stale))
	}
	if stale[0].Host != "stale" {
		t.Errorf("stale host = %q, want %q", stale[0].Host, "stale")
	}
}

func TestAggregateAgentMetrics5Min(t *testing.T) {
	s := openTestStore(t)

	base := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)

	// Insert metrics spanning 10 minutes
	for i := 0; i < 10; i++ {
		ts := base.Add(time.Duration(i) * time.Minute)
		s.InsertAgentMetricsBatch("h", ts, []Metric{
			{Name: "cpu.usage_pct", Value: float64(10 + i*10)}, // 10, 20, 30, 40, 50, 60, 70, 80, 90, 100
		})
	}

	// Aggregate
	err := s.AggregateAgentMetrics5Min(base, base.Add(10*time.Minute))
	if err != nil {
		t.Fatalf("aggregate: %v", err)
	}

	results, err := s.QueryAgentMetrics5Min("h", base, base.Add(10*time.Minute), nil)
	if err != nil {
		t.Fatalf("query 5min: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d 5-min buckets, want 2", len(results))
	}
}
