package store_test

import (
	"testing"
	"time"

	"github.com/alrayyes/hush-hush/internal/store"
	"github.com/stretchr/testify/require"
)

func TestCreateConsumerTokenThenAuthenticateSucceeds(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	_, token, err := s.CreateConsumerToken(t.Context(), "homelab", "deploy consumer", time.Hour)
	require.NoError(t, err)

	_, consumer, valid, err := s.AuthenticateConsumerToken(t.Context(), token)
	require.NoError(t, err)
	require.True(t, valid)
	require.Equal(t, "homelab", consumer)
}

func TestAuthenticateConsumerTokenReturnsItsID(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	ct, token, err := s.CreateConsumerToken(t.Context(), "homelab", "a", time.Hour)
	require.NoError(t, err)

	id, _, valid, err := s.AuthenticateConsumerToken(t.Context(), token)
	require.NoError(t, err)
	require.True(t, valid)
	require.Equal(t, ct.ID, id)
}

func TestAuthenticateConsumerTokenRejectsUnknownToken(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	id, consumer, valid, err := s.AuthenticateConsumerToken(t.Context(), "never-issued")
	require.NoError(t, err)
	require.False(t, valid)
	require.Empty(t, id)
	require.Empty(t, consumer)
}

func TestAuthenticateConsumerTokenRejectsExpiredToken(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	_, token, err := s.CreateConsumerToken(t.Context(), "homelab", "already expired", -time.Hour)
	require.NoError(t, err)

	_, _, valid, err := s.AuthenticateConsumerToken(t.Context(), token)
	require.NoError(t, err)
	require.False(t, valid)
}

func TestAuthenticateConsumerTokenRejectsRevokedToken(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	ct, token, err := s.CreateConsumerToken(t.Context(), "homelab", "a", time.Hour)
	require.NoError(t, err)
	require.NoError(t, s.RevokeConsumerToken(t.Context(), ct.ID))

	_, _, valid, err := s.AuthenticateConsumerToken(t.Context(), token)
	require.NoError(t, err)
	require.False(t, valid)
}

func TestCreateConsumerTokenReturnsUniqueIDsAndTokens(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	ct1, token1, err := s.CreateConsumerToken(t.Context(), "homelab", "a", time.Hour)
	require.NoError(t, err)
	ct2, token2, err := s.CreateConsumerToken(t.Context(), "homelab", "b", time.Hour)
	require.NoError(t, err)

	require.NotEqual(t, ct1.ID, ct2.ID)
	require.NotEqual(t, token1, token2)
}

func TestListConsumerTokensReturnsConsumerAndDescriptionNotToken(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	ct, _, err := s.CreateConsumerToken(t.Context(), "homelab", "deploy consumer", time.Hour)
	require.NoError(t, err)

	tokens, err := s.ListConsumerTokens(t.Context())
	require.NoError(t, err)
	require.Len(t, tokens, 1)
	require.Equal(t, ct.ID, tokens[0].ID)
	require.Equal(t, "homelab", tokens[0].Consumer)
	require.Equal(t, "deploy consumer", tokens[0].Description)
	require.NotEmpty(t, tokens[0].ExpiresAt)
	require.False(t, tokens[0].Revoked)
	require.Empty(t, tokens[0].LastUsedAt)
}

func TestRevokeConsumerTokenInvalidatesIt(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	ct, token, err := s.CreateConsumerToken(t.Context(), "homelab", "a", time.Hour)
	require.NoError(t, err)

	require.NoError(t, s.RevokeConsumerToken(t.Context(), ct.ID))

	_, _, valid, err := s.AuthenticateConsumerToken(t.Context(), token)
	require.NoError(t, err)
	require.False(t, valid)
}

