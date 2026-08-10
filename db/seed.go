package db

import (
	"embed"
	"errors"
	"io/fs"
	"log"
	"os"
	"path/filepath"
)

//go:embed seed.db
var seedFS embed.FS

// EnsureSeeded copies the embedded seed database to dstPath only if no file
// already exists there. On a fresh persistent disk (first deploy) this lays down
// the schema and initial data; on every later deploy the existing live data on
// the disk is left untouched.
func EnsureSeeded(dstPath string) error {
	if _, err := os.Stat(dstPath); err == nil {
		return nil // already present — keep live data
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err // a real error (permissions, bad path, ...)
	}

	if dir := filepath.Dir(dstPath); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	seed, err := seedFS.ReadFile("seed.db")
	if err != nil {
		return err
	}

	// Write to a temp file then rename so an interrupted copy can't leave a
	// half-written file that then "exists" and blocks a correct re-seed.
	tmp := dstPath + ".tmp"
	if err := os.WriteFile(tmp, seed, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, dstPath); err != nil {
		return err
	}

	log.Printf("seeded new database at %s", dstPath)
	return nil
}
