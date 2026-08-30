// Package cliconfig is the config-file convention every hush-hush binary
// follows (rules/cli.md): a YAML file under XDG_CONFIG_HOME that flags and
// environment variables both override, a starter an init command writes,
// and the interactive nudge to write one when nothing is configured yet.
package cliconfig

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/adrg/xdg"
)

// Path returns the YAML config file path for a named tool under
// XDG_CONFIG_HOME (e.g. "hush-hush-cli" -> ~/.config/hush-hush-cli/config.yaml),
// creating parent directories as needed. xdg.Reload picks up a
// XDG_CONFIG_HOME set after the process started - the adrg/xdg package
// otherwise resolves it once, at init.
func Path(tool string) (string, error) {
	xdg.Reload()

	p, err := xdg.ConfigFile(path.Join(tool, "config.yaml"))
	if err != nil {
		return "", fmt.Errorf("resolve config path: %w", err)
	}

	return p, nil
}

// Exists reports whether a config file already exists at configPath.
func Exists(configPath string) bool {
	_, err := os.Stat(configPath)

	return err == nil
}

// Confirm asks a yes/no question on out, reading the answer from in, and
// defaults to yes on a bare Enter (or EOF) - cli.md's "offers to run init
// right there" is meant to be the path of least resistance, not another
// gate.
func Confirm(in io.Reader, out io.Writer, question string) bool {
	// A failed prompt write doesn't change the answer - Confirm reports a
	// yes/no, not an I/O outcome - so it's ignored rather than plumbed
	// through a signature every caller would then have to handle.
	_, _ = fmt.Fprintf(out, "%s [Y/n] ", question)

	scanner := bufio.NewScanner(in)
	if !scanner.Scan() {
		return true
	}

	switch strings.ToLower(strings.TrimSpace(scanner.Text())) {
	case "", "y", "yes":
		return true
	default:
		return false
	}
}

// ShouldWriteStarter decides whether the "no config file" nudge should
// write one, given:
//
//   - configExists: a config file is already there - never overwrite it
//     unasked.
//   - anyRelevantEnvSet: the tool is already configured through the
//     environment (a CI job, say) - no file needed, no nudge either.
//   - yes: an explicit --yes flag - write unconditionally, no TTY needed.
//   - interactive: stdin is a terminal a prompt can actually use.
//   - confirmed: the answer to that prompt, meaningless when interactive
//     is false.
func ShouldWriteStarter(configExists, anyRelevantEnvSet, yes, interactive, confirmed bool) bool {
	if configExists || anyRelevantEnvSet {
		return false
	}

	if yes {
		return true
	}

	return interactive && confirmed
}
