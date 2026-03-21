package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// PushClient sends metric batches to the hub.
type PushClient struct {
	hubURL string
	apiKey string
	client *http.Client
}

// NewPushClient creates a new push client.
func NewPushClient(hubURL, apiKey string) *PushClient {
	return &PushClient{
		hubURL: hubURL,
		apiKey: apiKey,
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

// Send posts a metric batch to the hub.
func (p *PushClient) Send(ctx context.Context, batch MetricBatch) error {
	body, err := json.Marshal(batch)
	if err != nil {
		return fmt.Errorf("marshal batch: %w", err)
	}

	url := p.hubURL + "/api/v1/agent/metrics"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return &RetriableError{Err: fmt.Errorf("send metrics: %w", err)}
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return nil
	case resp.StatusCode == 401:
		return fmt.Errorf("unauthorized: invalid agent key")
	case resp.StatusCode >= 500:
		return &RetriableError{Err: fmt.Errorf("server error: %d", resp.StatusCode)}
	default:
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}
}

// RetriableError indicates the operation can be retried.
type RetriableError struct {
	Err error
}

func (e *RetriableError) Error() string { return e.Err.Error() }
func (e *RetriableError) Unwrap() error { return e.Err }

// IsRetriable checks if an error is retriable.
func IsRetriable(err error) bool {
	_, ok := err.(*RetriableError)
	return ok
}
