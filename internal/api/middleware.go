package api

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type contextKey string

const userIDKey contextKey = "userID"

// NewAuthMiddleware returns middleware that validates session cookies.
// Sessions are renewed on each authenticated request (sliding expiry).
func NewAuthMiddleware(store SessionStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("session")
			if err != nil {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			session, err := store.GetSession(cookie.Value)
			if err != nil || session == nil {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			if time.Now().After(session.ExpiresAt) {
				http.Error(w, `{"error":"session expired"}`, http.StatusUnauthorized)
				return
			}

			// Renew session (sliding expiry)
			newExpiry := time.Now().Add(sessionTTL)
			_ = store.RenewSession(session.Token, newExpiry)

			ctx := context.WithValue(r.Context(), userIDKey, session.UserID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// AgentKeyAuth validates agent bearer tokens.
// agentKeys is a map of host name -> plain-text agent key.
func AgentKeyAuth(agentKeys map[string]string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			if !strings.HasPrefix(auth, "Bearer ") {
				http.Error(w, `{"error":"missing bearer token"}`, http.StatusUnauthorized)
				return
			}
			token := strings.TrimPrefix(auth, "Bearer ")

			// Compare provided token against stored keys using constant-time comparison
			valid := false
			for _, storedKey := range agentKeys {
				if subtle.ConstantTimeCompare([]byte(token), []byte(storedKey)) == 1 {
					valid = true
					break
				}
			}

			if !valid {
				http.Error(w, `{"error":"invalid agent key"}`, http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// jsonError writes a JSON error response.
func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
