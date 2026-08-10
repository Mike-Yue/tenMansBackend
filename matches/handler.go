package matches

import (
	"encoding/json"
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

// MatchDetailResponse is the API DTO for GET /api/matches/{matchId}: the base
// match fields plus each team and its players' scoreboard.
type MatchDetailResponse struct {
	MatchResponse
	Teams []TeamResponse `json:"teams"`
}

type TeamResponse struct {
	ID           int64                `json:"id"`
	TeamSlot     string               `json:"teamSlot"`
	StartingSide string               `json:"startingSide"`
	RoundsWon    int64                `json:"roundsWon"`
	Result       string               `json:"result"`
	Players      []PlayerStatResponse `json:"players"`
}

type PlayerStatResponse struct {
	PlayerID      int64   `json:"playerId"`
	SteamID       int64   `json:"steamId"`
	SteamUsername *string `json:"steamUsername"`
	Kills         int64   `json:"kills"`
	Deaths        int64   `json:"deaths"`
	Assists       int64   `json:"assists"`
	KDRatio       float64 `json:"kdRatio"`
	MVPs          int64   `json:"mvps"`
}

// toDetailResponse maps the MatchDetail domain model onto its API DTO.
func toDetailResponse(d MatchDetail) MatchDetailResponse {
	teams := make([]TeamResponse, 0, len(d.Teams))
	for _, t := range d.Teams {
		players := make([]PlayerStatResponse, 0, len(t.Players))
		for _, p := range t.Players {
			players = append(players, PlayerStatResponse{
				PlayerID:      p.PlayerID,
				SteamID:       p.SteamID,
				SteamUsername: p.SteamUsername,
				Kills:         p.Kills,
				Deaths:        p.Deaths,
				Assists:       p.Assists,
				KDRatio:       p.KDRatio,
				MVPs:          p.MVPs,
			})
		}
		teams = append(teams, TeamResponse{
			ID:           t.ID,
			TeamSlot:     t.TeamSlot,
			StartingSide: t.StartingSide,
			RoundsWon:    t.RoundsWon,
			Result:       t.Result,
			Players:      players,
		})
	}
	return MatchDetailResponse{
		MatchResponse: toResponse(d.Match),
		Teams:         teams,
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
	mux.HandleFunc("POST /api/matches", h.CreateMatch)
	mux.HandleFunc("GET /api/matches/{matchId}", h.GetMatch)
}

// CreateMatch handles POST /api/matches. Eventually this will accept a demo
// upload reference and kick off parsing; for now it fabricates a random match
// from the existing players and returns it with 201 Created.
func (h *MatchHandler) CreateMatch(w http.ResponseWriter, r *http.Request) {
	match, err := h.svc.CreateMatch(r.Context())
	if err != nil {
		log.Printf("create match: %v", err)
		http.Error(w, "failed to create match", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(toResponse(*match)); err != nil {
		log.Printf("encode match: %v", err)
	}
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
	if err != nil {
		log.Printf("get match %d: %v", id, err)
		http.Error(w, "failed to get match", http.StatusInternalServerError)
		return
	}
	if match == nil {
		http.Error(w, "match not found", http.StatusNotFound)
		return
	}

	writeJSON(w, toDetailResponse(*match))
}

// writeJSON encodes v as a JSON response body.
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("encode response: %v", err)
	}
}
