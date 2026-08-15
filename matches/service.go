package matches

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
)

// Match lifecycle status values stored in the matches.status column. These must
// stay within the column's CHECK constraint
// (pending, uploaded, processed, failed).
const (
	statusProcessed = "processed"
	statusUploaded  = "uploaded"
	statusFailed    = "failed"
)

// ErrDuplicate is returned by InitiateUpload when a match with the same content
// hash already exists. The handler translates it into 409 Conflict.
var ErrDuplicate = errors.New("a match with this upload hash already exists")

// ErrInvalidEvent is returned by ProcessUploadEvent when the payload fails
// parser-event schema validation. The handler translates it into 400.
var ErrInvalidEvent = errors.New("invalid parser event")

// ErrAlreadyProcessed is returned by ProcessUploadEvent when the correlated
// match has already been processed. The handler translates it into 409, which
// makes ingestion idempotent against re-delivered events.
var ErrAlreadyProcessed = errors.New("match already processed")

// UploadEventOutcome reports how ProcessUploadEvent handled an event so the
// handler can pick the right status code. MatchID is set when a match was
// correlated; Persisted is true only for a succeeded event that wrote results.
type UploadEventOutcome struct {
	MatchID   int64
	Persisted bool
}

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
	// ProcessUploadEvent validates a parser-event payload, correlates it to a
	// match by source.key, and applies it: started/failed update status, while
	// succeeded persists teams and stats. The bool reports whether a match was
	// found. Returns ErrInvalidEvent on a bad payload and ErrAlreadyProcessed
	// when the match has already been processed.
	ProcessUploadEvent(ctx context.Context, raw []byte) (UploadEventOutcome, bool, error)
	// CreateRandomMatch fabricates a fully formed match (dev/testing stand-in for
	// the real upload/parse pipeline).
	CreateRandomMatch(ctx context.Context) (*Match, error)
	// DeleteMatch removes a match and its teams/stats. The bool reports existence.
	DeleteMatch(ctx context.Context, id int64) (bool, error)
}

// SeasonRecomputer rebuilds a season's derived data (player ratings) after its
// processed-match set changes. Implemented by the ratings service and injected in
// main.go. A nil recomputer disables the hook (ratings can always be rebuilt via
// POST /api/ratings/recompute).
type SeasonRecomputer interface {
	RecomputeSeason(ctx context.Context, seasonID int64) error
}

type matchService struct {
	repo       MatchRepository
	presigner  Presigner
	recomputer SeasonRecomputer
}

// NewMatchService returns a MatchService backed by the given repository and
// presigner. recomputer may be nil to disable rating recomputation.
func NewMatchService(repo MatchRepository, presigner Presigner, recomputer SeasonRecomputer) MatchService {
	return &matchService{repo: repo, presigner: presigner, recomputer: recomputer}
}

// recompute rebuilds ratings for a season after a match change. Failures are
// logged, not returned: ratings are derived data and the underlying match
// mutation has already succeeded.
func (s *matchService) recompute(ctx context.Context, seasonID int64) {
	if s.recomputer == nil {
		return
	}
	if err := s.recomputer.RecomputeSeason(ctx, seasonID); err != nil {
		log.Printf("recompute ratings for season %d: %v", seasonID, err)
	}
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
	found, err := s.repo.CompleteFromParse(ctx, id, pr)
	if err != nil || !found {
		return found, err
	}
	if seasonID, ok, err := s.repo.SeasonIDForMatch(ctx, id); err == nil && ok {
		s.recompute(ctx, seasonID)
	}
	return true, nil
}

func (s *matchService) DeleteMatch(ctx context.Context, id int64) (bool, error) {
	// Capture the season before deleting so we can rebuild its ratings afterward.
	seasonID, hadSeason, _ := s.repo.SeasonIDForMatch(ctx, id)

	found, err := s.repo.Delete(ctx, id)
	if err != nil || !found {
		return found, err
	}
	if hadSeason {
		s.recompute(ctx, seasonID)
	}
	return true, nil
}

func (s *matchService) ProcessUploadEvent(ctx context.Context, raw []byte) (UploadEventOutcome, bool, error) {
	if err := validateUploadEvent(raw); err != nil {
		return UploadEventOutcome{}, false, fmt.Errorf("%w: %v", ErrInvalidEvent, err)
	}

	var event parserEvent
	if err := json.Unmarshal(raw, &event); err != nil {
		return UploadEventOutcome{}, false, fmt.Errorf("%w: %v", ErrInvalidEvent, err)
	}

	match, err := s.repo.FindByStorageKey(ctx, event.Source.Key)
	if err != nil {
		return UploadEventOutcome{}, false, err
	}
	if match == nil {
		return UploadEventOutcome{}, false, nil
	}

	// Reject re-delivered events for an already-processed match before any write,
	// so ingestion can't duplicate teams/stats.
	if match.Status == statusProcessed {
		return UploadEventOutcome{MatchID: match.ID}, true, ErrAlreadyProcessed
	}

	switch event.EventType {
	case eventTypeStarted:
		if _, err := s.repo.UpdateStatus(ctx, match.ID, statusUploaded); err != nil {
			return UploadEventOutcome{}, false, err
		}
	case eventTypeFailed:
		if _, err := s.repo.UpdateStatus(ctx, match.ID, statusFailed); err != nil {
			return UploadEventOutcome{}, false, err
		}
	case eventTypeSucceeded:
		pr, err := event.toParseResult(func(steamID int64, displayName string) (int64, error) {
			return s.repo.EnsureUserBySteamID(ctx, steamID, displayName)
		})
		if err != nil {
			return UploadEventOutcome{}, false, err
		}
		if _, err := s.repo.CompleteFromParse(ctx, match.ID, pr); err != nil {
			return UploadEventOutcome{}, false, err
		}
		s.recompute(ctx, match.SeasonID)
		return UploadEventOutcome{MatchID: match.ID, Persisted: true}, true, nil
	}

	return UploadEventOutcome{MatchID: match.ID}, true, nil
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

	match, err := s.repo.Create(ctx, generateRandomMatch(playerIDs))
	if err != nil {
		return nil, err
	}
	s.recompute(ctx, match.SeasonID)
	return match, nil
}
