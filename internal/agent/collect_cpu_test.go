package agent

import (
	"context"
	"strings"
	"testing"
)

func TestCPUCollector_Collect(t *testing.T) {
	c := NewCPUCollector()
	metrics, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	names := metricNames(metrics)
	required := []string{"cpu.usage_pct", "cpu.load_1m", "cpu.load_5m", "cpu.load_15m"}
	for _, name := range required {
		if !containsName(names, name) {
			t.Errorf("missing metric: %s (got: %v)", name, names)
		}
	}
}

func TestCPUCollector_PerCore(t *testing.T) {
	c := NewCPUCollector()
	// First call initializes per-core baseline
	c.Collect(context.Background())
	// Second call should produce per-core metrics
	metrics, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	hasCore := false
	for _, m := range metrics {
		if strings.HasPrefix(m.Name, "cpu.core") && strings.HasSuffix(m.Name, "_pct") {
			hasCore = true
			break
		}
	}
	if !hasCore {
		t.Error("expected per-core metrics on second call")
	}
}

func TestCPUCollector_ValueRanges(t *testing.T) {
	c := NewCPUCollector()
	metrics, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, m := range metrics {
		if m.Name == "cpu.usage_pct" {
			if m.Value < 0 || m.Value > 100 {
				t.Errorf("cpu.usage_pct = %f, want 0-100", m.Value)
			}
		}
		if strings.HasPrefix(m.Name, "cpu.load_") {
			if m.Value < 0 {
				t.Errorf("%s = %f, want >= 0", m.Name, m.Value)
			}
		}
	}
}

// helpers
func metricNames(metrics []Metric) []string {
	names := make([]string, len(metrics))
	for i, m := range metrics {
		names[i] = m.Name
	}
	return names
}

func containsName(names []string, target string) bool {
	for _, n := range names {
		if n == target {
			return true
		}
	}
	return false
}
