package trayctl

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAutostartEntryName(t *testing.T) {
	switch runtime.GOOS {
	case "windows":
		assert.Equal(t, "sap-devs-tray", autostartEntryName())
	case "darwin":
		assert.Equal(t, "com.sap-devs.tray", autostartEntryName())
	case "linux":
		assert.Equal(t, "sap-devs-tray.desktop", autostartEntryName())
	}
}
