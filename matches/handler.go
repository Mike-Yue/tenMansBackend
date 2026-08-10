package matches

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
)

// MatchResponse is the API DTO for a match: it defines the JSON shape the API
// exposes, independent of the DB schema. Note it omits UploadHash — an internal
// dedup key clients have no need for — and owns the json field names, so the
// Matches table can be refactored without breaking the API contract.
type MatchResponse struct {
	ID          int64  `json:"id"`
	Map         string `json:"map"`
	PlayedAt    string `json:"playedAt"`
	UploadedAt  string `json:"uploadedAt"`
	Processed   bool   `json:"processed"`
	SeasonID    int64  `json:"seasonId"`
	TotalRounds int64  `json:"totalRounds"`
}

// toResponse maps the DB model onto the API DTO. This is the single place that
// translates between the two shapes.
func toResponse(m Match) MatchResponse {
	return MatchResponse{
		ID:          m.ID,
		Map:         m.Map,
		PlayedAt:    m.PlayedAt,
		UploadedAt:  m.UploadedAt,
		Processed:   m.Processed,
		SeasonID:    m.SeasonID,
		TotalRounds: m.TotalRounds,
	}
}

// MatchHandler translates HTTP requests into service calls for the matches domain.
type MatchHandler struct {
	svc MatchService
}

// NewMatchHandler returns a MatchHandler backed by the given service.
func NewMatchHandler(svc MatchService) *MatchHandler {
	return &MatchHandler{svc: svc}
}

// RegisterRoutes wires the matches endpoints onto the given mux.
func (h *MatchHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/matches", h.ListMatches)
	mux.HandleFunc("GET /api/matches/{matchId}", h.GetMatch)
}

// ListMatches handles GET /api/matches?season={id}. The season query param is
// optional; when omitted, all matches are returned.
func (h *MatchHandler) ListMatches(w http.ResponseWriter, r *http.Request) {
	var seasonID *int64
	if s := r.URL.Query().Get("season"); s != "" {
		parsed, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			http.Error(w, "invalid season", http.StatusBadRequest)
			return
		}
		seasonID = &parsed
	}

	matches, err := h.svc.ListMatches(r.Context(), seasonID)
	if err != nil {
		log.Printf("list matches: %v", err)
		http.Error(w, "failed to list matches", http.StatusInternalServerError)
		return
	}

	resp := make([]MatchResponse, 0, len(matches))
	for _, m := range matches {
		resp = append(resp, toResponse(m))
	}
	writeJSON(w, resp)
}

// GetMatch handles GET /api/matches/{matchId} — return a single match.
func (h *MatchHandler) GetMatch(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("matchId"), 10, 64)
	if err != nil {
		http.Error(w, "invalid match id", http.StatusBadRequest)
		return
	}

	match, err := h.svc.GetMatch(r.Context(), id)
	if errors.Is(err, errNotImplemented) {
		http.Error(w, "not implemented", http.StatusNotImplemented)
		return
	}
	if err != nil {
		log.Printf("get match %d: %v", id, err)
		http.Error(w, "failed to get match", http.StatusInternalServerError)
		return
	}
	if match == nil {
		http.Error(w, "match not found", http.StatusNotFound)
		return
	}

	writeJSON(w, toResponse(*match))
}

// writeJSON encodes v as a JSON response body.
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("encode response: %v", err)
	}
}
