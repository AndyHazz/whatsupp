package checks

import (
	"context"
	"fmt"
	"net"
	"time"
)

type DNSChecker struct {
	Host    string // DNS server address (e.g. "192.168.50.50")
	Port    int    // DNS port (default 53)
	Query   string // domain to resolve (default "google.com")
	Timeout int    // seconds
}

func (c *DNSChecker) Check(monitorName string) Result {
	port := c.Port
	if port == 0 {
		port = 53
	}
	query := c.Query
	if query == "" {
		query = "google.com"
	}
	timeout := time.Duration(c.Timeout) * time.Second
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	addr := fmt.Sprintf("%s:%d", c.Host, port)
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{Timeout: timeout}
			return d.DialContext(ctx, "udp", addr)
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	start := time.Now()
	ips, err := resolver.LookupHost(ctx, query)
	latency := float64(time.Since(start).Microseconds()) / 1000.0

	if err != nil {
		return Result{
			Monitor:   monitorName,
			Status:    "down",
			LatencyMs: latency,
			Error:     fmt.Sprintf("dns lookup %s via %s: %v", query, addr, err),
		}
	}

	if len(ips) == 0 {
		return Result{
			Monitor:   monitorName,
			Status:    "down",
			LatencyMs: latency,
			Error:     fmt.Sprintf("dns lookup %s via %s: no results", query, addr),
		}
	}

	return Result{
		Monitor:   monitorName,
		Status:    "up",
		LatencyMs: latency,
	}
}
