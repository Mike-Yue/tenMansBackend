package seasons

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// dateLayout is the format start_at/end_at are stored in and that
// matches.CurrentSeasonID compares against. Inputs are validated to this layout
// so season resolution keeps working.
const dateLayout = "2006-01-02"

// ErrInvalidSeason is returned when create input fails validation. Handlers map
// it to a 400 rather than a 500.
var ErrInvalidSeason = errors.New("invalid season")

// ErrSeasonReferenced is returned by DeleteSeason when matches still reference the
// season. Handlers map it to 409.
var ErrSeasonReferenced = errors.New("season has matches")

// SeasonService holds the business logic for seasons. Handlers depend on this
// interface rather than the concrete implementation.
type SeasonService interface {
	ListSeasons(ctx context.Context) ([]Season, error)
	// CreateSeason validates the input and persists a new season. Returns
	// ErrInvalidSeason (wrapped) when name is empty or the dates are malformed
	// or out of order.
	CreateSeason(ctx context.Context, name, startAt, endAt string) (*Season, error)
	// DeleteSeason removes a season with no associated matches. The bool reports
	// whether the season existed. Returns ErrSeasonReferenced when matches still
	// reference it.
	DeleteSeason(ctx context.Context, id int64) (bool, error)
}

type seasonService struct {
	repo SeasonRepository
}

// NewSeasonService returns a SeasonService backed by the given repository.
func NewSeasonService(repo SeasonRepository) SeasonService {
	return &seasonService{repo: repo}
}

func (s *seasonService) ListSeasons(ctx context.Context) ([]Season, error) {
	return s.repo.List(ctx)
}

func (s *seasonService) CreateSeason(ctx context.Context, name, startAt, endAt string) (*Season, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errInvalid("name is required")
	}

	start, err := time.Parse(dateLayout, startAt)
	if err != nil {
		return nil, errInvalid("startAt must be a YYYY-MM-DD date")
	}
	end, err := time.Parse(dateLayout, endAt)
	if err != nil {
		return nil, errInvalid("endAt must be a YYYY-MM-DD date")
	}
	if end.Before(start) {
		return nil, errInvalid("endAt must not be before startAt")
	}

	return s.repo.Create(ctx, name, startAt, endAt)
}

func (s *seasonService) DeleteSeason(ctx context.Context, id int64) (bool, error) {
	n, err := s.repo.CountMatches(ctx, id)
	if err != nil {
		return false, err
	}
	if n > 0 {
		return false, ErrSeasonReferenced
	}

	return s.repo.Delete(ctx, id)
}

// errInvalid wraps ErrInvalidSeason with a caller-facing reason, kept on one line
// so it reads cleanly as an HTTP 400 body (e.g. "invalid season: name is required").
func errInvalid(reason string) error {
	return fmt.Errorf("%w: %s", ErrInvalidSeason, reason)
}
