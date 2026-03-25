package config

import (
    "fmt"
    "os"
    "regexp"
    "time"

    "gopkg.in/yaml.v3"
)

type Config struct {
    Server        ServerConfig    `yaml:"server"`
    Auth          AuthConfig      `yaml:"auth"`
    Monitors      []Monitor       `yaml:"monitors"`
    Agents        []AgentConfig   `yaml:"agents"`
    Security      SecurityConfig  `yaml:"security"`
    Alerting      AlertingConfig  `yaml:"alerting"`
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
    Name               string        `yaml:"name"`
    Type               string        `yaml:"type"`
    URL                string        `yaml:"url,omitempty"`
    Host               string        `yaml:"host,omitempty"`
    Port               int           `yaml:"port,omitempty"`
    Query              string        `yaml:"query,omitempty"` // dns: domain to resolve (default "google.com")
    Group              string        `yaml:"group,omitempty"` // agent hostname to group this monitor under
    Interval           time.Duration `yaml:"interval"`
    FailureThreshold   int           `yaml:"failure_threshold,omitempty"`
    InsecureSkipVerify bool          `yaml:"insecure_skip_verify,omitempty"`
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
    Token    string `yaml:"token,omitempty"`
}

type ThresholdsConfig struct {
    SSLExpiryDays        []int         `yaml:"ssl_expiry_days"`
    DiskUsagePct         int           `yaml:"disk_usage_pct"`
    DiskHysteresisPct    int           `yaml:"disk_hysteresis_pct"`
    DownReminderInterval time.Duration `yaml:"down_reminder_interval"`
}

var envVarRe = regexp.MustCompile(`\$\{([^}]+)\}`)

func expandEnvVars(s string) string {
    return envVarRe.ReplaceAllStringFunc(s, func(match string) string {
        varName := envVarRe.FindStringSubmatch(match)[1]
        return os.Getenv(varName)
    })
}

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
        case "dns":
            if m.Host == "" {
                return fmt.Errorf("monitor %q: dns type requires host", m.Name)
            }
        default:
            return fmt.Errorf("monitor %q: unknown type %q", m.Name, m.Type)
        }
    }
    return nil
}
