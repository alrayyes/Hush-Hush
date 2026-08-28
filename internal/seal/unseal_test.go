package seal_test

import (
	"testing"

	"filippo.io/age"
	"github.com/alrayyes/hush-hush/internal/seal"
	"github.com/stretchr/testify/require"
)

func TestUnsealRecoversTheOriginalValue(t *testing.T) {
	t.Parallel()

	identity, err := age.GenerateX25519Identity()
	require.NoError(t, err)

	value := []byte("sealed-ciphertext-source")

	sealed, err := seal.Seal(value, []string{identity.Recipient().String()})
	require.NoError(t, err)

	plaintext, err := seal.Unseal(sealed, []string{identity.String()})
	require.NoError(t, err)
	require.Equal(t, value, plaintext)
}

func TestUnsealWithOneOfSeveralMatchingIdentities(t *testing.T) {
	t.Parallel()

	unrelated, err := age.GenerateX25519Identity()
	require.NoError(t, err)

	matching, err := age.GenerateX25519Identity()
	require.NoError(t, err)

	value := []byte("v")

	sealed, err := seal.Seal(value, []string{matching.Recipient().String()})
	require.NoError(t, err)

	plaintext, err := seal.Unseal(sealed, []string{unrelated.String(), matching.String()})
	require.NoError(t, err)
	require.Equal(t, value, plaintext)
}

func TestUnsealWithNoMatchingIdentityFailsClearly(t *testing.T) {
	t.Parallel()

	sealedTo, err := age.GenerateX25519Identity()
	require.NoError(t, err)

	wrongIdentity, err := age.GenerateX25519Identity()
	require.NoError(t, err)

	sealed, err := seal.Seal([]byte("v"), []string{sealedTo.Recipient().String()})
	require.NoError(t, err)

	_, err = seal.Unseal(sealed, []string{wrongIdentity.String()})
	require.Error(t, err)
}

func TestUnsealRejectsAMalformedIdentity(t *testing.T) {
	t.Parallel()

	_, err := seal.Unseal([]byte("irrelevant"), []string{"not-an-identity"})
	require.Error(t, err)
}
