package checks

import (
	"strings"
	"time"
)

// NodeExporterMapper maps Prometheus node_exporter metrics to whatsupp naming convention.
type NodeExporterMapper struct {
	prevCPU map[string]float64 // previous cpu_seconds_total per cpu:mode
	prevNet map[string]float64 // previous net bytes per iface+direction
	prevTS  time.Time
}

// NewNodeExporterMapper creates a new mapper.
func NewNodeExporterMapper() *NodeExporterMapper {
	return &NodeExporterMapper{
		prevCPU: make(map[string]float64),
		prevNet: make(map[string]float64),
	}
}

// Map converts Prometheus metrics to whatsupp metrics.
func (m *NodeExporterMapper) Map(metrics []PrometheusMetric) []MappedMetric {
	now := time.Now()
	var result []MappedMetric

	// Collect current CPU seconds
	currentCPU := make(map[string]float64) // cpu:mode -> value
	cpuTotals := make(map[string]float64)  // cpu -> total
	cpuIdle := make(map[string]float64)    // cpu -> idle

	// Also collect memory and filesystem values for derived metrics
	var memTotal, memAvail, swapTotal, swapFree float64
	sizeMap := make(map[string]float64)
	availMap := make(map[string]float64)

	for _, pm := range metrics {
		switch pm.Name {
		case "node_cpu_seconds_total":
			cpu := pm.Labels["cpu"]
			mode := pm.Labels["mode"]
			key := cpu + ":" + mode
			currentCPU[key] = pm.Value
			cpuTotals[cpu] += pm.Value
			if mode == "idle" {
				cpuIdle[cpu] = pm.Value
			}

		case "node_load1":
			result = append(result, MappedMetric{Name: "cpu.load_1m", Value: pm.Value})
		case "node_load5":
			result = append(result, MappedMetric{Name: "cpu.load_5m", Value: pm.Value})
		case "node_load15":
			result = append(result, MappedMetric{Name: "cpu.load_15m", Value: pm.Value})

		case "node_memory_MemTotal_bytes":
			memTotal = pm.Value
			result = append(result, MappedMetric{Name: "mem.total_bytes", Value: pm.Value})
		case "node_memory_MemAvailable_bytes":
			memAvail = pm.Value
			result = append(result, MappedMetric{Name: "mem.available_bytes", Value: pm.Value})
		case "node_memory_MemFree_bytes":
			result = append(result, MappedMetric{Name: "mem.free_bytes", Value: pm.Value})
		case "node_memory_SwapTotal_bytes":
			swapTotal = pm.Value
			result = append(result, MappedMetric{Name: "mem.swap_total_bytes", Value: pm.Value})
		case "node_memory_SwapFree_bytes":
			swapFree = pm.Value

		case "node_filesystem_size_bytes":
			mount := pm.Labels["mountpoint"]
			if !isFilteredMount(mount) {
				sizeMap[mount] = pm.Value
				result = append(result, MappedMetric{Name: "disk." + mount + ".total_bytes", Value: pm.Value})
			}
		case "node_filesystem_avail_bytes":
			mount := pm.Labels["mountpoint"]
			if !isFilteredMount(mount) {
				availMap[mount] = pm.Value
				result = append(result, MappedMetric{Name: "disk." + mount + ".avail_bytes", Value: pm.Value})
			}
		case "node_filesystem_free_bytes":
			mount := pm.Labels["mountpoint"]
			if !isFilteredMount(mount) {
				result = append(result, MappedMetric{Name: "disk." + mount + ".free_bytes", Value: pm.Value})
			}

		case "node_network_receive_bytes_total":
			device := pm.Labels["device"]
			if !isFilteredIface(device) {
				result = append(result, MappedMetric{Name: "net." + device + ".rx_bytes", Value: pm.Value})
			}
		case "node_network_transmit_bytes_total":
			device := pm.Labels["device"]
			if !isFilteredIface(device) {
				result = append(result, MappedMetric{Name: "net." + device + ".tx_bytes", Value: pm.Value})
			}
		case "node_network_receive_errs_total":
			device := pm.Labels["device"]
			if !isFilteredIface(device) {
				result = append(result, MappedMetric{Name: "net." + device + ".rx_errors", Value: pm.Value})
			}
		case "node_network_transmit_errs_total":
			device := pm.Labels["device"]
			if !isFilteredIface(device) {
				result = append(result, MappedMetric{Name: "net." + device + ".tx_errors", Value: pm.Value})
			}

		case "node_hwmon_temp_celsius":
			chip := pm.Labels["chip"]
			sensor := pm.Labels["sensor"]
			name := mapTempSensor(chip, sensor)
			result = append(result, MappedMetric{Name: "temp." + name, Value: pm.Value})
		}
	}

	// CPU usage rate calculation (must happen BEFORE updating prevCPU)
	if !m.prevTS.IsZero() && len(m.prevCPU) > 0 {
		// Calculate total delta and idle delta across all CPUs
		var totalDelta, idleDelta float64
		hasDelta := false
		for cpu := range cpuTotals {
			// Previous total for this CPU
			prevTotal := 0.0
			prevIdleVal := 0.0
			for key, val := range m.prevCPU {
				parts := strings.SplitN(key, ":", 2)
				if len(parts) == 2 && parts[0] == cpu {
					prevTotal += val
					if parts[1] == "idle" {
						prevIdleVal = val
					}
				}
			}
			td := cpuTotals[cpu] - prevTotal
			id := cpuIdle[cpu] - prevIdleVal
			if td > 0 {
				totalDelta += td
				idleDelta += id
				hasDelta = true
			}
		}
		if hasDelta && totalDelta > 0 {
			usagePct := (1 - idleDelta/totalDelta) * 100
			result = append(result, MappedMetric{Name: "cpu.usage_pct", Value: usagePct})
		}
	}

	// Derived memory metrics
	if memTotal > 0 {
		used := memTotal - memAvail
		result = append(result, MappedMetric{Name: "mem.used_bytes", Value: used})
		result = append(result, MappedMetric{Name: "mem.usage_pct", Value: (used / memTotal) * 100})
	}
	if swapTotal > 0 {
		result = append(result, MappedMetric{Name: "mem.swap_used_bytes", Value: swapTotal - swapFree})
	}

	// Derived disk metrics
	for mount, size := range sizeMap {
		if avail, ok := availMap[mount]; ok && size > 0 {
			used := size - avail
			result = append(result, MappedMetric{Name: "disk." + mount + ".used_bytes", Value: used})
			result = append(result, MappedMetric{Name: "disk." + mount + ".usage_pct", Value: (used / size) * 100})
		}
	}

	// Update state for next call
	m.prevTS = now
	m.prevCPU = currentCPU

	return result
}

// mapTempSensor maps hwmon chip/sensor to a whatsupp temperature metric name.
func mapTempSensor(chip, sensor string) string {
	lower := strings.ToLower(chip)
	if strings.Contains(lower, "coretemp") || strings.Contains(lower, "k10temp") {
		return "cpu"
	}
	if strings.Contains(lower, "nouveau") || strings.Contains(lower, "amdgpu") ||
		strings.Contains(lower, "nvidia") {
		return "gpu"
	}
	// Fall back to chip_sensor
	name := strings.ToLower(chip + "_" + sensor)
	name = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			return r
		}
		return '_'
	}, name)
	return name
}
