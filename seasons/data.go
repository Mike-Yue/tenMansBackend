package seasons

import (
	"context"
	"database/sql"
)

// SeasonRepository is the persistence seam for seasons. The service layer depends
// on this interface, not on the concrete implementation, so it can be mocked in
// tests.
type SeasonRepository interface {
	List(ctx context.Context) ([]Season, error)
	// Create inserts a season and returns it with the assigned id populated.
	Create(ctx context.Context, name, startAt, endAt string) (*Season, error)
}

// sqlSeasonRepository is a SeasonRepository backed by database/sql.
type sqlSeasonRepository struct {
	db *sql.DB
}

// NewSeasonRepository returns a SeasonRepository backed by the given database.
func NewSeasonRepository(db *sql.DB) SeasonRepository {
	return &sqlSeasonRepository{db: db}
}

func (r *sqlSeasonRepository) List(ctx context.Context) ([]Season, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, start_at, end_at FROM seasons ORDER BY start_at DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	seasons := make([]Season, 0)
	for rows.Next() {
		var s Season
		if err := rows.Scan(&s.ID, &s.Name, &s.StartAt, &s.EndAt); err != nil {
			return nil, err
		}
		seasons = append(seasons, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return seasons, nil
}

func (r *sqlSeasonRepository) Create(ctx context.Context, name, startAt, endAt string) (*Season, error) {
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO seasons (name, start_at, end_at) VALUES (?, ?, ?)`,
		name, startAt, endAt)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}

	return &Season{ID: id, Name: name, StartAt: startAt, EndAt: endAt}, nil
}
