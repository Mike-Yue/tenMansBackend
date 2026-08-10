package users

import "context"

// UserService holds the business logic for users. Handlers depend on this
// interface rather than the concrete implementation.
type UserService interface {
	ListUsers(ctx context.Context) ([]User, error)
	GetUserBySteamID(ctx context.Context, steamID int64) (*User, error)
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
