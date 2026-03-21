package checks

import (
	"fmt"
	"net"
	"sort"
	"sync"
	"time"
)

type SecurityScanner struct {
	Host        string
	Concurrency int
	Timeout     int
	PortStart   int
	PortEnd     int
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
	var openPorts []int

	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for port := portStart; port <= portEnd; port++ {
		wg.Add(1)
		sem <- struct{}{}
		go func(p int) {
			defer wg.Done()
			defer func() { <-sem }()

			addr := fmt.Sprintf("%s:%d", s.Host, p)
			conn, err := net.DialTimeout("tcp", addr, timeout)
			if err == nil {
				conn.Close()
				mu.Lock()
				openPorts = append(openPorts, p)
				mu.Unlock()
			}
		}(port)
	}

	wg.Wait()
	sort.Ints(openPorts)
	return openPorts, nil
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
