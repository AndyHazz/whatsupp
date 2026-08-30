package checks

import (
	"fmt"
	"net"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

type SecurityScanner struct {
	Host        string
	Concurrency int
	Timeout     int
	PortStart   int
	PortEnd     int
	ProgressFn  func(scanned, total int) // optional progress callback
}

func (s *SecurityScanner) Scan() ([]int, error) {
	portStart := s.PortStart
	if portStart == 0 {
		portStart = 1
	}
	portEnd := s.PortEnd
	if portEnd == 0 {
		portEnd = 65535
	}
	concurrency := s.Concurrency
	if concurrency == 0 {
		concurrency = 200
	}
	timeout := time.Duration(s.Timeout) * time.Second
	if timeout == 0 {
		timeout = 2 * time.Second
	}

	var mu sync.Mutex
	openPorts := make([]int, 0)

	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	var scanned int64
	total := portEnd - portStart + 1
	reportInterval := total / 100
	if reportInterval < 1 {
		reportInterval = 1
	}

	for port := portStart; port <= portEnd; port++ {
		wg.Add(1)
		sem <- struct{}{}
		go func(p int) {
			defer wg.Done()
			defer func() { <-sem }()

			addr := net.JoinHostPort(s.Host, fmt.Sprintf("%d", p))
			conn, err := net.DialTimeout("tcp", addr, timeout)
			if err == nil {
				conn.Close()
				mu.Lock()
				openPorts = append(openPorts, p)
				mu.Unlock()
			}

			n := atomic.AddInt64(&scanned, 1)
			if s.ProgressFn != nil && (n%int64(reportInterval) == 0 || n == int64(total)) {
				s.ProgressFn(int(n), total)
			}
		}(port)
	}

	wg.Wait()
	sort.Ints(openPorts)
	return openPorts, nil
}

// FilterByHysteresis suppresses transient scan flaps by requiring a port to
// be consistently new/gone across the last `hysteresis` scans (including the
// current one). recentScans is newest-first; recentScans[0] is the current
// scan. Returns the subset of newPorts/gonePorts that should actually alert.
//
// Behaviour:
//   - hysteresis <= 1: pass-through (no filtering).
//   - len(recentScans) < hysteresis: insufficient history → suppress everything.
//     This is the "first few scans of a new target" case — better silent than
//     noisy.
//   - For each candidate "gone" port: must be absent in ALL of the most recent
//     `hysteresis` scans. (One scan that saw it back means: not really gone.)
//   - For each candidate "new" port: must be present in ALL of the most recent
//     `hysteresis` scans. (One scan that missed it means: not really stable.)
func FilterByHysteresis(newPorts, gonePorts []int, recentScans [][]int, hysteresis int) (filteredNew, filteredGone []int) {
	if hysteresis <= 1 {
		return newPorts, gonePorts
	}
	if len(recentScans) < hysteresis {
		return nil, nil
	}
	window := recentScans[:hysteresis]

	for _, p := range newPorts {
		stable := true
		for _, scan := range window {
			if !containsInt(scan, p) {
				stable = false
				break
			}
		}
		if stable {
			filteredNew = append(filteredNew, p)
		}
	}
	for _, p := range gonePorts {
		gone := true
		for _, scan := range window {
			if containsInt(scan, p) {
				gone = false
				break
			}
		}
		if gone {
			filteredGone = append(filteredGone, p)
		}
	}
	return
}

func containsInt(xs []int, x int) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

func CompareBaseline(baseline, current []int) (newPorts, gonePorts []int) {
	baseSet := make(map[int]bool, len(baseline))
	for _, p := range baseline {
		baseSet[p] = true
	}
	currSet := make(map[int]bool, len(current))
	for _, p := range current {
		currSet[p] = true
	}

	for _, p := range current {
		if !baseSet[p] {
			newPorts = append(newPorts, p)
		}
	}
	for _, p := range baseline {
		if !currSet[p] {
			gonePorts = append(gonePorts, p)
		}
	}

	sort.Ints(newPorts)
	sort.Ints(gonePorts)
	return
}

// ApplyBaselineDrift folds an alerted change back into the baseline, so a
// change is reported once rather than on every subsequent scan.
//
// Only ports we actually alerted on are folded in. Drift that hysteresis
// suppressed is deliberately left out: absorbing it would let a transient
// flap become the new normal without the change ever being reported.
func ApplyBaselineDrift(baseline, alertedNew, alertedGone []int) []int {
	set := make(map[int]bool, len(baseline)+len(alertedNew))
	for _, p := range baseline {
		set[p] = true
	}
	for _, p := range alertedNew {
		set[p] = true
	}
	for _, p := range alertedGone {
		delete(set, p)
	}

	updated := make([]int, 0, len(set))
	for p := range set {
		updated = append(updated, p)
	}
	sort.Ints(updated)
	return updated
}
