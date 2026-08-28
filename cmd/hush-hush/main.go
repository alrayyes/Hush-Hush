// Command hush-hush is the composition root: it opens the SQLite store and
// starts the HTTP server. See CLAUDE.md and
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
	"github.com/spf13/viper"
)

// version is stamped in at build time by goreleaser, from the tag. "dev" is
// what a plain `go build` reports, which is the honest answer for a binary
// built off an unknown tree.
var version = "dev"

// config is the server's runtime configuration. There's no config file or
// CLI flag surface yet - it's all environment variables - but going through
// viper keeps loading and validation in one place if that changes.
type config struct {
	Addr        string `mapstructure:"addr"`
	DBPath      string `mapstructure:"db_path"`
	WriterToken string `mapstructure:"writer_token"`
}

// validate fails fast on a config that would start the server into a
// useless state. An empty writer token isn't a safe default to fall back
// on - requireBearerToken treats "" as never matching, so it would mean
// every write request is rejected forever.
var errWriterTokenRequired = errors.New("WRITER_TOKEN is required")

func (c config) validate() error {
	if c.WriterToken == "" {
		return errWriterTokenRequired
	}
	return nil
}

func loadConfig() (config, error) {
	v := viper.New()
	v.SetDefault("addr", ":8080")
	v.SetDefault("db_path", "hush-hush.db")

	for key, env := range map[string]string{
		"addr":         "ADDR",
		"db_path":      "DB_PATH",
		"writer_token": "WRITER_TOKEN",
	} {
		if err := v.BindEnv(key, env); err != nil {
			return config{}, fmt.Errorf("bind %s: %w", env, err)
		}
	}

	var c config
	if err := v.Unmarshal(&c); err != nil {
		return config{}, fmt.Errorf("unmarshal config: %w", err)
	}
	if err := c.validate(); err != nil {
		return config{}, err
	}

	return c, nil
}

func main() {
	// os.Exit from inside main skips every deferred call registered before
	// it - run returns instead, so main is the only place that exits.
	os.Exit(run())
}

func run() int {
	cfg, err := loadConfig()
	if err != nil {
		slog.Error("config", "error", err)
		return 1
	}

	s, err := store.Open(cfg.DBPath)
	if err != nil {
		slog.Error("open store", "error", err)
		return 1
	}
	defer func() {
		if err := s.Close(); err != nil {
			slog.Error("close store", "error", err)
		}
	}()

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           hushhush.NewMux(s, cfg.WriterToken),
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
		return 1
	}

	return 0
}
