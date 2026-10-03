// Command hush-hush is the composition root: it opens the SQLite store and
// starts the HTTP server, or (via its token subcommand) issues and revokes
// write-path tokens directly against that same store. See CLAUDE.md and
// openspec/changes/secrets-object-store/ for the design.
package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"unicode"
	"unicode/utf8"

	hushhush "github.com/alrayyes/hush-hush/internal/api"
	"github.com/alrayyes/hush-hush/internal/store"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// version is stamped in at build time by goreleaser, from the tag. "dev" is
// what a plain `go build` reports, which is the honest answer for a binary
// built off an unknown tree.
var version = "dev"

// Validate's own sentinels - fixed conditions, not per-call detail.
var (
	errInstanceLabelTooLong = fmt.Errorf("instance_label: INSTANCE_LABEL must be at most %d characters", maxInstanceLabelLength)
	errInstanceLabelControl = errors.New("instance_label: INSTANCE_LABEL must not contain control characters")
	errAddrRequired         = errors.New("addr: required")
	errDBPathRequired       = errors.New("db_path: required")
	errHealthzStatus        = errors.New("healthz check failed")
)

// config is this binary's runtime configuration, shared by serving and the
// token subcommands (both need db_path; only serving needs addr).
// Environment variables only, no config file, no init - cli.md's
// backend-service carve-out: hush-hush is a deployed service with no
// interactive user to persist a preference for (hush-hush#141), unlike
// hush-hush-cli.
type config struct {
	Addr   string `mapstructure:"addr"`
	DBPath string `mapstructure:"db_path"`
	// PublicURL is the address a browser actually reaches this server at -
	// unrelated to Addr, which is only the bind address. Optional: empty
	// means the web UI's WebAuthn endpoints are unusable (each answers its
	// own configuration error) but everything else runs unchanged
	// (openspec/changes/web-ui/design.md's Migration Plan).
	PublicURL string `mapstructure:"public_url"`
	// InstanceLabel is an optional short name for this deployment (for
	// example "prod / homelab"), shown in the web UI's top bar via
	// GET /healthz's `environment`. Unauthenticated there, so never
	// secret.
	InstanceLabel string `mapstructure:"instance_label"`
}

// maxInstanceLabelLength keeps the label short enough for a top-bar badge.
const maxInstanceLabelLength = 40

// Validate catches a bad value at startup rather than wherever it's first
// read - an empty Addr surfaces as a cryptic net/http bind failure and an
// empty DBPath as a hard-to-place sqlite open error, both well past where
// the actual mistake was made.
func (c config) Validate() error {
	if c.Addr == "" {
		return errAddrRequired
	}

	if c.DBPath == "" {
		return errDBPathRequired
	}

	return c.validateInstanceLabel()
}

func (c config) validateInstanceLabel() error {
	if utf8.RuneCountInString(c.InstanceLabel) > maxInstanceLabelLength {
		return errInstanceLabelTooLong
	}

	if strings.ContainsFunc(c.InstanceLabel, unicode.IsControl) {
		return errInstanceLabelControl
	}

	return nil
}

