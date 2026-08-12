package matches

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
)

// MatchResponse is the API DTO for a match. It omits internal fields
// (upload_hash, storage_key) and exposes the lifecycle via `status`. The
// parse-derived fields are nullable and serialize as null until known.
type MatchResponse struct {
	ID          int64   `json:"id"`
	Map         *string `json:"map"`
	PlayedAt    *string `json:"playedAt"`
	UploadedAt  *string `json:"uploadedAt"`
	Status      string  `json:"status"`
	SeasonID    int64   `json:"seasonId"`
	TotalRounds *int64  `json:"totalRounds"`
	CreatedAt   *string `json:"createdAt"`
}

// toResponse maps the DB model onto the API DTO.
func toResponse(m Match) MatchResponse {
	return MatchResponse{
		ID:          m.ID,
		Map:         m.Map,
		PlayedAt:    m.PlayedAt,
		UploadedAt:  m.UploadedAt,
		Status:      m.Status,
		SeasonID:    m.SeasonID,
		TotalRounds: m.TotalRounds,
		CreatedAt:   m.CreatedAt,
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
	mux.HandleFunc("POST /api/matches", h.InitiateUpload)
	mux.HandleFunc("POST /api/matches/random", h.CreateRandomMatch)
	mux.HandleFunc("POST /api/matches/{id}/uploaded", h.MarkUploaded)
	mux.HandleFunc("POST /api/matches/{id}/results", h.CompleteMatch)
	mux.HandleFunc("POST /api/matches/upload", h.UploadMatch)
	mux.HandleFunc("GET /api/matches/{matchId}", h.GetMatch)
}

// --- Upload lifecycle ---

type initiateRequest struct {
	ContentHash string `json:"contentHash"`
	SizeBytes   int64  `json:"sizeBytes"`
	SeasonID    *int64 `json:"seasonId"`
}

type initiateResponse struct {
	MatchID    int64  `json:"matchId"`
	StorageKey string `json:"storageKey"`
	UploadURL  string `json:"uploadUrl"`
}

// InitiateUpload handles POST /api/matches — reserve a match and get an upload
// URL. Body: { contentHash, sizeBytes?, seasonId? }.
func (h *MatchHandler) InitiateUpload(w http.ResponseWriter, r *http.Request) {
	var req initiateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.ContentHash == "" {
		http.Error(w, "contentHash is required", http.StatusBadRequest)
		return
	}

	match, uploadURL, err := h.svc.InitiateUpload(r.Context(), req.ContentHash, req.SeasonID)
	if errors.Is(err, ErrDuplicate) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error":   "a match with this content hash already exists",
			"matchId": match.ID,
		})
		return
	}
	if err != nil {
		log.Printf("initiate upload: %v", err)
		http.Error(w, "failed to initiate upload", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(initiateResponse{
		MatchID:    match.ID,
		StorageKey: match.StorageKey,
		UploadURL:  uploadURL,
	}); err != nil {
		log.Printf("encode initiate response: %v", err)
	}
}

// MarkUploaded handles POST /api/matches/{id}/uploaded — the client signals the
// demo finished uploading to storage.
func (h *MatchHandler) MarkUploaded(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid match id", http.StatusBadRequest)
		return
	}

	found, err := h.svc.MarkUploaded(r.Context(), id)
	if err != nil {
		log.Printf("mark uploaded %d: %v", id, err)
		http.Error(w, "failed to mark uploaded", http.StatusInternalServerError)
		return
	}
	if !found {
		http.Error(w, "match not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Parser callback ---

type parseResultRequest struct {
	Map         string             `json:"map"`
	PlayedAt    string             `json:"playedAt"`
	TotalRounds int64              `json:"totalRounds"`
	Teams       []parseTeamRequest `json:"teams"`
}

type parseTeamRequest struct {
	Slot      string               `json:"slot"`
	Side      string               `json:"side"`
	RoundsWon int64                `json:"roundsWon"`
	Result    string               `json:"result"`
	Players   []parsePlayerRequest `json:"players"`
}

type parsePlayerRequest struct {
	PlayerID int64   `json:"playerId"`
	Kills    int64   `json:"kills"`
	Deaths   int64   `json:"deaths"`
	Assists  int64   `json:"assists"`
	KDRatio  float64 `json:"kdRatio"`
	MVPs     int64   `json:"mvps"`
}

func (req parseResultRequest) toParseResult() ParseResult {
	teams := make([]NewTeam, 0, len(req.Teams))
	for _, t := range req.Teams {
		players := make([]NewPlayerStat, 0, len(t.Players))
		for _, p := range t.Players {
			players = append(players, NewPlayerStat{
				PlayerID: p.PlayerID,
				Kills:    p.Kills,
				Deaths:   p.Deaths,
				Assists:  p.Assists,
				KDRatio:  p.KDRatio,
				MVPs:     p.MVPs,
			})
		}
		teams = append(teams, NewTeam{
			TeamSlot:     t.Slot,
			StartingSide: t.Side,
			RoundsWon:    t.RoundsWon,
			Result:       t.Result,
			Players:      players,
		})
	}
	return ParseResult{
		Map:         req.Map,
		PlayedAt:    req.PlayedAt,
		TotalRounds: req.TotalRounds,
		Teams:       teams,
	}
}

// CompleteMatch handles POST /api/matches/{id}/results — the parser service
// reports the parsed match. Guarded by a shared secret header when
// PARSER_CALLBACK_SECRET is set.
func (h *MatchHandler) CompleteMatch(w http.ResponseWriter, r *http.Request) {
	if secret := os.Getenv("PARSER_CALLBACK_SECRET"); secret != "" {
		if r.Header.Get("X-Parser-Secret") != secret {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid match id", http.StatusBadRequest)
		return
	}

	var req parseResultRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	found, err := h.svc.CompleteMatch(r.Context(), id, req.toParseResult())
	if err != nil {
		log.Printf("complete match %d: %v", id, err)
		http.Error(w, "failed to complete match", http.StatusInternalServerError)
		return
	}
	if !found {
		http.Error(w, "match not found", http.StatusNotFound)
		return
	}

	// Return the freshly completed match with its scoreboard.
	detail, err := h.svc.GetMatch(r.Context(), id)
	if err != nil || detail == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeJSON(w, toDetailResponse(*detail))
}

// UploadMatch handles POST /api/matches/upload — the parser service reports a
// job event (started/succeeded/failed) conforming to the parser-event v1
// contract. The event is correlated to a match via source.key == storage_key.
// A succeeded event persists teams and stats and returns the completed match;
// started/failed update status and return 202.
func (h *MatchHandler) UploadMatch(w http.ResponseWriter, r *http.Request) {
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}

	outcome, found, err := h.svc.ProcessUploadEvent(r.Context(), raw)
	if errors.Is(err, ErrInvalidEvent) {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if errors.Is(err, ErrAlreadyProcessed) {
		http.Error(w, "match already processed", http.StatusConflict)
		return
	}
	if err != nil {
		log.Printf("process upload event: %v", err)
		http.Error(w, "failed to process event", http.StatusInternalServerError)
		return
	}
	if !found {
		http.Error(w, "match not found for source key", http.StatusNotFound)
		return
	}

	if !outcome.Persisted {
		// started/failed: acknowledged, status updated, nothing to return.
		w.WriteHeader(http.StatusAccepted)
		return
	}

	// Succeeded: return the freshly completed match with its scoreboard.
	detail, err := h.svc.GetMatch(r.Context(), outcome.MatchID)
	if err != nil || detail == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeJSON(w, toDetailResponse(*detail))
}

// --- Random generator (dev) ---

// CreateRandomMatch handles POST /api/matches/random — fabricate a fully formed
// match. Stand-in for the real pipeline; keeps the app populated for now.
func (h *MatchHandler) CreateRandomMatch(w http.ResponseWriter, r *http.Request) {
	match, err := h.svc.CreateRandomMatch(r.Context())
	if err != nil {
		log.Printf("create random match: %v", err)
		http.Error(w, "failed to create match", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(toResponse(*match)); err != nil {
		log.Printf("encode match: %v", err)
	}
}

// --- Reads ---

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

// GetMatch handles GET /api/matches/{matchId} — return a single match with teams.
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
