# sap-devs CLI — Plan 1: Foundation

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver a working Go CLI binary with `init`, `sync`, `profile`, `tip`, and `config` commands backed by a layered content system and per-category TTL-based auto-refresh.

**Architecture:** A thin Go binary built with Cobra reads content from a layered cache (official → company → user → project). On every invocation, stale content categories are refreshed in the background via HTTP archive download. The `tip` command is the canonical shell-profile hook; `sync` and `profile` give developers manual control.

**Tech Stack:** Go 1.22+, Cobra (CLI), Viper (config), glamour (Markdown rendering in terminal), testify (test assertions), gopkg.in/yaml.v3

**Spec:** `docs/superpowers/specs/2026-04-13-sap-devs-cli-design.md`

---

## File Map

```
sap-devs-cli/
  main.go                              Entry point — wires cobra root, calls Execute()
  go.mod
  go.sum

  cmd/
    root.go                            Root cobra command; global flags; triggers background staleness check
    init.go                            sap-devs init — first-time setup wizard
    sync.go                            sap-devs sync [--force] [--category <name>]
    profile.go                         sap-devs profile {set,show,list}
    tip.go                             sap-devs tip
    config.go                          sap-devs config {show,set,company}

  internal/
    xdg/
      xdg.go                           Platform-native config/cache/data paths (wraps os.UserConfigDir etc.)
      xdg_test.go
    config/
      config.go                        Load/save ~/.config/sap-devs/config.yaml and profile.yaml
      config_test.go
    content/
      pack.go                          Pack struct; parse pack.yaml + context.md + resources.yaml + tips.md + tools.yaml
      pack_test.go
      profile.go                       Profile struct; parse profiles/*.yaml; apply weight ordering to packs
      profile_test.go
      loader.go                        ContentLoader: merge official/company/user/project layers; return ordered packs
      loader_test.go
      tip.go                           Select a profile-relevant tip from merged pack pool (daily seed for consistency)
      tip_test.go
    sync/
      fetcher.go                       Download a tagged zip archive from a GitHub/GitLab HTTPS URL; extract to dir
      fetcher_test.go
      engine.go                        TTL staleness check per category; trigger fetcher; update last-sync timestamps
      engine_test.go

  content/                             Shipped with the repo — the official knowledge base
    packs/
      cap/
        pack.yaml
        context.md
        tips.md
        resources.yaml
        tools.yaml
        mcp.yaml
      abap/
        pack.yaml
        context.md
        tips.md
        resources.yaml
        tools.yaml
        mcp.yaml
      btp-core/
        pack.yaml
        context.md
        tips.md
        resources.yaml
        tools.yaml
        mcp.yaml
    profiles/
      cap-developer.yaml
      abap-developer.yaml
      btp-developer.yaml
    adapters/                          Placeholder stubs only in Plan 1 (implemented in Plan 2)
      claude-code.yaml
      cursor.yaml

  .github/
    workflows/
      ci.yml                           go test ./... + go build on push/PR

  .gitignore                           .superpowers/
```

---

## Task 1: Project Bootstrap

**Files:**
- Create: `go.mod`
- Create: `main.go`
- Create: `cmd/root.go`
- Create: `.github/workflows/ci.yml`
- Create: `.gitignore`

- [ ] **Step 1.1: Initialise Go module**

```bash
cd d:/projects/sap-devs-cli
go mod init github.com/SAP-samples/sap-devs-cli
```

- [ ] **Step 1.2: Add dependencies**

```bash
go get github.com/spf13/cobra@latest
go get github.com/spf13/viper@latest
go get github.com/charmbracelet/glamour@latest
go get github.com/stretchr/testify@latest
go get gopkg.in/yaml.v3@latest
go get github.com/blang/semver/v4@latest
```

- [ ] **Step 1.3: Write `main.go`**

```go
package main

import "github.com/SAP-samples/sap-devs-cli/cmd"

func main() {
	cmd.Execute()
}
```

- [ ] **Step 1.4: Write `cmd/root.go`**

```go
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "sap-devs",
	Short: "AI-first SAP developer toolkit",
	Long:  `sap-devs injects up-to-date SAP developer knowledge into your AI tools.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

- [ ] **Step 1.5: Write `.github/workflows/ci.yml`**

```yaml
name: CI
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      - run: go test ./...
      - run: go build ./...
```

- [ ] **Step 1.6: Write `.gitignore`**

```
.superpowers/
sap-devs
sap-devs.exe
```

- [ ] **Step 1.7: Verify it builds**

```bash
go build ./...
```

Expected: no output (success).

- [ ] **Step 1.8: Commit**

```bash
git add go.mod go.sum main.go cmd/root.go .github/workflows/ci.yml .gitignore
git commit -m "feat: bootstrap Go CLI project with cobra"
```

---

## Task 2: XDG Path Resolution

**Files:**
- Create: `internal/xdg/xdg.go`
- Create: `internal/xdg/xdg_test.go`

- [ ] **Step 2.1: Write the failing test**

```go
// internal/xdg/xdg_test.go
package xdg_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/SAP-samples/sap-devs-cli/internal/xdg"
)

func TestNew_ReturnsNonEmptyPaths(t *testing.T) {
	paths, err := xdg.New()
	require.NoError(t, err)
	assert.NotEmpty(t, paths.ConfigDir)
	assert.NotEmpty(t, paths.CacheDir)
	assert.NotEmpty(t, paths.DataDir)
}

func TestNew_PathsContainAppName(t *testing.T) {
	paths, err := xdg.New()
	require.NoError(t, err)
	assert.Contains(t, paths.ConfigDir, "sap-devs")
	assert.Contains(t, paths.CacheDir, "sap-devs")
	assert.Contains(t, paths.DataDir, "sap-devs")
}

func TestNew_XDGEnvOverridesOnLinux(t *testing.T) {
	if os.Getenv("GOOS") == "windows" {
		t.Skip("XDG env vars not honoured on Windows")
	}
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	paths, err := xdg.New()
	require.NoError(t, err)
	assert.Contains(t, paths.ConfigDir, "sap-devs")
}
```

- [ ] **Step 2.2: Run test to verify it fails**

```bash
go test ./internal/xdg/... -v
```

Expected: compile error — package doesn't exist yet.

- [ ] **Step 2.3: Write `internal/xdg/xdg.go`**

```go
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
	// macOS: sibling of config dir
	return filepath.Join(configDir, "data"), nil
}
```

- [ ] **Step 2.4: Run tests to verify they pass**

```bash
go test ./internal/xdg/... -v
```

Expected: all tests PASS.

- [ ] **Step 2.5: Commit**

```bash
git add internal/xdg/
git commit -m "feat: add XDG-compliant platform path resolution"
```

---

## Task 3: Config Management

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

