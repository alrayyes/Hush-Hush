package store_test

import (
	"testing"
	"time"

	"github.com/alrayyes/hush-hush/internal/store"
	"github.com/stretchr/testify/require"
)

func TestListCredentialsOnFreshStoreIsEmpty(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	creds, err := s.ListCredentials(t.Context())
	require.NoError(t, err)
	require.Empty(t, creds)
}

func TestCreateCredentialThenListReturnsIt(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	now := time.Now().UTC().Format(time.RFC3339)

	require.NoError(t, s.CreateCredential(t.Context(), store.Credential{
		ID: "cred-1", PublicKey: []byte("pubkey"), AAGUID: "aaguid", Nickname: "YubiKey 5C", CreatedAt: now,
	}))

	creds, err := s.ListCredentials(t.Context())
	require.NoError(t, err)
	require.Len(t, creds, 1)
	require.Equal(t, "cred-1", creds[0].ID)
	require.Equal(t, "YubiKey 5C", creds[0].Nickname)
	require.Empty(t, creds[0].LastUsedAt)
}

func TestUpdateCredentialUsageUpdatesSignCountAndLastUsedAt(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	now := time.Now().UTC().Format(time.RFC3339)

	require.NoError(t, s.CreateCredential(t.Context(), store.Credential{
		ID: "cred-1", PublicKey: []byte("pubkey"), CreatedAt: now,
	}))

	usedAt := time.Now().UTC().Add(time.Minute).Format(time.RFC3339)
	require.NoError(t, s.UpdateCredentialUsage(t.Context(), "cred-1", 5, usedAt))

	creds, err := s.ListCredentials(t.Context())
	require.NoError(t, err)
	require.Len(t, creds, 1)
	require.EqualValues(t, 5, creds[0].SignCount)
	require.Equal(t, usedAt, creds[0].LastUsedAt)
}

func TestUpdateCredentialUsageUnknownIDIsErrCredentialNotFound(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	err := s.UpdateCredentialUsage(t.Context(), "nope", 1, time.Now().UTC().Format(time.RFC3339))
	require.ErrorIs(t, err, store.ErrCredentialNotFound)
}
