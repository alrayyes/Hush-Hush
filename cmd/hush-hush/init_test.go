package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

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

	content, err := os.ReadFile(filepath.Join(dir, "hush-hush", "config.yaml")) //nolint:gosec // path is built from t.TempDir(), not user input
	require.NoError(t, err)
	require.Contains(t, string(content), `db_path: "hush-hush.db"`)
}

func TestInitRefusesToOverwriteAnExistingFileWithoutForce(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	viper.Reset()

	path := filepath.Join(dir, "hush-hush", "config.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte("addr: :9090\n"), 0o600))

	root := newRootCmd()
	root.SetArgs([]string{"init"})
	require.Error(t, root.Execute())

	content, err := os.ReadFile(path) //nolint:gosec // path is built from t.TempDir(), not user input
	require.NoError(t, err)
	require.Equal(t, "addr: :9090\n", string(content))
}

func TestInitForceOverwritesAnExistingFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	viper.Reset()

	path := filepath.Join(dir, "hush-hush", "config.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte("addr: :9090\n"), 0o600))

	root := newRootCmd()
	root.SetArgs([]string{"init", "--force"})
	require.NoError(t, root.Execute())

	content, err := os.ReadFile(path) //nolint:gosec // path is built from t.TempDir(), not user input
	require.NoError(t, err)
	require.Contains(t, string(content), `db_path: "hush-hush.db"`)
}

// TestTokenListNonInteractiveWithNoConfigProceedsOnDefaults confirms the
// nudge never blocks a script or CI job on a prompt nothing will answer:
// no config file, no ADDR/DB_PATH, and go test's own stdin is never a TTY.
func TestTokenListNonInteractiveWithNoConfigProceedsOnDefaults(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	// DB_PATH deliberately unset, unlike dbPath(t) - this test is about
	// what happens with nothing configured at all.
	t.Chdir(t.TempDir())
	viper.Reset()

	root := newRootCmd()
	root.SetArgs([]string{"token", "list"})
	var errOut bytes.Buffer
	root.SetErr(&errOut)

	require.NoError(t, root.Execute())
	require.Contains(t, errOut.String(), "no config file")
	require.NoFileExists(t, filepath.Join(dir, "hush-hush", "config.yaml"))
}

// TestYesFlagWritesAStarterConfigWithNoPrompt is rules/cli.md's "an
// explicit -y/--yes... opts into generating the file without asking".
func TestYesFlagWritesAStarterConfigWithNoPrompt(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Chdir(t.TempDir())
	viper.Reset()

	root := newRootCmd()
	root.SetArgs([]string{"token", "list", "--yes"})

	require.NoError(t, root.Execute())
	require.FileExists(t, filepath.Join(dir, "hush-hush", "config.yaml"))
}

// TestConfigNudgeNeverBlocksARunTheEnvironmentAlreadyConfigures is a
// regression test: an XDG_CONFIG_HOME under a directory this process
// can't write into must not stop `token list` from running when
// DB_PATH already configures everything it needs - exactly the shape
// of the published image's --read-only root filesystem, where
// resolving a config path is expected to fail outright.
func TestConfigNudgeNeverBlocksARunTheEnvironmentAlreadyConfigures(t *testing.T) {
	unwritable := t.TempDir()
	require.NoError(t, os.Chmod(unwritable, 0o500))                       //nolint:gosec // G302 checks file perms; this chmod is on a directory
	t.Cleanup(func() { require.NoError(t, os.Chmod(unwritable, 0o700)) }) //nolint:gosec // G302 checks file perms; this chmod is on a directory - TempDir cleanup also needs write back

	t.Setenv("XDG_CONFIG_HOME", filepath.Join(unwritable, "config"))
	t.Setenv("DB_PATH", filepath.Join(t.TempDir(), "hush-hush.db"))
	viper.Reset()

	root := newRootCmd()
	root.SetArgs([]string{"token", "list"})

	require.NoError(t, root.Execute())
}
