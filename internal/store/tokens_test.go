package store_test

import (
	"testing"
	"time"

	"github.com/alrayyes/hush-hush/internal/store"
	"github.com/stretchr/testify/require"
)

func TestCreateWriteTokenThenValidateWriteTokenSucceeds(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	_, token, err := s.CreateWriteToken(t.Context(), "homelab/vps-docker deploy", time.Hour)
	require.NoError(t, err)

	valid, err := s.ValidateWriteToken(t.Context(), token)
	require.NoError(t, err)
	require.True(t, valid)
}

func TestValidateWriteTokenRejectsUnknownToken(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	valid, err := s.ValidateWriteToken(t.Context(), "never-issued")
	require.NoError(t, err)
	require.False(t, valid)
}

func TestValidateWriteTokenRejectsExpiredToken(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	_, token, err := s.CreateWriteToken(t.Context(), "already expired", -time.Hour)
	require.NoError(t, err)

	valid, err := s.ValidateWriteToken(t.Context(), token)
	require.NoError(t, err)
	require.False(t, valid)
}

func TestCreateWriteTokenReturnsUniqueIDsAndTokens(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	id1, token1, err := s.CreateWriteToken(t.Context(), "a", time.Hour)
	require.NoError(t, err)
	id2, token2, err := s.CreateWriteToken(t.Context(), "b", time.Hour)
	require.NoError(t, err)

	require.NotEqual(t, id1, id2)
	require.NotEqual(t, token1, token2)
}

func TestListWriteTokensReturnsDescriptionsNotTokens(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	id, _, err := s.CreateWriteToken(t.Context(), "homelab/vps-docker deploy", time.Hour)
	require.NoError(t, err)

	tokens, err := s.ListWriteTokens(t.Context())
	require.NoError(t, err)
	require.Len(t, tokens, 1)
	require.Equal(t, id, tokens[0].ID)
	require.Equal(t, "homelab/vps-docker deploy", tokens[0].Description)
	require.NotEmpty(t, tokens[0].ExpiresAt)
}

func TestRevokeWriteTokenInvalidatesIt(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	id, token, err := s.CreateWriteToken(t.Context(), "a", time.Hour)
	require.NoError(t, err)

	require.NoError(t, s.RevokeWriteToken(t.Context(), id))

	valid, err := s.ValidateWriteToken(t.Context(), token)
	require.NoError(t, err)
	require.False(t, valid)
}

func TestRevokeWriteTokenUnknownIDIsErrTokenNotFound(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	err := s.RevokeWriteToken(t.Context(), "nope")
	require.ErrorIs(t, err, store.ErrTokenNotFound)
}

func TestRevokingOneTokenLeavesOthersValid(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	revokedID, _, err := s.CreateWriteToken(t.Context(), "revoked", time.Hour)
	require.NoError(t, err)
	_, survivingToken, err := s.CreateWriteToken(t.Context(), "surviving", time.Hour)
	require.NoError(t, err)

	require.NoError(t, s.RevokeWriteToken(t.Context(), revokedID))

	valid, err := s.ValidateWriteToken(t.Context(), survivingToken)
	require.NoError(t, err)
	require.True(t, valid)
}
