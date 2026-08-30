package main

import (
	"bytes"
	"net/http/httptest"
	"testing"

	"filippo.io/age"
	hushhush "github.com/alrayyes/hush-hush/internal/api"
	"github.com/alrayyes/hush-hush/internal/seal"
	"github.com/alrayyes/hush-hush/internal/store"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

// TestGetRunsFromEnvironmentAloneNoFlags mirrors
// TestInjectRunsFromEnvironmentAloneNoFlags - the same "runs unmodified
// inside CI" requirement, for the read path. get needs no bearer token,
// so only HUSH_HUSH_SERVER and HUSH_HUSH_IDENTITY are set.
func TestGetRunsFromEnvironmentAloneNoFlags(t *testing.T) {
	s, err := store.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, s.Close()) })

	srv := httptest.NewServer(hushhush.NewMux(s))
	t.Cleanup(srv.Close)

	identity, err := age.GenerateX25519Identity()
	require.NoError(t, err)

	sealed, err := seal.Seal([]byte("plaintext-value"), []string{identity.Recipient().String()})
	require.NoError(t, err)
	require.NoError(t, s.CreateObject(t.Context(), "mattermost_deploy_webhook", sealed, nil))

	t.Setenv("HUSH_HUSH_SERVER", srv.URL)
	t.Setenv("HUSH_HUSH_IDENTITY", identity.String())

	viper.Reset()

	var out bytes.Buffer

	root := newRootCmd()
	root.SetArgs([]string{"get", "mattermost_deploy_webhook"})
	root.SetOut(&out)

	require.NoError(t, root.Execute())
	require.Equal(t, "plaintext-value", out.String())
}
