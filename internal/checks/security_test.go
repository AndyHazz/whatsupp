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

func TestFilterByHysteresis_Disabled(t *testing.T) {
	// hysteresis <= 1 → pass-through
	newPorts := []int{4444}
	gonePorts := []int{443}
	fn, fg := FilterByHysteresis(newPorts, gonePorts, [][]int{{22, 80, 4444}}, 1)
	if len(fn) != 1 || fn[0] != 4444 {
		t.Errorf("disabled: filteredNew = %v, want [4444]", fn)
	}
	if len(fg) != 1 || fg[0] != 443 {
		t.Errorf("disabled: filteredGone = %v, want [443]", fg)
	}
}

func TestFilterByHysteresis_InsufficientHistory(t *testing.T) {
	// Only 1 scan in history but hysteresis=2 → suppress all
	fn, fg := FilterByHysteresis([]int{4444}, []int{443}, [][]int{{22, 80, 4444}}, 2)
	if len(fn) != 0 {
		t.Errorf("insufficient history: filteredNew = %v, want []", fn)
	}
	if len(fg) != 0 {
		t.Errorf("insufficient history: filteredGone = %v, want []", fg)
	}
}

func TestFilterByHysteresis_GoneStable(t *testing.T) {
	// Port 443 absent in both scans → confirmed gone
	scans := [][]int{
		{22, 80}, // current scan, 443 missing
		{22, 80}, // previous scan, 443 also missing
	}
	_, fg := FilterByHysteresis(nil, []int{443}, scans, 2)
	if len(fg) != 1 || fg[0] != 443 {
		t.Errorf("gone stable: filteredGone = %v, want [443]", fg)
	}
}

func TestFilterByHysteresis_GoneFlap(t *testing.T) {
	// Port 443 absent in current scan but present in previous → flap, suppress
	scans := [][]int{
		{22, 80},      // current, 443 missing
		{22, 80, 443}, // previous, 443 present
	}
	_, fg := FilterByHysteresis(nil, []int{443}, scans, 2)
	if len(fg) != 0 {
		t.Errorf("gone flap: filteredGone = %v, want []", fg)
	}
}

func TestFilterByHysteresis_NewStable(t *testing.T) {
	// Port 4444 present in both scans → confirmed new
	scans := [][]int{
		{22, 80, 4444},
		{22, 80, 4444},
	}
	fn, _ := FilterByHysteresis([]int{4444}, nil, scans, 2)
	if len(fn) != 1 || fn[0] != 4444 {
		t.Errorf("new stable: filteredNew = %v, want [4444]", fn)
	}
}

func TestFilterByHysteresis_NewFlap(t *testing.T) {
	// Port 4444 present in current but missing previously → not stable, suppress
	scans := [][]int{
		{22, 80, 4444},
		{22, 80},
	}
	fn, _ := FilterByHysteresis([]int{4444}, nil, scans, 2)
	if len(fn) != 0 {
		t.Errorf("new flap: filteredNew = %v, want []", fn)
	}
}

func TestFilterByHysteresis_ThreeScans(t *testing.T) {
	// hysteresis=3: 443 must be missing in all 3 most recent scans
	stable := [][]int{{22, 80}, {22, 80}, {22, 80}}
	_, fg := FilterByHysteresis(nil, []int{443}, stable, 3)
	if len(fg) != 1 {
		t.Errorf("3-scan stable gone: filteredGone = %v, want [443]", fg)
	}

	// One older scan saw 443 present → suppress
	flapping := [][]int{{22, 80}, {22, 80}, {22, 80, 443}}
	_, fg = FilterByHysteresis(nil, []int{443}, flapping, 3)
	if len(fg) != 0 {
		t.Errorf("3-scan flap: filteredGone = %v, want []", fg)
	}
}

func TestFilterByHysteresis_MixedPorts(t *testing.T) {
	// 443 stably gone, 8080 flapping gone, 4444 stably new, 9999 flapping new
	scans := [][]int{
		{22, 80, 4444},      // current — 443 + 8080 missing, 4444 + nothing else new
		{22, 80, 4444, 999}, // previous — 443 + 8080 missing too; 4444 also there
	}
	// Pretend the diff vs baseline gave: gone={443, 8080}, new={4444, 9999}
	fn, fg := FilterByHysteresis([]int{4444, 9999}, []int{443, 8080}, scans, 2)
	if len(fg) != 2 || fg[0] != 443 || fg[1] != 8080 {
		t.Errorf("mixed: filteredGone = %v, want [443 8080]", fg)
	}
	if len(fn) != 1 || fn[0] != 4444 {
		t.Errorf("mixed: filteredNew = %v, want [4444]", fn)
	}
}
