package auth

import (
	"context"
	"net/http"
	"os"
)

type contextKey string

// steamIDContextKey holds the authenticated SteamID injected by Middleware.
const steamIDContextKey contextKey = "steamID"

// publicPaths are reachable without a session: the health check plus the login
// initiation, OpenID callback, and logout (which must work regardless of session
// state). Everything else requires authentication.
var publicPaths = map[string]bool{
	"/healthz":                 true,
	"/api/auth/steam/login":    true,
	"/api/auth/steam/callback": true,
	"/api/auth/logout":         true,
}

// Middleware gates every route. A request passes when it targets a public path,
// presents the parser shared secret (machine-to-machine callers), or carries a
// valid session cookie; otherwise it gets 401. On success for a cookie-based
// request the authenticated SteamID is stored in the request context.
func (h *Handler) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if publicPaths[r.URL.Path] {
			next.ServeHTTP(w, r)
			return
		}

		// Machine callers (the demo parser) authenticate with a shared secret
		// header rather than a Steam session.
		if secret := os.Getenv("PARSER_CALLBACK_SECRET"); secret != "" &&
			r.Header.Get("X-Parser-Secret") == secret {
			next.ServeHTTP(w, r)
			return
		}

		steamID, ok := h.cfg.readSession(r)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), steamIDContextKey, steamID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// steamIDFromContext returns the SteamID that Middleware stored for an
// authenticated request.
func steamIDFromContext(ctx context.Context) (int64, bool) {
	steamID, ok := ctx.Value(steamIDContextKey).(int64)
	return steamID, ok
}
