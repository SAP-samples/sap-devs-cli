package trayctl

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBinaryName(t *testing.T) {
	if runtime.GOOS == "windows" {
		assert.Equal(t, "sap-devs-tray.exe", binaryName())
	} else {
		assert.Equal(t, "sap-devs-tray", binaryName())
	}
}

func TestAssetName(t *testing.T) {
	name := assetName("1.2.3")
	assert.Contains(t, name, "sap-devs-tray_1.2.3_")
	assert.Contains(t, name, runtime.GOOS)
	assert.Contains(t, name, runtime.GOARCH)
}

func TestBinDir(t *testing.T) {
	m := &Manager{CacheDir: "/tmp/cache"}
	assert.Equal(t, filepath.Join("/tmp/cache", "bin"), m.binDir())
}

func TestIsInstalled_NotInstalled(t *testing.T) {
	m := &Manager{CacheDir: t.TempDir()}
	assert.False(t, m.IsInstalled())
}
