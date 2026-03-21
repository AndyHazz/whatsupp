# WhatsUpp Plan 1: Core + Checks + Alerting

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox syntax for tracking.

**Goal:** Working monitoring hub that loads config, runs HTTP/ping/port checks on schedule, stores results in SQLite, manages incidents, sends ntfy alerts, and runs downsampling.

**Architecture:** Single Go binary (`whatsupp serve`), reads YAML config, schedules checks via goroutines, stores results in SQLite WAL, manages UP/DOWN state machine with incident tracking, sends alerts to ntfy with deduplication.

**Tech Stack:** Go 1.22+, SQLite (mattn/go-sqlite3 or modernc.org/sqlite), gopkg.in/yaml.v3, pro-bing (for ICMP ping), robfig/cron/v3 (for security scan scheduling)

---

## Task 1: Go Module Init + Project Scaffolding

**Files:** Create `go.mod`, `cmd/whatsupp/main.go`, directory tree

### Steps

- [ ] **1.1** Create the project directory structure:
  ```bash
  cd /home/andyhazz/projects/whatsupp
  mkdir -p cmd/whatsupp
  mkdir -p internal/{hub,checks,store,alerting,config,api,agent}
  ```

- [ ] **1.2** Initialize Go module:
  ```bash
  cd /home/andyhazz/projects/whatsupp
  go mod init github.com/andyhazz/whatsupp
  ```

- [ ] **1.3** Create a minimal `cmd/whatsupp/main.go` that prints usage:
  ```go
  // cmd/whatsupp/main.go
  package main

  import (
      "fmt"
      "os"
  )

  func main() {
      if len(os.Args) < 2 {
          fmt.Fprintf(os.Stderr, "Usage: whatsupp <serve|agent>\n")
          os.Exit(1)
      }

      switch os.Args[1] {
      case "serve":
          fmt.Println("whatsupp hub starting...")
          // Will be wired up in Task 14
      default:
          fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
          os.Exit(1)
      }
  }
  ```

- [ ] **1.4** Verify it compiles and runs:
  ```bash
  cd /home/andyhazz/projects/whatsupp
  go build ./cmd/whatsupp/
  ./whatsupp
  ./whatsupp serve
  ```
  Expected: first prints usage to stderr and exits 1, second prints "whatsupp hub starting..."

- [ ] **1.5** Create `.gitignore`:
  ```
  whatsupp
  *.db
  *.db-wal
  *.db-shm
  .env
  ```

- [ ] **1.6** Initialize git and commit:
  ```bash
  cd /home/andyhazz/projects/whatsupp
  git init
  git add go.mod cmd/ internal/ .gitignore
  git commit -m "scaffold: init Go module and project structure"
  ```

---

## Task 2: Config Loading (YAML Parsing + Env Var Expansion)

**Files:** Create `internal/config/config.go`, `internal/config/config_test.go`

### Steps

- [ ] **2.1** Write the test file first (`internal/config/config_test.go`):
  ```go
  package config

  import (
      "os"
      "path/filepath"
      "testing"
      "time"
  )

  func TestExpandEnvVars(t *testing.T) {
      os.Setenv("TEST_URL", "https://example.com")
      defer os.Unsetenv("TEST_URL")

      input := "url: ${TEST_URL}"
      got := expandEnvVars(input)
      want := "url: https://example.com"
      if got != want {
          t.Errorf("expandEnvVars() = %q, want %q", got, want)
      }
  }

  func TestExpandEnvVars_Missing(t *testing.T) {
      input := "url: ${MISSING_VAR_XYZ}"
      got := expandEnvVars(input)
      want := "url: "
      if got != want {
          t.Errorf("expandEnvVars() for missing var = %q, want %q", got, want)
      }
  }

  func TestLoadConfig_Minimal(t *testing.T) {
      yaml := `
  server:
    listen: ":8080"
    db_path: "/tmp/test.db"

  monitors:
    - name: "Test Site"
      type: http
      url: "https://example.com"
      interval: 60s
      failure_threshold: 3

  alerting:
    default_failure_threshold: 3
    ntfy:
      url: "https://ntfy.example.com"
      topic: "test"
    thresholds:
      ssl_expiry_days: [14, 7, 3, 1]
      disk_usage_pct: 90
      disk_hysteresis_pct: 5
      down_reminder_interval: "1h"
  `
      dir := t.TempDir()
      path := filepath.Join(dir, "config.yml")
      os.WriteFile(path, []byte(yaml), 0644)

      cfg, err := Load(path)
      if err != nil {
          t.Fatalf("Load() error: %v", err)
      }
      if cfg.Server.Listen != ":8080" {
          t.Errorf("Listen = %q, want %q", cfg.Server.Listen, ":8080")
      }
      if len(cfg.Monitors) != 1 {
          t.Fatalf("len(Monitors) = %d, want 1", len(cfg.Monitors))
      }
      m := cfg.Monitors[0]
      if m.Name != "Test Site" {
          t.Errorf("Name = %q, want %q", m.Name, "Test Site")
      }
      if m.Type != "http" {
          t.Errorf("Type = %q, want %q", m.Type, "http")
      }
      if m.URL != "https://example.com" {
          t.Errorf("URL = %q, want %q", m.URL, "https://example.com")
      }
      if m.Interval != 60*time.Second {
          t.Errorf("Interval = %v, want 60s", m.Interval)
      }
      if m.FailureThreshold != 3 {
          t.Errorf("FailureThreshold = %d, want 3", m.FailureThreshold)
      }
  }

  func TestLoadConfig_EnvExpansion(t *testing.T) {
      os.Setenv("TEST_NTFY_URL", "https://ntfy.test.com")
      os.Setenv("TEST_NTFY_TOPIC", "alerts")
      defer os.Unsetenv("TEST_NTFY_URL")
      defer os.Unsetenv("TEST_NTFY_TOPIC")

      yaml := `
  server:
    listen: ":8080"
    db_path: "/tmp/test.db"

  alerting:
    default_failure_threshold: 3
    ntfy:
      url: "${TEST_NTFY_URL}"
      topic: "${TEST_NTFY_TOPIC}"
    thresholds:
      ssl_expiry_days: [14, 7, 3, 1]
      disk_usage_pct: 90
      disk_hysteresis_pct: 5
      down_reminder_interval: "1h"
  `
      dir := t.TempDir()
      path := filepath.Join(dir, "config.yml")
      os.WriteFile(path, []byte(yaml), 0644)

      cfg, err := Load(path)
      if err != nil {
          t.Fatalf("Load() error: %v", err)
      }
      if cfg.Alerting.Ntfy.URL != "https://ntfy.test.com" {
          t.Errorf("Ntfy.URL = %q, want %q", cfg.Alerting.Ntfy.URL, "https://ntfy.test.com")
      }
      if cfg.Alerting.Ntfy.Topic != "alerts" {
          t.Errorf("Ntfy.Topic = %q, want %q", cfg.Alerting.Ntfy.Topic, "alerts")
      }
  }

  func TestLoadConfig_AllMonitorTypes(t *testing.T) {
      yaml := `
  server:
    listen: ":8080"
    db_path: "/tmp/test.db"

  monitors:
    - name: "Web"
      type: http
      url: "https://example.com"
      interval: 60s

    - name: "Gateway"
      type: ping
      host: "10.0.0.1"
      interval: 60s

    - name: "Game"
      type: port
      host: "game.example.com"
      port: 25565
      interval: 120s

  alerting:
    default_failure_threshold: 3
    ntfy:
      url: "https://ntfy.example.com"
      topic: "test"
    thresholds:
      ssl_expiry_days: [14, 7, 3, 1]
      disk_usage_pct: 90
      disk_hysteresis_pct: 5
      down_reminder_interval: "1h"
  `
      dir := t.TempDir()
      path := filepath.Join(dir, "config.yml")
      os.WriteFile(path, []byte(yaml), 0644)

      cfg, err := Load(path)
      if err != nil {
          t.Fatalf("Load() error: %v", err)
      }
      if len(cfg.Monitors) != 3 {
          t.Fatalf("len(Monitors) = %d, want 3", len(cfg.Monitors))
      }
      if cfg.Monitors[1].Host != "10.0.0.1" {
          t.Errorf("ping host = %q, want %q", cfg.Monitors[1].Host, "10.0.0.1")
      }
      if cfg.Monitors[2].Port != 25565 {
          t.Errorf("port = %d, want %d", cfg.Monitors[2].Port, 25565)
      }
  }

  func TestLoadConfig_SecurityTargets(t *testing.T) {
      yaml := `
  server:
    listen: ":8080"
    db_path: "/tmp/test.db"

  security:
    targets:
      - host: "203.0.113.1"
        schedule: "0 3 * * 0"
        scan_concurrency: 200
        timeout: "2s"

  alerting:
    default_failure_threshold: 3
    ntfy:
      url: "https://ntfy.example.com"
      topic: "test"
    thresholds:
      ssl_expiry_days: [14, 7, 3, 1]
      disk_usage_pct: 90
      disk_hysteresis_pct: 5
      down_reminder_interval: "1h"
  `
      dir := t.TempDir()
      path := filepath.Join(dir, "config.yml")
      os.WriteFile(path, []byte(yaml), 0644)

      cfg, err := Load(path)
      if err != nil {
          t.Fatalf("Load() error: %v", err)
      }
      if len(cfg.Security.Targets) != 1 {
          t.Fatalf("len(Security.Targets) = %d, want 1", len(cfg.Security.Targets))
      }
      tgt := cfg.Security.Targets[0]
      if tgt.Host != "203.0.113.1" {
          t.Errorf("Host = %q, want %q", tgt.Host, "203.0.113.1")
      }
      if tgt.ScanConcurrency != 200 {
          t.Errorf("ScanConcurrency = %d, want 200", tgt.ScanConcurrency)
      }
  }

  func TestLoadConfig_RetentionDefaults(t *testing.T) {
      yaml := `
  server:
    listen: ":8080"
    db_path: "/tmp/test.db"

  alerting:
    default_failure_threshold: 3
    ntfy:
      url: "https://ntfy.example.com"
      topic: "test"
    thresholds:
      ssl_expiry_days: [14, 7, 3, 1]
      disk_usage_pct: 90
      disk_hysteresis_pct: 5
      down_reminder_interval: "1h"
  `
      dir := t.TempDir()
      path := filepath.Join(dir, "config.yml")
      os.WriteFile(path, []byte(yaml), 0644)

      cfg, err := Load(path)
      if err != nil {
          t.Fatalf("Load() error: %v", err)
      }
      // Should have defaults applied
      if cfg.Retention.CheckResultsRaw != 720*time.Hour {
          t.Errorf("CheckResultsRaw = %v, want 720h", cfg.Retention.CheckResultsRaw)
      }
      if cfg.Retention.Hourly != 4320*time.Hour {
          t.Errorf("Hourly = %v, want 4320h", cfg.Retention.Hourly)
      }
  }
  ```

- [ ] **2.2** Run the tests (they will fail — no implementation yet):
  ```bash
  cd /home/andyhazz/projects/whatsupp
  go test ./internal/config/... -v
  ```

