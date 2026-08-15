// Package auth gates the API behind a Steam OpenID sign-in. It exposes the login
// flow (/api/auth/steam/*), a session-introspection endpoint (/api/auth/me),
// logout, and a Middleware that requires a valid session on every other route.
package auth

import (
	"errors"
	"os"
	"strings"
	"time"
)

// Config holds auth settings sourced from the environment.
type Config struct {
	// SessionSecret signs session cookies (HMAC-SHA256). Keep it secret and stable.
	SessionSecret []byte
	// AppBaseURL is the public origin the browser uses (the frontend static-site
	// origin, since the Steam round-trip happens entirely on that origin via the
	// Render rewrite/Vite proxy). Used to build OpenID realm/return_to and the
	// post-login redirect. No trailing slash.
	AppBaseURL string
	// CookieSecure controls the Secure attribute on the session cookie.
	CookieSecure bool
	// SessionTTL is how long a session cookie stays valid.
	SessionTTL time.Duration
}

// LoadConfig reads auth configuration from the environment. It returns an error
// when a required value (SESSION_SECRET, APP_BASE_URL) is missing so the server
// fails fast rather than issuing sessions it can never verify.
func LoadConfig() (*Config, error) {
	secret := os.Getenv("SESSION_SECRET")
	if secret == "" {
		return nil, errors.New("SESSION_SECRET is required")
	}

	appBaseURL := strings.TrimRight(os.Getenv("APP_BASE_URL"), "/")
	if appBaseURL == "" {
		return nil, errors.New("APP_BASE_URL is required (the public frontend origin, e.g. https://your-frontend.onrender.com)")
	}

	// Secure by default; opt out only for dev over a non-localhost http host.
	secure := os.Getenv("SESSION_COOKIE_SECURE") != "false"

	return &Config{
		SessionSecret: []byte(secret),
		AppBaseURL:    appBaseURL,
		CookieSecure:  secure,
		SessionTTL:    30 * 24 * time.Hour,
	}, nil
}
