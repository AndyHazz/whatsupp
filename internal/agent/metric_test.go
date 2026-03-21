package agent

import (
	"encoding/json"
	"testing"
	"time"
)

func TestMetric_JSON(t *testing.T) {
	m := Metric{Name: "cpu.usage_pct", Value: 23.5}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	expected := `{"name":"cpu.usage_pct","value":23.5}`
	if string(data) != expected {
		t.Errorf("JSON = %s, want %s", data, expected)
	}
}

func TestMetricBatch_JSON(t *testing.T) {
	batch := MetricBatch{
		Host:      "plexypi",
		Timestamp: time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC),
		Metrics: []Metric{
			{Name: "cpu.usage_pct", Value: 23.5},
			{Name: "mem.usage_pct", Value: 45.2},
		},
	}

	data, err := json.Marshal(batch)
	if err != nil {
		t.Fatal(err)
	}

	// Verify it round-trips
	var decoded MetricBatch
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Host != "plexypi" {
		t.Errorf("Host = %q, want %q", decoded.Host, "plexypi")
	}
	if len(decoded.Metrics) != 2 {
		t.Errorf("len(Metrics) = %d, want 2", len(decoded.Metrics))
	}
}

func TestMetricName_CPU(t *testing.T) {
	if got := CPUMetric("usage_pct"); got != "cpu.usage_pct" {
		t.Errorf("CPUMetric = %q, want %q", got, "cpu.usage_pct")
	}
}

func TestMetricName_Disk(t *testing.T) {
	if got := DiskMetric("/", "usage_pct"); got != "disk./.usage_pct" {
		t.Errorf("DiskMetric = %q, want %q", got, "disk./.usage_pct")
	}
}

func TestMetricName_Net(t *testing.T) {
	if got := NetMetric("eth0", "rx_bytes"); got != "net.eth0.rx_bytes" {
		t.Errorf("NetMetric = %q, want %q", got, "net.eth0.rx_bytes")
	}
}

func TestMetricName_Docker(t *testing.T) {
	if got := DockerMetric("plex", "cpu_pct"); got != "docker.plex.cpu_pct" {
		t.Errorf("DockerMetric = %q, want %q", got, "docker.plex.cpu_pct")
	}
}

func TestMetricName_Temp(t *testing.T) {
	if got := TempMetric("cpu"); got != "temp.cpu" {
		t.Errorf("TempMetric = %q, want %q", got, "temp.cpu")
	}
}

func TestMetricName_Mem(t *testing.T) {
	if got := MemMetric("used_bytes"); got != "mem.used_bytes" {
		t.Errorf("MemMetric = %q, want %q", got, "mem.used_bytes")
	}
}
