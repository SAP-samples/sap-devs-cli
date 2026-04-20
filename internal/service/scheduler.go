package service

import (
	"fmt"
	"time"
)

// Status describes the current state of the OS scheduler entry.
type Status struct {
	Installed bool
	Interval  time.Duration
	LastRun   time.Time // zero value if unknown or never run
	NextRun   time.Time // zero value if unknown
}

// Scheduler manages an OS-native scheduled task for background sync+inject.
type Scheduler interface {
	Install(interval time.Duration, binaryPath string) error
	Uninstall() error
	Status() (*Status, error)
}

// New returns the platform-appropriate Scheduler for the given cache directory.
// cacheDir is used for the daemon log path.
func New(cacheDir string) Scheduler {
	return newPlatformScheduler(cacheDir)
}

// ErrNotInstalled is returned when querying status of an uninstalled scheduler.
var ErrNotInstalled = fmt.Errorf("scheduler is not installed")
