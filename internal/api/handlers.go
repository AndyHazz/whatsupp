package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/andyhazz/whatsupp/internal/version"
	"github.com/go-chi/chi/v5"
)

// Handlers holds dependencies for all API handlers.
type Handlers struct {
	store       Store
	hub         HubState
	configPath  string
	rateLimiter *LoginRateLimiter
	wsHub       *WSHub
	backupDir   string // default: /data/backups
}

// NewHandlers creates a new Handlers instance.
func NewHandlers(store Store, hub HubState, configPath string) *Handlers {
	return &Handlers{
		store:       store,
		hub:         hub,
		configPath:  configPath,
		rateLimiter: NewLoginRateLimiter(),
	}
}

// Health handles GET /api/v1/health — public, no auth.
func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"version": version.Version,
	})
}

// TestNtfy handles POST /api/v1/test-ntfy — sends a test notification.
func (h *Handlers) TestNtfy(w http.ResponseWriter, r *http.Request) {
	if h.hub == nil {
		jsonError(w, "hub not available", http.StatusServiceUnavailable)
		return
	}
	if err := h.hub.SendTestNotification(); err != nil {
		jsonError(w, fmt.Sprintf("notification failed: %v", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "sent"})
}

// ListMonitors handles GET /api/v1/monitors.
func (h *Handlers) ListMonitors(w http.ResponseWriter, r *http.Request) {
	statuses := h.hub.MonitorStatuses()
	result := make([]MonitorStatus, 0, len(statuses))
	for _, s := range statuses {
		result = append(result, s)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// GetMonitor handles GET /api/v1/monitors/:name.
func (h *Handlers) GetMonitor(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	status, ok := h.hub.MonitorStatus(name)
	if !ok {
		jsonError(w, "monitor not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// selectCheckResultTier picks the storage tier based on the requested time range.
// Target: ~60-360 data points per monitor at each zoom level.
// <=6h → raw (~60-360 pts at 60s interval), <=7d → hourly (~24-168 pts), >7d → daily
func selectCheckResultTier(duration time.Duration) string {
	switch {
	case duration <= 6*time.Hour:
		return "raw"
	case duration <= 7*24*time.Hour:
		return "hourly"
	default:
		return "daily"
	}
}

// selectAgentMetricTier picks the storage tier based on the requested time range.
// Target: ~72-168 data points per metric at each zoom level.
// <=1h → raw (~120 pts), <=6h → 5min (~72 pts), <=48h → 15min (~96-192 pts),
// <=7d → hourly (~168 pts), >7d → daily
func selectAgentMetricTier(duration time.Duration) string {
	switch {
	case duration <= 1*time.Hour:
		return "raw"
	case duration <= 6*time.Hour:
		return "5min"
	case duration <= 48*time.Hour:
		return "15min"
	case duration <= 7*24*time.Hour:
		return "hourly"
	default:
		return "daily"
	}
}

// parseTimeRange extracts from/to query params (unix epoch seconds).
// Defaults: from = 24h ago, to = now.
func parseTimeRange(r *http.Request) (from, to time.Time) {
	now := time.Now()
	to = now
	from = now.Add(-24 * time.Hour)

	if v := r.URL.Query().Get("from"); v != "" {
		if epoch, err := strconv.ParseInt(v, 10, 64); err == nil {
			from = time.Unix(epoch, 0)
		}
	}
	if v := r.URL.Query().Get("to"); v != "" {
		if epoch, err := strconv.ParseInt(v, 10, 64); err == nil {
			to = time.Unix(epoch, 0)
		}
	}
	return from, to
}

// GetMonitorResults handles GET /api/v1/monitors/:name/results?from=&to=.
func (h *Handlers) GetMonitorResults(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	from, to := parseTimeRange(r)
	duration := to.Sub(from)

	w.Header().Set("Content-Type", "application/json")

	tier := selectCheckResultTier(duration)
	switch tier {
	case "raw":
		results, err := h.store.GetCheckResults(name, from, to)
		if err != nil {
			jsonError(w, "query failed", http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(results)
	case "hourly":
		results, err := h.store.GetCheckResultsHourly(name, from, to)
		if err != nil {
			jsonError(w, "query failed", http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(flattenCheckSummaries(name, results))
	case "daily":
		results, err := h.store.GetCheckResultsDaily(name, from, to)
		if err != nil {
			jsonError(w, "query failed", http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(flattenCheckSummaries(name, results))
	}
}

// ListHosts handles GET /api/v1/hosts.
func (h *Handlers) ListHosts(w http.ResponseWriter, r *http.Request) {
	heartbeats, err := h.store.GetAgentHeartbeats()
	if err != nil {
		jsonError(w, "query failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(heartbeats)
}

// GetHost handles GET /api/v1/hosts/:name.
func (h *Handlers) GetHost(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	hb, err := h.store.GetAgentHeartbeat(name)
	if err != nil {
		jsonError(w, "query failed", http.StatusInternalServerError)
		return
	}
	if hb == nil {
		jsonError(w, "host not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(hb)
}

// GetHostMetrics handles GET /api/v1/hosts/:name/metrics?from=&to=&names=.
func (h *Handlers) GetHostMetrics(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	from, to := parseTimeRange(r)
	duration := to.Sub(from)

	// Parse metric name filter
	var names []string
	if v := r.URL.Query().Get("names"); v != "" {
		names = strings.Split(v, ",")
	}

	w.Header().Set("Content-Type", "application/json")

	tier := selectAgentMetricTier(duration)
	switch tier {
	case "raw":
		results, err := h.store.GetAgentMetrics(name, from, to, names)
		if err != nil {
			jsonError(w, "query failed", http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(results)
	case "5min":
		results, err := h.store.GetAgentMetrics5Min(name, from, to, names)
		if err != nil {
			jsonError(w, "query failed", http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(flattenSummaries(name, results))
	case "15min":
		results, err := h.store.GetAgentMetrics15Min(name, from, to, names)
		if err != nil {
			jsonError(w, "query failed", http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(flattenSummaries(name, results))
	case "hourly":
		results, err := h.store.GetAgentMetricsHourly(name, from, to, names)
		if err != nil {
			jsonError(w, "query failed", http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(flattenSummaries(name, results))
	case "daily":
		results, err := h.store.GetAgentMetricsDaily(name, from, to, names)
		if err != nil {
			jsonError(w, "query failed", http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(flattenSummaries(name, results))
	}
}

// flattenCheckSummaries converts aggregated check result summaries into the same
// shape as raw check results so the frontend chart code works unchanged.
func flattenCheckSummaries(monitor string, summaries []CheckResultSummary) []CheckResult {
	out := make([]CheckResult, len(summaries))
	for i, s := range summaries {
		status := "up"
		if s.FailCount > 0 && s.SuccessCount == 0 {
			status = "down"
		}
		out[i] = CheckResult{
			Monitor:      monitor,
			Timestamp:    s.Bucket,
			Status:       status,
			LatencyMs:    s.AvgLatency,
			SuccessCount: s.SuccessCount,
			FailCount:    s.FailCount,
		}
	}
	return out
}

// flattenSummaries converts aggregated metric summaries into the same shape as
// raw metrics (timestamp + value) so the frontend doesn't need to handle two formats.
func flattenSummaries(host string, summaries []AgentMetricSummary) []AgentMetric {
	out := make([]AgentMetric, len(summaries))
	for i, s := range summaries {
		out[i] = AgentMetric{
			Host:       host,
			Timestamp:  s.Bucket,
			MetricName: s.MetricName,
			Value:      s.Avg,
		}
	}
	return out
}

// ListIncidents handles GET /api/v1/incidents?from=&to=.
func (h *Handlers) ListIncidents(w http.ResponseWriter, r *http.Request) {
	from, to := parseTimeRange(r)
	incidents, err := h.store.GetIncidents(from, to)
	if err != nil {
		jsonError(w, "query failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(incidents)
}

// ListSecurityScans handles GET /api/v1/security/scans.
func (h *Handlers) ListSecurityScans(w http.ResponseWriter, r *http.Request) {
	scans, err := h.store.GetSecurityScans()
	if err != nil {
		jsonError(w, "query failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(scans)
}

// ListSecurityBaselines handles GET /api/v1/security/baselines.
func (h *Handlers) ListSecurityBaselines(w http.ResponseWriter, r *http.Request) {
	baselines, err := h.store.GetSecurityBaselines()
	if err != nil {
		jsonError(w, "query failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(baselines)
}

// AcceptBaseline handles POST /api/v1/security/baselines/:target.
// Copies the latest scan's open ports as the new baseline for that target.
func (h *Handlers) AcceptBaseline(w http.ResponseWriter, r *http.Request) {
	target := chi.URLParam(r, "target")

	// Get the latest scan for this target
	scans, err := h.store.GetSecurityScans()
	if err != nil {
		jsonError(w, "query failed", http.StatusInternalServerError)
		return
	}

	// Find the most recent scan for this target
	var latestScan *SecurityScan
	for i := range scans {
		if scans[i].Target == target {
			if latestScan == nil || scans[i].Timestamp > latestScan.Timestamp {
				latestScan = &scans[i]
			}
		}
	}

	if latestScan == nil {
		jsonError(w, "no scan found for target", http.StatusNotFound)
		return
	}

	err = h.store.UpdateSecurityBaseline(target, latestScan.OpenPortsJSON, time.Now())
	if err != nil {
		jsonError(w, "failed to update baseline", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// GetScanSchedules handles GET /api/v1/security/schedules.
func (h *Handlers) GetScanSchedules(w http.ResponseWriter, r *http.Request) {
	schedules := h.hub.ScanSchedules()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(schedules)
}

// GetConfig handles GET /api/v1/config — returns current YAML config as string.
func (h *Handlers) GetConfig(w http.ResponseWriter, r *http.Request) {
	data, err := os.ReadFile(h.configPath)
	if err != nil {
		jsonError(w, "failed to read config", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/x-yaml")
	w.Write(data)
}

// PutConfig handles PUT /api/v1/config — writes updated YAML and triggers reload.
func (h *Handlers) PutConfig(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1MB limit
	if err != nil {
		jsonError(w, "failed to read request body", http.StatusBadRequest)
		return
	}

	// Basic YAML validation: try parsing with the config package
	// For now, just validate it's not empty
	if len(strings.TrimSpace(string(body))) == 0 {
		jsonError(w, "config body is empty", http.StatusBadRequest)
		return
	}

	// Write to a temp file first, then rename (atomic write)
	tmpPath := h.configPath + ".tmp"
	if err := os.WriteFile(tmpPath, body, 0644); err != nil {
		jsonError(w, "failed to write config", http.StatusInternalServerError)
		return
	}
	if err := os.Rename(tmpPath, h.configPath); err != nil {
		os.Remove(tmpPath)
		jsonError(w, "failed to replace config", http.StatusInternalServerError)
		return
	}

	// Trigger hub reload
	if h.hub != nil {
		if err := h.hub.ReloadConfig(); err != nil {
			// Config was written but reload failed — report but don't revert
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "written",
				"warning": fmt.Sprintf("config written but reload failed: %v", err),
			})
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// Backup handles GET /api/v1/admin/backup.
// Uses SQLite .backup API to create a consistent backup file and returns it for download.
func (h *Handlers) Backup(w http.ResponseWriter, r *http.Request) {
	backupDir := h.backupDir
	if backupDir == "" {
		backupDir = "/data/backups"
	}

	// Ensure backup directory exists
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		jsonError(w, "failed to create backup directory", http.StatusInternalServerError)
		return
	}

	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("whatsupp-backup-%s.db", timestamp)
	destPath := fmt.Sprintf("%s/%s", backupDir, filename)

	if err := h.store.Backup(destPath); err != nil {
		jsonError(w, fmt.Sprintf("backup failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	http.ServeFile(w, r, destPath)
}

// changePasswordRequest is the JSON body for POST /auth/change-password.
type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// ChangePassword handles POST /api/v1/auth/change-password.
func (h *Handlers) ChangePassword(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	var req changePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.CurrentPassword == "" || req.NewPassword == "" {
		jsonError(w, "current_password and new_password are required", http.StatusBadRequest)
		return
	}

	// Get user ID from session context
	userID, ok := r.Context().Value(userIDKey).(int64)
	if !ok {
		jsonError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	user, err := h.store.GetUserByID(userID)
	if err != nil || user == nil {
		jsonError(w, "user not found", http.StatusInternalServerError)
		return
	}

	if !CheckPassword(user.PasswordHash, req.CurrentPassword) {
		jsonError(w, "current password is incorrect", http.StatusUnauthorized)
		return
	}

	hash, err := HashPassword(req.NewPassword)
	if err != nil {
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}

	if err := h.store.UpdateUserPassword(user.ID, hash); err != nil {
		jsonError(w, "failed to update password", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// agentMetricsPayload is the JSON body for POST /agent/metrics.
type agentMetricsPayload struct {
	Host      string             `json:"host"`
	Timestamp string             `json:"timestamp"` // RFC3339
	Metrics   []AgentMetricPoint `json:"metrics"`
	Version   string             `json:"version,omitempty"`
}

// PostAgentMetrics handles POST /api/v1/agent/metrics.
// Authenticated via AgentKeyAuth middleware (bearer token).
func (h *Handlers) PostAgentMetrics(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB limit for metrics payload
	var payload agentMetricsPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if payload.Host == "" || len(payload.Metrics) == 0 {
		jsonError(w, "host and metrics are required", http.StatusBadRequest)
		return
	}

	// Verify agent is pushing metrics for its own authenticated host
	if authedHost, ok := r.Context().Value(agentHostKey).(string); ok && authedHost != "" {
		if payload.Host != authedHost {
			jsonError(w, "agent key does not match host", http.StatusForbidden)
			return
		}
	}

	ts, err := time.Parse(time.RFC3339, payload.Timestamp)
	if err != nil {
		// Fall back to current time if timestamp is invalid
		ts = time.Now()
	}

	// Store metrics
	if err := h.store.InsertAgentMetrics(payload.Host, ts, payload.Metrics); err != nil {
		jsonError(w, "failed to store metrics", http.StatusInternalServerError)
		return
	}

	// Update heartbeat
	if err := h.store.UpdateAgentHeartbeat(payload.Host, ts); err != nil {
		// Non-fatal — log but don't fail the request
		_ = err
	}
	if payload.Version != "" {
		if err := h.store.UpdateAgentVersion(payload.Host, payload.Version); err != nil {
			_ = err
		}
	}

	// Broadcast to WebSocket clients
	if h.wsHub != nil {
		h.wsHub.Broadcast(WSMessage{
			Type: "agent_metric",
			Data: map[string]interface{}{
				"host":      payload.Host,
				"metrics":   payload.Metrics,
				"timestamp": ts.Unix(),
				"version":   payload.Version,
			},
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// ListMutes handles GET /api/v1/mutes.
func (h *Handlers) ListMutes(w http.ResponseWriter, r *http.Request) {
	muted, err := h.store.GetMutedNames()
	if err != nil {
		jsonError(w, "query failed", http.StatusInternalServerError)
		return
	}
	names := make([]string, 0, len(muted))
	for name := range muted {
		names = append(names, name)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(names)
}

// ToggleMute handles PUT /api/v1/mutes/:name — toggles mute state.
func (h *Handlers) ToggleMute(w http.ResponseWriter, r *http.Request) {
	name, _ := url.PathUnescape(chi.URLParam(r, "name"))

	muted, err := h.store.GetMutedNames()
	if err != nil {
		jsonError(w, "query failed", http.StatusInternalServerError)
		return
	}

	nowMuted := false
	if muted[name] {
		if err := h.store.RemoveMute(name); err != nil {
			jsonError(w, "unmute failed", http.StatusInternalServerError)
			return
		}
		h.hub.UnmuteAlerts(name)
	} else {
		if err := h.store.SetMute(name); err != nil {
			jsonError(w, "mute failed", http.StatusInternalServerError)
			return
		}
		h.hub.MuteAlerts(name)
		nowMuted = true
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"name":  name,
		"muted": nowMuted,
	})
}