func loadConfig() (config, error) {
	v := viper.New()
	v.SetDefault("addr", ":8080")
	v.SetDefault("db_path", "hush-hush.db")

	for key, env := range map[string]string{
		"addr":           "ADDR",
		"db_path":        "DB_PATH",
		"public_url":     "PUBLIC_URL",
		"instance_label": "INSTANCE_LABEL",
	} {
		if err := v.BindEnv(key, env); err != nil {
			return config{}, fmt.Errorf("bind %s: %w", env, err)
		}
	}

	var c config
	if err := v.Unmarshal(&c); err != nil {
		return config{}, fmt.Errorf("unmarshal config: %w", err)
	}

	if err := c.Validate(); err != nil {
		return config{}, fmt.Errorf("invalid config: %w", err)
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
	root.AddCommand(newHealthcheckCmd())

	return root
}

// newHealthcheckCmd exists for the container's own HEALTHCHECK: the image is
// scratch-based (no shell, no curl, no wget), so there's nothing else inside
// it that could exec a probe. It reads the same ADDR this process is already
// serving on and asks its own /healthz over loopback - a nonzero exit is
// Docker's signal to mark the container unhealthy.
func newHealthcheckCmd() *cobra.Command {
	var ready bool

	cmd := &cobra.Command{
		Use:    "healthcheck",
		Short:  "Check that this server answers its own /healthz, or /readyz with --ready",
		Hidden: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			_, port, err := net.SplitHostPort(cfg.Addr)
			if err != nil {
				return fmt.Errorf("parse addr %q: %w", cfg.Addr, err)
			}

			path := "/healthz"
			if ready {
				path = "/readyz"
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Second)
			defer cancel()

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:"+port+path, nil)
			if err != nil {
				return fmt.Errorf("build request: %w", err)
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return fmt.Errorf("request %s: %w", path, err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("%w: %s returned %d", errHealthzStatus, path, resp.StatusCode)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&ready, "ready", false, "probe /readyz (can the database serve requests) instead of /healthz (is the process up)")

	return cmd
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

	build, err := fs.Sub(webBuild, "web/build")
	if err != nil {
		slog.Error("open embedded web build", "error", err)

		return fmt.Errorf("open embedded web build: %w", err)
	}

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           hushhush.NewMux(s, cfg.PublicURL, build, version, hushhush.WithInstanceLabel(cfg.InstanceLabel)),
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
		Short: "Issue, list, rotate, and revoke write-path tokens",
	}

	cmd.AddCommand(newTokenIssueCmd())
	cmd.AddCommand(newTokenListCmd())
	cmd.AddCommand(newTokenRevokeCmd())
	cmd.AddCommand(newTokenRotateCmd())

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

// errTTLAboveMaximum is what checkTokenTTL wraps, so a caller can test for it.
var errTTLAboveMaximum = errors.New("--ttl is above the maximum lifetime")

// checkTokenTTL holds the local token commands to the same lifetime limit the
// HTTP API enforces, since they mint straight into the store. Only the upper
// bound is checked here: a zero or negative TTL is the store's to reject, as
// it always was.
func checkTokenTTL(ttl time.Duration) error {
	if ttl > hushhush.MaxTokenTTL {
		return fmt.Errorf("%w: at most %s", errTTLAboveMaximum, hushhush.MaxTokenTTL)
	}

	return nil
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
			if err := checkTokenTTL(ttl); err != nil {
				return err
			}

			s, err := openStoreForTokenCmd()
			if err != nil {
				return err
			}
			defer func() { _ = s.Close() }()

			wt, token, err := s.CreateWriteToken(cmd.Context(), description, ttl, "")
			if err != nil {
				return fmt.Errorf("issue token: %w", err)
			}

			if _, err := fmt.Fprintf(cmd.OutOrStdout(),
				"id:    %s\ntoken: %s\n\nThe token is shown once - store it now, it can't be recovered later.\n",
				wt.ID, token,
			); err != nil {
				return fmt.Errorf("write issued token: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&description, "description", "", "what this token is for")
	cmd.Flags().DurationVar(&ttl, "ttl", hushhush.DefaultTokenTTL, "how long the token stays valid, at most "+hushhush.MaxTokenTTL.String())
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

func newTokenRotateCmd() *cobra.Command {
	var ttl time.Duration

	cmd := &cobra.Command{
		Use:   "rotate <id>",
		Short: "Replace a token's secret and expiry, keeping its id and description",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := checkTokenTTL(ttl); err != nil {
				return err
			}

			s, err := openStoreForTokenCmd()
			if err != nil {
				return err
			}
			defer func() { _ = s.Close() }()

			wt, token, err := s.RotateWriteToken(cmd.Context(), args[0], ttl)
			if err != nil {
				return fmt.Errorf("rotate token: %w", err)
			}

			if _, err := fmt.Fprintf(cmd.OutOrStdout(),
				"id:    %s\ntoken: %s\n\nThe token is shown once - store it now, it can't be recovered later.\n",
				wt.ID, token,
			); err != nil {
				return fmt.Errorf("write rotated token: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().DurationVar(&ttl, "ttl", 90*24*time.Hour, "how long the rotated token stays valid")

	return cmd
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
