package agent

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// GenerateConfig creates a new agent config YAML file.
// Returns error if the file already exists (safety).
func GenerateConfig(path, hubURL, key, hostname string) error {
	// Safety check: don't overwrite existing config
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("config file already exists: %s", path)
	}

	if hostname == "" {
		h, err := os.Hostname()
		if err != nil {
			return fmt.Errorf("detect hostname: %w", err)
		}
		hostname = h
	}

	cfg := AgentConfig{
		HubURL:   hubURL,
		AgentKey: key,
		Hostname: hostname,
		Interval: 0, // will use default 30s
		HostFS:   "/hostfs",
	}

	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}
