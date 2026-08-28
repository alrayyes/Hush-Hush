package seal_test

import (
	"bytes"
	"io"
	"testing"

	"filippo.io/age"
	"github.com/alrayyes/hush-hush/internal/seal"
	"github.com/stretchr/testify/require"
)

func TestSealDecryptsWithTheMatchingIdentity(t *testing.T) {
	t.Parallel()

	identity, err := age.GenerateX25519Identity()
	require.NoError(t, err)

	value := []byte("sealed-ciphertext-source")

	sealed, err := seal.Seal(value, []string{identity.Recipient().String()})
	require.NoError(t, err)
	require.NotEqual(t, value, sealed)

	r, err := age.Decrypt(bytes.NewReader(sealed), identity)
	require.NoError(t, err)

	plaintext, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, value, plaintext)
}

func TestSealToMultipleRecipientsEachDecryptsIndependently(t *testing.T) {
	t.Parallel()

	first, err := age.GenerateX25519Identity()
	require.NoError(t, err)

	second, err := age.GenerateX25519Identity()
	require.NoError(t, err)

	value := []byte("shared-secret")

	sealed, err := seal.Seal(value, []string{first.Recipient().String(), second.Recipient().String()})
	require.NoError(t, err)

	for _, identity := range []*age.X25519Identity{first, second} {
		r, err := age.Decrypt(bytes.NewReader(sealed), identity)
		require.NoError(t, err)

		plaintext, err := io.ReadAll(r)
		require.NoError(t, err)
		require.Equal(t, value, plaintext)
	}
}

func TestSealRejectsAMalformedRecipient(t *testing.T) {
	t.Parallel()

	_, err := seal.Seal([]byte("v"), []string{"not-a-recipient"})
	require.Error(t, err)
}
