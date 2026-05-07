package adapter

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Adapter defines how to inject SAP context into a specific AI tool.
type Adapter struct {
	ID           string       `yaml:"id"`
	Name         string       `yaml:"name"`
	Type         string       `yaml:"type"` // file-inject | clipboard-export | mcp-wire
	Targets      []Target     `yaml:"targets"`
	Format       string       `yaml:"format,omitempty"`      // "markdown" (default) | "plain-prose"
	Template     string       `yaml:"template"`
	Instructions string       `yaml:"instructions"`
	MaxTokens    int          `yaml:"max_tokens,omitempty"`  // 0 = unconstrained
	MaxBytes     int          `yaml:"max_bytes,omitempty"`   // hard byte ceiling; 0 = unconstrained
	Verbosity    string       `yaml:"verbosity,omitempty"`   // "minimal" | "standard" | "full"; default "full"
	ExportPath   string       `yaml:"export_path,omitempty"` // file-export: path to write full context
	MCPConfig       *MCPConfig   `yaml:"mcp_config,omitempty"`
	ExtraMCPConfigs []MCPConfig  `yaml:"extra_mcp_configs,omitempty"`
	HookConfig      *HookConfig  `yaml:"hook_config,omitempty"`
	SkillConfig     *SkillConfig `yaml:"skill_config,omitempty"`
	Detect       []DetectRule `yaml:"detect"`
}

// Target is a single file injection target.
type Target struct {
	Scope    string `yaml:"scope"`              // global | project
	Path     string `yaml:"path"`
	Mode     string `yaml:"mode"`               // replace-section | append | replace-file
	Section  string `yaml:"section"`
	Preamble string `yaml:"preamble,omitempty"` // prepended before content; replace-file only
}

// MCPConfig defines where to write MCP server configuration.
type MCPConfig struct {
	Path   string `yaml:"path"`
	Format string `yaml:"format"`
	Key    string `yaml:"key"`
}

// HookConfig defines where to write hook command entries.
type HookConfig struct {
	Path   string `yaml:"path"`
	Format string `yaml:"format"` // "json" only for now
	Key    string `yaml:"key"`    // dot-separated JSON path, e.g. "hooks.SessionStart"
}

// SkillConfig defines where to install skill files for this adapter.
type SkillConfig struct {
	Path string `yaml:"path"` // base directory, e.g. "~/.claude/skills"
}

// DetectRule defines a detection method for whether the tool is installed.
type DetectRule struct {
	Command string `yaml:"command,omitempty"`
	Path    string `yaml:"path,omitempty"`
}

// AllMCPConfigs returns the primary MCPConfig (if set) plus any extras.
func (a Adapter) AllMCPConfigs() []MCPConfig {
	var out []MCPConfig
	if a.MCPConfig != nil {
		out = append(out, *a.MCPConfig)
	}
	out = append(out, a.ExtraMCPConfigs...)
	return out
}

// LoadAdapters reads all *.yaml files from dir and returns the parsed adapters.
// If dir does not exist, returns an empty slice without error.
func LoadAdapters(dir string) ([]Adapter, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var adapters []Adapter
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		var a Adapter
		if err := yaml.Unmarshal(data, &a); err != nil {
			return nil, err
		}
		if a.ID != "" {
			adapters = append(adapters, a)
		}
	}
	return adapters, nil
}
