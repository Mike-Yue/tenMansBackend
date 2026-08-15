package ratings

// PlayerRating is one player's computed rating for a season, produced by a
// RatingEngine. Mu/Sigma are Gaussian engine state (left 0 by engines that don't
// use them, e.g. Elo/Glicko-style single numbers); Ordinal is the engine's single
// skill estimate in its own units, which the service scales for display.
type PlayerRating struct {
	PlayerID    int64
	Mu          float64
	Sigma       float64
	Ordinal     float64
	GamesPlayed int
}

// RatingEngine turns a season's ordered match rosters into final per-player
// ratings. It is the single swappable seam of the rating system: to change the
// algorithm (Glicko-2, per-round, performance-weighted, TrueSkill, ...), implement
// this interface in a new file and wire it in main.go via NewRatingService.
// Persistence, HTTP endpoints, recompute triggers, and the frontend are all
// engine-agnostic and need no changes.
//
// Compute must be pure and deterministic: the same rosters in produce the same
// ratings out (recompute-from-history relies on this).
type RatingEngine interface {
	// Name identifies the engine, for logging/debugging.
	Name() string
	// Compute returns a rating for every player appearing in the rosters, which
	// are ordered oldest match first.
	Compute(rosters []MatchRoster) []PlayerRating
}
