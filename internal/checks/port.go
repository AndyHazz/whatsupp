package checks

import (
	"encoding/json"
	"fmt"
	"net"
	"time"
)

type PortChecker struct {
	Host    string
	Port    int
	Timeout int
}

type portMetadata struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

func (c *PortChecker) Check(monitorName string) Result {
	timeout := time.Duration(c.Timeout) * time.Second
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	addr := fmt.Sprintf("%s:%d", c.Host, c.Port)
	start := time.Now()
	conn, err := net.DialTimeout("tcp", addr, timeout)
	latency := float64(time.Since(start).Microseconds()) / 1000.0

	meta := portMetadata{Host: c.Host, Port: c.Port}
	metaJSON, _ := json.Marshal(meta)

	if err != nil {
		return Result{
			Monitor:      monitorName,
			Status:       "down",
			LatencyMs:    latency,
			MetadataJSON: string(metaJSON),
			Error:        fmt.Sprintf("tcp connect %s: %v", addr, err),
		}
	}
	conn.Close()

	return Result{
		Monitor:      monitorName,
		Status:       "up",
		LatencyMs:    latency,
		MetadataJSON: string(metaJSON),
	}
}
