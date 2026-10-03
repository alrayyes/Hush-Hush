package api_test

import (
	"bytes"
	"errors"
	"testing"

	"filippo.io/age"
	hushhush "github.com/alrayyes/hush-hush/internal/api"
	"github.com/stretchr/testify/require"
)

// validateAgeCiphertext parses whatever bytes a client uploads, so it gets
// fuzzed (rules/go-test.md, "Fuzzing"). The seeds below run as ordinary tests
// on every `go test`; the actual fuzzing is a separate pass:
//
//	go test ./internal/api -run=^$ -fuzz=FuzzValidateAgeCiphertext -fuzztime=60s
//
// Any input it finds that breaks a property is written under
// testdata/fuzz/ and should be committed: from then on a plain `go test`
// replays it.

// A value is accepted, or refused for one of the two reasons the API names.
// It never panics and never fails some third way.
func FuzzValidateAgeCiphertext(f *testing.F) {
	f.Add(sealedFixture)
	f.Add(noRecipientAgeFile())
	f.Add([]byte{})
	f.Add([]byte("this is plaintext, not an age file"))
	f.Add([]byte("age-encryption.org/v1\n"))
	f.Add([]byte("age-encryption.org/v1\n-> X25519 AAAA\nAAAA\n--- "))
	f.Add(sealedFixture[:len(sealedFixture)/2]) // a truncated real file
	f.Add(append([]byte("age-encryption.org/v1\n"), bytes.Repeat([]byte("-> X25519 A\n"), 64)...))

	f.Fuzz(func(t *testing.T, data []byte) {
		err := hushhush.ValidateAgeCiphertext(data)
		require.True(t,
			err == nil || errors.Is(err, hushhush.ErrNotAnAgeFile) || errors.Is(err, hushhush.ErrNoRecipients),
			"unexpected error: %v", err)
	})
}

// The converse: anything age itself produces for a recipient is accepted, whatever
// it holds. Fuzzing the plaintext catches a size or content the check mishandles.
func FuzzValidateAcceptsWhatAgeProduces(f *testing.F) {
	identity, err := age.GenerateX25519Identity()
	require.NoError(f, err)

	f.Add([]byte{})
	f.Add([]byte("hello"))
	f.Add(bytes.Repeat([]byte("x"), 70000)) // more than one age chunk

	f.Fuzz(func(t *testing.T, plaintext []byte) {
		var sealed bytes.Buffer
		w, err := age.Encrypt(&sealed, identity.Recipient())
		require.NoError(t, err)
		_, err = w.Write(plaintext)
		require.NoError(t, err)
		require.NoError(t, w.Close())

		require.NoError(t, hushhush.ValidateAgeCiphertext(sealed.Bytes()))
	})
}
