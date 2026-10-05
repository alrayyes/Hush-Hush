// Package store persists objects, their used_by lineage, and the audit log
// in SQLite. See openspec/changes/secrets-object-store/design.md for why
// SQLite over a plain key-value store.
package store

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
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

	if err := migrateObjectSlugs(db); err != nil {
		_ = db.Close()

		return nil, err
	}

	if err := addAuditLogVariantColumn(db); err != nil {
		_ = db.Close()

		return nil, err
	}

	if err := migrateObjectIDsToUUID(db); err != nil {
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
	if err := addColumnIfMissing(db, "webauthn_credentials", "wrapped_identity", "TEXT"); err != nil {
		return err
	}

	return addObjectsSlugColumn(db)
}

// addObjectsSlugColumn adds objects.slug (the caller-facing identifier
// objects.id used to be, now split out so id can become an opaque internal
// one - schema.sql's own comment on objects, tasks.md group 6,
// specs/secret-objects/spec.md's "Internal id decoupled from user-facing
// slug" requirement) and its unique index, broken out of migrateColumns to
// keep that function's own cognitive complexity down.
//
// slug is added nullable, like every other column migrateColumns adds,
// because ALTER TABLE ADD COLUMN can't backfill a NOT NULL value per row
// on its own - migrateObjectSlugs is what actually backfills it (and
// generates a fresh internal id) for every pre-existing row; every row
// created after this shipped has it set at insert time (objects.go's
// CreateObject), so in practice it's never NULL once a database has been
// opened by a binary carrying this migration.
//
// A plain ALTER TABLE ADD COLUMN can't attach a UNIQUE constraint either
// (SQLite rejects it outright against a table that already has rows), so
// uniqueness is enforced by a separate index instead - functionally
// equivalent, and safe to create before migrateObjectSlugs backfills any
// values: SQLite's unique index treats every NULL as distinct from every
// other, so a column that's still all-NULL at this point can't violate it.
func addObjectsSlugColumn(db *sql.DB) error {
	if err := addColumnIfMissing(db, "objects", "slug", "TEXT"); err != nil {
		return err
	}

	// A slug is no longer unique: it can hold one variant per group of
	// consumers (alrayyes/hush-hush#668), with "a consumer is in at most one
	// variant" enforced in code, in the transaction that writes. A database
	// from before that still has the unique index, so drop it and keep a
	// plain one for lookups.
	if _, err := db.Exec(`DROP INDEX IF EXISTS idx_objects_slug`); err != nil {
		return fmt.Errorf("drop unique objects slug index: %w", err)
	}

	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_objects_slug_lookup ON objects (slug)`); err != nil {
		return fmt.Errorf("create objects slug index: %w", err)
	}

	return nil
}

// migrateObjectSlugs backfills objects.slug (added, nullable, by
// migrateColumns above) for every pre-existing row - slug IS NULL is
// exactly the set of rows a database created before this shipped, since
// CreateObject always sets it going forward, making this check its own
// idempotency guard the same way backfillOwnership's WHERE owner_id IS
// NULL is.
//
// This is more involved than migrateColumns' own ADD COLUMN pattern: a
// pre-existing row's current id becomes its slug verbatim, but that frees
// up id to become a freshly generated, opaque internal identifier
// (randomHex, the same convention tokens.go/users.go already use) - and
// used_by.object_id (schema.sql: ON DELETE CASCADE, not ON UPDATE
// CASCADE) already stores that row's OLD id, so it has to be repointed at
// the new one in the same transaction or it's left referencing a row that
// no longer exists under that id. PRAGMA defer_foreign_keys defers the
// foreign key check SQLite would otherwise run immediately after each
// statement to COMMIT instead, which is what makes updating objects.id
// and used_by.object_id in either order, within one transaction, safe.
func migrateObjectSlugs(db *sql.DB) error {
	oldIDs, err := objectIDsPendingSlugMigration(db)
	if err != nil {
		return err
	}

	if len(oldIDs) == 0 {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Scoped to this transaction alone, and automatically switched back
	// off at its end (SQLite's own documented behaviour for this pragma) -
	// nothing outside this function runs with foreign key checks
	// deferred.
	if _, err := tx.Exec(`PRAGMA defer_foreign_keys = ON`); err != nil {
		return fmt.Errorf("defer foreign keys: %w", err)
	}

	for _, oldID := range oldIDs {
		newID, err := newUUID()
		if err != nil {
			return fmt.Errorf("generate object id: %w", err)
		}

		if _, err := tx.Exec(`UPDATE used_by SET object_id = ? WHERE object_id = ?`, newID, oldID); err != nil {
			return fmt.Errorf("repoint used_by for slug migration: %w", err)
		}

		if _, err := tx.Exec(`UPDATE objects SET id = ?, slug = ? WHERE id = ?`, newID, oldID, oldID); err != nil {
			return fmt.Errorf("split object id/slug: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// objectIDsPendingSlugMigration returns the internal id of every objects
// row that still has no slug - broken out of migrateObjectSlugs to keep
// that function's own cognitive complexity down, and so the query's rows
// handle can be closed with a plain defer instead of a manual Close on
// every early return.
func objectIDsPendingSlugMigration(db *sql.DB) ([]string, error) {
	rows, err := db.Query(`SELECT id FROM objects WHERE slug IS NULL`)
	if err != nil {
		return nil, fmt.Errorf("find objects pending slug migration: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan object pending slug migration: %w", err)
		}

		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate objects pending slug migration: %w", err)
	}

	return ids, nil
}

// addAuditLogVariantColumn adds audit_log.variant_id, the variant of a name an
// entry is about, when it has one (alrayyes/hush-hush#684): a name can hold
// several objects, and object_id only holds the name. Nullable, since an entry
// written before this has none.
func addAuditLogVariantColumn(db *sql.DB) error {
	return addColumnIfMissing(db, "audit_log", "variant_id", "TEXT")
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

// Ready reports whether the database can actually serve a request: the
// connection answers a ping and a trivial read succeeds. It reads rather
// than writes, so a probe every few seconds never contends with real
// traffic.
func (s *Store) Ready(ctx context.Context) error {
	if err := s.db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}

	var one int
	if err := s.db.QueryRowContext(ctx, `SELECT 1 FROM objects LIMIT 1`).Scan(&one); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("read database: %w", err)
	}

	return nil
}

// Close closes the underlying database.
func (s *Store) Close() error {
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("close database: %w", err)
	}

	return nil
}
