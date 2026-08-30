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

	if err := addColumnIfMissing(db, "audit_log", "ip", "TEXT NOT NULL DEFAULT ''"); err != nil {
		_ = db.Close()

		return nil, err
	}

	return &Store{db: db}, nil
}

// addColumnIfMissing adds column to table if it isn't already there. The
// schema's CREATE TABLE IF NOT EXISTS is a no-op against a database that
// already has table, so a column added after a table's first release needs
// its own idempotent path to reach an existing file - modernc.org/sqlite's
// SQLite build doesn't support ALTER TABLE ... ADD COLUMN IF NOT EXISTS, so
// this checks first instead.
func addColumnIfMissing(db *sql.DB, table, column, definition string) error {
	rows, err := db.Query(`SELECT name FROM pragma_table_info(?)`, table)
	if err != nil {
		return fmt.Errorf("inspect %s columns: %w", table, err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return fmt.Errorf("scan %s column name: %w", table, err)
		}

		if name == column {
			return nil
		}
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate %s columns: %w", table, err)
	}

	// table and column are always call-site literals, never a caller-
	// supplied value - safe to interpolate, since a placeholder can't
	// stand in for an identifier.
	stmt := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, definition)

	if _, err := db.Exec(stmt); err != nil {
		return fmt.Errorf("add %s.%s: %w", table, column, err)
	}

	return nil
}

// DB returns the underlying database handle.
func (s *Store) DB() *sql.DB {
	return s.db
}

// Close closes the underlying database.
func (s *Store) Close() error {
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("close database: %w", err)
	}

	return nil
}
