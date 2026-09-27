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

	if err := migrateColumns(db); err != nil {
		_ = db.Close()

		return nil, err
	}

	if err := backfillOwnership(db); err != nil {
		_ = db.Close()

		return nil, err
	}

	return &Store{db: db}, nil
}

// migrateColumns adds every column that's been added to the schema since
// this service's first release - CREATE TABLE IF NOT EXISTS is a no-op
// against a database that already has the table, so each one needs its
// own idempotent path to reach an existing file.
func migrateColumns(db *sql.DB) error {
	if err := addColumnIfMissing(db, "audit_log", "ip", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}

	if err := addColumnIfMissing(db, "objects", "description", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}

	// owner: the admin account that created a token over HTTP, absent
	// (NULL) for one issued via the CLI's direct-DB path - never a
	// guessed value (openspec/changes/web-ui/design.md's "Token ownership
	// is nullable, not backfilled or guessed" decision).
	if err := addColumnIfMissing(db, "write_tokens", "owner", "TEXT"); err != nil {
		return err
	}

	// revoked_at: NULL while valid, set once revoked - a soft-delete so a
	// revoked token's audit history still resolves to a real description
	// and owner instead of a bare id (openspec/changes/web-ui/design.md's
	// "Token revocation moves from DELETE ... to a revoked_at timestamp
	// column" decision).
	if err := addColumnIfMissing(db, "write_tokens", "revoked_at", "TEXT"); err != nil {
		return err
	}

	// actor_type/actor_id: the verified credential (a token's id, or the
	// admin account for a session) that authenticated a write, kept
	// separate from the existing, self-reported, unverified caller column
	// rather than overwriting it (openspec/changes/web-ui/design.md's
	// "Audit log actor" decision).
	if err := addColumnIfMissing(db, "audit_log", "actor_type", "TEXT"); err != nil {
		return err
	}

	if err := addColumnIfMissing(db, "audit_log", "actor_id", "TEXT"); err != nil {
		return err
	}

	// last_used_at: NULL until a token first authenticates a write, then
	// its most recent success - the same signal webauthn_credentials'
	// own last_used_at already gives an admin for a passkey
	// (openspec/changes/tokens-last-used-at/proposal.md).
	if err := addColumnIfMissing(db, "write_tokens", "last_used_at", "TEXT"); err != nil {
		return err
	}

	// backup_eligible: the WebAuthn BE flag captured at registration -
	// go-webauthn's own login validation rejects the ceremony outright
	// if this disagrees with what a later assertion reports, so it has
	// to survive a restart the same way sign_count does
	// (alrayyes/hush-hush#260). Defaults false for a row that predates
	// this column; a credential that's actually backup-eligible and was
	// registered before this migration still needs re-registering to
	// pick up the correct value - there's nothing to derive it from on
	// an existing row.
	if err := addColumnIfMissing(db, "webauthn_credentials", "backup_eligible", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}

	// user_id: foreign-keys a credential to the users row it belongs to
	// (openspec/changes/client-side-encryption/design.md's "users table
	// and owner_id/user_id foreign keys" decision). Left nullable here -
	// backfillOwnership (users.go) is what actually fills it in for
	// every pre-existing row, since ADD COLUMN's DEFAULT can't be a
	// dynamically generated id. This migration doesn't start setting it
	// on a new registration; that's tasks.md group 3's job.
	if err := addColumnIfMissing(db, "webauthn_credentials", "user_id", "TEXT REFERENCES users(id)"); err != nil {
		return err
	}

	// owner_id: foreign-keys an object to the users row that created it,
	// nullable during backfill (design.md's Migration Plan step 1) and
	// left nullable afterward too - it's accountability metadata, not an
	// access-control mechanism (design.md's own decision on that), so
	// there's no invariant requiring every row to have one. backfillOwnership
	// (users.go) fills it in for every pre-existing row; this migration
	// doesn't start setting it on a new write, that's tasks.md group 5's
	// job.
	if err := addColumnIfMissing(db, "objects", "owner_id", "TEXT REFERENCES users(id)"); err != nil {
		return err
	}

	// public_key: a consumer's registered age public key, safe to store
	// server-side since it's public (design.md's "Consumer public key: a
	// plain stored field, not a directory service" decision). Nullable -
	// most consumers still have none, and nothing backfills one for a
	// pre-existing row; SetConsumerPublicKey (objects.go) is what sets it,
	// upserting a consumers row for a name that previously only existed
	// via used_by.
	if err := addColumnIfMissing(db, "consumers", "public_key", "TEXT"); err != nil {
		return err
	}

	// users.public_key/recovery_wrapped_identity: the user's escrowed
	// writer identity (specs/users/spec.md's "Escrowed writer identity"
	// and "Break-glass recovery phrase" requirements) - a real age
	// keypair generated client-side once, whose private key never
	// reaches this server unwrapped. public_key is the age recipient
	// string, safe to store in the clear; recovery_wrapped_identity is
	// the private key wrapped by a key derived from the one-time
	// break-glass recovery phrase - never the phrase itself, which this
	// server never sees at all. Both nullable until a first registration
	// establishes them (tasks.md group 3's job, not this migration's) -
	// there's nothing to backfill for a pre-existing user, since there
	// was no client-side identity to generate before this change
	// shipped.
	if err := addColumnIfMissing(db, "users", "public_key", "TEXT"); err != nil {
		return err
	}

	if err := addColumnIfMissing(db, "users", "recovery_wrapped_identity", "TEXT"); err != nil {
		return err
	}

	// wrapped_identity: this credential's own copy of the user's escrowed
	// writer identity private key, wrapped with a key derived from the
	// credential's WebAuthn PRF extension output, base64-encoded
	// (specs/users/spec.md's "Per-credential wrapping of the escrowed
	// identity" requirement). NULL for a credential that doesn't support
	// the PRF extension, or one registered before this change shipped.
	// Deleting a credential just deletes its own copy here - every other
	// credential's copy already stands on its own, which is what makes
	// losing one passkey non-stranding (design.md's "Multi-copy wrapping
	// over a single shared wrap" decision).
	return addColumnIfMissing(db, "webauthn_credentials", "wrapped_identity", "TEXT")
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
