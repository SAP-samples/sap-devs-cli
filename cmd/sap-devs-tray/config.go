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

// trayConfig mirrors internal/config.Config with identical YAML tags.
// Duplicated because the tray binary is a separate Go module that cannot
// import internal packages from the main CLI.
type trayConfig struct {
	CompanyRepo     string             `yaml:"company_repo,omitempty" json:"company_repo,omitempty"`
	Language        string             `yaml:"language,omitempty"     json:"language,omitempty"`
	Location        string             `yaml:"location,omitempty"    json:"location,omitempty"`
	Sync            traySyncConfig     `yaml:"sync"                  json:"sync"`
	Tip             trayTipConfig      `yaml:"tip,omitempty"         json:"tip,omitempty"`
	Events          trayEventsConfig   `yaml:"events,omitempty"      json:"events,omitempty"`
	Tutorial        trayTutorialConfig `yaml:"tutorial,omitempty"    json:"tutorial,omitempty"`
	ExperienceLevel string             `yaml:"experience_level,omitempty" json:"experience_level,omitempty"`
	Service         trayServiceConfig  `yaml:"service,omitempty"     json:"service,omitempty"`
	Tray            trayTrayConfig     `yaml:"tray,omitempty"        json:"tray,omitempty"`
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
	Rotation string `yaml:"rotation,omitempty" json:"rotation,omitempty"`
}

type trayEventsConfig struct {
	LocalRadius    int    `yaml:"local_radius,omitempty"    json:"local_radius,omitempty"`
	RegionalRadius int    `yaml:"regional_radius,omitempty" json:"regional_radius,omitempty"`
	NotifyDays     int    `yaml:"notify_days,omitempty"     json:"notify_days,omitempty"`
	NotifyMethod   string `yaml:"notify_method,omitempty"   json:"notify_method,omitempty"`
}

type trayTutorialConfig struct {
	Interactive bool `yaml:"interactive,omitempty" json:"interactive,omitempty"`
}

type trayServiceConfig struct {
	Interval time.Duration `yaml:"interval" json:"interval"`
}

type trayTrayConfig struct {
	Autostart bool `yaml:"autostart,omitempty" json:"autostart,omitempty"`
}

// defaultTrayConfig returns factory defaults matching internal/config.Default().
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
		Service: trayServiceConfig{
			Interval: 6 * time.Hour,
		},
	}
}

// loadTrayConfig reads config.yaml from configDir. Returns defaults if the file does not exist.
func loadTrayConfig(configDir string) (*trayConfig, error) {
	cfg := defaultTrayConfig()
	path := filepath.Join(configDir, "config.yaml")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return nil, err
	}
	return cfg, yaml.Unmarshal(data, cfg)
}

// saveTrayConfig writes config.yaml to configDir, creating the directory if needed.
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

// configJSON is a flat JSON-friendly representation of the config.
// All duration fields are serialised as Go duration strings (e.g. "24h0m0s").
// No nested structs — avoids time.Duration marshaling issues in the browser.
type configJSON struct {
	CompanyRepo         string `json:"company_repo"`
	Language            string `json:"language"`
	Location            string `json:"location"`
	ExperienceLevel     string `json:"experience_level"`
	TipRotation         string `json:"tip_rotation"`
	TutorialInteractive bool   `json:"tutorial_interactive"`
	EventsLocalRadius   int    `json:"events_local_radius"`
	EventsRegionalRadius int   `json:"events_regional_radius"`
	EventsNotifyDays    int    `json:"events_notify_days"`
	EventsNotifyMethod  string `json:"events_notify_method"`
	SyncDisabled        bool   `json:"sync_disabled"`
	SyncTips            string `json:"sync_tips"`
	SyncTools           string `json:"sync_tools"`
	SyncAdvocates       string `json:"sync_advocates"`
	SyncResources       string `json:"sync_resources"`
	SyncContext         string `json:"sync_context"`
	SyncMCP             string `json:"sync_mcp"`
	SyncEvents          string `json:"sync_events"`
	SyncYouTube         string `json:"sync_youtube"`
	SyncDiscovery       string `json:"sync_discovery"`
	SyncTutorials       string `json:"sync_tutorials"`
	SyncLearning        string `json:"sync_learning"`
	ServiceInterval     string `json:"service_interval"`
	TrayAutostart       bool   `json:"tray_autostart"`
}

