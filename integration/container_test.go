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
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// containerEndpoint and containerWriterToken are set once by TestMain -
// the image takes on the order of a minute to build, so every test in
// this package shares one running container rather than each paying that
// cost separately.
var (
	containerEndpoint    string
	containerWriterToken string
)

func TestMain(m *testing.M) {
	ctx := context.Background()

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Context:    "..",
				Dockerfile: "Dockerfile",
			},
			ExposedPorts: []string{"8080/tcp"},
			// A real file under the image's own baked-in, already-writable
			// /data (Dockerfile's own comment) rather than :memory: - the
			// token issued below runs as a separate exec into this same
			// container, which would get its own, disconnected in-memory
			// database otherwise.
			Env: map[string]string{
				"DB_PATH": "/data/hush-hush.db",
			},
			WaitingFor: wait.ForHTTP("/healthz").WithStartupTimeout(2 * time.Minute),
		},
		Started: true,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	containerEndpoint, err = container.Endpoint(ctx, "http")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	execCode, reader, err := container.Exec(ctx,
		[]string{"/hush-hush", "token", "issue", "--description", "container integration test"})
	if err != nil {
		fmt.Fprintln(os.Stderr, "issue token:", err)
		os.Exit(1)
	}

	issueOut, err := io.ReadAll(reader)
	if err != nil {
		fmt.Fprintln(os.Stderr, "read token issue output:", err)
		os.Exit(1)
	}

	if execCode != 0 {
		fmt.Fprintln(os.Stderr, "token issue exited nonzero:", string(issueOut))
		os.Exit(1)
	}

	var ok bool
	containerWriterToken, ok = parseIssuedToken(string(issueOut))
	if !ok {
		fmt.Fprintln(os.Stderr, "could not find issued token in output:", string(issueOut))
		os.Exit(1)
	}

	exitCode := m.Run()

	if err := container.Terminate(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}

	os.Exit(exitCode)
}

// parseIssuedToken pulls the plaintext out of `hush-hush token issue`'s
// "token: <value>" line - the only machine-readable thing about that
// output, which is meant for a human running the command by hand.
func parseIssuedToken(output string) (token string, ok bool) {
	for _, line := range strings.Split(output, "\n") {
		if after, found := strings.CutPrefix(line, "token: "); found {
			return after, true
		}
	}

	return "", false
}

func TestContainerHealthz(t *testing.T) {
	resp, err := http.Get(containerEndpoint + "/healthz") //nolint:noctx // fixed test URL, no request-scoped context needed
	require.NoError(t, err)

	defer func() { require.NoError(t, resp.Body.Close()) }()
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestContainerCreateGetRoundTrip(t *testing.T) {
	ctx := context.Background()

	createReq, err := http.NewRequestWithContext(ctx, http.MethodPost, containerEndpoint+"/objects",
		bytes.NewReader([]byte(`{"id":"container_smoke_test","value":"c2VhbGVkLWNpcGhlcnRleHQ="}`)))
	require.NoError(t, err)

	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Bearer "+containerWriterToken)

	createResp, err := http.DefaultClient.Do(createReq)
	require.NoError(t, err)

	defer func() { require.NoError(t, createResp.Body.Close()) }()
	require.Equal(t, http.StatusCreated, createResp.StatusCode)

	getResp, err := http.Get(containerEndpoint + "/objects/container_smoke_test") //nolint:noctx // fixed test URL, no request-scoped context needed
	require.NoError(t, err)

	defer func() { require.NoError(t, getResp.Body.Close()) }()
	require.Equal(t, http.StatusOK, getResp.StatusCode)

	body, err := io.ReadAll(getResp.Body)
	require.NoError(t, err)
	require.Equal(t, []byte("sealed-ciphertext"), body)
}
