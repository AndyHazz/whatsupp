package hub

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/andyhazz/whatsupp/internal/alerting"
	"github.com/andyhazz/whatsupp/internal/api"
	"github.com/andyhazz/whatsupp/internal/checks"
	"github.com/andyhazz/whatsupp/internal/config"
	"github.com/andyhazz/whatsupp/internal/store"
)

// Hub is the main orchestrator that ties together checks, storage,
// state management, incidents, alerting, and downsampling.
type Hub struct {
	mu              sync.RWMutex
	cfg             *config.Config
	configPath      string
	store           *store.Store
	alerter         *alerting.NtfyClient
	scheduler       *Scheduler
	downsampler     *Downsampler
	incidentManager *IncidentManager
	monitorStates   map[string]*MonitorState
	monitorTypes    map[string]string // monitor name -> type (http, ping, port)
	lastResults     map[string]checks.Result
	resultCh        chan checks.Result
	stopCh          chan struct{}
	apiServer       *http.Server
	wsHub           *api.WSHub
}

// New creates a Hub from config.
func New(cfg *config.Config, configPath string) (*Hub, error) {
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

	// Initialize monitor states and type map
	states := make(map[string]*MonitorState)
	monTypes := make(map[string]string)
	for _, m := range cfg.Monitors {
		threshold := m.FailureThreshold
		if threshold == 0 {
			threshold = cfg.Alerting.DefaultFailureThreshold
		}
		states[m.Name] = NewMonitorState(m.Name, threshold)
		monTypes[m.Name] = m.Type
	}

	// Create scheduler and register checkers
	sched := NewScheduler(cfg.Monitors, resultCh)
	for _, m := range cfg.Monitors {
		var checker Checker
		switch m.Type {
		case "http":
			checker = &checks.HTTPChecker{URL: m.URL, Timeout: 10, InsecureSkipVerify: m.InsecureSkipVerify}
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
		CheckResultsRaw:  cfg.Retention.CheckResultsRaw,
		AgentMetricsRaw:  cfg.Retention.AgentMetricsRaw,
		AgentMetrics5Min: cfg.Retention.AgentMetrics5Min,
		Hourly:           cfg.Retention.Hourly,
	}
	if retention.CheckResultsRaw == 0 {
		retention = DefaultRetentionConfig()
	}

	return &Hub{
		cfg:             cfg,
		configPath:      configPath,
		store:           s,
		alerter:         ntfyClient,
		scheduler:       sched,
		downsampler:     NewDownsampler(s, retention),
		incidentManager: NewIncidentManager(s),
		monitorStates:   states,
		monitorTypes:    monTypes,
		lastResults:     make(map[string]checks.Result),
		resultCh:        resultCh,
		stopCh:          make(chan struct{}),
	}, nil
}

// Run starts the hub: scheduler, result processor, downsampler, API server.
func (h *Hub) Run() error {
	log.Printf("hub: starting with %d monitors", len(h.cfg.Monitors))

	// Start API server
	if err := h.startAPI(); err != nil {
		return fmt.Errorf("start API: %w", err)
	}

	// Start scheduler
	h.scheduler.Start()

	// Start downsampler
	h.downsampler.Start()

	// Start security scan scheduler (if targets configured)
	h.startSecurityScans()

	// Start scrape targets
	h.startScrapeTargets()

	// Start staleness checker
	h.startStalenessChecker()

	// Process results in the main goroutine
	h.processResults()

	return nil
}

// startStalenessChecker runs the agent staleness detector every 60 seconds.
func (h *Hub) startStalenessChecker() {
	sc := NewStalenessChecker(h.store, h.alerter, 5*time.Minute)

	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := sc.Check(context.Background()); err != nil {
					log.Printf("hub: staleness check error: %v", err)
				}
			case <-h.stopCh:
				return
			}
		}
	}()
}

// Close shuts down the hub.
func (h *Hub) Close() error {
	close(h.stopCh)
	h.scheduler.Stop()
	h.downsampler.Stop()
	h.stopAPI(context.Background())
	if h.wsHub != nil {
		h.wsHub.Stop()
	}
	return h.store.Close()
}

// startAPI initializes and starts the HTTP API server.
func (h *Hub) startAPI() error {
	adapter := NewStoreAdapter(h.store)

	// Ensure initial admin user exists
	if err := api.EnsureAdminUser(adapter, h.cfg.Auth.InitialUsername, h.cfg.Auth.InitialPassword); err != nil {
		return fmt.Errorf("ensuring admin user: %w", err)
	}

	// Build agent key map from config
	agentKeys := make(map[string]string)
	for _, agent := range h.cfg.Agents {
		agentKeys[agent.Name] = agent.Key
	}

	result := api.NewRouter(api.RouterConfig{
		Store:      adapter,
		Hub:        h,
		ConfigPath: h.configPath,
		AgentKeys:  agentKeys,
		BackupDir:  "/data/backups",
	})

	h.wsHub = result.WSHub

	h.apiServer = &http.Server{
		Addr:    h.cfg.Server.Listen,
		Handler: result.Handler,
	}

	go func() {
		log.Printf("API server listening on %s", h.cfg.Server.Listen)
		if err := h.apiServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("API server error: %v", err)
		}
	}()

	return nil
}

