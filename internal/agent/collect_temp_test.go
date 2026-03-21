package agent

import (
	"context"
	"strings"
	"testing"
)

func TestTempCollector_Collect(t *testing.T) {
	c := NewTempCollector()
	metrics, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// On machines with thermal zones, should have at least one metric
	// On VMs/containers, may return empty — that's OK
	for _, m := range metrics {
		if !strings.HasPrefix(m.Name, "temp.") {
			t.Errorf("unexpected metric name: %s", m.Name)
		}
	}
	t.Logf("collected %d temperature metrics", len(metrics))
}

func TestTempCollector_NoSensors(t *testing.T) {
	// Verify that empty result is not an error
	c := NewTempCollector()
	metrics, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// On systems without sensors, should return empty slice (not nil error)
	_ = metrics
}

func TestTempCollector_NameSanitization(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"coretemp_Package id 0", "cpu"},
		{"k10temp_Tctl", "cpu"},
		{"k10temp_Tdie", "cpu"},
		{"nouveau_temp1", "gpu"},
		{"iwlwifi_1", "iwlwifi_1"},
		{"some sensor  name!", "some_sensor_name"},
	}

	for _, tc := range tests {
		got := sanitizeSensorName(tc.input)
		if got != tc.expected {
			t.Errorf("sanitizeSensorName(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}
