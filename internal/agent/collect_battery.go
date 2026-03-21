package agent

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// BatteryCollector collects battery charge metrics from sysfs.
type BatteryCollector struct{}

func NewBatteryCollector() *BatteryCollector { return &BatteryCollector{} }

func (c *BatteryCollector) Name() string { return "battery" }

func (c *BatteryCollector) Collect(ctx context.Context) ([]Metric, error) {
	matches, err := filepath.Glob("/sys/class/power_supply/BAT*")
	if err != nil || len(matches) == 0 {
		return nil, nil // no battery, not an error
	}

	var metrics []Metric
	for _, bat := range matches {
		capData, err := os.ReadFile(filepath.Join(bat, "capacity"))
		if err != nil {
			continue
		}
		pct, err := strconv.ParseFloat(strings.TrimSpace(string(capData)), 64)
		if err != nil {
			continue
		}
		metrics = append(metrics, Metric{
			Name:  BatteryMetric("charge_pct"),
			Value: pct,
		})

		statusData, err := os.ReadFile(filepath.Join(bat, "status"))
		if err != nil {
			continue
		}
		status := strings.TrimSpace(string(statusData))
		charging := 0.0
		if status == "Charging" || status == "Full" {
			charging = 1.0
		}
		metrics = append(metrics, Metric{
			Name:  BatteryMetric("charging"),
			Value: charging,
		})

		// Only report first battery
		break
	}

	return metrics, nil
}
