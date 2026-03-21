package checks

import (
	"strings"
	"testing"
	"time"
)

func TestMapCPU(t *testing.T) {
	mapper := NewNodeExporterMapper()
	metrics := []PrometheusMetric{
		{Name: "node_cpu_seconds_total", Labels: map[string]string{"cpu": "0", "mode": "idle"}, Value: 12345},
		{Name: "node_cpu_seconds_total", Labels: map[string]string{"cpu": "0", "mode": "user"}, Value: 567},
		{Name: "node_cpu_seconds_total", Labels: map[string]string{"cpu": "0", "mode": "system"}, Value: 234},
	}

	// First call establishes baseline
	mapper.Map(metrics)

	// Second call with different values should produce cpu.usage_pct
	metrics2 := []PrometheusMetric{
		{Name: "node_cpu_seconds_total", Labels: map[string]string{"cpu": "0", "mode": "idle"}, Value: 12445},
		{Name: "node_cpu_seconds_total", Labels: map[string]string{"cpu": "0", "mode": "user"}, Value: 617},
		{Name: "node_cpu_seconds_total", Labels: map[string]string{"cpu": "0", "mode": "system"}, Value: 284},
	}

	result := mapper.Map(metrics2)
	found := false
	for _, m := range result {
		if m.Name == "cpu.usage_pct" {
			found = true
			if m.Value < 0 || m.Value > 100 {
				t.Errorf("cpu.usage_pct = %f, want 0-100", m.Value)
			}
		}
	}
	if !found {
		t.Error("expected cpu.usage_pct in second call")
	}
}

func TestMapMemory(t *testing.T) {
	mapper := NewNodeExporterMapper()
	metrics := []PrometheusMetric{
		{Name: "node_memory_MemTotal_bytes", Labels: map[string]string{}, Value: 8589934592},
		{Name: "node_memory_MemAvailable_bytes", Labels: map[string]string{}, Value: 4294967296},
	}

	result := mapper.Map(metrics)
	names := mappedNames(result)

	if !containsMapped(names, "mem.total_bytes") {
		t.Error("missing mem.total_bytes")
	}
	if !containsMapped(names, "mem.available_bytes") {
		t.Error("missing mem.available_bytes")
	}
	if !containsMapped(names, "mem.used_bytes") {
		t.Error("missing mem.used_bytes")
	}
	if !containsMapped(names, "mem.usage_pct") {
		t.Error("missing mem.usage_pct")
	}
}

func TestMapDisk(t *testing.T) {
	mapper := NewNodeExporterMapper()
	metrics := []PrometheusMetric{
		{Name: "node_filesystem_avail_bytes", Labels: map[string]string{"mountpoint": "/"}, Value: 53687091200},
		{Name: "node_filesystem_size_bytes", Labels: map[string]string{"mountpoint": "/"}, Value: 107374182400},
	}

	result := mapper.Map(metrics)
	names := mappedNames(result)
	if !containsMapped(names, "disk./.avail_bytes") {
		t.Error("missing disk./.avail_bytes")
	}
	if !containsMapped(names, "disk./.total_bytes") {
		t.Error("missing disk./.total_bytes")
	}
}

func TestMapDisk_FiltersMounts(t *testing.T) {
	mapper := NewNodeExporterMapper()
	metrics := []PrometheusMetric{
		{Name: "node_filesystem_size_bytes", Labels: map[string]string{"mountpoint": "/sys"}, Value: 100},
		{Name: "node_filesystem_size_bytes", Labels: map[string]string{"mountpoint": "/proc"}, Value: 100},
		{Name: "node_filesystem_size_bytes", Labels: map[string]string{"mountpoint": "/dev"}, Value: 100},
		{Name: "node_filesystem_size_bytes", Labels: map[string]string{"mountpoint": "/"}, Value: 100},
	}

	result := mapper.Map(metrics)
	for _, m := range result {
		if strings.Contains(m.Name, "/sys") || strings.Contains(m.Name, "/proc") || strings.Contains(m.Name, "/dev.") {
			t.Errorf("filtered mount not excluded: %s", m.Name)
		}
	}
}

