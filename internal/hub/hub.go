package hub

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/andyhazz/whatsupp/internal/alerting"
	"github.com/andyhazz/whatsupp/internal/checks"
	"github.com/andyhazz/whatsupp/internal/config"
	"github.com/andyhazz/whatsupp/internal/store"
)

// Hub is the main orchestrator that ties together checks, storage,
// state management, incidents, alerting, and downsampling.
type Hub struct {
	cfg             *config.Config
	store           *store.Store
	alerter         *alerting.NtfyClient
	scheduler       *Scheduler
	downsampler     *Downsampler
	incidentManager *IncidentManager
	monitorStates   map[string]*MonitorState
	resultCh        chan checks.Result
	stopCh          chan struct{}
}

// New creates a Hub from config.
func New(cfg *config.Config) (*Hub, error) {
	s, err := store.Open(cfg.Server.DBPath)
	if err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}

	ntfyClient := alerting.NewNtfyClient(alerting.NtfyConfig{
		URL:              cfg.Alerting.Ntfy.URL,
		Topic:            cfg.Alerting.Ntfy.Topic,
		Username:         cfg.Alerting.Ntfy.Username,
		Password:         cfg.Alerting.Ntfy.Password,
		ReminderInterval: cfg.Alerting.Thresholds.DownReminderInterval,
	})

	resultCh := make(chan checks.Result, 100)

	// Initialize monitor states
	states := make(map[string]*MonitorState)
	for _, m := range cfg.Monitors {
		threshold := m.FailureThreshold
		if threshold == 0 {
			threshold = cfg.Alerting.DefaultFailureThreshold
		}
		states[m.Name] = NewMonitorState(m.Name, threshold)
	}

	// Create scheduler and register checkers
	sched := NewScheduler(cfg.Monitors, resultCh)
	for _, m := range cfg.Monitors {
		var checker Checker
		switch m.Type {
		case "http":
			checker = &checks.HTTPChecker{URL: m.URL, Timeout: 10}
		case "ping":
			checker = &checks.PingChecker{Host: m.Host, Count: 3, Timeout: 10}
		case "port":
			checker = &checks.PortChecker{Host: m.Host, Port: m.Port, Timeout: 10}
		}
		if checker != nil {
			sched.RegisterChecker(m.Name, checker)
		}
	}

	retention := RetentionConfig{
		CheckResultsRaw: cfg.Retention.CheckResultsRaw,
		Hourly:          cfg.Retention.Hourly,
	}
	if retention.CheckResultsRaw == 0 {
		retention = DefaultRetentionConfig()
	}

	return &Hub{
		cfg:             cfg,
		store:           s,
		alerter:         ntfyClient,
		scheduler:       sched,
		downsampler:     NewDownsampler(s, retention),
		incidentManager: NewIncidentManager(s),
		monitorStates:   states,
		resultCh:        resultCh,
		stopCh:          make(chan struct{}),
	}, nil
}

// Run starts the hub: scheduler, result processor, downsampler.
func (h *Hub) Run() error {
	log.Printf("hub: starting with %d monitors", len(h.cfg.Monitors))

	// Start scheduler
	h.scheduler.Start()

	// Start downsampler
	h.downsampler.Start()

	// Start security scan scheduler (if targets configured)
	h.startSecurityScans()

	// Process results in the main goroutine
	h.processResults()

	return nil
}

// Close shuts down the hub.
func (h *Hub) Close() error {
	close(h.stopCh)
	h.scheduler.Stop()
	h.downsampler.Stop()
	return h.store.Close()
}

// processResults runs the main result processing loop.
func (h *Hub) processResults() {
	for {
		select {
		case result := <-h.resultCh:
			h.processResult(result)
		case <-h.stopCh:
			return
		}
	}
}

