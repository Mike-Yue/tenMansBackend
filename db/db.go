package db

import (
	"database/sql"

	_ "modernc.org/sqlite"
)

// Open opens the SQLite database at the given path and verifies the connection
// with a Ping. The caller is responsible for closing the returned *sql.DB.
func Open(path string) (*sql.DB, error) {
	database, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	if err := database.Ping(); err != nil {
		database.Close()
		return nil, err
	}

	return database, nil
}
