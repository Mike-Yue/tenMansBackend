-- +goose Up
-- Per-season player ratings (OpenSkill). Derived data: a season's rows are recomputed from
-- that season's full match history and rewritten whenever its processed-match set changes.
-- ON DELETE CASCADE keeps this derived table from ever blocking user/season deletes.
CREATE TABLE IF NOT EXISTS "player_ratings" (
  player_id    INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  season_id    INTEGER NOT NULL REFERENCES seasons (id) ON DELETE CASCADE,
  mu           REAL NOT NULL,
  sigma        REAL NOT NULL,
  ordinal      REAL NOT NULL,
  games_played INTEGER NOT NULL,
  updated_at   TEXT NOT NULL,
  PRIMARY KEY (player_id, season_id)
);

-- +goose Down
DROP TABLE IF EXISTS "player_ratings";
