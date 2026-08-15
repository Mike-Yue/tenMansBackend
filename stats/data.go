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
	// AggregateBySteamID returns a user's aggregated stats, keyed by Steam ID and
	// optionally scoped to one season (nil = all-time). Returns (nil, nil) when no
	// user has that Steam ID, so the handler can respond 404.
	AggregateBySteamID(ctx context.Context, steamID int64, seasonID *int64) (*PlayerStats, error)
}

// sqlStatsRepository is a StatsRepository backed by database/sql.
type sqlStatsRepository struct {
	db *sql.DB
}

// NewStatsRepository returns a StatsRepository backed by the given database.
func NewStatsRepository(db *sql.DB) StatsRepository {
	return &sqlStatsRepository{db: db}
}

func (r *sqlStatsRepository) AggregateBySteamID(ctx context.Context, steamID int64, seasonID *int64) (*PlayerStats, error) {
	// Resolve the user first so an unknown Steam ID → 404, kept distinct from a
	// known user with no games in the selected season → a row of zeros.
	var userID int64
	err := r.db.QueryRowContext(ctx,
		`SELECT id FROM Users WHERE steam_id = ?`, steamID).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// Pure aggregate (no GROUP BY) always returns exactly one row, so a player with
	// no matching games yields COALESCE zeros rather than ErrNoRows. The season
	// filter is a nullable pair: when nil, "(? IS NULL OR ...)" leaves it unscoped.
	// Winrate = games won / games played, ties counting as non-wins.
	const query = `
		SELECT
			COALESCE(SUM(s.kills),          0),
			COALESCE(SUM(s.deaths),         0),
			COALESCE(SUM(s.assists),        0),
			COALESCE(SUM(s.mvps),           0),
			COALESCE(SUM(s.utility_damage), 0),
			COALESCE(SUM(s.flash_assists),  0),
			COALESCE(SUM(s.headshot_kills), 0),
			COALESCE(AVG(CASE WHEN mt.result = 'win' THEN 1.0 ELSE 0.0 END), 0.0)
		FROM stats s
		JOIN matches m           ON m.id = s.match_id
		LEFT JOIN match_teams mt ON mt.id = s.team_id
		WHERE s.player_id = ? AND (? IS NULL OR m.season_id = ?)`

	var (
		ps            PlayerStats
		headshotKills int64
		seasonArg     any // nil or int64, bound to both placeholders
	)
	if seasonID != nil {
		seasonArg = *seasonID
	}
	err = r.db.QueryRowContext(ctx, query, userID, seasonArg, seasonArg).Scan(
		&ps.Kills,
		&ps.Deaths,
		&ps.Assists,
		&ps.MVPs,
		&ps.UtilityDamage,
		&ps.FlashAssists,
		&headshotKills,
		&ps.Winrate,
	)
	if err != nil {
		return nil, err
	}

	if ps.Deaths > 0 {
		ps.KDRatio = float64(ps.Kills) / float64(ps.Deaths)
	} else {
		ps.KDRatio = float64(ps.Kills)
	}
	if ps.Kills > 0 {
		ps.HeadshotPct = float64(headshotKills) / float64(ps.Kills)
	}

	return &ps, nil
}
