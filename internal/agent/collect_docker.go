package agent

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

// DockerCollector collects Docker container metrics.
type DockerCollector struct {
	dockerHost string
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
		data, err := io.ReadAll(stats.Body)
		stats.Body.Close()
		if err != nil {
			continue
		}
		if err := json.Unmarshal(data, &statsJSON); err != nil {
			continue
		}

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
	}

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
