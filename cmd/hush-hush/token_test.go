package main

import (
	"bytes"
	"testing"
	"time"

	"github.com/alrayyes/hush-hush/internal/store"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

// dbPath points every subcommand under test at the same fresh SQLite file
// - DB_PATH is how a real deployment configures it too, per loadConfig.
// XDG_CONFIG_HOME is isolated too, so the config-nudge path this package
// also exercises never touches a real home directory.
func dbPath(t *testing.T) string {
	t.Helper()

	viper.Reset()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	path := t.TempDir() + "/hush-hush.db"
	t.Setenv("DB_PATH", path)

	return path
}

func TestTokenIssueThenListShowsItsDescription(t *testing.T) {
	path := dbPath(t)

	root := newRootCmd()
	root.SetArgs([]string{"token", "issue", "--description", "homelab/vps-docker deploy"})
	var issueOut bytes.Buffer
	root.SetOut(&issueOut)
	require.NoError(t, root.Execute())
	require.Contains(t, issueOut.String(), "token:")

	viper.Reset()
	t.Setenv("DB_PATH", path)
	root = newRootCmd()
	root.SetArgs([]string{"token", "list"})
	var listOut bytes.Buffer
	root.SetOut(&listOut)
	require.NoError(t, root.Execute())
	require.Contains(t, listOut.String(), "homelab/vps-docker deploy")
}

func TestTokenIssueRequiresDescription(t *testing.T) {
	dbPath(t)

	root := newRootCmd()
	root.SetArgs([]string{"token", "issue"})
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))

	require.Error(t, root.Execute())
}

func TestTokenIssuePrintsATokenTheServerAccepts(t *testing.T) {
	path := dbPath(t)

	root := newRootCmd()
	root.SetArgs([]string{"token", "issue", "--description", "a"})
	var out bytes.Buffer
	root.SetOut(&out)
	require.NoError(t, root.Execute())

	s, err := store.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, s.Close()) })

	tokens, err := s.ListWriteTokens(t.Context())
	require.NoError(t, err)
	require.Len(t, tokens, 1)
}

func TestTokenRevokeInvalidatesTheToken(t *testing.T) {
	path := dbPath(t)

	s, err := store.Open(path)
	require.NoError(t, err)
	id, token, err := s.CreateWriteToken(t.Context(), "a", time.Hour)
	require.NoError(t, err)
	require.NoError(t, s.Close())

	viper.Reset()
	t.Setenv("DB_PATH", path)
	root := newRootCmd()
	root.SetArgs([]string{"token", "revoke", id})
	require.NoError(t, root.Execute())

	s, err = store.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, s.Close()) })

	valid, err := s.ValidateWriteToken(t.Context(), token)
	require.NoError(t, err)
	require.False(t, valid)
}

func TestTokenRevokeUnknownIDFails(t *testing.T) {
	dbPath(t)

	root := newRootCmd()
	root.SetArgs([]string{"token", "revoke", "nope"})
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))

	require.Error(t, root.Execute())
}
