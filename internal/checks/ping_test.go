package checks

import (
	"testing"
)

func TestPingCheck_Localhost(t *testing.T) {
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
	c := &PingChecker{Host: "192.0.2.1", Count: 1, Timeout: 2}
	result := c.Check("TestPing")
	if result.Status != "down" {
		t.Errorf("Status = %q, want %q", result.Status, "down")
	}
}