// processResult handles a single check result: store, state machine, incidents, alerts.
func (h *Hub) processResult(result checks.Result) {
	now := time.Now().Unix()

	// 1. Store the result
	if err := h.store.InsertCheckResult(result.Monitor, now, result.Status, result.LatencyMs, result.MetadataJSON); err != nil {
		log.Printf("hub: store check result error: %v", err)
	}

	// 2. Run through state machine
	ms, ok := h.monitorStates[result.Monitor]
	if !ok {
		log.Printf("hub: unknown monitor %q", result.Monitor)
		return
	}
	transition := ms.RecordResult(result)

	// 3. Handle incidents
	inc, err := h.incidentManager.HandleTransition(result.Monitor, transition, now, result.Error)
	if err != nil {
		log.Printf("hub: incident handling error: %v", err)
	}

	// 4. Send alerts
	switch transition {
	case TransitionToDown:
		cause := fmt.Sprintf("%s (%d/%d failures)", result.Error, ms.FailureThreshold, ms.FailureThreshold)
		if err := h.alerter.SendDown(result.Monitor, cause); err != nil {
			log.Printf("hub: alert DOWN error: %v", err)
		}
	case TransitionToUp:
		duration := "unknown"
		if inc != nil && inc.ResolvedAt != nil {
			dur := time.Duration(*inc.ResolvedAt-inc.StartedAt) * time.Second
			duration = formatDuration(dur)
		}
		if err := h.alerter.SendRecovery(result.Monitor, duration); err != nil {
			log.Printf("hub: alert RECOVERY error: %v", err)
		}
	}

	// 5. Check SSL cert expiry for HTTPS monitors (if metadata contains cert info)
	h.checkSSLExpiry(result)

	log.Printf("hub: %s status=%s latency=%.1fms transition=%s",
		result.Monitor, result.Status, result.LatencyMs, transition)
}

// checkSSLExpiry inspects HTTP check metadata for cert expiry warnings.
func (h *Hub) checkSSLExpiry(result checks.Result) {
	if result.MetadataJSON == "" {
		return
	}
	var meta struct {
		CertDaysLeft *int `json:"cert_days_left"`
	}
	if err := json.Unmarshal([]byte(result.MetadataJSON), &meta); err != nil || meta.CertDaysLeft == nil {
		return
	}

	daysLeft := *meta.CertDaysLeft
	for _, threshold := range h.cfg.Alerting.Thresholds.SSLExpiryDays {
		if daysLeft == threshold {
			if err := h.alerter.SendSSLExpiry(result.Monitor, daysLeft); err != nil {
				log.Printf("hub: alert SSL expiry error: %v", err)
			}
			break
		}
	}
}

// startSecurityScans sets up cron-scheduled security scans.
func (h *Hub) startSecurityScans() {
	if len(h.cfg.Security.Targets) == 0 {
		return
	}

	for _, target := range h.cfg.Security.Targets {
		go h.runSecurityScanLoop(target)
	}
}

func (h *Hub) runSecurityScanLoop(target config.SecurityTarget) {
	scanner := &checks.SecurityScanner{
		Host:        target.Host,
		Concurrency: target.ScanConcurrency,
		Timeout:     int(target.Timeout.Seconds()),
	}

	log.Printf("hub: security scan of %s starting", target.Host)
	openPorts, err := scanner.Scan()
	if err != nil {
		log.Printf("hub: security scan of %s failed: %v", target.Host, err)
		return
	}

	now := time.Now().Unix()
	portsJSON, _ := json.Marshal(openPorts)
	if err := h.store.InsertSecurityScan(target.Host, now, string(portsJSON)); err != nil {
		log.Printf("hub: store security scan error: %v", err)
	}

	// Compare against baseline
	baseline, err := h.store.GetSecurityBaseline(target.Host)
	if err != nil {
		log.Printf("hub: get security baseline error: %v", err)
		return
	}

	if baseline == nil {
		// First scan — set as baseline
		if err := h.store.UpsertSecurityBaseline(target.Host, string(portsJSON), now); err != nil {
			log.Printf("hub: set initial baseline error: %v", err)
		}
		log.Printf("hub: security baseline set for %s: %v", target.Host, openPorts)
		return
	}

	var baselinePorts []int
	json.Unmarshal([]byte(baseline.ExpectedPortsJSON), &baselinePorts)

	newPorts, gonePorts := checks.CompareBaseline(baselinePorts, openPorts)
	for _, p := range newPorts {
		if err := h.alerter.SendNewPort(target.Host, p); err != nil {
			log.Printf("hub: alert new port error: %v", err)
		}
	}
	for _, p := range gonePorts {
		if err := h.alerter.SendPortGone(target.Host, p); err != nil {
			log.Printf("hub: alert port gone error: %v", err)
		}
	}

	log.Printf("hub: security scan of %s complete: %d open ports, %d new, %d gone",
		target.Host, len(openPorts), len(newPorts), len(gonePorts))
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		m := int(d.Minutes())
		s := int(d.Seconds()) % 60
		return fmt.Sprintf("%dm %ds", m, s)
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	return fmt.Sprintf("%dh %dm", h, m)
}
