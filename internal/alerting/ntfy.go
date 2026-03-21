package alerting

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type NtfyConfig struct {
	URL              string
	Topic            string
	Username         string
	Password         string
	Token            string // Bearer token auth (alternative to username/password)
	ReminderInterval time.Duration
}

type NtfyClient struct {
	config NtfyConfig
	client *http.Client

	mu            sync.Mutex
	lastDownAlert map[string]time.Time
}

type ntfyMessage struct {
	Topic    string `json:"topic"`
	Title    string `json:"title"`
	Message  string `json:"message"`
	Priority int    `json:"priority"`
	Tags     string `json:"tags,omitempty"`
}

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

func (n *NtfyClient) SendDown(monitor, cause string) error {
	n.mu.Lock()
	lastSent, exists := n.lastDownAlert[monitor]
	now := time.Now()
	if exists && now.Sub(lastSent) < n.config.ReminderInterval {
		n.mu.Unlock()
		return nil
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

	if n.config.Token != "" {
		req.Header.Set("Authorization", "Bearer "+n.config.Token)
	} else if n.config.Username != "" && n.config.Password != "" {
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
