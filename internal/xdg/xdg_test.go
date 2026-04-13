package xdg_test

import (
	"os"
	"runtime"
	"strings"
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
	if runtime.GOOS != "linux" {
		t.Skip("XDG env vars only honoured on Linux")
	}
	configBase := t.TempDir()
	cacheBase := t.TempDir()
	dataBase := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configBase)
	t.Setenv("XDG_CACHE_HOME", cacheBase)
	t.Setenv("XDG_DATA_HOME", dataBase)
	paths, err := xdg.New()
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(paths.ConfigDir, configBase), "ConfigDir should start with XDG_CONFIG_HOME")
	assert.True(t, strings.HasPrefix(paths.CacheDir, cacheBase), "CacheDir should start with XDG_CACHE_HOME")
	assert.True(t, strings.HasPrefix(paths.DataDir, dataBase), "DataDir should start with XDG_DATA_HOME")
}
