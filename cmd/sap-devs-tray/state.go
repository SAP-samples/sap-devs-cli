package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type DashboardState struct {
	Version string       `json:"version"`
	Profile ProfileState `json:"profile"`
	Sync    SyncState    `json:"sync"`
	Tools   []ToolState  `json:"tools"`
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
	home, _ := os.UserHomeDir()
	cfg := readConfig(configDir)
	syncSt := readSyncState(cacheDir)
	syncSt.NextSync = calcNextSync(syncSt.LastSynced, cfg.serviceInterval())

	state := &DashboardState{
		Version: version,
		Sync:    syncSt,
		Profile: readProfile(configDir, cacheDir),
		Tools:   detectTools(home),
	}
	return state
}

type configFile struct {
	Service struct {
		Interval string `yaml:"interval"`
	} `yaml:"service"`
}

func (c configFile) serviceInterval() time.Duration {
	if c.Service.Interval != "" {
		if d, err := time.ParseDuration(c.Service.Interval); err == nil {
			return d
		}
	}
	return 6 * time.Hour
}

func readConfig(configDir string) configFile {
	data, err := os.ReadFile(filepath.Join(configDir, "config.yaml"))
	if err != nil {
		return configFile{}
	}
	var cfg configFile
	_ = yaml.Unmarshal(data, &cfg)
	return cfg
}

func calcNextSync(lastSynced time.Time, interval time.Duration) time.Time {
	if lastSynced.IsZero() {
		return time.Time{}
	}
	return lastSynced.Add(interval)
}

func readSyncState(cacheDir string) SyncState {
	path := filepath.Join(cacheDir, "sync-state.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return SyncState{Status: "unknown"}
	}
	var raw struct {
		Version    int                  `json:"version"`
		Categories map[string]time.Time `json:"categories"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return SyncState{Status: "unknown"}
	}
	var latest time.Time
	count := 0
	for _, t := range raw.Categories {
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

func readProfile(configDir, cacheDir string) ProfileState {
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
	packs := readProfilePacks(cacheDir, p.ID)
	return ProfileState{ID: p.ID, Name: profileDisplayName(p.ID), Packs: packs}
}

func readProfilePacks(cacheDir, profileID string) []string {
	if profileID == "minimal" {
		return []string{"base"}
	}
	path := filepath.Join(cacheDir, "official", "content", "profiles", profileID+".yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var prof struct {
		Packs []struct {
			ID string `yaml:"id"`
		} `yaml:"packs"`
	}
	if err := yaml.Unmarshal(data, &prof); err != nil {
		return nil
	}
	result := []string{"base"}
	for _, p := range prof.Packs {
		result = append(result, p.ID)
	}
	return result
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

func detectTools(home string) []ToolState {
	tools := []struct {
		name string
		path string
	}{
		{"Claude Code", filepath.Join(home, ".claude", "CLAUDE.md")},
		{"Cursor", filepath.Join(home, ".cursor", "rules", "sap-developer-context.mdc")},
		{"GitHub Copilot", filepath.Join(home, ".github", "copilot-instructions.md")},
		{"Windsurf", filepath.Join(home, ".windsurf", "rules", "sap.md")},
		{"Gemini Code Assist", filepath.Join(home, ".gemini", "system.md")},
	}
	var result []ToolState
	for _, t := range tools {
		injected := fileContains(t.path, "sap-devs")
		result = append(result, ToolState{Name: t.name, Injected: injected})
	}
	return result
}

func fileContains(path, substr string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return strings.Contains(string(data), substr)
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
