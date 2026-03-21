package agent

import (
	"context"
	"testing"
)

func TestMemCollector_Collect(t *testing.T) {
	c := NewMemCollector()
	metrics, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	names := metricNames(metrics)
	required := []string{
		"mem.used_bytes", "mem.available_bytes", "mem.total_bytes",
		"mem.usage_pct", "mem.swap_used_bytes", "mem.swap_total_bytes",
	}
	for _, name := range required {
		if !containsName(names, name) {
			t.Errorf("missing metric: %s (got: %v)", name, names)
		}
	}
}

func TestMemCollector_ValueRanges(t *testing.T) {
	c := NewMemCollector()
	metrics, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, m := range metrics {
		if m.Name == "mem.usage_pct" {
			if m.Value < 0 || m.Value > 100 {
				t.Errorf("mem.usage_pct = %f, want 0-100", m.Value)
			}
		}
		if m.Name == "mem.total_bytes" {
			if m.Value <= 0 {
				t.Errorf("mem.total_bytes = %f, want > 0", m.Value)
			}
		}
	}
}

func TestMemCollector_SwapZero(t *testing.T) {
	// This test verifies that swap values are reported (even if 0) without error.
	c := NewMemCollector()
	metrics, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, m := range metrics {
		if m.Name == "mem.swap_used_bytes" || m.Name == "mem.swap_total_bytes" {
			if m.Value < 0 {
				t.Errorf("%s = %f, want >= 0", m.Name, m.Value)
			}
		}
	}
}