func TestRevokeConsumerTokenIsASoftDeleteThatStaysListed(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	ct, _, err := s.CreateConsumerToken(t.Context(), "homelab", "a", time.Hour)
	require.NoError(t, err)
	require.NoError(t, s.RevokeConsumerToken(t.Context(), ct.ID))

	tokens, err := s.ListConsumerTokens(t.Context())
	require.NoError(t, err)
	require.Len(t, tokens, 1)
	require.Equal(t, ct.ID, tokens[0].ID)
	require.True(t, tokens[0].Revoked)
}

func TestRevokeConsumerTokenUnknownIDIsErrTokenNotFound(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	err := s.RevokeConsumerToken(t.Context(), "nope")
	require.ErrorIs(t, err, store.ErrTokenNotFound)
}

func TestRevokeConsumerTokenAlreadyRevokedIsErrTokenNotFound(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	ct, _, err := s.CreateConsumerToken(t.Context(), "homelab", "a", time.Hour)
	require.NoError(t, err)
	require.NoError(t, s.RevokeConsumerToken(t.Context(), ct.ID))

	err = s.RevokeConsumerToken(t.Context(), ct.ID)
	require.ErrorIs(t, err, store.ErrTokenNotFound)
}

func TestUpdateConsumerTokenUsageRecordsLastUsedAt(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	ct, _, err := s.CreateConsumerToken(t.Context(), "homelab", "a", time.Hour)
	require.NoError(t, err)

	usedAt := time.Now().UTC().Format(time.RFC3339)
	require.NoError(t, s.UpdateConsumerTokenUsage(t.Context(), ct.ID, usedAt))

	tokens, err := s.ListConsumerTokens(t.Context())
	require.NoError(t, err)
	require.Len(t, tokens, 1)
	require.Equal(t, usedAt, tokens[0].LastUsedAt)
}

func TestUpdateConsumerTokenUsageUnknownIDIsErrTokenNotFound(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	err := s.UpdateConsumerTokenUsage(t.Context(), "nope", time.Now().UTC().Format(time.RFC3339))
	require.ErrorIs(t, err, store.ErrTokenNotFound)
}

func TestRotateConsumerTokenInvalidatesTheOldSecret(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	ct, oldToken, err := s.CreateConsumerToken(t.Context(), "homelab", "a", time.Hour)
	require.NoError(t, err)

	_, _, err = s.RotateConsumerToken(t.Context(), ct.ID, time.Hour)
	require.NoError(t, err)

	_, _, valid, err := s.AuthenticateConsumerToken(t.Context(), oldToken)
	require.NoError(t, err)
	require.False(t, valid)
}

func TestRotateConsumerTokenIssuesANewWorkingSecret(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	ct, _, err := s.CreateConsumerToken(t.Context(), "homelab", "a", time.Hour)
	require.NoError(t, err)

	_, newToken, err := s.RotateConsumerToken(t.Context(), ct.ID, time.Hour)
	require.NoError(t, err)

	_, consumer, valid, err := s.AuthenticateConsumerToken(t.Context(), newToken)
	require.NoError(t, err)
	require.True(t, valid)
	require.Equal(t, "homelab", consumer)
}

func TestRotateConsumerTokenKeepsIDConsumerAndDescription(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	ct, _, err := s.CreateConsumerToken(t.Context(), "homelab", "deploy consumer", time.Hour)
	require.NoError(t, err)

	rotated, _, err := s.RotateConsumerToken(t.Context(), ct.ID, time.Hour)
	require.NoError(t, err)

	require.Equal(t, ct.ID, rotated.ID)
	require.Equal(t, "homelab", rotated.Consumer)
	require.Equal(t, "deploy consumer", rotated.Description)
}

func TestRotateConsumerTokenClearsLastUsedAt(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	ct, _, err := s.CreateConsumerToken(t.Context(), "homelab", "a", time.Hour)
	require.NoError(t, err)
	require.NoError(t, s.UpdateConsumerTokenUsage(t.Context(), ct.ID, time.Now().UTC().Format(time.RFC3339)))

	rotated, _, err := s.RotateConsumerToken(t.Context(), ct.ID, time.Hour)
	require.NoError(t, err)

	require.Empty(t, rotated.LastUsedAt)
}

