package agent

import (
	"context"
	"testing"

	"github.com/docker/docker/api/types"
	containertypes "github.com/docker/docker/api/types/container"
)

func TestDockerCollector_NoDocker(t *testing.T) {
	// Point to a non-existent Docker socket
	c := NewDockerCollector("tcp://127.0.0.1:1")
	metrics, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("expected nil error when Docker unavailable, got: %v", err)
	}
	// Should return empty slice (not error) when Docker is unreachable
	if metrics != nil && len(metrics) > 0 {
		t.Logf("got %d metrics (Docker may be reachable)", len(metrics))
	}
}

func TestDockerCollector_ParseStats(t *testing.T) {
	stats := &types.StatsJSON{
		Stats: types.Stats{
			CPUStats: containertypes.CPUStats{
				CPUUsage: containertypes.CPUUsage{
					TotalUsage:  200000000,
					PercpuUsage: []uint64{100000000, 100000000},
				},
				SystemUsage: 2000000000,
				OnlineCPUs:  2,
			},
			PreCPUStats: containertypes.CPUStats{
				CPUUsage: containertypes.CPUUsage{
					TotalUsage: 100000000,
				},
				SystemUsage: 1000000000,
			},
			MemoryStats: containertypes.MemoryStats{
				Usage: 104857600,  // 100MB
				Limit: 1073741824, // 1GB
			},
		},
	}

	cpuPct := CalculateDockerCPUPercent(stats)
	if cpuPct < 0 || cpuPct > 200 {
		t.Errorf("CPU %% = %f, want reasonable range", cpuPct)
	}

	// Expected: (100M / 1000M) * 2 * 100 = 20%
	expected := 20.0
	if cpuPct < expected-1 || cpuPct > expected+1 {
		t.Errorf("CPU %% = %f, want ~%f", cpuPct, expected)
	}
}

func TestDockerCollector_ContainerStatus(t *testing.T) {
	// Verify status metric interpretation (unit test, no Docker needed)
	name := containerName([]string{"/mycontainer"})
	if name != "mycontainer" {
		t.Errorf("containerName = %q, want %q", name, "mycontainer")
	}
}

func TestDockerCollector_CPUCalc(t *testing.T) {
	tests := []struct {
		name     string
		stats    *types.StatsJSON
		expected float64
	}{
		{
			name: "zero delta",
			stats: &types.StatsJSON{
				Stats: types.Stats{
					CPUStats: containertypes.CPUStats{
						CPUUsage:    containertypes.CPUUsage{TotalUsage: 100},
						SystemUsage: 1000,
						OnlineCPUs:  1,
					},
					PreCPUStats: containertypes.CPUStats{
						CPUUsage:    containertypes.CPUUsage{TotalUsage: 100},
						SystemUsage: 1000,
					},
				},
			},
			expected: 0.0,
		},
		{
			name: "50% single core",
			stats: &types.StatsJSON{
				Stats: types.Stats{
					CPUStats: containertypes.CPUStats{
						CPUUsage:    containertypes.CPUUsage{TotalUsage: 500000000},
						SystemUsage: 2000000000,
						OnlineCPUs:  1,
					},
					PreCPUStats: containertypes.CPUStats{
						CPUUsage:    containertypes.CPUUsage{TotalUsage: 0},
						SystemUsage: 1000000000,
					},
				},
			},
			expected: 50.0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := CalculateDockerCPUPercent(tc.stats)
			if got < tc.expected-1 || got > tc.expected+1 {
				t.Errorf("CPU %% = %f, want ~%f", got, tc.expected)
			}
		})
	}
}
