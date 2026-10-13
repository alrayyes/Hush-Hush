package store_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/alrayyes/hush-hush/internal/store"
	"github.com/stretchr/testify/require"
)

// insertVariant writes an object and its used_by rows with raw SQL, the way
// a path that skips the Go checks would.
func insertVariant(t *testing.T, s *store.Store, id, slug string, consumers ...string) error {
	t.Helper()

	db := s.DB()
	if _, err := db.Exec(`INSERT INTO objects (id, slug, value, created_at, updated_at) VALUES (?, ?, x'00', 'now', 'now')`, id, slug); err != nil {
		return fmt.Errorf("insert object: %w", err)
	}

	for _, c := range consumers {
		if _, err := db.Exec(`INSERT INTO used_by (object_id, consumer) VALUES (?, ?)`, id, c); err != nil {
			return fmt.Errorf("insert used_by: %w", err)
		}
	}

	return nil
}

func TestTheSchemaRefusesAConsumerInTwoVariantsOfOneSlug(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	require.NoError(t, insertVariant(t, s, "id-1", "db", "a"))
	require.NoError(t, insertVariant(t, s, "id-2", "db", "b"))

	_, err := s.DB().Exec(`INSERT INTO used_by (object_id, consumer) VALUES ('id-2', 'a')`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "UNIQUE")
}

func TestTheSameConsumerMayHoldDifferentSlugs(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	require.NoError(t, insertVariant(t, s, "id-1", "db", "a"))
	require.NoError(t, insertVariant(t, s, "id-2", "cache", "a"))
}

func TestRenamingASlugKeepsTheConstraintHolding(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	ctx := context.Background()

	require.NoError(t, s.CreateObject(ctx, "db", []byte("x"), []string{"a"}, "", ""))
	require.NoError(t, s.CreateObject(ctx, "db", []byte("x"), []string{"b"}, "", ""))

	newSlug := "database"
	_, _, err := s.PatchObject(ctx, "db", store.ObjectPatch{Slug: &newSlug}, store.ForConsumer("a"))
	require.NoError(t, err)

	// "a" is now in a variant of "database"; a third object under the new
	// name may not take it again.
	require.ErrorIs(t, s.CreateObject(ctx, "database", []byte("x"), []string{"a"}, "", ""), store.ErrAlreadyExists)

	_, err = s.DB().Exec(`INSERT INTO objects (id, slug, value, created_at, updated_at) VALUES ('raw', 'database', x'00', 'now', 'now')`)
	require.NoError(t, err)
	_, err = s.DB().Exec(`INSERT INTO used_by (object_id, consumer) VALUES ('raw', 'a')`)
	require.Error(t, err)
}

func TestOpenReportsExistingDuplicatesInsteadOfPickingOne(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "h.db")

	s, err := store.Open(path)
	require.NoError(t, err)
	// Recreate the pre-fix shape: no guard, and a duplicate already in.
	for _, stmt := range []string{
		`DROP TRIGGER IF EXISTS used_by_fill_slug`,
		`DROP TRIGGER IF EXISTS objects_sync_used_by_slug`,
		`DROP INDEX IF EXISTS used_by_slug_consumer`,
	} {
		_, err = s.DB().Exec(stmt)
		require.NoError(t, err)
	}
	require.NoError(t, insertVariant(t, s, "id-1", "db", "a"))
	require.NoError(t, insertVariant(t, s, "id-2", "db", "a"))
	require.NoError(t, s.Close())

	_, err = store.Open(path)
	require.Error(t, err)
	require.Contains(t, err.Error(), `"db"`)
	require.Contains(t, err.Error(), `"a"`)
}

func TestOpenBackfillsSlugOnExistingUsedByRows(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "h.db")

	s, err := store.Open(path)
	require.NoError(t, err)
	require.NoError(t, insertVariant(t, s, "id-1", "db", "a"))
	_, err = s.DB().Exec(`UPDATE used_by SET slug = NULL`)
	require.NoError(t, err)
	require.NoError(t, s.Close())

	s, err = store.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, s.Close()) })

	var slug string
	require.NoError(t, s.DB().QueryRow(`SELECT slug FROM used_by WHERE consumer = 'a'`).Scan(&slug))
	require.Equal(t, "db", slug)
}
