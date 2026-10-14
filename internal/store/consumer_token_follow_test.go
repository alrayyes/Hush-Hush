package store_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// A consumer's read tokens go with it when it is deleted and follow it when
// it is renamed, so a token never outlives or loses the consumer it was
// minted for.

func TestDeletingAConsumerRevokesItsTokens(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	require.NoError(t, s.AddConsumer(t.Context(), "homelab"))

	_, token, err := s.CreateConsumerToken(t.Context(), "homelab", "deploy", time.Hour)
	require.NoError(t, err)

	require.NoError(t, s.DeleteConsumer(t.Context(), "homelab"))

	_, _, valid, err := s.AuthenticateConsumerToken(t.Context(), token)
	require.NoError(t, err)
	require.False(t, valid, "the token must not outlive the consumer")
}

func TestDeletingAConsumerLeavesOtherConsumersTokensAlone(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	require.NoError(t, s.AddConsumer(t.Context(), "gone"))
	require.NoError(t, s.AddConsumer(t.Context(), "stays"))

	_, token, err := s.CreateConsumerToken(t.Context(), "stays", "deploy", time.Hour)
	require.NoError(t, err)

	require.NoError(t, s.DeleteConsumer(t.Context(), "gone"))

	_, _, valid, err := s.AuthenticateConsumerToken(t.Context(), token)
	require.NoError(t, err)
	require.True(t, valid)
}

func TestRenamingAConsumerMovesItsTokensToTheNewName(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	require.NoError(t, s.AddConsumer(t.Context(), "old-name"))

	_, token, err := s.CreateConsumerToken(t.Context(), "old-name", "deploy", time.Hour)
	require.NoError(t, err)

	_, err = s.RenameConsumer(t.Context(), "old-name", "new-name")
	require.NoError(t, err)

	_, consumer, valid, err := s.AuthenticateConsumerToken(t.Context(), token)
	require.NoError(t, err)
	require.True(t, valid)
	require.Equal(t, "new-name", consumer)
}

func TestRenamingIntoAnExistingConsumerLeavesBothNamesTokensReadingAsTheMergedName(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	require.NoError(t, s.AddConsumer(t.Context(), "from"))
	require.NoError(t, s.AddConsumer(t.Context(), "into"))

	_, fromToken, err := s.CreateConsumerToken(t.Context(), "from", "a", time.Hour)
	require.NoError(t, err)

	_, intoToken, err := s.CreateConsumerToken(t.Context(), "into", "b", time.Hour)
	require.NoError(t, err)

	_, err = s.RenameConsumer(t.Context(), "from", "into")
	require.NoError(t, err)

	for _, token := range []string{fromToken, intoToken} {
		_, consumer, valid, err := s.AuthenticateConsumerToken(t.Context(), token)
		require.NoError(t, err)
		require.True(t, valid)
		require.Equal(t, "into", consumer)
	}
}

func TestRenamingAConsumerToItsOwnNameKeepsItsTokens(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	require.NoError(t, s.AddConsumer(t.Context(), "same"))

	_, token, err := s.CreateConsumerToken(t.Context(), "same", "deploy", time.Hour)
	require.NoError(t, err)

	_, err = s.RenameConsumer(t.Context(), "same", "same")
	require.NoError(t, err)

	_, consumer, valid, err := s.AuthenticateConsumerToken(t.Context(), token)
	require.NoError(t, err)
	require.True(t, valid)
	require.Equal(t, "same", consumer)
}

func TestAConsumerReAddedAfterADeleteDoesNotRevivePreviousTokens(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	require.NoError(t, s.AddConsumer(t.Context(), "homelab"))

	_, token, err := s.CreateConsumerToken(t.Context(), "homelab", "deploy", time.Hour)
	require.NoError(t, err)

	require.NoError(t, s.DeleteConsumer(t.Context(), "homelab"))
	require.NoError(t, s.AddConsumer(t.Context(), "homelab"))

	_, _, valid, err := s.AuthenticateConsumerToken(t.Context(), token)
	require.NoError(t, err)
	require.False(t, valid, "re-adding the name must not bring the old token back")
}

func TestAConsumerCreatedUnderARenamedAwayNameInheritsNoTokens(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	require.NoError(t, s.AddConsumer(t.Context(), "old-name"))

	_, token, err := s.CreateConsumerToken(t.Context(), "old-name", "deploy", time.Hour)
	require.NoError(t, err)

	_, err = s.RenameConsumer(t.Context(), "old-name", "new-name")
	require.NoError(t, err)
	require.NoError(t, s.AddConsumer(t.Context(), "old-name"))

	_, consumer, valid, err := s.AuthenticateConsumerToken(t.Context(), token)
	require.NoError(t, err)
	require.True(t, valid)
	require.Equal(t, "new-name", consumer, "the token stays with the renamed consumer")
}
