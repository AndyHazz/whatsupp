package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// mockSessionStore implements the SessionStore interface for testing.
type mockSessionStore struct {
	sessions map[string]Session
}

func (m *mockSessionStore) GetSession(token string) (*Session, error) {
	s, ok := m.sessions[token]
	if !ok {
		return nil, nil
	}
	return &s, nil
}

func (m *mockSessionStore) RenewSession(token string, expiresAt time.Time) error {
	if s, ok := m.sessions[token]; ok {
		s.ExpiresAt = expiresAt
		m.sessions[token] = s
	}
	return nil
}

func TestAuthMiddleware_NoCookie_Returns401(t *testing.T) {
	store := &mockSessionStore{sessions: map[string]Session{}}
	mw := NewAuthMiddleware(store)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/monitors", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestAuthMiddleware_ValidCookie_Passes(t *testing.T) {
	store := &mockSessionStore{
		sessions: map[string]Session{
			"valid-token": {Token: "valid-token", UserID: 1, ExpiresAt: time.Now().Add(time.Hour)},
		},
	}
	mw := NewAuthMiddleware(store)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/monitors", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "valid-token"})
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestAuthMiddleware_ExpiredSession_Returns401(t *testing.T) {
	store := &mockSessionStore{
		sessions: map[string]Session{
			"expired-token": {Token: "expired-token", UserID: 1, ExpiresAt: time.Now().Add(-time.Hour)},
		},
	}
	mw := NewAuthMiddleware(store)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/monitors", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "expired-token"})
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}
