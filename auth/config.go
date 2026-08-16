// Package auth gates the API behind a Steam OpenID sign-in. It exposes the login
// flow (/api/auth/steam/*), a session-introspection endpoint (/api/auth/me), and
// a Middleware that requires a valid bearer token on every other route.
//
// Sessions are stateless signed tokens (no DB, no cookie): after Steam login the
// backend redirects the browser back to the frontend with the token in the URL
// fragment; the frontend stores it and sends it as `Authorization: Bearer`.
package auth

import (
	"errors"
	"os"
	"strings"
	"time"
)

// Config holds auth settings sourced from the environment.
type Config struct {
	// SessionSecret signs session tokens (HMAC-SHA256). Keep it secret and stable.
	SessionSecret []byte
	// FrontendURL is where the app lives; the callback redirects here with the
	// token (FrontendURL + "/#token=..."). No trailing slash.
	FrontendURL string
	// BackendURL is this backend's own public origin, used to build the OpenID
	// realm/return_to and to reconstruct the callback URL for verification
	// (rather than trusting r.Host behind Render's load balancer). No trailing slash.
	BackendURL string
	// SteamAPIKey is a Steam Web API key (https://steamcommunity.com/dev/apikey).
	// Optional: when set, /api/auth/me resolves the SteamID to a display name.
	// When empty, callers fall back to the numeric SteamID.
	SteamAPIKey string
	// SessionTTL is how long an issued token stays valid.
	SessionTTL time.Duration
}

// LoadConfig reads auth configuration from the environment. It returns an error
// when a required value is missing so the server fails fast rather than issuing
// tokens it can never verify (SESSION_SECRET) or building broken redirects
// (FRONTEND_URL / BACKEND_URL).
func LoadConfig() (*Config, error) {
	secret := os.Getenv("SESSION_SECRET")
	if secret == "" {
		return nil, errors.New("SESSION_SECRET is required")
	}

	frontendURL := strings.TrimRight(os.Getenv("FRONTEND_URL"), "/")
	if frontendURL == "" {
		return nil, errors.New("FRONTEND_URL is required (the app origin to return to after login, e.g. https://decamans.onrender.com)")
	}

	backendURL := strings.TrimRight(os.Getenv("BACKEND_URL"), "/")
	if backendURL == "" {
		return nil, errors.New("BACKEND_URL is required (this backend's own public origin, e.g. https://tenmansbackend.onrender.com)")
	}

	return &Config{
		SessionSecret: []byte(secret),
		FrontendURL:   frontendURL,
		BackendURL:    backendURL,
		SteamAPIKey:   os.Getenv("STEAM_API_KEY"),
		SessionTTL:    30 * 24 * time.Hour,
	}, nil
}