- [ ] **2.3** Implement `internal/config/config.go`:
  ```go
  package config

  import (
      "fmt"
      "os"
      "regexp"
      "time"

      "gopkg.in/yaml.v3"
  )

  // Config is the top-level configuration.
  type Config struct {
      Server    ServerConfig    `yaml:"server"`
      Auth      AuthConfig      `yaml:"auth"`
      Monitors  []Monitor       `yaml:"monitors"`
      Agents    []AgentConfig   `yaml:"agents"`
      Security  SecurityConfig  `yaml:"security"`
      Alerting  AlertingConfig  `yaml:"alerting"`
      Retention RetentionConfig `yaml:"retention"`
  }

  type ServerConfig struct {
      Listen string `yaml:"listen"`
      DBPath string `yaml:"db_path"`
  }

  type AuthConfig struct {
      InitialUsername string `yaml:"initial_username"`
      InitialPassword string `yaml:"initial_password"`
  }

  type Monitor struct {
      Name             string        `yaml:"name"`
      Type             string        `yaml:"type"`              // http, ping, port
      URL              string        `yaml:"url,omitempty"`     // for http
      Host             string        `yaml:"host,omitempty"`    // for ping, port
      Port             int           `yaml:"port,omitempty"`    // for port
      Interval         time.Duration `yaml:"interval"`
      FailureThreshold int           `yaml:"failure_threshold,omitempty"`
  }

  type AgentConfig struct {
      Name string `yaml:"name"`
      Key  string `yaml:"key"`
  }

  type SecurityConfig struct {
      Targets []SecurityTarget `yaml:"targets"`
  }

  type SecurityTarget struct {
      Host            string        `yaml:"host"`
      Schedule        string        `yaml:"schedule"`
      ScanConcurrency int           `yaml:"scan_concurrency"`
      Timeout         time.Duration `yaml:"timeout"`
  }

  type AlertingConfig struct {
      DefaultFailureThreshold int              `yaml:"default_failure_threshold"`
      Ntfy                    NtfyConfig       `yaml:"ntfy"`
      Thresholds              ThresholdsConfig `yaml:"thresholds"`
  }

  type NtfyConfig struct {
      URL      string `yaml:"url"`
      Topic    string `yaml:"topic"`
      Username string `yaml:"username,omitempty"`
      Password string `yaml:"password,omitempty"`
  }

  type ThresholdsConfig struct {
      SSLExpiryDays        []int         `yaml:"ssl_expiry_days"`
      DiskUsagePct         int           `yaml:"disk_usage_pct"`
      DiskHysteresisPct    int           `yaml:"disk_hysteresis_pct"`
      DownReminderInterval time.Duration `yaml:"down_reminder_interval"`
  }

  type RetentionConfig struct {
      CheckResultsRaw  time.Duration `yaml:"check_results_raw"`
      AgentMetricsRaw  time.Duration `yaml:"agent_metrics_raw"`
      AgentMetrics5Min time.Duration `yaml:"agent_metrics_5min"`
      Hourly           time.Duration `yaml:"hourly"`
      Daily            time.Duration `yaml:"daily"`
  }

  var envVarRe = regexp.MustCompile(`\$\{([^}]+)\}`)

  // expandEnvVars replaces ${VAR} with the value of the VAR environment variable.
  func expandEnvVars(s string) string {
      return envVarRe.ReplaceAllStringFunc(s, func(match string) string {
          varName := envVarRe.FindStringSubmatch(match)[1]
          return os.Getenv(varName)
      })
  }

  // Load reads and parses a YAML config file, expanding environment variables.
  func Load(path string) (*Config, error) {
      data, err := os.ReadFile(path)
      if err != nil {
          return nil, fmt.Errorf("read config: %w", err)
      }

      expanded := expandEnvVars(string(data))

      var cfg Config
      if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
          return nil, fmt.Errorf("parse config: %w", err)
      }

      applyDefaults(&cfg)

      if err := validate(&cfg); err != nil {
          return nil, fmt.Errorf("validate config: %w", err)
      }

      return &cfg, nil
  }

  func applyDefaults(cfg *Config) {
      if cfg.Server.Listen == "" {
          cfg.Server.Listen = ":8080"
      }
      if cfg.Server.DBPath == "" {
          cfg.Server.DBPath = "/data/whatsupp.db"
      }
      if cfg.Alerting.DefaultFailureThreshold == 0 {
          cfg.Alerting.DefaultFailureThreshold = 3
      }
      if cfg.Retention.CheckResultsRaw == 0 {
          cfg.Retention.CheckResultsRaw = 720 * time.Hour // 30 days
      }
      if cfg.Retention.AgentMetricsRaw == 0 {
          cfg.Retention.AgentMetricsRaw = 48 * time.Hour
      }
      if cfg.Retention.AgentMetrics5Min == 0 {
          cfg.Retention.AgentMetrics5Min = 2160 * time.Hour // 90 days
      }
      if cfg.Retention.Hourly == 0 {
          cfg.Retention.Hourly = 4320 * time.Hour // 180 days
      }
      // Daily = 0 means forever (no deletion)

      for i := range cfg.Monitors {
          if cfg.Monitors[i].FailureThreshold == 0 {
              cfg.Monitors[i].FailureThreshold = cfg.Alerting.DefaultFailureThreshold
          }
          if cfg.Monitors[i].Interval == 0 {
              switch cfg.Monitors[i].Type {
              case "http", "ping":
                  cfg.Monitors[i].Interval = 60 * time.Second
              case "port":
                  cfg.Monitors[i].Interval = 120 * time.Second
              }
          }
      }

      for i := range cfg.Security.Targets {
          if cfg.Security.Targets[i].ScanConcurrency == 0 {
              cfg.Security.Targets[i].ScanConcurrency = 200
          }
          if cfg.Security.Targets[i].Timeout == 0 {
              cfg.Security.Targets[i].Timeout = 2 * time.Second
          }
      }

      if cfg.Alerting.Thresholds.DownReminderInterval == 0 {
          cfg.Alerting.Thresholds.DownReminderInterval = time.Hour
      }
      if len(cfg.Alerting.Thresholds.SSLExpiryDays) == 0 {
          cfg.Alerting.Thresholds.SSLExpiryDays = []int{14, 7, 3, 1}
      }
      if cfg.Alerting.Thresholds.DiskUsagePct == 0 {
          cfg.Alerting.Thresholds.DiskUsagePct = 90
      }
      if cfg.Alerting.Thresholds.DiskHysteresisPct == 0 {
          cfg.Alerting.Thresholds.DiskHysteresisPct = 5
      }
  }

  func validate(cfg *Config) error {
      for _, m := range cfg.Monitors {
          if m.Name == "" {
              return fmt.Errorf("monitor missing name")
          }
          switch m.Type {
          case "http":
              if m.URL == "" {
                  return fmt.Errorf("monitor %q: http type requires url", m.Name)
              }
          case "ping":
              if m.Host == "" {
                  return fmt.Errorf("monitor %q: ping type requires host", m.Name)
              }
          case "port":
              if m.Host == "" || m.Port == 0 {
                  return fmt.Errorf("monitor %q: port type requires host and port", m.Name)
              }
          default:
              return fmt.Errorf("monitor %q: unknown type %q", m.Name, m.Type)
          }
      }
      return nil
  }
  ```

- [ ] **2.4** Fetch the yaml.v3 dependency:
  ```bash
  cd /home/andyhazz/projects/whatsupp
  go get gopkg.in/yaml.v3
  ```

- [ ] **2.5** Run tests — all should pass:
  ```bash
  cd /home/andyhazz/projects/whatsupp
  go test ./internal/config/... -v
  ```

- [ ] **2.6** Commit:
  ```bash
  cd /home/andyhazz/projects/whatsupp
  git add internal/config/ go.mod go.sum
  git commit -m "feat: config loading with YAML parsing and env var expansion"
  ```

---

## Task 3: SQLite Store (Schema, Migrations, Writes, Queries, Downsampling)

**Files:** Create `internal/store/store.go`, `internal/store/migrations.go`, `internal/store/queries.go`, `internal/store/store_test.go`

### Steps

- [ ] **3.1** Write `internal/store/store_test.go` with tests for opening the DB and running migrations:
  ```go
  package store

  import (
      "os"
      "path/filepath"
      "testing"
      "time"
  )

  func testDB(t *testing.T) *Store {
      t.Helper()
      dir := t.TempDir()
      path := filepath.Join(dir, "test.db")
      s, err := Open(path)
      if err != nil {
          t.Fatalf("Open() error: %v", err)
      }
      t.Cleanup(func() { s.Close() })
      return s
  }

  func TestOpen(t *testing.T) {
      s := testDB(t)
      // Verify WAL mode
      var journalMode string
      err := s.db.QueryRow("PRAGMA journal_mode").Scan(&journalMode)
      if err != nil {
          t.Fatalf("PRAGMA journal_mode error: %v", err)
      }
      if journalMode != "wal" {
          t.Errorf("journal_mode = %q, want %q", journalMode, "wal")
      }
  }

  func TestInsertCheckResult(t *testing.T) {
      s := testDB(t)
      now := time.Now().Unix()
      err := s.InsertCheckResult("Plex", now, "up", 45.2, `{"status_code":200}`)
      if err != nil {
          t.Fatalf("InsertCheckResult() error: %v", err)
      }

      results, err := s.GetCheckResults("Plex", now-60, now+60)
      if err != nil {
          t.Fatalf("GetCheckResults() error: %v", err)
      }
      if len(results) != 1 {
          t.Fatalf("len(results) = %d, want 1", len(results))
      }
      if results[0].Status != "up" {
          t.Errorf("Status = %q, want %q", results[0].Status, "up")
      }
      if results[0].LatencyMs != 45.2 {
          t.Errorf("LatencyMs = %f, want 45.2", results[0].LatencyMs)
      }
  }

  func TestInsertAndResolveIncident(t *testing.T) {
      s := testDB(t)
      now := time.Now().Unix()
      id, err := s.CreateIncident("Plex", now, "connection refused")
      if err != nil {
          t.Fatalf("CreateIncident() error: %v", err)
      }
      if id == 0 {
          t.Fatal("CreateIncident() returned id 0")
      }

      err = s.ResolveIncident(id, now+300)
      if err != nil {
          t.Fatalf("ResolveIncident() error: %v", err)
      }

      inc, err := s.GetOpenIncident("Plex")
      if err != nil {
          t.Fatalf("GetOpenIncident() error: %v", err)
      }
      if inc != nil {
          t.Error("GetOpenIncident() should return nil after resolve")
      }
  }

  func TestGetOpenIncident(t *testing.T) {
      s := testDB(t)
      now := time.Now().Unix()
      id, err := s.CreateIncident("Plex", now, "timeout")
      if err != nil {
          t.Fatalf("CreateIncident() error: %v", err)
      }

      inc, err := s.GetOpenIncident("Plex")
      if err != nil {
          t.Fatalf("GetOpenIncident() error: %v", err)
      }
      if inc == nil {
          t.Fatal("GetOpenIncident() returned nil, want incident")
      }
      if inc.ID != id {
          t.Errorf("ID = %d, want %d", inc.ID, id)
      }
      if inc.Cause != "timeout" {
          t.Errorf("Cause = %q, want %q", inc.Cause, "timeout")
      }
  }

  func TestInsertSecurityScan(t *testing.T) {
      s := testDB(t)
      now := time.Now().Unix()
      err := s.InsertSecurityScan("203.0.113.1", now, `[22,80,443]`)
      if err != nil {
          t.Fatalf("InsertSecurityScan() error: %v", err)
      }
  }

  func TestSecurityBaseline(t *testing.T) {
      s := testDB(t)
      now := time.Now().Unix()
      err := s.UpsertSecurityBaseline("203.0.113.1", `[22,80,443]`, now)
      if err != nil {
          t.Fatalf("UpsertSecurityBaseline() error: %v", err)
      }

      bl, err := s.GetSecurityBaseline("203.0.113.1")
      if err != nil {
          t.Fatalf("GetSecurityBaseline() error: %v", err)
      }
      if bl == nil {
          t.Fatal("GetSecurityBaseline() returned nil")
      }
      if bl.ExpectedPortsJSON != `[22,80,443]` {
          t.Errorf("ExpectedPortsJSON = %q, want %q", bl.ExpectedPortsJSON, `[22,80,443]`)
      }
  }

  func TestDeleteOldCheckResults(t *testing.T) {
      s := testDB(t)
      now := time.Now().Unix()
      old := now - 86400*31 // 31 days ago

      s.InsertCheckResult("Plex", old, "up", 40.0, "")
      s.InsertCheckResult("Plex", now, "up", 50.0, "")

      cutoff := now - 86400*30 // 30 days
      n, err := s.DeleteOldCheckResults(cutoff)
      if err != nil {
          t.Fatalf("DeleteOldCheckResults() error: %v", err)
      }
      if n != 1 {
          t.Errorf("deleted = %d, want 1", n)
      }

      results, err := s.GetCheckResults("Plex", 0, now+60)
      if err != nil {
          t.Fatalf("GetCheckResults() error: %v", err)
      }
      if len(results) != 1 {
          t.Errorf("remaining results = %d, want 1", len(results))
      }
  }

  func TestAggregateHourly(t *testing.T) {
      s := testDB(t)
      // Insert 3 check results in the same hour
      base := int64(1711018800) // some fixed hour boundary
      s.InsertCheckResult("Plex", base+10, "up", 40.0, "")
      s.InsertCheckResult("Plex", base+20, "up", 60.0, "")
      s.InsertCheckResult("Plex", base+30, "down", 0.0, "")

      err := s.AggregateCheckResultsHourly(base, base+3600)
      if err != nil {
          t.Fatalf("AggregateCheckResultsHourly() error: %v", err)
      }

      rows, err := s.GetCheckResultsHourly("Plex", base, base+3600)
      if err != nil {
          t.Fatalf("GetCheckResultsHourly() error: %v", err)
      }
      if len(rows) != 1 {
          t.Fatalf("len(hourly) = %d, want 1", len(rows))
      }
      r := rows[0]
      if r.SuccessCount != 2 {
          t.Errorf("SuccessCount = %d, want 2", r.SuccessCount)
      }
      if r.FailCount != 1 {
          t.Errorf("FailCount = %d, want 1", r.FailCount)
      }
  }

  func TestDBFileCreated(t *testing.T) {
      dir := t.TempDir()
      path := filepath.Join(dir, "subdir", "test.db")
      // Subdir doesn't exist; Open should create it
      s, err := Open(path)
      if err != nil {
          t.Fatalf("Open() error: %v", err)
      }
      s.Close()

      if _, err := os.Stat(path); os.IsNotExist(err) {
          t.Error("DB file was not created")
      }
  }
  ```

- [ ] **3.2** Run tests (they will fail):
  ```bash
  cd /home/andyhazz/projects/whatsupp
  go test ./internal/store/... -v
  ```

