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
	if result.MetadataJSON == "" {
		t.Error("MetadataJSON should contain SSL cert info for HTTPS")
	}
}