- [ ] **Step 3.1: Write the failing tests**

```go
// internal/config/config_test.go
package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/SAP-samples/sap-devs-cli/internal/config"
)

func TestLoad_DefaultsWhenNoFile(t *testing.T) {
	dir := t.TempDir()
	cfg, err := config.Load(dir)
	require.NoError(t, err)
	assert.Equal(t, 24*time.Hour, cfg.Sync.Tips)
	assert.Equal(t, 168*time.Hour, cfg.Sync.Resources)
	assert.False(t, cfg.Sync.Disabled)
}

func TestLoad_ReadsExistingFile(t *testing.T) {
	dir := t.TempDir()
	yaml := `company_repo: "https://github.com/myco/sap-content"`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(yaml), 0600))

	cfg, err := config.Load(dir)
	require.NoError(t, err)
	assert.Equal(t, "https://github.com/myco/sap-content", cfg.CompanyRepo)
}

func TestSaveAndLoad_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Default()
	cfg.CompanyRepo = "https://example.com/repo"
	require.NoError(t, cfg.Save(dir))

	loaded, err := config.Load(dir)
	require.NoError(t, err)
	assert.Equal(t, cfg.CompanyRepo, loaded.CompanyRepo)
}

func TestLoadProfile_DefaultsWhenNoFile(t *testing.T) {
	dir := t.TempDir()
	p, err := config.LoadProfile(dir)
	require.NoError(t, err)
	assert.Empty(t, p.ID)
}

func TestSaveAndLoadProfile_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	p := &config.Profile{ID: "cap-developer"}
	require.NoError(t, config.SaveProfile(dir, p))

	loaded, err := config.LoadProfile(dir)
	require.NoError(t, err)
	assert.Equal(t, "cap-developer", loaded.ID)
}
```

- [ ] **Step 3.2: Run tests to verify they fail**

```bash
go test ./internal/config/... -v
```

Expected: compile error.

- [ ] **Step 3.3: Write `internal/config/config.go`**

```go
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
	Sync        SyncConfig `yaml:"sync"`
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
```

- [ ] **Step 3.4: Run tests to verify they pass**

```bash
go test ./internal/config/... -v
```

Expected: all PASS.

- [ ] **Step 3.5: Commit**

```bash
git add internal/config/
git commit -m "feat: add config and profile load/save"
```

---

## Task 4: Content Pack Loader

**Files:**
- Create: `internal/content/pack.go`
- Create: `internal/content/pack_test.go`
- Create: `internal/content/profile.go`
- Create: `internal/content/profile_test.go`
- Create: `internal/content/loader.go`
- Create: `internal/content/loader_test.go`

- [ ] **Step 4.1: Write failing tests for Pack**

```go
// internal/content/pack_test.go
package content_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/SAP-samples/sap-devs-cli/internal/content"
)

func TestLoadPack_ParsesAllFiles(t *testing.T) {
	dir := makeTempPack(t, "cap", `
id: cap
name: SAP CAP
description: Cloud Application Programming Model
tags: [cloud, node, java]
profiles: [cap-developer]
weight: 100
`, "# CAP Context\nUse CDS for data modelling.", `
- id: cap/docs
  title: CAP Docs
  url: https://cap.cloud.sap
  type: official-docs
`, "## Tip One\nTags: cap,nodejs\nUse cds watch for local development.")

	pack, err := content.LoadPack(dir)
	require.NoError(t, err)
	assert.Equal(t, "cap", pack.ID)
	assert.Equal(t, "SAP CAP", pack.Name)
	assert.Contains(t, pack.ContextMD, "CDS")
	assert.Len(t, pack.Resources, 1)
	assert.Equal(t, "cap/docs", pack.Resources[0].ID)
	assert.Len(t, pack.Tips, 1)
	assert.Contains(t, pack.Tips[0].Content, "cds watch")
}

func TestLoadPack_MissingOptionalFilesOK(t *testing.T) {
	dir := t.TempDir()
	yaml := "id: abap\nname: ABAP\ndescription: ABAP Cloud\ntags: []\nprofiles: []\nweight: 90\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(yaml), 0644))

	pack, err := content.LoadPack(dir)
	require.NoError(t, err)
	assert.Equal(t, "abap", pack.ID)
	assert.Empty(t, pack.ContextMD)
	assert.Empty(t, pack.Tips)
}

// makeTempPack creates a temporary pack directory with the given file contents.
func makeTempPack(t *testing.T, id, packYAML, contextMD, resourcesYAML, tipsMD string) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"pack.yaml":      packYAML,
		"context.md":     contextMD,
		"resources.yaml": resourcesYAML,
		"tips.md":        tipsMD,
	}
	for name, content := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0644))
	}
	return dir
}
```

- [ ] **Step 4.2: Run to verify failure**

```bash
go test ./internal/content/... -v -run TestLoadPack
```

Expected: compile error.

- [ ] **Step 4.3: Write `internal/content/pack.go`**

```go
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
}

// ToolDef is an entry in the tool catalog with version detection rules.
type ToolDef struct {
	ID       string          `yaml:"id"`
	Name     string          `yaml:"name"`
	Required string          `yaml:"required"`
	Detect   ToolDetect      `yaml:"detect"`
	Install  map[string]string `yaml:"install"`
	Docs     string          `yaml:"docs"`
}

// ToolDetect holds the command and regex to detect an installed tool's version.
type ToolDetect struct {
	Command string `yaml:"command"`
	Pattern string `yaml:"pattern"`
}

// MCPServer defines an installable MCP server for this pack's domain.
type MCPServer struct {
	ID          string   `yaml:"id"`
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Install     MCPInstall `yaml:"install"`
	Hosts       []string `yaml:"hosts"`
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
			section = strings.TrimPrefix(section, "## ")
			if strings.TrimSpace(section) == "" {
				continue
			}
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
```

- [ ] **Step 4.4: Write `internal/content/profile.go`**

