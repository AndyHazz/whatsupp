package checks

import (
	"net"
	"testing"
)

func TestSecurityScanner_FindsOpenPort(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error: %v", err)
	}
	defer ln.Close()

	addr := ln.Addr().(*net.TCPAddr)
	port := addr.Port

	s := &SecurityScanner{
		Host:        "127.0.0.1",
		Concurrency: 50,
		Timeout:     1,
		PortStart:   port,
		PortEnd:     port,
	}
	openPorts, err := s.Scan()
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}
	if len(openPorts) != 1 || openPorts[0] != port {
		t.Errorf("openPorts = %v, want [%d]", openPorts, port)
	}
}

func TestSecurityScanner_NoOpenPorts(t *testing.T) {
	s := &SecurityScanner{
		Host:        "127.0.0.1",
		Concurrency: 50,
		Timeout:     1,
		PortStart:   1,
		PortEnd:     5,
	}
	_, err := s.Scan()
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}
}

func TestCompareBaseline_NewPort(t *testing.T) {
	baseline := []int{22, 80, 443}
	current := []int{22, 80, 443, 4444}

	newPorts, gonePorts := CompareBaseline(baseline, current)
	if len(newPorts) != 1 || newPorts[0] != 4444 {
		t.Errorf("newPorts = %v, want [4444]", newPorts)
	}
	if len(gonePorts) != 0 {
		t.Errorf("gonePorts = %v, want []", gonePorts)
	}
}

func TestCompareBaseline_PortDisappeared(t *testing.T) {
	baseline := []int{22, 80, 443}
	current := []int{22, 80}

	newPorts, gonePorts := CompareBaseline(baseline, current)
	if len(newPorts) != 0 {
		t.Errorf("newPorts = %v, want []", newPorts)
	}
	if len(gonePorts) != 1 || gonePorts[0] != 443 {
		t.Errorf("gonePorts = %v, want [443]", gonePorts)
	}
}

func TestCompareBaseline_NoChange(t *testing.T) {
	baseline := []int{22, 80, 443}
	current := []int{22, 80, 443}

	newPorts, gonePorts := CompareBaseline(baseline, current)
	if len(newPorts) != 0 {
		t.Errorf("newPorts = %v, want []", newPorts)
	}
	if len(gonePorts) != 0 {
		t.Errorf("gonePorts = %v, want []", gonePorts)
	}
}
