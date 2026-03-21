package api

import (
	"log"
	"net/http"
	"time"

	"github.com/andyhazz/whatsupp/internal/web"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// RouterConfig holds the dependencies needed to build the API router.
type RouterConfig struct {
	Store      Store
	Hub        HubState
	ConfigPath string
	AgentKeys  map[string]string // host name -> SHA-256 hash of agent key
	BackupDir  string
}

// RouterResult holds the router and its dependencies that callers need access to.
type RouterResult struct {
	Handler http.Handler
	WSHub   *WSHub
}

// NewRouter creates and returns the fully-wired chi router.
func NewRouter(cfg RouterConfig) RouterResult {
	r := chi.NewRouter()

	// Global middleware
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(securityHeaders)

	// Create WebSocket hub
	wsHub := NewWSHub()
	go wsHub.Run()

	// Create handlers
	h := &Handlers{
		store:       cfg.Store,
		hub:         cfg.Hub,
		configPath:  cfg.ConfigPath,
		rateLimiter: NewLoginRateLimiter(),
		wsHub:       wsHub,
		backupDir:   cfg.BackupDir,
	}

	// Session auth middleware
	authMW := NewAuthMiddleware(cfg.Store)

	// Agent key auth middleware
	agentMW := AgentKeyAuth(cfg.AgentKeys)

	// Start session cleanup goroutine (runs every hour)
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			if err := cfg.Store.DeleteExpiredSessions(); err != nil {
				log.Printf("session cleanup error: %v", err)
			}
			h.rateLimiter.Cleanup()
		}
	}()

	r.Route("/api/v1", func(r chi.Router) {
		// Public routes (no auth)
		r.Get("/health", h.Health)
		r.Post("/auth/login", h.Login)
		r.Post("/auth/logout", h.Logout)

		// Agent routes (bearer token auth)
		r.Group(func(r chi.Router) {
			r.Use(agentMW)
			r.Post("/agent/metrics", h.PostAgentMetrics)
		})

		// Authenticated routes (session cookie)
		r.Group(func(r chi.Router) {
			r.Use(authMW)

			// Config
			r.Get("/config", h.GetConfig)
			r.Put("/config", h.PutConfig)

			// Test notifications
			r.Post("/test-ntfy", h.TestNtfy)

			// Monitors
			r.Get("/monitors", h.ListMonitors)
			r.Get("/monitors/{name}", h.GetMonitor)
			r.Get("/monitors/{name}/results", h.GetMonitorResults)

			// Hosts
			r.Get("/hosts", h.ListHosts)
			r.Get("/hosts/{name}", h.GetHost)
			r.Get("/hosts/{name}/metrics", h.GetHostMetrics)

			// Incidents
			r.Get("/incidents", h.ListIncidents)

			// Security
			r.Get("/security/scans", h.ListSecurityScans)
			r.Get("/security/baselines", h.ListSecurityBaselines)
			r.Post("/security/baselines/{target}", h.AcceptBaseline)

			// Admin
			r.Get("/admin/backup", h.Backup)

			// WebSocket (auth handled inside the handler via cookie)
			r.Get("/ws", h.HandleWebSocket)
		})
	})

	// Serve SPA for all non-API routes (catch-all, registered after API routes)
	r.Handle("/*", web.Handler())

	return RouterResult{
		Handler: r,
		WSHub:   wsHub,
	}
}

