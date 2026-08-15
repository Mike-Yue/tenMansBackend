package ratings

import (
	"context"
	"math"
)

// Display scaling for the engine ordinal (OpenSkill ordinals sit roughly 0–50).
// ratingBase keeps a fresh player near a familiar ladder number and stops early
// losers from showing a negative rating. Both are tunable in one place; adjust if
// you swap in an engine whose ordinal lives in a very different range.
const (
	ratingScale = 100.0
	ratingBase  = 1000.0
)

// RatingService holds the rating business logic.
type RatingService interface {
	// RecomputeSeason rebuilds every player's rating for one season from that
	// season's full processed-match history, then persists it.
	RecomputeSeason(ctx context.Context, seasonID int64) error
	// RecomputeAll rebuilds ratings for every season (backfill / repair).
	RecomputeAll(ctx context.Context) error
	// GetPlayerRatings returns a player's per-season ratings. The bool reports
	// whether the user exists (false → 404).
	GetPlayerRatings(ctx context.Context, steamID int64) ([]PlayerSeasonRating, bool, error)
}

type ratingService struct {
	repo   RatingRepository
	engine RatingEngine
}

// NewRatingService returns a RatingService backed by the given repository and
// rating engine. Swap the engine to change the algorithm; nothing else changes.
func NewRatingService(repo RatingRepository, engine RatingEngine) RatingService {
	return &ratingService{repo: repo, engine: engine}
}

func (s *ratingService) RecomputeSeason(ctx context.Context, seasonID int64) error {
	rosters, err := s.repo.SeasonMatchRosters(ctx, seasonID)
	if err != nil {
		return err
	}

	computed := s.engine.Compute(rosters)

	rows := make([]ratingRow, 0, len(computed))
	for _, pr := range computed {
		rows = append(rows, ratingRow{
			PlayerID:    pr.PlayerID,
			Mu:          pr.Mu,
			Sigma:       pr.Sigma,
			Ordinal:     pr.Ordinal,
			GamesPlayed: pr.GamesPlayed,
		})
	}

	return s.repo.ReplaceSeasonRatings(ctx, seasonID, rows)
}

func (s *ratingService) RecomputeAll(ctx context.Context) error {
	ids, err := s.repo.ListSeasonIDs(ctx)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := s.RecomputeSeason(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

func (s *ratingService) GetPlayerRatings(ctx context.Context, steamID int64) ([]PlayerSeasonRating, bool, error) {
	raw, exists, err := s.repo.GetByPlayerSteamID(ctx, steamID)
	if err != nil || !exists {
		return nil, exists, err
	}

	out := make([]PlayerSeasonRating, 0, len(raw))
	for _, sr := range raw {
		out = append(out, PlayerSeasonRating{
			SeasonID:    sr.SeasonID,
			SeasonName:  sr.SeasonName,
			Rating:      int(math.Round(ratingBase + sr.Ordinal*ratingScale)),
			GamesPlayed: sr.GamesPlayed,
		})
	}
	return out, true, nil
}
