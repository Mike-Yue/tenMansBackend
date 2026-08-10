package stats

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
)

// StatsHandler translates HTTP requests into service calls for the stats domain.
type StatsHandler struct {
	svc StatsService
}

// NewStatsHandler returns a StatsHandler backed by the given service.
func NewStatsHandler(svc StatsService) *StatsHandler {
	return &StatsHandler{svc: svc}
}

// RegisterRoutes wires the stats endpoints onto the given mux. The stats domain
// lives under the user path since stats are always scoped to a user.
func (h *StatsHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/users/{id}/stats", h.GetUserStats)
}

// GetUserStats handles GET /api/users/{id}/stats — the {id} is the user's Steam
// ID. Returns aggregated all-time stats for that user.
func (h *StatsHandler) GetUserStats(w http.ResponseWriter, r *http.Request) {
	steamID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid user id", http.StatusBadRequest)
		return
	}

	stats, err := h.svc.GetPlayerStats(r.Context(), steamID)
	if err != nil {
		log.Printf("get stats %d: %v", steamID, err)
		http.Error(w, "failed to get stats", http.StatusInternalServerError)
		return
	}
	if stats == nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	writeJSON(w, stats)
}

// writeJSON encodes v as a JSON response body.
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("encode response: %v", err)
	}
}
