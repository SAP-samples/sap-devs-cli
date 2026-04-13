package xdg_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/xdg"
)

func TestNew_ReturnsNonEmptyPaths(t *testing.T) {
	paths, err := xdg.New()
	require.NoError(t, err)
	assert.NotEmpty(t, paths.ConfigDir)
	assert.NotEmpty(t, paths.CacheDir)
	assert.NotEmpty(t, paths.DataDir)
}

func TestNew_PathsContainAppName(t *testing.T) {
	paths, err := xdg.New()
	require.NoError(t, err)
	assert.Contains(t, paths.ConfigDir, "sap-devs")
	assert.Contains(t, paths.CacheDir, "sap-devs")
	assert.Contains(t, paths.DataDir, "sap-devs")
}

func TestNew_XDGEnvOverridesOnLinux(t *testing.T) {
	if os.Getenv("GOOS") == "windows" {
		t.Skip("XDG env vars not honoured on Windows")
	}
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	paths, err := xdg.New()
	require.NoError(t, err)
	assert.Contains(t, paths.ConfigDir, "sap-devs")
}
