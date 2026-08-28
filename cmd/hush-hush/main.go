// Command hush-hush is the composition root for the service: it wires the
// in-memory widget store to the handlers and starts the server. This is
// still the scaffold's placeholder example - see CLAUDE.md and
// openspec/changes/secrets-object-store/ for the real API replacing it.
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

	widgets := map[string]hushhush.Widget{
		"hammer": {ID: "hammer", Name: "Claw hammer"},
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           hushhush.NewMux(widgets),
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

	slog.Info("starting", "version", version, "addr", addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("server stopped", "error", err)
		return 1
	}

	return 0
}
