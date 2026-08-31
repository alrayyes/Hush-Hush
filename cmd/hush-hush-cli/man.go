package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

// newManCmd generates one man page per command into dir, from root's own
// already-defined tree - the same Use/Short/Long/flags --help already
// reads from, so the page can't drift out of sync with the command it
// documents. Hidden: this is a build-time step
// (.goreleaser.yml's before.hooks runs `go run ./cmd/hush-hush-cli man`),
// not something an end user reaches for.
//
// root is captured by reference, not value: newManCmd runs before
// newRootCmd's own AddCommand calls finish, but RunE only runs later, by
// which point root's full command tree exists.
func newManCmd(root *cobra.Command) *cobra.Command {
	return &cobra.Command{
		Use:    "man <dir>",
		Short:  "Generate man pages into dir",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			// GenManTree writes into dir directly, no mkdir of its own.
			if err := os.MkdirAll(args[0], 0o750); err != nil {
				return fmt.Errorf("create %s: %w", args[0], err)
			}

			header := &doc.GenManHeader{
				Title:   "HUSH-HUSH-CLI",
				Section: "1",
			}

			if err := doc.GenManTree(root, header, args[0]); err != nil {
				return fmt.Errorf("generate man pages: %w", err)
			}

			return nil
		},
	}
}
