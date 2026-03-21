package checks

import (
	"net"
	"testing"
)

func TestPortCheck_Open(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error: %v", err)
	}
	defer ln.Close()

	addr := ln.Addr().(*net.TCPAddr)
	c := &PortChecker{Host: "127.0.0.1", Port: addr.Port, Timeout: 5}
	result := c.Check("TestPort")
	if result.Status != "up" {
		t.Errorf("Status = %q, want %q (error: %s)", result.Status, "up", result.Error)
	}
	if result.LatencyMs <= 0 {
		t.Logf("LatencyMs = %f", result.LatencyMs)
	}
}

func TestPortCheck_Closed(t *testing.T) {
	c := &PortChecker{Host: "127.0.0.1", Port: 1, Timeout: 2}
	result := c.Check("TestPort")
	if result.Status != "down" {
		t.Errorf("Status = %q, want %q", result.Status, "down")
	}
	if result.Error == "" {
		t.Error("Error should be non-empty for closed port")
	}
}

func TestPortCheck_Unreachable(t *testing.T) {
	c := &PortChecker{Host: "192.0.2.1", Port: 80, Timeout: 2}
	result := c.Check("TestPort")
	if result.Status != "down" {
		t.Errorf("Status = %q, want %q", result.Status, "down")
	}
}
