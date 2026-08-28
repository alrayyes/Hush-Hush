package main

import (
	"bytes"
	"net/http/httptest"
	"testing"

	"filippo.io/age"
	hushhush "github.com/alrayyes/hush-hush/internal/api"
	"github.com/alrayyes/hush-hush/internal/store"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

// TestInjectRunsFromEnvironmentAloneNoFlags is the CLI spec's "runs
// unmodified inside CI" requirement: a CI job supplies configuration
// through its own secret storage as environment variables, never flags,
// and never a bespoke wrapper or Action.
func TestInjectRunsFromEnvironmentAloneNoFlags(t *testing.T) {
	s, err := store.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, s.Close()) })

	srv := httptest.NewServer(hushhush.NewMux(s, "env-token"))
	t.Cleanup(srv.Close)

	identity, err := age.GenerateX25519Identity()
	require.NoError(t, err)

	t.Setenv("HUSH_HUSH_SERVER", srv.URL)
	t.Setenv("HUSH_HUSH_TOKEN", "env-token")
	t.Setenv("HUSH_HUSH_RECIPIENTS", identity.Recipient().String())

	// A fresh viper instance per test: the package-level default one
	// otherwise carries flag bindings and values across tests in this
	// package, which is exactly what a "no CI-specific code path" test
	// must not rely on to pass.
	viper.Reset()

	root := newRootCmd()
	root.SetArgs([]string{"inject", "mattermost_deploy_webhook"})
	root.SetIn(bytes.NewReader([]byte("plaintext-value")))

	require.NoError(t, root.Execute())

	obj, err := s.GetObject(t.Context(), "mattermost_deploy_webhook")
	require.NoError(t, err)
	require.NotEqual(t, []byte("plaintext-value"), obj.Value)
}
