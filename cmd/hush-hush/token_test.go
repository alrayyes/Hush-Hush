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
func dbPath(t *testing.T) string {
	t.Helper()

	viper.Reset()
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
	wt, token, err := s.CreateWriteToken(t.Context(), "a", time.Hour, "")
	require.NoError(t, err)
	require.NoError(t, s.Close())

	viper.Reset()
	t.Setenv("DB_PATH", path)
	root := newRootCmd()
	root.SetArgs([]string{"token", "revoke", wt.ID})
	require.NoError(t, root.Execute())

	s, err = store.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, s.Close()) })

	valid, err := s.ValidateWriteToken(t.Context(), token)
	require.NoError(t, err)
	require.False(t, valid)
}

func TestTokenRotateInvalidatesTheOldTokenAndPrintsANewOne(t *testing.T) {
	path := dbPath(t)

	s, err := store.Open(path)
	require.NoError(t, err)
	wt, oldToken, err := s.CreateWriteToken(t.Context(), "a", time.Hour, "")
	require.NoError(t, err)
	require.NoError(t, s.Close())

	viper.Reset()
	t.Setenv("DB_PATH", path)
	root := newRootCmd()
	root.SetArgs([]string{"token", "rotate", wt.ID, "--ttl", "2h"})
	var out bytes.Buffer
	root.SetOut(&out)
	require.NoError(t, root.Execute())
	require.Contains(t, out.String(), "token:")

	s, err = store.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, s.Close()) })

	oldValid, err := s.ValidateWriteToken(t.Context(), oldToken)
	require.NoError(t, err)
	require.False(t, oldValid)
}

func TestTokenRotateUnknownIDFails(t *testing.T) {
	dbPath(t)

	root := newRootCmd()
	root.SetArgs([]string{"token", "rotate", "nope", "--ttl", "1h"})
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))

	require.Error(t, root.Execute())
}

func TestTokenRevokeUnknownIDFails(t *testing.T) {
	dbPath(t)

	root := newRootCmd()
	root.SetArgs([]string{"token", "revoke", "nope"})
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))

	require.Error(t, root.Execute())
}

// The local token commands mint straight into the store, so they have to
// hold the same lifetime limit the HTTP API does (alrayyes/hush-hush#537).
func TestTokenIssueRejectsATTLAboveTheMaximum(t *testing.T) {
	path := dbPath(t)

	root := newRootCmd()
	root.SetArgs([]string{"token", "issue", "--description", "too long", "--ttl", "8761h"})
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))

	err := root.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "8760h", "the message should name the limit")

	s, openErr := store.Open(path)
	require.NoError(t, openErr)
	t.Cleanup(func() { require.NoError(t, s.Close()) })
	tokens, listErr := s.ListWriteTokens(t.Context())
	require.NoError(t, listErr)
	require.Empty(t, tokens, "a rejected TTL must mint nothing")
}

func TestTokenIssueAcceptsATTLAtTheMaximum(t *testing.T) {
	dbPath(t)

	root := newRootCmd()
	root.SetArgs([]string{"token", "issue", "--description", "a year", "--ttl", "8760h"})
	root.SetOut(new(bytes.Buffer))
	require.NoError(t, root.Execute())
}

func TestTokenRotateRejectsATTLAboveTheMaximum(t *testing.T) {
	path := dbPath(t)

	s, err := store.Open(path)
	require.NoError(t, err)
	wt, _, err := s.CreateWriteToken(t.Context(), "a", time.Hour, "")
	require.NoError(t, err)
	require.NoError(t, s.Close())

	viper.Reset()
	t.Setenv("DB_PATH", path)
	root := newRootCmd()
	root.SetArgs([]string{"token", "rotate", wt.ID, "--ttl", "8761h"})
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))

	err = root.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "8760h")
}
