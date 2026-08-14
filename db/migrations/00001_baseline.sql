-- +goose Up
-- Baseline schema, captured verbatim from the pre-migration production.db (quoting and
-- identifier casing preserved intentionally). Uses IF NOT EXISTS so it is a no-op against
-- the existing prod database (which already has these tables) yet builds the full schema
-- on a fresh, empty database.
CREATE TABLE IF NOT EXISTS "users" (id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL, steam_id NUMERIC UNIQUE NOT NULL, steam_username TEXT, created_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS "seasons" (id INTEGER PRIMARY KEY AUTOINCREMENT UNIQUE NOT NULL, name TEXT NOT NULL, start_at TEXT NOT NULL, end_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS "matches" (id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL UNIQUE, map TEXT, played_at TEXT, upload_hash TEXT NOT NULL UNIQUE, status TEXT NOT NULL CHECK (status IN ('pending', 'uploaded', 'processed', 'failed')), season_id INTEGER REFERENCES seasons (id) ON DELETE NO ACTION, total_rounds INTEGER, created_at TEXT, storage_key TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS "match_teams" (id INTEGER PRIMARY KEY AUTOINCREMENT, match_id INTEGER REFERENCES Matches (id), team_slot TEXT CHECK (team_slot IN ('A', 'B')) NOT NULL, starting_side TEXT CHECK (starting_side IN ('T', 'CT')) NOT NULL, rounds_won INTEGER NOT NULL, result TEXT CHECK (result IN ('win', 'loss', 'tie')) NOT NULL);
CREATE TABLE IF NOT EXISTS "stats" (id INTEGER PRIMARY KEY AUTOINCREMENT, match_id NUMERIC REFERENCES Matches (id) NOT NULL, team_id INTEGER REFERENCES match_teams (id) NOT NULL, player_id INTEGER REFERENCES Users (id) NOT NULL, kills INTEGER NOT NULL, deaths INTEGER NOT NULL, assists INTEGER NOT NULL, kd_ratio NUMERIC NOT NULL, mvps INTEGER NOT NULL, flash_assists INTEGER NOT NULL, headshot_kills INTEGER NOT NULL, total_damage INTEGER NOT NULL, utility_damage INTEGER NOT NULL, damage_assists INTEGER NOT NULL, rounds_played INTEGER NOT NULL);

-- +goose Down
DROP TABLE IF EXISTS "stats";
DROP TABLE IF EXISTS "match_teams";
DROP TABLE IF EXISTS "matches";
DROP TABLE IF EXISTS "seasons";
DROP TABLE IF EXISTS "users";
