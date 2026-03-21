package agent

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseAgentConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "agent.yml")

	yaml := `hub_url: "https://monitor.example.com"
agent_key: "sk-abc123"
hostname: "plexypi"
interval: 45s
host_fs: "/hostfs"
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := ParseAgentConfig(cfgPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.HubURL != "https://monitor.example.com" {
		t.Errorf("HubURL = %q, want %q", cfg.HubURL, "https://monitor.example.com")
	}
	if cfg.AgentKey != "sk-abc123" {
		t.Errorf("AgentKey = %q, want %q", cfg.AgentKey, "sk-abc123")
	}
	if cfg.Hostname != "plexypi" {
		t.Errorf("Hostname = %q, want %q", cfg.Hostname, "plexypi")
	}
	if cfg.Interval != 45*time.Second {
		t.Errorf("Interval = %v, want %v", cfg.Interval, 45*time.Second)
	}
	if cfg.HostFS != "/hostfs" {
		t.Errorf("HostFS = %q, want %q", cfg.HostFS, "/hostfs")
	}
}

func TestParseAgentConfig_Defaults(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "agent.yml")

	yaml := `hub_url: "https://monitor.example.com"
agent_key: "sk-abc123"
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := ParseAgentConfig(cfgPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Interval != 30*time.Second {
		t.Errorf("Interval = %v, want default 30s", cfg.Interval)
	}

	// Hostname should default to os.Hostname()
	expected, _ := os.Hostname()
	if cfg.Hostname != expected {
		t.Errorf("Hostname = %q, want %q (os.Hostname())", cfg.Hostname, expected)
	}
}

func TestParseAgentConfig_EnvOverrides(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "agent.yml")

	yaml := `hub_url: "https://original.example.com"
agent_key: "sk-original"
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("WHATSUPP_HUB_URL", "https://override.example.com")
	t.Setenv("WHATSUPP_AGENT_KEY", "sk-override")

	cfg, err := ParseAgentConfig(cfgPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.HubURL != "https://override.example.com" {
		t.Errorf("HubURL = %q, want env override", cfg.HubURL)
	}
	if cfg.AgentKey != "sk-override" {
		t.Errorf("AgentKey = %q, want env override", cfg.AgentKey)
	}
}

func TestParseAgentConfig_Invalid(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "agent.yml")

	yaml := `agent_key: "sk-abc123"
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	// Ensure env var doesn't provide fallback
	t.Setenv("WHATSUPP_HUB_URL", "")

	_, err := ParseAgentConfig(cfgPath)
	if err == nil {
		t.Fatal("expected error for missing hub_url")
	}
}
