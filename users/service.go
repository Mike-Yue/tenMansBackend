package users

import (
	"context"
	"errors"
)

// ErrUserReferenced is returned by DeleteUserBySteamID when the user is tied to
// existing stats/match data and so must not be deleted. Handlers map it to 409.
var ErrUserReferenced = errors.New("user has associated stats")

// UserService holds the business logic for users. Handlers depend on this
// interface rather than the concrete implementation.
type UserService interface {
	ListUsers(ctx context.Context) ([]User, error)
	GetUserBySteamID(ctx context.Context, steamID int64) (*User, error)
	// DeleteUserBySteamID deletes a user (identified by Steam ID) that has no
	// associated stats. The bool reports whether the user existed. Returns
	// ErrUserReferenced when the user is referenced by stats rows.
	DeleteUserBySteamID(ctx context.Context, steamID int64) (bool, error)
}

type userService struct {
	repo UserRepository
}

// NewUserService returns a UserService backed by the given repository.
func NewUserService(repo UserRepository) UserService {
	return &userService{repo: repo}
}

func (s *userService) ListUsers(ctx context.Context) ([]User, error) {
	// No business rules yet; this is the seam where validation, filtering, or
	// enrichment will live as the domain grows.
	return s.repo.List(ctx)
}

func (s *userService) GetUserBySteamID(ctx context.Context, steamID int64) (*User, error) {
	return s.repo.GetBySteamID(ctx, steamID)
}

func (s *userService) DeleteUserBySteamID(ctx context.Context, steamID int64) (bool, error) {
	user, err := s.repo.GetBySteamID(ctx, steamID)
	if err != nil {
		return false, err
	}
	if user == nil {
		return false, nil // no such user → 404
	}

	n, err := s.repo.CountStats(ctx, user.ID)
	if err != nil {
		return false, err
	}
	if n > 0 {
		return false, ErrUserReferenced
	}

	return s.repo.Delete(ctx, user.ID)
}
