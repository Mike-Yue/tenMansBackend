package main

import (
	"net/http"
	"os"
	"strings"
)

// corsMiddleware adds CORS headers so a browser-based frontend on another origin
// can call the API. Allowed origins come from the CORS_ALLOWED_ORIGINS env var
// (comma-separated, e.g. "https://app.example.com,http://localhost:5173"). When
// it is unset, all origins are allowed — acceptable here because the API uses no
// cookies/credentials. It also answers CORS preflight (OPTIONS) requests.
func corsMiddleware(next http.Handler) http.Handler {
	allowed := parseAllowedOrigins(os.Getenv("CORS_ALLOWED_ORIGINS"))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && originAllowed(origin, allowed) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Add("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Access-Control-Max-Age", "86400")
		}

		// Preflight requests end here.
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func parseAllowedOrigins(env string) []string {
	origins := make([]string, 0)
	for _, part := range strings.Split(env, ",") {
		if p := strings.TrimSpace(part); p != "" {
			origins = append(origins, p)
		}
	}
	return origins
}

// originAllowed reports whether origin is permitted. An empty allow-list means
// "allow all"; a "*" entry also allows all.
func originAllowed(origin string, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, a := range allowed {
		if a == "*" || a == origin {
			return true
		}
	}
	return false
}
