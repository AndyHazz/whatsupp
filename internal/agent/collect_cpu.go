package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/load"
)

// CPUCollector collects CPU usage metrics.
type CPUCollector struct {
	firstPerCore bool
}

// NewCPUCollector creates a new CPU collector.
func NewCPUCollector() *CPUCollector {
	return &CPUCollector{firstPerCore: true}
}

func (c *CPUCollector) Name() string { return "cpu" }

func (c *CPUCollector) Collect(ctx context.Context) ([]Metric, error) {
	var metrics []Metric

	// Overall CPU usage (1s sample)
	pcts, err := cpu.PercentWithContext(ctx, 1*time.Second, false)
	if err == nil && len(pcts) > 0 {
		metrics = append(metrics, Metric{Name: CPUMetric("usage_pct"), Value: pcts[0]})
	}

	// Load averages
	avg, err := load.AvgWithContext(ctx)
	if err == nil {
		metrics = append(metrics, Metric{Name: CPUMetric("load_1m"), Value: avg.Load1})
		metrics = append(metrics, Metric{Name: CPUMetric("load_5m"), Value: avg.Load5})
		metrics = append(metrics, Metric{Name: CPUMetric("load_15m"), Value: avg.Load15})
	}

	// Per-core usage (skip first call as delta is zero)
	corePcts, err := cpu.PercentWithContext(ctx, 0, true)
	if err == nil {
		if c.firstPerCore {
			c.firstPerCore = false
		} else {
			for i, pct := range corePcts {
				metrics = append(metrics, Metric{
					Name:  CPUMetric(fmt.Sprintf("core%d_pct", i)),
					Value: pct,
				})
			}
		}
	}

	return metrics, nil
}
