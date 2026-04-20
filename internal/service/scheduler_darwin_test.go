//go:build darwin

package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDarwinPlistGeneration(t *testing.T) {
	dir := t.TempDir()
	s := &darwinScheduler{cacheDir: dir}
	plist := s.generatePlist(6*time.Hour, "/usr/local/bin/sap-devs")
	assert.Contains(t, plist, "<integer>21600</integer>")
	assert.Contains(t, plist, "/usr/local/bin/sap-devs")
	assert.Contains(t, plist, "com.sap-devs.sync")
}

func TestDarwinPlistPath(t *testing.T) {
	s := &darwinScheduler{cacheDir: t.TempDir()}
	path := s.plistPath()
	assert.Contains(t, path, "LaunchAgents")
	assert.Contains(t, path, "com.sap-devs.sync.plist")
}
