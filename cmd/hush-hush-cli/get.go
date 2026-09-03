package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/alrayyes/hush-hush/internal/cli"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// errNoIdentity is a sentinel: a fixed condition (no decrypting key was
// configured at all), not a message built from per-call detail.
var errNoIdentity = errors.New("no identity configured (--identity or HUSH_HUSH_IDENTITY)")

// newGetCmd writes the decrypted value to stdout, and nothing else - no
// assembled file, no consumer-side file-shape logic (the cli spec's
// "get returns exactly one decrypted value to stdout per call").
func newGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Fetch and decrypt one value to stdout",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			identities := viper.GetString("identity")
			if identities == "" {
				return errNoIdentity
			}

			cfg, err := config()
			if err != nil {
				return err
			}

			plaintext, err := cli.Get(cmd.Context(), cfg, args[0], strings.Split(identities, ","))
			if err != nil {
				return fmt.Errorf("get: %w", err)
			}

			_, err = cmd.OutOrStdout().Write(plaintext)
			if err != nil {
				return fmt.Errorf("write value to stdout: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().String("identity", "", "comma-separated age private keys")
	_ = viper.BindPFlag("identity", cmd.Flags().Lookup("identity"))

	return cmd
}
