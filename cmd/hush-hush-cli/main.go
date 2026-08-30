// Command hush-hush-cli is the client every hush-hush consumer speaks
// through - the writer's only interface to the service, and the same
// binary any consumer (CI job, deploy script) runs to fetch and decrypt a
// value locally. See CLAUDE.md and openspec/changes/secrets-object-store/
// for the design.
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/alrayyes/hush-hush/internal/cli"
	"github.com/alrayyes/hush-hush/internal/cliconfig"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/term"
)

// version is stamped in at build time by goreleaser, from the tag.
var version = "dev"

// errConfigAlreadyExists is a sentinel rather than a plain fmt.Errorf: a
// fixed condition (a file is already there), not a message built from
// per-call detail - the path itself is per-call detail, so it's wrapped
// in rather than folded into the message.
var errConfigAlreadyExists = errors.New("config file already exists (use --force to overwrite)")

// configEnvVars are every HUSH_HUSH_* variable a command reads - the
// persistent flags below plus recipients/identity, which are bound
// per-subcommand rather than on root. Used only to decide whether the
// tool is already configured through the environment, not to read a
// value.
var configEnvVars = []string{
	"HUSH_HUSH_SERVER", "HUSH_HUSH_TOKEN", "HUSH_HUSH_CALLER",
	"HUSH_HUSH_RECIPIENTS", "HUSH_HUSH_IDENTITY",
}

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
// inside CI" requirement). A config file at configPath() sits below both:
// rules/cli.md's flags > environment > config file > defaults.
func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "hush-hush-cli",
		Short:         "Client for the hush-hush secrets object store",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			if cmd.Name() == "init" {
				return nil
			}

			return maybeOfferInit(cmd)
		},
	}

	root.PersistentFlags().String("server", "http://localhost:8080", "hush-hush server URL")
	root.PersistentFlags().String("token", "", "write-path bearer token")
	root.PersistentFlags().String("caller", "", "self-presented identity recorded in the audit log")
	root.PersistentFlags().BoolP("yes", "y", false, "write a starter config with no prompt, if none exists")

	for _, name := range []string{"server", "token", "caller"} {
		_ = viper.BindPFlag(name, root.PersistentFlags().Lookup(name))
	}

	viper.SetEnvPrefix("hush_hush")
	viper.AutomaticEnv()

	if path, err := configFilePath(); err == nil {
		viper.SetConfigFile(path)
		viper.SetConfigType("yaml")
		_ = viper.ReadInConfig() // no config file yet is not an error
	}

	root.AddCommand(newInitCmd())
	root.AddCommand(newInjectCmd())
	root.AddCommand(newGetCmd())
	root.AddCommand(newUpdateCmd())
	root.AddCommand(newDeleteCmd())

	return root
}

func config() cli.Config {
	return cli.Config{
		Server: viper.GetString("server"),
		Token:  viper.GetString("token"),
		Caller: viper.GetString("caller"),
	}
}

func configFilePath() (string, error) {
	path, err := cliconfig.Path("hush-hush-cli")
	if err != nil {
		return "", fmt.Errorf("resolve hush-hush-cli config path: %w", err)
	}

	return path, nil
}

// newInitCmd writes a starter config file populated with the same
// defaults the tool would otherwise fall back to, ready to edit
// (rules/cli.md).
func newInitCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Write a starter config file",
		RunE: func(cmd *cobra.Command, _ []string) error {
			path, err := configFilePath()
			if err != nil {
				return err
			}

			if cliconfig.Exists(path) && !force {
				return fmt.Errorf("%s: %w", path, errConfigAlreadyExists)
			}

			return writeStarterConfig(cmd, path)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing config file")

	return cmd
}

const starterConfig = `# hush-hush-cli config file. Flags and HUSH_HUSH_* environment variables
# both override these - see README.md#configuration.
server: http://localhost:8080
token: ""
caller: ""
recipients: ""
identity: ""
`

func writeStarterConfig(cmd *cobra.Command, path string) error {
	if err := os.WriteFile(path, []byte(starterConfig), 0o600); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}

	if _, err := fmt.Fprintf(cmd.OutOrStdout(), "wrote %s\n", path); err != nil {
		return fmt.Errorf("write init confirmation: %w", err)
	}

	return nil
}

// maybeOfferInit is rules/cli.md's "a run with no config file and no
// relevant environment variable set offers to run init right there":
// skipped entirely once a config file exists or the environment already
// configures the tool, and never blocks a non-interactive run (no TTY)
// on a prompt nothing will ever answer.
//
// anyEnvSet is checked before ever resolving a path: ShouldWriteStarter
// always skips once it's true, and resolving one has a real side effect
// (creating the parent directory) that can fail on its own - a CI job
// that already sets every HUSH_HUSH_* variable, exactly the case
// cli.md's own "runs unmodified inside CI" requirement targets, must
// never be blocked by a nudge it was never going to act on anyway.
func maybeOfferInit(cmd *cobra.Command) error {
	anyEnvSet := anyConfigEnvVarSet()
	if anyEnvSet {
		return nil
	}

	path, err := configFilePath()
	if err != nil {
		// Advisory only: a run this environment doesn't already
		// configure still has to work even where the config path
		// itself can't be resolved or created.
		return nil //nolint:nilerr // advisory only, error already explained above
	}

	exists := cliconfig.Exists(path)
	yes, _ := cmd.Flags().GetBool("yes")
	interactive := term.IsTerminal(int(os.Stdin.Fd()))

	confirmed := false
	if !yes && interactive && !exists && !anyEnvSet {
		confirmed = cliconfig.Confirm(cmd.InOrStdin(), cmd.OutOrStdout(),
			"No config file found. Write a starter one at "+path+" now?")
	}

	if !cliconfig.ShouldWriteStarter(exists, anyEnvSet, yes, interactive, confirmed) {
		if !exists && !anyEnvSet && !interactive {
			if _, err := fmt.Fprintf(cmd.ErrOrStderr(),
				"no config file and no HUSH_HUSH_* environment variables set - running on defaults (`hush-hush-cli init` writes a starter config)\n",
			); err != nil {
				return fmt.Errorf("write config nudge: %w", err)
			}
		}

		return nil
	}

	if err := writeStarterConfig(cmd, path); err != nil {
		return err
	}

	if err := viper.ReadInConfig(); err != nil {
		return fmt.Errorf("read newly written config: %w", err)
	}

	return nil
}

func anyConfigEnvVarSet() bool {
	for _, name := range configEnvVars {
		if _, ok := os.LookupEnv(name); ok {
			return true
		}
	}

	return false
}
