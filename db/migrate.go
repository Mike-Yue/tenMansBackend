package db

import (
	"database/sql"
	"embed"

	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Migrate applies any pending migrations to database. It is safe to call on every
// startup: goose records applied migrations in the goose_db_version table and skips
// ones already run, so this is a no-op once the database is up to date.
//
// The migrations are embedded, so they ship inside the binary — no files need to exist
// alongside the deployed server.
func Migrate(database *sql.DB) error {
	goose.SetBaseFS(migrationsFS)
	// "sqlite3" selects goose's SQL dialect; the underlying driver is modernc's
	// "sqlite" registered in db.Open. The two names are unrelated on purpose.
	if err := goose.SetDialect("sqlite3"); err != nil {
		return err
	}
	return goose.Up(database, "migrations")
}
