package cli

import (
	"context"
	"fmt"

	"github.com/alrayyes/hush-hush/internal/seal"
)

// Inject seals value to recipients and creates a new object under id - the
// writer's process never handles a private key, only recipients' public
// keys (design.md).
func Inject(ctx context.Context, cfg Config, id string, value []byte, recipients []string, usedBy []string) error {
	sealed, err := seal.Seal(value, recipients)
	if err != nil {
		return fmt.Errorf("seal value: %w", err)
	}

	if _, err := cfg.newClient().Create(ctx, id, sealed, usedBy); err != nil {
		return fmt.Errorf("create object: %w", err)
	}

	return nil
}
