package cmd_test

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.tools.sap/developer-relations/sap-devs-cli/cmd"
)

// executeCommand runs a cobra command with given args, returns combined output and error.
// It resets all flags to their defaults before each invocation to avoid state leaking
// between test calls on the shared rootCmd singleton.
func executeCommand(root *cobra.Command, args ...string) (string, error) {
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)
	resetFlags(root)
	_, err := root.ExecuteC()
	return buf.String(), err
}

// resetFlags resets all flags in the command tree to their default values.
// Required because cobra reuses the same command instances across test invocations.
func resetFlags(cmd *cobra.Command) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		f.Value.Set(f.DefValue) //nolint:errcheck
		f.Changed = false
	})
	for _, sub := range cmd.Commands() {
		resetFlags(sub)
	}
}

// skipOnWindows skips XDG-dependent tests on Windows (XDG_CONFIG_HOME is not honoured there).
// CI (ubuntu-latest) is the authoritative runner per project convention.
func skipOnWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("XDG_CONFIG_HOME isolation not supported on Windows; run in CI")
	}
}

func TestConfigToken_StoreViaArg(t *testing.T) {
	skipOnWindows(t)
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	out, err := executeCommand(cmd.RootCmd(), "config", "token", "ghp_testtoken")
	require.NoError(t, err)
	assert.Contains(t, out, "Warning: token passed as argument")
	assert.Contains(t, out, "Token stored securely.")
	assert.NotContains(t, out, "ghp_testtoken") // token must never appear in output
}

func TestConfigToken_NonTTYReturnsError(t *testing.T) {
	skipOnWindows(t)
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	// When called with no args in a non-TTY context (test stdin is not a TTY),
	// term.ReadPassword fails and the command should return a non-zero exit with
	// the fallback message — not block silently.
	_, err := executeCommand(cmd.RootCmd(), "config", "token")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "interactive input not available")
	assert.Contains(t, err.Error(), "sap-devs config token <value>")
}

func TestConfigToken_DeleteWhenStored(t *testing.T) {
	skipOnWindows(t)
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	sapDir := filepath.Join(dir, "sap-devs")
	require.NoError(t, os.MkdirAll(sapDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(sapDir, "credentials"), []byte("tok"), 0600))

	out, err := executeCommand(cmd.RootCmd(), "config", "token", "--delete")
	require.NoError(t, err)
	assert.Contains(t, out, "Token removed.")
}

func TestConfigToken_DeleteWhenNothingStored(t *testing.T) {
	skipOnWindows(t)
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "sap-devs"), 0755))

	out, err := executeCommand(cmd.RootCmd(), "config", "token", "--delete")
	require.NoError(t, err)
	assert.Contains(t, out, "No token was stored.")
}

func TestConfigToken_DeleteAndValueMutuallyExclusive(t *testing.T) {
	skipOnWindows(t)
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	_, err := executeCommand(cmd.RootCmd(), "config", "token", "ghp_abc", "--delete")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot use --delete with a token value")
}

func TestConfigShow_MasksToken(t *testing.T) {
	skipOnWindows(t)
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	sapDir := filepath.Join(dir, "sap-devs")
	require.NoError(t, os.MkdirAll(sapDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(sapDir, "credentials"), []byte("ghp_abcdefgh"), 0600))

	out, err := executeCommand(cmd.RootCmd(), "config", "show")
	require.NoError(t, err)
	assert.Contains(t, out, "ghp_****")
	assert.NotContains(t, out, "ghp_abcdefgh")
}

func TestConfigShow_NoTokenDisplaysNotSet(t *testing.T) {
	skipOnWindows(t)
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "sap-devs"), 0755))

	out, err := executeCommand(cmd.RootCmd(), "config", "show")
	require.NoError(t, err)
	assert.Contains(t, out, "(not set)")
}

func TestConfigToken_DeleteThenShowNotSet(t *testing.T) {
	skipOnWindows(t)
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	sapDir := filepath.Join(dir, "sap-devs")
	require.NoError(t, os.MkdirAll(sapDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(sapDir, "credentials"), []byte("tok"), 0600))

	_, err := executeCommand(cmd.RootCmd(), "config", "token", "--delete")
	require.NoError(t, err)

	out, err := executeCommand(cmd.RootCmd(), "config", "show")
	require.NoError(t, err)
	assert.Contains(t, out, "(not set)")
}
