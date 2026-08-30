// Package cli implements every hush-hush CLI command as a plain function
// over Config, independent of cobra - the actual command definitions in
// cmd/hush-hush-cli are a thin parsing shell around these
// (rules/go.md: "Keep RunE a thin shell").
package cli

import (
	"fmt"

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

func (c Config) newClient() (*client.Client, error) {
	cl, err := client.New(c.Server, c.Token)
	if err != nil {
		return nil, fmt.Errorf("build client: %w", err)
	}

	cl.Caller = c.Caller

	return cl, nil
}
