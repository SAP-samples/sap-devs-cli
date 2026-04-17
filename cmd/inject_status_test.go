package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInjectStatus_FlagExists(t *testing.T) {
	require.NotNil(t, injectCmd.Flags().Lookup("status"), "--status flag must be registered")
	require.NotNil(t, injectCmd.Flags().Lookup("json"), "--json flag must be registered")
	require.NotNil(t, injectCmd.Flags().Lookup("verbose"), "--verbose flag must be registered")
}

func TestInjectStatus_MutualExclusion_WithUninstall(t *testing.T) {
	injectStatus = true
	injectUninstall = true
	t.Cleanup(func() {
		injectStatus = false
		injectUninstall = false
	})
	err := injectCmd.RunE(injectCmd, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--status is incompatible")
}

func TestInjectStatus_MutualExclusion_WithSync(t *testing.T) {
	injectStatus = true
	injectSync = true
	t.Cleanup(func() {
		injectStatus = false
		injectSync = false
	})
	err := injectCmd.RunE(injectCmd, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--status is incompatible")
}

func TestInjectStatus_MutualExclusion_WithNoSync(t *testing.T) {
	injectStatus = true
	injectNoSync = true
	t.Cleanup(func() {
		injectStatus = false
		injectNoSync = false
	})
	err := injectCmd.RunE(injectCmd, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--status is incompatible")
}

func TestInjectStatus_MutualExclusion_WithStats(t *testing.T) {
	injectStatus = true
	injectStats = true
	t.Cleanup(func() {
		injectStatus = false
		injectStats = false
	})
	err := injectCmd.RunE(injectCmd, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--status is incompatible")
}

func TestInjectStatus_MutualExclusion_WithDryRun(t *testing.T) {
	injectStatus = true
	injectDryRun = true
	t.Cleanup(func() {
		injectStatus = false
		injectDryRun = false
	})
	err := injectCmd.RunE(injectCmd, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--status is incompatible")
}

func TestInjectStatus_JSONWithoutStatusNoError(t *testing.T) {
	// --json alone (no --status) must not trigger the mutual-exclusion error.
	injectJSON = true
	t.Cleanup(func() { injectJSON = false })
	err := injectCmd.RunE(injectCmd, nil)
	if err != nil {
		assert.NotContains(t, err.Error(), "--status is incompatible")
		assert.NotContains(t, err.Error(), "mutually exclusive")
	}
}
