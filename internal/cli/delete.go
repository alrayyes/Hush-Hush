package cli

import (
	"context"
	"fmt"
)

// Delete permanently removes id - there's nothing to seal or unseal here,
// unlike every other command.
func Delete(ctx context.Context, cfg Config, id string) error {
	if err := cfg.newClient().Delete(ctx, id); err != nil {
		return fmt.Errorf("delete object: %w", err)
	}

	return nil
}
