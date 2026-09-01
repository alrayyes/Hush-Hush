//go:build pact

// pact-go statically links the native pact_ffi library into this test
// binary at link time - not just when a Pact test actually runs, but the
// moment this package's tests are built at all. Gating it behind a build
// tag keeps every other test in this package, and `go test ./...` for
// everyone who hasn't installed that library, unaffected.
package client_test

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/alrayyes/hush-hush/internal/client"
	"github.com/pact-foundation/pact-go/v2/consumer"
	"github.com/pact-foundation/pact-go/v2/matchers"
	"github.com/stretchr/testify/require"
)

// pactDir is the repo-root pacts/ directory - committed, and read directly
// by the provider verification test in internal/api, per design.md's
// "local pact file, no broker" decision.
func pactDir(t *testing.T) string {
	t.Helper()

	dir, err := filepath.Abs("../../pacts")
	require.NoError(t, err)

	return dir
}

func newMockProvider(t *testing.T) *consumer.V4HTTPMockProvider {
	t.Helper()

	provider, err := consumer.NewV4Pact(consumer.MockHTTPProviderConfig{
		Consumer: "hush-hush-cli",
		Provider: "hush-hush-server",
		PactDir:  pactDir(t),
	})
	require.NoError(t, err)

	return provider
}

// createTestObjectID deliberately differs from the id the get/update/delete
// interactions share (seeded by the provider's "an object exists" state
// handler) - this interaction has no precondition and needs an id the
// provider hasn't already created, or the real server would answer 409
// instead of the 201 this interaction expects.
const createTestObjectID = "new_secret"

func TestClientCreatePact(t *testing.T) {
	provider := newMockProvider(t)

	provider.AddInteraction().
		UponReceiving("a request to create an object").
		WithRequest(http.MethodPost, "/objects", func(b *consumer.V4RequestBuilder) {
			b.Header("Authorization", matchers.String("Bearer test-token"))
			b.JSONBody(map[string]any{
				"id":      createTestObjectID,
				"value":   matchers.String("c2VhbGVkLWNpcGhlcnRleHQ="),
				"used_by": []string{"homelab/vps-docker"},
			})
		}).
		WillRespondWith(http.StatusCreated, func(b *consumer.V4ResponseBuilder) {
			b.Header("Content-Type", matchers.String("application/json")).
				JSONBody(map[string]any{
					"id":      createTestObjectID,
					"used_by": []string{"homelab/vps-docker"},
				})
		})

	err := provider.ExecuteTest(t, func(cfg consumer.MockServerConfig) error {
		c, err := client.New(fmt.Sprintf("http://%s:%d", cfg.Host, cfg.Port), "test-token")
		if err != nil {
			return fmt.Errorf("build client: %w", err)
		}

		meta, err := c.Create(context.Background(), createTestObjectID, []byte("sealed-ciphertext"), []string{"homelab/vps-docker"}, "")
		if err != nil {
			return fmt.Errorf("create: %w", err)
		}

		require.Equal(t, createTestObjectID, meta.ID)
		require.Equal(t, []string{"homelab/vps-docker"}, meta.UsedBy)

		return nil
	})
	require.NoError(t, err)
}

func TestClientGetPact(t *testing.T) {
	provider := newMockProvider(t)

	provider.AddInteraction().
		Given("an object exists").
		UponReceiving("a request to fetch an object").
		WithRequest(http.MethodGet, "/objects/mattermost_deploy_webhook").
		WillRespondWith(http.StatusOK, func(b *consumer.V4ResponseBuilder) {
			b.Header("Content-Type", matchers.String("application/octet-stream")).
				Body("application/octet-stream", []byte("sealed-ciphertext"))
		})

	err := provider.ExecuteTest(t, func(cfg consumer.MockServerConfig) error {
		c, err := client.New(fmt.Sprintf("http://%s:%d", cfg.Host, cfg.Port), "")
		if err != nil {
			return fmt.Errorf("build client: %w", err)
		}

		value, err := c.Get(context.Background(), "mattermost_deploy_webhook")
		if err != nil {
			return fmt.Errorf("get: %w", err)
		}

		require.Equal(t, []byte("sealed-ciphertext"), value)

		return nil
	})
	require.NoError(t, err)
}

func TestClientUpdatePact(t *testing.T) {
	provider := newMockProvider(t)

	provider.AddInteraction().
		Given("an object exists").
		UponReceiving("a request to update an object's value").
		WithRequest(http.MethodPut, "/objects/mattermost_deploy_webhook", func(b *consumer.V4RequestBuilder) {
			b.Header("Authorization", matchers.String("Bearer test-token"))
			b.JSONBody(map[string]any{
				"value": matchers.String("bmV3LXNlYWxlZC1jaXBoZXJ0ZXh0"),
			})
		}).
		WillRespondWith(http.StatusOK, func(b *consumer.V4ResponseBuilder) {
			b.Header("Content-Type", matchers.String("application/json")).
				JSONBody(map[string]any{
					"id": "mattermost_deploy_webhook",
				})
		})

	err := provider.ExecuteTest(t, func(cfg consumer.MockServerConfig) error {
		c, err := client.New(fmt.Sprintf("http://%s:%d", cfg.Host, cfg.Port), "test-token")
		if err != nil {
			return fmt.Errorf("build client: %w", err)
		}

		meta, err := c.Update(context.Background(), "mattermost_deploy_webhook", []byte("new-sealed-ciphertext"))
		if err != nil {
			return fmt.Errorf("update: %w", err)
		}

		require.Equal(t, "mattermost_deploy_webhook", meta.ID)

		return nil
	})
	require.NoError(t, err)
}

func TestClientDeletePact(t *testing.T) {
	provider := newMockProvider(t)

	provider.AddInteraction().
		Given("an object exists").
		UponReceiving("a request to delete an object").
		WithRequest(http.MethodDelete, "/objects/mattermost_deploy_webhook", func(b *consumer.V4RequestBuilder) {
			b.Header("Authorization", matchers.String("Bearer test-token"))
		}).
		WillRespondWith(http.StatusNoContent, func(_ *consumer.V4ResponseBuilder) {})

	err := provider.ExecuteTest(t, func(cfg consumer.MockServerConfig) error {
		c, err := client.New(fmt.Sprintf("http://%s:%d", cfg.Host, cfg.Port), "test-token")
		if err != nil {
			return fmt.Errorf("build client: %w", err)
		}

		return c.Delete(context.Background(), "mattermost_deploy_webhook")
	})
	require.NoError(t, err)
}
