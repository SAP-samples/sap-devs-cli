# GUI Config Editor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a graphical config editor to the sap-devs-tray system tray companion, accessible from the tray context menu and from the dashboard panel.

**Architecture:** A new webview window (520×700, titled, resizable) served by the existing embedded HTTP server. New API endpoints in `config.go` handle config CRUD, city search, language list, location detection, and service/autostart management. A new `config.html` + `config.js` frontend uses SAP Fundamental Styles (Fiori panels, form items, switches) with horizon/dark theme auto-switching. Cities data is copied at build time from the main CLI's `internal/geo/cities.json`.

**Tech Stack:** Go (net/http, yaml.v3, embed), HTML/CSS/JS (SAP Fundamental Styles), Wails v3 alpha webview

**Spec:** `docs/superpowers/specs/2026-04-20-gui-config-editor-design.md`

---

## File Structure

| File | Responsibility |
| --- | --- |
| `cmd/sap-devs-tray/config.go` | **New** — All config API handlers: GET/POST /api/config, GET /api/cities, GET /api/languages, POST /api/detect-location, GET /api/service-status, POST /api/service-install, POST /api/service-uninstall, POST /api/autostart-install, POST /api/autostart-uninstall |
| `cmd/sap-devs-tray/server.go` | **Modify** — Register new config routes on the mux; add `configWindowFunc` field to Server; add `ConfigURL()` method |
| `cmd/sap-devs-tray/app.go` | **Modify** — Create config webview window; add "Config" menu item to tray; wire `srv.configWindowFunc` |
| `cmd/sap-devs-tray/data/cities.json` | **New** — Build-time copy of `internal/geo/cities.json`, embedded via `//go:embed` |
| `cmd/sap-devs-tray/frontend/config.html` | **New** — Config editor page with 5 Fiori panels |
| `cmd/sap-devs-tray/frontend/js/config.js` | **New** — Form population, typeahead, client-side validation, save, service/autostart actions |
| `cmd/sap-devs-tray/frontend/css/app.css` | **Modify** — Add config editor styles (panels, form fields, typeahead, validation, save bar) |
| `cmd/sap-devs-tray/frontend/index.html` | **Modify** — Add "Config" button to dashboard |
| `cmd/sap-devs-tray/frontend/js/app.js` | **Modify** — Add click handler to open config window |
| `build.ps1` | **Modify** — Add cities.json copy step before tray build |

---

### Task 1: Build Script — cities.json Copy Step

**Files:**
- Copy source: `internal/geo/cities.json`
- Copy target: `cmd/sap-devs-tray/data/cities.json`
- Modify: `build.ps1`

- [ ] **Step 1: Create the data directory and copy cities.json**

```bash
mkdir -p cmd/sap-devs-tray/data
cp internal/geo/cities.json cmd/sap-devs-tray/data/cities.json
```

- [ ] **Step 2: Update build.ps1 to copy cities.json before tray build**

In `build.ps1`, inside the `if (Test-Path "cmd\sap-devs-tray\go.mod")` block (line 16), add a copy step **before** the existing `$env:CGO_ENABLED = "1"` line. Insert these lines after `if (Test-Path "cmd\sap-devs-tray\go.mod") {`:

```powershell
    # Copy cities.json for embedding in tray binary
    $citiesSrc = "internal\geo\cities.json"
    $citiesDst = "cmd\sap-devs-tray\data\cities.json"
    if (Test-Path $citiesSrc) {
        New-Item -ItemType Directory -Path (Split-Path $citiesDst) -Force | Out-Null
        Copy-Item $citiesSrc $citiesDst -Force
    }
```

Do **not** change any other lines in the block (keep the existing `-ldflags`, `Push-Location`, etc. as-is).

- [ ] **Step 3: Add data/cities.json to .gitignore**

The copied file should not be committed — it's a build artifact. Add to `.gitignore`:

```
/cmd/sap-devs-tray/data/cities.json
```

But keep `cmd/sap-devs-tray/data/` as a directory by creating a `.gitkeep`:

```bash
touch cmd/sap-devs-tray/data/.gitkeep
```

- [ ] **Step 4: Verify build still works**

Run: `powershell -File build.ps1`

Expected: Both binaries build successfully; `cmd/sap-devs-tray/data/cities.json` exists after build.

- [ ] **Step 5: Commit**

```bash
git add build.ps1 .gitignore cmd/sap-devs-tray/data/.gitkeep
git commit -m "build: copy cities.json into tray binary data dir at build time"
```

---

### Task 2: Config API Handlers — Read, Write, Validate

**Files:**
- Create: `cmd/sap-devs-tray/config.go`
- Modify: `cmd/sap-devs-tray/server.go`

This task creates the core config CRUD endpoints. The tray binary cannot import `internal/config`, so it re-reads/writes `config.yaml` directly using `gopkg.in/yaml.v3` (already a dependency).

- [ ] **Step 1: Create config.go with config data types**

Create `cmd/sap-devs-tray/config.go`. Define the config struct matching `internal/config/config.go` exactly (same YAML tags). This is a necessary duplication since the tray is a separate Go module.

```go
package main

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// trayConfig mirrors internal/config.Config — the tray binary cannot import it.
type trayConfig struct {
	CompanyRepo     string          `yaml:"company_repo,omitempty" json:"company_repo"`
	Language        string          `yaml:"language,omitempty"     json:"language"`
	Location        string          `yaml:"location,omitempty"    json:"location"`
	Sync            traySyncConfig  `yaml:"sync"                  json:"sync"`
	Tip             trayTipConfig   `yaml:"tip,omitempty"         json:"tip"`
	Events          trayEventsConfig `yaml:"events,omitempty"     json:"events"`
	Tutorial        trayTutorialConfig `yaml:"tutorial,omitempty" json:"tutorial"`
	ExperienceLevel string          `yaml:"experience_level,omitempty" json:"experience_level"`
	Service         trayServiceConfig `yaml:"service,omitempty"   json:"service"`
	Tray            trayTrayConfig  `yaml:"tray,omitempty"        json:"tray"`
}

type traySyncConfig struct {
	Tips      time.Duration `yaml:"tips"      json:"tips"`
	Tools     time.Duration `yaml:"tools"     json:"tools"`
	Advocates time.Duration `yaml:"advocates" json:"advocates"`
	Resources time.Duration `yaml:"resources" json:"resources"`
	Context   time.Duration `yaml:"context"   json:"context"`
	MCP       time.Duration `yaml:"mcp"       json:"mcp"`
	Events    time.Duration `yaml:"events"    json:"events"`
	YouTube   time.Duration `yaml:"youtube"   json:"youtube"`
	Discovery time.Duration `yaml:"discovery" json:"discovery"`
	Tutorials time.Duration `yaml:"tutorials" json:"tutorials"`
	Learning  time.Duration `yaml:"learning"  json:"learning"`
	Disabled  bool          `yaml:"disabled"  json:"disabled"`
}

type trayTipConfig struct {
	Rotation string `yaml:"rotation,omitempty" json:"rotation"`
}

type trayEventsConfig struct {
	LocalRadius    int    `yaml:"local_radius,omitempty"    json:"local_radius"`
	RegionalRadius int    `yaml:"regional_radius,omitempty" json:"regional_radius"`
	NotifyDays     int    `yaml:"notify_days,omitempty"     json:"notify_days"`
	NotifyMethod   string `yaml:"notify_method,omitempty"   json:"notify_method"`
}

type trayTutorialConfig struct {
	Interactive bool `yaml:"interactive,omitempty" json:"interactive"`
}

type trayServiceConfig struct {
	Interval time.Duration `yaml:"interval" json:"interval"`
}

type trayTrayConfig struct {
	Autostart bool `yaml:"autostart,omitempty" json:"autostart"`
}
```

- [ ] **Step 2: Add config load/save helpers and defaults**

Append to `config.go`:

```go
func defaultTrayConfig() *trayConfig {
	return &trayConfig{
		Sync: traySyncConfig{
			Tips:      24 * time.Hour,
			Tools:     24 * time.Hour,
			Advocates: 72 * time.Hour,
			Resources: 168 * time.Hour,
			Context:   168 * time.Hour,
			MCP:       168 * time.Hour,
			Events:    4 * time.Hour,
			YouTube:   6 * time.Hour,
			Discovery: 168 * time.Hour,
			Tutorials: 168 * time.Hour,
			Learning:  168 * time.Hour,
		},
		Service: trayServiceConfig{Interval: 6 * time.Hour},
	}
}

func loadTrayConfig(configDir string) (*trayConfig, error) {
	cfg := defaultTrayConfig()
	data, err := os.ReadFile(filepath.Join(configDir, "config.yaml"))
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return nil, err
	}
	return cfg, yaml.Unmarshal(data, cfg)
}

func saveTrayConfig(configDir string, cfg *trayConfig) error {
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(configDir, "config.yaml"), data, 0600)
}
```

