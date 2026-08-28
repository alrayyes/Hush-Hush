package store_test

import (
	"testing"

	"github.com/alrayyes/hush-hush/internal/store"
	"github.com/stretchr/testify/require"
)

func TestOpenAppliesSchemaToFreshDatabase(t *testing.T) {
	s, err := store.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, s.Close()) })

	tables := tableNames(t, s)
	require.Contains(t, tables, "objects")
	require.Contains(t, tables, "used_by")
	require.Contains(t, tables, "audit_log")
}

func TestOpenIsIdempotent(t *testing.T) {
	// A fresh database, then Open again against the same file, must not
	// error - the schema is applied with CREATE TABLE IF NOT EXISTS, not a
	// one-shot migration that fails the second time it runs.
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
