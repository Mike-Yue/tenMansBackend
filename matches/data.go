package matches

import (
	"context"
	"database/sql"
	"errors"
)

// errNotImplemented marks a repository method that is scaffolded but not yet
// wired to real SQL. Handlers translate it into a 501 response.
var errNotImplemented = errors.New("not implemented")

// MatchRepository is the persistence seam for matches. The service layer depends
// on this interface, not on the concrete implementation, so it can be mocked in
// tests.
type MatchRepository interface {
	// List returns matches, optionally filtered by season. A nil seasonID means
	// "all seasons".
	List(ctx context.Context, seasonID *int64) ([]Match, error)
	// GetByID looks a match up by its primary key. Returns (nil, nil) when no
	// such match exists.
	GetByID(ctx context.Context, id int64) (*Match, error)
}

// sqlMatchRepository is a MatchRepository backed by database/sql.
type sqlMatchRepository struct {
	db *sql.DB
}

// NewMatchRepository returns a MatchRepository backed by the given database.
func NewMatchRepository(db *sql.DB) MatchRepository {
	return &sqlMatchRepository{db: db}
}

func (r *sqlMatchRepository) List(ctx context.Context, seasonID *int64) ([]Match, error) {
	// TODO: when seasonID != nil, append "WHERE season_id = ?" and pass the
	//       value as a query arg. For now this returns all matches regardless.
	_ = seasonID

	rows, err := r.db.QueryContext(ctx,
		`SELECT id, map, played_at, uploaded_at, upload_hash, processed, season_id, total_rounds
		 FROM Matches`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	matches := make([]Match, 0)
	for rows.Next() {
		var m Match
		if err := rows.Scan(
			&m.ID,
			&m.Map,
			&m.PlayedAt,
			&m.UploadedAt,
			&m.UploadHash,
			&m.Processed,
			&m.SeasonID,
			&m.TotalRounds,
		); err != nil {
			return nil, err
		}
		matches = append(matches, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return matches, nil
}

func (r *sqlMatchRepository) GetByID(ctx context.Context, id int64) (*Match, error) {
	// TODO: SELECT ... FROM Matches WHERE id = ? — scan a single row, and
	//       return (nil, nil) on sql.ErrNoRows so the handler can respond 404.
	return nil, errNotImplemented
}
