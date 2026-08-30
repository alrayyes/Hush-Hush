package cli

import (
	"context"
	"fmt"

	"github.com/alrayyes/hush-hush/internal/seal"
)

// Get fetches id's ciphertext and decrypts it locally with whichever of
// identities matches - the CLI never writes an assembled file or applies
// consumer-side file-shape logic (design.md).
func Get(ctx context.Context, cfg Config, id string, identities []string) ([]byte, error) {
	cl, err := cfg.newClient()
	if err != nil {
		return nil, err
	}

	sealed, err := cl.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("fetch object: %w", err)
	}

	plaintext, err := seal.Unseal(sealed, identities)
	if err != nil {
		return nil, fmt.Errorf("decrypt object: %w", err)
	}

	return plaintext, nil
}
