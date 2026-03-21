package agent

import (
	"context"

	"github.com/shirou/gopsutil/v4/mem"
)

// MemCollector collects memory usage metrics.
type MemCollector struct{}

func NewMemCollector() *MemCollector { return &MemCollector{} }

func (c *MemCollector) Name() string { return "mem" }

func (c *MemCollector) Collect(ctx context.Context) ([]Metric, error) {
	var metrics []Metric

	v, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return nil, err
	}

	metrics = append(metrics,
		Metric{Name: MemMetric("used_bytes"), Value: float64(v.Used)},
		Metric{Name: MemMetric("available_bytes"), Value: float64(v.Available)},
		Metric{Name: MemMetric("total_bytes"), Value: float64(v.Total)},
		Metric{Name: MemMetric("usage_pct"), Value: v.UsedPercent},
	)

	sw, err := mem.SwapMemoryWithContext(ctx)
	if err == nil {
		metrics = append(metrics,
			Metric{Name: MemMetric("swap_used_bytes"), Value: float64(sw.Used)},
			Metric{Name: MemMetric("swap_total_bytes"), Value: float64(sw.Total)},
		)
	}

	return metrics, nil
}