- [ ] **3.3** Implement `internal/store/migrations.go` — full schema from spec:
  ```go
  package store

  const schema = `
  CREATE TABLE IF NOT EXISTS check_results (
      monitor TEXT NOT NULL,
      timestamp INTEGER NOT NULL,
      status TEXT NOT NULL,
      latency_ms REAL,
      metadata_json TEXT
  );
  CREATE INDEX IF NOT EXISTS idx_check_results_monitor_time ON check_results(monitor, timestamp);

  CREATE TABLE IF NOT EXISTS check_results_hourly (
      monitor TEXT NOT NULL,
      hour INTEGER NOT NULL,
      avg_latency REAL,
      min_latency REAL,
      max_latency REAL,
      success_count INTEGER,
      fail_count INTEGER,
      uptime_pct REAL
  );
  CREATE INDEX IF NOT EXISTS idx_check_hourly_monitor_time ON check_results_hourly(monitor, hour);

  CREATE TABLE IF NOT EXISTS check_results_daily (
      monitor TEXT NOT NULL,
      day INTEGER NOT NULL,
      avg_latency REAL,
      min_latency REAL,
      max_latency REAL,
      success_count INTEGER,
      fail_count INTEGER,
      uptime_pct REAL
  );
  CREATE INDEX IF NOT EXISTS idx_check_daily_monitor_time ON check_results_daily(monitor, day);

  CREATE TABLE IF NOT EXISTS incidents (
      id INTEGER PRIMARY KEY,
      monitor TEXT NOT NULL,
      started_at INTEGER NOT NULL,
      resolved_at INTEGER,
      cause TEXT
  );
  CREATE INDEX IF NOT EXISTS idx_incidents_monitor ON incidents(monitor, started_at);

  CREATE TABLE IF NOT EXISTS agent_metrics (
      host TEXT NOT NULL,
      timestamp INTEGER NOT NULL,
      metric_name TEXT NOT NULL,
      value REAL NOT NULL
  );
  CREATE INDEX IF NOT EXISTS idx_agent_metrics_host_time ON agent_metrics(host, timestamp);
  CREATE INDEX IF NOT EXISTS idx_agent_metrics_name_time ON agent_metrics(host, metric_name, timestamp);

  CREATE TABLE IF NOT EXISTS agent_metrics_5min (
      host TEXT NOT NULL,
      bucket INTEGER NOT NULL,
      metric_name TEXT NOT NULL,
      avg REAL,
      min REAL,
      max REAL
  );
  CREATE INDEX IF NOT EXISTS idx_agent_5min_host_time ON agent_metrics_5min(host, metric_name, bucket);

  CREATE TABLE IF NOT EXISTS agent_metrics_hourly (
      host TEXT NOT NULL,
      hour INTEGER NOT NULL,
      metric_name TEXT NOT NULL,
      avg REAL,
      min REAL,
      max REAL
  );
  CREATE INDEX IF NOT EXISTS idx_agent_hourly_host_time ON agent_metrics_hourly(host, metric_name, hour);

  CREATE TABLE IF NOT EXISTS agent_metrics_daily (
      host TEXT NOT NULL,
      day INTEGER NOT NULL,
      metric_name TEXT NOT NULL,
      avg REAL,
      min REAL,
      max REAL
  );
  CREATE INDEX IF NOT EXISTS idx_agent_daily_host_time ON agent_metrics_daily(host, metric_name, day);

  CREATE TABLE IF NOT EXISTS security_scans (
      id INTEGER PRIMARY KEY,
      target TEXT NOT NULL,
      timestamp INTEGER NOT NULL,
      open_ports_json TEXT NOT NULL
  );
  CREATE INDEX IF NOT EXISTS idx_security_scans_target ON security_scans(target, timestamp);

  CREATE TABLE IF NOT EXISTS security_baselines (
      target TEXT PRIMARY KEY,
      expected_ports_json TEXT NOT NULL,
      updated_at INTEGER NOT NULL
  );

  CREATE TABLE IF NOT EXISTS users (
      id INTEGER PRIMARY KEY,
      username TEXT UNIQUE NOT NULL,
      password_hash TEXT NOT NULL
  );

  CREATE TABLE IF NOT EXISTS sessions (
      token TEXT PRIMARY KEY,
      user_id INTEGER REFERENCES users(id),
      expires_at INTEGER NOT NULL
  );

  CREATE TABLE IF NOT EXISTS agent_heartbeats (
      host TEXT PRIMARY KEY,
      last_seen_at INTEGER NOT NULL
  );
  `

  func migrate(db interface{ Exec(string, ...any) (any, error) }) error {
      // We use ExecMulti approach - split isn't needed because SQLite handles
      // multiple statements in a single Exec call.
      return nil
  }
  ```

  **Note:** The `migrate` function is a placeholder; the actual migration is run inside `Open()` using `db.Exec(schema)`.

- [ ] **3.4** Implement `internal/store/store.go`:
  ```go
  package store

  import (
      "database/sql"
      "fmt"
      "os"
      "path/filepath"

      _ "github.com/mattn/go-sqlite3"
  )

  // CheckResult represents a single check result row.
  type CheckResult struct {
      Monitor      string
      Timestamp    int64
      Status       string
      LatencyMs    float64
      MetadataJSON string
  }

  // CheckResultHourly represents an hourly aggregation.
  type CheckResultHourly struct {
      Monitor      string
      Hour         int64
      AvgLatency   float64
      MinLatency   float64
      MaxLatency   float64
      SuccessCount int
      FailCount    int
      UptimePct    float64
  }

  // Incident represents an incident row.
  type Incident struct {
      ID         int64
      Monitor    string
      StartedAt  int64
      ResolvedAt *int64
      Cause      string
  }

  // SecurityBaseline represents a security baseline row.
  type SecurityBaseline struct {
      Target            string
      ExpectedPortsJSON string
      UpdatedAt         int64
  }

  // Store wraps the SQLite database.
  type Store struct {
      db *sql.DB
  }

  // Open creates or opens the SQLite database and runs migrations.
  func Open(path string) (*Store, error) {
      // Ensure parent directory exists
      dir := filepath.Dir(path)
      if err := os.MkdirAll(dir, 0755); err != nil {
          return nil, fmt.Errorf("create db directory: %w", err)
      }

      db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_busy_timeout=5000&_synchronous=NORMAL&_foreign_keys=ON")
      if err != nil {
          return nil, fmt.Errorf("open db: %w", err)
      }

      // Run schema migration
      if _, err := db.Exec(schema); err != nil {
          db.Close()
          return nil, fmt.Errorf("migrate: %w", err)
      }

      return &Store{db: db}, nil
  }

  // Close closes the database.
  func (s *Store) Close() error {
      return s.db.Close()
  }

  // DB returns the underlying *sql.DB for advanced use.
  func (s *Store) DB() *sql.DB {
      return s.db
  }
  ```

- [ ] **3.5** Implement `internal/store/queries.go`:
  ```go
  package store

  import (
      "database/sql"
      "fmt"
  )

  // InsertCheckResult stores a single check result.
  func (s *Store) InsertCheckResult(monitor string, timestamp int64, status string, latencyMs float64, metadataJSON string) error {
      _, err := s.db.Exec(
          `INSERT INTO check_results (monitor, timestamp, status, latency_ms, metadata_json) VALUES (?, ?, ?, ?, ?)`,
          monitor, timestamp, status, latencyMs, metadataJSON,
      )
      return err
  }

  // GetCheckResults returns raw check results for a monitor in a time range.
  func (s *Store) GetCheckResults(monitor string, from, to int64) ([]CheckResult, error) {
      rows, err := s.db.Query(
          `SELECT monitor, timestamp, status, latency_ms, COALESCE(metadata_json, '') FROM check_results WHERE monitor = ? AND timestamp >= ? AND timestamp <= ? ORDER BY timestamp`,
          monitor, from, to,
      )
      if err != nil {
          return nil, err
      }
      defer rows.Close()

      var results []CheckResult
      for rows.Next() {
          var r CheckResult
          if err := rows.Scan(&r.Monitor, &r.Timestamp, &r.Status, &r.LatencyMs, &r.MetadataJSON); err != nil {
              return nil, err
          }
          results = append(results, r)
      }
      return results, rows.Err()
  }

  // CreateIncident inserts a new open incident and returns its ID.
  func (s *Store) CreateIncident(monitor string, startedAt int64, cause string) (int64, error) {
      res, err := s.db.Exec(
          `INSERT INTO incidents (monitor, started_at, cause) VALUES (?, ?, ?)`,
          monitor, startedAt, cause,
      )
      if err != nil {
          return 0, err
      }
      return res.LastInsertId()
  }

  // ResolveIncident sets resolved_at on an incident.
  func (s *Store) ResolveIncident(id int64, resolvedAt int64) error {
      _, err := s.db.Exec(
          `UPDATE incidents SET resolved_at = ? WHERE id = ?`,
          resolvedAt, id,
      )
      return err
  }

  // GetOpenIncident returns the open (unresolved) incident for a monitor, or nil.
  func (s *Store) GetOpenIncident(monitor string) (*Incident, error) {
      row := s.db.QueryRow(
          `SELECT id, monitor, started_at, resolved_at, cause FROM incidents WHERE monitor = ? AND resolved_at IS NULL ORDER BY started_at DESC LIMIT 1`,
          monitor,
      )
      var inc Incident
      var resolvedAt sql.NullInt64
      err := row.Scan(&inc.ID, &inc.Monitor, &inc.StartedAt, &resolvedAt, &inc.Cause)
      if err == sql.ErrNoRows {
          return nil, nil
      }
      if err != nil {
          return nil, err
      }
      if resolvedAt.Valid {
          inc.ResolvedAt = &resolvedAt.Int64
      }
      return &inc, nil
  }

  // InsertSecurityScan stores a security scan result.
  func (s *Store) InsertSecurityScan(target string, timestamp int64, openPortsJSON string) error {
      _, err := s.db.Exec(
          `INSERT INTO security_scans (target, timestamp, open_ports_json) VALUES (?, ?, ?)`,
          target, timestamp, openPortsJSON,
      )
      return err
  }

  // UpsertSecurityBaseline creates or updates the baseline for a target.
  func (s *Store) UpsertSecurityBaseline(target, expectedPortsJSON string, updatedAt int64) error {
      _, err := s.db.Exec(
          `INSERT INTO security_baselines (target, expected_ports_json, updated_at) VALUES (?, ?, ?)
           ON CONFLICT(target) DO UPDATE SET expected_ports_json = excluded.expected_ports_json, updated_at = excluded.updated_at`,
          target, expectedPortsJSON, updatedAt,
      )
      return err
  }

  // GetSecurityBaseline returns the baseline for a target, or nil.
  func (s *Store) GetSecurityBaseline(target string) (*SecurityBaseline, error) {
      row := s.db.QueryRow(
          `SELECT target, expected_ports_json, updated_at FROM security_baselines WHERE target = ?`,
          target,
      )
      var bl SecurityBaseline
      err := row.Scan(&bl.Target, &bl.ExpectedPortsJSON, &bl.UpdatedAt)
      if err == sql.ErrNoRows {
          return nil, nil
      }
      if err != nil {
          return nil, err
      }
      return &bl, nil
  }

  // DeleteOldCheckResults deletes check results older than cutoff and returns count deleted.
  func (s *Store) DeleteOldCheckResults(cutoff int64) (int64, error) {
      res, err := s.db.Exec(`DELETE FROM check_results WHERE timestamp < ?`, cutoff)
      if err != nil {
          return 0, err
      }
      return res.RowsAffected()
  }

  // AggregateCheckResultsHourly aggregates raw check results for the given hour range
  // into check_results_hourly. hourStart and hourEnd define the window [hourStart, hourEnd).
  func (s *Store) AggregateCheckResultsHourly(hourStart, hourEnd int64) error {
      _, err := s.db.Exec(`
          INSERT INTO check_results_hourly (monitor, hour, avg_latency, min_latency, max_latency, success_count, fail_count, uptime_pct)
          SELECT
              monitor,
              ? AS hour,
              AVG(CASE WHEN status = 'up' THEN latency_ms END),
              MIN(CASE WHEN status = 'up' THEN latency_ms END),
              MAX(CASE WHEN status = 'up' THEN latency_ms END),
              SUM(CASE WHEN status = 'up' THEN 1 ELSE 0 END),
              SUM(CASE WHEN status = 'down' THEN 1 ELSE 0 END),
              ROUND(100.0 * SUM(CASE WHEN status = 'up' THEN 1 ELSE 0 END) / COUNT(*), 2)
          FROM check_results
          WHERE timestamp >= ? AND timestamp < ?
          GROUP BY monitor
      `, hourStart, hourStart, hourEnd)
      return err
  }

  // GetCheckResultsHourly returns hourly aggregated results for a monitor.
  func (s *Store) GetCheckResultsHourly(monitor string, from, to int64) ([]CheckResultHourly, error) {
      rows, err := s.db.Query(
          `SELECT monitor, hour, avg_latency, min_latency, max_latency, success_count, fail_count, uptime_pct
           FROM check_results_hourly WHERE monitor = ? AND hour >= ? AND hour <= ? ORDER BY hour`,
          monitor, from, to,
      )
      if err != nil {
          return nil, err
      }
      defer rows.Close()

      var results []CheckResultHourly
      for rows.Next() {
          var r CheckResultHourly
          var avgLat, minLat, maxLat sql.NullFloat64
          if err := rows.Scan(&r.Monitor, &r.Hour, &avgLat, &minLat, &maxLat, &r.SuccessCount, &r.FailCount, &r.UptimePct); err != nil {
              return nil, err
          }
          if avgLat.Valid {
              r.AvgLatency = avgLat.Float64
          }
          if minLat.Valid {
              r.MinLatency = minLat.Float64
          }
          if maxLat.Valid {
              r.MaxLatency = maxLat.Float64
          }
          results = append(results, r)
      }
      return results, rows.Err()
  }

  // AggregateCheckResultsDaily aggregates hourly results for the given day range into daily.
  func (s *Store) AggregateCheckResultsDaily(dayStart, dayEnd int64) error {
      _, err := s.db.Exec(`
          INSERT INTO check_results_daily (monitor, day, avg_latency, min_latency, max_latency, success_count, fail_count, uptime_pct)
          SELECT
              monitor,
              ? AS day,
              AVG(avg_latency),
              MIN(min_latency),
              MAX(max_latency),
              SUM(success_count),
              SUM(fail_count),
              ROUND(100.0 * SUM(success_count) / (SUM(success_count) + SUM(fail_count)), 2)
          FROM check_results_hourly
          WHERE hour >= ? AND hour < ?
          GROUP BY monitor
      `, dayStart, dayStart, dayEnd)
      return err
  }

  // DeleteOldHourlyCheckResults deletes hourly results older than cutoff.
  func (s *Store) DeleteOldHourlyCheckResults(cutoff int64) (int64, error) {
      res, err := s.db.Exec(`DELETE FROM check_results_hourly WHERE hour < ?`, cutoff)
      if err != nil {
          return 0, err
      }
      return res.RowsAffected()
  }

  // GetLastNCheckResults returns the last N check results for a monitor (most recent first).
  func (s *Store) GetLastNCheckResults(monitor string, n int) ([]CheckResult, error) {
      rows, err := s.db.Query(
          `SELECT monitor, timestamp, status, latency_ms, COALESCE(metadata_json, '')
           FROM check_results WHERE monitor = ? ORDER BY timestamp DESC LIMIT ?`,
          monitor, n,
      )
      if err != nil {
          return nil, err
      }
      defer rows.Close()

      var results []CheckResult
      for rows.Next() {
          var r CheckResult
          if err := rows.Scan(&r.Monitor, &r.Timestamp, &r.Status, &r.LatencyMs, &r.MetadataJSON); err != nil {
              return nil, err
          }
          results = append(results, r)
      }
      return results, rows.Err()
  }

  // CountConsecutiveFailures returns the number of consecutive "down" results
  // from the most recent check result backwards. Returns 0 if the latest is "up".
  func (s *Store) CountConsecutiveFailures(monitor string) (int, error) {
      results, err := s.GetLastNCheckResults(monitor, 100) // enough headroom
      if err != nil {
          return 0, err
      }
      count := 0
      for _, r := range results {
          if r.Status != "down" {
              break
          }
          count++
      }
      return count, nil
  }

  // GetIncidents returns incidents in a time range.
  func (s *Store) GetIncidents(from, to int64) ([]Incident, error) {
      rows, err := s.db.Query(
          `SELECT id, monitor, started_at, resolved_at, cause FROM incidents
           WHERE started_at >= ? AND started_at <= ? ORDER BY started_at DESC`,
          from, to,
      )
      if err != nil {
          return nil, err
      }
      defer rows.Close()

      var incidents []Incident
      for rows.Next() {
          var inc Incident
          var resolvedAt sql.NullInt64
          if err := rows.Scan(&inc.ID, &inc.Monitor, &inc.StartedAt, &resolvedAt, &inc.Cause); err != nil {
              return nil, fmt.Errorf("scan incident: %w", err)
          }
          if resolvedAt.Valid {
              inc.ResolvedAt = &resolvedAt.Int64
          }
          incidents = append(incidents, inc)
      }
      return incidents, rows.Err()
  }
  ```

