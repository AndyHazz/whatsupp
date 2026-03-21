//go:build integration

package checks

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestScrape_EndToEnd(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(sampleNodeExporterOutput))
	}))
	defer srv.Close()

	sc := NewScrapeCheck("test-node", srv.URL)
	metrics, err := sc.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}

	if len(metrics) == 0 {
		t.Fatal("expected non-empty metrics")
	}

	names := make(map[string]bool)
	for _, m := range metrics {
		names[m.Name] = true
	}

	// Verify correct metric names
	expected := []string{
		"cpu.load_1m",
		"mem.total_bytes",
		"mem.available_bytes",
		"mem.used_bytes",
		"mem.usage_pct",
	}
	for _, e := range expected {
		if !names[e] {
			t.Errorf("missing metric: %s", e)
		}
	}

	// Should have network metrics for eth0
	hasEth0 := false
	for name := range names {
		if strings.HasPrefix(name, "net.eth0.") {
			hasEth0 = true
			break
		}
	}
	if !hasEth0 {
		t.Error("missing eth0 network metrics")
	}

	// Should not have lo metrics
	for name := range names {
		if strings.Contains(name, ".lo.") {
			t.Errorf("should not have loopback metrics: %s", name)
		}
	}

	// Should have temperature
	if !names["temp.cpu"] {
		t.Error("missing temp.cpu")
	}

	t.Logf("collected %d metrics", len(metrics))
}