// toConfigJSON converts a trayConfig to the flat JSON representation,
// applying defaults for zero values so the frontend always sees usable values.
func toConfigJSON(cfg *trayConfig) configJSON {
	defaults := defaultTrayConfig()

	tipRotation := cfg.Tip.Rotation
	if tipRotation == "" {
		tipRotation = "daily"
	}
	notifyMethod := cfg.Events.NotifyMethod
	if notifyMethod == "" {
		notifyMethod = "hook"
	}
	localRadius := cfg.Events.LocalRadius
	if localRadius == 0 {
		localRadius = 200
	}
	regionalRadius := cfg.Events.RegionalRadius
	if regionalRadius == 0 {
		regionalRadius = 800
	}
	notifyDays := cfg.Events.NotifyDays
	if notifyDays == 0 {
		notifyDays = 7
	}
	serviceInterval := cfg.Service.Interval
	if serviceInterval == 0 {
		serviceInterval = defaults.Service.Interval
	}

	durationOr := func(val, def time.Duration) string {
		if val == 0 {
			return def.String()
		}
		return val.String()
	}

	return configJSON{
		CompanyRepo:          cfg.CompanyRepo,
		Language:             cfg.Language,
		Location:             cfg.Location,
		ExperienceLevel:      cfg.ExperienceLevel,
		TipRotation:          tipRotation,
		TutorialInteractive:  cfg.Tutorial.Interactive,
		EventsLocalRadius:    localRadius,
		EventsRegionalRadius: regionalRadius,
		EventsNotifyDays:     notifyDays,
		EventsNotifyMethod:   notifyMethod,
		SyncDisabled:         cfg.Sync.Disabled,
		SyncTips:             durationOr(cfg.Sync.Tips, defaults.Sync.Tips),
		SyncTools:            durationOr(cfg.Sync.Tools, defaults.Sync.Tools),
		SyncAdvocates:        durationOr(cfg.Sync.Advocates, defaults.Sync.Advocates),
		SyncResources:        durationOr(cfg.Sync.Resources, defaults.Sync.Resources),
		SyncContext:          durationOr(cfg.Sync.Context, defaults.Sync.Context),
		SyncMCP:              durationOr(cfg.Sync.MCP, defaults.Sync.MCP),
		SyncEvents:           durationOr(cfg.Sync.Events, defaults.Sync.Events),
		SyncYouTube:          durationOr(cfg.Sync.YouTube, defaults.Sync.YouTube),
		SyncDiscovery:        durationOr(cfg.Sync.Discovery, defaults.Sync.Discovery),
		SyncTutorials:        durationOr(cfg.Sync.Tutorials, defaults.Sync.Tutorials),
		SyncLearning:         durationOr(cfg.Sync.Learning, defaults.Sync.Learning),
		ServiceInterval:      serviceInterval.String(),
		TrayAutostart:        cfg.Tray.Autostart,
	}
}

// handleConfig dispatches to GET or POST handlers for /api/config.
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

// handleConfigGet loads the config and returns it as flat JSON.
func (s *Server) handleConfigGet(w http.ResponseWriter, r *http.Request) {
	cfg, err := loadTrayConfig(s.ConfigDir)
	if err != nil {
		http.Error(w, "failed to load config", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toConfigJSON(cfg))
}

// handleConfigPost decodes JSON input, validates it, saves the config, and
// returns {"status":"ok"} on success or {"errors":{...}} on validation failure.
func (s *Server) handleConfigPost(w http.ResponseWriter, r *http.Request) {
	var input configJSON
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	if errs := validateConfigInput(input); len(errs) > 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"errors": errs})
		return
	}

	cfg, err := loadTrayConfig(s.ConfigDir)
	if err != nil {
		http.Error(w, "failed to load config", http.StatusInternalServerError)
		return
	}

	// Apply input values to config.
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
	cfg.Tray.Autostart = input.TrayAutostart

	// Parse and apply duration fields. Validation already passed, so errors are safe to ignore.
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
		http.Error(w, "failed to save config", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// validateConfigInput checks the incoming configJSON for invalid values.
// Returns a map of field name to error message. An empty map means valid.
func validateConfigInput(input configJSON) map[string]string {
	errs := map[string]string{}

	// company_repo: if non-empty, must be an https:// URL.
	if input.CompanyRepo != "" {
		u, err := url.Parse(input.CompanyRepo)
		if err != nil || u.Scheme != "https" || u.Host == "" {
			errs["company_repo"] = "must be an https:// URL"
		}
	}

	// events radii and notify days: must be > 0.
	if input.EventsLocalRadius <= 0 {
		errs["events_local_radius"] = "must be greater than 0"
	}
	if input.EventsRegionalRadius <= 0 {
		errs["events_regional_radius"] = "must be greater than 0"
	}
	if input.EventsNotifyDays <= 0 {
		errs["events_notify_days"] = "must be greater than 0"
	}

	// All duration fields must parse as valid Go durations.
	durations := map[string]string{
		"sync_tips":        input.SyncTips,
		"sync_tools":       input.SyncTools,
		"sync_advocates":   input.SyncAdvocates,
		"sync_resources":   input.SyncResources,
		"sync_context":     input.SyncContext,
		"sync_mcp":         input.SyncMCP,
		"sync_events":      input.SyncEvents,
		"sync_youtube":     input.SyncYouTube,
		"sync_discovery":   input.SyncDiscovery,
		"sync_tutorials":   input.SyncTutorials,
		"sync_learning":    input.SyncLearning,
		"service_interval": input.ServiceInterval,
	}
	for field, val := range durations {
		if val == "" {
			errs[field] = "must be a valid Go duration (e.g. 24h, 6h30m)"
			continue
		}
		if _, err := time.ParseDuration(val); err != nil {
			errs[field] = "must be a valid Go duration (e.g. 24h, 6h30m)"
		}
	}

	return errs
}