- [ ] **3.6** Fetch the sqlite3 dependency:
  ```bash
  cd /home/andyhazz/projects/whatsupp
  go get github.com/mattn/go-sqlite3
  ```

- [ ] **3.7** Run tests — all should pass:
  ```bash
  cd /home/andyhazz/projects/whatsupp
  go test ./internal/store/... -v
  ```

- [ ] **3.8** Commit:
  ```bash
  cd /home/andyhazz/projects/whatsupp
  git add internal/store/ go.mod go.sum
  git commit -m "feat: SQLite store with schema, migrations, queries, and downsampling"
  ```

---

## Task 4: HTTP Checker

**Files:** Create `internal/checks/http.go`, `internal/checks/http_test.go`, `internal/checks/types.go`

### Steps

- [ ] **4.1** Create the shared types file `internal/checks/types.go`:
  ```go
  package checks

  // Result is the outcome of a single check execution.
  type Result struct {
      Monitor      string
      Status       string  // "up" or "down"
      LatencyMs    float64
      MetadataJSON string  // JSON string with check-specific data
      Error        string  // human-readable error for alerting (empty on success)
  }
  ```

- [ ] **4.2** Write `internal/checks/http_test.go`:
  ```go
  package checks

  import (
      "net/http"
      "net/http/httptest"
      "testing"
  )

  func TestHTTPCheck_Success(t *testing.T) {
      srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
          w.WriteHeader(200)
      }))
      defer srv.Close()

      c := &HTTPChecker{URL: srv.URL, Timeout: 5}
      result := c.Check("TestHTTP")
      if result.Status != "up" {
          t.Errorf("Status = %q, want %q", result.Status, "up")
      }
      if result.LatencyMs <= 0 {
          t.Errorf("LatencyMs = %f, want > 0", result.LatencyMs)
      }
  }

  func TestHTTPCheck_ServerError(t *testing.T) {
      srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
          w.WriteHeader(500)
      }))
      defer srv.Close()

      c := &HTTPChecker{URL: srv.URL, Timeout: 5}
      result := c.Check("TestHTTP")
      if result.Status != "down" {
          t.Errorf("Status = %q, want %q", result.Status, "down")
      }
  }

  func TestHTTPCheck_ConnectionRefused(t *testing.T) {
      c := &HTTPChecker{URL: "http://127.0.0.1:1", Timeout: 2}
      result := c.Check("TestHTTP")
      if result.Status != "down" {
          t.Errorf("Status = %q, want %q", result.Status, "down")
      }
      if result.Error == "" {
          t.Error("Error should be non-empty on connection failure")
      }
  }

  func TestHTTPCheck_SSLMetadata(t *testing.T) {
      // httptest.NewTLSServer provides a self-signed cert
      srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
          w.WriteHeader(200)
      }))
      defer srv.Close()

      c := &HTTPChecker{
          URL:                srv.URL,
          Timeout:            5,
          InsecureSkipVerify: true,
      }
      result := c.Check("TestHTTPS")
      if result.Status != "up" {
          t.Errorf("Status = %q, want %q", result.Status, "up")
      }
      // Metadata should contain cert info
      if result.MetadataJSON == "" {
          t.Error("MetadataJSON should contain SSL cert info for HTTPS")
      }
  }
  ```

- [ ] **4.3** Run tests (they will fail):
  ```bash
  cd /home/andyhazz/projects/whatsupp
  go test ./internal/checks/... -run TestHTTP -v
  ```

- [ ] **4.4** Implement `internal/checks/http.go`:
  ```go
  package checks

  import (
      "crypto/tls"
      "encoding/json"
      "fmt"
      "net/http"
      "strings"
      "time"
  )

  // HTTPChecker performs HTTP/HTTPS checks.
  type HTTPChecker struct {
      URL                string
      Timeout            int  // seconds
      InsecureSkipVerify bool // for self-signed certs
  }

  type httpMetadata struct {
      StatusCode    int    `json:"status_code"`
      CertExpiryAt *int64 `json:"cert_expiry_at,omitempty"` // unix epoch
      CertDaysLeft *int   `json:"cert_days_left,omitempty"`
  }

  // Check performs the HTTP check and returns a Result.
  func (c *HTTPChecker) Check(monitorName string) Result {
      timeout := time.Duration(c.Timeout) * time.Second
      if timeout == 0 {
          timeout = 10 * time.Second
      }

      transport := &http.Transport{
          TLSClientConfig: &tls.Config{
              InsecureSkipVerify: c.InsecureSkipVerify,
          },
      }
      client := &http.Client{
          Timeout:   timeout,
          Transport: transport,
          // Don't follow redirects — we want to see the actual response
          CheckRedirect: func(req *http.Request, via []*http.Request) error {
              if len(via) >= 10 {
                  return fmt.Errorf("too many redirects")
              }
              return nil
          },
      }

      start := time.Now()
      resp, err := client.Get(c.URL)
      latency := float64(time.Since(start).Microseconds()) / 1000.0

      if err != nil {
          return Result{
              Monitor:   monitorName,
              Status:    "down",
              LatencyMs: latency,
              Error:     err.Error(),
          }
      }
      defer resp.Body.Close()

      meta := httpMetadata{StatusCode: resp.StatusCode}

      // Extract SSL cert info if HTTPS
      if resp.TLS != nil && len(resp.TLS.PeerCertificates) > 0 {
          cert := resp.TLS.PeerCertificates[0]
          expiryUnix := cert.NotAfter.Unix()
          daysLeft := int(time.Until(cert.NotAfter).Hours() / 24)
          meta.CertExpiryAt = &expiryUnix
          meta.CertDaysLeft = &daysLeft
      }

      metaJSON, _ := json.Marshal(meta)

      status := "up"
      var errMsg string
      if resp.StatusCode >= 400 {
          status = "down"
          errMsg = fmt.Sprintf("HTTP %d", resp.StatusCode)
      }

      return Result{
          Monitor:      monitorName,
          Status:       status,
          LatencyMs:    latency,
          MetadataJSON: string(metaJSON),
          Error:        errMsg,
      }
  }

  // IsHTTPS returns true if the URL uses HTTPS.
  func (c *HTTPChecker) IsHTTPS() bool {
      return strings.HasPrefix(c.URL, "https://")
  }
  ```

- [ ] **4.5** Run tests — all should pass:
  ```bash
  cd /home/andyhazz/projects/whatsupp
  go test ./internal/checks/... -run TestHTTP -v
  ```

- [ ] **4.6** Commit:
  ```bash
  cd /home/andyhazz/projects/whatsupp
  git add internal/checks/
  git commit -m "feat: HTTP checker with SSL cert metadata extraction"
  ```

---

## Task 5: Ping Checker (ICMP)

**Files:** Create `internal/checks/ping.go`, `internal/checks/ping_test.go`

### Steps

- [ ] **5.1** Write `internal/checks/ping_test.go`:
  ```go
  package checks

  import (
      "testing"
  )

  func TestPingCheck_Localhost(t *testing.T) {
      // This test requires CAP_NET_RAW or root. Skip in CI if needed.
      c := &PingChecker{Host: "127.0.0.1", Count: 3, Timeout: 5}
      result := c.Check("TestPing")
      if result.Status != "up" {
          t.Errorf("Status = %q, want %q (error: %s)", result.Status, "up", result.Error)
      }
      if result.LatencyMs <= 0 {
          t.Logf("LatencyMs = %f (localhost can be very fast)", result.LatencyMs)
      }
  }

  func TestPingCheck_Unreachable(t *testing.T) {
      // 192.0.2.1 is TEST-NET-1, should be unreachable
      c := &PingChecker{Host: "192.0.2.1", Count: 1, Timeout: 2}
      result := c.Check("TestPing")
      if result.Status != "down" {
          t.Errorf("Status = %q, want %q", result.Status, "down")
      }
  }
  ```

- [ ] **5.2** Run tests (they will fail):
  ```bash
  cd /home/andyhazz/projects/whatsupp
  go test ./internal/checks/... -run TestPing -v
  ```

- [ ] **5.3** Fetch the pro-bing dependency:
  ```bash
  cd /home/andyhazz/projects/whatsupp
  go get github.com/prometheus-community/pro-bing
  ```

- [ ] **5.4** Implement `internal/checks/ping.go`:
  ```go
  package checks

  import (
      "encoding/json"
      "fmt"
      "time"

      probing "github.com/prometheus-community/pro-bing"
  )

  // PingChecker performs ICMP ping checks.
  type PingChecker struct {
      Host    string
      Count   int // number of pings
      Timeout int // seconds
  }

  type pingMetadata struct {
      PacketsSent int     `json:"packets_sent"`
      PacketsRecv int     `json:"packets_recv"`
      PacketLoss  float64 `json:"packet_loss_pct"`
      MinRTT      float64 `json:"min_rtt_ms"`
      AvgRTT      float64 `json:"avg_rtt_ms"`
      MaxRTT      float64 `json:"max_rtt_ms"`
  }

  // Check performs the ICMP ping check.
  func (c *PingChecker) Check(monitorName string) Result {
      count := c.Count
      if count == 0 {
          count = 3
      }
      timeout := time.Duration(c.Timeout) * time.Second
      if timeout == 0 {
          timeout = 10 * time.Second
      }

      pinger, err := probing.NewPinger(c.Host)
      if err != nil {
          return Result{
              Monitor: monitorName,
              Status:  "down",
              Error:   fmt.Sprintf("create pinger: %v", err),
          }
      }

      pinger.Count = count
      pinger.Timeout = timeout
      pinger.SetPrivileged(true) // requires CAP_NET_RAW

      err = pinger.Run()
      if err != nil {
          return Result{
              Monitor: monitorName,
              Status:  "down",
              Error:   fmt.Sprintf("ping: %v", err),
          }
      }

      stats := pinger.Statistics()

      meta := pingMetadata{
          PacketsSent: stats.PacketsSent,
          PacketsRecv: stats.PacketsRecv,
          PacketLoss:  stats.PacketLoss,
          MinRTT:      float64(stats.MinRtt.Microseconds()) / 1000.0,
          AvgRTT:      float64(stats.AvgRtt.Microseconds()) / 1000.0,
          MaxRTT:      float64(stats.MaxRtt.Microseconds()) / 1000.0,
      }
      metaJSON, _ := json.Marshal(meta)

      status := "up"
      var errMsg string
      if stats.PacketsRecv == 0 {
          status = "down"
          errMsg = fmt.Sprintf("100%% packet loss to %s", c.Host)
      }

      return Result{
          Monitor:      monitorName,
          Status:       status,
          LatencyMs:    meta.AvgRTT,
          MetadataJSON: string(metaJSON),
          Error:        errMsg,
      }
  }
  ```

- [ ] **5.5** Run tests (may need `sudo` or `CAP_NET_RAW`):
  ```bash
  cd /home/andyhazz/projects/whatsupp
  sudo go test ./internal/checks/... -run TestPing -v
  ```
  If running without root, the localhost test may fail. That is expected — ICMP requires privileges. The test serves as a smoke test for development.

- [ ] **5.6** Commit:
  ```bash
  cd /home/andyhazz/projects/whatsupp
  git add internal/checks/ping.go internal/checks/ping_test.go go.mod go.sum
  git commit -m "feat: ICMP ping checker using pro-bing"
  ```