```go
package content

import (
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// Profile is a developer persona that weights packs by relevance.
type Profile struct {
	ID          string        `yaml:"id"`
	Name        string        `yaml:"name"`
	Description string        `yaml:"description"`
	Packs       []PackWeight  `yaml:"packs"`
	TipTags     []string      `yaml:"tip_tags"`
}

// PackWeight pairs a pack ID with a priority weight.
type PackWeight struct {
	ID     string `yaml:"id"`
	Weight int    `yaml:"weight"`
}

// LoadProfile reads a profile YAML file.
func LoadProfile(path string) (*Profile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var p Profile
	return &p, yaml.Unmarshal(data, &p)
}

// LoadProfiles reads all *.yaml files from profilesDir.
func LoadProfiles(profilesDir string) ([]*Profile, error) {
	entries, err := os.ReadDir(profilesDir)
	if err != nil {
		return nil, err
	}
	var profiles []*Profile
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
			continue
		}
		p, err := LoadProfile(filepath.Join(profilesDir, e.Name()))
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, p)
	}
	return profiles, nil
}

// ApplyWeights returns packs sorted by the profile's weight declarations.
// Packs not mentioned by the profile retain their base weight.
func ApplyWeights(packs []*Pack, profile *Profile) []*Pack {
	if profile == nil {
		return packs
	}
	weightMap := make(map[string]int)
	for _, pw := range profile.Packs {
		weightMap[pw.ID] = pw.Weight
	}
	result := make([]*Pack, len(packs))
	copy(result, packs)
	sort.SliceStable(result, func(i, j int) bool {
		wi := weightMap[result[i].ID]
		if wi == 0 {
			wi = result[i].Weight
		}
		wj := weightMap[result[j].ID]
		if wj == 0 {
			wj = result[j].Weight
		}
		return wi > wj
	})
	return result
}
```

- [ ] **Step 4.5: Write failing tests for Profile**

```go
// internal/content/profile_test.go
package content_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/SAP-samples/sap-devs-cli/internal/content"
)

func TestApplyWeights_OrdersPacksByProfileWeight(t *testing.T) {
	packs := []*content.Pack{
		{ID: "abap", Weight: 90},
		{ID: "cap", Weight: 100},
		{ID: "fiori", Weight: 70},
	}
	profile := &content.Profile{
		Packs: []content.PackWeight{
			{ID: "fiori", Weight: 200},
			{ID: "cap", Weight: 50},
		},
	}
	ordered := content.ApplyWeights(packs, profile)
	assert.Equal(t, "fiori", ordered[0].ID)
	assert.Equal(t, "abap", ordered[1].ID)
	assert.Equal(t, "cap", ordered[2].ID)
}

func TestApplyWeights_NilProfileReturnsUnchanged(t *testing.T) {
	packs := []*content.Pack{{ID: "cap"}, {ID: "abap"}}
	result := content.ApplyWeights(packs, nil)
	assert.Equal(t, "cap", result[0].ID)
}

func TestLoadProfiles_ReadsAllYAML(t *testing.T) {
	dir := t.TempDir()
	yaml1 := "id: cap-developer\nname: CAP Developer\npacks:\n  - id: cap\n    weight: 100\n"
	yaml2 := "id: abap-developer\nname: ABAP Developer\npacks:\n  - id: abap\n    weight: 100\n"
	writeFile(t, filepath.Join(dir, "cap-developer.yaml"), yaml1)
	writeFile(t, filepath.Join(dir, "abap-developer.yaml"), yaml2)

	profiles, err := content.LoadProfiles(dir)
	require.NoError(t, err)
	assert.Len(t, profiles, 2)
}

func writeFile(t *testing.T, path, data string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(data), 0644))
}
```

- [ ] **Step 4.6: Write `internal/content/loader.go`**

```go
package content

import (
	"os"
	"path/filepath"
)

// ContentLoader merges packs from multiple layers: official → company → user → project.
type ContentLoader struct {
	OfficialDir string
	CompanyDir  string // empty if not configured
	UserDir     string
	ProjectDir  string // empty if not in a project
}

// LoadPacks loads and merges packs from all configured layers,
// then orders them by the given profile. Later layers override earlier ones by pack ID.
func (cl *ContentLoader) LoadPacks(profile *Profile) ([]*Pack, error) {
	packMap := make(map[string]*Pack)

	for _, dir := range cl.activeDirs() {
		packsDir := filepath.Join(dir, "packs")
		entries, err := os.ReadDir(packsDir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			pack, err := LoadPack(filepath.Join(packsDir, e.Name()))
			if err != nil {
				return nil, err
			}
			packMap[pack.ID] = pack // later layers override
		}
	}

	packs := make([]*Pack, 0, len(packMap))
	for _, p := range packMap {
		packs = append(packs, p)
	}
	return ApplyWeights(packs, profile), nil
}

// LoadProfiles loads profiles from all configured layers (later layers override).
func (cl *ContentLoader) LoadProfiles() ([]*Profile, error) {
	profileMap := make(map[string]*Profile)
	for _, dir := range cl.activeDirs() {
		profilesDir := filepath.Join(dir, "profiles")
		profiles, err := LoadProfiles(profilesDir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, p := range profiles {
			profileMap[p.ID] = p
		}
	}
	result := make([]*Profile, 0, len(profileMap))
	for _, p := range profileMap {
		result = append(result, p)
	}
	return result, nil
}

// FindProfile returns a profile by ID from all layers, or nil if not found.
func (cl *ContentLoader) FindProfile(id string) (*Profile, error) {
	profiles, err := cl.LoadProfiles()
	if err != nil {
		return nil, err
	}
	for _, p := range profiles {
		if p.ID == id {
			return p, nil
		}
	}
	return nil, nil
}

func (cl *ContentLoader) activeDirs() []string {
	dirs := []string{cl.OfficialDir}
	if cl.CompanyDir != "" {
		dirs = append(dirs, cl.CompanyDir)
	}
	if cl.UserDir != "" {
		dirs = append(dirs, cl.UserDir)
	}
	if cl.ProjectDir != "" {
		dirs = append(dirs, cl.ProjectDir)
	}
	return dirs
}
```

- [ ] **Step 4.7: Write loader tests**

```go
// internal/content/loader_test.go
package content_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/SAP-samples/sap-devs-cli/internal/content"
)

func TestContentLoader_LoadPacks_MergesLayers(t *testing.T) {
	official := makeTempPacksDir(t, map[string]string{
		"cap":  "id: cap\nname: CAP Official\nweight: 100\n",
		"abap": "id: abap\nname: ABAP\nweight: 90\n",
	})
	company := makeTempPacksDir(t, map[string]string{
		"cap": "id: cap\nname: CAP Company Override\nweight: 100\n",
	})

	loader := &content.ContentLoader{
		OfficialDir: official,
		CompanyDir:  company,
	}
	packs, err := loader.LoadPacks(nil)
	require.NoError(t, err)
	assert.Len(t, packs, 2)
	capPack := findPack(packs, "cap")
	require.NotNil(t, capPack)
	assert.Equal(t, "CAP Company Override", capPack.Name)
}

func findPack(packs []*content.Pack, id string) *content.Pack {
	for _, p := range packs {
		if p.ID == id {
			return p
		}
	}
	return nil
}

func makeTempPacksDir(t *testing.T, packs map[string]string) string {
	t.Helper()
	root := t.TempDir()
	packsDir := filepath.Join(root, "packs")
	require.NoError(t, os.MkdirAll(packsDir, 0755))
	for id, yaml := range packs {
		packDir := filepath.Join(packsDir, id)
		require.NoError(t, os.MkdirAll(packDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(packDir, "pack.yaml"), []byte(yaml), 0644))
	}
	return root
}
```

