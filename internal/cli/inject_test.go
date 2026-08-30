package cli_test

import (
	"bytes"
	"context"
	"io"
	"net/http/httptest"
	"testing"
	"time"

	"filippo.io/age"
	hushhush "github.com/alrayyes/hush-hush/internal/api"
	"github.com/alrayyes/hush-hush/internal/cli"
	"github.com/alrayyes/hush-hush/internal/client"
	"github.com/alrayyes/hush-hush/internal/store"
	"github.com/stretchr/testify/require"
)

// newTestServer issues a fresh write token valid against its own store -
// none of this package's tests need a distinct one.
func newTestServer(t *testing.T) (srv *httptest.Server, s *store.Store, token string) {
	t.Helper()

	s, err := store.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, s.Close()) })

	_, token, err = s.CreateWriteToken(t.Context(), "test", time.Hour)
	require.NoError(t, err)

	srv = httptest.NewServer(hushhush.NewMux(s))
	t.Cleanup(srv.Close)

	return srv, s, token
}

func TestInjectCreatesAnObjectTheMatchingIdentityCanDecrypt(t *testing.T) {
	t.Parallel()

	srv, s, token := newTestServer(t)

	identity, err := age.GenerateX25519Identity()
	require.NoError(t, err)

	cfg := cli.Config{Server: srv.URL, Token: token}
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

	srv, _, _ := newTestServer(t)

	identity, err := age.GenerateX25519Identity()
	require.NoError(t, err)

	cfg := cli.Config{Server: srv.URL, Token: "wrong-token"}

	err = cli.Inject(context.Background(), cfg, "x", []byte("v"), []string{identity.Recipient().String()}, nil)
	require.ErrorIs(t, err, client.ErrUnauthorized)
}

func TestInjectWithAMalformedRecipientFailsBeforeCallingTheServer(t *testing.T) {
	t.Parallel()

	srv, s, token := newTestServer(t)

	cfg := cli.Config{Server: srv.URL, Token: token}

	err := cli.Inject(context.Background(), cfg, "x", []byte("v"), []string{"not-a-recipient"}, nil)
	require.Error(t, err)

	_, getErr := s.GetObject(context.Background(), "x")
	require.ErrorIs(t, getErr, store.ErrNotFound, "a sealing failure must not reach the server")
}
