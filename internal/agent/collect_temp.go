package agent

import (
	"context"
	"strings"

	"github.com/shirou/gopsutil/v4/sensors"
)

// TempCollector collects temperature sensor metrics.
type TempCollector struct{}

func NewTempCollector() *TempCollector { return &TempCollector{} }

func (c *TempCollector) Name() string { return "temp" }

func (c *TempCollector) Collect(ctx context.Context) ([]Metric, error) {
	temps, err := sensors.TemperaturesWithContext(ctx)
	if err != nil {
		// No sensors available is not an error
		if strings.Contains(err.Error(), "not implemented") ||
			strings.Contains(err.Error(), "no temperature") {
			return nil, nil
		}
		// gopsutil may return warnings alongside data; if we got data, use it
		if len(temps) == 0 {
			return nil, nil
		}
	}

	var metrics []Metric
	for _, t := range temps {
		if t.Temperature == 0 {
			continue
		}
		name := sanitizeSensorName(t.SensorKey)
		metrics = append(metrics, Metric{
			Name:  TempMetric(name),
			Value: t.Temperature,
		})
	}

	return metrics, nil
}

// sanitizeSensorName maps sensor keys to safe metric names.
func sanitizeSensorName(key string) string {
	// Map common CPU temperature sensor names to "cpu"
	lower := strings.ToLower(key)
	if strings.Contains(lower, "coretemp") && strings.Contains(lower, "package") {
		return "cpu"
	}
	if strings.Contains(lower, "k10temp") && strings.Contains(lower, "tctl") {
		return "cpu"
	}
	if strings.Contains(lower, "k10temp") && strings.Contains(lower, "tdie") {
		return "cpu"
	}

	// Map GPU sensors
	if strings.Contains(lower, "nouveau") || strings.Contains(lower, "amdgpu") ||
		strings.Contains(lower, "nvidia") {
		return "gpu"
	}

	// General sanitization: replace spaces and special chars with underscores
	name := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '_' || r == '-' {
			return r
		}
		return '_'
	}, key)

	// Collapse multiple underscores
	for strings.Contains(name, "__") {
		name = strings.ReplaceAll(name, "__", "_")
	}
	name = strings.Trim(name, "_")
	return strings.ToLower(name)
}
