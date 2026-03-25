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

// ScanState tracks the progress of an active security scan.
type ScanState struct {
	Scanned int
	Total   int
}

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
	monitorURLs      map[string]string // monitor name -> service URL (for linking)
	monitorGroups     map[string]string // monitor name -> agent host (for grouping)
	monitorIntervals map[string]int   // monitor name -> check interval in seconds
	lastResults     map[string]checks.Result
	lastCheckAt     map[string]int64 // monitor name -> unix timestamp of last check
	resultCh        chan checks.Result
	stopCh          chan struct{}
	apiServer       *http.Server
	wsHub           *api.WSHub
	scanNextRun     map[string]int64      // target -> next run unix timestamp
	scanProgress    map[string]*ScanState // target -> active scan state (nil if not scanning)
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
		Token:            cfg.Alerting.Ntfy.Token,
		ReminderInterval: cfg.Alerting.Thresholds.DownReminderInterval,
	})

	resultCh := make(chan checks.Result, 100)

	// Initialize monitor states, type map, and URL map
	states := make(map[string]*MonitorState)
	monTypes := make(map[string]string)
	monURLs := make(map[string]string)
	monGroups := make(map[string]string)
	monIntervals := make(map[string]int)
	for _, m := range cfg.Monitors {
		threshold := m.FailureThreshold
		if threshold == 0 {
			threshold = cfg.Alerting.DefaultFailureThreshold
		}
		states[m.Name] = NewMonitorState(m.Name, threshold)
		monTypes[m.Name] = m.Type
		if m.URL != "" {
			monURLs[m.Name] = m.URL
		}
		if m.Group != "" {
			monGroups[m.Name] = m.Group
		}
		monIntervals[m.Name] = int(m.Interval.Seconds())
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
		case "dns":
			checker = &checks.DNSChecker{Host: m.Host, Port: m.Port, Query: m.Query, Timeout: 10}
		}
		if checker != nil {
			sched.RegisterChecker(m.Name, checker)
		}
	}

	return &Hub{
		cfg:             cfg,
		configPath:      configPath,
		store:           s,
		alerter:         ntfyClient,
		scheduler:       sched,
		downsampler:     NewDownsampler(s),
		incidentManager: NewIncidentManager(s),
		monitorStates:   states,
		monitorTypes:     monTypes,
		monitorURLs:      monURLs,
		monitorGroups:     monGroups,
		monitorIntervals: monIntervals,
		lastResults:     make(map[string]checks.Result),
		lastCheckAt:     make(map[string]int64),
		resultCh:        resultCh,
		stopCh:          make(chan struct{}),
		scanNextRun:     make(map[string]int64),
		scanProgress:    make(map[string]*ScanState),
	}, nil
}

// resolveStaleIncidents resolves any open incidents for monitors that no longer exist
// or were fixed by config changes (e.g. insecure_skip_verify, HTTP status tolerance).
func (h *Hub) resolveStaleIncidents() {
	now := time.Now().Unix()
	// Only fetch open incidents — avoids scanning the entire history
	openIncidents, err := h.store.GetAllOpenIncidents()
	if err != nil {
		log.Printf("hub: resolve stale incidents error: %v", err)
		return
	}
	for _, inc := range openIncidents {
		// Resolve if monitor no longer exists or is UP
		h.mu.RLock()
		ms, exists := h.monitorStates[inc.Monitor]
		h.mu.RUnlock()
		if !exists || ms.Status == StatusUp {
			if err := h.store.ResolveIncident(inc.ID, now); err != nil {
				log.Printf("hub: resolve stale incident %d error: %v", inc.ID, err)
			} else {
				log.Printf("hub: resolved stale incident #%d for %s", inc.ID, inc.Monitor)
			}
		}
	}
}

