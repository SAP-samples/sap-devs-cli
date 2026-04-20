package service

import (
	"time"
)

// Status describes the current state of the OS scheduler entry.
type Status struct {
	Installed bool
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
