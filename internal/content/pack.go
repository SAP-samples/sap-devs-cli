package content

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Pack is a named bundle of SAP knowledge for a specific domain.
type Pack struct {
	ID          string
	Name        string
	Description string
	Tags        []string
	Profiles    []string
	Weight      int
	ContextMD   string
	Resources   []Resource
	Tools       []ToolDef
	MCPServers  []MCPServer
	Tips        []Tip
}

// Resource is a curated link within a pack.
type Resource struct {
	ID       string   `yaml:"id"`
	Title    string   `yaml:"title"`
	URL      string   `yaml:"url"`
	Type     string   `yaml:"type"`
	Tags     []string `yaml:"tags"`
	Advocate string   `yaml:"advocate,omitempty"`
	PackID   string   // set at load time, not in YAML
}

// ToolDef is an entry in the tool catalog with version detection rules.
type ToolDef struct {
	ID       string            `yaml:"id"`
	Name     string            `yaml:"name"`
	Required string            `yaml:"required"`
	Detect   ToolDetect        `yaml:"detect"`
	Install  map[string]string `yaml:"install"`
	Docs     string            `yaml:"docs"`
}

// ToolDetect holds the command and regex to detect an installed tool's version.
type ToolDetect struct {
	Command string `yaml:"command"`
	Pattern string `yaml:"pattern"`
}

// MCPServer defines an installable MCP server for this pack's domain.
type MCPServer struct {
	ID          string     `yaml:"id"`
	Name        string     `yaml:"name"`
	Description string     `yaml:"description"`
	Install     MCPInstall `yaml:"install"`
	Hosts       []string   `yaml:"hosts"`
}

// MCPInstall defines how to run the MCP server.
type MCPInstall struct {
	Command string   `yaml:"command"`
	Args    []string `yaml:"args"`
}

// Tip is a single actionable tip with profile tags.
type Tip struct {
	Title   string
	Content string
	Tags    []string
}

type packMeta struct {
	ID          string   `yaml:"id"`
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Tags        []string `yaml:"tags"`
	Profiles    []string `yaml:"profiles"`
	Weight      int      `yaml:"weight"`
}

// LoadPack reads all files from packDir and returns a populated Pack.
func LoadPack(packDir string) (*Pack, error) {
	metaData, err := os.ReadFile(filepath.Join(packDir, "pack.yaml"))
	if err != nil {
		return nil, err
	}
	var meta packMeta
	if err := yaml.Unmarshal(metaData, &meta); err != nil {
		return nil, err
	}

	pack := &Pack{
		ID:          meta.ID,
		Name:        meta.Name,
		Description: meta.Description,
		Tags:        meta.Tags,
		Profiles:    meta.Profiles,
		Weight:      meta.Weight,
	}

	if data, err := os.ReadFile(filepath.Join(packDir, "context.md")); err == nil {
		pack.ContextMD = string(data)
	}
	if data, err := os.ReadFile(filepath.Join(packDir, "resources.yaml")); err == nil {
		_ = yaml.Unmarshal(data, &pack.Resources)
		for i := range pack.Resources {
			pack.Resources[i].PackID = pack.ID
		}
	}
	if data, err := os.ReadFile(filepath.Join(packDir, "tools.yaml")); err == nil {
		_ = yaml.Unmarshal(data, &pack.Tools)
	}
	if data, err := os.ReadFile(filepath.Join(packDir, "mcp.yaml")); err == nil {
		_ = yaml.Unmarshal(data, &pack.MCPServers)
	}
	if data, err := os.ReadFile(filepath.Join(packDir, "tips.md")); err == nil {
		pack.Tips = parseTips(string(data))
	}

	return pack, nil
}

// parseTips splits a Markdown file on H2 headings into individual Tip entries.
// Each tip optionally has a "Tags: a,b,c" line immediately after the heading.
func parseTips(md string) []Tip {
	var tips []Tip
	sections := strings.Split(md, "\n## ")
	for i, section := range sections {
		if i == 0 && !strings.HasPrefix(section, "## ") {
			continue // preamble before first heading — not a tip
		}
		lines := strings.SplitN(strings.TrimSpace(section), "\n", 3)
		if len(lines) == 0 {
			continue
		}
		tip := Tip{Title: strings.TrimPrefix(lines[0], "## ")}
		rest := ""
		if len(lines) >= 2 {
			if strings.HasPrefix(lines[1], "Tags:") {
				tagStr := strings.TrimPrefix(lines[1], "Tags:")
				for _, t := range strings.Split(tagStr, ",") {
					tip.Tags = append(tip.Tags, strings.TrimSpace(t))
				}
				if len(lines) >= 3 {
					rest = lines[2]
				}
			} else {
				rest = strings.Join(lines[1:], "\n")
			}
		}
		tip.Content = strings.TrimSpace(rest)
		if tip.Title != "" {
			tips = append(tips, tip)
		}
	}
	return tips
}