- [ ] **Step 4.8: Run all content tests**

```bash
go test ./internal/content/... -v
```

Expected: all PASS.

- [ ] **Step 4.9: Commit**

```bash
git add internal/content/
git commit -m "feat: add content pack/profile loader with layer merging"
```

---

## Task 5: Tip Selection

**Files:**
- Create: `internal/content/tip.go`
- Create: `internal/content/tip_test.go`

- [ ] **Step 5.1: Write the failing tests**

```go
// internal/content/tip_test.go
package content_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/SAP-samples/sap-devs-cli/internal/content"
)

func TestSelectTip_ReturnsATip(t *testing.T) {
	packs := []*content.Pack{
		{ID: "cap", Tips: []content.Tip{
			{Title: "CAP tip 1", Content: "Use cds watch", Tags: []string{"cap"}},
			{Title: "CAP tip 2", Content: "Use CQL", Tags: []string{"cap", "nodejs"}},
		}},
	}
	tip, err := content.SelectTip(packs, []string{"cap"}, 0)
	require.NoError(t, err)
	assert.NotEmpty(t, tip.Title)
}

func TestSelectTip_FiltersByProfileTags(t *testing.T) {
	packs := []*content.Pack{
		{ID: "cap", Tips: []content.Tip{
			{Title: "CAP tip", Content: "For CAP", Tags: []string{"cap"}},
		}},
		{ID: "abap", Tips: []content.Tip{
			{Title: "ABAP tip", Content: "For ABAP", Tags: []string{"abap"}},
		}},
	}
	// Only request cap-tagged tips
	for i := 0; i < 20; i++ {
		tip, err := content.SelectTip(packs, []string{"cap"}, int64(i))
		require.NoError(t, err)
		assert.Contains(t, tip.Tags, "cap")
		assert.Equal(t, "CAP tip", tip.Title)
	}
}

func TestSelectTip_EmptyPoolReturnsError(t *testing.T) {
	_, err := content.SelectTip(nil, []string{"cap"}, 0)
	assert.Error(t, err)
}

func TestSelectTip_SameSeedReturnsSameTip(t *testing.T) {
	packs := []*content.Pack{
		{ID: "cap", Tips: []content.Tip{
			{Title: "tip A", Tags: []string{"cap"}},
			{Title: "tip B", Tags: []string{"cap"}},
		}},
	}
	tip1, _ := content.SelectTip(packs, []string{"cap"}, 42)
	tip2, _ := content.SelectTip(packs, []string{"cap"}, 42)
	assert.Equal(t, tip1.Title, tip2.Title)
}
```

- [ ] **Step 5.2: Run to verify failure**

```bash
go test ./internal/content/... -v -run TestSelectTip
```

Expected: compile error — `SelectTip` not defined.

- [ ] **Step 5.3: Write `internal/content/tip.go`**

```go
package content

import (
	"errors"
	"math/rand"
)

// SelectTip picks a tip from the filtered pool using the given seed.
// profileTags narrows the pool to tips that share at least one tag with the profile.
// seed 0 means "today" (use time.Now().YearDay() * year as seed for daily consistency).
func SelectTip(packs []*Pack, profileTags []string, seed int64) (*Tip, error) {
	tagSet := make(map[string]bool, len(profileTags))
	for _, t := range profileTags {
		tagSet[t] = true
	}

	var pool []Tip
	for _, pack := range packs {
		for _, tip := range pack.Tips {
			if len(profileTags) == 0 {
				pool = append(pool, tip)
				continue
			}
			for _, tag := range tip.Tags {
				if tagSet[tag] {
					pool = append(pool, tip)
					break
				}
			}
		}
	}

	if len(pool) == 0 {
		return nil, errors.New("no tips available for the current profile tags")
	}

	r := rand.New(rand.NewSource(seed)) //nolint:gosec // non-cryptographic selection
	idx := r.Intn(len(pool))
	return &pool[idx], nil
}
```

- [ ] **Step 5.4: Run tests to verify they pass**

```bash
go test ./internal/content/... -v
```

Expected: all PASS.

- [ ] **Step 5.5: Commit**

```bash
git add internal/content/tip.go internal/content/tip_test.go
git commit -m "feat: add profile-aware tip selection"
```

---

## Task 6: Sync Engine

**Files:**
- Create: `internal/sync/fetcher.go`
- Create: `internal/sync/fetcher_test.go`
- Create: `internal/sync/engine.go`
- Create: `internal/sync/engine_test.go`

- [ ] **Step 6.1: Write failing fetcher test**

```go
// internal/sync/fetcher_test.go
package sync_test

import (
	"archive/zip"
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sapSync "github.com/SAP-samples/sap-devs-cli/internal/sync"
)

func TestFetcher_DownloadsAndExtractsZip(t *testing.T) {
	// Create an in-memory zip with one file
	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)
	f, _ := w.Create("packs/cap/pack.yaml")
	f.Write([]byte("id: cap\nname: CAP\n"))
	w.Close()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		w.Write(buf.Bytes())
	}))
	defer srv.Close()

	dest := t.TempDir()
	err := sapSync.FetchArchive(srv.URL, dest)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dest, "packs", "cap", "pack.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "id: cap")
}
```

- [ ] **Step 6.2: Write `internal/sync/fetcher.go`**

```go
package sync

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// FetchArchive downloads a zip archive from url and extracts it to destDir.
// Existing files are overwritten; directories are created as needed.
func FetchArchive(url, destDir string) error {
	resp, err := http.Get(url) //nolint:gosec // URL comes from user config, not untrusted input
	if err != nil {
		return fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch %s: HTTP %d", url, resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}

	// Strip one leading path component (GitHub archives include repo-name-sha/ prefix)
	strip := zipStripPrefix(zr)

	for _, f := range zr.File {
		name := strings.TrimPrefix(f.Name, strip)
		if name == "" || strings.HasSuffix(name, "/") {
			continue
		}
		dest := filepath.Join(destDir, filepath.FromSlash(name))
		if err := extractFile(f, dest); err != nil {
			return err
		}
	}
	return nil
}

func zipStripPrefix(zr *zip.Reader) string {
	for _, f := range zr.File {
		parts := strings.SplitN(f.Name, "/", 2)
		if len(parts) == 2 {
			return parts[0] + "/"
		}
	}
	return ""
}

func extractFile(f *zip.File, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, rc)
	return err
}
```

- [ ] **Step 6.3: Write failing engine tests**

