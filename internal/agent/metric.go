package agent

import "time"

// Metric is a single named metric value.
type Metric struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}

// MetricBatch is a collection of metrics from a single host at a point in time.
type MetricBatch struct {
	Host      string    `json:"host"`
	Timestamp time.Time `json:"timestamp"`
	Metrics   []Metric  `json:"metrics"`
	Version   string    `json:"version,omitempty"`
}

// Metric naming convention helpers.

func CPUMetric(name string) string                    { return "cpu." + name }
func MemMetric(name string) string                    { return "mem." + name }
func DiskMetric(mount, name string) string             { return "disk." + mount + "." + name }
func NetMetric(iface, name string) string              { return "net." + iface + "." + name }
func TempMetric(name string) string                    { return "temp." + name }
func DockerMetric(container, name string) string       { return "docker." + container + "." + name }
func BatteryMetric(name string) string                  { return "battery." + name }
