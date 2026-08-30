package main

import (
	"bytes"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	hushhush "github.com/alrayyes/hush-hush/internal/api"
	"github.com/alrayyes/hush-hush/internal/store"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestInitWritesAStarterConfigFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	viper.Reset()

	root := newRootCmd()
	root.SetArgs([]string{"init"})
	require.NoError(t, root.Execute())

	content, err := os.ReadFile(filepath.Join(dir, "hush-hush-cli", "config.yaml")) //nolint:gosec // path is built from t.TempDir(), not user input
	require.NoError(t, err)
	require.Contains(t, string(content), "server: http://localhost:8080")
}

func TestInitRefusesToOverwriteAnExistingFileWithoutForce(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	viper.Reset()

	path := filepath.Join(dir, "hush-hush-cli", "config.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte("server: https://edited\n"), 0o600))

	root := newRootCmd()
	root.SetArgs([]string{"init"})
	require.Error(t, root.Execute())

	content, err := os.ReadFile(path) //nolint:gosec // path is built from t.TempDir(), not user input
	require.NoError(t, err)
	require.Equal(t, "server: https://edited\n", string(content))
}

func TestInitForceOverwritesAnExistingFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	viper.Reset()

	path := filepath.Join(dir, "hush-hush-cli", "config.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte("server: https://edited\n"), 0o600))

	root := newRootCmd()
	root.SetArgs([]string{"init", "--force"})
	require.NoError(t, root.Execute())

	content, err := os.ReadFile(path) //nolint:gosec // path is built from t.TempDir(), not user input
	require.NoError(t, err)
	require.Contains(t, string(content), "server: http://localhost:8080")
}

// TestUnconfiguredNonInteractiveRunProceedsOnDefaults confirms the nudge
// never blocks a script or CI job on a prompt nothing will answer: no
// config file, no HUSH_HUSH_* environment variable, and go test's own
// stdin is never a TTY.
func TestUnconfiguredNonInteractiveRunProceedsOnDefaults(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	viper.Reset()

	s, err := store.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, s.Close()) })

	srv := httptest.NewServer(hushhush.NewMux(s))
	t.Cleanup(srv.Close)

	root := newRootCmd()
	root.SetArgs([]string{"get", "nope", "--server", srv.URL})
	var errOut bytes.Buffer
	root.SetErr(&errOut)

	err = root.Execute()
	require.Error(t, err) // unknown object - the command itself still ran
	require.Contains(t, errOut.String(), "no config file")

	require.NoFileExists(t, filepath.Join(dir, "hush-hush-cli", "config.yaml"))
}

// TestYesFlagWritesAStarterConfigWithNoPrompt is rules/cli.md's "an
// explicit -y/--yes... opts into generating the file without asking" -
// the one way to get the starter file written outside an interactive
// terminal.
func TestYesFlagWritesAStarterConfigWithNoPrompt(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	viper.Reset()

	s, err := store.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, s.Close()) })

	srv := httptest.NewServer(hushhush.NewMux(s))
	t.Cleanup(srv.Close)

	root := newRootCmd()
	root.SetArgs([]string{"get", "nope", "--server", srv.URL, "--yes"})

	_ = root.Execute() // unknown object - irrelevant to this test

	require.FileExists(t, filepath.Join(dir, "hush-hush-cli", "config.yaml"))
}

// TestConfigNudgeNeverBlocksARunTheEnvironmentAlreadyConfigures is a
// regression test: an XDG_CONFIG_HOME under a directory this process
// can't write into must not stop a command from running when
// HUSH_HUSH_SERVER already configures everything it needs - the CI job
// this tool's own spec commits to running "unmodified inside" is exactly
// the case that sets environment variables and nothing else, and may not
// have a writable home directory to resolve one against at all.
func TestConfigNudgeNeverBlocksARunTheEnvironmentAlreadyConfigures(t *testing.T) {
	unwritable := t.TempDir()
	require.NoError(t, os.Chmod(unwritable, 0o500))                       //nolint:gosec // G302 checks file perms; this chmod is on a directory
	t.Cleanup(func() { require.NoError(t, os.Chmod(unwritable, 0o700)) }) //nolint:gosec // G302 checks file perms; this chmod is on a directory - TempDir cleanup also needs write back

	t.Setenv("XDG_CONFIG_HOME", filepath.Join(unwritable, "config"))
	viper.Reset()

	s, err := store.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, s.Close()) })

	srv := httptest.NewServer(hushhush.NewMux(s))
	t.Cleanup(srv.Close)

	t.Setenv("HUSH_HUSH_SERVER", srv.URL)

	root := newRootCmd()
	root.SetArgs([]string{"get", "nope"})

	err = root.Execute()
	require.Error(t, err) // unknown object - the command itself still ran
}
