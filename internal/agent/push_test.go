package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestPushClient_Send(t *testing.T) {
	var receivedBody MetricBatch

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer srv.Close()

	pc := NewPushClient(srv.URL, "sk-test123")
	batch := MetricBatch{
		Host:      "testhost",
		Timestamp: time.Now(),
		Metrics:   []Metric{{Name: "cpu.usage_pct", Value: 42.0}},
	}

	err := pc.Send(context.Background(), batch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedBody.Host != "testhost" {
		t.Errorf("host = %q, want %q", receivedBody.Host, "testhost")
	}
	if len(receivedBody.Metrics) != 1 {
		t.Errorf("metrics count = %d, want 1", len(receivedBody.Metrics))
	}
}

func TestPushClient_BearerToken(t *testing.T) {
	var receivedAuth string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(200)
	}))
	defer srv.Close()

	pc := NewPushClient(srv.URL, "sk-mykey")
	batch := MetricBatch{
		Host:    "testhost",
		Metrics: []Metric{{Name: "test", Value: 1}},
	}
	pc.Send(context.Background(), batch)

	if receivedAuth != "Bearer sk-mykey" {
		t.Errorf("Authorization = %q, want %q", receivedAuth, "Bearer sk-mykey")
	}
}

func TestPushClient_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	pc := NewPushClient(srv.URL, "sk-test")
	err := pc.Send(context.Background(), MetricBatch{Host: "h", Metrics: []Metric{{Name: "t", Value: 1}}})
	if err != nil {
		t.Errorf("expected nil error, got: %v", err)
	}
}

func TestPushClient_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
	}))
	defer srv.Close()

	pc := NewPushClient(srv.URL, "bad-key")
	err := pc.Send(context.Background(), MetricBatch{Host: "h", Metrics: []Metric{{Name: "t", Value: 1}}})
	if err == nil {
		t.Fatal("expected error for 401")
	}
	if IsRetriable(err) {
		t.Error("401 should not be retriable")
	}
}

func TestPushClient_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	pc := NewPushClient(srv.URL, "sk-test")
	err := pc.Send(context.Background(), MetricBatch{Host: "h", Metrics: []Metric{{Name: "t", Value: 1}}})
	if err == nil {
		t.Fatal("expected error for 500")
	}
	if !IsRetriable(err) {
		t.Error("500 should be retriable")
	}
}

func TestPushClient_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Second)
	}))
	defer srv.Close()

	pc := NewPushClient(srv.URL, "sk-test")
	pc.client.Timeout = 100 * time.Millisecond

	err := pc.Send(context.Background(), MetricBatch{Host: "h", Metrics: []Metric{{Name: "t", Value: 1}}})
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !IsRetriable(err) {
		t.Error("timeout should be retriable")
	}
}

func TestPushClient_ConnectionRefused(t *testing.T) {
	pc := NewPushClient("http://127.0.0.1:1", "sk-test")
	err := pc.Send(context.Background(), MetricBatch{Host: "h", Metrics: []Metric{{Name: "t", Value: 1}}})
	if err == nil {
		t.Fatal("expected connection error")
	}
	if !IsRetriable(err) {
		t.Error("connection refused should be retriable")
	}
}
