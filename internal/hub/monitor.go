package hub

import (
	"github.com/andyhazz/whatsupp/internal/checks"
)

type MonitorStatus int

const (
	StatusUp   MonitorStatus = iota
	StatusDown
)

func (s MonitorStatus) String() string {
	switch s {
	case StatusUp:
		return "UP"
	case StatusDown:
		return "DOWN"
	default:
		return "UNKNOWN"
	}
}

type Transition int

const (
	TransitionNone   Transition = iota
	TransitionToDown
	TransitionToUp
)

func (t Transition) String() string {
	switch t {
	case TransitionNone:
		return "none"
	case TransitionToDown:
		return "to_down"
	case TransitionToUp:
		return "to_up"
	default:
		return "unknown"
	}
}

type MonitorState struct {
	Name                string
	Status              MonitorStatus
	FailureThreshold    int
	ConsecutiveFailures int
	LastError           string
	TotalChecks         int64
	TotalUp             int64
}

// UptimePct returns the uptime percentage based on recorded results.
func (ms *MonitorState) UptimePct() float64 {
	if ms.TotalChecks == 0 {
		return 100.0
	}
	return float64(ms.TotalUp) / float64(ms.TotalChecks) * 100.0
}

func NewMonitorState(name string, failureThreshold int) *MonitorState {
	return &MonitorState{
		Name:             name,
		Status:           StatusUp,
		FailureThreshold: failureThreshold,
	}
}

func (ms *MonitorState) RecordResult(result checks.Result) Transition {
	ms.TotalChecks++
	if result.Status == "up" {
		ms.TotalUp++
		ms.ConsecutiveFailures = 0
		ms.LastError = ""
		if ms.Status == StatusDown {
			ms.Status = StatusUp
			return TransitionToUp
		}
		return TransitionNone
	}

	ms.ConsecutiveFailures++
	ms.LastError = result.Error

	if ms.Status == StatusUp && ms.ConsecutiveFailures >= ms.FailureThreshold {
		ms.Status = StatusDown
		return TransitionToDown
	}

	return TransitionNone
}
