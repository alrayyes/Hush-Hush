// Command hush-hush is the composition root: it opens the SQLite store and
// starts the HTTP server, or (via its token subcommand) issues and revokes
// write-path tokens directly against that same store. See CLAUDE.md and
// openspec/changes/secrets-object-store/ for the design.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	hushhush "github.com/alrayyes/hush-hush/internal/api"
	"github.com/alrayyes/hush-hush/internal/cliconfig"
	"github.com/alrayyes/hush-hush/internal/store"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/term"
)

// version is stamped in at build time by goreleaser, from the tag. "dev" is
// what a plain `go build` reports, which is the honest answer for a binary
// built off an unknown tree.
var version = "dev"

// configEnvVars are every environment variable loadConfig binds - used
// only to decide whether the tool is already configured through the
// environment, not to read a value.
var configEnvVars = []string{"ADDR", "DB_PATH"}

// errConfigAlreadyExists is a sentinel rather than a plain fmt.Errorf: a
// fixed condition (a file is already there), not a message built from
// per-call detail - the path itself is per-call detail, so it's wrapped
// in rather than folded into the message.
var errConfigAlreadyExists = errors.New("config file already exists (use --force to overwrite)")

// config is this binary's runtime configuration, shared by serving and the
// token subcommands (both need db_path; only serving needs addr).
// rules/cli.md's flags > environment > config file > defaults - there are
// no flags yet, so this is env over the file at configFilePath().
type config struct {
	Addr   string `mapstructure:"addr"`
	DBPath string `mapstructure:"db_path"`
}

func configFilePath() (string, error) {
	path, err := cliconfig.Path("hush-hush")
	if err != nil {
		return "", fmt.Errorf("resolve hush-hush config path: %w", err)
	}

	return path, nil
}

func loadConfig() (config, error) {
	v := viper.New()
	v.SetDefault("addr", ":8080")
	v.SetDefault("db_path", "hush-hush.db")

	if path, err := configFilePath(); err == nil {
		v.SetConfigFile(path)
		v.SetConfigType("yaml")
		_ = v.ReadInConfig() // no config file yet is not an error
	}

	for key, env := range map[string]string{
		"addr":    "ADDR",
		"db_path": "DB_PATH",
	} {
		if err := v.BindEnv(key, env); err != nil {
			return config{}, fmt.Errorf("bind %s: %w", env, err)
		}
	}

	var c config
	if err := v.Unmarshal(&c); err != nil {
		return config{}, fmt.Errorf("unmarshal config: %w", err)
	}

	return c, nil
}

func main() {
	// os.Exit from inside main skips every deferred call registered before
	// it - run returns instead, so main is the only place that exits.
	os.Exit(run())
}

func run() int {
	// JSON on stdout, set before anything else logs - a container's log
	// collector reads structured lines, not slog's default text form.
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	if err := newRootCmd().Execute(); err != nil {
		return 1
	}

	return 0
}

// newRootCmd wires the server's default action (running it, exactly what
// a bare `hush-hush` has always done) alongside the token subcommand -
// existing deployments that just run the binary see no change.
func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "hush-hush",
		Short:         "hush-hush secrets object store server",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			if cmd.Name() == "init" {
				return nil
			}

			return maybeOfferInit(cmd)
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return serve()
		},
	}

	root.PersistentFlags().BoolP("yes", "y", false, "write a starter config with no prompt, if none exists")

	root.AddCommand(newInitCmd())
	root.AddCommand(newTokenCmd())

	return root
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

const starterConfig = `# hush-hush server config file. Environment variables override these -
# see README.md#configuration.
addr: ":8080"
db_path: "hush-hush.db"
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
// (creating the parent directory) that can fail on its own - a read-only
// container filesystem, deliberately how the published image runs, is
// exactly the case that already sets DB_PATH and must never be blocked
// by a nudge it was never going to act on anyway.
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
				"no config file and no ADDR/DB_PATH environment variables set - running on defaults (`hush-hush init` writes a starter config)\n",
			); err != nil {
				return fmt.Errorf("write config nudge: %w", err)
			}
		}

		return nil
	}

	if err := writeStarterConfig(cmd, path); err != nil {
		return err
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

func serve() error {
	cfg, err := loadConfig()
	if err != nil {
		slog.Error("config", "error", err)

		return err
	}

	s, err := store.Open(cfg.DBPath)
	if err != nil {
		slog.Error("open store", "error", err)

		return fmt.Errorf("open store: %w", err)
	}
	defer func() {
		if err := s.Close(); err != nil {
			slog.Error("close store", "error", err)
		}
	}()

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           hushhush.NewMux(s),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Error("shutdown", "error", err)
		}
	}()

	slog.Info("starting", "version", version, "addr", cfg.Addr, "db", cfg.DBPath)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("server stopped", "error", err)

		return fmt.Errorf("server stopped: %w", err)
	}

	return nil
}

// newTokenCmd groups write-path token management - issued and revoked by
// direct store access (openapi.yaml's bearerAuth description), never over
// HTTP: an admin endpoint for minting the very credential that
// authenticates admin endpoints would need its own bootstrap token to
// call it with.
func newTokenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "token",
		Short: "Issue, list, and revoke write-path tokens",
	}

	cmd.AddCommand(newTokenIssueCmd())
	cmd.AddCommand(newTokenListCmd())
	cmd.AddCommand(newTokenRevokeCmd())

	return cmd
}

func openStoreForTokenCmd() (*store.Store, error) {
	cfg, err := loadConfig()
	if err != nil {
		return nil, err
	}

	s, err := store.Open(cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}

	return s, nil
}

func newTokenIssueCmd() *cobra.Command {
	var (
		description string
		ttl         time.Duration
	)

	cmd := &cobra.Command{
		Use:   "issue",
		Short: "Issue a new write-path token",
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := openStoreForTokenCmd()
			if err != nil {
				return err
			}
			defer func() { _ = s.Close() }()

			id, token, err := s.CreateWriteToken(cmd.Context(), description, ttl)
			if err != nil {
				return fmt.Errorf("issue token: %w", err)
			}

			if _, err := fmt.Fprintf(cmd.OutOrStdout(),
				"id:    %s\ntoken: %s\n\nThe token is shown once - store it now, it can't be recovered later.\n",
				id, token,
			); err != nil {
				return fmt.Errorf("write issued token: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&description, "description", "", "what this token is for")
	cmd.Flags().DurationVar(&ttl, "ttl", 90*24*time.Hour, "how long the token stays valid")
	_ = cmd.MarkFlagRequired("description")

	return cmd
}

func newTokenListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List issued write-path tokens",
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := openStoreForTokenCmd()
			if err != nil {
				return err
			}
			defer func() { _ = s.Close() }()

			tokens, err := s.ListWriteTokens(cmd.Context())
			if err != nil {
				return fmt.Errorf("list tokens: %w", err)
			}

			for _, t := range tokens {
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\texpires %s\n", t.ID, t.Description, t.ExpiresAt); err != nil {
					return fmt.Errorf("write token list: %w", err)
				}
			}

			return nil
		},
	}
}

func newTokenRevokeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "revoke <id>",
		Short: "Revoke a write-path token, leaving every other token valid",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStoreForTokenCmd()
			if err != nil {
				return err
			}
			defer func() { _ = s.Close() }()

			if err := s.RevokeWriteToken(cmd.Context(), args[0]); err != nil {
				return fmt.Errorf("revoke token: %w", err)
			}

			return nil
		},
	}
}