// stopAPI gracefully shuts down the HTTP API server.
func (h *Hub) stopAPI(ctx context.Context) {
	if h.apiServer != nil {
		h.apiServer.Shutdown(ctx)
	}
}

// --- api.HubState implementation ---

// MonitorStatuses returns the current status of all monitors.
func (h *Hub) MonitorStatuses() map[string]api.MonitorStatus {
	h.mu.RLock()
	defer h.mu.RUnlock()
	result := make(map[string]api.MonitorStatus, len(h.monitorStates))
	for name, ms := range h.monitorStates {
		status := "unknown"
		switch ms.Status {
		case StatusUp:
			status = "up"
		case StatusDown:
			status = "down"
		}
		var latencyMs float64
		var lastCheck int64
		if r, ok := h.lastResults[name]; ok {
			latencyMs = r.LatencyMs
			lastCheck = time.Now().Unix() // approximate
		}
		result[name] = api.MonitorStatus{
			Name:      name,
			Type:      h.monitorTypes[name],
			Status:    status,
			LatencyMs: latencyMs,
			LastCheck: lastCheck,
		}
	}
	return result
}

// MonitorStatus returns the current status of a single monitor.
func (h *Hub) MonitorStatus(name string) (api.MonitorStatus, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	ms, ok := h.monitorStates[name]
	if !ok {
		return api.MonitorStatus{}, false
	}
	status := "unknown"
	switch ms.Status {
	case StatusUp:
		status = "up"
	case StatusDown:
		status = "down"
	}
	var latencyMs float64
	var lastCheck int64
	if r, ok := h.lastResults[name]; ok {
		latencyMs = r.LatencyMs
		lastCheck = time.Now().Unix()
	}
	return api.MonitorStatus{
		Name:      name,
		Type:      h.monitorTypes[name],
		Status:    status,
		LatencyMs: latencyMs,
		LastCheck: lastCheck,
	}, true
}

// ReloadConfig re-reads the YAML config and applies changes.
func (h *Hub) ReloadConfig() error {
	cfg, err := config.Load(h.configPath)
	if err != nil {
		return err
	}
	h.mu.Lock()
	h.cfg = cfg
	h.mu.Unlock()
	log.Printf("hub: config reloaded")
	return nil
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
	h.mu.Lock()
	ms, ok := h.monitorStates[result.Monitor]
	if !ok {
		h.mu.Unlock()
		log.Printf("hub: unknown monitor %q", result.Monitor)
		return
	}
	transition := ms.RecordResult(result)
	h.lastResults[result.Monitor] = result
	h.mu.Unlock()

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

	// 6. Broadcast to WebSocket clients
	if h.wsHub != nil {
		h.wsHub.Broadcast(api.WSMessage{
			Type: "check_result",
			Data: map[string]interface{}{
				"monitor":    result.Monitor,
				"status":     result.Status,
				"latency_ms": result.LatencyMs,
				"timestamp":  now,
			},
		})
	}

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

// startScrapeTargets launches scrape loops for Prometheus endpoints.
func (h *Hub) startScrapeTargets() {
	if len(h.cfg.ScrapeTargets) == 0 {
		return
	}

	for _, target := range h.cfg.ScrapeTargets {
		go h.runScrapeLoop(target)
	}
}

func (h *Hub) runScrapeLoop(target config.ScrapeTarget) {
	sc := checks.NewScrapeCheck(target.Name, target.URL)
	interval := target.Interval
	if interval == 0 {
		interval = 30 * time.Second
	}

	log.Printf("hub: scraping %s at %s every %s", target.Name, target.URL, interval)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			metrics, err := sc.Execute(context.Background())
			if err != nil {
				log.Printf("hub: scrape %s error: %v", target.Name, err)
				continue
			}

			// Convert to store metrics
			storeMetrics := make([]store.Metric, len(metrics))
			for i, m := range metrics {
				storeMetrics[i] = store.Metric{Name: m.Name, Value: m.Value}
			}

			now := time.Now()
			if err := h.store.InsertAgentMetricsBatch(target.Name, now, storeMetrics); err != nil {
				log.Printf("hub: store scraped metrics for %s: %v", target.Name, err)
			}
			if err := h.store.UpsertHeartbeat(target.Name, now); err != nil {
				log.Printf("hub: update heartbeat for %s: %v", target.Name, err)
			}

			log.Printf("hub: scraped %d metrics from %s", len(metrics), target.Name)

		case <-h.stopCh:
			return
		}
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
