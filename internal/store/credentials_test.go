package store_test

import (
	"database/sql"
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

// TestCreateCredentialPersistsBackupEligible covers alrayyes/hush-hush#260:
// go-webauthn's own ValidateLogin rejects a login outright when the
// credential it's handed has a different BackupEligible flag than the
// live assertion reports - a synced/multi-device passkey (the kind every
// major browser creates by default) sets this true, so a credential
// stored with it silently dropped fails every login with "Backup Eligible
// flag inconsistency detected during login validation".
func TestCreateCredentialPersistsBackupEligible(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	now := time.Now().UTC().Format(time.RFC3339)

	require.NoError(t, s.CreateCredential(t.Context(), store.Credential{
		ID: "cred-1", PublicKey: []byte("pubkey"), CreatedAt: now, BackupEligible: true,
	}))

	creds, err := s.ListCredentials(t.Context())
	require.NoError(t, err)
	require.Len(t, creds, 1)
	require.True(t, creds[0].BackupEligible)
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

func TestRenameCredentialUpdatesNickname(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	require.NoError(t, s.CreateCredential(t.Context(), store.Credential{
		ID: "cred-1", PublicKey: []byte("pubkey"), Nickname: "old", CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}))

	require.NoError(t, s.RenameCredential(t.Context(), "cred-1", "new"))

	creds, err := s.ListCredentials(t.Context())
	require.NoError(t, err)
	require.Equal(t, "new", creds[0].Nickname)
}

func TestRenameCredentialUnknownIDIsErrCredentialNotFound(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	err := s.RenameCredential(t.Context(), "nope", "new")
	require.ErrorIs(t, err, store.ErrCredentialNotFound)
}

func TestDeleteCredentialRemovesIt(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	require.NoError(t, s.CreateCredential(t.Context(), store.Credential{
		ID: "cred-1", PublicKey: []byte("pubkey"), CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}))

	require.NoError(t, s.DeleteCredential(t.Context(), "cred-1"))

	creds, err := s.ListCredentials(t.Context())
	require.NoError(t, err)
	require.Empty(t, creds)
}

func TestDeleteCredentialUnknownIDIsErrCredentialNotFound(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	err := s.DeleteCredential(t.Context(), "nope")
	require.ErrorIs(t, err, store.ErrCredentialNotFound)
}

// TestCreateCredentialPersistsWrappedIdentityOnlyWhenGiven covers
// tasks.md group 3.3: a PRF-capable credential's own wrapped copy of the
// escrowed identity round-trips through ListCredentials, and a
// non-PRF credential (no wrapped_identity given) comes back with none -
// specs/users/spec.md's "Per-credential wrapping of the escrowed
// identity" and "PRF support is detected at registration" requirements.
func TestCreateCredentialPersistsWrappedIdentityOnlyWhenGiven(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	now := time.Now().UTC().Format(time.RFC3339)

	require.NoError(t, s.CreateCredential(t.Context(), store.Credential{
		ID: "prf-capable", PublicKey: []byte("pubkey"), CreatedAt: now,
		WrappedIdentity: "wrapped-copy-1",
	}))
	require.NoError(t, s.CreateCredential(t.Context(), store.Credential{
		ID: "non-prf", PublicKey: []byte("pubkey"), CreatedAt: now,
	}))

	creds, err := s.ListCredentials(t.Context())
	require.NoError(t, err)
	require.Len(t, creds, 2)

	byID := map[string]store.Credential{}
	for _, c := range creds {
		byID[c.ID] = c
	}

	require.Equal(t, "wrapped-copy-1", byID["prf-capable"].WrappedIdentity)
	require.Empty(t, byID["non-prf"].WrappedIdentity)
}

// TestCreateCredentialPersistsUserID covers the same registration setting
// webauthn_credentials.user_id on a new row - store.go's migrateColumns
// comment notes this is left to tasks.md group 3, not the schema
// migration itself.
func TestCreateCredentialPersistsUserID(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	userID, err := s.CurrentUserID(t.Context())
	require.NoError(t, err)

	require.NoError(t, s.CreateCredential(t.Context(), store.Credential{
		ID: "cred-1", PublicKey: []byte("pubkey"), CreatedAt: time.Now().UTC().Format(time.RFC3339),
		UserID: userID,
	}))

	var storedUserID sql.NullString
	require.NoError(t, s.DB().QueryRow(
		`SELECT user_id FROM webauthn_credentials WHERE id = 'cred-1'`,
	).Scan(&storedUserID))
	require.True(t, storedUserID.Valid)
	require.Equal(t, userID, storedUserID.String)
}

// TestDeletingOneCredentialLeavesAnotherCredentialsWrappedIdentityIntact
// covers tasks.md group 3.4 and specs/users/spec.md's "Deleting one of
// several passkeys does not strand the identity" scenario: deleting one
// PRF-capable credential never touches another's own wrapped copy, and
// requires no re-encryption of anything to stay recoverable through the
// survivor.
func TestDeletingOneCredentialLeavesAnotherCredentialsWrappedIdentityIntact(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	now := time.Now().UTC().Format(time.RFC3339)

	require.NoError(t, s.CreateCredential(t.Context(), store.Credential{
		ID: "first", PublicKey: []byte("pubkey"), CreatedAt: now, WrappedIdentity: "wrapped-copy-1",
	}))
	require.NoError(t, s.CreateCredential(t.Context(), store.Credential{
		ID: "second", PublicKey: []byte("pubkey"), CreatedAt: now, WrappedIdentity: "wrapped-copy-2",
	}))

	require.NoError(t, s.DeleteCredential(t.Context(), "first"))

	remaining, err := s.ListCredentials(t.Context())
	require.NoError(t, err)
	require.Len(t, remaining, 1)
	require.Equal(t, "second", remaining[0].ID)
	require.Equal(t, "wrapped-copy-2", remaining[0].WrappedIdentity)
}

// TestSetUserEscrowIsIdempotentAgainstALaterRegistration covers
// specs/users/spec.md's "Escrowed identity generated once" scenario at
// the store layer: a second call (as a later registration's request
// might replay if a client bug resent public_key) never overwrites the
// identity already recorded, since every existing credential's wrapped
// copy was wrapped against the original one.
func TestSetUserEscrowIsIdempotentAgainstALaterRegistration(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	userID, err := s.CurrentUserID(t.Context())
	require.NoError(t, err)

	require.NoError(t, s.SetUserEscrow(t.Context(), userID, "age1original", "recovery-wrapped-original"))
	require.NoError(t, s.SetUserEscrow(t.Context(), userID, "age1replacement", "recovery-wrapped-replacement"))

	var publicKey, recoveryWrapped string
	require.NoError(t, s.DB().QueryRow(
		`SELECT public_key, recovery_wrapped_identity FROM users WHERE id = ?`, userID,
	).Scan(&publicKey, &recoveryWrapped))
	require.Equal(t, "age1original", publicKey)
	require.Equal(t, "recovery-wrapped-original", recoveryWrapped)
}
