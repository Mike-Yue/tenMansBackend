package users

import (
	"context"
	"database/sql"
	"errors"
)

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
	var u User
	err := r.db.QueryRowContext(ctx,
		`SELECT id, steam_id, steam_username, created_at FROM Users WHERE steam_id = ?`,
		steamID,
	).Scan(&u.ID, &u.SteamID, &u.SteamUsername, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &u, nil
}
