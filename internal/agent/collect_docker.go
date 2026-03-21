package agent

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

type dockerIOSnapshot struct {
	netRx     uint64
	netTx     uint64
	blkRead   uint64
	blkWrite  uint64
}

// DockerCollector collects Docker container metrics.
type DockerCollector struct {
	dockerHost string
	mu         sync.Mutex
	prevIO     map[string]dockerIOSnapshot
	prevTime   time.Time
}

func NewDockerCollector(dockerHost string) *DockerCollector {
	return &DockerCollector{dockerHost: dockerHost}
}

func (c *DockerCollector) Name() string { return "docker" }

func (c *DockerCollector) Collect(ctx context.Context) ([]Metric, error) {
	opts := []client.Opt{client.FromEnv, client.WithAPIVersionNegotiation()}
	if c.dockerHost != "" {
		opts = append(opts, client.WithHost(c.dockerHost))
	}

	cli, err := client.NewClientWithOpts(opts...)
	if err != nil {
		log.Printf("docker collector: cannot create client: %v", err)
		return nil, nil // non-fatal
	}
	defer cli.Close()

	containers, err := cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		log.Printf("docker collector: cannot list containers: %v", err)
		return nil, nil // non-fatal
	}

	now := time.Now()
	currentIO := make(map[string]dockerIOSnapshot)

	c.mu.Lock()
	defer c.mu.Unlock()

	var metrics []Metric
	for _, ctr := range containers {
		name := containerName(ctr.Names)
		state := ctr.State

		// Status metric: 1 for running, 0 for stopped/exited
		statusVal := 0.0
		if state == "running" {
			statusVal = 1.0
		}
		metrics = append(metrics, Metric{
			Name:  DockerMetric(name, "status"),
			Value: statusVal,
		})

		// Only get stats for running containers
		if state != "running" {
			continue
		}

		stats, err := cli.ContainerStats(ctx, ctr.ID, false)
		if err != nil {
			log.Printf("docker collector: stats for %s: %v", name, err)
			continue
		}

		var statsJSON types.StatsJSON
		data, err := io.ReadAll(io.LimitReader(stats.Body, 1<<20)) // 1MB limit
		stats.Body.Close()
		if err != nil {
			continue
		}
		if err := json.Unmarshal(data, &statsJSON); err != nil {
			continue
		}

		// CPU & memory (existing)
		cpuPct := CalculateDockerCPUPercent(&statsJSON)
		memUsage := float64(statsJSON.MemoryStats.Usage)
		memLimit := float64(statsJSON.MemoryStats.Limit)
		memPct := 0.0
		if memLimit > 0 {
			memPct = (memUsage / memLimit) * 100.0
		}

		metrics = append(metrics,
			Metric{Name: DockerMetric(name, "cpu_pct"), Value: cpuPct},
			Metric{Name: DockerMetric(name, "mem_bytes"), Value: memUsage},
			Metric{Name: DockerMetric(name, "mem_limit_bytes"), Value: memLimit},
			Metric{Name: DockerMetric(name, "mem_usage_pct"), Value: memPct},
		)

		// Network I/O — sum across all container interfaces
		var netRx, netTx uint64
		for _, iface := range statsJSON.Networks {
			netRx += iface.RxBytes
			netTx += iface.TxBytes
		}
		metrics = append(metrics,
			Metric{Name: DockerMetric(name, "net_rx_bytes"), Value: float64(netRx)},
			Metric{Name: DockerMetric(name, "net_tx_bytes"), Value: float64(netTx)},
		)

		// Block I/O — sum read/write across all devices
		var blkRead, blkWrite uint64
		for _, entry := range statsJSON.BlkioStats.IoServiceBytesRecursive {
			switch strings.ToLower(entry.Op) {
			case "read":
				blkRead += entry.Value
			case "write":
				blkWrite += entry.Value
			}
		}
		metrics = append(metrics,
			Metric{Name: DockerMetric(name, "disk_read_bytes"), Value: float64(blkRead)},
			Metric{Name: DockerMetric(name, "disk_write_bytes"), Value: float64(blkWrite)},
		)

		// Store snapshot for rate calculation
		snap := dockerIOSnapshot{
			netRx:    netRx,
			netTx:    netTx,
			blkRead:  blkRead,
			blkWrite: blkWrite,
		}
		currentIO[name] = snap

		// Rate metrics (delta from previous sample)
		if c.prevIO != nil && !c.prevTime.IsZero() {
			elapsed := now.Sub(c.prevTime).Seconds()
			if prev, ok := c.prevIO[name]; ok && elapsed > 0 {
				metrics = append(metrics,
					Metric{Name: DockerMetric(name, "net_rx_bytes_sec"), Value: float64(netRx-prev.netRx) / elapsed},
					Metric{Name: DockerMetric(name, "net_tx_bytes_sec"), Value: float64(netTx-prev.netTx) / elapsed},
					Metric{Name: DockerMetric(name, "disk_read_bytes_sec"), Value: float64(blkRead-prev.blkRead) / elapsed},
					Metric{Name: DockerMetric(name, "disk_write_bytes_sec"), Value: float64(blkWrite-prev.blkWrite) / elapsed},
				)
			}
		}
	}

	c.prevIO = currentIO
	c.prevTime = now

	return metrics, nil
}

// CalculateDockerCPUPercent computes CPU % from Docker stats JSON.
func CalculateDockerCPUPercent(stats *types.StatsJSON) float64 {
	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage - stats.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(stats.CPUStats.SystemUsage - stats.PreCPUStats.SystemUsage)
	if systemDelta <= 0 || cpuDelta < 0 {
		return 0.0
	}
	numCPUs := float64(stats.CPUStats.OnlineCPUs)
	if numCPUs == 0 {
		numCPUs = float64(len(stats.CPUStats.CPUUsage.PercpuUsage))
	}
	if numCPUs == 0 {
		numCPUs = 1
	}
	return (cpuDelta / systemDelta) * numCPUs * 100.0
}

// containerName extracts a clean name from Docker container names.
func containerName(names []string) string {
	if len(names) == 0 {
		return "unknown"
	}
	name := names[0]
	name = strings.TrimPrefix(name, "/")
	return name
}
