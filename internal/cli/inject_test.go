package cli_test

import (
	"bytes"
	"context"
	"io"
	"net/http/httptest"
	"testing"

	"filippo.io/age"
	hushhush "github.com/alrayyes/hush-hush/internal/api"
	"github.com/alrayyes/hush-hush/internal/cli"
	"github.com/alrayyes/hush-hush/internal/client"
	"github.com/alrayyes/hush-hush/internal/store"
	"github.com/stretchr/testify/require"
)

// testWriterToken is shared by every test in this package that calls
// newTestServer - none of them need a distinct token.
const testWriterToken = "writer-token"

func newTestServer(t *testing.T) (*httptest.Server, *store.Store) {
	t.Helper()

	s, err := store.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, s.Close()) })

	srv := httptest.NewServer(hushhush.NewMux(s, testWriterToken))
	t.Cleanup(srv.Close)

	return srv, s
}

func TestInjectCreatesAnObjectTheMatchingIdentityCanDecrypt(t *testing.T) {
	t.Parallel()

	srv, s := newTestServer(t)

	identity, err := age.GenerateX25519Identity()
	require.NoError(t, err)

	cfg := cli.Config{Server: srv.URL, Token: testWriterToken}
	value := []byte("plaintext-value")

	err = cli.Inject(context.Background(), cfg, "mattermost_deploy_webhook", value,
		[]string{identity.Recipient().String()}, []string{"homelab/vps-docker"})
	require.NoError(t, err)

	obj, err := s.GetObject(context.Background(), "mattermost_deploy_webhook")
	require.NoError(t, err)
	require.Equal(t, []string{"homelab/vps-docker"}, obj.UsedBy)
	require.NotEqual(t, value, obj.Value, "stored value must be sealed, not plaintext")

	r, err := age.Decrypt(bytes.NewReader(obj.Value), identity)
	require.NoError(t, err)

	plaintext, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, value, plaintext)
}

func TestInjectWithoutAValidTokenIsRejected(t *testing.T) {
	t.Parallel()

	srv, _ := newTestServer(t)

	identity, err := age.GenerateX25519Identity()
	require.NoError(t, err)

	cfg := cli.Config{Server: srv.URL, Token: "wrong-token"}

	err = cli.Inject(context.Background(), cfg, "x", []byte("v"), []string{identity.Recipient().String()}, nil)
	require.ErrorIs(t, err, client.ErrUnauthorized)
}

func TestInjectWithAMalformedRecipientFailsBeforeCallingTheServer(t *testing.T) {
	t.Parallel()

	srv, s := newTestServer(t)

	cfg := cli.Config{Server: srv.URL, Token: testWriterToken}

	err := cli.Inject(context.Background(), cfg, "x", []byte("v"), []string{"not-a-recipient"}, nil)
	require.Error(t, err)

	_, getErr := s.GetObject(context.Background(), "x")
	require.ErrorIs(t, getErr, store.ErrNotFound, "a sealing failure must not reach the server")
}
