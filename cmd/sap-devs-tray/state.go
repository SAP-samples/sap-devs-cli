package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"gopkg.in/yaml.v3"
)

type DashboardState struct {
	Version   string       `json:"version"`
	Profile   ProfileState `json:"profile"`
	Sync      SyncState    `json:"sync"`
	Tools     []ToolState  `json:"tools"`
	ServiceUp bool         `json:"serviceUp"`
}

type ProfileState struct {
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	Packs []string `json:"packs"`
}

type SyncState struct {
	LastSynced time.Time `json:"lastSynced"`
	NextSync   time.Time `json:"nextSync"`
	PackCount  int       `json:"packCount"`
	Status     string    `json:"status"`
}

type ToolState struct {
	Name     string `json:"name"`
	Injected bool   `json:"injected"`
}

func ReadState(configDir, cacheDir string) *DashboardState {
	state := &DashboardState{
		Version: version,
		Sync:    readSyncState(cacheDir),
		Profile: readProfile(configDir),
	}
	return state
}

func readSyncState(cacheDir string) SyncState {
	path := filepath.Join(cacheDir, "sync-state.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return SyncState{Status: "unknown"}
	}
	var raw map[string]time.Time
	if err := json.Unmarshal(data, &raw); err != nil {
		return SyncState{Status: "unknown"}
	}
	var latest time.Time
	count := 0
	for _, t := range raw {
		count++
		if t.After(latest) {
			latest = t
		}
	}
	st := SyncState{
		LastSynced: latest,
		PackCount:  count,
		Status:     "up_to_date",
	}
	if time.Since(latest) > 12*time.Hour {
		st.Status = "stale"
	}
	return st
}

func readProfile(configDir string) ProfileState {
	path := filepath.Join(configDir, "profile.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return ProfileState{ID: "unknown"}
	}
	var p struct {
		ID string `yaml:"id"`
	}
	if err := yaml.Unmarshal(data, &p); err != nil || p.ID == "" {
		return ProfileState{ID: "unknown"}
	}
	return ProfileState{ID: p.ID, Name: profileDisplayName(p.ID)}
}

func profileDisplayName(id string) string {
	names := map[string]string{
		"cap-developer":  "CAP Developer",
		"abap-developer": "ABAP Developer",
		"btp-developer":  "BTP Developer",
		"all":            "All Packs",
		"minimal":        "Minimal",
	}
	if name, ok := names[id]; ok {
		return name
	}
	return id
}

func defaultConfigDir() string {
	switch runtime.GOOS {
	case "windows":
		return filepath.Join(os.Getenv("APPDATA"), "sap-devs")
	case "darwin":
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Library", "Application Support", "sap-devs")
	default:
		if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
			return filepath.Join(xdg, "sap-devs")
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".config", "sap-devs")
	}
}

func defaultCacheDir() string {
	switch runtime.GOOS {
	case "windows":
		return filepath.Join(os.Getenv("LOCALAPPDATA"), "sap-devs", "cache")
	case "darwin":
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Library", "Caches", "sap-devs")
	default:
		if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" {
			return filepath.Join(xdg, "sap-devs")
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".cache", "sap-devs")
	}
}