```go
// internal/sync/engine_test.go
package sync_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sapSync "github.com/SAP-samples/sap-devs-cli/internal/sync"
)

func TestEngine_IsStale_TrueWhenNeverSynced(t *testing.T) {
	dir := t.TempDir()
	eng := sapSync.NewEngine(dir, 24*time.Hour, nil)
	assert.True(t, eng.IsStale("tips"))
}

func TestEngine_IsStale_FalseWhenRecentlySynced(t *testing.T) {
	dir := t.TempDir()
	eng := sapSync.NewEngine(dir, 24*time.Hour, nil)
	require.NoError(t, eng.MarkSynced("tips"))
	assert.False(t, eng.IsStale("tips"))
}

func TestEngine_IsStale_TrueWhenExpired(t *testing.T) {
	dir := t.TempDir()
	// Write a timestamp 2 days ago
	ts := map[string]time.Time{"tips": time.Now().Add(-48 * time.Hour)}
	data, _ := json.Marshal(ts)
	os.WriteFile(filepath.Join(dir, "sync-state.json"), data, 0600)

	eng := sapSync.NewEngine(dir, 24*time.Hour, nil)
	assert.True(t, eng.IsStale("tips"))
}

func TestEngine_IsStale_HonoursPerCategoryTTL(t *testing.T) {
	dir := t.TempDir()
	// resources was synced 2 days ago
	ts := map[string]time.Time{"resources": time.Now().Add(-48 * time.Hour)}
	data, _ := json.Marshal(ts)
	os.WriteFile(filepath.Join(dir, "sync-state.json"), data, 0600)

	// 168h TTL for resources — 2 days is not stale
	eng := sapSync.NewEngine(dir, 24*time.Hour, map[string]time.Duration{"resources": 168 * time.Hour})
	assert.False(t, eng.IsStale("resources"))
	// But tips with default 24h TTL and no sync record is stale
	assert.True(t, eng.IsStale("tips"))
}
```

- [ ] **Step 6.4: Write `internal/sync/engine.go`**

```go
package sync

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Engine tracks per-category sync timestamps and determines staleness.
type Engine struct {
	stateDir string
	ttls     map[string]time.Duration
	defaultTTL time.Duration
}

// NewEngine creates an Engine that stores state in stateDir.
// ttls maps category name → TTL; categories not in the map use defaultTTL.
func NewEngine(stateDir string, defaultTTL time.Duration, ttls map[string]time.Duration) *Engine {
	return &Engine{stateDir: stateDir, defaultTTL: defaultTTL, ttls: ttls}
}

// IsStale reports whether the given category needs a refresh.
func (e *Engine) IsStale(category string) bool {
	ttl := e.defaultTTL
	if t, ok := e.ttls[category]; ok && t > 0 {
		ttl = t
	}
	state := e.loadState()
	last, ok := state[category]
	if !ok {
		return true
	}
	return time.Since(last) > ttl
}

// MarkSynced records the current time as the last sync time for category.
func (e *Engine) MarkSynced(category string) error {
	state := e.loadState()
	state[category] = time.Now()
	return e.saveState(state)
}

func (e *Engine) loadState() map[string]time.Time {
	state := make(map[string]time.Time)
	data, err := os.ReadFile(filepath.Join(e.stateDir, "sync-state.json"))
	if err != nil {
		return state
	}
	_ = json.Unmarshal(data, &state)
	return state
}

func (e *Engine) saveState(state map[string]time.Time) error {
	if err := os.MkdirAll(e.stateDir, 0755); err != nil {
		return err
	}
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(e.stateDir, "sync-state.json"), data, 0600)
}
```

- [ ] **Step 6.5: Run all sync tests**

```bash
go test ./internal/sync/... -v
```

Expected: all PASS.

- [ ] **Step 6.6: Commit**

```bash
git add internal/sync/
git commit -m "feat: add HTTP archive fetcher and TTL-based sync engine"
```

---

## Task 7: Initial Content

**Files:** Create the official knowledge base under `content/`. This is real content, not stub files.

- [ ] **Step 7.1: Create `content/packs/cap/pack.yaml`**

```yaml
id: cap
name: SAP Cloud Application Programming Model
description: Node.js and Java framework for building cloud-native business applications on BTP
tags: [cloud, btp, nodejs, java, odata, cds]
profiles: [cap-developer, btp-developer]
weight: 100
```

- [ ] **Step 7.2: Create `content/packs/cap/context.md`**

