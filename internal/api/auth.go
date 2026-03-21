package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	sessionTTL      = 24 * time.Hour
	bcryptCost      = 12
	sessionTokenLen = 32 // 32 bytes = 64 hex chars

	rateLimitPerMinute = 5
	lockoutThreshold   = 10
	lockoutDuration    = 15 * time.Minute
	rateLimitWindow    = 1 * time.Minute
)

// HashPassword hashes a plaintext password with bcrypt.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CheckPassword compares a plaintext password against a bcrypt hash.
func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// GenerateSessionToken creates a cryptographically random session token.
func GenerateSessionToken() (string, error) {
	b := make([]byte, sessionTokenLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// loginRecord tracks attempts per IP.
type loginRecord struct {
	attempts    []time.Time // timestamps of recent attempts within the window
	failures    int         // total consecutive failures
	lockedUntil time.Time
}

// LoginRateLimiter enforces per-IP login rate limits.
type LoginRateLimiter struct {
	mu      sync.Mutex
	records map[string]*loginRecord
}

func NewLoginRateLimiter() *LoginRateLimiter {
	return &LoginRateLimiter{
		records: make(map[string]*loginRecord),
	}
}

// Allow returns true if the IP is allowed to attempt login.
func (rl *LoginRateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rec := rl.records[ip]
	if rec == nil {
		rec = &loginRecord{}
		rl.records[ip] = rec
	}

	now := time.Now()

	// Check lockout
	if now.Before(rec.lockedUntil) {
		return false
	}

	// Prune old attempts outside the window
	cutoff := now.Add(-rateLimitWindow)
	valid := rec.attempts[:0]
	for _, t := range rec.attempts {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	rec.attempts = valid

	// Check per-minute limit
	if len(rec.attempts) >= rateLimitPerMinute {
		return false
	}

	rec.attempts = append(rec.attempts, now)
	return true
}

// RecordFailure increments the failure counter and applies lockout if threshold hit.
func (rl *LoginRateLimiter) RecordFailure(ip string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rec := rl.records[ip]
	if rec == nil {
		rec = &loginRecord{}
		rl.records[ip] = rec
	}

	rec.failures++
	if rec.failures >= lockoutThreshold {
		rec.lockedUntil = time.Now().Add(lockoutDuration)
	}
}

// RecordSuccess resets the failure counter for an IP.
func (rl *LoginRateLimiter) RecordSuccess(ip string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if rec, ok := rl.records[ip]; ok {
		rec.failures = 0
		rec.lockedUntil = time.Time{}
	}
}

// Cleanup removes stale records that have no recent attempts and are not locked.
// Should be called periodically (e.g. every hour).
func (rl *LoginRateLimiter) Cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rateLimitWindow * 2) // 2x the window to be safe
	for ip, rec := range rl.records {
		// Skip if still locked
		if now.Before(rec.lockedUntil) {
			continue
		}
		// Remove if no recent attempts
		hasRecent := false
		for _, t := range rec.attempts {
			if t.After(cutoff) {
				hasRecent = true
				break
			}
		}
		if !hasRecent && rec.failures == 0 {
			delete(rl.records, ip)
		}
	}
}

// loginRequest is the JSON body for POST /auth/login.
type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Login handles POST /api/v1/auth/login.
func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	// Extract client IP (handle X-Forwarded-For for reverse proxy)
	ip := extractIP(r)

	if !h.rateLimiter.Allow(ip) {
		jsonError(w, "too many login attempts", http.StatusTooManyRequests)
		return
	}

	var req loginRequest
	r.Body = http.MaxBytesReader(w, r.Body, 4096) // 4KB limit for login payload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	user, err := h.store.GetUserByUsername(req.Username)
	if err != nil || user == nil {
		h.rateLimiter.RecordFailure(ip)
		jsonError(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	if !CheckPassword(user.PasswordHash, req.Password) {
		h.rateLimiter.RecordFailure(ip)
		jsonError(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	h.rateLimiter.RecordSuccess(ip)

	token, err := GenerateSessionToken()
	if err != nil {
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}

	expiresAt := time.Now().Add(sessionTTL)
	if err := h.store.CreateSession(token, user.ID, expiresAt); err != nil {
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		Expires:  expiresAt,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// Logout handles POST /api/v1/auth/logout.
func (h *Handlers) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session")
	if err == nil {
		_ = h.store.DeleteSession(cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// extractIP gets the client IP from RemoteAddr.
// We rely on chi's RealIP middleware to set RemoteAddr from trusted proxy headers,
// so we do NOT parse X-Forwarded-For here (avoids spoofing that bypasses rate limiting).
func extractIP(r *http.Request) string {
	// RemoteAddr is already set by chi's RealIP middleware if behind a reverse proxy.
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// EnsureAdminUser creates the initial admin user from config if no users exist.
// Called once during hub startup.
func EnsureAdminUser(store UserStore, username, password string) error {
	count, err := store.UserCount()
	if err != nil {
		return fmt.Errorf("checking user count: %w", err)
	}
	if count > 0 {
		return nil // users already exist, skip
	}

	if username == "" || password == "" {
		return fmt.Errorf("initial_username and initial_password must be set in config when no users exist")
	}

	hash, err := HashPassword(password)
	if err != nil {
		return fmt.Errorf("hashing initial password: %w", err)
	}

	if err := store.CreateUser(username, hash); err != nil {
		return fmt.Errorf("creating initial admin user: %w", err)
	}

	return nil
}
