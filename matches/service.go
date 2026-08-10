package matches

import (
	"context"
	"fmt"
)

// MatchService holds the business logic for matches. Handlers depend on this
// interface rather than the concrete implementation.
type MatchService interface {
	// ListMatches returns matches, optionally filtered by season (nil = all).
	ListMatches(ctx context.Context, seasonID *int64) ([]Match, error)
	// GetMatch returns a match with its teams and per-player stats, or (nil, nil)
	// when no such match exists.
	GetMatch(ctx context.Context, id int64) (*MatchDetail, error)
	// CreateMatch creates a new match. Until the demo-upload/parser pipeline
	// exists, it fabricates a random match from the existing players.
	CreateMatch(ctx context.Context) (*Match, error)
}

type matchService struct {
	repo MatchRepository
}

// NewMatchService returns a MatchService backed by the given repository.
func NewMatchService(repo MatchRepository) MatchService {
	return &matchService{repo: repo}
}

func (s *matchService) ListMatches(ctx context.Context, seasonID *int64) ([]Match, error) {
	// No business rules yet; this is the seam where filtering or enrichment
	// will live as the domain grows.
	return s.repo.List(ctx, seasonID)
}

func (s *matchService) GetMatch(ctx context.Context, id int64) (*MatchDetail, error) {
	return s.repo.GetDetailByID(ctx, id)
}

func (s *matchService) CreateMatch(ctx context.Context) (*Match, error) {
	playerIDs, err := s.repo.ListPlayerIDs(ctx)
	if err != nil {
		return nil, err
	}
	if len(playerIDs) < playersPerTeam*2 {
		return nil, fmt.Errorf(
			"need at least %d players to create a match, have %d",
			playersPerTeam*2, len(playerIDs))
	}

	return s.repo.Create(ctx, generateRandomMatch(playerIDs))
}
