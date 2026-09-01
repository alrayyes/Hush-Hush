package cli_test

import (
	"context"
	"testing"

	"filippo.io/age"
	"github.com/alrayyes/hush-hush/internal/cli"
	"github.com/alrayyes/hush-hush/internal/client"
	"github.com/stretchr/testify/require"
)

func TestGetDecryptsTheStoredValue(t *testing.T) {
	t.Parallel()

	srv, _, token := newTestServer(t)

	identity, err := age.GenerateX25519Identity()
	require.NoError(t, err)

	value := []byte("plaintext-value")

	injectCfg := cli.Config{Server: srv.URL, Token: token}
	require.NoError(t, cli.Inject(context.Background(), injectCfg, "mattermost_deploy_webhook", value,
		[]string{identity.Recipient().String()}, nil, ""))

	getCfg := cli.Config{Server: srv.URL}
	plaintext, err := cli.Get(context.Background(), getCfg, "mattermost_deploy_webhook", []string{identity.String()})
	require.NoError(t, err)
	require.Equal(t, value, plaintext)
}

func TestGetWithNoMatchingIdentityFailsClearly(t *testing.T) {
	t.Parallel()

	srv, _, token := newTestServer(t)

	sealedTo, err := age.GenerateX25519Identity()
	require.NoError(t, err)

	wrongIdentity, err := age.GenerateX25519Identity()
	require.NoError(t, err)

	injectCfg := cli.Config{Server: srv.URL, Token: token}
	require.NoError(t, cli.Inject(context.Background(), injectCfg, "x", []byte("v"),
		[]string{sealedTo.Recipient().String()}, nil, ""))

	getCfg := cli.Config{Server: srv.URL}
	_, err = cli.Get(context.Background(), getCfg, "x", []string{wrongIdentity.String()})
	require.Error(t, err)
}

func TestGetUnknownIDReturnsNotFound(t *testing.T) {
	t.Parallel()

	srv, _, _ := newTestServer(t)

	identity, err := age.GenerateX25519Identity()
	require.NoError(t, err)

	getCfg := cli.Config{Server: srv.URL}
	_, err = cli.Get(context.Background(), getCfg, "nope", []string{identity.String()})
	require.ErrorIs(t, err, client.ErrNotFound)
}
