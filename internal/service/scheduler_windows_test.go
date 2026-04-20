//go:build windows

package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWindowsTaskName(t *testing.T) {
	s := &windowsScheduler{cacheDir: t.TempDir()}
	assert.Equal(t, "sap-devs-sync", s.taskName())
}

func TestWindowsIntervalMinutes(t *testing.T) {
	assert.Equal(t, "360", intervalMinutes(6*time.Hour))
	assert.Equal(t, "60", intervalMinutes(1*time.Hour))
	assert.Equal(t, "1440", intervalMinutes(24*time.Hour))
}
