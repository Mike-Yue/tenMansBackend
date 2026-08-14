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
	// CountStats returns how many stats rows reference the given user (internal id).
	// A non-zero count means the user is tied to match data and can't be deleted.
	CountStats(ctx context.Context, userID int64) (int, error)
	// Delete removes a user by internal id. The bool reports whether a row existed.
	Delete(ctx context.Context, userID int64) (bool, error)
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

func (r *sqlUserRepository) CountStats(ctx context.Context, userID int64) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM stats WHERE player_id = ?`, userID).Scan(&n)
	if err != nil {
		return 0, err
	}
	return n, nil
}

func (r *sqlUserRepository) Delete(ctx context.Context, userID int64) (bool, error) {
	res, err := r.db.ExecContext(ctx, `DELETE FROM Users WHERE id = ?`, userID)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}
