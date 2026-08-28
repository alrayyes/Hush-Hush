//go:build pact

// See internal/client/pact_test.go's build-tag comment - the same static
// link requirement applies here.
package api_test

import (
	"context"
	"errors"
	"fmt"
	"net/http/httptest"
	"path/filepath"
	"testing"

	hushhush "github.com/alrayyes/hush-hush/internal/api"
	"github.com/alrayyes/hush-hush/internal/store"
	"github.com/pact-foundation/pact-go/v2/models"
	"github.com/pact-foundation/pact-go/v2/provider"
	"github.com/stretchr/testify/require"
)

const pactWriterToken = "test-token"

// TestPactProviderVerification confirms the real server satisfies every
// interaction internal/client's consumer tests recorded - the other half
// of the CLI-server contract (design.md: "a local pact file, no broker").
func TestPactProviderVerification(t *testing.T) {
	s, err := store.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, s.Close()) })

	srv := httptest.NewServer(hushhush.NewMux(s, pactWriterToken))
	t.Cleanup(srv.Close)

	pactFile, err := filepath.Abs("../../pacts/hush-hush-cli-hush-hush-server.json")
	require.NoError(t, err)

	verifier := provider.NewVerifier()
	err = verifier.VerifyProvider(t, provider.VerifyRequest{
		ProviderBaseURL: srv.URL,
		PactFiles:       []string{pactFile},
		StateHandlers: models.StateHandlers{
			// Idempotent: every interaction needing this state runs it
			// immediately beforehand, including one that deletes the
			// object as its own action - recreating unconditionally
			// keeps each interaction's precondition true regardless of
			// what a prior interaction in the same run did to it.
			"an object exists": func(_ bool, _ models.ProviderState) (models.ProviderStateResponse, error) {
				err := s.CreateObject(context.Background(), "mattermost_deploy_webhook", []byte("sealed-ciphertext"), []string{"homelab/vps-docker"})
				if err != nil && !errors.Is(err, store.ErrAlreadyExists) {
					return nil, fmt.Errorf("seed test object: %w", err)
				}

				return nil, nil
			},
		},
	})
	require.NoError(t, err)
}
