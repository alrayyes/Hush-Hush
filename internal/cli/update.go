package cli

import (
	"context"
	"fmt"

	"github.com/alrayyes/hush-hush/internal/seal"
)

// Update seals value to recipients and replaces id's stored value, leaving
// its used_by metadata unchanged (design.md) - the writer's process never
// handles a private key here either, same as Inject.
func Update(ctx context.Context, cfg Config, id string, value []byte, recipients []string) error {
	sealed, err := seal.Seal(value, recipients)
	if err != nil {
		return fmt.Errorf("seal value: %w", err)
	}

	if _, err := cfg.newClient().Update(ctx, id, sealed); err != nil {
		return fmt.Errorf("update object: %w", err)
	}

	return nil
}
