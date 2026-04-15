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
	ClipFormat   string       `yaml:"format"`
	Template     string       `yaml:"template"`
	Instructions string       `yaml:"instructions"`
	MaxTokens    int          `yaml:"max_tokens,omitempty"` // 0 = unconstrained
	MCPConfig    *MCPConfig   `yaml:"mcp_config,omitempty"`
	Detect       []DetectRule `yaml:"detect"`
}

// Target is a single file injection target.
type Target struct {
	Scope   string `yaml:"scope"` // global | project
	Path    string `yaml:"path"`
	Mode    string `yaml:"mode"` // replace-section | append
	Section string `yaml:"section"`
}

// MCPConfig defines where to write MCP server configuration.
type MCPConfig struct {
	Path   string `yaml:"path"`
	Format string `yaml:"format"`
	Key    string `yaml:"key"`
}

// DetectRule defines a detection method for whether the tool is installed.
type DetectRule struct {
	Command string `yaml:"command,omitempty"`
	Path    string `yaml:"path,omitempty"`
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
