package cliconfig_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alrayyes/hush-hush/internal/cliconfig"
	"github.com/stretchr/testify/require"
)

// Path resolves through the adrg/xdg package's own process-wide state,
// which XDG_CONFIG_HOME only refreshes into on an explicit Reload() -
// t.Setenv itself already forbids parallel use, so these two stay
// sequential rather than fighting over that shared state too.
func TestPathIsUnderXDGConfigHome(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	path, err := cliconfig.Path("hush-hush-cli")
	require.NoError(t, err)
	require.Equal(t, filepath.Join(dir, "hush-hush-cli", "config.yaml"), path)
}

func TestPathCreatesTheParentDirectory(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	_, err := cliconfig.Path("hush-hush-cli")
	require.NoError(t, err)

	info, err := os.Stat(filepath.Join(dir, "hush-hush-cli"))
	require.NoError(t, err)
	require.True(t, info.IsDir())
}

func TestExistsIsFalseForAMissingFile(t *testing.T) {
	t.Parallel()

	require.False(t, cliconfig.Exists(filepath.Join(t.TempDir(), "config.yaml")))
}

func TestExistsIsTrueForAnExistingFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("server: x\n"), 0o600))

	require.True(t, cliconfig.Exists(path))
}

func TestConfirmDefaultsToYesOnBareEnter(t *testing.T) {
	t.Parallel()

	require.True(t, cliconfig.Confirm(strings.NewReader("\n"), new(strings.Builder), "?"))
}

func TestConfirmAcceptsYVariants(t *testing.T) {
	t.Parallel()

	for _, answer := range []string{"y\n", "Y\n", "yes\n", "YES\n"} {
		require.True(t, cliconfig.Confirm(strings.NewReader(answer), new(strings.Builder), "?"), "answer %q", answer)
	}
}

func TestConfirmRejectsNVariants(t *testing.T) {
	t.Parallel()

	for _, answer := range []string{"n\n", "N\n", "no\n", "anything-else\n"} {
		require.False(t, cliconfig.Confirm(strings.NewReader(answer), new(strings.Builder), "?"), "answer %q", answer)
	}
}

func TestConfirmWritesTheQuestion(t *testing.T) {
	t.Parallel()

	var out strings.Builder
	cliconfig.Confirm(strings.NewReader("y\n"), &out, "write a config now?")

	require.Contains(t, out.String(), "write a config now?")
}

func TestShouldWriteStarterSkipsWhenAConfigFileAlreadyExists(t *testing.T) {
	t.Parallel()

	require.False(t, cliconfig.ShouldWriteStarter(true, false, false, true, true))
}

func TestShouldWriteStarterSkipsWhenARelevantEnvVarIsSet(t *testing.T) {
	t.Parallel()

	require.False(t, cliconfig.ShouldWriteStarter(false, true, false, true, true))
}

func TestShouldWriteStarterProceedsOnExplicitYesEvenWithoutATTY(t *testing.T) {
	t.Parallel()

	require.True(t, cliconfig.ShouldWriteStarter(false, false, true, false, false))
}

func TestShouldWriteStarterSkipsNonInteractivelyWithNoExplicitYes(t *testing.T) {
	t.Parallel()

	require.False(t, cliconfig.ShouldWriteStarter(false, false, false, false, true))
}

func TestShouldWriteStarterFollowsTheConfirmationInteractively(t *testing.T) {
	t.Parallel()

	require.True(t, cliconfig.ShouldWriteStarter(false, false, false, true, true))
	require.False(t, cliconfig.ShouldWriteStarter(false, false, false, true, false))
}
