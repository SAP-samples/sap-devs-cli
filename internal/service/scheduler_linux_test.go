//go:build linux

package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLinuxServiceUnit(t *testing.T) {
	s := &linuxScheduler{cacheDir: "/tmp/test"}
	unit := s.generateServiceUnit("/usr/local/bin/sap-devs")
	assert.Contains(t, unit, "ExecStart=/bin/sh")
	assert.Contains(t, unit, "/usr/local/bin/sap-devs sync")
	assert.Contains(t, unit, "sap-devs background sync")
}

func TestLinuxTimerUnit(t *testing.T) {
	s := &linuxScheduler{cacheDir: "/tmp/test"}
	timer := s.generateTimerUnit(6 * time.Hour)
	assert.Contains(t, timer, "OnUnitActiveSec=6h0m0s")
	assert.Contains(t, timer, "OnBootSec=5min")
}
