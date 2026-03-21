package agent

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/net"
)

// NetCollector collects network interface metrics.
type NetCollector struct {
	mu       sync.Mutex
	prevIO   map[string]net.IOCountersStat
	prevTime time.Time
}

func NewNetCollector() *NetCollector {
	return &NetCollector{}
}

func (c *NetCollector) Name() string { return "net" }

func (c *NetCollector) Collect(ctx context.Context) ([]Metric, error) {
	counters, err := net.IOCountersWithContext(ctx, true)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	var metrics []Metric

	c.mu.Lock()
	defer c.mu.Unlock()

	// Build current map
	current := make(map[string]net.IOCountersStat)
	for _, ioc := range counters {
		if isFilteredInterface(ioc.Name) {
			continue
		}
		current[ioc.Name] = ioc

		// Cumulative counters
		metrics = append(metrics,
			Metric{Name: NetMetric(ioc.Name, "rx_bytes"), Value: float64(ioc.BytesRecv)},
			Metric{Name: NetMetric(ioc.Name, "tx_bytes"), Value: float64(ioc.BytesSent)},
			Metric{Name: NetMetric(ioc.Name, "rx_errors"), Value: float64(ioc.Errin)},
			Metric{Name: NetMetric(ioc.Name, "tx_errors"), Value: float64(ioc.Errout)},
			Metric{Name: NetMetric(ioc.Name, "rx_drops"), Value: float64(ioc.Dropin)},
			Metric{Name: NetMetric(ioc.Name, "tx_drops"), Value: float64(ioc.Dropout)},
		)

		// Rate metrics (delta from previous)
		if c.prevIO != nil && !c.prevTime.IsZero() {
			elapsed := now.Sub(c.prevTime).Seconds()
			if prev, ok := c.prevIO[ioc.Name]; ok && elapsed > 0 {
				metrics = append(metrics,
					Metric{Name: NetMetric(ioc.Name, "rx_bytes_sec"), Value: float64(ioc.BytesRecv-prev.BytesRecv) / elapsed},
					Metric{Name: NetMetric(ioc.Name, "tx_bytes_sec"), Value: float64(ioc.BytesSent-prev.BytesSent) / elapsed},
				)
			}
		}
	}

	c.prevIO = current
	c.prevTime = now

	return metrics, nil
}

func isFilteredInterface(name string) bool {
	if name == "lo" {
		return true
	}
	if strings.HasPrefix(name, "veth") {
		return true
	}
	if strings.HasPrefix(name, "docker") {
		return true
	}
	if strings.HasPrefix(name, "br-") {
		return true
	}
	return false
}
