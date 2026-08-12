package matches

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// matchColumns is the shared column list for reading a full match row.
const matchColumns = `id, map, played_at, upload_hash, status, season_id, total_rounds, created_at, storage_key`

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
	// FindByHash returns the match with the given upload hash, or (nil, nil) if
	// none exists — used for dedup at upload time.
	FindByHash(ctx context.Context, uploadHash string) (*Match, error)
	// FindByStorageKey returns the match with the given storage key, or (nil, nil)
	// if none exists — used to correlate parser events to a match.
	FindByStorageKey(ctx context.Context, storageKey string) (*Match, error)
	// UpdateStatus sets a match's lifecycle status. The bool reports whether a
	// matching row existed.
	UpdateStatus(ctx context.Context, id int64, status string) (bool, error)
	// EnsureUserBySteamID returns the internal user id for the given Steam ID,
	// creating the Users row (with displayName) when none exists yet.
	EnsureUserBySteamID(ctx context.Context, steamID int64, displayName string) (int64, error)
	// CurrentSeasonID returns the season active now, falling back to the latest.
	CurrentSeasonID(ctx context.Context) (int64, error)
	// CreatePending inserts a new match in the 'pending' state (before upload).
	CreatePending(ctx context.Context, uploadHash, storageKey string, seasonID int64) (*Match, error)
	// MarkUploaded flips a match to 'uploaded'. The bool reports whether a
	// matching row existed.
	MarkUploaded(ctx context.Context, id int64) (bool, error)
	// CompleteFromParse fills a match from parser output and inserts its teams +
	// stats, all in one transaction, setting status to 'processed'. The bool
	// reports whether a matching row existed.
	CompleteFromParse(ctx context.Context, id int64, pr ParseResult) (bool, error)
	// ListPlayerIDs returns the primary keys of every user, used by the random
	// generator to assign players to a match.
	ListPlayerIDs(ctx context.Context) ([]int64, error)
	// Create persists a fully formed (random) match and its teams/stats in one
	// transaction, returning the created match.
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

// scanMatch scans a full match row (matchColumns order) from a Scanner.
func scanMatch(s interface{ Scan(...any) error }) (Match, error) {
	var m Match
	err := s.Scan(
		&m.ID, &m.Map, &m.PlayedAt, &m.UploadHash,
		&m.Status, &m.SeasonID, &m.TotalRounds, &m.CreatedAt, &m.StorageKey,
	)
	return m, err
}