---

## Task 6: Port Checker (TCP Connect)

**Files:** Create `internal/checks/port.go`, `internal/checks/port_test.go`

### Steps

- [ ] **6.1** Write `internal/checks/port_test.go`:
  ```go
  package checks

  import (
      "net"
      "testing"
  )

  func TestPortCheck_Open(t *testing.T) {
      // Start a TCP listener
      ln, err := net.Listen("tcp", "127.0.0.1:0")
      if err != nil {
          t.Fatalf("Listen() error: %v", err)
      }
      defer ln.Close()

      addr := ln.Addr().(*net.TCPAddr)
      c := &PortChecker{Host: "127.0.0.1", Port: addr.Port, Timeout: 5}
      result := c.Check("TestPort")
      if result.Status != "up" {
          t.Errorf("Status = %q, want %q (error: %s)", result.Status, "up", result.Error)
      }
      if result.LatencyMs <= 0 {
          t.Logf("LatencyMs = %f", result.LatencyMs)
      }
  }

  func TestPortCheck_Closed(t *testing.T) {
      // Port 1 is almost certainly not open on localhost
      c := &PortChecker{Host: "127.0.0.1", Port: 1, Timeout: 2}
      result := c.Check("TestPort")
      if result.Status != "down" {
          t.Errorf("Status = %q, want %q", result.Status, "down")
      }
      if result.Error == "" {
          t.Error("Error should be non-empty for closed port")
      }
  }

  func TestPortCheck_Unreachable(t *testing.T) {
      // 192.0.2.1 is TEST-NET-1
      c := &PortChecker{Host: "192.0.2.1", Port: 80, Timeout: 2}
      result := c.Check("TestPort")
      if result.Status != "down" {
          t.Errorf("Status = %q, want %q", result.Status, "down")
      }
  }
  ```

- [ ] **6.2** Run tests (they will fail):
  ```bash
  cd /home/andyhazz/projects/whatsupp
  go test ./internal/checks/... -run TestPort -v
  ```

- [ ] **6.3** Implement `internal/checks/port.go`:
  ```go
  package checks

  import (
      "encoding/json"
      "fmt"
      "net"
      "time"
  )

  // PortChecker performs TCP connect checks.
  type PortChecker struct {
      Host    string
      Port    int
      Timeout int // seconds
  }

  type portMetadata struct {
      Host string `json:"host"`
      Port int    `json:"port"`
  }

  // Check performs the TCP connect check.
  func (c *PortChecker) Check(monitorName string) Result {
      timeout := time.Duration(c.Timeout) * time.Second
      if timeout == 0 {
          timeout = 10 * time.Second
      }

      addr := fmt.Sprintf("%s:%d", c.Host, c.Port)
      start := time.Now()
      conn, err := net.DialTimeout("tcp", addr, timeout)
      latency := float64(time.Since(start).Microseconds()) / 1000.0

      meta := portMetadata{Host: c.Host, Port: c.Port}
      metaJSON, _ := json.Marshal(meta)

      if err != nil {
          return Result{
              Monitor:      monitorName,
              Status:       "down",
              LatencyMs:    latency,
              MetadataJSON: string(metaJSON),
              Error:        fmt.Sprintf("tcp connect %s: %v", addr, err),
          }
      }
      conn.Close()

      return Result{
          Monitor:      monitorName,
          Status:       "up",
          LatencyMs:    latency,
          MetadataJSON: string(metaJSON),
      }
  }
  ```

- [ ] **6.4** Run tests — all should pass:
  ```bash
  cd /home/andyhazz/projects/whatsupp
  go test ./internal/checks/... -run TestPort -v
  ```

- [ ] **6.5** Commit:
  ```bash
  cd /home/andyhazz/projects/whatsupp
  git add internal/checks/port.go internal/checks/port_test.go
  git commit -m "feat: TCP port checker"
  ```

---

## Task 7: Security Scanner (Full 65535 TCP Connect Scan)

**Files:** Create `internal/checks/security.go`, `internal/checks/security_test.go`

### Steps

- [ ] **7.1** Write `internal/checks/security_test.go`:
  ```go
  package checks

  import (
      "net"
      "testing"
  )

  func TestSecurityScanner_FindsOpenPort(t *testing.T) {
      // Start a TCP listener
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
          PortEnd:     5, // very low ports, unlikely to be open in test env
      }
      // This might find ports or not — we just test it doesn't crash
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
  ```

- [ ] **7.2** Run tests (they will fail):
  ```bash
  cd /home/andyhazz/projects/whatsupp
  go test ./internal/checks/... -run "TestSecurity|TestCompareBaseline" -v
  ```

- [ ] **7.3** Implement `internal/checks/security.go`:
  ```go
  package checks

  import (
      "fmt"
      "net"
      "sort"
      "sync"
      "time"
  )

  // SecurityScanner performs a TCP connect scan of a port range.
  type SecurityScanner struct {
      Host        string
      Concurrency int
      Timeout     int // seconds
      PortStart   int // default 1
      PortEnd     int // default 65535
  }

  // Scan performs the full TCP connect scan and returns a sorted list of open ports.
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
          sem <- struct{}{} // acquire semaphore
          go func(p int) {
              defer wg.Done()
              defer func() { <-sem }() // release semaphore

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

  // CompareBaseline compares a baseline port list with current scan results.
  // Returns (new ports not in baseline, baseline ports no longer open).
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
  ```

- [ ] **7.4** Run tests — all should pass:
  ```bash
  cd /home/andyhazz/projects/whatsupp
  go test ./internal/checks/... -run "TestSecurity|TestCompareBaseline" -v
  ```

- [ ] **7.5** Commit:
  ```bash
  cd /home/andyhazz/projects/whatsupp
  git add internal/checks/security.go internal/checks/security_test.go
  git commit -m "feat: security scanner with full TCP connect scan and baseline comparison"
  ```

---

## Task 8: Monitor State Machine (Consecutive Failure Counting, UP/DOWN Transitions)

**Files:** Create `internal/hub/monitor.go`, `internal/hub/monitor_test.go`

### Steps

- [ ] **8.1** Write `internal/hub/monitor_test.go`:
  ```go
  package hub

  import (
      "testing"

      "github.com/andyhazz/whatsupp/internal/checks"
  )

  func TestMonitorState_TransitionsToDown(t *testing.T) {
      ms := NewMonitorState("Test", 3)

      // 3 consecutive failures should trigger DOWN
      for i := 0; i < 3; i++ {
          transition := ms.RecordResult(checks.Result{
              Monitor: "Test",
              Status:  "down",
              Error:   "connection refused",
          })
          if i < 2 {
              if transition != TransitionNone {
                  t.Errorf("iteration %d: transition = %v, want None", i, transition)
              }
          }
      }
      // After 3rd failure, should transition to DOWN
      if ms.Status != StatusDown {
          t.Errorf("Status = %v, want DOWN", ms.Status)
      }
  }

  func TestMonitorState_TransitionsToUp(t *testing.T) {
      ms := NewMonitorState("Test", 3)
      // Force to DOWN state
      for i := 0; i < 3; i++ {
          ms.RecordResult(checks.Result{Monitor: "Test", Status: "down", Error: "timeout"})
      }
      if ms.Status != StatusDown {
          t.Fatalf("Status = %v, want DOWN after 3 failures", ms.Status)
      }

      // One success should bring it back UP
      transition := ms.RecordResult(checks.Result{Monitor: "Test", Status: "up"})
      if transition != TransitionToUp {
          t.Errorf("transition = %v, want TransitionToUp", transition)
      }
      if ms.Status != StatusUp {
          t.Errorf("Status = %v, want UP", ms.Status)
      }
  }

  func TestMonitorState_NoTransitionOnIntermittentFailure(t *testing.T) {
      ms := NewMonitorState("Test", 3)

      // fail, fail, success, fail — should never go DOWN
      ms.RecordResult(checks.Result{Monitor: "Test", Status: "down", Error: "timeout"})
      ms.RecordResult(checks.Result{Monitor: "Test", Status: "down", Error: "timeout"})
      ms.RecordResult(checks.Result{Monitor: "Test", Status: "up"})
      transition := ms.RecordResult(checks.Result{Monitor: "Test", Status: "down", Error: "timeout"})

      if ms.Status != StatusUp {
          t.Errorf("Status = %v, want UP (intermittent failures shouldn't trigger DOWN)", ms.Status)
      }
      if transition != TransitionNone {
          t.Errorf("transition = %v, want None", transition)
      }
  }

  func TestMonitorState_ConsecutiveFailureCount(t *testing.T) {
      ms := NewMonitorState("Test", 5)

      ms.RecordResult(checks.Result{Monitor: "Test", Status: "down", Error: "err"})
      ms.RecordResult(checks.Result{Monitor: "Test", Status: "down", Error: "err"})
      if ms.ConsecutiveFailures != 2 {
          t.Errorf("ConsecutiveFailures = %d, want 2", ms.ConsecutiveFailures)
      }

      ms.RecordResult(checks.Result{Monitor: "Test", Status: "up"})
      if ms.ConsecutiveFailures != 0 {
          t.Errorf("ConsecutiveFailures = %d after success, want 0", ms.ConsecutiveFailures)
      }
  }

  func TestMonitorState_TransitionToDown_ReturnsTransition(t *testing.T) {
      ms := NewMonitorState("Test", 2)
      ms.RecordResult(checks.Result{Monitor: "Test", Status: "down", Error: "err"})
      transition := ms.RecordResult(checks.Result{Monitor: "Test", Status: "down", Error: "err"})

      if transition != TransitionToDown {
          t.Errorf("transition = %v, want TransitionToDown", transition)
      }
  }
  ```

- [ ] **8.2** Run tests (they will fail):
  ```bash
  cd /home/andyhazz/projects/whatsupp
  go test ./internal/hub/... -run TestMonitorState -v
  ```

- [ ] **8.3** Implement `internal/hub/monitor.go`:
  ```go
  package hub

  import (
      "github.com/andyhazz/whatsupp/internal/checks"
  )

  // MonitorStatus represents the current state of a monitor.
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

  // Transition represents a state change event.
  type Transition int

  const (
      TransitionNone   Transition = iota
      TransitionToDown            // was UP, now DOWN
      TransitionToUp              // was DOWN, now UP
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

  // MonitorState tracks the UP/DOWN state machine for a single monitor.
  type MonitorState struct {
      Name                string
      Status              MonitorStatus
      FailureThreshold    int
      ConsecutiveFailures int
      LastError           string
  }

  // NewMonitorState creates a new MonitorState starting in UP state.
  func NewMonitorState(name string, failureThreshold int) *MonitorState {
      return &MonitorState{
          Name:             name,
          Status:           StatusUp,
          FailureThreshold: failureThreshold,
      }
  }

  // RecordResult processes a check result and returns any state transition.
  func (ms *MonitorState) RecordResult(result checks.Result) Transition {
      if result.Status == "up" {
          ms.ConsecutiveFailures = 0
          ms.LastError = ""
          if ms.Status == StatusDown {
              ms.Status = StatusUp
              return TransitionToUp
          }
          return TransitionNone
      }

      // Status is "down"
      ms.ConsecutiveFailures++
      ms.LastError = result.Error

      if ms.Status == StatusUp && ms.ConsecutiveFailures >= ms.FailureThreshold {
          ms.Status = StatusDown
          return TransitionToDown
      }

      return TransitionNone
  }
  ```

- [ ] **8.4** Run tests — all should pass:
  ```bash
  cd /home/andyhazz/projects/whatsupp
  go test ./internal/hub/... -run TestMonitorState -v
  ```

- [ ] **8.5** Commit:
  ```bash
  cd /home/andyhazz/projects/whatsupp
  git add internal/hub/monitor.go internal/hub/monitor_test.go
  git commit -m "feat: monitor state machine with consecutive failure counting"
  ```

---

## Task 9: Incident Management (Create on DOWN, Resolve on UP)

**Files:** Create `internal/hub/incidents.go`, `internal/hub/incidents_test.go`

### Steps

- [ ] **9.1** Write `internal/hub/incidents_test.go`:
  ```go
  package hub

  import (
      "path/filepath"
      "testing"
      "time"

      "github.com/andyhazz/whatsupp/internal/store"
  )

  func testStore(t *testing.T) *store.Store {
      t.Helper()
      dir := t.TempDir()
      path := filepath.Join(dir, "test.db")
      s, err := store.Open(path)
      if err != nil {
          t.Fatalf("store.Open() error: %v", err)
      }
      t.Cleanup(func() { s.Close() })
      return s
  }

  func TestIncidentManager_CreateOnDown(t *testing.T) {
      s := testStore(t)
      im := NewIncidentManager(s)

      now := time.Now().Unix()
      inc, err := im.HandleTransition("Plex", TransitionToDown, now, "connection refused")
      if err != nil {
          t.Fatalf("HandleTransition() error: %v", err)
      }
      if inc == nil {
          t.Fatal("HandleTransition() returned nil incident on DOWN transition")
      }
      if inc.Cause != "connection refused" {
          t.Errorf("Cause = %q, want %q", inc.Cause, "connection refused")
      }
  }

  func TestIncidentManager_ResolveOnUp(t *testing.T) {
      s := testStore(t)
      im := NewIncidentManager(s)

      now := time.Now().Unix()
      // Create incident
      _, err := im.HandleTransition("Plex", TransitionToDown, now, "timeout")
      if err != nil {
          t.Fatalf("HandleTransition(DOWN) error: %v", err)
      }

      // Resolve it
      inc, err := im.HandleTransition("Plex", TransitionToUp, now+300, "")
      if err != nil {
          t.Fatalf("HandleTransition(UP) error: %v", err)
      }
      if inc == nil {
          t.Fatal("HandleTransition(UP) returned nil incident")
      }
      if inc.ResolvedAt == nil {
          t.Fatal("ResolvedAt should be set after resolve")
      }
      if *inc.ResolvedAt != now+300 {
          t.Errorf("ResolvedAt = %d, want %d", *inc.ResolvedAt, now+300)
      }
  }

  func TestIncidentManager_NoOpOnNone(t *testing.T) {
      s := testStore(t)
      im := NewIncidentManager(s)

      now := time.Now().Unix()
      inc, err := im.HandleTransition("Plex", TransitionNone, now, "")
      if err != nil {
          t.Fatalf("HandleTransition(None) error: %v", err)
      }
      if inc != nil {
          t.Error("HandleTransition(None) should return nil")
      }
  }

  func TestIncidentManager_ResolveNoOpen(t *testing.T) {
      s := testStore(t)
      im := NewIncidentManager(s)

      now := time.Now().Unix()
      // Resolve with no open incident — should be a no-op
      inc, err := im.HandleTransition("Plex", TransitionToUp, now, "")
      if err != nil {
          t.Fatalf("HandleTransition(UP) error: %v", err)
      }
      if inc != nil {
          t.Error("HandleTransition(UP) with no open incident should return nil")
      }
  }
  ```

