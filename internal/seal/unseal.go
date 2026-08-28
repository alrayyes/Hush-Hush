package seal

import (
	"bytes"
	"fmt"
	"io"

	"filippo.io/age"
)

// Unseal decrypts sealed with whichever of identityKeys matches one of
// the recipients it was sealed to. age.Decrypt itself fails outright when
// none match, rather than producing corrupted output - Unseal only adds
// context to that failure, never papering over it.
func Unseal(sealed []byte, identityKeys []string) ([]byte, error) {
	identities := make([]age.Identity, len(identityKeys))

	for i, key := range identityKeys {
		id, err := age.ParseX25519Identity(key)
		if err != nil {
			return nil, fmt.Errorf("parse identity %d: %w", i, err)
		}

		identities[i] = id
	}

	r, err := age.Decrypt(bytes.NewReader(sealed), identities...)
	if err != nil {
		return nil, fmt.Errorf("decrypt value: %w", err)
	}

	plaintext, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read decrypted value: %w", err)
	}

	return plaintext, nil
}
