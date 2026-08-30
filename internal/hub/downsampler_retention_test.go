package hub

import (
	"testing"
	"time"

	"github.com/andyhazz/whatsupp/internal/store"
)

// The 90-day hourly retention was only ever applied to check_results_hourly.
// agent_metrics_hourly grew without bound, which is what took the live DB to
// 870MB. This covers the missing call, not just the store method.
func TestDownsampler_DailyAggregation_PrunesOldHourlyAgentMetrics(t *testing.T) {
	s := testStore(t)
	d := NewDownsampler(s)

	now := time.Now()
	stale := now.Add(-100 * 24 * time.Hour).Truncate(time.Hour)

	if err := s.InsertAgentMetricsBatch("plexypi", stale, []store.Metric{
		{Name: "cpu_percent", Value: 50},
	}); err != nil {
		t.Fatalf("InsertAgentMetricsBatch() error: %v", err)
	}
	if err := s.AggregateAgentMetrics5Min(stale, stale.Add(time.Hour)); err != nil {
		t.Fatalf("AggregateAgentMetrics5Min() error: %v", err)
	}
	if err := s.AggregateAgentMetricsHourly(stale.Unix(), stale.Add(time.Hour).Unix()); err != nil {
		t.Fatalf("AggregateAgentMetricsHourly() error: %v", err)
	}

	before, err := s.QueryAgentMetricsHourly("plexypi", stale.Add(-time.Hour), stale.Add(time.Hour), nil)
	if err != nil {
		t.Fatalf("QueryAgentMetricsHourly() error: %v", err)
	}
	if len(before) == 0 {
		t.Fatalf("test setup did not create an hourly row to prune")
	}

	d.runDailyAggregation()

	after, err := s.QueryAgentMetricsHourly("plexypi", stale.Add(-time.Hour), stale.Add(time.Hour), nil)
	if err != nil {
		t.Fatalf("QueryAgentMetricsHourly() error: %v", err)
	}
	if len(after) != 0 {
		t.Errorf("hourly rows 100 days old = %d, want 0 (retention is 90d)", len(after))
	}
}
