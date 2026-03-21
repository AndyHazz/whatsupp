package checks

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const sampleNodeExporterOutput = `# HELP node_cpu_seconds_total Seconds the CPUs spent in each mode.
# TYPE node_cpu_seconds_total counter
node_cpu_seconds_total{cpu="0",mode="idle"} 12345.67
node_cpu_seconds_total{cpu="0",mode="system"} 234.56
node_cpu_seconds_total{cpu="0",mode="user"} 567.89
# HELP node_load1 1m load average.
# TYPE node_load1 gauge
node_load1 1.5
# HELP node_load5 5m load average.
# TYPE node_load5 gauge
node_load5 1.2
# HELP node_load15 15m load average.
# TYPE node_load15 gauge
node_load15 0.9
# HELP node_memory_MemTotal_bytes Machine memory size in bytes.
# TYPE node_memory_MemTotal_bytes gauge
node_memory_MemTotal_bytes 8589934592
# HELP node_memory_MemAvailable_bytes Memory information field MemAvailable_bytes.
# TYPE node_memory_MemAvailable_bytes gauge
node_memory_MemAvailable_bytes 4294967296
# HELP node_memory_MemFree_bytes Memory information field MemFree_bytes.
# TYPE node_memory_MemFree_bytes gauge
node_memory_MemFree_bytes 2147483648
# HELP node_memory_SwapTotal_bytes Memory information field SwapTotal_bytes.
# TYPE node_memory_SwapTotal_bytes gauge
node_memory_SwapTotal_bytes 2147483648
# HELP node_memory_SwapFree_bytes Memory information field SwapFree_bytes.
# TYPE node_memory_SwapFree_bytes gauge
node_memory_SwapFree_bytes 1073741824
# HELP node_filesystem_size_bytes Filesystem size in bytes.
# TYPE node_filesystem_size_bytes gauge
node_filesystem_size_bytes{device="/dev/sda1",mountpoint="/",fstype="ext4"} 107374182400
node_filesystem_size_bytes{device="tmpfs",mountpoint="/dev/shm",fstype="tmpfs"} 4294967296
# HELP node_filesystem_avail_bytes Filesystem space available to non-root users in bytes.
# TYPE node_filesystem_avail_bytes gauge
node_filesystem_avail_bytes{device="/dev/sda1",mountpoint="/",fstype="ext4"} 53687091200
node_filesystem_avail_bytes{device="tmpfs",mountpoint="/dev/shm",fstype="tmpfs"} 4294967296
# HELP node_network_receive_bytes_total Network device statistic receive_bytes.
# TYPE node_network_receive_bytes_total counter
node_network_receive_bytes_total{device="eth0"} 1234567890
node_network_receive_bytes_total{device="lo"} 999999
# HELP node_network_transmit_bytes_total Network device statistic transmit_bytes.
# TYPE node_network_transmit_bytes_total counter
node_network_transmit_bytes_total{device="eth0"} 987654321
node_network_transmit_bytes_total{device="lo"} 888888
# HELP node_network_receive_errs_total Network device statistic receive_errs.
# TYPE node_network_receive_errs_total counter
node_network_receive_errs_total{device="eth0"} 5
# HELP node_network_transmit_errs_total Network device statistic transmit_errs.
# TYPE node_network_transmit_errs_total counter
node_network_transmit_errs_total{device="eth0"} 3
# HELP node_hwmon_temp_celsius Hardware monitor for temperature.
# TYPE node_hwmon_temp_celsius gauge
node_hwmon_temp_celsius{chip="coretemp",sensor="temp1"} 45.5
# HELP node_scrape_collector_duration_seconds Duration of scrapes.
# TYPE node_scrape_collector_duration_seconds gauge
node_scrape_collector_duration_seconds{collector="cpu"} 0.001
`

