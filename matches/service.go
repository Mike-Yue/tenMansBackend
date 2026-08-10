package matches

import "context"

// MatchService holds the business logic for matches. Handlers depend on this
// interface rather than the concrete implementation.
type MatchService interface {
	// ListMatches returns matches, optionally filtered by season (nil = all).
	ListMatches(ctx context.Context, seasonID *int64) ([]Match, error)
	GetMatch(ctx context.Context, id int64) (*Match, error)
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

func (s *matchService) GetMatch(ctx context.Context, id int64) (*Match, error) {
	return s.repo.GetByID(ctx, id)
}
