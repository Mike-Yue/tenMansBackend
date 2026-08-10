package matches

import (
	"context"
	"database/sql"
	"errors"
)

// MatchRepository is the persistence seam for matches. The service layer depends
// on this interface, not on the concrete implementation, so it can be mocked in
// tests.
type MatchRepository interface {
	// List returns matches, optionally filtered by season. A nil seasonID means
	// "all seasons".
	List(ctx context.Context, seasonID *int64) ([]Match, error)
	// GetDetailByID returns a match with its teams and every player's stats.
	// Returns (nil, nil) when no such match exists.
	GetDetailByID(ctx context.Context, id int64) (*MatchDetail, error)
	// ListPlayerIDs returns the primary keys of every user, used to assign
	// players to a newly created match.
	ListPlayerIDs(ctx context.Context) ([]int64, error)
	// Create persists a match, its teams, and every player stat line in a single
	// transaction, returning the created match with its assigned ID.
	Create(ctx context.Context, nm NewMatch) (*Match, error)
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

func (r *sqlMatchRepository) GetDetailByID(ctx context.Context, id int64) (*MatchDetail, error) {
	var m Match
	err := r.db.QueryRowContext(ctx,
		`SELECT id, map, played_at, uploaded_at, upload_hash, processed, season_id, total_rounds
		 FROM Matches WHERE id = ?`, id).
		Scan(&m.ID, &m.Map, &m.PlayedAt, &m.UploadedAt, &m.UploadHash, &m.Processed, &m.SeasonID, &m.TotalRounds)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	detail := &MatchDetail{Match: m}

	// Teams first, so we can attach players to them by team id.
	teamRows, err := r.db.QueryContext(ctx,
		`SELECT id, team_slot, starting_side, rounds_won, result
		 FROM match_teams WHERE match_id = ? ORDER BY team_slot`, id)
	if err != nil {
		return nil, err
	}
	defer teamRows.Close()
	teamIndex := make(map[int64]int) // team id -> index in detail.Teams
	for teamRows.Next() {
		var t TeamDetail
		if err := teamRows.Scan(&t.ID, &t.TeamSlot, &t.StartingSide, &t.RoundsWon, &t.Result); err != nil {
			return nil, err
		}
		t.Players = []PlayerStatDetail{}
		teamIndex[t.ID] = len(detail.Teams)
		detail.Teams = append(detail.Teams, t)
	}
	if err := teamRows.Err(); err != nil {
		return nil, err
	}

	// Players (joined with identity), ordered as a scoreboard by kills.
	statRows, err := r.db.QueryContext(ctx,
		`SELECT s.team_id, u.id, u.steam_id, u.steam_username,
		        s.kills, s.deaths, s.assists, s.kd_ratio, s.mvps
		 FROM Stats s
		 JOIN Users u ON u.id = s.player_id
		 WHERE s.match_id = ?
		 ORDER BY s.kills DESC`, id)
	if err != nil {
		return nil, err
	}
	defer statRows.Close()
	for statRows.Next() {
		var teamID int64
		var p PlayerStatDetail
		if err := statRows.Scan(
			&teamID, &p.PlayerID, &p.SteamID, &p.SteamUsername,
			&p.Kills, &p.Deaths, &p.Assists, &p.KDRatio, &p.MVPs,
		); err != nil {
			return nil, err
		}
		if i, ok := teamIndex[teamID]; ok {
			detail.Teams[i].Players = append(detail.Teams[i].Players, p)
		}
	}
	if err := statRows.Err(); err != nil {
		return nil, err
	}

	return detail, nil
}

func (r *sqlMatchRepository) ListPlayerIDs(ctx context.Context) ([]int64, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id FROM Users ORDER BY id`)
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
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return ids, nil
}

func (r *sqlMatchRepository) Create(ctx context.Context, nm NewMatch) (*Match, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	// Rolled back automatically if we return before Commit; a no-op afterwards.
	defer tx.Rollback()

	// The match is fully "processed" since we already have its stats.
	res, err := tx.ExecContext(ctx,
		`INSERT INTO Matches (map, played_at, uploaded_at, upload_hash, processed, season_id, total_rounds)
		 VALUES (?, ?, ?, ?, 1, ?, ?)`,
		nm.Map, nm.PlayedAt, nm.UploadedAt, nm.UploadHash, nm.SeasonID, nm.TotalRounds)
	if err != nil {
		return nil, err
	}
	matchID, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}

	for _, team := range nm.Teams {
		teamRes, err := tx.ExecContext(ctx,
			`INSERT INTO match_teams (match_id, team_slot, starting_side, rounds_won, result)
			 VALUES (?, ?, ?, ?, ?)`,
			matchID, team.TeamSlot, team.StartingSide, team.RoundsWon, team.Result)
		if err != nil {
			return nil, err
		}
		teamID, err := teamRes.LastInsertId()
		if err != nil {
			return nil, err
		}

		for _, p := range team.Players {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO Stats (match_id, team_id, player_id, kills, deaths, assists, kd_ratio, mvps)
				 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
				matchID, teamID, p.PlayerID, p.Kills, p.Deaths, p.Assists, p.KDRatio, p.MVPs); err != nil {
				return nil, err
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &Match{
		ID:          matchID,
		Map:         nm.Map,
		PlayedAt:    nm.PlayedAt,
		UploadedAt:  nm.UploadedAt,
		UploadHash:  nm.UploadHash,
		Processed:   true,
		SeasonID:    nm.SeasonID,
		TotalRounds: nm.TotalRounds,
	}, nil
}
