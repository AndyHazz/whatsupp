package agent

import (
	"context"
	"strings"
	"testing"
)

func TestNetCollector_Collect(t *testing.T) {
	c := NewNetCollector()
	metrics, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have at least one non-loopback interface
	if len(metrics) == 0 {
		t.Log("no non-loopback interfaces found (may be expected in containers)")
		return
	}

	// Check metric names have the expected pattern
	for _, m := range metrics {
		if !strings.HasPrefix(m.Name, "net.") {
			t.Errorf("unexpected metric name prefix: %s", m.Name)
		}
	}
}

func TestNetCollector_MetricNames(t *testing.T) {
	c := NewNetCollector()
	metrics, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedSuffixes := []string{"rx_bytes", "tx_bytes", "rx_errors", "tx_errors", "rx_drops", "tx_drops"}
	for _, suffix := range expectedSuffixes {
		found := false
		for _, m := range metrics {
			if strings.HasSuffix(m.Name, "."+suffix) {
				found = true
				break
			}
		}
		if !found && len(metrics) > 0 {
			t.Errorf("missing network metric suffix: %s", suffix)
		}
	}
}

func TestNetCollector_FiltersLoopback(t *testing.T) {
	c := NewNetCollector()
	metrics, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, m := range metrics {
		if strings.HasPrefix(m.Name, "net.lo.") {
			t.Errorf("loopback interface not filtered: %s", m.Name)
		}
	}
}

func TestNetCollector_FiltersVeth(t *testing.T) {
	c := NewNetCollector()
	metrics, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, m := range metrics {
		parts := strings.SplitN(m.Name, ".", 3)
		if len(parts) >= 2 {
			iface := parts[1]
			if strings.HasPrefix(iface, "veth") || strings.HasPrefix(iface, "docker") {
				t.Errorf("veth/docker interface not filtered: %s", m.Name)
			}
		}
	}
}

func TestNetCollector_Rates(t *testing.T) {
	c := NewNetCollector()
	// First call establishes baseline
	c.Collect(context.Background())
	// Second call should produce rate values
	metrics, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	hasRate := false
	for _, m := range metrics {
		if strings.HasSuffix(m.Name, "_bytes_sec") {
			hasRate = true
			break
		}
	}
	if !hasRate && len(metrics) > 0 {
		t.Error("expected rate metrics (_bytes_sec) on second call")
	}
}
