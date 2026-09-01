package cli

import (
	"context"
	"fmt"

	"github.com/alrayyes/hush-hush/internal/seal"
)

// Inject seals value to recipients and creates a new object under id - the
// writer's process never handles a private key, only recipients' public
// keys (design.md). description is fixed at creation, the same as usedBy.
func Inject(ctx context.Context, cfg Config, id string, value []byte, recipients []string, usedBy []string, description string) error {
	sealed, err := seal.Seal(value, recipients)
	if err != nil {
		return fmt.Errorf("seal value: %w", err)
	}

	cl, err := cfg.newClient()
	if err != nil {
		return err
	}

	if _, err := cl.Create(ctx, id, sealed, usedBy, description); err != nil {
		return fmt.Errorf("create object: %w", err)
	}

	return nil
}
