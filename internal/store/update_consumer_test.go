package store_test

import (
	"testing"

	"github.com/alrayyes/hush-hush/internal/store"
	"github.com/stretchr/testify/require"
)

const updateConsumerKey = "age1xl8amwv0vaxfk3gkrsgfv8l9nwkfuyqf448jwauhkn4vsw9jfqzq5290c0"

func TestUpdateConsumerRenamesAndSetsTheKeyTogether(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	require.NoError(t, s.CreateObject(t.Context(), "uc_target", []byte("v"), []string{"old-name"}, "", ""))

	entry, err := s.UpdateConsumer(t.Context(), "old-name", "new-name", updateConsumerKey)
	require.NoError(t, err)
	require.Equal(t, "new-name", entry.Name)
	require.Equal(t, 1, entry.SecretCount)
	require.Equal(t, updateConsumerKey, entry.PublicKey)

	obj, err := s.GetObject(t.Context(), "uc_target")
	require.NoError(t, err)
	require.Equal(t, []string{"new-name"}, obj.UsedBy)
}

func TestUpdateConsumerWithAFailingKeyWriteLeavesTheRenameUnapplied(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	require.NoError(t, s.CreateObject(t.Context(), "uc_atomic", []byte("v"), []string{"old-name"}, "", ""))

	// The key write is the second step; make it fail without touching the first.
	_, err := s.DB().ExecContext(t.Context(), `
		CREATE TRIGGER refuse_key BEFORE INSERT ON consumers
		WHEN NEW.public_key = '`+updateConsumerKey+`'
		BEGIN SELECT RAISE(ABORT, 'key write refused'); END`)
	require.NoError(t, err)

	_, err = s.UpdateConsumer(t.Context(), "old-name", "new-name", updateConsumerKey)
	require.Error(t, err)

	obj, err := s.GetObject(t.Context(), "uc_atomic")
	require.NoError(t, err)
	require.Equal(t, []string{"old-name"}, obj.UsedBy, "a failed key write must not leave the rename applied")
}

func TestUpdateConsumerWithOnlyANameOrOnlyAKeyBehavesAsEach(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	require.NoError(t, s.CreateObject(t.Context(), "uc_single", []byte("v"), []string{"first"}, "", ""))

	renamed, err := s.UpdateConsumer(t.Context(), "first", "second", "")
	require.NoError(t, err)
	require.Equal(t, "second", renamed.Name)
	require.Empty(t, renamed.PublicKey)

	keyed, err := s.UpdateConsumer(t.Context(), "second", "", updateConsumerKey)
	require.NoError(t, err)
	require.Equal(t, "second", keyed.Name)
	require.Equal(t, updateConsumerKey, keyed.PublicKey)
}

func TestUpdateConsumerOfAnUnknownNameThatIsRenamedIsRefused(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	_, err := s.UpdateConsumer(t.Context(), "ghost", "other", updateConsumerKey)
	require.ErrorIs(t, err, store.ErrUnknownConsumer)
}
