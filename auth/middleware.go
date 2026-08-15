package auth

import (
	"context"
	"net/http"
	"os"
	"strings"
)

type contextKey string

// steamIDContextKey holds the authenticated SteamID injected by Middleware.
const steamIDContextKey contextKey = "steamID"

// publicPaths are reachable without a token: the health check plus the login
// initiation and OpenID callback. Everything else requires authentication.
var publicPaths = map[string]bool{
	"/healthz":                 true,
	"/api/auth/steam/login":    true,
	"/api/auth/steam/callback": true,
}

// Middleware gates every route. A request passes when it targets a public path,
// presents the parser shared secret (machine-to-machine callers), or carries a
// valid bearer token; otherwise it gets 401. On success for a token-based request
// the authenticated SteamID is stored in the request context.
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

		steamID, ok := h.authenticate(r)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), steamIDContextKey, steamID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// authenticate extracts and verifies the bearer token from the Authorization
// header, returning the SteamID it encodes.
func (h *Handler) authenticate(r *http.Request) (int64, bool) {
	const prefix = "Bearer "
	authz := r.Header.Get("Authorization")
	if !strings.HasPrefix(authz, prefix) {
		return 0, false
	}
	return h.cfg.parseSession(strings.TrimPrefix(authz, prefix))
}

// steamIDFromContext returns the SteamID that Middleware stored for an
// authenticated request.
func steamIDFromContext(ctx context.Context) (int64, bool) {
	steamID, ok := ctx.Value(steamIDContextKey).(int64)
	return steamID, ok
}
