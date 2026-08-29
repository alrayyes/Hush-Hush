package main

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/alrayyes/hush-hush/internal/cli"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// errNoRecipients is a sentinel rather than a plain fmt.Errorf: it's a
// fixed condition (recipients weren't configured at all), not a message
// built from per-call detail.
var errNoRecipients = errors.New("no recipients configured (--recipients or HUSH_HUSH_RECIPIENTS)")

// newInjectCmd reads the plaintext value from stdin rather than a flag -
// a flag value ends up in shell history and process listings, exactly
// what injecting a secret should avoid.
func newInjectCmd() *cobra.Command {
	var usedBy []string

	cmd := &cobra.Command{
		Use:   "inject <id>",
		Short: "Seal a value (read from stdin) and create a new object",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Rebound here rather than at construction: inject and update
			// share the --recipients/HUSH_HUSH_RECIPIENTS name, and binding
			// both flag objects to the one viper key at construction time
			// leaves viper pointing at whichever command was registered
			// last on root, silently ignoring the other's flag.
			_ = viper.BindPFlag("recipients", cmd.Flags().Lookup("recipients"))

			recipients := viper.GetString("recipients")
			if recipients == "" {
				return errNoRecipients
			}

			value, err := io.ReadAll(cmd.InOrStdin())
			if err != nil {
				return fmt.Errorf("read value from stdin: %w", err)
			}

			return cli.Inject(cmd.Context(), config(), args[0], value, strings.Split(recipients, ","), usedBy)
		},
	}

	cmd.Flags().StringSliceVar(&usedBy, "used-by", nil, "consumers of this secret (repeatable, or comma-separated)")
	cmd.Flags().String("recipients", "", "comma-separated age recipient public keys")

	return cmd
}