- [ ] **9.2** Run tests (they will fail):
  ```bash
  cd /home/andyhazz/projects/whatsupp
  go test ./internal/hub/... -run TestIncidentManager -v
  ```

- [ ] **9.3** Implement `internal/hub/incidents.go`:
  ```go
  package hub

  import (
      "github.com/andyhazz/whatsupp/internal/store"
  )

  // IncidentManager handles creating and resolving incidents.
  type IncidentManager struct {
      store *store.Store
  }

  // NewIncidentManager creates a new IncidentManager.
  func NewIncidentManager(s *store.Store) *IncidentManager {
      return &IncidentManager{store: s}
  }

  // HandleTransition processes a state transition and creates/resolves incidents.
  // Returns the affected incident (or nil if no action taken).
  func (im *IncidentManager) HandleTransition(monitor string, transition Transition, timestamp int64, cause string) (*store.Incident, error) {
      switch transition {
      case TransitionToDown:
          id, err := im.store.CreateIncident(monitor, timestamp, cause)
          if err != nil {
              return nil, err
          }
          return &store.Incident{
              ID:        id,
              Monitor:   monitor,
              StartedAt: timestamp,
              Cause:     cause,
          }, nil

      case TransitionToUp:
          inc, err := im.store.GetOpenIncident(monitor)
          if err != nil {
              return nil, err
          }
          if inc == nil {
              return nil, nil // no open incident to resolve
          }
          if err := im.store.ResolveIncident(inc.ID, timestamp); err != nil {
              return nil, err
          }
          inc.ResolvedAt = &timestamp
          return inc, nil

      default:
          return nil, nil
      }
  }
  ```

- [ ] **9.4** Run tests — all should pass:
  ```bash
  cd /home/andyhazz/projects/whatsupp
  go test ./internal/hub/... -run TestIncidentManager -v
  ```

- [ ] **9.5** Commit:
  ```bash
  cd /home/andyhazz/projects/whatsupp
  git add internal/hub/incidents.go internal/hub/incidents_test.go
  git commit -m "feat: incident management - create on DOWN, resolve on UP"
  ```

---

## Task 10: ntfy Alerting Client (with Deduplication)

**Files:** Create `internal/alerting/ntfy.go`, `internal/alerting/ntfy_test.go`

### Steps

- [ ] **10.1** Write `internal/alerting/ntfy_test.go`:
  ```go
  package alerting

  import (
      "encoding/json"
      "io"
      "net/http"
      "net/http/httptest"
      "sync"
      "testing"
      "time"
  )

  type capturedMessage struct {
      Topic    string `json:"topic"`
      Title    string `json:"title"`
      Message  string `json:"message"`
      Priority int    `json:"priority"`
      Tags     string `json:"tags"`
  }

  func captureServer(t *testing.T) (*httptest.Server, *[]capturedMessage, *sync.Mutex) {
      t.Helper()
      var msgs []capturedMessage
      var mu sync.Mutex

      srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
          body, _ := io.ReadAll(r.Body)
          var msg capturedMessage
          json.Unmarshal(body, &msg)
          mu.Lock()
          msgs = append(msgs, msg)
          mu.Unlock()
          w.WriteHeader(200)
      }))
      t.Cleanup(srv.Close)
      return srv, &msgs, &mu
  }

  func TestNtfyClient_SendDownAlert(t *testing.T) {
      srv, msgs, mu := captureServer(t)

      client := NewNtfyClient(NtfyConfig{
          URL:              srv.URL,
          Topic:            "test",
          ReminderInterval: time.Hour,
      })

      err := client.SendDown("Plex", "connection refused (3/3 failures)")
      if err != nil {
          t.Fatalf("SendDown() error: %v", err)
      }

      mu.Lock()
      defer mu.Unlock()
      if len(*msgs) != 1 {
          t.Fatalf("messages sent = %d, want 1", len(*msgs))
      }
      if (*msgs)[0].Priority != 4 {
          t.Errorf("Priority = %d, want 4", (*msgs)[0].Priority)
      }
  }

  func TestNtfyClient_SendRecovery(t *testing.T) {
      srv, msgs, mu := captureServer(t)

      client := NewNtfyClient(NtfyConfig{
          URL:              srv.URL,
          Topic:            "test",
          ReminderInterval: time.Hour,
      })

      err := client.SendRecovery("Plex", "4m 32s")
      if err != nil {
          t.Fatalf("SendRecovery() error: %v", err)
      }

      mu.Lock()
      defer mu.Unlock()
      if len(*msgs) != 1 {
          t.Fatalf("messages sent = %d, want 1", len(*msgs))
      }
      if (*msgs)[0].Priority != 3 {
          t.Errorf("Priority = %d, want 3", (*msgs)[0].Priority)
      }
  }

  func TestNtfyClient_Deduplication(t *testing.T) {
      srv, msgs, mu := captureServer(t)

      client := NewNtfyClient(NtfyConfig{
          URL:              srv.URL,
          Topic:            "test",
          ReminderInterval: time.Hour,
      })

      // First DOWN alert should send
      client.SendDown("Plex", "timeout")
      // Second DOWN alert should be suppressed (dedup)
      client.SendDown("Plex", "timeout")
      // Third DOWN alert should also be suppressed
      client.SendDown("Plex", "timeout")

      mu.Lock()
      defer mu.Unlock()
      if len(*msgs) != 1 {
          t.Errorf("messages sent = %d, want 1 (dedup should suppress duplicates)", len(*msgs))
      }
  }

  func TestNtfyClient_ReminderAfterInterval(t *testing.T) {
      srv, msgs, mu := captureServer(t)

      client := NewNtfyClient(NtfyConfig{
          URL:              srv.URL,
          Topic:            "test",
          ReminderInterval: 100 * time.Millisecond, // short for testing
      })

      client.SendDown("Plex", "timeout")
      time.Sleep(150 * time.Millisecond)
      client.SendDown("Plex", "timeout") // should send reminder

      mu.Lock()
      defer mu.Unlock()
      if len(*msgs) != 2 {
          t.Errorf("messages sent = %d, want 2 (initial + reminder)", len(*msgs))
      }
  }

  func TestNtfyClient_RecoveryClearsDedup(t *testing.T) {
      srv, msgs, mu := captureServer(t)

      client := NewNtfyClient(NtfyConfig{
          URL:              srv.URL,
          Topic:            "test",
          ReminderInterval: time.Hour,
      })

      client.SendDown("Plex", "timeout")  // sends
      client.SendRecovery("Plex", "5m")   // sends, clears dedup
      client.SendDown("Plex", "timeout")  // should send again (dedup cleared)

      mu.Lock()
      defer mu.Unlock()
      if len(*msgs) != 3 {
          t.Errorf("messages sent = %d, want 3 (down + recovery + new down)", len(*msgs))
      }
  }

  func TestNtfyClient_SecurityAlerts(t *testing.T) {
      srv, msgs, mu := captureServer(t)

      client := NewNtfyClient(NtfyConfig{
          URL:              srv.URL,
          Topic:            "test",
          ReminderInterval: time.Hour,
      })

      err := client.SendNewPort("84.18.245.85", 4444)
      if err != nil {
          t.Fatalf("SendNewPort() error: %v", err)
      }

      err = client.SendPortGone("84.18.245.85", 443)
      if err != nil {
          t.Fatalf("SendPortGone() error: %v", err)
      }

      mu.Lock()
      defer mu.Unlock()
      if len(*msgs) != 2 {
          t.Fatalf("messages sent = %d, want 2", len(*msgs))
      }
      if (*msgs)[0].Priority != 5 {
          t.Errorf("NewPort priority = %d, want 5", (*msgs)[0].Priority)
      }
      if (*msgs)[1].Priority != 4 {
          t.Errorf("PortGone priority = %d, want 4", (*msgs)[1].Priority)
      }
  }

  func TestNtfyClient_SSLExpiryAlert(t *testing.T) {
      srv, msgs, mu := captureServer(t)

      client := NewNtfyClient(NtfyConfig{
          URL:              srv.URL,
          Topic:            "test",
          ReminderInterval: time.Hour,
      })

      err := client.SendSSLExpiry("example.com", 7)
      if err != nil {
          t.Fatalf("SendSSLExpiry() error: %v", err)
      }

      mu.Lock()
      defer mu.Unlock()
      if len(*msgs) != 1 {
          t.Fatalf("messages sent = %d, want 1", len(*msgs))
      }
      if (*msgs)[0].Priority != 4 {
          t.Errorf("Priority = %d, want 4", (*msgs)[0].Priority)
      }
  }

  func TestNtfyClient_BasicAuth(t *testing.T) {
      var authHeader string
      srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
          authHeader = r.Header.Get("Authorization")
          w.WriteHeader(200)
      }))
      defer srv.Close()

      client := NewNtfyClient(NtfyConfig{
          URL:              srv.URL,
          Topic:            "test",
          Username:         "user",
          Password:         "pass",
          ReminderInterval: time.Hour,
      })

      client.SendDown("Test", "err")
      if authHeader == "" {
          t.Error("Authorization header should be set when username/password configured")
      }
  }
  ```

- [ ] **10.2** Run tests (they will fail):
  ```bash
  cd /home/andyhazz/projects/whatsupp
  go test ./internal/alerting/... -v
  ```

- [ ] **10.3** Implement `internal/alerting/ntfy.go`:
  ```go
  package alerting

  import (
      "bytes"
      "encoding/json"
      "fmt"
      "net/http"
      "sync"
      "time"
  )

  // NtfyConfig holds ntfy connection settings.
  type NtfyConfig struct {
      URL              string
      Topic            string
      Username         string
      Password         string
      ReminderInterval time.Duration
  }

  // NtfyClient sends alerts to an ntfy server with deduplication.
  type NtfyClient struct {
      config NtfyConfig
      client *http.Client

      mu            sync.Mutex
      lastDownAlert map[string]time.Time // monitor -> last DOWN alert time
  }

  type ntfyMessage struct {
      Topic    string `json:"topic"`
      Title    string `json:"title"`
      Message  string `json:"message"`
      Priority int    `json:"priority"`
      Tags     string `json:"tags,omitempty"`
  }

  // NewNtfyClient creates a new ntfy alert client.
  func NewNtfyClient(cfg NtfyConfig) *NtfyClient {
      if cfg.ReminderInterval == 0 {
          cfg.ReminderInterval = time.Hour
      }
      return &NtfyClient{
          config:        cfg,
          client:        &http.Client{Timeout: 10 * time.Second},
          lastDownAlert: make(map[string]time.Time),
      }
  }

  // SendDown sends a monitor DOWN alert, with deduplication.
  func (n *NtfyClient) SendDown(monitor, cause string) error {
      n.mu.Lock()
      lastSent, exists := n.lastDownAlert[monitor]
      now := time.Now()
      if exists && now.Sub(lastSent) < n.config.ReminderInterval {
          n.mu.Unlock()
          return nil // suppressed by dedup
      }
      n.lastDownAlert[monitor] = now
      n.mu.Unlock()

      msg := ntfyMessage{
          Topic:    n.config.Topic,
          Title:    fmt.Sprintf("%s is DOWN", monitor),
          Message:  fmt.Sprintf("%s is DOWN - %s", monitor, cause),
          Priority: 4,
          Tags:     "rotating_light",
      }
      return n.send(msg)
  }

  // SendRecovery sends a monitor RECOVERED alert and clears dedup state.
  func (n *NtfyClient) SendRecovery(monitor, downDuration string) error {
      n.mu.Lock()
      delete(n.lastDownAlert, monitor)
      n.mu.Unlock()

      msg := ntfyMessage{
          Topic:    n.config.Topic,
          Title:    fmt.Sprintf("%s is UP", monitor),
          Message:  fmt.Sprintf("%s is UP - was down for %s", monitor, downDuration),
          Priority: 3,
          Tags:     "white_check_mark",
      }
      return n.send(msg)
  }

  // SendNewPort sends an alert for a newly detected open port.
  func (n *NtfyClient) SendNewPort(target string, port int) error {
      msg := ntfyMessage{
          Topic:    n.config.Topic,
          Title:    fmt.Sprintf("Security: new port on %s", target),
          Message:  fmt.Sprintf("Security: new port %d/tcp on %s (not in baseline)", port, target),
          Priority: 5,
          Tags:     "warning",
      }
      return n.send(msg)
  }

  // SendPortGone sends an alert for a port that is no longer open.
  func (n *NtfyClient) SendPortGone(target string, port int) error {
      msg := ntfyMessage{
          Topic:    n.config.Topic,
          Title:    fmt.Sprintf("Security: port gone on %s", target),
          Message:  fmt.Sprintf("Security: port %d/tcp no longer open on %s", port, target),
          Priority: 4,
          Tags:     "warning",
      }
      return n.send(msg)
  }

  // SendSSLExpiry sends an SSL certificate expiry warning.
  func (n *NtfyClient) SendSSLExpiry(domain string, daysLeft int) error {
      msg := ntfyMessage{
          Topic:    n.config.Topic,
          Title:    fmt.Sprintf("SSL cert expiring: %s", domain),
          Message:  fmt.Sprintf("SSL cert for %s expires in %d days", domain, daysLeft),
          Priority: 4,
          Tags:     "lock,warning",
      }
      return n.send(msg)
  }

  // send posts a message to the ntfy server.
  func (n *NtfyClient) send(msg ntfyMessage) error {
      body, err := json.Marshal(msg)
      if err != nil {
          return fmt.Errorf("marshal ntfy message: %w", err)
      }

      req, err := http.NewRequest("POST", n.config.URL, bytes.NewReader(body))
      if err != nil {
          return fmt.Errorf("create ntfy request: %w", err)
      }
      req.Header.Set("Content-Type", "application/json")

      if n.config.Username != "" && n.config.Password != "" {
          req.SetBasicAuth(n.config.Username, n.config.Password)
      }

      resp, err := n.client.Do(req)
      if err != nil {
          return fmt.Errorf("send ntfy alert: %w", err)
      }
      defer resp.Body.Close()

      if resp.StatusCode >= 400 {
          return fmt.Errorf("ntfy returned status %d", resp.StatusCode)
      }
      return nil
  }
  ```

