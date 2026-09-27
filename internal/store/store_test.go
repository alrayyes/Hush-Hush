package store_test

import (
	"database/sql"
	"testing"

	"github.com/alrayyes/hush-hush/internal/store"
	"github.com/stretchr/testify/require"
)

func TestOpenAppliesSchemaToFreshDatabase(t *testing.T) {
	t.Parallel()

	s, err := store.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, s.Close()) })

	tables := tableNames(t, s)
	require.Contains(t, tables, "objects")
	require.Contains(t, tables, "used_by")
	require.Contains(t, tables, "audit_log")
}

func TestOpenAppliesSchemaToFreshDatabaseWebAuthnTables(t *testing.T) {
	t.Parallel()

	s, err := store.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, s.Close()) })

	tables := tableNames(t, s)
	require.Contains(t, tables, "webauthn_credentials")
	require.Contains(t, tables, "sessions")
	require.Contains(t, tables, "webauthn_ceremonies")
}

func TestOpenAppliesSchemaToFreshDatabaseConsumersTable(t *testing.T) {
	t.Parallel()

	s, err := store.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, s.Close()) })

	tables := tableNames(t, s)
	require.Contains(t, tables, "consumers")
}

func TestOpenMigratesExistingTokensAndAuditLogWithNewColumns(t *testing.T) {
	// Simulates a database created before owner/revoked_at (write_tokens)
	// and actor_type/actor_id (audit_log) existed, to prove
	// addColumnIfMissing actually reaches a pre-existing file rather than
	// only ever being exercised by a fresh CREATE TABLE that already has
	// the columns.
	t.Parallel()

	dir := t.TempDir()
	path := dir + "/hush-hush.db"

	legacy, err := sql.Open("sqlite", path)
	require.NoError(t, err)

	_, err = legacy.Exec(`
		CREATE TABLE write_tokens (
			id TEXT PRIMARY KEY,
			token_hash TEXT NOT NULL UNIQUE,
			description TEXT NOT NULL,
			created_at TEXT NOT NULL,
			expires_at TEXT NOT NULL
		);
		CREATE TABLE audit_log (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			object_id TEXT NOT NULL,
			action TEXT NOT NULL,
			caller TEXT,
			ip TEXT NOT NULL DEFAULT '',
			timestamp TEXT NOT NULL
		);
		INSERT INTO write_tokens (id, token_hash, description, created_at, expires_at)
			VALUES ('legacy-id', 'legacy-hash', 'pre-existing token', '2026-01-01T00:00:00Z', '2026-02-01T00:00:00Z');
		INSERT INTO audit_log (object_id, action, caller, ip, timestamp)
			VALUES ('legacy-object', 'create', 'legacy-caller', '203.0.113.1', '2026-01-01T00:00:00Z');
	`)
	require.NoError(t, err)
	require.NoError(t, legacy.Close())

	s, err := store.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, s.Close()) })

	var owner, revokedAt sql.NullString
	require.NoError(t, s.DB().QueryRow(
		`SELECT owner, revoked_at FROM write_tokens WHERE id = 'legacy-id'`,
	).Scan(&owner, &revokedAt))
	require.False(t, owner.Valid)
	require.False(t, revokedAt.Valid)

	var description string
	require.NoError(t, s.DB().QueryRow(
		`SELECT description FROM write_tokens WHERE id = 'legacy-id'`,
	).Scan(&description))
	require.Equal(t, "pre-existing token", description)

	var actorType, actorID sql.NullString
	require.NoError(t, s.DB().QueryRow(
		`SELECT actor_type, actor_id FROM audit_log WHERE object_id = 'legacy-object'`,
	).Scan(&actorType, &actorID))
	require.False(t, actorType.Valid)
	require.False(t, actorID.Valid)
}

func TestOpenIsIdempotent(t *testing.T) {
	// A fresh database, then Open again against the same file, must not
	// error - the schema is applied with CREATE TABLE IF NOT EXISTS, not a
	// one-shot migration that fails the second time it runs.
	t.Parallel()

	dir := t.TempDir()
	path := dir + "/hush-hush.db"

	first, err := store.Open(path)
	require.NoError(t, err)
	require.NoError(t, first.Close())

	second, err := store.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, second.Close()) })

	tables := tableNames(t, second)
	require.Contains(t, tables, "objects")
}

// TestOpenBackfillsExactlyOneUser covers tasks.md group 1.1: a fresh
// database ends up with exactly one users row - the sole admin account
// modelled as a real row rather than an implicit fact about the schema
// (openspec/changes/client-side-encryption/design.md).
func TestOpenBackfillsExactlyOneUser(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	var count int
	require.NoError(t, s.DB().QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count))
	require.Equal(t, 1, count)
}

