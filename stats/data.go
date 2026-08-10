package stats

import (
	"context"
	"database/sql"
	"errors"
)

// StatsRepository is the persistence seam for aggregated stats. The service
// layer depends on this interface, not on the concrete implementation, so it can
// be mocked in tests.
type StatsRepository interface {
	// AggregateBySteamID returns a user's all-time aggregated stats, keyed by
	// Steam ID. Returns (nil, nil) when no user has that Steam ID, so the handler
	// can respond 404.
	AggregateBySteamID(ctx context.Context, steamID int64) (*PlayerStats, error)
}

// sqlStatsRepository is a StatsRepository backed by database/sql.
type sqlStatsRepository struct {
	db *sql.DB
}

// NewStatsRepository returns a StatsRepository backed by the given database.
func NewStatsRepository(db *sql.DB) StatsRepository {
	return &sqlStatsRepository{db: db}
}

func (r *sqlStatsRepository) AggregateBySteamID(ctx context.Context, steamID int64) (*PlayerStats, error) {
	// GROUP BY u.id makes an unknown Steam ID return zero rows (→ ErrNoRows →
	// 404), while a user who exists but has no games returns one row of zeros
	// (via the LEFT JOINs + COALESCE). Winrate = games won / games played, with
	// ties counting as non-wins.
	const query = `
		SELECT
			COALESCE(SUM(s.kills),   0),
			COALESCE(SUM(s.deaths),  0),
			COALESCE(SUM(s.assists), 0),
			COALESCE(SUM(s.mvps),    0),
			COALESCE(AVG(CASE WHEN mt.result = 'win' THEN 1.0 ELSE 0.0 END), 0.0)
		FROM Users u
		LEFT JOIN Stats s        ON s.player_id = u.id
		LEFT JOIN match_teams mt ON mt.id = s.team_id
		WHERE u.steam_id = ?
		GROUP BY u.id`

	var ps PlayerStats
	err := r.db.QueryRowContext(ctx, query, steamID).Scan(
		&ps.Kills,
		&ps.Deaths,
		&ps.Assists,
		&ps.MVPs,
		&ps.Winrate,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &ps, nil
}
