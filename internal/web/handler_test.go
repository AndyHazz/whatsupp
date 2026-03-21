package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandler_ServesIndexForRoot(t *testing.T) {
	handler := Handler()

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("expected HTML content type, got %s", rec.Header().Get("Content-Type"))
	}
}

func TestHandler_FallbackToIndex(t *testing.T) {
	handler := Handler()

	req := httptest.NewRequest("GET", "/monitors/Plex", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for SPA fallback, got %d", rec.Code)
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("expected HTML content type for SPA fallback, got %s", rec.Header().Get("Content-Type"))
	}
}

func TestHandler_ApiPathsNotServed(t *testing.T) {
	handler := Handler()

	req := httptest.NewRequest("GET", "/api/v1/monitors", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for API path, got %d", rec.Code)
	}
}