- [ ] **Step 3: Add JSON marshaling for time.Duration fields**

The default `json.Marshal` encodes `time.Duration` as nanoseconds (int64), but the frontend needs human-readable strings like `"24h0m0s"`. Add a custom JSON response type that converts durations to strings:

```go
type configJSON struct {
	CompanyRepo     string `json:"company_repo"`
	Language        string `json:"language"`
	Location        string `json:"location"`
	ExperienceLevel string `json:"experience_level"`

	TipRotation         string `json:"tip_rotation"`
	TutorialInteractive bool   `json:"tutorial_interactive"`

	EventsLocalRadius    int    `json:"events_local_radius"`
	EventsRegionalRadius int    `json:"events_regional_radius"`
	EventsNotifyDays     int    `json:"events_notify_days"`
	EventsNotifyMethod   string `json:"events_notify_method"`

	SyncDisabled  bool   `json:"sync_disabled"`
	SyncTips      string `json:"sync_tips"`
	SyncTools     string `json:"sync_tools"`
	SyncAdvocates string `json:"sync_advocates"`
	SyncResources string `json:"sync_resources"`
	SyncContext   string `json:"sync_context"`
	SyncMCP       string `json:"sync_mcp"`
	SyncEvents    string `json:"sync_events"`
	SyncYouTube   string `json:"sync_youtube"`
	SyncDiscovery string `json:"sync_discovery"`
	SyncTutorials string `json:"sync_tutorials"`
	SyncLearning  string `json:"sync_learning"`

	ServiceInterval string `json:"service_interval"`
}

func toConfigJSON(cfg *trayConfig) configJSON {
	tipRot := cfg.Tip.Rotation
	if tipRot == "" {
		tipRot = "daily"
	}
	notifyMethod := cfg.Events.NotifyMethod
	if notifyMethod == "" {
		notifyMethod = "hook"
	}
	localR := cfg.Events.LocalRadius
	if localR <= 0 {
		localR = 200
	}
	regionalR := cfg.Events.RegionalRadius
	if regionalR <= 0 {
		regionalR = 800
	}
	notifyDays := cfg.Events.NotifyDays
	if notifyDays <= 0 {
		notifyDays = 7
	}
	serviceInt := cfg.Service.Interval
	if serviceInt == 0 {
		serviceInt = 6 * time.Hour
	}
	return configJSON{
		CompanyRepo:          cfg.CompanyRepo,
		Language:             cfg.Language,
		Location:             cfg.Location,
		ExperienceLevel:      cfg.ExperienceLevel,
		TipRotation:          tipRot,
		TutorialInteractive:  cfg.Tutorial.Interactive,
		EventsLocalRadius:    localR,
		EventsRegionalRadius: regionalR,
		EventsNotifyDays:     notifyDays,
		EventsNotifyMethod:   notifyMethod,
		SyncDisabled:         cfg.Sync.Disabled,
		SyncTips:             cfg.Sync.Tips.String(),
		SyncTools:            cfg.Sync.Tools.String(),
		SyncAdvocates:        cfg.Sync.Advocates.String(),
		SyncResources:        cfg.Sync.Resources.String(),
		SyncContext:          cfg.Sync.Context.String(),
		SyncMCP:              cfg.Sync.MCP.String(),
		SyncEvents:           cfg.Sync.Events.String(),
		SyncYouTube:          cfg.Sync.YouTube.String(),
		SyncDiscovery:        cfg.Sync.Discovery.String(),
		SyncTutorials:        cfg.Sync.Tutorials.String(),
		SyncLearning:         cfg.Sync.Learning.String(),
		ServiceInterval:      serviceInt.String(),
	}
}
```

- [ ] **Step 4: Add GET /api/config handler**

Append to `config.go`:

```go
func (s *Server) handleConfigGet(w http.ResponseWriter, r *http.Request) {
	cfg, err := loadTrayConfig(s.ConfigDir)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toConfigJSON(cfg))
}
```

- [ ] **Step 5: Add validation logic and POST /api/config handler**

Append to `config.go`:

```go
func (s *Server) handleConfigPost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var input configJSON
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	errors := validateConfigInput(input)
	if len(errors) > 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"errors": errors})
		return
	}

	cfg, err := loadTrayConfig(s.ConfigDir)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	cfg.CompanyRepo = input.CompanyRepo
	cfg.Language = input.Language
	cfg.Location = input.Location
	cfg.ExperienceLevel = input.ExperienceLevel
	cfg.Tip.Rotation = input.TipRotation
	cfg.Tutorial.Interactive = input.TutorialInteractive
	cfg.Events.LocalRadius = input.EventsLocalRadius
	cfg.Events.RegionalRadius = input.EventsRegionalRadius
	cfg.Events.NotifyDays = input.EventsNotifyDays
	cfg.Events.NotifyMethod = input.EventsNotifyMethod
	cfg.Sync.Disabled = input.SyncDisabled
	cfg.Sync.Tips, _ = time.ParseDuration(input.SyncTips)
	cfg.Sync.Tools, _ = time.ParseDuration(input.SyncTools)
	cfg.Sync.Advocates, _ = time.ParseDuration(input.SyncAdvocates)
	cfg.Sync.Resources, _ = time.ParseDuration(input.SyncResources)
	cfg.Sync.Context, _ = time.ParseDuration(input.SyncContext)
	cfg.Sync.MCP, _ = time.ParseDuration(input.SyncMCP)
	cfg.Sync.Events, _ = time.ParseDuration(input.SyncEvents)
	cfg.Sync.YouTube, _ = time.ParseDuration(input.SyncYouTube)
	cfg.Sync.Discovery, _ = time.ParseDuration(input.SyncDiscovery)
	cfg.Sync.Tutorials, _ = time.ParseDuration(input.SyncTutorials)
	cfg.Sync.Learning, _ = time.ParseDuration(input.SyncLearning)
	cfg.Service.Interval, _ = time.ParseDuration(input.ServiceInterval)

	if err := saveTrayConfig(s.ConfigDir, cfg); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func validateConfigInput(input configJSON) map[string]string {
	errs := make(map[string]string)

	if input.CompanyRepo != "" {
		u, err := url.Parse(input.CompanyRepo)
		if err != nil || u.Scheme != "https" || u.Host == "" {
			errs["company_repo"] = "Must be a valid URL (https://...)"
		}
	}

	if input.EventsLocalRadius <= 0 {
		errs["events_local_radius"] = "Must be greater than 0"
	}
	if input.EventsRegionalRadius <= 0 {
		errs["events_regional_radius"] = "Must be greater than 0"
	}
	if input.EventsNotifyDays <= 0 {
		errs["events_notify_days"] = "Must be greater than 0"
	}

	durationFields := map[string]string{
		"sync_tips": input.SyncTips, "sync_tools": input.SyncTools,
		"sync_advocates": input.SyncAdvocates, "sync_resources": input.SyncResources,
		"sync_context": input.SyncContext, "sync_mcp": input.SyncMCP,
		"sync_events": input.SyncEvents, "sync_youtube": input.SyncYouTube,
		"sync_discovery": input.SyncDiscovery, "sync_tutorials": input.SyncTutorials,
		"sync_learning": input.SyncLearning, "service_interval": input.ServiceInterval,
	}
	for field, val := range durationFields {
		if val == "" {
			errs[field] = "Duration required (e.g. 24h, 168h)"
			continue
		}
		if _, err := time.ParseDuration(val); err != nil {
			errs[field] = "Invalid duration format"
		}
	}

	return errs
}
```

- [ ] **Step 6: Add config route dispatcher to handleConfig**

The spec uses a single `/api/config` path for both GET and POST. Add a dispatcher:

```go
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleConfigGet(w, r)
	case http.MethodPost:
		s.handleConfigPost(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
```

- [ ] **Step 7: Register config route in server.go**

In `server.go`, add inside `NewServer()` after the existing route registrations (after line 54):

```go
s.mux.HandleFunc("/api/config", s.requireToken(s.handleConfig))
```

- [ ] **Step 8: Verify build**

Run: `powershell -File build.ps1`

Expected: Both binaries build successfully.

