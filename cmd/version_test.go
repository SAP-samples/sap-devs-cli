// cmd/version_test.go
package cmd_test

import (
	"bytes"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.tools.sap/developer-relations/sap-devs-cli/cmd"
)

// executeVersionCommand runs the version command with extra args and returns
// (stdout, stderr) as separate strings. It uses cobra's SetOut/SetErr so the
// implementation must write via cmd.OutOrStdout() / cmd.ErrOrStderr().
func executeVersionCommand(t *testing.T, args ...string) (stdout, stderr string) {
	t.Helper()
	root := cmd.RootCmd()
	bufOut := new(bytes.Buffer)
	bufErr := new(bytes.Buffer)
	root.SetOut(bufOut)
	root.SetErr(bufErr)
	root.SetArgs(append([]string{"version"}, args...))
	resetFlags(root) // defined in config_token_test.go, same package
	root.Execute()   //nolint:errcheck
	return bufOut.String(), bufErr.String()
}

func TestVersionDefault_DevBuild(t *testing.T) {
	stdout, stderr := executeVersionCommand(t)
	assert.Equal(t, "dev\n", stdout)
	assert.Contains(t, stderr, "dev build")
	assert.Contains(t, stderr, "auto-update is disabled")
}

func TestVersionDefault_RealBuild(t *testing.T) {
	original := cmd.GetVersion()
	cmd.SetVersion("v1.2.3")
	defer cmd.SetVersion(original)

	stdout, stderr := executeVersionCommand(t)
	assert.Equal(t, "v1.2.3\n", stdout)
	assert.Empty(t, stderr, "no hint expected for real builds")
}

func TestVersionVerbose_DevBuild(t *testing.T) {
	stdout, stderr := executeVersionCommand(t, "--verbose")
	assert.Contains(t, stdout, "sap-devs dev")
	assert.Contains(t, stdout, "go:")
	assert.Contains(t, stdout, "os/arch:")
	assert.Contains(t, stderr, "dev build")
	assert.Contains(t, stderr, "auto-update is disabled")
}

func TestVersionVerbose_RealBuild(t *testing.T) {
	original := cmd.GetVersion()
	cmd.SetVersion("v1.2.3")
	defer cmd.SetVersion(original)

	stdout, stderr := executeVersionCommand(t, "--verbose")
	assert.Contains(t, stdout, "sap-devs v1.2.3")
	assert.Contains(t, stdout, "go:")
	assert.Contains(t, stdout, "os/arch:")
	assert.Empty(t, stderr)
}

func TestVersionShortFlag(t *testing.T) {
	stdout, _ := executeVersionCommand(t, "-v")
	assert.True(t, strings.Contains(stdout, "go:"), "-v short flag should produce verbose output")
}

func TestVersionVerbose_GoAndOSPresent(t *testing.T) {
	stdout, _ := executeVersionCommand(t, "--verbose")
	assert.Contains(t, stdout, runtime.Version())
	assert.Contains(t, stdout, runtime.GOOS)
	assert.Contains(t, stdout, runtime.GOARCH)
}
