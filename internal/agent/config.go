package agent

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// AgentConfig is the agent-side configuration loaded from /etc/whatsupp/agent.yml
type AgentConfig struct {
	HubURL     string        `yaml:"hub_url"`
	AgentKey   string        `yaml:"agent_key"`
	Hostname   string        `yaml:"hostname"`
	Interval   time.Duration `yaml:"interval"`
	HostFS     string        `yaml:"host_fs"`
	DockerHost string        `yaml:"docker_host"`
}

// ParseAgentConfig reads and parses the agent YAML config file.
func ParseAgentConfig(path string) (*AgentConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read agent config: %w", err)
	}

	var cfg AgentConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse agent config: %w", err)
	}

	// Environment variable overrides
	if v := os.Getenv("WHATSUPP_HUB_URL"); v != "" {
		cfg.HubURL = v
	}
	if v := os.Getenv("WHATSUPP_AGENT_KEY"); v != "" {
		cfg.AgentKey = v
	}

	// Defaults
	if cfg.Interval == 0 {
		cfg.Interval = 30 * time.Second
	}
	if cfg.Hostname == "" {
		hostname, err := os.Hostname()
		if err != nil {
			return nil, fmt.Errorf("detect hostname: %w", err)
		}
		cfg.Hostname = hostname
	}
	if cfg.DockerHost == "" {
		cfg.DockerHost = os.Getenv("DOCKER_HOST")
	}

	// Validation
	if cfg.HubURL == "" {
		return nil, fmt.Errorf("hub_url is required")
	}

	return &cfg, nil
}
