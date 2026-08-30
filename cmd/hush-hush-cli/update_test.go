package main

import (
	"bytes"
	"net/http/httptest"
	"testing"
	"time"

	"filippo.io/age"
	hushhush "github.com/alrayyes/hush-hush/internal/api"
	"github.com/alrayyes/hush-hush/internal/seal"
	"github.com/alrayyes/hush-hush/internal/store"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

// TestUpdateRunsFromEnvironmentAloneNoFlags mirrors
// TestInjectRunsFromEnvironmentAloneNoFlags - the same "runs unmodified
// inside CI" requirement, for the write-path update command.
func TestUpdateRunsFromEnvironmentAloneNoFlags(t *testing.T) {
	s, err := store.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, s.Close()) })

	srv := httptest.NewServer(hushhush.NewMux(s))
	t.Cleanup(srv.Close)

	_, token, err := s.CreateWriteToken(t.Context(), "test", time.Hour)
	require.NoError(t, err)

	identity, err := age.GenerateX25519Identity()
	require.NoError(t, err)

	sealed, err := seal.Seal([]byte("old-value"), []string{identity.Recipient().String()})
	require.NoError(t, err)
	require.NoError(t, s.CreateObject(t.Context(), "mattermost_deploy_webhook", sealed, nil))

	t.Setenv("HUSH_HUSH_SERVER", srv.URL)
	t.Setenv("HUSH_HUSH_TOKEN", token)
	t.Setenv("HUSH_HUSH_RECIPIENTS", identity.Recipient().String())

	viper.Reset()

	root := newRootCmd()
	root.SetArgs([]string{"update", "mattermost_deploy_webhook"})
	root.SetIn(bytes.NewReader([]byte("new-value")))

	require.NoError(t, root.Execute())

	obj, err := s.GetObject(t.Context(), "mattermost_deploy_webhook")
	require.NoError(t, err)
	require.NotEqual(t, []byte("new-value"), obj.Value)
}