func TestRotateConsumerTokenUnknownIDIsErrTokenNotFound(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	_, _, err := s.RotateConsumerToken(t.Context(), "nope", time.Hour)
	require.ErrorIs(t, err, store.ErrTokenNotFound)
}

func TestRotateConsumerTokenRevokedIsErrTokenNotFound(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	ct, _, err := s.CreateConsumerToken(t.Context(), "homelab", "a", time.Hour)
	require.NoError(t, err)
	require.NoError(t, s.RevokeConsumerToken(t.Context(), ct.ID))

	_, _, err = s.RotateConsumerToken(t.Context(), ct.ID, time.Hour)
	require.ErrorIs(t, err, store.ErrTokenNotFound)
}

func TestRotateConsumerTokenExpiredIsErrTokenNotFound(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	ct, _, err := s.CreateConsumerToken(t.Context(), "homelab", "already expired", -time.Hour)
	require.NoError(t, err)

	_, _, err = s.RotateConsumerToken(t.Context(), ct.ID, time.Hour)
	require.ErrorIs(t, err, store.ErrTokenNotFound)
}

func TestRevokingOneConsumerTokenLeavesOthersValid(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	revoked, _, err := s.CreateConsumerToken(t.Context(), "homelab", "revoked", time.Hour)
	require.NoError(t, err)
	_, survivingToken, err := s.CreateConsumerToken(t.Context(), "homelab", "surviving", time.Hour)
	require.NoError(t, err)

	require.NoError(t, s.RevokeConsumerToken(t.Context(), revoked.ID))

	_, _, valid, err := s.AuthenticateConsumerToken(t.Context(), survivingToken)
	require.NoError(t, err)
	require.True(t, valid)
}

func TestConsumerTokenAndWriteTokenHashesDoNotCollideAcrossKinds(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	_, writeToken, err := s.CreateWriteToken(t.Context(), "a", time.Hour, "")
	require.NoError(t, err)

	_, _, valid, err := s.AuthenticateConsumerToken(t.Context(), writeToken)
	require.NoError(t, err)
	require.False(t, valid)
}

func TestPurgeConsumerTokenRemovesARevokedToken(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	ct, _, err := s.CreateConsumerToken(t.Context(), "homelab", "a", time.Hour)
	require.NoError(t, err)
	require.NoError(t, s.RevokeConsumerToken(t.Context(), ct.ID))

	require.NoError(t, s.PurgeConsumerToken(t.Context(), ct.ID))

	tokens, err := s.ListConsumerTokens(t.Context())
	require.NoError(t, err)
	require.Empty(t, tokens)
}

func TestPurgeConsumerTokenRemovesAnExpiredNeverRevokedToken(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	ct, _, err := s.CreateConsumerToken(t.Context(), "homelab", "already expired", -time.Hour)
	require.NoError(t, err)

	require.NoError(t, s.PurgeConsumerToken(t.Context(), ct.ID))

	tokens, err := s.ListConsumerTokens(t.Context())
	require.NoError(t, err)
	require.Empty(t, tokens)
}

func TestPurgeConsumerTokenRejectsAnActiveToken(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	ct, _, err := s.CreateConsumerToken(t.Context(), "homelab", "a", time.Hour)
	require.NoError(t, err)

	err = s.PurgeConsumerToken(t.Context(), ct.ID)
	require.ErrorIs(t, err, store.ErrTokenStillActive)

	tokens, err := s.ListConsumerTokens(t.Context())
	require.NoError(t, err)
	require.Len(t, tokens, 1)
}

func TestPurgeConsumerTokenUnknownIDIsErrTokenNotFound(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	err := s.PurgeConsumerToken(t.Context(), "nope")
	require.ErrorIs(t, err, store.ErrTokenNotFound)
}