func TestParsePrometheusText_Counter(t *testing.T) {
	input := `# TYPE node_cpu_seconds_total counter
node_cpu_seconds_total{cpu="0",mode="idle"} 12345.67
`
	metrics, err := ParsePrometheusText(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(metrics) != 1 {
		t.Fatalf("got %d metrics, want 1", len(metrics))
	}
	if metrics[0].Name != "node_cpu_seconds_total" {
		t.Errorf("name = %q, want node_cpu_seconds_total", metrics[0].Name)
	}
	if metrics[0].Value != 12345.67 {
		t.Errorf("value = %f, want 12345.67", metrics[0].Value)
	}
	if metrics[0].Labels["cpu"] != "0" {
		t.Errorf("label cpu = %q, want 0", metrics[0].Labels["cpu"])
	}
}

func TestParsePrometheusText_Gauge(t *testing.T) {
	input := `# TYPE node_memory_MemAvailable_bytes gauge
node_memory_MemAvailable_bytes 1073741824
`
	metrics, err := ParsePrometheusText(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(metrics) != 1 {
		t.Fatalf("got %d metrics, want 1", len(metrics))
	}
	if metrics[0].Value != 1073741824 {
		t.Errorf("value = %f, want 1073741824", metrics[0].Value)
	}
}

func TestParsePrometheusText_MultipleLines(t *testing.T) {
	metrics, err := ParsePrometheusText(strings.NewReader(sampleNodeExporterOutput))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(metrics) == 0 {
		t.Fatal("expected non-empty metrics")
	}

	// Check some expected metrics exist
	names := make(map[string]bool)
	for _, m := range metrics {
		names[m.Name] = true
	}
	expected := []string{"node_cpu_seconds_total", "node_load1", "node_memory_MemTotal_bytes", "node_filesystem_size_bytes"}
	for _, e := range expected {
		if !names[e] {
			t.Errorf("missing metric: %s", e)
		}
	}
}

func TestParsePrometheusText_Empty(t *testing.T) {
	metrics, err := ParsePrometheusText(strings.NewReader(""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(metrics) != 0 {
		t.Errorf("got %d metrics, want 0", len(metrics))
	}
}

func TestParsePrometheusText_Malformed(t *testing.T) {
	input := `not valid prometheus format
# TYPE good_metric gauge
good_metric 42.0
`
	metrics, err := ParsePrometheusText(strings.NewReader(input))
	// Parser may return error, partial results, or both - all are acceptable
	if err != nil {
		t.Logf("parser returned error for malformed input (acceptable): %v", err)
	}
	if len(metrics) > 0 {
		t.Logf("parser returned %d metrics despite malformed input", len(metrics))
	}
	// The main requirement: should not panic
}

func TestScrapeCheck_Execute(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(sampleNodeExporterOutput))
	}))
	defer srv.Close()

	sc := NewScrapeCheck("test", srv.URL)
	metrics, err := sc.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}

	if len(metrics) == 0 {
		t.Fatal("expected non-empty metrics")
	}

	// Check for mapped metric names
	names := make(map[string]bool)
	for _, m := range metrics {
		names[m.Name] = true
	}

	expected := []string{"cpu.load_1m", "mem.total_bytes", "mem.available_bytes"}
	for _, e := range expected {
		if !names[e] {
			t.Errorf("missing mapped metric: %s (got: %v)", e, nameList(metrics))
		}
	}
}

func TestScrapeCheck_Timeout(t *testing.T) {
	done := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-done:
		case <-r.Context().Done():
		}
	}))
	defer func() {
		close(done)
		srv.Close()
	}()

	sc := NewScrapeCheck("test", srv.URL)
	sc.client.Timeout = 100 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_, err := sc.Execute(ctx)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestScrapeCheck_BadStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	sc := NewScrapeCheck("test", srv.URL)
	_, err := sc.Execute(context.Background())
	if err == nil {
		t.Fatal("expected error for 500 status")
	}
}

func nameList(metrics []ScrapeMetric) []string {
	names := make([]string, len(metrics))
	for i, m := range metrics {
		names[i] = m.Name
	}
	return names
}
