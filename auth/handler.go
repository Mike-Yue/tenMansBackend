package auth

import (
	"encoding/json"
	"log"
	"net/http"

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

// RegisterRoutes wires the auth endpoints onto the given mux. login, callback,
// and logout are public (see publicPaths in the middleware); me is protected, so
// a 401 there is the frontend's "not signed in" signal.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/auth/steam/login", h.Login)
	mux.HandleFunc("GET /api/auth/steam/callback", h.Callback)
	mux.HandleFunc("GET /api/auth/me", h.Me)
	mux.HandleFunc("POST /api/auth/logout", h.Logout)
}

// Login handles GET /api/auth/steam/login — redirect the browser to Steam's
// OpenID login. realm/return_to are built from the configured public origin.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	returnTo := h.cfg.AppBaseURL + "/api/auth/steam/callback"
	url := steamLoginRedirectURL(returnTo, h.cfg.AppBaseURL)
	http.Redirect(w, r, url, http.StatusFound)
}

// Callback handles GET /api/auth/steam/callback — verify Steam's assertion, set
// the session cookie, and redirect back to the app.
func (h *Handler) Callback(w http.ResponseWriter, r *http.Request) {
	// Rebuild the verification URL from the configured public origin rather than
	// r.Host: behind the frontend's rewrite proxy r.Host is the backend's host,
	// but Steam signed the assertion against the frontend origin in return_to.
	fullURL := h.cfg.AppBaseURL + r.URL.RequestURI()

	steamID, err := verifySteamLogin(fullURL)
	if err != nil {
		log.Printf("steam verify: %v", err)
		http.Error(w, "steam login verification failed", http.StatusUnauthorized)
		return
	}

	h.cfg.setSessionCookie(w, steamID)
	http.Redirect(w, r, h.cfg.AppBaseURL+"/", http.StatusFound)
}

// meResponse is the payload for GET /api/auth/me. SteamID serializes as a bare
// int64; the frontend's big-int-safe JSON parser quotes it into a string.
type meResponse struct {
	SteamID       int64   `json:"steamId"`
	SteamUsername *string `json:"steamUsername"`
}

// Me handles GET /api/auth/me — return the signed-in user's identity. The
// middleware guarantees a session here (otherwise it returns 401), so the SteamID
// is always present in context. The username is filled in only when this Steam
// user already has a player row (from match ingestion); pure viewers have none.
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	steamID, ok := steamIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	resp := meResponse{SteamID: steamID}
	if user, err := h.userSvc.GetUserBySteamID(r.Context(), steamID); err != nil {
		log.Printf("me lookup %d: %v", steamID, err)
	} else if user != nil {
		resp.SteamUsername = user.SteamUsername
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("encode me: %v", err)
	}
}

// Logout handles POST /api/auth/logout — clear the session cookie. Safe to call
// whether or not a valid session exists.
func (h *Handler) Logout(w http.ResponseWriter, _ *http.Request) {
	h.cfg.clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}
