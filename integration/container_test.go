//go:build integration

// Package integration boots the actual, built Docker image and proves it
// serves real requests - the layer none of internal/api's or
// internal/store's tests reach, since they all run the server's Go code
// directly rather than the packaged distroless artifact someone actually
// pulls and runs (rules/go-test.md: "The container/integration layer uses
// testcontainers-go"). Gated behind a build tag and its own CI job for the
// same reason `pact` is: it needs something (here, a Docker daemon) most
// of the test suite doesn't, and shouldn't hold up `go test ./...` for
// everyone who has it.
package integration

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const containerWriterToken = "container-test-token"

func TestContainerServesHealthzAndACreateGetRoundTrip(t *testing.T) {
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    "..",
			Dockerfile: "Dockerfile",
		},
		ExposedPorts: []string{"8080/tcp"},
		Env: map[string]string{
			"WRITER_TOKEN": containerWriterToken,
			"DB_PATH":      ":memory:",
		},
		WaitingFor: wait.ForHTTP("/healthz").WithStartupTimeout(2 * time.Minute),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, container.Terminate(context.Background()))
	})

	endpoint, err := container.Endpoint(ctx, "http")
	require.NoError(t, err)

	healthResp, err := http.Get(endpoint + "/healthz") //nolint:noctx // fixed test URL, no request-scoped context needed
	require.NoError(t, err)

	defer func() { require.NoError(t, healthResp.Body.Close()) }()
	require.Equal(t, http.StatusOK, healthResp.StatusCode)

	createReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint+"/objects",
		bytes.NewReader([]byte(`{"id":"container_smoke_test","value":"c2VhbGVkLWNpcGhlcnRleHQ="}`)))
	require.NoError(t, err)

	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Bearer "+containerWriterToken)

	createResp, err := http.DefaultClient.Do(createReq)
	require.NoError(t, err)

	defer func() { require.NoError(t, createResp.Body.Close()) }()
	require.Equal(t, http.StatusCreated, createResp.StatusCode)

	getResp, err := http.Get(endpoint + "/objects/container_smoke_test") //nolint:noctx // fixed test URL, no request-scoped context needed
	require.NoError(t, err)

	defer func() { require.NoError(t, getResp.Body.Close()) }()
	require.Equal(t, http.StatusOK, getResp.StatusCode)

	body, err := io.ReadAll(getResp.Body)
	require.NoError(t, err)
	require.Equal(t, []byte("sealed-ciphertext"), body)
}
