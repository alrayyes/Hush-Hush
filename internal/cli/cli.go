// Package cli implements every hush-hush CLI command as a plain function
// over Config, independent of cobra - the actual command definitions in
// cmd/hush-hush-cli are a thin parsing shell around these
// (rules/go.md: "Keep RunE a thin shell").
package cli

import (
	"errors"
	"fmt"
	"net/url"

	"github.com/alrayyes/hush-hush/internal/client"
)

// Config is the CLI's runtime configuration, shared by every command.
type Config struct {
	// Server is the hush-hush server's base URL.
	Server string
	// Token is the write-path bearer token - needed by inject, update,
	// and delete, ignored by get.
	Token string
	// Caller is this CLI's self-presented identity for the audit log
	// (api/openapi.yaml's X-Caller header) - optional.
	Caller string
}

// Validate's own sentinels - fixed conditions, not messages built from
// per-call detail, which stays in the %w wrap around each at the call site.
var (
	errServerRequired    = errors.New("server: required")
	errServerNotAbsolute = errors.New("not an absolute URL")
)

// Validate catches a bad Server value at startup rather than at the first
// request - an empty or malformed base URL otherwise only surfaces as a
// generic connection error from deep inside net/http.
func (c Config) Validate() error {
	if c.Server == "" {
		return errServerRequired
	}

	u, err := url.Parse(c.Server)
	if err != nil {
		return fmt.Errorf("server: %w", err)
	}

	if u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("server %q: %w", c.Server, errServerNotAbsolute)
	}

	return nil
}

func (c Config) newClient() (*client.Client, error) {
	cl, err := client.New(c.Server, c.Token)
	if err != nil {
		return nil, fmt.Errorf("build client: %w", err)
	}

	cl.Caller = c.Caller

	return cl, nil
}
