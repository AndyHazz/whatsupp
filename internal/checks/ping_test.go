package checks

import (
	"os"
	"strings"
	"testing"
)

func canPing() bool {
	c := &PingChecker{Host: "127.0.0.1", Count: 1, Timeout: 2}
	r := c.Check("probe")
	return !strings.Contains(r.Error, "operation not permitted")
}

func TestPingCheck_Localhost(t *testing.T) {
	if os.Getuid() != 0 && !canPing() {
		t.Skip("skipping: ICMP ping requires root or CAP_NET_RAW")
	}
	c := &PingChecker{Host: "127.0.0.1", Count: 3, Timeout: 5}
	result := c.Check("TestPing")
	if result.Status != "up" {
		t.Errorf("Status = %q, want %q (error: %s)", result.Status, "up", result.Error)
	}
	if result.LatencyMs <= 0 {
		t.Logf("LatencyMs = %f (localhost can be very fast)", result.LatencyMs)
	}
}

func TestPingCheck_Unreachable(t *testing.T) {
	if os.Getuid() != 0 && !canPing() {
		t.Skip("skipping: ICMP ping requires root or CAP_NET_RAW")
	}
	c := &PingChecker{Host: "192.0.2.1", Count: 1, Timeout: 2}
	result := c.Check("TestPing")
	if result.Status != "down" {
		t.Errorf("Status = %q, want %q", result.Status, "down")
	}
}
