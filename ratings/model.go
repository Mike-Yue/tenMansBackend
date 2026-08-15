package ratings

// PlayerSeasonRating is the API DTO for a player's rating within one season. The
// underlying mu/sigma are intentionally omitted — only the display Rating and the
// game count are exposed.
type PlayerSeasonRating struct {
	SeasonID    int64  `json:"seasonId"`
	SeasonName  string `json:"seasonName"`
	Rating      int    `json:"rating"`
	GamesPlayed int    `json:"gamesPlayed"`
}

// MatchRoster is one processed match reduced to what rating needs: the two teams,
// each with the players on it and the rounds they won (the OpenSkill "score").
type MatchRoster struct {
	MatchID int64
	Teams   [2]TeamRoster
}

// TeamRoster is one side of a match: its player ids and rounds won.
type TeamRoster struct {
	PlayerIDs []int64
	RoundsWon int
}

// ratingRow is the persisted rating for one player in one season (internal).
type ratingRow struct {
	PlayerID    int64
	Mu          float64
	Sigma       float64
	Ordinal     float64
	GamesPlayed int
}
