package checks

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type HTTPChecker struct {
	URL                string
	Timeout            int
	InsecureSkipVerify bool
}

type httpMetadata struct {
	StatusCode   int    `json:"status_code"`
	CertExpiryAt *int64 `json:"cert_expiry_at,omitempty"`
	CertDaysLeft *int   `json:"cert_days_left,omitempty"`
}

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

func (c *HTTPChecker) IsHTTPS() bool {
	return strings.HasPrefix(c.URL, "https://")
}