```markdown
## SAP CAP (Cloud Application Programming Model)

CAP is SAP's primary framework for building cloud-native business applications on SAP BTP.
It uses CDS (Core Data Services) for data and service definitions, Node.js or Java for service logic.

### Key Tools
- `@sap/cds-dk` — CAP development kit (CLI: `cds`)
- `cds watch` — local dev server with live reload
- `cds deploy` — deploy to database / cloud

### CDS Data Modelling (entity-relationship)
```cds
entity Books : managed {
  key ID     : Integer;
  title      : localized String(111);
  author     : Association to Authors;
}
```

### Service Definition
```cds
service CatalogService @(path:'/browse') {
  @readonly entity Books as SELECT from my.Books;
}
```

### Best Practices
- Define entities in `db/schema.cds`, services in `srv/*.cds`
- Use `cds.ql` for type-safe CQL queries
- Leverage built-in authentication via `@requires` annotations
- Always run `cds lint` before committing
```

- [ ] **Step 7.3: Create `content/packs/cap/tips.md`**

```markdown
## Use cds watch for local development
Tags: cap,nodejs
Run `cds watch` instead of `node server.js` — it reloads on every file change and logs all requests.

## Define managed entities for audit fields
Tags: cap,cds
Add `: managed` to your entities to get `createdAt`, `createdBy`, `modifiedAt`, `modifiedBy` for free.

## Use @readonly in service layer
Tags: cap,odata,security
Expose `@readonly` in the service layer rather than restricting at DB level — keeps schema flexible.

## Check CAP version compatibility
Tags: cap,versions
Run `cds version` to see your full CAP stack versions. Mismatched `@sap/cds` and `@sap/cds-dk` cause subtle errors.
```

- [ ] **Step 7.4: Create `content/packs/cap/resources.yaml`**

```yaml
- id: cap/docs-official
  title: CAP Documentation
  url: https://cap.cloud.sap/docs
  type: official-docs
  tags: [reference, getting-started]

- id: cap/samples-github
  title: CAP Samples on GitHub
  url: https://github.com/SAP-samples/cloud-cap-samples
  type: sample
  tags: [examples, reference]

- id: cap/community-forum
  title: SAP Community — CAP
  url: https://community.sap.com/t5/technology-q-a/questions-related-to-tag/ta-p/9850/tag-id/73555000100800000895
  type: community
  tags: [q&a, help]
```

- [ ] **Step 7.5: Create `content/packs/cap/tools.yaml`**

```yaml
- id: nodejs
  name: Node.js
  required: ">=18.0.0"
  detect:
    command: "node --version"
    pattern: "v(\\d+\\.\\d+\\.\\d+)"
  install:
    windows: "winget install OpenJS.NodeJS.LTS"
    macos: "brew install node@20"
    linux: "nvm install 20"
  docs: "https://nodejs.org"

- id: cds-dk
  name: SAP CDS CLI
  required: ">=7.0.0"
  detect:
    command: "cds --version"
    pattern: "@sap/cds: (\\d+\\.\\d+\\.\\d+)"
  install:
    all: "npm install -g @sap/cds-dk"
  docs: "https://cap.cloud.sap"
```

- [ ] **Step 7.6: Create `content/packs/cap/mcp.yaml`** (placeholder — implemented in Plan 2)

```yaml
# MCP server definitions for CAP — implemented in Plan 2
```

- [ ] **Step 7.7: Create minimal content for `abap` and `btp-core` packs** following the same pattern as cap (pack.yaml, context.md, tips.md, resources.yaml, tools.yaml). Content can be brief in Plan 1 — these packs will be fleshed out in Plan 4.

Minimum for each pack:
- `pack.yaml` — id, name, description, tags, profiles, weight
- `context.md` — 200–400 word overview of the domain with key concepts
- `tips.md` — at least 3 tips with Tags lines
- `resources.yaml` — at least 2 official links
- `tools.yaml` — key tools for that domain

- [ ] **Step 7.8: Create `content/profiles/cap-developer.yaml`**

```yaml
id: cap-developer
name: CAP Developer
description: Building cloud-native apps with SAP CAP on BTP
packs:
  - id: cap
    weight: 100
  - id: btp-core
    weight: 80
  - id: fiori
    weight: 60
  - id: ai-joule
    weight: 40
  - id: integration
    weight: 20
  - id: abap
    weight: 10
tip_tags: [cap, nodejs, odata, cds, btp]
```

- [ ] **Step 7.9: Create `content/profiles/abap-developer.yaml` and `content/profiles/btp-developer.yaml`** with appropriate pack weights.

- [ ] **Step 7.10: Create adapter placeholder stubs**

`content/adapters/claude-code.yaml`:
```yaml
# Adapter definition — implemented in Plan 2
id: claude-code
name: Claude Code
type: file-inject
```

`content/adapters/cursor.yaml`:
```yaml
# Adapter definition — implemented in Plan 2
id: cursor
name: Cursor
type: file-inject
```

- [ ] **Step 7.11: Commit**

```bash
git add content/
git commit -m "content: add initial CAP, ABAP, BTP-core packs and developer profiles"
```

---

## Task 8: Config Command

**Files:**
- Create: `cmd/config.go`

- [ ] **Step 8.1: Write `cmd/config.go`**

```go
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/SAP-samples/sap-devs-cli/internal/config"
	"github.com/SAP-samples/sap-devs-cli/internal/xdg"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage sap-devs configuration",
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Display current configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}
		cfg, err := config.Load(paths.ConfigDir)
		if err != nil {
			return err
		}
		fmt.Printf("company_repo: %s\n", cfg.CompanyRepo)
		fmt.Printf("sync.tips:      %s\n", cfg.Sync.Tips)
		fmt.Printf("sync.tools:     %s\n", cfg.Sync.Tools)
		fmt.Printf("sync.disabled:  %v\n", cfg.Sync.Disabled)
		return nil
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}
		cfg, err := config.Load(paths.ConfigDir)
		if err != nil {
			return err
		}
		switch args[0] {
		case "company_repo":
			cfg.CompanyRepo = args[1]
		default:
			return fmt.Errorf("unknown config key: %s", args[0])
		}
		if err := cfg.Save(paths.ConfigDir); err != nil {
			return err
		}
		fmt.Printf("Set %s = %s\n", args[0], args[1])
		return nil
	},
}

var configCompanyCmd = &cobra.Command{
	Use:   "company <git-url>",
	Short: "Configure the company content repo URL",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}
		cfg, err := config.Load(paths.ConfigDir)
		if err != nil {
			return err
		}
		cfg.CompanyRepo = args[0]
		if err := cfg.Save(paths.ConfigDir); err != nil {
			return err
		}
		fmt.Printf("Company repo set to: %s\n", args[0])
		return nil
	},
}

func init() {
	configCmd.AddCommand(configShowCmd, configSetCmd, configCompanyCmd)
	rootCmd.AddCommand(configCmd)
}
```

- [ ] **Step 8.2: Verify it builds and runs**

```bash
go build ./... && ./sap-devs config show
```

Expected: prints default config values.

- [ ] **Step 8.3: Commit**

```bash
git add cmd/config.go
git commit -m "feat: add config show/set/company commands"
```

---

## Task 9: Profile Command

**Files:**
- Create: `cmd/profile.go`

- [ ] **Step 9.1: Write `cmd/profile.go`**

```go
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/SAP-samples/sap-devs-cli/internal/config"
	"github.com/SAP-samples/sap-devs-cli/internal/content"
	"github.com/SAP-samples/sap-devs-cli/internal/xdg"
)

var profileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Manage your developer profile",
}

var profileListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available developer profiles",
	RunE: func(cmd *cobra.Command, args []string) error {
		loader, err := newContentLoader()
		if err != nil {
			return err
		}
		profiles, err := loader.LoadProfiles()
		if err != nil {
			return err
		}
		for _, p := range profiles {
			fmt.Printf("  %-25s %s\n", p.ID, p.Description)
		}
		return nil
	},
}

var profileSetCmd = &cobra.Command{
	Use:   "set <profile-id>",
	Short: "Set your active developer profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}
		loader, err := newContentLoader()
		if err != nil {
			return err
		}
		p, err := loader.FindProfile(args[0])
		if err != nil {
			return err
		}
		if p == nil {
			return fmt.Errorf("profile %q not found — run 'sap-devs profile list' to see options", args[0])
		}
		if err := config.SaveProfile(paths.ConfigDir, &config.Profile{ID: p.ID}); err != nil {
			return err
		}
		fmt.Printf("Profile set to: %s (%s)\n", p.ID, p.Name)
		return nil
	},
}

var profileShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show your current profile and pack weights",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}
		saved, err := config.LoadProfile(paths.ConfigDir)
		if err != nil {
			return err
		}
		if saved.ID == "" {
			fmt.Println("No profile set. Run 'sap-devs profile list' to see options.")
			return nil
		}
		loader, err := newContentLoader()
		if err != nil {
			return err
		}
		p, err := loader.FindProfile(saved.ID)
		if err != nil {
			return err
		}
		if p == nil {
			fmt.Printf("Profile: %s (definition not found in content)\n", saved.ID)
			return nil
		}
		fmt.Printf("Profile: %s — %s\n\n", p.Name, p.Description)
		fmt.Println("Pack weights:")
		for _, pw := range p.Packs {
			fmt.Printf("  %-20s %d\n", pw.ID, pw.Weight)
		}
		return nil
	},
}

func init() {
	profileCmd.AddCommand(profileListCmd, profileSetCmd, profileShowCmd)
	rootCmd.AddCommand(profileCmd)
}
```

- [ ] **Step 9.2: Add `newContentLoader()` helper to `cmd/root.go`**

This helper is shared across commands. Add to the bottom of `cmd/root.go`:

```go
// newContentLoader constructs a ContentLoader using the platform paths and config.
func newContentLoader() (*content.ContentLoader, error) {
	paths, err := xdg.New()
	if err != nil {
		return nil, err
	}
	cfg, err := config.Load(paths.ConfigDir)
	if err != nil {
		return nil, err
	}
	loader := &content.ContentLoader{
		OfficialDir: filepath.Join(paths.CacheDir, "official"),
		UserDir:     paths.DataDir,
	}
	if cfg.CompanyRepo != "" {
		loader.CompanyDir = filepath.Join(paths.CacheDir, "company")
	}
	// Check for per-project .sap-devs dir
	if _, err := os.Stat(".sap-devs"); err == nil {
		loader.ProjectDir = ".sap-devs"
	}
	return loader, nil
}
```

Also add the necessary imports: `"os"`, `"path/filepath"`, plus the internal packages.

Note: Until `sap-devs sync` has been run, the cache will be empty. During development, seed it by copying the `content/` directory to the cache:

```bash
mkdir -p ~/.cache/sap-devs/official
cp -r content/* ~/.cache/sap-devs/official/
```

(On Windows: `%LOCALAPPDATA%\sap-devs\cache\official`)

- [ ] **Step 9.3: Build and manually test**

```bash
go build -o sap-devs ./...
./sap-devs profile list
./sap-devs profile set cap-developer
./sap-devs profile show
```

Expected: lists profiles, sets profile, shows pack weights.

- [ ] **Step 9.4: Commit**

```bash
git add cmd/profile.go cmd/root.go
git commit -m "feat: add profile list/set/show commands"
```

---

## Task 10: Sync Command

**Files:**
- Create: `cmd/sync.go`

- [ ] **Step 10.1: Write `cmd/sync.go`**

The sync command fetches the official content repo (and company repo if configured). The official repo archive URL follows GitHub Releases convention: `https://<host>/<owner>/<repo>/archive/refs/heads/main.zip`.

```go
package cmd

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/SAP-samples/sap-devs-cli/internal/config"
	sapSync "github.com/SAP-samples/sap-devs-cli/internal/sync"
	"github.com/SAP-samples/sap-devs-cli/internal/xdg"
)

const officialRepoArchive = "https://github.com/SAP-samples/sap-devs-cli/archive/refs/heads/main.zip"

var syncForce bool
var syncCategory string

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Pull latest SAP developer content",
	Long:  `Syncs content from the official repo (and company repo if configured). Respects per-category TTLs unless --force is set.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}
		cfg, err := config.Load(paths.ConfigDir)
		if err != nil {
			return err
		}
		if cfg.Sync.Disabled {
			fmt.Println("Sync is disabled in config.")
			return nil
		}

		categories := allCategories()
		if syncCategory != "" {
			categories = []string{syncCategory}
		}

		officialCache := filepath.Join(paths.CacheDir, "official")
		ttls := map[string]time.Duration{
			"tips":      cfg.Sync.Tips,
			"tools":     cfg.Sync.Tools,
			"advocates": cfg.Sync.Advocates,
			"resources": cfg.Sync.Resources,
			"context":   cfg.Sync.Context,
			"mcp":       cfg.Sync.MCP,
		}
		engine := sapSync.NewEngine(paths.CacheDir, 24*time.Hour, ttls)

		updated := []string{}
		for _, cat := range categories {
			if !syncForce && !engine.IsStale(cat) {
				continue
			}
			fmt.Printf("Syncing %s...\n", cat)
			if err := sapSync.FetchArchive(officialRepoArchive, officialCache); err != nil {
				return fmt.Errorf("sync %s: %w", cat, err)
			}
			if err := engine.MarkSynced(cat); err != nil {
				return err
			}
			updated = append(updated, cat)
		}

		if len(updated) == 0 {
			fmt.Println("All content is up to date.")
		} else {
			fmt.Printf("Updated: %v\n", updated)
		}

		// Sync company repo if configured
		if cfg.CompanyRepo != "" {
			companyCache := filepath.Join(paths.CacheDir, "company")
			companyArchive := cfg.CompanyRepo + "/archive/refs/heads/main.zip"
			fmt.Println("Syncing company repo...")
			if err := sapSync.FetchArchive(companyArchive, companyCache); err != nil {
				fmt.Printf("Warning: company repo sync failed: %v\n", err)
			}
		}
		return nil
	},
}

func allCategories() []string {
	return []string{"tips", "tools", "resources", "context", "mcp", "advocates"}
}

func init() {
	syncCmd.Flags().BoolVar(&syncForce, "force", false, "Re-sync all categories regardless of TTL")
	syncCmd.Flags().StringVar(&syncCategory, "category", "", "Sync a single category only")
	rootCmd.AddCommand(syncCmd)
}
```

- [ ] **Step 10.2: Build and test manually**

```bash
go build -o sap-devs ./...
./sap-devs sync --force
```

Expected: downloads content to cache, prints "Updated: [tips tools resources context mcp advocates]".

- [ ] **Step 10.3: Commit**

```bash
git add cmd/sync.go
git commit -m "feat: add sync command with TTL-aware category refresh"
```

---

## Task 11: Tip Command

**Files:**
- Create: `cmd/tip.go`

- [ ] **Step 11.1: Write `cmd/tip.go`**

```go
package cmd

import (
	"fmt"
	"time"

	"github.com/charmbracelet/glamour"
	"github.com/spf13/cobra"
	"github.com/SAP-samples/sap-devs-cli/internal/config"
	"github.com/SAP-samples/sap-devs-cli/internal/content"
	"github.com/SAP-samples/sap-devs-cli/internal/xdg"
)

var tipCmd = &cobra.Command{
	Use:   "tip",
	Short: "Print a SAP developer tip (add to your shell profile)",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}
		profileCfg, err := config.LoadProfile(paths.ConfigDir)
		if err != nil {
			return err
		}

		loader, err := newContentLoader()
		if err != nil {
			return err
		}

		var activeProfile *content.Profile
		if profileCfg.ID != "" {
			activeProfile, err = loader.FindProfile(profileCfg.ID)
			if err != nil {
				return err
			}
		}

		packs, err := loader.LoadPacks(activeProfile)
		if err != nil {
			return err
		}

		var tipTags []string
		if activeProfile != nil {
			tipTags = activeProfile.TipTags
		}

		// Use year*day as seed for daily consistency
		now := time.Now()
		seed := int64(now.Year()*1000 + now.YearDay())

		tip, err := content.SelectTip(packs, tipTags, seed)
		if err != nil {
			return err
		}

		md := fmt.Sprintf("## 💡 %s\n\n%s\n", tip.Title, tip.Content)
		rendered, err := glamour.Render(md, "dark")
		if err != nil {
			// Fallback to plain output if glamour fails
			fmt.Printf("💡 %s\n\n%s\n", tip.Title, tip.Content)
			return nil
		}
		fmt.Print(rendered)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(tipCmd)
}
```

- [ ] **Step 11.2: Build and run**

```bash
go build -o sap-devs ./...
./sap-devs tip
```

Expected: a rendered Markdown tip printed to terminal.

- [ ] **Step 11.3: Print shell profile instructions**

Verify the output looks good in a real terminal, then document the shell hook:

```bash
# Add to ~/.zshrc or ~/.bashrc:
sap-devs tip

# PowerShell $PROFILE:
sap-devs tip
```

- [ ] **Step 11.4: Commit**

```bash
git add cmd/tip.go
git commit -m "feat: add tip command with glamour rendering and daily seed"
```

---

## Task 12: Init Command

**Files:**
- Create: `cmd/init.go`

- [ ] **Step 12.1: Write `cmd/init.go`**

`init` is an interactive wizard. It must: sync content, prompt for profile, save profile, offer to add `sap-devs tip` to shell profile.

```go
package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/SAP-samples/sap-devs-cli/internal/config"
	"github.com/SAP-samples/sap-devs-cli/internal/content"
	"github.com/SAP-samples/sap-devs-cli/internal/xdg"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "First-time setup wizard",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Welcome to sap-devs — your AI-first SAP developer toolkit.")
		fmt.Println()

		// Step 1: Sync content
		fmt.Println("Step 1/3: Downloading SAP developer content...")
		if err := runSync(true); err != nil {
			fmt.Printf("Warning: content sync failed (%v). Continuing with any cached content.\n", err)
		}

		paths, err := xdg.New()
		if err != nil {
			return err
		}

		// Step 2: Choose profile
		fmt.Println("\nStep 2/3: What kind of SAP developer are you?")
		loader, err := newContentLoader()
		if err != nil {
			return err
		}
		profiles, err := loader.LoadProfiles()
		if err != nil {
			return err
		}
		for i, p := range profiles {
			fmt.Printf("  [%d] %-25s %s\n", i+1, p.ID, p.Description)
		}
		fmt.Print("\nEnter number (or press Enter to skip): ")
		choice := readLine()
		if choice != "" {
			idx := 0
			fmt.Sscanf(choice, "%d", &idx)
			if idx >= 1 && idx <= len(profiles) {
				chosen := profiles[idx-1]
				if err := config.SaveProfile(paths.ConfigDir, &config.Profile{ID: chosen.ID}); err != nil {
					return err
				}
				fmt.Printf("Profile set to: %s\n", chosen.Name)
			}
		}

		// Step 3: Shell profile hook
		fmt.Println("\nStep 3/3: Add SAP tip to your terminal startup?")
		fmt.Println("  This adds 'sap-devs tip' to your shell profile so you see a tip each time you open a terminal.")
		fmt.Print("  Add it? [y/N]: ")
		if strings.ToLower(strings.TrimSpace(readLine())) == "y" {
			if err := addShellHook(); err != nil {
				fmt.Printf("  Could not auto-add hook: %v\n  Add 'sap-devs tip' to your shell profile manually.\n", err)
			} else {
				fmt.Println("  Added. Restart your terminal to see your first tip.")
			}
		}

		fmt.Println("\nSetup complete! Run 'sap-devs --help' to explore all commands.")
		fmt.Println("Next: run 'sap-devs inject' (Plan 2) to inject SAP context into your AI tools.")
		return nil
	},
}

func runSync(force bool) error {
	syncForce = force
	return syncCmd.RunE(syncCmd, nil)
}

func readLine() string {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	return strings.TrimSpace(scanner.Text())
}

func addShellHook() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	// Try common shell rc files
	candidates := []string{".zshrc", ".bashrc", ".bash_profile"}
	for _, rc := range candidates {
		path := home + "/" + rc
		if _, err := os.Stat(path); err == nil {
			f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
			if err != nil {
				return err
			}
			defer f.Close()
			_, err = f.WriteString("\n# SAP developer tips\nsap-devs tip\n")
			return err
		}
	}
	return fmt.Errorf("no shell rc file found")
}

// detectInstalledAITools returns a list of AI tool IDs that appear to be installed.
// Used in Plan 2 to know which tools to inject context into during init.
func detectInstalledAITools() []string {
	var found []string
	checks := map[string]string{
		"claude-code": os.Getenv("HOME") + "/.claude",
		"cursor":      os.Getenv("HOME") + "/.cursor",
	}
	for id, path := range checks {
		if _, err := os.Stat(path); err == nil {
			found = append(found, id)
		}
	}
	return found
}

func init() {
	rootCmd.AddCommand(initCmd)
}
```

- [ ] **Step 12.2: Build and run in a test environment**

```bash
go build -o sap-devs ./...
./sap-devs init
```

Walk through the wizard manually. Verify: content downloads, profile saves, shell hook offer appears.

- [ ] **Step 12.3: Final build and test sweep**

```bash
go test ./...
go build ./...
./sap-devs --help
./sap-devs profile list
./sap-devs tip
```

Expected: all tests pass, binary works end-to-end.

- [ ] **Step 12.4: Final commit**

```bash
git add cmd/init.go
git commit -m "feat: add init wizard with sync, profile selection, and shell hook"
```

---

## Verification

End-to-end smoke test for Plan 1:

```bash
# Build
go build -o sap-devs ./...

# Sync content
./sap-devs sync --force

# Set profile
./sap-devs profile set cap-developer

# Confirm profile
./sap-devs profile show

# Get a tip
./sap-devs tip

# Config
./sap-devs config show
./sap-devs config company https://github.com/myco/sap-content

# Run tests
go test ./...
```

All commands should complete without errors. `sap-devs tip` should render a styled Markdown tip in the terminal.

---

## What's Next

- **Plan 2 — AI Injection:** Adapter engine, `inject` command, file-inject/clipboard-export/mcp-wire for all known AI tools
- **Plan 3 — Developer Tools:** `doctor`, `resources`, `mcp install`, `update`
- **Plan 4 — Content Packs:** Full knowledge content for all 8 domains + advocates
