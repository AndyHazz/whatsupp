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

// Retention is no longer configurable — hardcoded in the downsampler
// based on tier thresholds. See internal/hub/downsampler.go.
