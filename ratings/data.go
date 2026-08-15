package ratings

import (
	"context"
	"database/sql"
	"time"
)

// seasonRatingRaw is one stored rating joined with its season name (internal).
type seasonRatingRaw struct {
	SeasonID    int64
	SeasonName  string
	Ordinal     float64
	GamesPlayed int
}

// RatingRepository is the persistence seam for player ratings.
type RatingRepository interface {
	// SeasonMatchRosters returns every processed match in the season, in play order,
	// reduced to the two teams' players and rounds won. Matches that don't have
	// exactly two teams are skipped (malformed data).
	SeasonMatchRosters(ctx context.Context, seasonID int64) ([]MatchRoster, error)
	// ReplaceSeasonRatings deletes the season's rating rows and inserts the given
	// ones, in one transaction.
	ReplaceSeasonRatings(ctx context.Context, seasonID int64, rows []ratingRow) error
	// ListSeasonIDs returns every season id, for a full recompute.
	ListSeasonIDs(ctx context.Context) ([]int64, error)
	// GetByPlayerSteamID returns a player's per-season ratings (newest season first).
	// userExists is false when no user has that Steam ID, so the handler can 404.
	GetByPlayerSteamID(ctx context.Context, steamID int64) (rows []seasonRatingRaw, userExists bool, err error)
}

type sqlRatingRepository struct {
	db *sql.DB
}

// NewRatingRepository returns a RatingRepository backed by the given database.
func NewRatingRepository(db *sql.DB) RatingRepository {
	return &sqlRatingRepository{db: db}
}

func (r *sqlRatingRepository) SeasonMatchRosters(ctx context.Context, seasonID int64) ([]MatchRoster, error) {
	// One row per (match, team, player), grouped by match then team via ORDER BY.
	// Same join path as stats.AggregateBySteamID: stats → match_teams → matches.
	const query = `
		SELECT m.id, mt.id, mt.rounds_won, s.player_id
		FROM matches m
		JOIN match_teams mt ON mt.match_id = m.id
		JOIN stats s        ON s.team_id = mt.id
		WHERE m.status = 'processed' AND m.season_id = ?
		ORDER BY m.played_at, m.id, mt.id, s.player_id`

	rows, err := r.db.QueryContext(ctx, query, seasonID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var (
		rosters   []MatchRoster
		curMatch  int64 = -1
		teamOrder []int64            // team ids in appearance order for the current match
		teams     map[int64]*TeamRoster
	)

	// flush finalizes the current match into rosters if it has exactly two teams.
	flush := func() {
		if curMatch == -1 {
			return
		}
		if len(teamOrder) == 2 {
			mr := MatchRoster{MatchID: curMatch}
			for i, tid := range teamOrder {
				mr.Teams[i] = *teams[tid]
			}
			rosters = append(rosters, mr)
		}
	}

	for rows.Next() {
		var matchID, teamID, playerID int64
		var roundsWon int
		if err := rows.Scan(&matchID, &teamID, &roundsWon, &playerID); err != nil {
			return nil, err
		}

		if matchID != curMatch {
			flush()
			curMatch = matchID
			teamOrder = teamOrder[:0]
			teams = make(map[int64]*TeamRoster)
		}
		t, ok := teams[teamID]
		if !ok {
			t = &TeamRoster{RoundsWon: roundsWon}
			teams[teamID] = t
			teamOrder = append(teamOrder, teamID)
		}
		t.PlayerIDs = append(t.PlayerIDs, playerID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	flush()

	return rosters, nil
}

func (r *sqlRatingRepository) ReplaceSeasonRatings(ctx context.Context, seasonID int64, ratingRows []ratingRow) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM player_ratings WHERE season_id = ?`, seasonID); err != nil {
		return err
	}

	now := time.Now().Format(time.RFC3339)
	for _, row := range ratingRows {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO player_ratings (player_id, season_id, mu, sigma, ordinal, games_played, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			row.PlayerID, seasonID, row.Mu, row.Sigma, row.Ordinal, row.GamesPlayed, now); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *sqlRatingRepository) ListSeasonIDs(ctx context.Context) ([]int64, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id FROM seasons`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *sqlRatingRepository) GetByPlayerSteamID(ctx context.Context, steamID int64) ([]seasonRatingRaw, bool, error) {
	var playerID int64
	err := r.db.QueryRowContext(ctx,
		`SELECT id FROM users WHERE steam_id = ?`, steamID).Scan(&playerID)
	if err == sql.ErrNoRows {
		return nil, false, nil // no such user → 404
	}
	if err != nil {
		return nil, false, err
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT pr.season_id, se.name, pr.ordinal, pr.games_played
		FROM player_ratings pr
		JOIN seasons se ON se.id = pr.season_id
		WHERE pr.player_id = ?
		ORDER BY se.start_at DESC, pr.season_id DESC`, playerID)
	if err != nil {
		return nil, true, err
	}
	defer rows.Close()

	out := make([]seasonRatingRaw, 0)
	for rows.Next() {
		var sr seasonRatingRaw
		if err := rows.Scan(&sr.SeasonID, &sr.SeasonName, &sr.Ordinal, &sr.GamesPlayed); err != nil {
			return nil, true, err
		}
		out = append(out, sr)
	}
	return out, true, rows.Err()
}
