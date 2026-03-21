package hub

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/andyhazz/whatsupp/internal/alerting"
	"github.com/andyhazz/whatsupp/internal/store"
)

// StalenessChecker detects agents that haven't reported metrics recently.
type StalenessChecker struct {
	store     *store.Store
	alerter   *alerting.NtfyClient
	threshold time.Duration
}

// NewStalenessChecker creates a new staleness checker.
func NewStalenessChecker(s *store.Store, alerter *alerting.NtfyClient, threshold time.Duration) *StalenessChecker {
	if threshold == 0 {
		threshold = 5 * time.Minute
	}
	return &StalenessChecker{
		store:     s,
		alerter:   alerter,
		threshold: threshold,
	}
}

// Check runs the staleness detection.
func (sc *StalenessChecker) Check(ctx context.Context) error {
	now := time.Now()
	threshold := now.Add(-sc.threshold)

	// Get stale agents
	staleAgents, err := sc.store.GetStaleAgents(threshold)
	if err != nil {
		return fmt.Errorf("get stale agents: %w", err)
	}

	// For each stale agent, check if open incident exists, if not create one
	for _, agent := range staleAgents {
		monitorName := "agent:" + agent.Host
		inc, err := sc.store.GetOpenIncidentForMonitor(monitorName)
		if err != nil {
			log.Printf("staleness: get open incident for %s: %v", agent.Host, err)
			continue
		}
		if inc != nil {
			continue // already has open incident
		}

		cause := fmt.Sprintf("no metrics from %s for %s", agent.Host, sc.threshold)
		_, err = sc.store.CreateIncidentWithTime(monitorName, now, cause)
		if err != nil {
			log.Printf("staleness: create incident for %s: %v", agent.Host, err)
			continue
		}

		// Send alert
		if sc.alerter != nil {
			if err := sc.alerter.SendDown(monitorName, cause); err != nil {
				log.Printf("staleness: alert for %s: %v", agent.Host, err)
			}
		}

		log.Printf("staleness: %s is stale (last seen: %s)", agent.Host, time.Unix(agent.LastSeenAt, 0))
	}

	// Check for recovered agents (have open incident but fresh heartbeat)
	allHeartbeats, err := sc.store.GetAllHeartbeats()
	if err != nil {
		return fmt.Errorf("get all heartbeats: %w", err)
	}

	for _, hb := range allHeartbeats {
		if time.Unix(hb.LastSeenAt, 0).Before(threshold) {
			continue // still stale
		}

		monitorName := "agent:" + hb.Host
		inc, err := sc.store.GetOpenIncidentForMonitor(monitorName)
		if err != nil || inc == nil {
			continue // no open incident
		}

		// Resolve the incident
		if err := sc.store.ResolveIncidentWithTime(inc.ID, now); err != nil {
			log.Printf("staleness: resolve incident for %s: %v", hb.Host, err)
			continue
		}

		// Send recovery alert
		if sc.alerter != nil {
			duration := now.Sub(time.Unix(inc.StartedAt, 0))
			if err := sc.alerter.SendRecovery(monitorName, formatDuration(duration)); err != nil {
				log.Printf("staleness: recovery alert for %s: %v", hb.Host, err)
			}
		}

		log.Printf("staleness: %s recovered", hb.Host)
	}

	return nil
}