func TestMapNetwork(t *testing.T) {
	mapper := NewNodeExporterMapper()
	metrics := []PrometheusMetric{
		{Name: "node_network_receive_bytes_total", Labels: map[string]string{"device": "eth0"}, Value: 1234567890},
		{Name: "node_network_transmit_bytes_total", Labels: map[string]string{"device": "eth0"}, Value: 987654321},
	}

	result := mapper.Map(metrics)
	names := mappedNames(result)
	if !containsMapped(names, "net.eth0.rx_bytes") {
		t.Error("missing net.eth0.rx_bytes")
	}
	if !containsMapped(names, "net.eth0.tx_bytes") {
		t.Error("missing net.eth0.tx_bytes")
	}
}

func TestMapNetwork_FiltersInterfaces(t *testing.T) {
	mapper := NewNodeExporterMapper()
	metrics := []PrometheusMetric{
		{Name: "node_network_receive_bytes_total", Labels: map[string]string{"device": "lo"}, Value: 999},
		{Name: "node_network_receive_bytes_total", Labels: map[string]string{"device": "veth123"}, Value: 999},
		{Name: "node_network_receive_bytes_total", Labels: map[string]string{"device": "docker0"}, Value: 999},
		{Name: "node_network_receive_bytes_total", Labels: map[string]string{"device": "eth0"}, Value: 999},
	}

	result := mapper.Map(metrics)
	for _, m := range result {
		if strings.Contains(m.Name, "lo.") || strings.Contains(m.Name, "veth") || strings.Contains(m.Name, "docker") {
			t.Errorf("filtered interface not excluded: %s", m.Name)
		}
	}
}

func TestMapTemperature(t *testing.T) {
	mapper := NewNodeExporterMapper()
	metrics := []PrometheusMetric{
		{Name: "node_hwmon_temp_celsius", Labels: map[string]string{"chip": "coretemp", "sensor": "temp1"}, Value: 45.5},
	}

	result := mapper.Map(metrics)
	found := false
	for _, m := range result {
		if m.Name == "temp.cpu" {
			found = true
			if m.Value != 45.5 {
				t.Errorf("temp.cpu = %f, want 45.5", m.Value)
			}
		}
	}
	if !found {
		t.Error("missing temp.cpu")
	}
}

func TestMapUnknown_Ignored(t *testing.T) {
	mapper := NewNodeExporterMapper()
	metrics := []PrometheusMetric{
		{Name: "node_scrape_collector_duration_seconds", Labels: map[string]string{}, Value: 0.001},
	}

	result := mapper.Map(metrics)
	if len(result) != 0 {
		t.Errorf("unmapped metric should produce no output, got %d", len(result))
	}
}

func TestMapCPURate(t *testing.T) {
	mapper := NewNodeExporterMapper()

	// First scrape
	metrics1 := []PrometheusMetric{
		{Name: "node_cpu_seconds_total", Labels: map[string]string{"cpu": "0", "mode": "idle"}, Value: 1000},
		{Name: "node_cpu_seconds_total", Labels: map[string]string{"cpu": "0", "mode": "user"}, Value: 100},
		{Name: "node_cpu_seconds_total", Labels: map[string]string{"cpu": "0", "mode": "system"}, Value: 50},
	}
	mapper.Map(metrics1)

	// Simulate time passing
	mapper.prevTS = time.Now().Add(-10 * time.Second)

	// Second scrape: idle increased by 5, user by 3, system by 2 = total 10
	metrics2 := []PrometheusMetric{
		{Name: "node_cpu_seconds_total", Labels: map[string]string{"cpu": "0", "mode": "idle"}, Value: 1005},
		{Name: "node_cpu_seconds_total", Labels: map[string]string{"cpu": "0", "mode": "user"}, Value: 103},
		{Name: "node_cpu_seconds_total", Labels: map[string]string{"cpu": "0", "mode": "system"}, Value: 52},
	}

	result := mapper.Map(metrics2)
	for _, m := range result {
		if m.Name == "cpu.usage_pct" {
			// idle delta = 5, total delta = 10, usage = (1 - 5/10) * 100 = 50%
			if m.Value < 49 || m.Value > 51 {
				t.Errorf("cpu.usage_pct = %f, want ~50%%", m.Value)
			}
			return
		}
	}
	t.Error("missing cpu.usage_pct in second scrape")
}

func mappedNames(metrics []MappedMetric) []string {
	names := make([]string, len(metrics))
	for i, m := range metrics {
		names[i] = m.Name
	}
	return names
}

func containsMapped(names []string, target string) bool {
	for _, n := range names {
		if n == target {
			return true
		}
	}
	return false
}
