package cli_test

import (
	"context"
	"testing"

	"filippo.io/age"
	"github.com/alrayyes/hush-hush/internal/cli"
	"github.com/alrayyes/hush-hush/internal/client"
	"github.com/alrayyes/hush-hush/internal/store"
	"github.com/stretchr/testify/require"
)

func TestDeleteRemovesTheObject(t *testing.T) {
	t.Parallel()

	srv, s, token := newTestServer(t)

	identity, err := age.GenerateX25519Identity()
	require.NoError(t, err)

	cfg := cli.Config{Server: srv.URL, Token: token}
	require.NoError(t, cli.Inject(context.Background(), cfg, "mattermost_deploy_webhook", []byte("v"),
		[]string{identity.Recipient().String()}, nil, ""))

	require.NoError(t, cli.Delete(context.Background(), cfg, "mattermost_deploy_webhook"))

	_, err = s.GetObject(context.Background(), "mattermost_deploy_webhook")
	require.ErrorIs(t, err, store.ErrNotFound)
}

func TestDeleteWithoutAValidTokenIsRejected(t *testing.T) {
	t.Parallel()

	srv, s, token := newTestServer(t)

	identity, err := age.GenerateX25519Identity()
	require.NoError(t, err)

	cfg := cli.Config{Server: srv.URL, Token: token}
	require.NoError(t, cli.Inject(context.Background(), cfg, "x", []byte("v"),
		[]string{identity.Recipient().String()}, nil, ""))

	wrongCfg := cli.Config{Server: srv.URL, Token: "wrong-token"}
	err = cli.Delete(context.Background(), wrongCfg, "x")
	require.ErrorIs(t, err, client.ErrUnauthorized)

	_, err = s.GetObject(context.Background(), "x")
	require.NoError(t, err, "a rejected delete must not remove the object")
}

func TestDeleteUnknownIDReturnsNotFound(t *testing.T) {
	t.Parallel()

	srv, _, token := newTestServer(t)

	cfg := cli.Config{Server: srv.URL, Token: token}
	err := cli.Delete(context.Background(), cfg, "nope")
	require.ErrorIs(t, err, client.ErrNotFound)
}
