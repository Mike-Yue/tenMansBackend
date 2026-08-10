package users

import (
	"context"
	"database/sql"
	"errors"
)

// errNotImplemented marks a repository method that is scaffolded but not yet
// wired to real SQL. Handlers translate it into a 501 response.
var errNotImplemented = errors.New("not implemented")

// UserRepository is the persistence seam for users. The service layer depends
// on this interface, not on the concrete implementation, so it can be mocked
// in tests.
type UserRepository interface {
	List(ctx context.Context) ([]User, error)
	// GetBySteamID looks a user up by their Steam ID (the steam_id column),
	// not the internal primary key. Returns (nil, nil) when no such user exists.
	GetBySteamID(ctx context.Context, steamID int64) (*User, error)
}

// sqlUserRepository is a UserRepository backed by database/sql.
type sqlUserRepository struct {
	db *sql.DB
}

// NewUserRepository returns a UserRepository backed by the given database.
func NewUserRepository(db *sql.DB) UserRepository {
	return &sqlUserRepository{db: db}
}

func (r *sqlUserRepository) List(ctx context.Context) ([]User, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, steam_id, steam_username, created_at FROM Users`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := make([]User, 0)
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.SteamID, &u.SteamUsername, &u.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return users, nil
}

func (r *sqlUserRepository) GetBySteamID(ctx context.Context, steamID int64) (*User, error) {
	// TODO: SELECT id, steam_id, steam_username, created_at FROM Users
	//       WHERE steam_id = ?  — scan a single row, and return (nil, nil) on
	//       sql.ErrNoRows so the handler can respond 404.
	return nil, errNotImplemented
}