// TestOpenReopenDoesNotMintASecondUser covers reopening an
// already-migrated database not creating a second users row, nor
// changing the id of the existing one.
func TestOpenReopenDoesNotMintASecondUser(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := dir + "/hush-hush.db"

	first, err := store.Open(path)
	require.NoError(t, err)

	var firstID string
	require.NoError(t, first.DB().QueryRow(`SELECT id FROM users LIMIT 1`).Scan(&firstID))
	require.NoError(t, first.Close())

	second, err := store.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, second.Close()) })

	var count int
	require.NoError(t, second.DB().QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count))
	require.Equal(t, 1, count)

	var secondID string
	require.NoError(t, second.DB().QueryRow(`SELECT id FROM users LIMIT 1`).Scan(&secondID))
	require.Equal(t, firstID, secondID)
}

// TestOpenBackfillsCredentialUserID covers tasks.md group 1.2: a
// credential row from before webauthn_credentials.user_id existed gets
// backfilled, on Open, to reference the sole users row.
func TestOpenBackfillsCredentialUserID(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := dir + "/hush-hush.db"

	legacy, err := sql.Open("sqlite", path)
	require.NoError(t, err)

	_, err = legacy.Exec(`
		CREATE TABLE webauthn_credentials (
			id TEXT PRIMARY KEY,
			public_key BLOB NOT NULL,
			sign_count INTEGER NOT NULL DEFAULT 0,
			aaguid TEXT NOT NULL DEFAULT '',
			nickname TEXT NOT NULL,
			created_at TEXT NOT NULL,
			last_used_at TEXT
		);
		INSERT INTO webauthn_credentials (id, public_key, nickname, created_at)
			VALUES ('legacy-cred', X'00', 'legacy passkey', '2026-01-01T00:00:00Z');
	`)
	require.NoError(t, err)
	require.NoError(t, legacy.Close())

	s, err := store.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, s.Close()) })

	var userID sql.NullString
	require.NoError(t, s.DB().QueryRow(
		`SELECT user_id FROM webauthn_credentials WHERE id = 'legacy-cred'`,
	).Scan(&userID))
	require.True(t, userID.Valid)
	require.NotEmpty(t, userID.String)

	var soleUserID string
	require.NoError(t, s.DB().QueryRow(`SELECT id FROM users LIMIT 1`).Scan(&soleUserID))
	require.Equal(t, soleUserID, userID.String)
}

// TestOpenBackfillsObjectOwnerID covers tasks.md group 1.3: every
// pre-existing object ends up with a non-null owner_id, referencing the
// sole users row, after Open backfills it.
func TestOpenBackfillsObjectOwnerID(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := dir + "/hush-hush.db"

	legacy, err := sql.Open("sqlite", path)
	require.NoError(t, err)

	_, err = legacy.Exec(`
		CREATE TABLE objects (
			id TEXT PRIMARY KEY,
			value BLOB NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);
		INSERT INTO objects (id, value, created_at, updated_at)
			VALUES ('legacy-object-1', X'00', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
		INSERT INTO objects (id, value, created_at, updated_at)
			VALUES ('legacy-object-2', X'00', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
	`)
	require.NoError(t, err)
	require.NoError(t, legacy.Close())

	s, err := store.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, s.Close()) })

	rows, err := s.DB().Query(`SELECT owner_id FROM objects ORDER BY id`)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, rows.Close()) })

	var count int
	for rows.Next() {
		var ownerID sql.NullString
		require.NoError(t, rows.Scan(&ownerID))
		require.True(t, ownerID.Valid)
		require.NotEmpty(t, ownerID.String)
		count++
	}
	require.NoError(t, rows.Err())
	require.Equal(t, 2, count)
}

// TestOpenMigratesExistingConsumersWithPublicKeyColumn proves
// addColumnIfMissing reaches a pre-existing consumers table that predates
// public_key, the same way TestOpenMigratesExistingTokensAndAuditLogWithNewColumns
// does for write_tokens/audit_log.
func TestOpenMigratesExistingConsumersWithPublicKeyColumn(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := dir + "/hush-hush.db"

	legacy, err := sql.Open("sqlite", path)
	require.NoError(t, err)

	_, err = legacy.Exec(`
		CREATE TABLE consumers (
			name TEXT PRIMARY KEY,
			created_at TEXT NOT NULL
		);
		INSERT INTO consumers (name, created_at) VALUES ('legacy-consumer', '2026-01-01T00:00:00Z');
	`)
	require.NoError(t, err)
	require.NoError(t, legacy.Close())

	s, err := store.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, s.Close()) })

	var publicKey sql.NullString
	require.NoError(t, s.DB().QueryRow(
		`SELECT public_key FROM consumers WHERE name = 'legacy-consumer'`,
	).Scan(&publicKey))
	require.False(t, publicKey.Valid)
}

func tableNames(t *testing.T, s *store.Store) []string {
	t.Helper()

	rows, err := s.DB().Query(`SELECT name FROM sqlite_master WHERE type = 'table'`)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, rows.Close()) })

	var names []string
	for rows.Next() {
		var name string
		require.NoError(t, rows.Scan(&name))
		names = append(names, name)
	}
	require.NoError(t, rows.Err())

	return names
}
