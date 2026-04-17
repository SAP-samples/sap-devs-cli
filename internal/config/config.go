package config

import (
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds user-level tool configuration from ~/.config/sap-devs/config.yaml.
type Config struct {
	CompanyRepo string     `yaml:"company_repo,omitempty"`
	Language    string     `yaml:"language,omitempty"` // e.g. "de"; empty = auto-detect from locale
	Location    string     `yaml:"location,omitempty"`
	Sync        SyncConfig `yaml:"sync"`
	Tip         TipConfig  `yaml:"tip,omitempty"`
}

// SyncConfig controls per-category TTLs for background content refresh.
type SyncConfig struct {
	Tips      time.Duration `yaml:"tips"`
	Tools     time.Duration `yaml:"tools"`
	Advocates time.Duration `yaml:"advocates"`
	Resources time.Duration `yaml:"resources"`
	Context   time.Duration `yaml:"context"`
	MCP       time.Duration `yaml:"mcp"`
	Disabled  bool          `yaml:"disabled"`
}

// TipConfig controls tip display behaviour.
type TipConfig struct {
	Rotation string `yaml:"rotation,omitempty"` // "daily" | "hourly" | "session"; empty = "daily"
}

// Profile holds the user's active developer persona.
type Profile struct {
	ID string `yaml:"id"`
}

// Default returns a Config with factory defaults applied.
func Default() *Config {
	return &Config{
		Sync: SyncConfig{
			Tips:      24 * time.Hour,
			Tools:     24 * time.Hour,
			Advocates: 72 * time.Hour,
			Resources: 168 * time.Hour,
			Context:   168 * time.Hour,
			MCP:       168 * time.Hour,
		},
	}
}

// Load reads config.yaml from configDir. Returns defaults if the file does not exist.
func Load(configDir string) (*Config, error) {
	cfg := Default()
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

// Save writes the config to configDir/config.yaml, creating the directory if needed.
func (c *Config) Save(configDir string) error {
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(configDir, "config.yaml"), data, 0600)
}

// LoadProfile reads profile.yaml from configDir. Returns empty profile if file does not exist.
func LoadProfile(configDir string) (*Profile, error) {
	p := &Profile{}
	path := filepath.Join(configDir, "profile.yaml")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return p, nil
	}
	if err != nil {
		return nil, err
	}
	return p, yaml.Unmarshal(data, p)
}

// SaveProfile writes the profile to configDir/profile.yaml.
func SaveProfile(configDir string, p *Profile) error {
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}
	data, err := yaml.Marshal(p)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(configDir, "profile.yaml"), data, 0600)
}
