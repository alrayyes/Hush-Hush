package store_test

import (
	"database/sql"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/alrayyes/hush-hush/internal/store"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

// One slug can hold a different value for different consumers
// (alrayyes/hush-hush#668). Each variant is its own object with its own
// used_by list, and a consumer is in at most one variant of a slug.

func seedReleaseToken(t *testing.T, s *store.Store) {
	t.Helper()

	ctx := t.Context()
	require.NoError(t, s.CreateObject(ctx, "release_token", []byte("tokena"), []string{"a", "b", "c"}, "", ""))
	require.NoError(t, s.CreateObject(ctx, "release_token", []byte("tokenb"), []string{"d"}, "", ""))
}

func TestOneSlugHoldsADifferentValuePerConsumer(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	seedReleaseToken(t, s)

	for _, consumer := range []string{"a", "b", "c"} {
		obj, err := s.GetObject(t.Context(), "release_token", store.ForConsumer(consumer))
		require.NoError(t, err, consumer)
		require.Equal(t, []byte("tokena"), obj.Value, consumer)
	}

	obj, err := s.GetObject(t.Context(), "release_token", store.ForConsumer("d"))
	require.NoError(t, err)
	require.Equal(t, []byte("tokenb"), obj.Value)
	require.Equal(t, []string{"d"}, obj.UsedBy)
}

func TestAConsumerWithNoVariantFindsNothing(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	seedReleaseToken(t, s)

	_, err := s.GetObject(t.Context(), "release_token", store.ForConsumer("e"))
	require.ErrorIs(t, err, store.ErrNotFound)
}

func TestASlugWithSeveralVariantsIsAmbiguousWithoutAConsumer(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	seedReleaseToken(t, s)
	ctx := t.Context()

	_, err := s.GetObject(ctx, "release_token")
	require.ErrorIs(t, err, store.ErrAmbiguousSlug)

	require.ErrorIs(t, s.UpdateObject(ctx, "release_token", []byte("x"), nil), store.ErrAmbiguousSlug)
	require.ErrorIs(t, s.DeleteObject(ctx, "release_token"), store.ErrAmbiguousSlug)
}

func TestASlugWithOneVariantStillWorksWithoutAConsumer(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	require.NoError(t, s.CreateObject(t.Context(), "only", []byte("v"), []string{"a"}, "", ""))

	obj, err := s.GetObject(t.Context(), "only")
	require.NoError(t, err)
	require.Equal(t, []byte("v"), obj.Value)
}

func TestCreatingAVariantForAConsumerThatHasOneIsRefused(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	seedReleaseToken(t, s)
	ctx := t.Context()

	require.ErrorIs(t, s.CreateObject(ctx, "release_token", []byte("x"), []string{"e", "a"}, "", ""), store.ErrAlreadyExists)
	require.ErrorIs(t, s.CreateObject(ctx, "release_token", []byte("x"), nil, "", ""), store.ErrAlreadyExists,
		"a repeat of the slug with no consumers has no variant to be")
	require.NoError(t, s.CreateObject(ctx, "release_token", []byte("tokenc"), []string{"e"}, "", ""))
}

func TestUpdatingAVariantLeavesTheOthersAlone(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	seedReleaseToken(t, s)
	ctx := t.Context()

	require.NoError(t, s.UpdateObject(ctx, "release_token", []byte("tokenb2"), nil, store.ForConsumer("d")))

	d, err := s.GetObject(ctx, "release_token", store.ForConsumer("d"))
	require.NoError(t, err)
	require.Equal(t, []byte("tokenb2"), d.Value)

	a, err := s.GetObject(ctx, "release_token", store.ForConsumer("a"))
	require.NoError(t, err)
	require.Equal(t, []byte("tokena"), a.Value)
}

func TestAnUpdateThatPutsAConsumerInTwoVariantsIsRefusedAndChangesNothing(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	seedReleaseToken(t, s)
	ctx := t.Context()

	moved := []string{"d", "a"}
	err := s.UpdateObject(ctx, "release_token", []byte("changed"), &moved, store.ForConsumer("d"))
	require.ErrorIs(t, err, store.ErrVariantConflict)

	d, err := s.GetObject(ctx, "release_token", store.ForConsumer("d"))
	require.NoError(t, err)
	require.Equal(t, []byte("tokenb"), d.Value, "the value didn't change either")
	require.Equal(t, []string{"d"}, d.UsedBy)
}

func TestDeletingAVariantLeavesTheOthersAlone(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	seedReleaseToken(t, s)
	ctx := t.Context()

	require.NoError(t, s.DeleteObject(ctx, "release_token", store.ForConsumer("d")))

	_, err := s.GetObject(ctx, "release_token", store.ForConsumer("d"))
	require.ErrorIs(t, err, store.ErrNotFound)

	a, err := s.GetObject(ctx, "release_token")
	require.NoError(t, err, "one variant left, so no consumer is needed again")
	require.Equal(t, []byte("tokena"), a.Value)
}

func TestARenameThatPutsAConsumerInTwoVariantsIsRefused(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	seedReleaseToken(t, s)
	ctx := t.Context()

	_, err := s.RenameConsumer(ctx, "a", "d")
	require.ErrorIs(t, err, store.ErrVariantConflict)

	a, err := s.GetObject(ctx, "release_token", store.ForConsumer("a"))
	require.NoError(t, err, "a is still where it was")
	require.Equal(t, []byte("tokena"), a.Value)

	_, err = s.RenameConsumer(ctx, "a", "z")
	require.NoError(t, err, "a rename that doesn't collide still works")
}

func TestADatabaseWithTheOldUniqueSlugIndexAcceptsVariantsAfterOpen(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "old.db")

	s, err := store.Open(path)
	require.NoError(t, err)
	require.NoError(t, s.CreateObject(t.Context(), "release_token", []byte("tokena"), []string{"a"}, "", ""))
	require.NoError(t, s.Close())

	// Put the schema back to what a database from before variants has.
	db, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	_, err = db.Exec(`DROP INDEX IF EXISTS idx_objects_slug_lookup`)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_objects_slug ON objects (slug)`)
	require.NoError(t, err)
	require.NoError(t, db.Close())

	s, err = store.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, s.Close()) })

	require.NoError(t, s.CreateObject(t.Context(), "release_token", []byte("tokenb"), []string{"d"}, "", ""))

	a, err := s.GetObject(t.Context(), "release_token", store.ForConsumer("a"))
	require.NoError(t, err)
	require.Equal(t, []byte("tokena"), a.Value, "what was there before is untouched")
}

var uuidV4 = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestEveryObjectHasAUUIDID(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	seedReleaseToken(t, s)

	a, err := s.GetObject(t.Context(), "release_token", store.ForConsumer("a"))
	require.NoError(t, err)
	d, err := s.GetObject(t.Context(), "release_token", store.ForConsumer("d"))
	require.NoError(t, err)

	require.Regexp(t, uuidV4, a.ID)
	require.Regexp(t, uuidV4, d.ID)
	require.NotEqual(t, a.ID, d.ID)
}

func TestAVariantCanBeAddressedByItsID(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	seedReleaseToken(t, s)
	ctx := t.Context()

	d, err := s.GetObject(ctx, "release_token", store.ForConsumer("d"))
	require.NoError(t, err)

	got, err := s.GetObject(ctx, "release_token", store.WithID(d.ID))
	require.NoError(t, err, "an id settles what the bare slug can't")
	require.Equal(t, []byte("tokenb"), got.Value)

	require.NoError(t, s.UpdateObject(ctx, "release_token", []byte("tokenb2"), nil, store.WithID(d.ID)))
	got, err = s.GetObject(ctx, "release_token", store.WithID(d.ID))
	require.NoError(t, err)
	require.Equal(t, []byte("tokenb2"), got.Value)

	require.NoError(t, s.DeleteObject(ctx, "release_token", store.WithID(d.ID)))
	_, err = s.GetObject(ctx, "release_token", store.WithID(d.ID))
	require.ErrorIs(t, err, store.ErrNotFound)
}

func TestAnIDThatBelongsToAnotherSlugIsNotFound(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	seedReleaseToken(t, s)
	ctx := t.Context()
	require.NoError(t, s.CreateObject(ctx, "other", []byte("x"), []string{"z"}, "", ""))

	other, err := s.GetObject(ctx, "other")
	require.NoError(t, err)

	_, err = s.GetObject(ctx, "release_token", store.WithID(other.ID))
	require.ErrorIs(t, err, store.ErrNotFound)
}

func TestHexIDsFromAnOlderDatabaseBecomeUUIDsAndKeepTheirRows(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "old.db")

	s, err := store.Open(path)
	require.NoError(t, err)
	require.NoError(t, s.CreateObject(t.Context(), "legacy", []byte("v"), []string{"a", "b"}, "", "", store.WithTags([]string{"prod"})))
	require.NoError(t, s.Close())

	// Give the object the kind of id a database from before this had.
	db, err := sql.Open("sqlite", path)
	require.NoError(t, err)

	const hexID = "0123456789abcdef0123456789abcdef"

	tx, err := db.Begin()
	require.NoError(t, err)
	_, err = tx.Exec(`PRAGMA defer_foreign_keys = ON`)
	require.NoError(t, err)
	_, err = tx.Exec(`UPDATE used_by SET object_id = ?`, hexID)
	require.NoError(t, err)
	_, err = tx.Exec(`UPDATE tags SET object_id = ?`, hexID)
	require.NoError(t, err)
	_, err = tx.Exec(`UPDATE objects SET id = ?`, hexID)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())
	require.NoError(t, db.Close())

	s, err = store.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, s.Close()) })

	obj, err := s.GetObject(t.Context(), "legacy")
	require.NoError(t, err)
	require.Regexp(t, uuidV4, obj.ID)
	require.Equal(t, []string{"a", "b"}, obj.UsedBy)
	require.Equal(t, []string{"prod"}, obj.Tags)
	require.Equal(t, []byte("v"), obj.Value)
}