- [ ] **Step 9: Commit**

```bash
git add cmd/sap-devs-tray/config.go cmd/sap-devs-tray/server.go
git commit -m "feat(tray): add config read/write API with validation"
```

---

### Task 3: Input Assistance APIs — Cities, Languages, Detect Location

**Files:**
- Modify: `cmd/sap-devs-tray/config.go`
- Modify: `cmd/sap-devs-tray/server.go`

- [ ] **Step 1: Add cities data embedding and search handler**

Add to the top of `config.go`, alongside the existing imports:

```go
import (
	_ "embed"
	// ... existing imports
	"strings"
)

//go:embed data/cities.json
var citiesData []byte

type cityEntry struct {
	Name    string  `json:"name"`
	Country string  `json:"country"`
	Lat     float64 `json:"lat"`
	Lon     float64 `json:"lon"`
}

var citiesCache []cityEntry

func loadCities() []cityEntry {
	if citiesCache != nil {
		return citiesCache
	}
	_ = json.Unmarshal(citiesData, &citiesCache)
	return citiesCache
}

func (s *Server) handleCities(w http.ResponseWriter, r *http.Request) {
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	if q == "" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]cityEntry{})
		return
	}

	cities := loadCities()
	var matches []cityEntry
	for _, c := range cities {
		if strings.HasPrefix(strings.ToLower(c.Name), q) {
			matches = append(matches, c)
			if len(matches) >= 10 {
				break
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(matches)
}
```

- [ ] **Step 2: Add languages handler**

The tray binary cannot import `internal/i18n`. The supported languages are discovered from the i18n catalog filenames: `en`, `de`, `es`, `fr`, `ja`, `pt-br`. Hardcode these in the tray binary — the list changes very rarely.

```go
var supportedLanguages = []map[string]string{
	{"code": "", "label": "(auto-detect from OS)"},
	{"code": "en", "label": "English"},
	{"code": "de", "label": "Deutsch"},
	{"code": "es", "label": "Español"},
	{"code": "fr", "label": "Français"},
	{"code": "ja", "label": "日本語"},
	{"code": "pt-br", "label": "Português (BR)"},
}

func (s *Server) handleLanguages(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(supportedLanguages)
}
```

- [ ] **Step 3: Add detect-location handler**

```go
func (s *Server) handleDetectLocation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://ip-api.com/json")
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		json.NewEncoder(w).Encode(map[string]string{"error": "Could not detect location"})
		return
	}
	defer resp.Body.Close()

	var result struct {
		City    string `json:"city"`
		Country string `json:"country"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil || result.City == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		json.NewEncoder(w).Encode(map[string]string{"error": "Could not parse location response"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"city":    result.City,
		"country": result.Country,
	})
}
```

- [ ] **Step 4: Register routes in server.go**

Add to `NewServer()` after the `/api/config` route:

```go
s.mux.HandleFunc("/api/cities", s.requireToken(s.handleCities))
s.mux.HandleFunc("/api/languages", s.requireToken(s.handleLanguages))
s.mux.HandleFunc("/api/detect-location", s.requireToken(s.handleDetectLocation))
```

- [ ] **Step 5: Verify build**

Run: `powershell -File build.ps1`

Expected: Both binaries build successfully.

- [ ] **Step 6: Commit**

```bash
git add cmd/sap-devs-tray/config.go cmd/sap-devs-tray/server.go
git commit -m "feat(tray): add cities search, languages list, and location detect APIs"
```

---

### Task 4: Service & Autostart Status/Action APIs

**Files:**
- Modify: `cmd/sap-devs-tray/config.go`
- Modify: `cmd/sap-devs-tray/server.go`

Service install/uninstall run `sap-devs service install` and `sap-devs service uninstall` as subprocesses — same pattern as sync/inject in `server.go`. Autostart is handled by checking the OS-specific autostart entry (registry key on Windows, LaunchAgent plist on macOS, .desktop file on Linux).

- [ ] **Step 1: Add service-status handler**

The handler checks whether the scheduler is installed by running `sap-devs service status` and parsing the output. It also checks autostart by looking for the platform-specific entry.

First, add these imports to the import block in `config.go` (alongside the existing ones):

```go
"fmt"
"os/exec"
"runtime"
```

Then add the handler code:

```go

type serviceStatusResponse struct {
	Scheduler struct {
		Installed bool   `json:"installed"`
		LastRun   string `json:"last_run,omitempty"`
		NextRun   string `json:"next_run,omitempty"`
	} `json:"scheduler"`
	Autostart struct {
		Installed bool `json:"installed"`
	} `json:"autostart"`
}

