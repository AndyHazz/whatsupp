package hub

import (
	"sync"
	"time"

	"github.com/andyhazz/whatsupp/internal/checks"
	"github.com/andyhazz/whatsupp/internal/config"
)

// Checker is the interface that all check types implement.
type Checker interface {
	Check(monitorName string) checks.Result
}

// Scheduler runs checks at configured intervals.
type Scheduler struct {
	monitors []config.Monitor
	checkers map[string]Checker // monitor name -> checker
	resultCh chan<- checks.Result
	stopCh   chan struct{}
	wg       sync.WaitGroup
	mu       sync.Mutex
}

// NewScheduler creates a new check scheduler.
func NewScheduler(monitors []config.Monitor, resultCh chan<- checks.Result) *Scheduler {
	return &Scheduler{
		monitors: monitors,
		checkers: make(map[string]Checker),
		resultCh: resultCh,
		stopCh:   make(chan struct{}),
	}
}

// RegisterChecker registers a checker for a named monitor.
func (s *Scheduler) RegisterChecker(name string, checker Checker) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.checkers[name] = checker
}

// Start begins running all scheduled checks.
func (s *Scheduler) Start() {
	for _, m := range s.monitors {
		checker, ok := s.checkers[m.Name]
		if !ok {
			continue
		}
		s.wg.Add(1)
		go s.runMonitor(m, checker)
	}
}

// Stop signals all check goroutines to stop and waits for them.
func (s *Scheduler) Stop() {
	close(s.stopCh)
	s.wg.Wait()
}

func (s *Scheduler) runMonitor(m config.Monitor, checker Checker) {
	defer s.wg.Done()

	ticker := time.NewTicker(m.Interval)
	defer ticker.Stop()

	// Run immediately on start
	result := checker.Check(m.Name)
	result.Monitor = m.Name
	select {
	case s.resultCh <- result:
	case <-s.stopCh:
		return
	}

	for {
		select {
		case <-ticker.C:
			result := checker.Check(m.Name)
			result.Monitor = m.Name
			select {
			case s.resultCh <- result:
			case <-s.stopCh:
				return
			}
		case <-s.stopCh:
			return
		}
	}
}
