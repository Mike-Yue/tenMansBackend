package users

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
)

// UserHandler translates HTTP requests into service calls for the users domain
// (exposed under /api/users).
type UserHandler struct {
	svc UserService
}

// NewUserHandler returns a UserHandler backed by the given service.
func NewUserHandler(svc UserService) *UserHandler {
	return &UserHandler{svc: svc}
}

// RegisterRoutes wires the user endpoints onto the given mux.
func (h *UserHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/users", h.ListUsers)
	mux.HandleFunc("GET /api/users/{id}", h.GetUser)
}

// ListUsers handles GET /api/users — list all users.
func (h *UserHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.svc.ListUsers(r.Context())
	if err != nil {
		log.Printf("list users: %v", err)
		http.Error(w, "failed to list users", http.StatusInternalServerError)
		return
	}

	writeJSON(w, users)
}

// GetUser handles GET /api/users/{id} — the {id} is the user's Steam ID.
func (h *UserHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	steamID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid user id", http.StatusBadRequest)
		return
	}

	user, err := h.svc.GetUserBySteamID(r.Context(), steamID)
	if err != nil {
		log.Printf("get user %d: %v", steamID, err)
		http.Error(w, "failed to get user", http.StatusInternalServerError)
		return
	}
	if user == nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	writeJSON(w, user)
}

// writeJSON encodes v as a JSON response body.
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("encode response: %v", err)
	}
}
