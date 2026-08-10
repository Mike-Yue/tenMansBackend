package stats

import "context"

// StatsService holds the business logic for player stats. Handlers depend on
// this interface rather than the concrete implementation.
type StatsService interface {
	// GetPlayerStats returns a user's all-time aggregated stats by Steam ID, or
	// (nil, nil) when no such user exists.
	GetPlayerStats(ctx context.Context, steamID int64) (*PlayerStats, error)
}

type statsService struct {
	repo StatsRepository
}

// NewStatsService returns a StatsService backed by the given repository.
func NewStatsService(repo StatsRepository) StatsService {
	return &statsService{repo: repo}
}

func (s *statsService) GetPlayerStats(ctx context.Context, steamID int64) (*PlayerStats, error) {
	// No business rules yet; this is the seam where per-season filtering or
	// derived metrics would live as the domain grows.
	return s.repo.AggregateBySteamID(ctx, steamID)
}
