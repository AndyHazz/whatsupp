package checks

import (
	"encoding/json"
	"fmt"
	"time"

	probing "github.com/prometheus-community/pro-bing"
)

type PingChecker struct {
	Host    string
	Count   int
	Timeout int
}

type pingMetadata struct {
	PacketsSent int     `json:"packets_sent"`
	PacketsRecv int     `json:"packets_recv"`
	PacketLoss  float64 `json:"packet_loss_pct"`
	MinRTT      float64 `json:"min_rtt_ms"`
	AvgRTT      float64 `json:"avg_rtt_ms"`
	MaxRTT      float64 `json:"max_rtt_ms"`
}

func (c *PingChecker) Check(monitorName string) Result {
	count := c.Count
	if count == 0 {
		count = 3
	}
	timeout := time.Duration(c.Timeout) * time.Second
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	pinger, err := probing.NewPinger(c.Host)
	if err != nil {
		return Result{
			Monitor: monitorName,
			Status:  "down",
			Error:   fmt.Sprintf("create pinger: %v", err),
		}
	}

	pinger.Count = count
	pinger.Timeout = timeout
	pinger.SetPrivileged(true)

	err = pinger.Run()
	if err != nil {
		return Result{
			Monitor: monitorName,
			Status:  "down",
			Error:   fmt.Sprintf("ping: %v", err),
		}
	}

	stats := pinger.Statistics()

	meta := pingMetadata{
		PacketsSent: stats.PacketsSent,
		PacketsRecv: stats.PacketsRecv,
		PacketLoss:  stats.PacketLoss,
		MinRTT:      float64(stats.MinRtt.Microseconds()) / 1000.0,
		AvgRTT:      float64(stats.AvgRtt.Microseconds()) / 1000.0,
		MaxRTT:      float64(stats.MaxRtt.Microseconds()) / 1000.0,
	}
	metaJSON, _ := json.Marshal(meta)

	status := "up"
	var errMsg string
	if stats.PacketsRecv == 0 {
		status = "down"
		errMsg = fmt.Sprintf("100%% packet loss to %s", c.Host)
	}

	return Result{
		Monitor:      monitorName,
		Status:       status,
		LatencyMs:    meta.AvgRTT,
		MetadataJSON: string(metaJSON),
		Error:        errMsg,
	}
}
