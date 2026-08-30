package main

import (
	"net/http/httptest"
	"testing"
	"time"

	hushhush "github.com/alrayyes/hush-hush/internal/api"
	"github.com/alrayyes/hush-hush/internal/store"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

// TestDeleteRunsFromEnvironmentAloneNoFlags mirrors
// TestInjectRunsFromEnvironmentAloneNoFlags - the same "runs unmodified
// inside CI" requirement, for the delete command.
func TestDeleteRunsFromEnvironmentAloneNoFlags(t *testing.T) {
	s, err := store.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, s.Close()) })

	srv := httptest.NewServer(hushhush.NewMux(s))
	t.Cleanup(srv.Close)

	_, token, err := s.CreateWriteToken(t.Context(), "test", time.Hour)
	require.NoError(t, err)

	require.NoError(t, s.CreateObject(t.Context(), "mattermost_deploy_webhook", []byte("sealed"), nil))

	t.Setenv("HUSH_HUSH_SERVER", srv.URL)
	t.Setenv("HUSH_HUSH_TOKEN", token)

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	viper.Reset()

	root := newRootCmd()
	root.SetArgs([]string{"delete", "mattermost_deploy_webhook"})

	require.NoError(t, root.Execute())

	_, err = s.GetObject(t.Context(), "mattermost_deploy_webhook")
	require.ErrorIs(t, err, store.ErrNotFound)
}
