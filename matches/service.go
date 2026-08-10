package matches

import (
	"context"
	"errors"
	"fmt"
)

// ErrDuplicate is returned by InitiateUpload when a match with the same content
// hash already exists. The handler translates it into 409 Conflict.
var ErrDuplicate = errors.New("a match with this upload hash already exists")

// MatchService holds the business logic for matches. Handlers depend on this
// interface rather than the concrete implementation.
type MatchService interface {
	// ListMatches returns matches, optionally filtered by season (nil = all).
	ListMatches(ctx context.Context, seasonID *int64) ([]Match, error)
	// GetMatch returns a match with its teams and per-player stats, or (nil, nil)
	// when no such match exists.
	GetMatch(ctx context.Context, id int64) (*MatchDetail, error)
	// InitiateUpload dedups on the content hash, creates a pending match, and
	// returns it plus an upload URL. Returns ErrDuplicate (with the existing
	// match) when the hash is already known.
	InitiateUpload(ctx context.Context, contentHash string, seasonID *int64) (*Match, string, error)
	// MarkUploaded transitions a match to 'uploaded'. The bool reports existence.
	MarkUploaded(ctx context.Context, id int64) (bool, error)
	// CompleteMatch applies parser output to a match. The bool reports existence.
	CompleteMatch(ctx context.Context, id int64, pr ParseResult) (bool, error)
	// CreateRandomMatch fabricates a fully formed match (dev/testing stand-in for
	// the real upload/parse pipeline).
	CreateRandomMatch(ctx context.Context) (*Match, error)
}

type matchService struct {
	repo      MatchRepository
	presigner Presigner
}

// NewMatchService returns a MatchService backed by the given repository and
// presigner.
func NewMatchService(repo MatchRepository, presigner Presigner) MatchService {
	return &matchService{repo: repo, presigner: presigner}
}

func (s *matchService) ListMatches(ctx context.Context, seasonID *int64) ([]Match, error) {
	return s.repo.List(ctx, seasonID)
}

func (s *matchService) GetMatch(ctx context.Context, id int64) (*MatchDetail, error) {
	return s.repo.GetDetailByID(ctx, id)
}

func (s *matchService) InitiateUpload(ctx context.Context, contentHash string, seasonID *int64) (*Match, string, error) {
	if existing, err := s.repo.FindByHash(ctx, contentHash); err != nil {
		return nil, "", err
	} else if existing != nil {
		return existing, "", ErrDuplicate
	}

	season, err := s.resolveSeason(ctx, seasonID)
	if err != nil {
		return nil, "", err
	}

	storageKey := "demos/" + randomHash() + ".dem"
	match, err := s.repo.CreatePending(ctx, contentHash, storageKey, season)
	if err != nil {
		return nil, "", err
	}

	uploadURL, err := s.presigner.PresignUpload(ctx, storageKey)
	if err != nil {
		return nil, "", err
	}
	return match, uploadURL, nil
}

func (s *matchService) resolveSeason(ctx context.Context, seasonID *int64) (int64, error) {
	if seasonID != nil {
		return *seasonID, nil
	}
	return s.repo.CurrentSeasonID(ctx)
}

func (s *matchService) MarkUploaded(ctx context.Context, id int64) (bool, error) {
	return s.repo.MarkUploaded(ctx, id)
}

func (s *matchService) CompleteMatch(ctx context.Context, id int64, pr ParseResult) (bool, error) {
	return s.repo.CompleteFromParse(ctx, id, pr)
}

func (s *matchService) CreateRandomMatch(ctx context.Context) (*Match, error) {
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
