package api

import (
	"encoding/json"
	"net/http"
)

// Handlers holds dependencies for all API handlers.
type Handlers struct {
	store       Store
	hub         HubState
	configPath  string
	rateLimiter *LoginRateLimiter
}

// NewHandlers creates a new Handlers instance.
func NewHandlers(store Store, hub HubState, configPath string) *Handlers {
	return &Handlers{
		store:       store,
		hub:         hub,
		configPath:  configPath,
		rateLimiter: NewLoginRateLimiter(),
	}
}

// Health handles GET /api/v1/health — public, no auth.
func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
