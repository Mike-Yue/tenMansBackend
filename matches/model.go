package matches

// Match is the DB model: it mirrors a row in the "Matches" table and is used
// only by the data and service layers. It intentionally carries no json tags —
// how a match is exposed over the API is the handler's concern (see
// MatchResponse in handler.go).
//
// All columns are NOT NULL, so no pointer fields are needed. The processed
// column is stored as an INTEGER 0/1 and represented here as a bool.
type Match struct {
	ID          int64
	Map         string
	PlayedAt    string
	UploadedAt  string
	UploadHash  string
	Processed   bool
	SeasonID    int64
	TotalRounds int64
}

// NewMatch is the input model for creating a match together with its two teams
// and every player's stat line. The service builds it (currently with random
// data standing in for the future demo-parser output) and MatchRepository.Create
// persists the whole graph in one transaction.
type NewMatch struct {
	Map         string
	PlayedAt    string
	UploadedAt  string
	UploadHash  string
	SeasonID    int64
	TotalRounds int64
	Teams       []NewTeam
}

// NewTeam is one side of a match: its slot ("A"/"B"), starting side ("T"/"CT"),
// final round count, result ("win"/"loss"/"tie"), and its five players' stats.
type NewTeam struct {
	TeamSlot     string
	StartingSide string
	RoundsWon    int64
	Result       string
	Players      []NewPlayerStat
}

// NewPlayerStat is one player's stat line within a team for a single match.
type NewPlayerStat struct {
	PlayerID int64
	Kills    int64
	Deaths   int64
	Assists  int64
	KDRatio  float64
	MVPs     int64
}

// MatchDetail is a match together with its teams and every player's stat line —
// the shape returned by GET /api/matches/{matchId}. It embeds the base Match.
type MatchDetail struct {
	Match
	Teams []TeamDetail
}

// TeamDetail is one side of a match plus its players' scoreboard.
type TeamDetail struct {
	ID           int64
	TeamSlot     string
	StartingSide string
	RoundsWon    int64
	Result       string
	Players      []PlayerStatDetail
}

// PlayerStatDetail is a player's identity joined with their stat line for a match.
type PlayerStatDetail struct {
	PlayerID      int64
	SteamID       int64
	SteamUsername *string
	Kills         int64
	Deaths        int64
	Assists       int64
	KDRatio       float64
	MVPs          int64
}
