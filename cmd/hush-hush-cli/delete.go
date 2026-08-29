package main

import (
	"fmt"

	"github.com/alrayyes/hush-hush/internal/cli"
	"github.com/spf13/cobra"
)

func newDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Permanently remove an object",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cli.Delete(cmd.Context(), config(), args[0]); err != nil {
				return fmt.Errorf("delete: %w", err)
			}

			return nil
		},
	}
}
