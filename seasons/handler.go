package seasons

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
)

// SeasonHandler translates HTTP requests into service calls for the seasons
// domain (exposed under /api/seasons).
type SeasonHandler struct {
	svc SeasonService
}

// NewSeasonHandler returns a SeasonHandler backed by the given service.
func NewSeasonHandler(svc SeasonService) *SeasonHandler {
	return &SeasonHandler{svc: svc}
}

// RegisterRoutes wires the season endpoints onto the given mux.
func (h *SeasonHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/seasons", h.ListSeasons)
	mux.HandleFunc("POST /api/seasons", h.CreateSeason)
	mux.HandleFunc("DELETE /api/seasons/{id}", h.DeleteSeason)
}

// DeleteSeason handles DELETE /api/seasons/{id} — remove a season that has no
// associated matches.
func (h *SeasonHandler) DeleteSeason(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid season id", http.StatusBadRequest)
		return
	}

	found, err := h.svc.DeleteSeason(r.Context(), id)
	if errors.Is(err, ErrSeasonReferenced) {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	if err != nil {
		log.Printf("delete season %d: %v", id, err)
		http.Error(w, "failed to delete season", http.StatusInternalServerError)
		return
	}
	if !found {
		http.Error(w, "season not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListSeasons handles GET /api/seasons — list all seasons, newest first.
func (h *SeasonHandler) ListSeasons(w http.ResponseWriter, r *http.Request) {
	seasons, err := h.svc.ListSeasons(r.Context())
	if err != nil {
		log.Printf("list seasons: %v", err)
		http.Error(w, "failed to list seasons", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, seasons)
}

type createSeasonRequest struct {
	Name    string `json:"name"`
	StartAt string `json:"startAt"`
	EndAt   string `json:"endAt"`
}

// CreateSeason handles POST /api/seasons — create a season.
// Body: { name, startAt, endAt } with dates as "YYYY-MM-DD".
func (h *SeasonHandler) CreateSeason(w http.ResponseWriter, r *http.Request) {
	var req createSeasonRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	season, err := h.svc.CreateSeason(r.Context(), req.Name, req.StartAt, req.EndAt)
	if errors.Is(err, ErrInvalidSeason) {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err != nil {
		log.Printf("create season: %v", err)
		http.Error(w, "failed to create season", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, season)
}

// writeJSON writes v as a JSON response body with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("encode response: %v", err)
	}
}
