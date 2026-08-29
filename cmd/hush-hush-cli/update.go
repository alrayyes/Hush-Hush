package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/alrayyes/hush-hush/internal/cli"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// newUpdateCmd reads the new plaintext value from stdin, same reasoning as
// newInjectCmd: a flag value ends up in shell history and process listings.
func newUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Seal a new value (read from stdin) and replace an object's stored value",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// See newInjectCmd: rebound here, not at construction, since
			// inject and update share the --recipients/HUSH_HUSH_RECIPIENTS
			// name and only one command's flag object can hold the viper
			// key at a time.
			_ = viper.BindPFlag("recipients", cmd.Flags().Lookup("recipients"))

			recipients := viper.GetString("recipients")
			if recipients == "" {
				return errNoRecipients
			}

			value, err := io.ReadAll(cmd.InOrStdin())
			if err != nil {
				return fmt.Errorf("read value from stdin: %w", err)
			}

			return cli.Update(cmd.Context(), config(), args[0], value, strings.Split(recipients, ","))
		},
	}

	cmd.Flags().String("recipients", "", "comma-separated age recipient public keys")

	return cmd
}