- [ ] **10.4** Run tests — all should pass:
  ```bash
  cd /home/andyhazz/projects/whatsupp
  go test ./internal/alerting/... -v
  ```

- [ ] **10.5** Commit:
  ```bash
  cd /home/andyhazz/projects/whatsupp
  git add internal/alerting/
  git commit -m "feat: ntfy alerting client with deduplication and security/SSL alerts"
  ```

---

## Task 11: Scheduler (Runs Checks at Configured Intervals)

**Files:** Create `internal/hub/scheduler.go`, `internal/hub/scheduler_test.go`

### Steps

- [ ] **11.1** Write `internal/hub/scheduler_test.go`:
  ```go
  package hub

  import (
      "sync"
      "sync/atomic"
      "testing"
      "time"

      "github.com/andyhazz/whatsupp/internal/checks"
      "github.com/andyhazz/whatsupp/internal/config"
  )

  // mockChecker records how many times Check was called.
  type mockChecker struct {
      callCount atomic.Int32
      result    checks.Result
  }

  func (m *mockChecker) Check(name string) checks.Result {
      m.callCount.Add(1)
      return m.result
  }

  func TestScheduler_RunsChecks(t *testing.T) {
      resultCh := make(chan checks.Result, 100)
      mock := &mockChecker{result: checks.Result{Status: "up", LatencyMs: 10}}

      monitors := []config.Monitor{
          {Name: "Test", Type: "http", URL: "http://example.com", Interval: 100 * time.Millisecond},
      }

      s := NewScheduler(monitors, resultCh)
      s.RegisterChecker("Test", mock)
      s.Start()

      // Wait for at least 2 checks to fire
      time.Sleep(250 * time.Millisecond)
      s.Stop()

      count := mock.callCount.Load()
      if count < 2 {
          t.Errorf("check ran %d times, want >= 2", count)
      }

      // Drain results channel
      close(resultCh)
      var results []checks.Result
      for r := range resultCh {
          results = append(results, r)
      }
      if len(results) < 2 {
          t.Errorf("received %d results, want >= 2", len(results))
      }
  }

  func TestScheduler_StopsCleanly(t *testing.T) {
      resultCh := make(chan checks.Result, 100)
      mock := &mockChecker{result: checks.Result{Status: "up"}}

      monitors := []config.Monitor{
          {Name: "Test", Type: "http", URL: "http://example.com", Interval: 50 * time.Millisecond},
      }

      s := NewScheduler(monitors, resultCh)
      s.RegisterChecker("Test", mock)
      s.Start()
      time.Sleep(100 * time.Millisecond)
      s.Stop()

      countAtStop := mock.callCount.Load()
      time.Sleep(150 * time.Millisecond)
      countAfter := mock.callCount.Load()

      if countAfter != countAtStop {
          t.Errorf("checks continued after Stop(): %d -> %d", countAtStop, countAfter)
      }
  }

  func TestScheduler_MultipleMonitors(t *testing.T) {
      resultCh := make(chan checks.Result, 100)

      var mu sync.Mutex
      seen := make(map[string]int)

      monitors := []config.Monitor{
          {Name: "A", Type: "http", URL: "http://a.com", Interval: 100 * time.Millisecond},
          {Name: "B", Type: "http", URL: "http://b.com", Interval: 100 * time.Millisecond},
      }

      mockA := &mockChecker{result: checks.Result{Monitor: "A", Status: "up"}}
      mockB := &mockChecker{result: checks.Result{Monitor: "B", Status: "up"}}

      s := NewScheduler(monitors, resultCh)
      s.RegisterChecker("A", mockA)
      s.RegisterChecker("B", mockB)
      s.Start()

      time.Sleep(250 * time.Millisecond)
      s.Stop()
      close(resultCh)

      for r := range resultCh {
          mu.Lock()
          seen[r.Monitor]++
          mu.Unlock()
      }

      if seen["A"] < 1 {
          t.Errorf("monitor A ran %d times, want >= 1", seen["A"])
      }
      if seen["B"] < 1 {
          t.Errorf("monitor B ran %d times, want >= 1", seen["B"])
      }
  }
  ```

- [ ] **11.2** Run tests (they will fail):
  ```bash
  cd /home/andyhazz/projects/whatsupp
  go test ./internal/hub/... -run TestScheduler -v
  ```

- [ ] **11.3** Implement `internal/hub/scheduler.go`:
  ```go
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
  ```

- [ ] **11.4** Run tests — all should pass:
  ```bash
  cd /home/andyhazz/projects/whatsupp
  go test ./internal/hub/... -run TestScheduler -v
  ```

- [ ] **11.5** Commit:
  ```bash
  cd /home/andyhazz/projects/whatsupp
  git add internal/hub/scheduler.go internal/hub/scheduler_test.go
  git commit -m "feat: scheduler runs checks at configured intervals with clean shutdown"
  ```

---

## Task 12: Downsampling Goroutine (Raw -> Hourly -> Daily + Retention Cleanup)

**Files:** Create `internal/hub/downsampler.go`, `internal/hub/downsampler_test.go`

### Steps

- [ ] **12.1** Write `internal/hub/downsampler_test.go`:
  ```go
  package hub

  import (
      "testing"
      "time"
  )

  func TestDownsampler_HourlyAggregation(t *testing.T) {
      s := testStore(t)

      // Insert check results across two hours
      hour1 := int64(1711000000) // some epoch, truncated to hour
      hour1 = hour1 - (hour1 % 3600)
      hour2 := hour1 + 3600

      // Hour 1: 2 up, 1 down
      s.InsertCheckResult("Plex", hour1+10, "up", 40.0, "")
      s.InsertCheckResult("Plex", hour1+20, "up", 60.0, "")
      s.InsertCheckResult("Plex", hour1+30, "down", 0.0, "")

      // Hour 2: 3 up
      s.InsertCheckResult("Plex", hour2+10, "up", 30.0, "")
      s.InsertCheckResult("Plex", hour2+20, "up", 50.0, "")
      s.InsertCheckResult("Plex", hour2+30, "up", 70.0, "")

      d := NewDownsampler(s, DefaultRetentionConfig())

      // Aggregate hour 1
      err := d.AggregateHour(hour1)
      if err != nil {
          t.Fatalf("AggregateHour() error: %v", err)
      }

      rows, err := s.GetCheckResultsHourly("Plex", hour1, hour1+3600)
      if err != nil {
          t.Fatalf("GetCheckResultsHourly() error: %v", err)
      }
      if len(rows) != 1 {
          t.Fatalf("hourly rows = %d, want 1", len(rows))
      }
      if rows[0].SuccessCount != 2 {
          t.Errorf("SuccessCount = %d, want 2", rows[0].SuccessCount)
      }
      if rows[0].FailCount != 1 {
          t.Errorf("FailCount = %d, want 1", rows[0].FailCount)
      }
  }

  func TestDownsampler_Cleanup(t *testing.T) {
      s := testStore(t)

      now := time.Now().Unix()
      old := now - 86400*31 // 31 days ago

      s.InsertCheckResult("Plex", old, "up", 40.0, "")
      s.InsertCheckResult("Plex", now, "up", 50.0, "")

      d := NewDownsampler(s, DefaultRetentionConfig())
      n, err := d.CleanupRawCheckResults()
      if err != nil {
          t.Fatalf("CleanupRawCheckResults() error: %v", err)
      }
      if n != 1 {
          t.Errorf("cleaned up = %d, want 1", n)
      }
  }

  func TestDefaultRetentionConfig(t *testing.T) {
      rc := DefaultRetentionConfig()
      if rc.CheckResultsRaw != 30*24*time.Hour {
          t.Errorf("CheckResultsRaw = %v, want 720h", rc.CheckResultsRaw)
      }
      if rc.Hourly != 180*24*time.Hour {
          t.Errorf("Hourly = %v, want 4320h", rc.Hourly)
      }
  }
  ```

- [ ] **12.2** Run tests (they will fail):
  ```bash
  cd /home/andyhazz/projects/whatsupp
  go test ./internal/hub/... -run TestDownsampler -v
  ```

- [ ] **12.3** Implement `internal/hub/downsampler.go`:
  ```go
  package hub

  import (
      "log"
      "sync"
      "time"

      "github.com/andyhazz/whatsupp/internal/store"
  )

  // RetentionConfig defines how long each tier is kept.
  type RetentionConfig struct {
      CheckResultsRaw time.Duration
      Hourly           time.Duration
      // Daily is forever (no deletion)
  }

  // DefaultRetentionConfig returns spec defaults.
  func DefaultRetentionConfig() RetentionConfig {
      return RetentionConfig{
          CheckResultsRaw: 30 * 24 * time.Hour,  // 30 days
          Hourly:          180 * 24 * time.Hour,  // 6 months
      }
  }

  // Downsampler performs periodic aggregation and cleanup.
  type Downsampler struct {
      store     *store.Store
      retention RetentionConfig
      stopCh    chan struct{}
      wg        sync.WaitGroup
  }

  // NewDownsampler creates a new downsampler.
  func NewDownsampler(s *store.Store, retention RetentionConfig) *Downsampler {
      return &Downsampler{
          store:     s,
          retention: retention,
          stopCh:    make(chan struct{}),
      }
  }

  // Start begins the downsampling goroutines.
  func (d *Downsampler) Start() {
      d.wg.Add(2)
      go d.hourlyLoop()
      go d.dailyLoop()
  }

  // Stop signals downsampling goroutines to stop and waits.
  func (d *Downsampler) Stop() {
      close(d.stopCh)
      d.wg.Wait()
  }

  func (d *Downsampler) hourlyLoop() {
      defer d.wg.Done()
      ticker := time.NewTicker(time.Hour)
      defer ticker.Stop()

      for {
          select {
          case <-ticker.C:
              d.runHourlyAggregation()
          case <-d.stopCh:
              return
          }
      }
  }

  func (d *Downsampler) dailyLoop() {
      defer d.wg.Done()

      // Calculate time until next midnight
      now := time.Now()
      nextMidnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
      timer := time.NewTimer(time.Until(nextMidnight))
      defer timer.Stop()

      for {
          select {
          case <-timer.C:
              d.runDailyAggregation()
              // Reset timer for next midnight
              now := time.Now()
              nextMidnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
              timer.Reset(time.Until(nextMidnight))
          case <-d.stopCh:
              return
          }
      }
  }

  func (d *Downsampler) runHourlyAggregation() {
      // Aggregate the previous hour
      now := time.Now()
      hourEnd := now.Truncate(time.Hour)
      hourStart := hourEnd.Add(-time.Hour)

      if err := d.AggregateHour(hourStart.Unix()); err != nil {
          log.Printf("downsampler: hourly aggregation error: %v", err)
      }

      // Cleanup old raw check results
      if n, err := d.CleanupRawCheckResults(); err != nil {
          log.Printf("downsampler: cleanup raw check results error: %v", err)
      } else if n > 0 {
          log.Printf("downsampler: deleted %d old raw check results", n)
      }
  }

  func (d *Downsampler) runDailyAggregation() {
      // Aggregate the previous day from hourly
      now := time.Now()
      dayEnd := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
      dayStart := dayEnd.AddDate(0, 0, -1)

      if err := d.store.AggregateCheckResultsDaily(dayStart.Unix(), dayEnd.Unix()); err != nil {
          log.Printf("downsampler: daily aggregation error: %v", err)
      }

      // Cleanup old hourly data
      cutoff := now.Add(-d.retention.Hourly).Unix()
      if n, err := d.store.DeleteOldHourlyCheckResults(cutoff); err != nil {
          log.Printf("downsampler: cleanup hourly error: %v", err)
      } else if n > 0 {
          log.Printf("downsampler: deleted %d old hourly check results", n)
      }
  }

  // AggregateHour aggregates raw check results for the hour starting at hourStart.
  func (d *Downsampler) AggregateHour(hourStartUnix int64) error {
      return d.store.AggregateCheckResultsHourly(hourStartUnix, hourStartUnix+3600)
  }

  // CleanupRawCheckResults deletes raw check results older than retention period.
  func (d *Downsampler) CleanupRawCheckResults() (int64, error) {
      cutoff := time.Now().Add(-d.retention.CheckResultsRaw).Unix()
      return d.store.DeleteOldCheckResults(cutoff)
  }
  ```

