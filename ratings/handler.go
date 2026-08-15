package ratings

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
)

// RatingHandler translates HTTP requests into service calls for the ratings
// domain. Per-player reads live under the user path (like stats); the recompute
// action lives under /api/ratings.
type RatingHandler struct {
	svc RatingService
}

// NewRatingHandler returns a RatingHandler backed by the given service.
func NewRatingHandler(svc RatingService) *RatingHandler {
	return &RatingHandler{svc: svc}
}

// RegisterRoutes wires the rating endpoints onto the given mux.
func (h *RatingHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/users/{id}/ratings", h.GetUserRatings)
	mux.HandleFunc("POST /api/ratings/recompute", h.Recompute)
}

// GetUserRatings handles GET /api/users/{id}/ratings — {id} is the Steam ID.
// Returns the player's per-season ratings.
func (h *RatingHandler) GetUserRatings(w http.ResponseWriter, r *http.Request) {
	steamID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid user id", http.StatusBadRequest)
		return
	}

	ratings, exists, err := h.svc.GetPlayerRatings(r.Context(), steamID)
	if err != nil {
		log.Printf("get ratings %d: %v", steamID, err)
		http.Error(w, "failed to get ratings", http.StatusInternalServerError)
		return
	}
	if !exists {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	writeJSON(w, ratings)
}

// Recompute handles POST /api/ratings/recompute — rebuild every season's ratings
// from match history. Used to backfill on first deploy and to repair.
func (h *RatingHandler) Recompute(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.RecomputeAll(r.Context()); err != nil {
		log.Printf("recompute ratings: %v", err)
		http.Error(w, "failed to recompute ratings", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// writeJSON encodes v as a JSON response body.
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("encode response: %v", err)
	}
}
