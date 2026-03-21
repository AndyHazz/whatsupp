package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "agent.yml")

	err := GenerateConfig(cfgPath, "https://hub.example.com", "sk-test123", "myhost")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "hub.example.com") {
		t.Error("config should contain hub URL")
	}
	if !strings.Contains(content, "sk-test123") {
		t.Error("config should contain agent key")
	}
	if !strings.Contains(content, "myhost") {
		t.Error("config should contain hostname")
	}

	// Verify the generated config can be parsed
	cfg, err := ParseAgentConfig(cfgPath)
	if err != nil {
		t.Fatalf("parse generated config: %v", err)
	}
	if cfg.HubURL != "https://hub.example.com" {
		t.Errorf("HubURL = %q, want %q", cfg.HubURL, "https://hub.example.com")
	}
}

func TestGenerateConfig_DetectsHostname(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "agent.yml")

	err := GenerateConfig(cfgPath, "https://hub.example.com", "sk-test123", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg, err := ParseAgentConfig(cfgPath)
	if err != nil {
		t.Fatalf("parse generated config: %v", err)
	}

	expected, _ := os.Hostname()
	if cfg.Hostname != expected {
		t.Errorf("Hostname = %q, want %q (auto-detected)", cfg.Hostname, expected)
	}
}

func TestGenerateConfig_NoOverwrite(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "agent.yml")

	// Create existing file
	if err := os.WriteFile(cfgPath, []byte("existing"), 0644); err != nil {
		t.Fatal(err)
	}

	err := GenerateConfig(cfgPath, "https://hub.example.com", "sk-test123", "myhost")
	if err == nil {
		t.Fatal("expected error for existing file")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error should mention 'already exists', got: %v", err)
	}
}
