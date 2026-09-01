package cli_test

import (
	"bytes"
	"context"
	"io"
	"testing"

	"filippo.io/age"
	"github.com/alrayyes/hush-hush/internal/cli"
	"github.com/alrayyes/hush-hush/internal/client"
	"github.com/stretchr/testify/require"
)

func TestUpdateChangesTheStoredValue(t *testing.T) {
	t.Parallel()

	srv, s, token := newTestServer(t)

	identity, err := age.GenerateX25519Identity()
	require.NoError(t, err)

	cfg := cli.Config{Server: srv.URL, Token: token}
	require.NoError(t, cli.Inject(context.Background(), cfg, "mattermost_deploy_webhook", []byte("old-value"),
		[]string{identity.Recipient().String()}, []string{"homelab/vps-docker"}, ""))

	newValue := []byte("new-value")
	err = cli.Update(context.Background(), cfg, "mattermost_deploy_webhook", newValue,
		[]string{identity.Recipient().String()})
	require.NoError(t, err)

	obj, err := s.GetObject(context.Background(), "mattermost_deploy_webhook")
	require.NoError(t, err)
	require.Equal(t, []string{"homelab/vps-docker"}, obj.UsedBy, "update must not touch used_by")

	r, err := age.Decrypt(bytes.NewReader(obj.Value), identity)
	require.NoError(t, err)

	plaintext, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, newValue, plaintext)
}

func TestUpdateWithoutAValidTokenIsRejected(t *testing.T) {
	t.Parallel()

	srv, _, _ := newTestServer(t)

	identity, err := age.GenerateX25519Identity()
	require.NoError(t, err)

	cfg := cli.Config{Server: srv.URL, Token: "wrong-token"}

	err = cli.Update(context.Background(), cfg, "x", []byte("v"), []string{identity.Recipient().String()})
	require.ErrorIs(t, err, client.ErrUnauthorized)
}

func TestUpdateWithAMalformedRecipientFailsBeforeCallingTheServer(t *testing.T) {
	t.Parallel()

	srv, s, token := newTestServer(t)

	identity, err := age.GenerateX25519Identity()
	require.NoError(t, err)

	cfg := cli.Config{Server: srv.URL, Token: token}
	require.NoError(t, cli.Inject(context.Background(), cfg, "x", []byte("v"),
		[]string{identity.Recipient().String()}, nil, ""))

	before, err := s.GetObject(context.Background(), "x")
	require.NoError(t, err)

	err = cli.Update(context.Background(), cfg, "x", []byte("v2"), []string{"not-a-recipient"})
	require.Error(t, err)

	after, err := s.GetObject(context.Background(), "x")
	require.NoError(t, err)
	require.Equal(t, before.Value, after.Value, "a sealing failure must not reach the server")
}
