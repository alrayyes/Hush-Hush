// Package seal wraps age encryption for the writer's side of hush-hush:
// value in, ciphertext out, no server or CLI process ever holding a
// private key (openspec/changes/secrets-object-store/design.md).
package seal

import (
	"bytes"
	"fmt"

	"filippo.io/age"
)

// Seal encrypts value to every one of recipientKeys independently -
// multi-recipient sealing, so a single object can serve several
// consumers, each decrypting with their own private key, rather than one
// shared key distributed to all of them.
func Seal(value []byte, recipientKeys []string) ([]byte, error) {
	recipients := make([]age.Recipient, len(recipientKeys))

	for i, key := range recipientKeys {
		r, err := age.ParseX25519Recipient(key)
		if err != nil {
			return nil, fmt.Errorf("parse recipient %d: %w", i, err)
		}

		recipients[i] = r
	}

	var buf bytes.Buffer

	w, err := age.Encrypt(&buf, recipients...)
	if err != nil {
		return nil, fmt.Errorf("start encryption: %w", err)
	}

	if _, err := w.Write(value); err != nil {
		return nil, fmt.Errorf("write plaintext: %w", err)
	}

	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("finish encryption: %w", err)
	}

	return buf.Bytes(), nil
}
