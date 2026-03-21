package web

import (
	"io/fs"
	"net/http"
	"strings"
)

// Handler returns an http.Handler that serves the embedded SPA.
// For any path that does not match a static file, it serves index.html
// to support client-side routing.
func Handler() http.Handler {
	// Strip the "dist/" prefix from the embedded filesystem
	distFS, err := fs.Sub(DistFS, "dist")
	if err != nil {
		panic("failed to create sub filesystem: " + err.Error())
	}

	fileServer := http.FileServer(http.FS(distFS))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Don't serve SPA for API routes
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}

		// Try to serve the file directly
		path := r.URL.Path
		if path == "/" {
			path = "/index.html"
		}

		// Check if the file exists in the embedded FS
		_, err := distFS.(fs.ReadFileFS).ReadFile(strings.TrimPrefix(path, "/"))
		if err != nil {
			// File not found — serve index.html for SPA routing
			indexData, err := distFS.(fs.ReadFileFS).ReadFile("index.html")
			if err != nil {
				http.Error(w, "index.html not found", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write(indexData)
			return
		}

		// File exists — let the standard file server handle it
		// (this sets correct content types and caching headers)
		fileServer.ServeHTTP(w, r)
	})
}
