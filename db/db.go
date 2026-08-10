package db

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// Open opens the SQLite database at the given path and verifies the connection
// with a Ping. The caller is responsible for closing the returned *sql.DB.
//
// It enables WAL journalling (better read/write concurrency for an HTTP server),
// a busy timeout (so brief write contention retries instead of erroring with
// "database is locked"), and foreign-key enforcement (off by default in SQLite).
func Open(path string) (*sql.DB, error) {
	dsn := fmt.Sprintf(
		"file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)",
		path,
	)

	database, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}

	if err := database.Ping(); err != nil {
		database.Close()
		return nil, err
	}

	return database, nil
}
