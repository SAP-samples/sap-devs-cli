package cmd_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.tools.sap/developer-relations/sap-devs-cli/cmd"
)

// TestInit_NonTTYSkipsTokenPrompt verifies that when stdin is not a TTY
// (term.ReadPassword fails), initCmd.RunE does not error on the token step —
// it prints the fallback note and continues to Step 2.
//
// When run in a test environment stdin is not a TTY, so term.ReadPassword
// will always return an error here, exercising the non-TTY branch.
func TestInit_NonTTYSkipsTokenPrompt(t *testing.T) {
	skipOnWindows(t)
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	out, _ := executeCommand(cmd.RootCmd(), "init")
	// init may fail at sync or inject steps in a test environment — that is OK.
	// The non-TTY note must be printed, confirming Step 1 was handled not silently dropped.
	assert.Contains(t, out, "interactive token input unavailable",
		"init must print the non-TTY fallback note for the token step")
}
