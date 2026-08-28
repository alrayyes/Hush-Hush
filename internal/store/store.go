// Package store persists objects, their used_by lineage, and the audit log
// in SQLite. See openspec/changes/secrets-object-store/design.md for why
// SQLite over a plain key-value store.
package store

import (
	"database/sql"
	_ "embed"
	"fmt"

	// Registers the "sqlite" driver with database/sql; nothing here calls
	// it by name.
	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema string

// Store wraps a SQLite database with the schema this service needs already
// applied.
type Store struct {
	db *sql.DB
}

// Open opens (creating if necessary) a SQLite database at path and applies
// the schema. path may be ":memory:" for an in-memory database.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// objects.id -> used_by.object_id is declared ON DELETE CASCADE in the
	// schema, but SQLite ignores foreign keys unless a connection turns
	// them on for itself - it's a per-connection pragma, not a database
	// setting.
	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}

	return &Store{db: db}, nil
}

// DB returns the underlying database handle.
func (s *Store) DB() *sql.DB {
	return s.db
}

// Close closes the underlying database.
func (s *Store) Close() error {
	return s.db.Close()
}
