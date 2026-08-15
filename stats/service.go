package stats

import "context"

// StatsService holds the business logic for player stats. Handlers depend on
// this interface rather than the concrete implementation.
type StatsService interface {
	// GetPlayerStats returns a user's aggregated stats by Steam ID, optionally
	// scoped to one season (nil = all-time). (nil, nil) when no such user exists.
	GetPlayerStats(ctx context.Context, steamID int64, seasonID *int64) (*PlayerStats, error)
}

type statsService struct {
	repo StatsRepository
}

// NewStatsService returns a StatsService backed by the given repository.
func NewStatsService(repo StatsRepository) StatsService {
	return &statsService{repo: repo}
}

func (s *statsService) GetPlayerStats(ctx context.Context, steamID int64, seasonID *int64) (*PlayerStats, error) {
	return s.repo.AggregateBySteamID(ctx, steamID, seasonID)
}
