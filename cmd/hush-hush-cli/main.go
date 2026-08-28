// Command hush-hush-cli is the client every hush-hush consumer speaks
// through - the writer's only interface to the service, and the same
// binary any consumer (CI job, deploy script) runs to fetch and decrypt a
// value locally. See CLAUDE.md and openspec/changes/secrets-object-store/
// for the design.
package main

import (
	"fmt"
	"os"

	"github.com/alrayyes/hush-hush/internal/cli"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// version is stamped in at build time by goreleaser, from the tag.
var version = "dev"

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// newRootCmd wires the persistent, env-overridable connection config
// (server URL, bearer token, caller identity) shared by every subcommand.
// HUSH_HUSH_SERVER, HUSH_HUSH_TOKEN, and HUSH_HUSH_CALLER override their
// matching flags - a CI job supplies these through its own secret storage,
// with no bespoke wrapper or Action (the cli spec's "runs unmodified
// inside CI" requirement).
func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "hush-hush-cli",
		Short:         "Client for the hush-hush secrets object store",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.PersistentFlags().String("server", "http://localhost:8080", "hush-hush server URL")
	root.PersistentFlags().String("token", "", "write-path bearer token")
	root.PersistentFlags().String("caller", "", "self-presented identity recorded in the audit log")

	for _, name := range []string{"server", "token", "caller"} {
		_ = viper.BindPFlag(name, root.PersistentFlags().Lookup(name))
	}

	viper.SetEnvPrefix("hush_hush")
	viper.AutomaticEnv()

	root.AddCommand(newInjectCmd())

	return root
}

func config() cli.Config {
	return cli.Config{
		Server: viper.GetString("server"),
		Token:  viper.GetString("token"),
		Caller: viper.GetString("caller"),
	}
}
