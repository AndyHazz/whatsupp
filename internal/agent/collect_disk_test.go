package agent

import (
	"context"
	"strings"
	"testing"
)

func TestDiskCollector_Collect(t *testing.T) {
	c := NewDiskCollector()
	metrics, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have at least root partition metrics
	hasRoot := false
	for _, m := range metrics {
		if strings.HasPrefix(m.Name, "disk./.") {
			hasRoot = true
			break
		}
	}
	if !hasRoot {
		t.Error("expected at least root disk metrics (disk./.)")
	}

	// Check expected metric suffixes
	suffixes := []string{"usage_pct", "total_bytes", "used_bytes", "avail_bytes"}
	for _, suffix := range suffixes {
		found := false
		for _, m := range metrics {
			if strings.HasSuffix(m.Name, "."+suffix) && strings.HasPrefix(m.Name, "disk./") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing disk metric suffix: %s", suffix)
		}
	}
}

func TestDiskCollector_IOPS(t *testing.T) {
	c := NewDiskCollector()
	// First call establishes baseline
	c.Collect(context.Background())
	// Second call should produce delta IOPS
	metrics, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	hasIOPS := false
	for _, m := range metrics {
		if strings.Contains(m.Name, "read_iops") || strings.Contains(m.Name, "write_iops") {
			hasIOPS = true
			break
		}
	}
	if !hasIOPS {
		t.Log("no IOPS metrics on second call (may be expected in some environments)")
	}
}

func TestDiskCollector_FiltersVirtual(t *testing.T) {
	c := NewDiskCollector()
	metrics, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, m := range metrics {
		if strings.HasPrefix(m.Name, "disk./proc.") ||
			strings.HasPrefix(m.Name, "disk./sys.") ||
			strings.HasPrefix(m.Name, "disk./dev.") {
			t.Errorf("virtual filesystem not filtered: %s", m.Name)
		}
	}
}

func TestDiskCollector_MountNames(t *testing.T) {
	c := NewDiskCollector()
	metrics, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, m := range metrics {
		if !strings.HasPrefix(m.Name, "disk.") {
			continue
		}
		// Verify mount point is part of the name
		parts := strings.SplitN(m.Name, ".", 3)
		if len(parts) < 3 {
			t.Errorf("unexpected disk metric format: %s", m.Name)
		}
	}
}
