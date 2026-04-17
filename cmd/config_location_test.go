package cmd_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.tools.sap/developer-relations/sap-devs-cli/cmd"
)

func TestConfigLocation_SetValue(t *testing.T) {
	skipOnWindows(t)
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	out, err := executeCommand(cmd.RootCmd(), "config", "location", "Berlin, Germany")
	require.NoError(t, err)
	assert.Contains(t, out, "Berlin, Germany")
}

func TestConfigLocation_ShowDisplaysValue(t *testing.T) {
	skipOnWindows(t)
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	_, err := executeCommand(cmd.RootCmd(), "config", "location", "Berlin, Germany")
	require.NoError(t, err)

	out, err := executeCommand(cmd.RootCmd(), "config", "show")
	require.NoError(t, err)
	assert.Contains(t, out, "Berlin, Germany")
}

func TestConfigLocation_ShowNotSet(t *testing.T) {
	skipOnWindows(t)
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	out, err := executeCommand(cmd.RootCmd(), "config", "location")
	require.NoError(t, err)
	assert.Contains(t, out, "(not set)")
}

func TestConfigLocation_DetectWithValueErrors(t *testing.T) {
	skipOnWindows(t)
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	_, err := executeCommand(cmd.RootCmd(), "config", "location", "--detect", "Hamburg, Germany")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot use --detect")
}

func TestConfigLocation_DetectFlagAloneAccepted(t *testing.T) {
	skipOnWindows(t)
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	// --detect alone should not error even when the HTTP call fails (soft failure)
	// In tests, ip-api.com is unreachable or returns quickly; command returns nil with a warning message.
	_, err := executeCommand(cmd.RootCmd(), "config", "location", "--detect")
	require.NoError(t, err)
}
