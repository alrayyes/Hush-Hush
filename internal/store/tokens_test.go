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

	_, token, err := s.CreateWriteToken(t.Context(), "homelab/vps-docker deploy", time.Hour, "")
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

	_, token, err := s.CreateWriteToken(t.Context(), "already expired", -time.Hour, "")
	require.NoError(t, err)

	valid, err := s.ValidateWriteToken(t.Context(), token)
	require.NoError(t, err)
	require.False(t, valid)
}

func TestValidateWriteTokenRejectsRevokedToken(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	wt, token, err := s.CreateWriteToken(t.Context(), "a", time.Hour, "")
	require.NoError(t, err)
	require.NoError(t, s.RevokeWriteToken(t.Context(), wt.ID))

	valid, err := s.ValidateWriteToken(t.Context(), token)
	require.NoError(t, err)
	require.False(t, valid)
}

func TestAuthenticateWriteTokenReturnsItsID(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	wt, token, err := s.CreateWriteToken(t.Context(), "a", time.Hour, "")
	require.NoError(t, err)

	id, valid, err := s.AuthenticateWriteToken(t.Context(), token)
	require.NoError(t, err)
	require.True(t, valid)
	require.Equal(t, wt.ID, id)
}

func TestAuthenticateWriteTokenRejectsUnknownToken(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	id, valid, err := s.AuthenticateWriteToken(t.Context(), "never-issued")
	require.NoError(t, err)
	require.False(t, valid)
	require.Empty(t, id)
}

func TestAuthenticateWriteTokenRejectsRevokedToken(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	wt, token, err := s.CreateWriteToken(t.Context(), "a", time.Hour, "")
	require.NoError(t, err)
	require.NoError(t, s.RevokeWriteToken(t.Context(), wt.ID))

	id, valid, err := s.AuthenticateWriteToken(t.Context(), token)
	require.NoError(t, err)
	require.False(t, valid)
	require.Empty(t, id)
}

func TestCreateWriteTokenReturnsUniqueIDsAndTokens(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	wt1, token1, err := s.CreateWriteToken(t.Context(), "a", time.Hour, "")
	require.NoError(t, err)
	wt2, token2, err := s.CreateWriteToken(t.Context(), "b", time.Hour, "")
	require.NoError(t, err)

	require.NotEqual(t, wt1.ID, wt2.ID)
	require.NotEqual(t, token1, token2)
}

func TestCreateWriteTokenRecordsItsOwner(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	wt, _, err := s.CreateWriteToken(t.Context(), "web UI token", time.Hour, "admin")
	require.NoError(t, err)
	require.Equal(t, "admin", wt.Owner)

	tokens, err := s.ListWriteTokens(t.Context())
	require.NoError(t, err)
	require.Len(t, tokens, 1)
	require.Equal(t, "admin", tokens[0].Owner)
}

func TestCreateWriteTokenWithNoOwnerListsWithNoOwner(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	_, _, err := s.CreateWriteToken(t.Context(), "cli token", time.Hour, "")
	require.NoError(t, err)

	tokens, err := s.ListWriteTokens(t.Context())
	require.NoError(t, err)
	require.Len(t, tokens, 1)
	require.Empty(t, tokens[0].Owner)
}

func TestListWriteTokensReturnsDescriptionsNotTokens(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	wt, _, err := s.CreateWriteToken(t.Context(), "homelab/vps-docker deploy", time.Hour, "")
	require.NoError(t, err)

	tokens, err := s.ListWriteTokens(t.Context())
	require.NoError(t, err)
	require.Len(t, tokens, 1)
	require.Equal(t, wt.ID, tokens[0].ID)
	require.Equal(t, "homelab/vps-docker deploy", tokens[0].Description)
	require.NotEmpty(t, tokens[0].ExpiresAt)
	require.False(t, tokens[0].Revoked)
}

func TestRevokeWriteTokenInvalidatesIt(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	wt, token, err := s.CreateWriteToken(t.Context(), "a", time.Hour, "")
	require.NoError(t, err)

	require.NoError(t, s.RevokeWriteToken(t.Context(), wt.ID))

	valid, err := s.ValidateWriteToken(t.Context(), token)
	require.NoError(t, err)
	require.False(t, valid)
}

func TestRevokeWriteTokenIsASoftDeleteThatStaysListed(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	wt, _, err := s.CreateWriteToken(t.Context(), "a", time.Hour, "admin")
	require.NoError(t, err)
	require.NoError(t, s.RevokeWriteToken(t.Context(), wt.ID))

	tokens, err := s.ListWriteTokens(t.Context())
	require.NoError(t, err)
	require.Len(t, tokens, 1)
	require.Equal(t, wt.ID, tokens[0].ID)
	require.Equal(t, "a", tokens[0].Description)
	require.Equal(t, "admin", tokens[0].Owner)
	require.True(t, tokens[0].Revoked)
}

func TestRevokeWriteTokenUnknownIDIsErrTokenNotFound(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	err := s.RevokeWriteToken(t.Context(), "nope")
	require.ErrorIs(t, err, store.ErrTokenNotFound)
}

func TestRevokeWriteTokenAlreadyRevokedIsErrTokenNotFound(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	wt, _, err := s.CreateWriteToken(t.Context(), "a", time.Hour, "")
	require.NoError(t, err)
	require.NoError(t, s.RevokeWriteToken(t.Context(), wt.ID))

	err = s.RevokeWriteToken(t.Context(), wt.ID)
	require.ErrorIs(t, err, store.ErrTokenNotFound)
}

func TestRevokingOneTokenLeavesOthersValid(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	revoked, _, err := s.CreateWriteToken(t.Context(), "revoked", time.Hour, "")
	require.NoError(t, err)
	_, survivingToken, err := s.CreateWriteToken(t.Context(), "surviving", time.Hour, "")
	require.NoError(t, err)

	require.NoError(t, s.RevokeWriteToken(t.Context(), revoked.ID))

	valid, err := s.ValidateWriteToken(t.Context(), survivingToken)
	require.NoError(t, err)
	require.True(t, valid)
}
