package auth

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"

	"tenMansBackend/users"
)

// Handler serves the Steam OpenID login flow and session endpoints, and owns the
// auth Middleware.
type Handler struct {
	cfg     *Config
	userSvc users.UserService
}

// NewHandler returns an auth Handler backed by the given user service and config.
func NewHandler(userSvc users.UserService, cfg *Config) *Handler {
	return &Handler{cfg: cfg, userSvc: userSvc}
}

// RegisterRoutes wires the auth endpoints onto the given mux. login and callback
// are public (see publicPaths in the middleware); me is protected, so a 401 there
// is the frontend's "not signed in" signal. There is no server-side logout:
// tokens are stateless, so the frontend logs out by discarding its token.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/auth/steam/login", h.Login)
	mux.HandleFunc("GET /api/auth/steam/callback", h.Callback)
	mux.HandleFunc("GET /api/auth/me", h.Me)
}

// Login handles GET /api/auth/steam/login — redirect the browser to Steam's
// OpenID login. realm/return_to are built from this backend's own public origin,
// because the callback lands here on the backend (not the frontend).
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	returnTo := h.cfg.BackendURL + "/api/auth/steam/callback"
	url := steamLoginRedirectURL(returnTo, h.cfg.BackendURL)
	http.Redirect(w, r, url, http.StatusFound)
}

// Callback handles GET /api/auth/steam/callback — verify Steam's assertion, sign
// a session token, and redirect back to the frontend with the token in the URL
// fragment (`#token=...`). A fragment is not sent to servers, so the token stays
// out of access logs and Referer headers.
func (h *Handler) Callback(w http.ResponseWriter, r *http.Request) {
	// Rebuild the verification URL from the configured public origin rather than
	// r.Host: behind Render's TLS-terminating load balancer r.Host/scheme aren't
	// the public https origin, but Steam signed the assertion against BackendURL
	// in return_to.
	fullURL := h.cfg.BackendURL + r.URL.RequestURI()

	steamID, err := verifySteamLogin(fullURL)
	if err != nil {
		log.Printf("steam verify: %v", err)
		http.Error(w, "steam login verification failed", http.StatusUnauthorized)
		return
	}

	token := h.cfg.issueToken(steamID)
	http.Redirect(w, r, h.cfg.FrontendURL+"/#token="+url.QueryEscape(token), http.StatusFound)
}

// meResponse is the payload for GET /api/auth/me. SteamID serializes as a bare
// int64; the frontend's big-int-safe JSON parser quotes it into a string.
type meResponse struct {
	SteamID       int64   `json:"steamId"`
	SteamUsername *string `json:"steamUsername"`
	AvatarURL     *string `json:"avatarUrl"`
}

// Me handles GET /api/auth/me — return the signed-in user's identity. The
// middleware guarantees a valid token here (otherwise it returns 401), so the
// SteamID is always present in context. The username is filled in only when this
// Steam user already has a player row (from match ingestion); pure viewers have none.
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	steamID, ok := steamIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	resp := meResponse{SteamID: steamID}
	// Prefer the live Steam profile (requires STEAM_API_KEY): its display name,
	// falling back to a stored username, then to null (the frontend shows the
	// SteamID); plus the avatar when available.
	profile := h.cfg.steamProfileFor(steamID)
	if profile.Name != "" {
		resp.SteamUsername = &profile.Name
	} else if user, err := h.userSvc.GetUserBySteamID(r.Context(), steamID); err != nil {
		log.Printf("me lookup %d: %v", steamID, err)
	} else if user != nil {
		resp.SteamUsername = user.SteamUsername
	}
	if profile.Avatar != "" {
		resp.AvatarURL = &profile.Avatar
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("encode me: %v", err)
	}
}
