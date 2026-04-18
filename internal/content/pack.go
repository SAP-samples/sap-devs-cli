package content

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Pack is a named bundle of SAP knowledge for a specific domain.
type Pack struct {
	ID               string
	Name             string
	Description      string
	Tags             []string
	Profiles         []string
	Weight           int
	Overlaps         []string
	Base             bool
	Additive         bool
	AdditivePosition string // "before" | "after"; normalised to "after" if empty

	ContextMD  string
	Resources  []Resource
	Tools      []ToolDef
	MCPServers []MCPServer
	Tips       []Tip

	PreambleMD string
	Hooks      []HookDef
	Influencers []Influencer
	Samples     []Sample

	EventTypes     []EventType
	EventInstances []EventInstance

	YouTubeSources []YouTubeSource
	Videos         []Video
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
	PackID      string     // set at load time, not in YAML
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

// HookDef declares a hook command to wire into an AI tool's event system.
type HookDef struct {
	ID      string   `yaml:"id"`
	Event   string   `yaml:"event"`
	Command string   `yaml:"command"`
	Tools   []string `yaml:"tools"`
	PackID  string   // set at load time, not in YAML
}

// Influencer is a community influencer or thought leader within a pack.
type Influencer struct {
	ID     string            `yaml:"id"`
	Name   string            `yaml:"name"`
	Role   string            `yaml:"role"`
	Org    string            `yaml:"org"`
	Focus  []string          `yaml:"focus"`
	Links  map[string]string `yaml:"links"`
	PackID string            // set at load time, not in YAML
}

// Sample is a canonical code sample reference within a pack.
type Sample struct {
	ID          string   `yaml:"id"`
	Label       string   `yaml:"label"`
	URL         string   `yaml:"url"`
	Description string   `yaml:"description"`
	Tags        []string `yaml:"tags"`
	Inject      bool     `yaml:"inject"`
	PackID      string   // set at load time, not in YAML
}

// YouTubeSource declares a playlist or individual video in youtube.yaml.
type YouTubeSource struct {
	ID         string   `yaml:"id"`
	Type       string   `yaml:"type"`
	Name       string   `yaml:"name"`
	PlaylistID string   `yaml:"playlist_id,omitempty"`
	VideoID    string   `yaml:"video_id,omitempty"`
	Tags       []string `yaml:"tags,omitempty"`
	PackID     string
}

// Video is a resolved YouTube video (fetched from a playlist or declared individually).
type Video struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	URL         string    `json:"url"`
	VideoID     string    `json:"video_id"`
	Published   time.Time `json:"published"`
	Description string    `json:"description"`
	Duration    string    `json:"duration,omitempty"`
	SourceID    string    `json:"source_id"`
	Tags        []string  `json:"tags,omitempty"`
	PackID      string    `json:"pack_id"`
}

// EventType defines a category of events and its data source.
type EventType struct {
	ID           string   `yaml:"id"`
	Name         string   `yaml:"name"`
	Description  string   `yaml:"description,omitempty"`
	Source       string   `yaml:"source"`
	RSSURL        string   `yaml:"rss_url,omitempty"`
	KhorosBoardID string   `yaml:"khoros_board_id,omitempty"`
	DefaultScope string   `yaml:"default_scope"`
	Tags         []string `yaml:"tags,omitempty"`
	PackID       string   // set at load time
}

// EventInstance is a specific event occurrence.
type EventInstance struct {
	ID         string   `yaml:"id"`
	Type       string   `yaml:"type"`
	Title      string   `yaml:"title"`
	DateStr    string   `yaml:"date"`
	EndDateStr string   `yaml:"end_date,omitempty"`
	Location   string   `yaml:"location,omitempty"`
	Scope      string   `yaml:"scope"`
	URL        string   `yaml:"url"`
	Room       string   `yaml:"room,omitempty"`
	Speaker    string   `yaml:"speaker,omitempty"`
	Tags       []string `yaml:"tags,omitempty"`
	PackID     string   // set at load time
}

// ParseDate parses the DateStr field into time.Time.
func (e *EventInstance) ParseDate() (time.Time, error) {
	return parseEventDate(e.DateStr)
}

// ParseEndDate parses the EndDateStr field into time.Time.
func (e *EventInstance) ParseEndDate() (time.Time, error) {
	if e.EndDateStr == "" {
		return time.Time{}, nil
	}
	return parseEventDate(e.EndDateStr)
}

func parseEventDate(s string) (time.Time, error) {
	for _, layout := range []string{"2006-01-02", time.RFC3339, time.RFC1123Z} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse date: %s", s)
}

type packMetaLocale struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

