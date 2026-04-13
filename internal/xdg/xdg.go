package xdg

import (
	"os"
	"path/filepath"
	"runtime"
)

const appName = "sap-devs"

// Paths holds platform-native directories for this application.
type Paths struct {
	ConfigDir string // user config: ~/.config/sap-devs (Linux), ~/Library/Application Support/sap-devs (macOS), %APPDATA%/sap-devs (Windows)
	CacheDir  string // cache: ~/.cache/sap-devs (Linux), ~/Library/Caches/sap-devs (macOS), %LOCALAPPDATA%/sap-devs/cache (Windows)
	DataDir   string // data: ~/.local/share/sap-devs (Linux), ~/Library/Application Support/sap-devs/data (macOS), %LOCALAPPDATA%/sap-devs/data (Windows)
}

// New returns platform-appropriate paths for the application.
// On Linux, XDG_CONFIG_HOME, XDG_CACHE_HOME, and XDG_DATA_HOME are honoured.
func New() (*Paths, error) {
	configDir, err := configDir()
	if err != nil {
		return nil, err
	}
	cacheDir, err := cacheDir()
	if err != nil {
		return nil, err
	}
	dataDir, err := dataDir(configDir)
	if err != nil {
		return nil, err
	}
	return &Paths{
		ConfigDir: configDir,
		CacheDir:  cacheDir,
		DataDir:   dataDir,
	}, nil
}

func configDir() (string, error) {
	// On Linux, honour XDG_CONFIG_HOME
	if runtime.GOOS == "linux" {
		if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
			return filepath.Join(xdg, appName), nil
		}
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, appName), nil
}

func cacheDir() (string, error) {
	if runtime.GOOS == "linux" {
		if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" {
			return filepath.Join(xdg, appName), nil
		}
	}
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	// On Windows, UserCacheDir returns %LocalAppData%; add /cache sub-dir for clarity
	if runtime.GOOS == "windows" {
		return filepath.Join(base, appName, "cache"), nil
	}
	return filepath.Join(base, appName), nil
}

func dataDir(configDir string) (string, error) {
	if runtime.GOOS == "linux" {
		if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
			return filepath.Join(xdg, appName), nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".local", "share", appName), nil
	}
	if runtime.GOOS == "windows" {
		base, err := os.UserCacheDir() // %LOCALAPPDATA%
		if err != nil {
			return "", err
		}
		return filepath.Join(base, appName, "data"), nil
	}
	// macOS: ~/Library/Application Support/sap-devs/data
	return filepath.Join(configDir, "data"), nil
}