// Run starts the hub: scheduler, result processor, downsampler, API server.
func (h *Hub) Run() error {
	log.Printf("hub: starting with %d monitors", len(h.cfg.Monitors))

	// Load alert mutes from DB
	if muted, err := h.store.GetMutedNames(); err != nil {
		log.Printf("hub: load mutes error: %v", err)
	} else {
		h.alerter.SetMuted(muted)
		log.Printf("hub: loaded %d muted alerts", len(muted))
	}

	// Resolve any stale incidents from previous runs
	h.resolveStaleIncidents()

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

	// Ensure initial admin user exists, then clear plaintext password from memory
	if err := api.EnsureAdminUser(adapter, h.cfg.Auth.InitialUsername, h.cfg.Auth.InitialPassword); err != nil {
		return fmt.Errorf("ensuring admin user: %w", err)
	}
	h.cfg.Auth.InitialPassword = ""

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

type certMeta struct {
	CertDaysLeft *int `json:"cert_days_left,omitempty"`
}

// MonitorStatuses returns the current status of all monitors.
func (h *Hub) MonitorStatuses() map[string]api.MonitorStatus {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Fetch 24h uptime from DB
	uptimes, _ := h.store.Get24hUptimeAll()

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
		var certDays *int
		if r, ok := h.lastResults[name]; ok {
			latencyMs = r.LatencyMs
			if ts, ok := h.lastCheckAt[name]; ok {
				lastCheck = ts
			}
			// Extract cert days from metadata
			if r.MetadataJSON != "" {
				var cm certMeta
				if json.Unmarshal([]byte(r.MetadataJSON), &cm) == nil {
					certDays = cm.CertDaysLeft
				}
			}
		}
		uptimePct := ms.UptimePct() // fallback
		if u, ok := uptimes[name]; ok {
			uptimePct = u
		}
		result[name] = api.MonitorStatus{
			Name:         name,
			Type:         h.monitorTypes[name],
			Status:       status,
			LatencyMs:    latencyMs,
			LastCheck:    lastCheck,
			UptimePct:    uptimePct,
			URL:          h.monitorURLs[name],
			Group:        h.monitorGroups[name],
			Interval:     h.monitorIntervals[name],
			CertDaysLeft: certDays,
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

	uptimes, _ := h.store.Get24hUptimeAll()

	status := "unknown"
	switch ms.Status {
	case StatusUp:
		status = "up"
	case StatusDown:
		status = "down"
	}
	var latencyMs float64
	var lastCheck int64
	var certDays *int
	if r, ok := h.lastResults[name]; ok {
		latencyMs = r.LatencyMs
		if ts, ok := h.lastCheckAt[name]; ok {
			lastCheck = ts
		}
		if r.MetadataJSON != "" {
			var cm certMeta
			if json.Unmarshal([]byte(r.MetadataJSON), &cm) == nil {
				certDays = cm.CertDaysLeft
			}
		}
	}
	uptimePct := ms.UptimePct()
	if u, ok := uptimes[name]; ok {
		uptimePct = u
	}
	return api.MonitorStatus{
		Name:         name,
		Type:         h.monitorTypes[name],
		Status:       status,
		LatencyMs:    latencyMs,
		LastCheck:    lastCheck,
		UptimePct:    uptimePct,
		URL:          h.monitorURLs[name],
		Group:        h.monitorGroups[name],
		Interval:     h.monitorIntervals[name],
		CertDaysLeft: certDays,
	}, true
}

// SendTestNotification sends a test alert via ntfy.
func (h *Hub) SendTestNotification() error {
	return h.alerter.SendTest()
}

// MuteAlerts mutes notifications for the given name.
func (h *Hub) MuteAlerts(name string) {
	h.alerter.Mute(name)
}

// UnmuteAlerts unmutes notifications for the given name.
func (h *Hub) UnmuteAlerts(name string) {
	h.alerter.Unmute(name)
}

// ReloadConfig re-reads the YAML config and applies changes.
// Restarts the scheduler with updated monitors.
func (h *Hub) ReloadConfig() error {
	cfg, err := config.Load(h.configPath)
	if err != nil {
		return err
	}

	// Stop the old scheduler
	h.scheduler.Stop()

	h.mu.Lock()
	h.cfg = cfg

	// Rebuild monitor states — keep existing state for monitors that still exist
	newStates := make(map[string]*MonitorState)
	newTypes := make(map[string]string)
	newURLs := make(map[string]string)
	newGroups := make(map[string]string)
	newIntervals := make(map[string]int)
	for _, m := range cfg.Monitors {
		threshold := m.FailureThreshold
		if threshold == 0 {
			threshold = cfg.Alerting.DefaultFailureThreshold
		}
		if existing, ok := h.monitorStates[m.Name]; ok {
			// Preserve existing state (UP/DOWN, failure count)
			existing.FailureThreshold = threshold
			newStates[m.Name] = existing
		} else {
			newStates[m.Name] = NewMonitorState(m.Name, threshold)
		}
		newTypes[m.Name] = m.Type
		if m.URL != "" {
			newURLs[m.Name] = m.URL
		}
		if m.Group != "" {
			newGroups[m.Name] = m.Group
		}
		newIntervals[m.Name] = int(m.Interval.Seconds())
	}
	h.monitorStates = newStates
	h.monitorTypes = newTypes
	h.monitorURLs = newURLs
	h.monitorGroups = newGroups
	h.monitorIntervals = newIntervals

	// Clean up lastResults and lastCheckAt for removed monitors
	for name := range h.lastResults {
		if _, exists := newStates[name]; !exists {
			delete(h.lastResults, name)
			delete(h.lastCheckAt, name)
		}
	}
	h.mu.Unlock()

	// Create and start new scheduler
	sched := NewScheduler(cfg.Monitors, h.resultCh)
	for _, m := range cfg.Monitors {
		var checker Checker
		switch m.Type {
		case "http":
			checker = &checks.HTTPChecker{URL: m.URL, Timeout: 10, InsecureSkipVerify: m.InsecureSkipVerify}
		case "ping":
			checker = &checks.PingChecker{Host: m.Host, Count: 3, Timeout: 10}
		case "port":
			checker = &checks.PortChecker{Host: m.Host, Port: m.Port, Timeout: 10}
		case "dns":
			checker = &checks.DNSChecker{Host: m.Host, Port: m.Port, Query: m.Query, Timeout: 10}
		}
		if checker != nil {
			sched.RegisterChecker(m.Name, checker)
		}
	}
	h.scheduler = sched
	h.scheduler.Start()

	log.Printf("hub: config reloaded, %d monitors active", len(cfg.Monitors))
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
	h.lastCheckAt[result.Monitor] = time.Now().Unix()
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
		go h.runSecurityScanSchedule(target)
	}
}

func (h *Hub) runSecurityScanSchedule(target config.SecurityTarget) {
	// Run once immediately on startup
	h.executeSecurityScan(target)

	// If no schedule configured, don't repeat
	if target.Schedule == "" {
		return
	}

	for {
		nextRun, err := nextCronTime(target.Schedule, time.Now())
		if err != nil {
			log.Printf("hub: invalid cron schedule for %s: %v", target.Host, err)
			return
		}

		// Store next run time
		h.mu.Lock()
		h.scanNextRun[target.Host] = nextRun.Unix()
		h.mu.Unlock()

		// Broadcast schedule update
		if h.wsHub != nil {
			h.wsHub.Broadcast(api.WSMessage{
				Type: "security_scan_scheduled",
				Data: map[string]interface{}{
					"target":   target.Host,
					"next_run": nextRun.Unix(),
				},
			})
		}

		// Wait until next run
		timer := time.NewTimer(time.Until(nextRun))
		select {
		case <-timer.C:
			h.executeSecurityScan(target)
		case <-h.stopCh:
			timer.Stop()
			return
		}
	}
}

func (h *Hub) executeSecurityScan(target config.SecurityTarget) {
	scanner := &checks.SecurityScanner{
		Host:        target.Host,
		Concurrency: target.ScanConcurrency,
		Timeout:     int(target.Timeout.Seconds()),
		ProgressFn: func(scanned, total int) {
			h.mu.Lock()
			h.scanProgress[target.Host] = &ScanState{Scanned: scanned, Total: total}
			h.mu.Unlock()

			if h.wsHub != nil {
				h.wsHub.Broadcast(api.WSMessage{
					Type: "security_scan_progress",
					Data: map[string]interface{}{
						"target":  target.Host,
						"scanned": scanned,
						"total":   total,
					},
				})
			}
		},
	}

	log.Printf("hub: security scan of %s starting", target.Host)

	// Broadcast scan start
	if h.wsHub != nil {
		h.wsHub.Broadcast(api.WSMessage{
			Type: "security_scan_start",
			Data: map[string]interface{}{
				"target": target.Host,
				"total":  65535,
			},
		})
	}

	openPorts, err := scanner.Scan()

	// Clear progress
	h.mu.Lock()
	delete(h.scanProgress, target.Host)
	h.mu.Unlock()

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

	var newPorts, gonePorts []int
	if baseline == nil {
		if err := h.store.UpsertSecurityBaseline(target.Host, string(portsJSON), now); err != nil {
			log.Printf("hub: set initial baseline error: %v", err)
		}
		log.Printf("hub: security baseline set for %s: %v", target.Host, openPorts)
	} else {
		var baselinePorts []int
		json.Unmarshal([]byte(baseline.ExpectedPortsJSON), &baselinePorts)
		newPorts, gonePorts = checks.CompareBaseline(baselinePorts, openPorts)
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
	}

	// Broadcast scan complete
	if h.wsHub != nil {
		h.wsHub.Broadcast(api.WSMessage{
			Type: "security_scan_complete",
			Data: map[string]interface{}{
				"target":     target.Host,
				"timestamp":  now,
				"open_ports": openPorts,
				"new_ports":  len(newPorts),
				"gone_ports": len(gonePorts),
			},
		})
	}

	log.Printf("hub: security scan of %s complete: %d open ports, %d new, %d gone",
		target.Host, len(openPorts), len(newPorts), len(gonePorts))
}

// ScanSchedules returns the next run time and progress for all security targets.
func (h *Hub) ScanSchedules() map[string]api.ScanSchedule {
	h.mu.RLock()
	defer h.mu.RUnlock()
	result := make(map[string]api.ScanSchedule)
	for _, t := range h.cfg.Security.Targets {
		ss := api.ScanSchedule{
			Target:   t.Host,
			Schedule: t.Schedule,
		}
		if nextRun, ok := h.scanNextRun[t.Host]; ok {
			ss.NextRun = nextRun
		}
		if prog, ok := h.scanProgress[t.Host]; ok && prog != nil {
			ss.Scanning = true
			ss.Scanned = prog.Scanned
			ss.Total = prog.Total
		}
		result[t.Host] = ss
	}
	return result
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