func (r *sqlMatchRepository) List(ctx context.Context, seasonID *int64) ([]Match, error) {
	// TODO: when seasonID != nil, append "WHERE season_id = ?" and pass the
	//       value as a query arg. For now this returns all matches regardless.
	_ = seasonID

	rows, err := r.db.QueryContext(ctx, `SELECT `+matchColumns+` FROM matches`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	matches := make([]Match, 0)
	for rows.Next() {
		m, err := scanMatch(rows)
		if err != nil {
			return nil, err
		}
		matches = append(matches, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return matches, nil
}

func (r *sqlMatchRepository) FindByHash(ctx context.Context, uploadHash string) (*Match, error) {
	m, err := scanMatch(r.db.QueryRowContext(ctx,
		`SELECT `+matchColumns+` FROM matches WHERE upload_hash = ?`, uploadHash))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *sqlMatchRepository) FindByStorageKey(ctx context.Context, storageKey string) (*Match, error) {
	m, err := scanMatch(r.db.QueryRowContext(ctx,
		`SELECT `+matchColumns+` FROM matches WHERE storage_key = ?`, storageKey))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *sqlMatchRepository) UpdateStatus(ctx context.Context, id int64, status string) (bool, error) {
	res, err := r.db.ExecContext(ctx,
		`UPDATE matches SET status = ? WHERE id = ?`, status, id)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (r *sqlMatchRepository) EnsureUserBySteamID(ctx context.Context, steamID int64, displayName string) (int64, error) {
	var id int64
	err := r.db.QueryRowContext(ctx,
		`SELECT id FROM Users WHERE steam_id = ?`, steamID).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}

	now := time.Now().Format(time.RFC3339)
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO Users (steam_id, steam_username, created_at) VALUES (?, ?, ?)`,
		steamID, displayName, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (r *sqlMatchRepository) CurrentSeasonID(ctx context.Context) (int64, error) {
	today := time.Now().Format("2006-01-02")

	var id int64
	err := r.db.QueryRowContext(ctx,
		`SELECT id FROM seasons WHERE start_at <= ? AND end_at >= ? ORDER BY id DESC LIMIT 1`,
		today, today).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		// No season covers today; fall back to the most recent one.
		err = r.db.QueryRowContext(ctx,
			`SELECT id FROM seasons ORDER BY id DESC LIMIT 1`).Scan(&id)
	}
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (r *sqlMatchRepository) CreatePending(ctx context.Context, uploadHash, storageKey string, seasonID int64) (*Match, error) {
	now := time.Now().Format(time.RFC3339)
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO matches (upload_hash, status, season_id, created_at, storage_key)
		 VALUES (?, 'pending', ?, ?, ?)`,
		uploadHash, seasonID, now, storageKey)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}

	return &Match{
		ID:         id,
		UploadHash: uploadHash,
		Status:     "pending",
		SeasonID:   seasonID,
		CreatedAt:  &now,
		StorageKey: storageKey,
	}, nil
}

func (r *sqlMatchRepository) MarkUploaded(ctx context.Context, id int64) (bool, error) {
	res, err := r.db.ExecContext(ctx,
		`UPDATE matches SET status = 'uploaded' WHERE id = ?`, id)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (r *sqlMatchRepository) CompleteFromParse(ctx context.Context, id int64, pr ParseResult) (bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx,
		`UPDATE matches SET map = ?, played_at = ?, total_rounds = ?, status = 'processed' WHERE id = ?`,
		pr.Map, pr.PlayedAt, pr.TotalRounds, id)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	if n == 0 {
		return false, nil // no such match
	}

	if err := insertTeamsAndStats(ctx, tx, id, pr.Teams); err != nil {
		return false, err
	}

	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func (r *sqlMatchRepository) GetDetailByID(ctx context.Context, id int64) (*MatchDetail, error) {
	m, err := scanMatch(r.db.QueryRowContext(ctx,
		`SELECT `+matchColumns+` FROM matches WHERE id = ?`, id))
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
		        s.kills, s.deaths, s.assists, s.kd_ratio, s.mvps,
		        s.damage_assists, s.flash_assists, s.headshot_kills,
		        s.total_damage, s.utility_damage, s.rounds_played
		 FROM stats s
		 JOIN users u ON u.id = s.player_id
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
			&p.DamageAssists, &p.FlashAssists, &p.HeadshotKills,
			&p.TotalDamage, &p.UtilityDamage, &p.RoundsPlayed,
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
	rows, err := r.db.QueryContext(ctx, `SELECT id FROM users ORDER BY id`)
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
	defer tx.Rollback()

	now := time.Now().Format(time.RFC3339)
	// A generated match is already fully known, so it goes straight to 'processed'.
	res, err := tx.ExecContext(ctx,
		`INSERT INTO matches (map, played_at, upload_hash, status, season_id, total_rounds, created_at, storage_key)
		 VALUES (?, ?, ?, 'processed', ?, ?, ?, ?)`,
		nm.Map, nm.PlayedAt, nm.UploadHash, nm.SeasonID, nm.TotalRounds, now, nm.StorageKey)
	if err != nil {
		return nil, err
	}
	matchID, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}

	if err := insertTeamsAndStats(ctx, tx, matchID, nm.Teams); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &Match{
		ID:          matchID,
		Map:         &nm.Map,
		PlayedAt:    &nm.PlayedAt,
		UploadHash:  nm.UploadHash,
		Status:      "processed",
		SeasonID:    nm.SeasonID,
		TotalRounds: &nm.TotalRounds,
		CreatedAt:   &now,
		StorageKey:  nm.StorageKey,
	}, nil
}

// insertTeamsAndStats inserts both teams and each team's player stats for a match
// within the given transaction. Shared by Create and CompleteFromParse.
func insertTeamsAndStats(ctx context.Context, tx *sql.Tx, matchID int64, teams []NewTeam) error {
	for _, team := range teams {
		teamRes, err := tx.ExecContext(ctx,
			`INSERT INTO match_teams (match_id, team_slot, starting_side, rounds_won, result)
			 VALUES (?, ?, ?, ?, ?)`,
			matchID, team.TeamSlot, team.StartingSide, team.RoundsWon, team.Result)
		if err != nil {
			return err
		}
		teamID, err := teamRes.LastInsertId()
		if err != nil {
			return err
		}

		for _, p := range team.Players {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO stats (match_id, team_id, player_id, kills, deaths, assists, kd_ratio, mvps,
				                    damage_assists, flash_assists, headshot_kills, total_damage, utility_damage, rounds_played)
				 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				matchID, teamID, p.PlayerID, p.Kills, p.Deaths, p.Assists, p.KDRatio, p.MVPs,
				p.DamageAssists, p.FlashAssists, p.HeadshotKills, p.TotalDamage, p.UtilityDamage, p.RoundsPlayed); err != nil {
				return err
			}
		}
	}
	return nil
}
