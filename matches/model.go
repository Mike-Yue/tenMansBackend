package matches

// Match is the DB model: it mirrors a row in the "matches" table and is used
// only by the data and service layers.
//
// A match is created before it's parsed, so most metadata is filled in later:
// Map, PlayedAt, UploadedAt and TotalRounds are nullable (pointers) and remain
// nil until the corresponding lifecycle step. Status tracks that lifecycle.
type Match struct {
	ID          int64
	Map         *string
	PlayedAt    *string
	UploadedAt  *string
	UploadHash  string
	Status      string
	SeasonID    int64
	TotalRounds *int64
	CreatedAt   *string
	StorageKey  string
}

// NewMatch is the input model for the random generator, which produces a fully
// formed (already "processed") match in one transaction via Create.
type NewMatch struct {
	Map         string
	PlayedAt    string
	UploadedAt  string
	UploadHash  string
	StorageKey  string
	SeasonID    int64
	TotalRounds int64
	Teams       []NewTeam
}

// ParseResult is what the parser service reports for a match: the metadata it
// learned plus both teams' scoreboards. Consumed by CompleteFromParse.
type ParseResult struct {
	Map         string
	PlayedAt    string
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