type packMeta struct {
	ID               string                    `yaml:"id"`
	Name             string                    `yaml:"name"`
	Description      string                    `yaml:"description"`
	Tags             []string                  `yaml:"tags"`
	Profiles         []string                  `yaml:"profiles"`
	Weight           int                       `yaml:"weight"`
	Overlaps         []string                  `yaml:"overlaps,omitempty"`
	Base             bool                      `yaml:"base,omitempty"`
	Additive         bool                      `yaml:"additive,omitempty"`
	AdditivePosition string                    `yaml:"additive_position,omitempty"`
	Locales          map[string]packMetaLocale `yaml:"locales,omitempty"`
}

// LoadPack reads all files from packDir and returns a populated Pack.
// lang selects locale-specific files and metadata overrides (e.g. "de", "fr").
// Pass "" or "en" to use the base (English) content.
func LoadPack(packDir string, lang string) (*Pack, error) {
	metaData, err := os.ReadFile(filepath.Join(packDir, "pack.yaml"))
	if err != nil {
		return nil, err
	}
	var meta packMeta
	if err := yaml.Unmarshal(metaData, &meta); err != nil {
		return nil, err
	}

	pack := &Pack{
		ID:               meta.ID,
		Name:             meta.Name,
		Description:      meta.Description,
		Tags:             meta.Tags,
		Profiles:         meta.Profiles,
		Weight:           meta.Weight,
		Overlaps:         meta.Overlaps,
		Base:             meta.Base,
		Additive:         meta.Additive,
		AdditivePosition: meta.AdditivePosition,
	}

	if pack.Additive && pack.AdditivePosition != "before" {
		pack.AdditivePosition = "after"
	}

	if lang != "" && lang != "en" {
		if loc, ok := meta.Locales[lang]; ok {
			if loc.Name != "" {
				pack.Name = loc.Name
			}
			if loc.Description != "" {
				pack.Description = loc.Description
			}
		}
	}

	// Context file: locale variant → expanded base → base
	contextFile := filepath.Join(packDir, "context.md")
	localeFound := false
	if lang != "" && lang != "en" {
		if loc := filepath.Join(packDir, "context."+lang+".md"); fileExists(loc) {
			contextFile = loc
			localeFound = true
		}
	}
	// If no locale variant selected, prefer the sync-expanded file when present.
	if !localeFound {
		if exp := filepath.Join(packDir, "context.expanded.md"); fileExists(exp) {
			contextFile = exp
		}
	}
	if data, err := os.ReadFile(contextFile); err == nil {
		pack.ContextMD = string(data)
	}
	if data, err := os.ReadFile(filepath.Join(packDir, "preamble.md")); err == nil {
		pack.PreambleMD = string(data)
	}
	if data, err := os.ReadFile(filepath.Join(packDir, "hook.yaml")); err == nil {
		_ = yaml.Unmarshal(data, &pack.Hooks)
		for i := range pack.Hooks {
			pack.Hooks[i].PackID = pack.ID
		}
	}
	if data, err := os.ReadFile(filepath.Join(packDir, "influencers.yaml")); err == nil {
		_ = yaml.Unmarshal(data, &pack.Influencers)
		for i := range pack.Influencers {
			pack.Influencers[i].PackID = pack.ID
		}
	}
	if data, err := os.ReadFile(filepath.Join(packDir, "samples.yaml")); err == nil {
		_ = yaml.Unmarshal(data, &pack.Samples)
		for i := range pack.Samples {
			pack.Samples[i].PackID = pack.ID
		}
	}
	if data, err := os.ReadFile(filepath.Join(packDir, "youtube.yaml")); err == nil {
		_ = yaml.Unmarshal(data, &pack.YouTubeSources)
		for i := range pack.YouTubeSources {
			pack.YouTubeSources[i].PackID = pack.ID
		}
	}
	if data, err := os.ReadFile(filepath.Join(packDir, "event-types.yaml")); err == nil {
		_ = yaml.Unmarshal(data, &pack.EventTypes)
		for i := range pack.EventTypes {
			pack.EventTypes[i].PackID = pack.ID
		}
	}
	if data, err := os.ReadFile(filepath.Join(packDir, "event-instances.yaml")); err == nil {
		_ = yaml.Unmarshal(data, &pack.EventInstances)
		for i := range pack.EventInstances {
			pack.EventInstances[i].PackID = pack.ID
		}
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
		for i := range pack.MCPServers {
			pack.MCPServers[i].PackID = pack.ID
		}
	}
	tipsFile := filepath.Join(packDir, "tips.md")
	if lang != "" && lang != "en" {
		if loc := filepath.Join(packDir, "tips."+lang+".md"); fileExists(loc) {
			tipsFile = loc
		}
	}
	if data, err := os.ReadFile(tipsFile); err == nil {
		pack.Tips = parseTips(string(data))
	}

	return pack, nil
}

// fileExists reports whether the file at path exists and is accessible.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
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
