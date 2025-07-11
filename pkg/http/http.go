package http

import (
	"net/http"
	"strings"
)

// HandleFileServer returns a handler that serves static files
func HandleFileServer(fs http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Remove any query parameters
		path := r.URL.Path
		if idx := strings.Index(path, "?"); idx != -1 {
			path = path[:idx]
		}

		// Create a new request with cleaned path
		r.URL.Path = path

		// Set appropriate headers for static files
		if strings.HasSuffix(path, ".js") {
			w.Header().Set("Content-Type", "application/javascript")
		} else if strings.HasSuffix(path, ".css") {
			w.Header().Set("Content-Type", "text/css")
		} else if strings.HasSuffix(path, ".html") {
			w.Header().Set("Content-Type", "text/html")
		}

		fs.ServeHTTP(w, r)
	}
}
