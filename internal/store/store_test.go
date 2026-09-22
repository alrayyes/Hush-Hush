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
