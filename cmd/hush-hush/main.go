// Command hush-hush is the composition root: it opens the SQLite store and
// starts the HTTP server. See CLAUDE.md and
// openspec/changes/secrets-object-store/ for the design.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	hushhush "github.com/alrayyes/hush-hush/internal/api"
	"github.com/alrayyes/hush-hush/internal/store"
)

// version is stamped in at build time by goreleaser, from the tag. "dev" is
// what a plain `go build` reports, which is the honest answer for a binary
// built off an unknown tree.
var version = "dev"

func main() {
	// os.Exit from inside main skips every deferred call registered before
	// it - run returns instead, so main is the only place that exits.
	os.Exit(run())
}

func run() int {
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "hush-hush.db"
	}

	// No default: an empty token would mean every write request is
	// rejected forever (requireBearerToken treats "" as never matching),
	// which is a server nobody can write to rather than a safe default -
	// better to fail fast at startup and say so.
	writerToken := os.Getenv("WRITER_TOKEN")
	if writerToken == "" {
		slog.Error("WRITER_TOKEN is required")
		os.Exit(1)
	}

	s, err := store.Open(dbPath)
	if err != nil {
		slog.Error("open store", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := s.Close(); err != nil {
			slog.Error("close store", "error", err)
		}
	}()

	srv := &http.Server{
		Addr:              addr,
		Handler:           hushhush.NewMux(s, writerToken),
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

	slog.Info("starting", "version", version, "addr", addr, "db", dbPath)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("server stopped", "error", err)
		return 1
	}

	return 0
}
