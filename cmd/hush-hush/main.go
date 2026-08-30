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
	"github.com/alrayyes/hush-hush/internal/store"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// version is stamped in at build time by goreleaser, from the tag. "dev" is
// what a plain `go build` reports, which is the honest answer for a binary
// built off an unknown tree.
var version = "dev"

// config is this binary's runtime configuration, shared by serving and the
// token subcommands (both need db_path; only serving needs addr). There's
// no config file or CLI flag surface yet - it's all environment variables
// - but going through viper keeps loading in one place if that changes.
type config struct {
	Addr   string `mapstructure:"addr"`
	DBPath string `mapstructure:"db_path"`
}

func loadConfig() (config, error) {
	v := viper.New()
	v.SetDefault("addr", ":8080")
	v.SetDefault("db_path", "hush-hush.db")

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
		RunE: func(_ *cobra.Command, _ []string) error {
			return serve()
		},
	}

	root.AddCommand(newTokenCmd())

	return root
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