- [ ] **12.4** Run tests — all should pass:
  ```bash
  cd /home/andyhazz/projects/whatsupp
  go test ./internal/hub/... -run "TestDownsampler|TestDefaultRetention" -v
  ```

- [ ] **12.5** Commit:
  ```bash
  cd /home/andyhazz/projects/whatsupp
  git add internal/hub/downsampler.go internal/hub/downsampler_test.go
  git commit -m "feat: downsampling goroutines with hourly/daily aggregation and retention cleanup"
  ```

---

## Task 13: Hub Orchestration (Wires Everything Together)

**Files:** Create `internal/hub/hub.go`, `internal/hub/hub_test.go`

### Steps

- [ ] **13.1** Write `internal/hub/hub_test.go`:
  ```go
  package hub

  import (
      "net/http"
      "net/http/httptest"
      "path/filepath"
      "testing"
      "time"

      "github.com/andyhazz/whatsupp/internal/alerting"
      "github.com/andyhazz/whatsupp/internal/config"
  )

  func TestHub_ProcessResult_DownTransition(t *testing.T) {
      s := testStore(t)

      // Capture ntfy messages
      var alertCount int
      srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
          alertCount++
          w.WriteHeader(200)
      }))
      defer srv.Close()

      ntfyClient := alerting.NewNtfyClient(alerting.NtfyConfig{
          URL:              srv.URL,
          Topic:            "test",
          ReminderInterval: time.Hour,
      })

      h := &Hub{
          store:           s,
          alerter:         ntfyClient,
          incidentManager: NewIncidentManager(s),
          monitorStates:   make(map[string]*MonitorState),
      }
      h.monitorStates["Plex"] = NewMonitorState("Plex", 2)

      // Two failures should trigger DOWN + incident + alert
      for i := 0; i < 2; i++ {
          h.processResult(checks.Result{Monitor: "Plex", Status: "down", Error: "timeout"})
      }

      if h.monitorStates["Plex"].Status != StatusDown {
          t.Errorf("Status = %v, want DOWN", h.monitorStates["Plex"].Status)
      }
      if alertCount != 1 {
          t.Errorf("alerts sent = %d, want 1", alertCount)
      }

      // Verify incident was created
      inc, _ := s.GetOpenIncident("Plex")
      if inc == nil {
          t.Error("no open incident after DOWN transition")
      }
  }

  func TestHub_ProcessResult_Recovery(t *testing.T) {
      s := testStore(t)

      var alertCount int
      srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
          alertCount++
          w.WriteHeader(200)
      }))
      defer srv.Close()

      ntfyClient := alerting.NewNtfyClient(alerting.NtfyConfig{
          URL:              srv.URL,
          Topic:            "test",
          ReminderInterval: time.Hour,
      })

      h := &Hub{
          store:           s,
          alerter:         ntfyClient,
          incidentManager: NewIncidentManager(s),
          monitorStates:   make(map[string]*MonitorState),
      }
      h.monitorStates["Plex"] = NewMonitorState("Plex", 2)

      // Drive to DOWN
      h.processResult(checks.Result{Monitor: "Plex", Status: "down", Error: "timeout"})
      h.processResult(checks.Result{Monitor: "Plex", Status: "down", Error: "timeout"})

      // Recover
      h.processResult(checks.Result{Monitor: "Plex", Status: "up", LatencyMs: 50})

      if h.monitorStates["Plex"].Status != StatusUp {
          t.Errorf("Status = %v, want UP after recovery", h.monitorStates["Plex"].Status)
      }
      // Should have sent DOWN + RECOVERY = 2 alerts
      if alertCount != 2 {
          t.Errorf("alerts sent = %d, want 2 (down + recovery)", alertCount)
      }

      // Incident should be resolved
      inc, _ := s.GetOpenIncident("Plex")
      if inc != nil {
          t.Error("incident should be resolved after recovery")
      }
  }

  func TestHub_NewFromConfig(t *testing.T) {
      dir := t.TempDir()
      dbPath := filepath.Join(dir, "test.db")

      cfg := &config.Config{
          Server: config.ServerConfig{
              Listen: ":8080",
              DBPath: dbPath,
          },
          Monitors: []config.Monitor{
              {Name: "Test", Type: "http", URL: "https://example.com", Interval: 60 * time.Second, FailureThreshold: 3},
          },
          Alerting: config.AlertingConfig{
              DefaultFailureThreshold: 3,
              Ntfy: config.NtfyConfig{
                  URL:   "https://ntfy.example.com",
                  Topic: "test",
              },
              Thresholds: config.ThresholdsConfig{
                  DownReminderInterval: time.Hour,
              },
          },
          Retention: config.RetentionConfig{
              CheckResultsRaw: 720 * time.Hour,
              Hourly:          4320 * time.Hour,
          },
      }

      h, err := New(cfg)
      if err != nil {
          t.Fatalf("New() error: %v", err)
      }
      defer h.Close()

      if h.store == nil {
          t.Error("store is nil")
      }
      if h.alerter == nil {
          t.Error("alerter is nil")
      }
      if _, ok := h.monitorStates["Test"]; !ok {
          t.Error("monitor state not initialized for 'Test'")
      }
  }
  ```

- [ ] **13.2** Run tests (they will fail):
  ```bash
  cd /home/andyhazz/projects/whatsupp
  go test ./internal/hub/... -run TestHub -v
  ```

- [ ] **13.3** Implement `internal/hub/hub.go`:
  ```go
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
              // Extract domain from monitor URL (simplified)
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

      // Security scans run on cron schedules.
      // For Plan 1 we use a simple goroutine-based approach.
      // robfig/cron integration can be added if needed for complex cron parsing.
      for _, target := range h.cfg.Security.Targets {
          go h.runSecurityScanLoop(target)
      }
  }

  func (h *Hub) runSecurityScanLoop(target config.SecurityTarget) {
      // For now, run once on startup and then stop.
      // Full cron scheduling will use robfig/cron.
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
  ```

- [ ] **13.4** Run tests — all should pass:
  ```bash
  cd /home/andyhazz/projects/whatsupp
  go test ./internal/hub/... -v
  ```

- [ ] **13.5** Commit:
  ```bash
  cd /home/andyhazz/projects/whatsupp
  git add internal/hub/hub.go internal/hub/hub_test.go
  git commit -m "feat: hub orchestration wiring checks, state, incidents, alerts, and downsampling"
  ```

---

## Task 14: CLI Entry Point (`whatsupp serve` Command)

**Files:** Modify `cmd/whatsupp/main.go`

### Steps

- [ ] **14.1** Update `cmd/whatsupp/main.go` to wire the hub:
  ```go
  package main

  import (
      "flag"
      "fmt"
      "log"
      "os"
      "os/signal"
      "syscall"

      "github.com/andyhazz/whatsupp/internal/config"
      "github.com/andyhazz/whatsupp/internal/hub"
  )

  const defaultConfigPath = "/etc/whatsupp/config.yml"

  func main() {
      if len(os.Args) < 2 {
          fmt.Fprintf(os.Stderr, "Usage: whatsupp <serve|agent>\n")
          os.Exit(1)
      }

      switch os.Args[1] {
      case "serve":
          serve()
      default:
          fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
          os.Exit(1)
      }
  }

  func serve() {
      fs := flag.NewFlagSet("serve", flag.ExitOnError)
      configPath := fs.String("config", defaultConfigPath, "path to config.yml")
      fs.Parse(os.Args[2:])

      log.Printf("whatsupp: loading config from %s", *configPath)
      cfg, err := config.Load(*configPath)
      if err != nil {
          log.Fatalf("whatsupp: failed to load config: %v", err)
      }

      h, err := hub.New(cfg)
      if err != nil {
          log.Fatalf("whatsupp: failed to create hub: %v", err)
      }

      // Handle graceful shutdown
      sigCh := make(chan os.Signal, 1)
      signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

      go func() {
          <-sigCh
          log.Println("whatsupp: shutting down...")
          if err := h.Close(); err != nil {
              log.Printf("whatsupp: shutdown error: %v", err)
          }
          os.Exit(0)
      }()

      log.Println("whatsupp: hub starting")
      if err := h.Run(); err != nil {
          log.Fatalf("whatsupp: hub error: %v", err)
      }
  }
  ```

- [ ] **14.2** Verify it compiles:
  ```bash
  cd /home/andyhazz/projects/whatsupp
  go build ./cmd/whatsupp/
  ```

- [ ] **14.3** Create `config.example.yml` at project root for reference:
  ```yaml
  server:
    listen: ":8080"
    db_path: "/data/whatsupp.db"

  auth:
    initial_username: "admin"
    initial_password: "${WHATSUPP_ADMIN_PASSWORD}"

  monitors:
    - name: "Plex"
      type: http
      url: "https://plex.intadnet.duckdns.org"
      interval: 60s
      failure_threshold: 3

    - name: "N8N"
      type: http
      url: "https://n8n8n8n.duckdns.org"
      interval: 60s

    - name: "Gateway"
      type: ping
      host: "192.168.50.1"
      interval: 60s

    - name: "VPN Tunnel"
      type: ping
      host: "10.7.0.2"
      interval: 120s

    - name: "Minecraft"
      type: port
      host: "84.18.245.85"
      port: 25565
      interval: 120s

  agents:
    - name: "plexypi"
      key: "${AGENT_KEY_PLEXYPI}"
    - name: "dietpi"
      key: "${AGENT_KEY_DIETPI}"

  security:
    targets:
      - host: "84.18.245.85"
        schedule: "0 3 * * 0"
        scan_concurrency: 200
        timeout: "2s"
      - host: "145.241.217.231"
        schedule: "0 4 * * 0"
        scan_concurrency: 500
        timeout: "2s"

  alerting:
    default_failure_threshold: 3
    ntfy:
      url: "${NTFY_URL}"
      topic: "${NTFY_TOPIC}"
      username: "${NTFY_USERNAME}"
      password: "${NTFY_PASSWORD}"
    thresholds:
      ssl_expiry_days: [14, 7, 3, 1]
      disk_usage_pct: 90
      disk_hysteresis_pct: 5
      down_reminder_interval: "1h"

  retention:
    check_results_raw: "720h"
    agent_metrics_raw: "48h"
    agent_metrics_5min: "2160h"
    hourly: "4320h"
    daily: "0"
  ```

- [ ] **14.4** Test with a local config file:
  ```bash
  cd /home/andyhazz/projects/whatsupp
  # Create a minimal test config
  cat > /tmp/whatsupp-test.yml << 'EOF'
  server:
    listen: ":8080"
    db_path: "/tmp/whatsupp-test.db"

  monitors:
    - name: "Localhost HTTP"
      type: http
      url: "http://127.0.0.1:8080"
      interval: 30s
      failure_threshold: 2

  alerting:
    default_failure_threshold: 3
    ntfy:
      url: "https://ntfy.example.com"
      topic: "test"
    thresholds:
      ssl_expiry_days: [14, 7, 3, 1]
      disk_usage_pct: 90
      disk_hysteresis_pct: 5
      down_reminder_interval: "1h"
  EOF

  # Run it (will start monitoring — Ctrl+C to stop)
  timeout 5 ./whatsupp serve -config /tmp/whatsupp-test.yml || true
  ```
  Expected: starts up, logs check results, shuts down cleanly on timeout.

- [ ] **14.5** Run all tests to verify nothing is broken:
  ```bash
  cd /home/andyhazz/projects/whatsupp
  go test ./... -v
  ```

- [ ] **14.6** Commit:
  ```bash
  cd /home/andyhazz/projects/whatsupp
  git add cmd/whatsupp/main.go config.example.yml
  git commit -m "feat: CLI entry point with whatsupp serve command and example config"
  ```

---

## Summary

After completing all 14 tasks, you have a working `whatsupp serve` binary that:

1. **Loads YAML config** with `${ENV_VAR}` expansion and validation
2. **Opens SQLite** in WAL mode with full schema (all tables from spec, including agent/user tables for future plans)
3. **Runs HTTP checks** with SSL cert expiry metadata extraction
4. **Runs ICMP ping checks** via pro-bing (requires `CAP_NET_RAW`)
5. **Runs TCP port checks** with latency measurement
6. **Runs full 65535-port security scans** with baseline comparison and drift alerting
7. **Tracks UP/DOWN state** via consecutive failure counting state machine
8. **Creates/resolves incidents** automatically on state transitions
9. **Sends ntfy alerts** with deduplication (DOWN sent once, reminder after configurable interval, recovery clears dedup)
10. **Schedules checks** at per-monitor intervals with clean shutdown
11. **Downsamples data** (hourly aggregation every hour, daily aggregation at midnight, retention cleanup)
12. **Handles graceful shutdown** via SIGINT/SIGTERM

### What is NOT in this plan (deferred to Plans 2-4):
- HTTP API and authentication (Plan 2)
- WebSocket live updates (Plan 2)
- Agent mode and metrics collection (Plan 3)
- Prometheus scraping (Plan 3)
- Svelte frontend dashboard (Plan 4)
- Config file watching / hot reload (Plan 2)
- Dockerfile and deployment (Plan 4)
- robfig/cron for security scan scheduling (simplified to startup-only scan in this plan)