func (s *Server) handleServiceStatus(w http.ResponseWriter, r *http.Request) {
	resp := serviceStatusResponse{}

	// Check scheduler by running sap-devs service status
	out, err := exec.Command(sapDevsBinary(), "service", "status").Output()
	if err == nil {
		resp.Scheduler.Installed = strings.Contains(string(out), "installed")
	}

	// Check autostart by looking for platform-specific entry
	resp.Autostart.Installed = autostartInstalled()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func autostartInstalled() bool {
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "windows":
		out, err := exec.Command("reg", "query",
			`HKCU\Software\Microsoft\Windows\CurrentVersion\Run`,
			"/v", "sap-devs-tray").Output()
		return err == nil && strings.Contains(string(out), "sap-devs-tray")
	case "darwin":
		path := filepath.Join(home, "Library", "LaunchAgents", "com.sap-devs.tray.plist")
		_, err := os.Stat(path)
		return err == nil
	default:
		path := filepath.Join(home, ".config", "autostart", "sap-devs-tray.desktop")
		_, err := os.Stat(path)
		return err == nil
	}
}
```

- [ ] **Step 2: Add service install/uninstall handlers**

```go
func (s *Server) handleServiceInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	out, err := exec.Command(sapDevsBinary(), "service", "install").CombinedOutput()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": strings.TrimSpace(string(out))})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleServiceUninstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	out, err := exec.Command(sapDevsBinary(), "service", "uninstall").CombinedOutput()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": strings.TrimSpace(string(out))})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
```

- [ ] **Step 3: Add autostart install/uninstall handlers**

Autostart is managed by the tray binary directly (registry, plist, .desktop file) — same logic as `internal/trayctl/autostart.go` but inline since tray can't import it.

```go
func (s *Server) handleAutostartInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	selfPath, err := os.Executable()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "could not determine binary path"})
		return
	}

	if err := registerAutostart(selfPath); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleAutostartUninstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := unregisterAutostart(); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func registerAutostart(binaryPath string) error {
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "windows":
		return exec.Command("reg", "add",
			`HKCU\Software\Microsoft\Windows\CurrentVersion\Run`,
			"/v", "sap-devs-tray", "/t", "REG_SZ", "/d", binaryPath, "/f").Run()
	case "darwin":
		plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>com.sap-devs.tray</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
</dict>
</plist>`, binaryPath)
		path := filepath.Join(home, "Library", "LaunchAgents", "com.sap-devs.tray.plist")
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		return os.WriteFile(path, []byte(plist), 0644)
	default:
		entry := fmt.Sprintf("[Desktop Entry]\nType=Application\nName=sap-devs Tray\nExec=%s\nTerminal=false\nStartupNotify=false\nX-GNOME-Autostart-enabled=true\n", binaryPath)
		dir := filepath.Join(home, ".config", "autostart")
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dir, "sap-devs-tray.desktop"), []byte(entry), 0644)
	}
}

func unregisterAutostart() error {
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "windows":
		return exec.Command("reg", "delete",
			`HKCU\Software\Microsoft\Windows\CurrentVersion\Run`,
			"/v", "sap-devs-tray", "/f").Run()
	case "darwin":
		path := filepath.Join(home, "Library", "LaunchAgents", "com.sap-devs.tray.plist")
		_ = exec.Command("launchctl", "unload", path).Run()
		return os.Remove(path)
	default:
		return os.Remove(filepath.Join(home, ".config", "autostart", "sap-devs-tray.desktop"))
	}
}
```

- [ ] **Step 4: Register all service/autostart routes in server.go**

Add to `NewServer()`:

```go
s.mux.HandleFunc("/api/service-status", s.requireToken(s.handleServiceStatus))
s.mux.HandleFunc("/api/service-install", s.requireToken(s.handleServiceInstall))
s.mux.HandleFunc("/api/service-uninstall", s.requireToken(s.handleServiceUninstall))
s.mux.HandleFunc("/api/autostart-install", s.requireToken(s.handleAutostartInstall))
s.mux.HandleFunc("/api/autostart-uninstall", s.requireToken(s.handleAutostartUninstall))
```

- [ ] **Step 5: Verify build**

Run: `powershell -File build.ps1`

Expected: Both binaries build successfully.

- [ ] **Step 6: Commit**

```bash
git add cmd/sap-devs-tray/config.go cmd/sap-devs-tray/server.go
git commit -m "feat(tray): add service status, install/uninstall, and autostart APIs"
```

---

### Task 5: Config Editor Frontend — HTML

**Files:**
- Create: `cmd/sap-devs-tray/frontend/config.html`

- [ ] **Step 1: Create config.html**

Create `cmd/sap-devs-tray/frontend/config.html`. This page has the same CSS imports as `index.html` (Fundamental Styles + app.css), plus five collapsible Fiori panels and a sticky save bar. All panels expanded by default.

```html
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>sap-devs Configuration</title>
    <link rel="stylesheet" href="css/sap_horizon.css" media="(prefers-color-scheme: light)">
    <link rel="stylesheet" href="css/sap_horizon_dark.css" media="(prefers-color-scheme: dark)">
    <link rel="stylesheet" href="css/fundamental-styles.min.css">
    <link rel="stylesheet" href="css/app.css">
</head>
<body class="config-editor">
    <div class="config-header">
        <span class="config-title">sap-devs Configuration</span>
    </div>

    <div class="config-form" id="config-form">

        <!-- General Panel -->
        <div class="cfg-panel" data-panel="general">
            <div class="cfg-panel-header" onclick="togglePanel(this)">
                <span class="cfg-chevron">&#9660;</span>
                <span class="cfg-panel-title">General</span>
            </div>
            <div class="cfg-panel-body">
                <div class="cfg-field">
                    <label for="cfg-language">Language</label>
                    <select id="cfg-language" class="fd-input"></select>
                </div>
                <div class="cfg-field">
                    <label for="cfg-location">Location</label>
                    <div class="cfg-location-row">
                        <div class="cfg-typeahead-wrap">
                            <input type="text" id="cfg-location" class="fd-input" placeholder="City, Country" autocomplete="off">
                            <div class="cfg-typeahead-list hidden" id="cfg-location-list"></div>
                        </div>
                        <button type="button" class="fd-button" id="btn-detect-location">Detect</button>
                    </div>
                    <span class="cfg-hint">Type to search cities, or click Detect for IP-based lookup</span>
                    <span class="cfg-warning hidden" id="cfg-location-warning">Location not found in city database — event filtering may not work.</span>
                </div>
                <div class="cfg-field">
                    <label for="cfg-experience">Experience Level</label>
                    <select id="cfg-experience" class="fd-input">
                        <option value="">(not set)</option>
                        <option value="beginner">Beginner</option>
                        <option value="intermediate">Intermediate</option>
                        <option value="advanced">Advanced</option>
                    </select>
                </div>
                <div class="cfg-field">
                    <label for="cfg-company-repo">Company Repo</label>
                    <input type="text" id="cfg-company-repo" class="fd-input" placeholder="https://github.com/...">
                    <span class="cfg-error hidden" id="cfg-company-repo-error"></span>
                </div>
            </div>
        </div>

        <!-- Preferences Panel -->
        <div class="cfg-panel" data-panel="preferences">
            <div class="cfg-panel-header" onclick="togglePanel(this)">
                <span class="cfg-chevron">&#9660;</span>
                <span class="cfg-panel-title">Preferences</span>
            </div>
            <div class="cfg-panel-body">
                <div class="cfg-field">
                    <label for="cfg-tip-rotation">Tip Rotation</label>
                    <select id="cfg-tip-rotation" class="fd-input">
                        <option value="daily">Daily</option>
                        <option value="hourly">Hourly</option>
                        <option value="session">Session</option>
                    </select>
                </div>
                <div class="cfg-field cfg-switch-row">
                    <label class="cfg-switch-label">
                        <input type="checkbox" id="cfg-tutorial-interactive">
                        <span class="cfg-switch-track"><span class="cfg-switch-thumb"></span></span>
                        <span class="cfg-switch-text">Interactive Tutorials</span>
                    </label>
                    <span class="cfg-hint">Open tutorials in step-by-step TUI mode by default</span>
                </div>
            </div>
        </div>

        <!-- Events Panel -->
        <div class="cfg-panel" data-panel="events">
            <div class="cfg-panel-header" onclick="togglePanel(this)">
                <span class="cfg-chevron">&#9660;</span>
                <span class="cfg-panel-title">Events</span>
            </div>
            <div class="cfg-panel-body">
                <div class="cfg-row-2col">
                    <div class="cfg-field">
                        <label for="cfg-local-radius">Local Radius</label>
                        <div class="cfg-suffix-input">
                            <input type="number" id="cfg-local-radius" class="fd-input" min="1">
                            <span class="cfg-suffix">km</span>
                        </div>
                        <span class="cfg-error hidden" id="cfg-local-radius-error"></span>
                    </div>
                    <div class="cfg-field">
                        <label for="cfg-regional-radius">Regional Radius</label>
                        <div class="cfg-suffix-input">
                            <input type="number" id="cfg-regional-radius" class="fd-input" min="1">
                            <span class="cfg-suffix">km</span>
                        </div>
                        <span class="cfg-error hidden" id="cfg-regional-radius-error"></span>
                    </div>
                </div>
                <div class="cfg-row-2col">
                    <div class="cfg-field">
                        <label for="cfg-notify-days">Notify Days</label>
                        <input type="number" id="cfg-notify-days" class="fd-input" min="1">
                        <span class="cfg-error hidden" id="cfg-notify-days-error"></span>
                    </div>
                    <div class="cfg-field">
                        <label for="cfg-notify-method">Notify Method</label>
                        <select id="cfg-notify-method" class="fd-input">
                            <option value="hook">Hook</option>
                            <option value="os">OS notification</option>
                            <option value="both">Both</option>
                        </select>
                    </div>
                </div>
            </div>
        </div>

        <!-- Sync TTLs Panel -->
        <div class="cfg-panel" data-panel="sync">
            <div class="cfg-panel-header" onclick="togglePanel(this)">
                <span class="cfg-chevron">&#9660;</span>
                <span class="cfg-panel-title">Sync TTLs</span>
            </div>
            <div class="cfg-panel-body">
                <div class="cfg-field cfg-switch-row">
                    <label class="cfg-switch-label">
                        <input type="checkbox" id="cfg-sync-disabled">
                        <span class="cfg-switch-track"><span class="cfg-switch-thumb"></span></span>
                        <span class="cfg-switch-text">Disable All Sync</span>
                    </label>
                </div>
                <div class="cfg-grid-2col" id="cfg-sync-fields">
                    <div class="cfg-field">
                        <label for="cfg-sync-tips">Tips</label>
                        <input type="text" id="cfg-sync-tips" class="fd-input">
                        <span class="cfg-error hidden" id="cfg-sync-tips-error"></span>
                    </div>
                    <div class="cfg-field">
                        <label for="cfg-sync-tools">Tools</label>
                        <input type="text" id="cfg-sync-tools" class="fd-input">
                        <span class="cfg-error hidden" id="cfg-sync-tools-error"></span>
                    </div>
                    <div class="cfg-field">
                        <label for="cfg-sync-resources">Resources</label>
                        <input type="text" id="cfg-sync-resources" class="fd-input">
                        <span class="cfg-error hidden" id="cfg-sync-resources-error"></span>
                    </div>
                    <div class="cfg-field">
                        <label for="cfg-sync-context">Context</label>
                        <input type="text" id="cfg-sync-context" class="fd-input">
                        <span class="cfg-error hidden" id="cfg-sync-context-error"></span>
                    </div>
                    <div class="cfg-field">
                        <label for="cfg-sync-events">Events</label>
                        <input type="text" id="cfg-sync-events" class="fd-input">
                        <span class="cfg-error hidden" id="cfg-sync-events-error"></span>
                    </div>
                    <div class="cfg-field">
                        <label for="cfg-sync-youtube">YouTube</label>
                        <input type="text" id="cfg-sync-youtube" class="fd-input">
                        <span class="cfg-error hidden" id="cfg-sync-youtube-error"></span>
                    </div>
                    <div class="cfg-field">
                        <label for="cfg-sync-discovery">Discovery</label>
                        <input type="text" id="cfg-sync-discovery" class="fd-input">
                        <span class="cfg-error hidden" id="cfg-sync-discovery-error"></span>
                    </div>
                    <div class="cfg-field">
                        <label for="cfg-sync-tutorials">Tutorials</label>
                        <input type="text" id="cfg-sync-tutorials" class="fd-input">
                        <span class="cfg-error hidden" id="cfg-sync-tutorials-error"></span>
                    </div>
                    <div class="cfg-field">
                        <label for="cfg-sync-advocates">Advocates</label>
                        <input type="text" id="cfg-sync-advocates" class="fd-input">
                        <span class="cfg-error hidden" id="cfg-sync-advocates-error"></span>
                    </div>
                    <div class="cfg-field">
                        <label for="cfg-sync-mcp">MCP</label>
                        <input type="text" id="cfg-sync-mcp" class="fd-input">
                        <span class="cfg-error hidden" id="cfg-sync-mcp-error"></span>
                    </div>
                    <div class="cfg-field">
                        <label for="cfg-sync-learning">Learning</label>
                        <input type="text" id="cfg-sync-learning" class="fd-input">
                        <span class="cfg-error hidden" id="cfg-sync-learning-error"></span>
                    </div>
                </div>
                <span class="cfg-hint">Go duration format: e.g. 24h, 168h, 4h30m</span>
            </div>
        </div>

        <!-- Service & Tray Panel -->
        <div class="cfg-panel" data-panel="service">
            <div class="cfg-panel-header" onclick="togglePanel(this)">
                <span class="cfg-chevron">&#9660;</span>
                <span class="cfg-panel-title">Service &amp; Tray</span>
            </div>
            <div class="cfg-panel-body">
                <div class="cfg-service-section" id="scheduler-section">
                    <div class="cfg-service-header">
                        <div>
                            <span class="cfg-service-title">Background Scheduler</span>
                            <span class="cfg-service-desc">Runs sync + inject automatically on a schedule</span>
                        </div>
                        <span class="cfg-badge" id="scheduler-badge">Loading...</span>
                    </div>
                    <div id="scheduler-installed" class="hidden">
                        <div class="cfg-field" style="margin-bottom:10px">
                            <label for="cfg-service-interval">Interval</label>
                            <div style="display:flex;align-items:center;gap:8px">
                                <input type="text" id="cfg-service-interval" class="fd-input" style="width:120px">
                                <span class="cfg-hint" style="margin:0">Go duration format</span>
                            </div>
                            <span class="cfg-error hidden" id="cfg-service-interval-error"></span>
                        </div>
                        <button type="button" class="fd-button cfg-btn-danger" id="btn-scheduler-uninstall">Uninstall Scheduler</button>
                    </div>
                    <div id="scheduler-not-installed" class="hidden">
                        <button type="button" class="fd-button fd-button--emphasized" id="btn-scheduler-install">Install Scheduler</button>
                    </div>
                </div>

                <div class="cfg-divider"></div>

                <div class="cfg-service-section" id="autostart-section">
                    <div class="cfg-service-header">
                        <div>
                            <span class="cfg-service-title">Tray Autostart</span>
                            <span class="cfg-service-desc">Launch tray companion at OS login</span>
                        </div>
                        <span class="cfg-badge" id="autostart-badge">Loading...</span>
                    </div>
                    <div id="autostart-installed" class="hidden">
                        <button type="button" class="fd-button cfg-btn-danger" id="btn-autostart-uninstall">Uninstall Autostart</button>
                    </div>
                    <div id="autostart-not-installed" class="hidden">
                        <button type="button" class="fd-button fd-button--emphasized" id="btn-autostart-install">Install Autostart</button>
                    </div>
                </div>
            </div>
        </div>

    </div>

    <!-- Sticky Save Bar -->
    <div class="cfg-save-bar">
        <span class="cfg-save-status hidden" id="cfg-save-status"></span>
        <button type="button" class="fd-button fd-button--emphasized" id="btn-save">Save</button>
    </div>

    <script src="js/config.js"></script>
</body>
</html>
```

- [ ] **Step 2: Verify build (the HTML is embedded by go:embed frontend)**

Run: `powershell -File build.ps1`

Expected: Both binaries build successfully. The new `config.html` is included in the embedded filesystem.

- [ ] **Step 3: Commit**

```bash
git add cmd/sap-devs-tray/frontend/config.html
git commit -m "feat(tray): add config editor HTML with Fiori panel layout"
```

---

### Task 6: Config Editor Frontend — CSS

**Files:**
- Modify: `cmd/sap-devs-tray/frontend/css/app.css`

- [ ] **Step 1: Add config editor styles to app.css**

Append these styles at the end of `cmd/sap-devs-tray/frontend/css/app.css`:

```css
/* ── Config Editor ── */

body.config-editor {
    width: auto;
    max-height: none;
    min-height: 100vh;
    display: flex;
    flex-direction: column;
}

.config-header {
    padding: 14px 20px;
    background: var(--sapBrandColor, #0a6ed1);
    color: #fff;
}

.config-title {
    font-size: 15px;
    font-weight: 700;
}

.config-form {
    flex: 1;
    overflow-y: auto;
    padding-bottom: 60px;
}

/* Panels */
.cfg-panel {
    background: var(--sapGroup_ContentBackground, #fff);
    border-bottom: 1px solid var(--sapGroup_ContentBorderColor, #d9d9d9);
}

.cfg-panel-header {
    display: flex;
    align-items: center;
    padding: 12px 16px;
    cursor: pointer;
    gap: 8px;
    user-select: none;
}

.cfg-panel-header:hover {
    background: var(--sapList_Hover_Background, #f5f6f7);
}

.cfg-chevron {
    font-size: 11px;
    color: var(--sapContent_LabelColor, #6a6d70);
    transition: transform 0.15s;
}

.cfg-panel.collapsed .cfg-chevron {
    transform: rotate(-90deg);
}

.cfg-panel.collapsed .cfg-panel-body {
    display: none;
}

.cfg-panel-title {
    font-size: 14px;
    font-weight: 600;
    color: var(--sapTextColor, #32363a);
}

.cfg-panel-body {
    padding: 0 16px 16px;
}

/* Fields */
.cfg-field {
    margin-bottom: 14px;
}

.cfg-field:last-child {
    margin-bottom: 0;
}

.cfg-field label {
    display: block;
    font-size: 12px;
    color: var(--sapContent_LabelColor, #6a6d70);
    margin-bottom: 4px;
}

.cfg-field .fd-input,
.cfg-field select.fd-input {
    width: 100%;
    box-sizing: border-box;
    padding: 6px 10px;
    border: 1px solid var(--sapField_BorderColor, #89919a);
    border-radius: 4px;
    font-size: 13px;
    font-family: inherit;
    background: var(--sapField_Background, #fff);
    color: var(--sapField_TextColor, #32363a);
}

.cfg-field .fd-input:focus,
.cfg-field select.fd-input:focus {
    outline: none;
    border-color: var(--sapField_Active_BorderColor, #0a6ed1);
    box-shadow: 0 0 0 1px var(--sapField_Active_BorderColor, #0a6ed1);
}

.cfg-field .fd-input.has-error {
    border-color: var(--sapNegativeColor, #bb0000);
}

.cfg-hint {
    display: block;
    font-size: 11px;
    color: var(--sapContent_LabelColor, #6a6d70);
    margin-top: 4px;
}

.cfg-error {
    display: block;
    font-size: 11px;
    color: var(--sapNegativeColor, #bb0000);
    margin-top: 2px;
}

.cfg-warning {
    display: block;
    font-size: 11px;
    color: var(--sapCriticalColor, #e76500);
    margin-top: 2px;
}

/* Location row */
.cfg-location-row {
    display: flex;
    gap: 8px;
    align-items: start;
}

.cfg-typeahead-wrap {
    flex: 1;
    position: relative;
}

.cfg-typeahead-list {
    position: absolute;
    top: 100%;
    left: 0;
    right: 0;
    background: var(--sapGroup_ContentBackground, #fff);
    border: 1px solid var(--sapField_BorderColor, #89919a);
    border-top: none;
    border-radius: 0 0 4px 4px;
    box-shadow: 0 4px 8px rgba(0,0,0,0.12);
    z-index: 10;
    max-height: 200px;
    overflow-y: auto;
}

.cfg-typeahead-item {
    padding: 8px 10px;
    font-size: 13px;
    cursor: pointer;
    color: var(--sapTextColor, #32363a);
}

.cfg-typeahead-item:hover,
.cfg-typeahead-item.highlighted {
    background: var(--sapList_Hover_Background, #e8f0fe);
}

/* Suffix input (e.g. "km") */
.cfg-suffix-input {
    display: flex;
    align-items: center;
    gap: 6px;
}

.cfg-suffix-input .fd-input {
    flex: 1;
}

.cfg-suffix {
    font-size: 12px;
    color: var(--sapContent_LabelColor, #6a6d70);
    flex-shrink: 0;
}

/* 2-column layouts */
.cfg-row-2col {
    display: flex;
    gap: 12px;
    margin-bottom: 14px;
}

.cfg-row-2col .cfg-field {
    flex: 1;
    margin-bottom: 0;
}

.cfg-grid-2col {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 10px;
    margin-bottom: 6px;
}

.cfg-grid-2col .cfg-field {
    margin-bottom: 0;
}

/* Switch (toggle) */
.cfg-switch-row {
    margin-bottom: 14px;
}

.cfg-switch-label {
    display: flex;
    align-items: center;
    gap: 10px;
    cursor: pointer;
}

.cfg-switch-label input[type="checkbox"] {
    display: none;
}

.cfg-switch-track {
    width: 36px;
    height: 20px;
    background: var(--sapNeutralBackground, #bcc3ca);
    border-radius: 10px;
    position: relative;
    flex-shrink: 0;
    transition: background 0.15s;
}

.cfg-switch-label input:checked + .cfg-switch-track {
    background: var(--sapBrandColor, #0a6ed1);
}

.cfg-switch-thumb {
    width: 16px;
    height: 16px;
    background: #fff;
    border-radius: 50%;
    position: absolute;
    left: 2px;
    top: 2px;
    transition: left 0.15s;
}

.cfg-switch-label input:checked + .cfg-switch-track .cfg-switch-thumb {
    left: 18px;
}

.cfg-switch-text {
    font-size: 13px;
    color: var(--sapTextColor, #32363a);
}

/* Service & Tray */
.cfg-service-section {
    margin-bottom: 4px;
}

.cfg-service-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 10px;
}

.cfg-service-title {
    display: block;
    font-size: 13px;
    font-weight: 600;
    color: var(--sapTextColor, #32363a);
}

.cfg-service-desc {
    display: block;
    font-size: 11px;
    color: var(--sapContent_LabelColor, #6a6d70);
}

.cfg-badge {
    display: inline-block;
    padding: 3px 10px;
    border-radius: 4px;
    font-size: 12px;
    font-weight: 600;
    white-space: nowrap;
}

.cfg-badge.installed {
    color: #fff;
    background: var(--sapPositiveColor, #256f3a);
}

.cfg-badge.not-installed {
    color: var(--sapNeutralColor, #6a6d70);
    background: var(--sapNeutralBackground, #eaecee);
}

.cfg-btn-danger {
    border-color: var(--sapNegativeColor, #bb0000) !important;
    color: var(--sapNegativeColor, #bb0000) !important;
    background: transparent !important;
}

.cfg-btn-danger:hover {
    background: rgba(187, 0, 0, 0.06) !important;
}

.cfg-divider {
    height: 1px;
    background: var(--sapGroup_ContentBorderColor, #d9d9d9);
    margin: 16px 0;
}

/* Sticky Save Bar */
.cfg-save-bar {
    position: sticky;
    bottom: 0;
    padding: 12px 16px;
    background: var(--sapGroup_ContentBackground, #fff);
    border-top: 2px solid var(--sapGroup_ContentBorderColor, #d9d9d9);
    display: flex;
    justify-content: flex-end;
    align-items: center;
    gap: 12px;
}

.cfg-save-status {
    font-size: 13px;
    font-weight: 600;
}

.cfg-save-status.success {
    color: var(--sapPositiveColor, #256f3a);
}

.cfg-save-status.error {
    color: var(--sapNegativeColor, #bb0000);
}
```

- [ ] **Step 2: Verify build**

Run: `powershell -File build.ps1`

- [ ] **Step 3: Commit**

```bash
git add cmd/sap-devs-tray/frontend/css/app.css
git commit -m "feat(tray): add config editor CSS styles (panels, fields, typeahead, save bar)"
```

---

### Task 7: Config Editor Frontend — JavaScript

**Files:**
- Create: `cmd/sap-devs-tray/frontend/js/config.js`

This is the largest frontend file. It handles: initial data loading, form population, typeahead for location, detect-location, client-side validation, save, and service/autostart actions.

- [ ] **Step 1: Create config.js with initialization and data loading**

Create `cmd/sap-devs-tray/frontend/js/config.js`:

```javascript
(function() {
    var params = new URLSearchParams(window.location.search);
    var token = params.get('token');
    var typeaheadTimer = null;
    var lastCityResults = [];

    function api(path, opts) {
        opts = opts || {};
        var sep = path.indexOf('?') >= 0 ? '&' : '?';
        var url = path + sep + 'token=' + token;
        return fetch(url, opts).then(function(r) {
            if (!r.ok && r.status !== 400) throw new Error('HTTP ' + r.status);
            return r.json();
        });
    }

    // ── Panel collapse/expand ──
    window.togglePanel = function(headerEl) {
        var panel = headerEl.parentElement;
        panel.classList.toggle('collapsed');
    };

    // ── Load initial data ──
    function init() {
        Promise.all([
            api('/api/config'),
            api('/api/languages'),
            api('/api/service-status')
        ]).then(function(results) {
            populateForm(results[0]);
            populateLanguages(results[1], results[0].language);
            populateServiceStatus(results[2]);
        }).catch(function(e) {
            console.error('Failed to load config:', e);
        });
    }

    function populateLanguages(languages, current) {
        var sel = document.getElementById('cfg-language');
        sel.textContent = '';
        for (var i = 0; i < languages.length; i++) {
            var opt = document.createElement('option');
            opt.value = languages[i].code;
            opt.textContent = languages[i].label;
            if (languages[i].code === current) opt.selected = true;
            sel.appendChild(opt);
        }
    }

    function populateForm(cfg) {
        setVal('cfg-location', cfg.location);
        setVal('cfg-experience', cfg.experience_level);
        setVal('cfg-company-repo', cfg.company_repo);
        setVal('cfg-tip-rotation', cfg.tip_rotation);
        setChecked('cfg-tutorial-interactive', cfg.tutorial_interactive);
        setVal('cfg-local-radius', cfg.events_local_radius);
        setVal('cfg-regional-radius', cfg.events_regional_radius);
        setVal('cfg-notify-days', cfg.events_notify_days);
        setVal('cfg-notify-method', cfg.events_notify_method);
        setChecked('cfg-sync-disabled', cfg.sync_disabled);
        setVal('cfg-sync-tips', cfg.sync_tips);
        setVal('cfg-sync-tools', cfg.sync_tools);
        setVal('cfg-sync-resources', cfg.sync_resources);
        setVal('cfg-sync-context', cfg.sync_context);
        setVal('cfg-sync-events', cfg.sync_events);
        setVal('cfg-sync-youtube', cfg.sync_youtube);
        setVal('cfg-sync-discovery', cfg.sync_discovery);
        setVal('cfg-sync-tutorials', cfg.sync_tutorials);
        setVal('cfg-sync-advocates', cfg.sync_advocates);
        setVal('cfg-sync-mcp', cfg.sync_mcp);
        setVal('cfg-sync-learning', cfg.sync_learning);
        setVal('cfg-service-interval', cfg.service_interval);
    }

    function setVal(id, val) {
        var el = document.getElementById(id);
        if (el) el.value = val != null ? val : '';
    }

    function setChecked(id, val) {
        var el = document.getElementById(id);
        if (el) el.checked = !!val;
    }

    // ── Service/Autostart status ──
    function populateServiceStatus(status) {
        var schBadge = document.getElementById('scheduler-badge');
        var schInstalled = document.getElementById('scheduler-installed');
        var schNotInstalled = document.getElementById('scheduler-not-installed');

        if (status.scheduler.installed) {
            schBadge.textContent = 'Installed';
            schBadge.className = 'cfg-badge installed';
            schInstalled.classList.remove('hidden');
            schNotInstalled.classList.add('hidden');
        } else {
            schBadge.textContent = 'Not Installed';
            schBadge.className = 'cfg-badge not-installed';
            schInstalled.classList.add('hidden');
            schNotInstalled.classList.remove('hidden');
        }

        var asBadge = document.getElementById('autostart-badge');
        var asInstalled = document.getElementById('autostart-installed');
        var asNotInstalled = document.getElementById('autostart-not-installed');

        if (status.autostart.installed) {
            asBadge.textContent = 'Installed';
            asBadge.className = 'cfg-badge installed';
            asInstalled.classList.remove('hidden');
            asNotInstalled.classList.add('hidden');
        } else {
            asBadge.textContent = 'Not Installed';
            asBadge.className = 'cfg-badge not-installed';
            asInstalled.classList.add('hidden');
            asNotInstalled.classList.remove('hidden');
        }
    }

    function refreshServiceStatus() {
        api('/api/service-status').then(populateServiceStatus);
    }

    // ── Location typeahead ──
    function setupTypeahead() {
        var input = document.getElementById('cfg-location');
        var list = document.getElementById('cfg-location-list');

        input.addEventListener('input', function() {
            clearTimeout(typeaheadTimer);
            var q = input.value.trim();
            if (q.length < 2) {
                list.classList.add('hidden');
                return;
            }
            typeaheadTimer = setTimeout(function() {
                api('/api/cities?q=' + encodeURIComponent(q)).then(function(cities) {
                    lastCityResults = cities;
                    list.textContent = '';
                    if (!cities || cities.length === 0) {
                        list.classList.add('hidden');
                        return;
                    }
                    for (var i = 0; i < cities.length; i++) {
                        var item = document.createElement('div');
                        item.className = 'cfg-typeahead-item';
                        item.textContent = cities[i].name + ', ' + cities[i].country;
                        item.dataset.value = cities[i].name + ', ' + cities[i].country;
                        item.addEventListener('click', function() {
                            input.value = this.dataset.value;
                            list.classList.add('hidden');
                            checkLocationWarning();
                        });
                        list.appendChild(item);
                    }
                    list.classList.remove('hidden');
                });
            }, 200);
        });

        input.addEventListener('blur', function() {
            setTimeout(function() { list.classList.add('hidden'); }, 150);
            checkLocationWarning();
        });
    }

    function checkLocationWarning() {
        var input = document.getElementById('cfg-location');
        var warn = document.getElementById('cfg-location-warning');
        var val = input.value.trim();
        if (!val) {
            warn.classList.add('hidden');
            return;
        }
        var found = false;
        for (var i = 0; i < lastCityResults.length; i++) {
            var match = lastCityResults[i].name + ', ' + lastCityResults[i].country;
            if (match.toLowerCase() === val.toLowerCase()) { found = true; break; }
        }
        if (!found) {
            api('/api/cities?q=' + encodeURIComponent(val.split(',')[0])).then(function(cities) {
                var matched = false;
                for (var i = 0; i < cities.length; i++) {
                    var m = cities[i].name + ', ' + cities[i].country;
                    if (m.toLowerCase() === val.toLowerCase()) { matched = true; break; }
                }
                if (matched) warn.classList.add('hidden');
                else warn.classList.remove('hidden');
            });
        } else {
            warn.classList.add('hidden');
        }
    }

    // ── Detect location ──
    function setupDetect() {
        var btn = document.getElementById('btn-detect-location');
        btn.addEventListener('click', function() {
            btn.disabled = true;
            btn.textContent = 'Detecting...';
            api('/api/detect-location', { method: 'POST' })
                .then(function(data) {
                    if (data.error) {
                        showSaveStatus(data.error, 'error');
                    } else {
                        document.getElementById('cfg-location').value = data.city + ', ' + data.country;
                        document.getElementById('cfg-location-warning').classList.add('hidden');
                    }
                })
                .catch(function() {
                    showSaveStatus('Could not detect location', 'error');
                })
                .finally(function() {
                    btn.disabled = false;
                    btn.textContent = 'Detect';
                });
        });
    }

    // ── Validation ──
    function validateForm() {
        clearErrors();
        var errors = {};

        var companyRepo = getVal('cfg-company-repo');
        if (companyRepo) {
            try {
                var u = new URL(companyRepo);
                if (u.protocol !== 'https:') errors['company_repo'] = 'Must be a valid URL (https://...)';
            } catch(e) {
                errors['company_repo'] = 'Must be a valid URL (https://...)';
            }
        }

        var intFields = [
            { id: 'cfg-local-radius', key: 'events_local_radius' },
            { id: 'cfg-regional-radius', key: 'events_regional_radius' },
            { id: 'cfg-notify-days', key: 'events_notify_days' }
        ];
        for (var i = 0; i < intFields.length; i++) {
            var v = parseInt(getVal(intFields[i].id), 10);
            if (isNaN(v) || v <= 0) errors[intFields[i].key] = 'Must be greater than 0';
        }

        var durationFields = [
            'sync_tips', 'sync_tools', 'sync_advocates', 'sync_resources',
            'sync_context', 'sync_mcp', 'sync_events', 'sync_youtube',
            'sync_discovery', 'sync_tutorials', 'sync_learning', 'service_interval'
        ];
        var durationRe = /^(\d+(\.\d+)?(h|m|s|ms|us|µs|ns))+$/;
        for (var j = 0; j < durationFields.length; j++) {
            var dv = getVal('cfg-' + durationFields[j].replace(/_/g, '-'));
            if (!dv || !durationRe.test(dv)) {
                errors[durationFields[j]] = 'Invalid duration format';
            }
        }

        return errors;
    }

    function clearErrors() {
        var errEls = document.querySelectorAll('.cfg-error');
        for (var i = 0; i < errEls.length; i++) errEls[i].classList.add('hidden');
        var inputs = document.querySelectorAll('.fd-input.has-error');
        for (var j = 0; j < inputs.length; j++) inputs[j].classList.remove('has-error');
    }

    function showErrors(errors) {
        var fieldMap = {
            company_repo: 'cfg-company-repo',
            events_local_radius: 'cfg-local-radius',
            events_regional_radius: 'cfg-regional-radius',
            events_notify_days: 'cfg-notify-days',
            sync_tips: 'cfg-sync-tips', sync_tools: 'cfg-sync-tools',
            sync_advocates: 'cfg-sync-advocates', sync_resources: 'cfg-sync-resources',
            sync_context: 'cfg-sync-context', sync_mcp: 'cfg-sync-mcp',
            sync_events: 'cfg-sync-events', sync_youtube: 'cfg-sync-youtube',
            sync_discovery: 'cfg-sync-discovery', sync_tutorials: 'cfg-sync-tutorials',
            sync_learning: 'cfg-sync-learning', service_interval: 'cfg-service-interval'
        };
        for (var key in errors) {
            var inputId = fieldMap[key];
            if (!inputId) continue;
            var input = document.getElementById(inputId);
            if (input) input.classList.add('has-error');
            var errEl = document.getElementById(inputId + '-error');
            if (errEl) {
                errEl.textContent = errors[key];
                errEl.classList.remove('hidden');
            }
        }
    }

    // ── Save ──
    function setupSave() {
        document.getElementById('btn-save').addEventListener('click', function() {
            var errors = validateForm();
            if (Object.keys(errors).length > 0) {
                showErrors(errors);
                showSaveStatus('Please fix the errors above', 'error');
                return;
            }

            var body = {
                language: getVal('cfg-language'),
                location: getVal('cfg-location'),
                experience_level: getVal('cfg-experience'),
                company_repo: getVal('cfg-company-repo'),
                tip_rotation: getVal('cfg-tip-rotation'),
                tutorial_interactive: document.getElementById('cfg-tutorial-interactive').checked,
                events_local_radius: parseInt(getVal('cfg-local-radius'), 10),
                events_regional_radius: parseInt(getVal('cfg-regional-radius'), 10),
                events_notify_days: parseInt(getVal('cfg-notify-days'), 10),
                events_notify_method: getVal('cfg-notify-method'),
                sync_disabled: document.getElementById('cfg-sync-disabled').checked,
                sync_tips: getVal('cfg-sync-tips'),
                sync_tools: getVal('cfg-sync-tools'),
                sync_advocates: getVal('cfg-sync-advocates'),
                sync_resources: getVal('cfg-sync-resources'),
                sync_context: getVal('cfg-sync-context'),
                sync_mcp: getVal('cfg-sync-mcp'),
                sync_events: getVal('cfg-sync-events'),
                sync_youtube: getVal('cfg-sync-youtube'),
                sync_discovery: getVal('cfg-sync-discovery'),
                sync_tutorials: getVal('cfg-sync-tutorials'),
                sync_learning: getVal('cfg-sync-learning'),
                service_interval: getVal('cfg-service-interval')
            };

            api('/api/config', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(body)
            }).then(function(data) {
                if (data.errors) {
                    showErrors(data.errors);
                    showSaveStatus('Validation failed', 'error');
                } else {
                    showSaveStatus('Configuration saved', 'success');
                    setTimeout(function() { hideSaveStatus(); }, 3000);
                }
            }).catch(function(e) {
                showSaveStatus('Save failed: ' + e.message, 'error');
            });
        });
    }

    function showSaveStatus(msg, type) {
        var el = document.getElementById('cfg-save-status');
        el.textContent = msg;
        el.className = 'cfg-save-status ' + type;
        el.classList.remove('hidden');
    }

    function hideSaveStatus() {
        document.getElementById('cfg-save-status').classList.add('hidden');
    }

    function getVal(id) {
        var el = document.getElementById(id);
        return el ? el.value : '';
    }

    // ── Service/Autostart actions ──
    function setupServiceActions() {
        bindAction('btn-scheduler-install', '/api/service-install', 'Installing...');
        bindAction('btn-scheduler-uninstall', '/api/service-uninstall', 'Uninstalling...');
        bindAction('btn-autostart-install', '/api/autostart-install', 'Installing...');
        bindAction('btn-autostart-uninstall', '/api/autostart-uninstall', 'Uninstalling...');
    }

    function bindAction(btnId, endpoint, loadingText) {
        var btn = document.getElementById(btnId);
        if (!btn) return;
        btn.addEventListener('click', function() {
            var origText = btn.textContent;
            btn.disabled = true;
            btn.textContent = loadingText;
            api(endpoint, { method: 'POST' })
                .then(function(data) {
                    if (data.error) {
                        showSaveStatus(data.error, 'error');
                    }
                    refreshServiceStatus();
                })
                .catch(function(e) {
                    showSaveStatus('Action failed: ' + e.message, 'error');
                })
                .finally(function() {
                    btn.disabled = false;
                    btn.textContent = origText;
                });
        });
    }

    // ── Boot ──
    document.addEventListener('DOMContentLoaded', function() {
        setupTypeahead();
        setupDetect();
        setupSave();
        setupServiceActions();
        init();
    });
})();
```

- [ ] **Step 2: Verify build**

Run: `powershell -File build.ps1`

- [ ] **Step 3: Commit**

```bash
git add cmd/sap-devs-tray/frontend/js/config.js
git commit -m "feat(tray): add config editor JavaScript (form logic, typeahead, validation, save)"
```

---

### Task 8: Config Window and Tray Menu Integration

**Files:**
- Modify: `cmd/sap-devs-tray/server.go`
- Modify: `cmd/sap-devs-tray/app.go`

- [ ] **Step 1: Add ConfigURL method and configWindowFunc to Server**

In `server.go`, add a `configWindowFunc` field to the `Server` struct and a `ConfigURL()` method. The field follows the same pattern as the existing `hideFunc`.

Add `configWindowFunc func()` to the `Server` struct, right after the `hideFunc` field (line 28):

```go
hideFunc         func()
configWindowFunc func() // opens/shows the config window
```

Add the `ConfigURL` method (after the existing `PanelURL` method around line 75):

```go
func (s *Server) ConfigURL() string {
	return fmt.Sprintf("%s/config.html?token=%s", s.URL(), s.Token)
}
```

- [ ] **Step 2: Create config webview window in app.go**

In `app.go`, after the `panel` window creation (after line 41), add the config window:

```go
configWin := app.Window.NewWithOptions(application.WebviewWindowOptions{
	Name:   "sap-devs Config",
	Width:  520,
	Height: 700,
	URL:    srv.ConfigURL(),
	Hidden: true,
})

configWin.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
	configWin.Hide()
	e.Cancel()
})

srv.configWindowFunc = func() {
	configWin.SetURL(srv.ConfigURL())
	configWin.Show()
	configWin.Focus()
}
```

- [ ] **Step 3: Add "Config" menu item to tray menu**

In `app.go`, add a "Config" item to the tray context menu. Insert it after the "Inject Now" item (after line 68) and before the separator:

```go
menu.AddSeparator()
menu.Add("Config...").OnClick(func(ctx *application.Context) {
	if srv.configWindowFunc != nil {
		srv.configWindowFunc()
	}
})
```

- [ ] **Step 4: Verify build**

Run: `powershell -File build.ps1`

- [ ] **Step 5: Commit**

```bash
git add cmd/sap-devs-tray/server.go cmd/sap-devs-tray/app.go
git commit -m "feat(tray): add config window (520x700) and Config menu item"
```

---

### Task 9: Dashboard "Config" Button

**Files:**
- Modify: `cmd/sap-devs-tray/server.go`
- Modify: `cmd/sap-devs-tray/frontend/index.html`
- Modify: `cmd/sap-devs-tray/frontend/js/app.js`

- [ ] **Step 1: Add /api/open-config endpoint in server.go**

Wails webviews may not support `window.open`. Instead, add a server-side endpoint that triggers the config window via the already-wired `configWindowFunc`:

```go
func (s *Server) handleOpenConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.configWindowFunc != nil {
		s.configWindowFunc()
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
```

Register it in `NewServer()`:

```go
s.mux.HandleFunc("/api/open-config", s.requireToken(s.handleOpenConfig))
```

- [ ] **Step 2: Add Config button to dashboard HTML**

In `index.html`, add a Config button in the action-buttons div (line 68 area). Insert it after the Inject button:

```html
<div class="action-buttons">
    <button type="button" class="fd-button fd-button--emphasized" id="btn-sync">&#8635; Sync Now</button>
    <button type="button" class="fd-button" id="btn-inject">&#8635; Inject Now</button>
    <button type="button" class="fd-button" id="btn-config">&#9881; Config</button>
</div>
```

- [ ] **Step 3: Add click handler in app.js**

In `app.js`, inside the `DOMContentLoaded` handler (around line 176), add:

```javascript
var btnConfig = document.getElementById('btn-config');
if (btnConfig) {
    btnConfig.addEventListener('click', function() {
        fetch('/api/open-config?token=' + token, { method: 'POST' });
    });
}
```

- [ ] **Step 4: Verify build**

Run: `powershell -File build.ps1`

- [ ] **Step 5: Commit**

```bash
git add cmd/sap-devs-tray/server.go cmd/sap-devs-tray/frontend/index.html cmd/sap-devs-tray/frontend/js/app.js
git commit -m "feat(tray): add Config button to dashboard panel via open-config API"
```

---

### Task 10: Manual Testing & Polish

**Files:** All files from previous tasks

This task is manual verification — build the tray binary, run it, and test each feature through the GUI.

- [ ] **Step 1: Build and run**

```bash
powershell -File build.ps1
./sap-devs-tray.exe
```

- [ ] **Step 2: Test config window opens**

Click the tray icon → "Config..." menu item. Verify:
- Window opens at ~520×700
- Title bar shows "sap-devs Config"
- All 5 panels visible and expanded
- Form fields populated with current config values
- Language dropdown shows all 6 languages + auto-detect
- Close button hides (doesn't destroy) the window; re-opening shows the same state

- [ ] **Step 3: Test location typeahead**

Type a city prefix (e.g. "Ber") in the Location field. Verify:
- Dropdown appears with matching cities after brief delay
- Clicking an entry fills the input as "City, Country"
- Soft warning appears when entering a non-matching location
- Detect button calls ip-api.com and populates the field

- [ ] **Step 4: Test form validation**

Enter invalid values and click Save:
- Company repo: "not-a-url" → error shown
- Local radius: 0 or empty → error shown
- Sync TTL: "abc" → error shown
- Fix all errors and save → "Configuration saved" message appears

- [ ] **Step 5: Test save persistence**

After saving, close and reopen the config window. Verify values persisted. Also check `config.yaml` on disk.

- [ ] **Step 6: Test Service & Tray panel**

- If scheduler is not installed: "Not Installed" badge shown, "Install Scheduler" button shown
- Click Install → badge changes to "Installed", Interval field and Uninstall button appear
- Click Uninstall → reverts to "Not Installed" state
- Same for Tray Autostart section

- [ ] **Step 7: Test dark mode**

Toggle OS theme to dark. Verify:
- All panels, inputs, badges, and buttons render correctly
- No unreadable text or invisible borders

- [ ] **Step 8: Test dashboard Config button**

Open dashboard panel → click Config button → verify config window opens.

- [ ] **Step 9: Fix any issues found, commit**

```bash
git add -A
git commit -m "fix(tray): polish config editor after manual testing"
```
