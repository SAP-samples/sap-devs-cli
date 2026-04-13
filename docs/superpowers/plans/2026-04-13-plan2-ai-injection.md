# Plan 2: AI Injection Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the `sap-devs inject` command and the full adapter engine that writes SAP context into every AI tool a developer uses.

**Architecture:** A data-driven adapter engine in `internal/adapter/` reads YAML adapter definitions from the content layer and dispatches to one of three handler types: `file-inject` (idempotent section replacement in files), `clipboard-export` (render context to clipboard for web AI tools), and `mcp-wire` (write MCP server config to a host's settings.json). The `cmd/inject.go` command resolves the active profile, renders a context string from loaded packs, then calls the engine. All adapter definitions live in `content/adapters/` as pure YAML — no code changes are needed to add a new AI tool.

**Tech Stack:** Go 1.26, cobra, gopkg.in/yaml.v3, os.UserHomeDir for path expansion, `golang.design/x/clipboard` for clipboard access, encoding/json for MCP config patching.

---

## File Map

### New Files

| File | Responsibility |
|---|---|
| `internal/adapter/adapter.go` | Adapter struct and loader — reads YAML from a directory, returns `[]Adapter` |
| `internal/adapter/engine.go` | Engine struct — accepts list of adapters + rendered context; dispatches to handlers |
| `internal/adapter/file_inject.go` | file-inject handler — expand `~`, read file, replace-section, write back |
| `internal/adapter/clipboard.go` | clipboard-export handler — render template, write to clipboard |
| `internal/adapter/mcp_wire.go` | mcp-wire handler — read settings.json, merge mcpServers entry, write back |
| `internal/adapter/adapter_test.go` | Tests for adapter loading |
| `internal/adapter/file_inject_test.go` | Tests for section replace logic |
| `internal/adapter/mcp_wire_test.go` | Tests for JSON merge logic |
| `cmd/inject.go` | `sap-devs inject` command with `--global`, `--project`, `--tool`, `--dry-run` flags |
| `content/adapters/claude-code.yaml` | Full Claude Code adapter (replaces stub) |
| `content/adapters/cursor.yaml` | Full Cursor adapter (replaces stub) |
| `content/adapters/copilot.yaml` | GitHub Copilot adapter |
| `content/adapters/continue.yaml` | Continue.dev adapter |
| `content/adapters/jetbrains-ai.yaml` | JetBrains AI adapter |
| `content/adapters/cody.yaml` | Cody (Sourcegraph) adapter |
| `content/adapters/chatgpt.yaml` | ChatGPT clipboard-export adapter |
| `content/adapters/gemini.yaml` | Gemini clipboard-export adapter |
| `content/adapters/claude-ai.yaml` | Claude.ai clipboard-export adapter |
| `content/adapters/sap-ai-core.yaml` | SAP AI Core clipboard-export adapter |
| `content/adapters/sap-joule.yaml` | SAP Joule clipboard-export adapter |

### Modified Files

| File | Change |
|---|---|
| `cmd/root.go` | Add `newAdapterEngine()` helper |
| `cmd/init.go` | Wire inject step into wizard after profile selection |
| `go.mod` / `go.sum` | Add `golang.design/x/clipboard` dependency |

---

## Task 1: Adapter Struct and Loader

**Files:**
- Create: `internal/adapter/adapter.go`
- Create: `internal/adapter/adapter_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/adapter/adapter_test.go
package adapter_test

import (
    "os"
    "path/filepath"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.tools.sap/developer-relations/sap-devs-cli/internal/adapter"
)

func TestLoadAdapters(t *testing.T) {
    dir := t.TempDir()

    writeYAML(t, filepath.Join(dir, "claude-code.yaml"), `
id: claude-code
name: Claude Code
type: file-inject
targets:
  - scope: global
    path: "~/.claude/CLAUDE.md"
    mode: replace-section
    section: "SAP Developer Context"
detect:
  - path: "~/.claude"
`)

    adapters, err := adapter.LoadAdapters(dir)
    require.NoError(t, err)
    require.Len(t, adapters, 1)
    assert.Equal(t, "claude-code", adapters[0].ID)
    assert.Equal(t, "file-inject", adapters[0].Type)
    require.Len(t, adapters[0].Targets, 1)
    assert.Equal(t, "global", adapters[0].Targets[0].Scope)
    assert.Equal(t, "~/.claude/CLAUDE.md", adapters[0].Targets[0].Path)
    assert.Equal(t, "replace-section", adapters[0].Targets[0].Mode)
    assert.Equal(t, "SAP Developer Context", adapters[0].Targets[0].Section)
}

func TestLoadAdapters_EmptyDir(t *testing.T) {
    dir := t.TempDir()
    adapters, err := adapter.LoadAdapters(dir)
    require.NoError(t, err)
    assert.Empty(t, adapters)
}

func TestLoadAdapters_NonexistentDir(t *testing.T) {
    adapters, err := adapter.LoadAdapters("/no/such/dir")
    require.NoError(t, err)
    assert.Empty(t, adapters)
}

func writeYAML(t *testing.T, path, content string) {
    t.Helper()
    require.NoError(t, os.WriteFile(path, []byte(content), 0644))
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd d:/projects/sap-devs-cli && go test ./internal/adapter/... -v`
Expected: FAIL — package does not exist yet

- [ ] **Step 3: Implement adapter.go**

```go
// internal/adapter/adapter.go
package adapter

import (
    "os"
    "path/filepath"

    "gopkg.in/yaml.v3"
)

// Adapter defines how to inject SAP context into a specific AI tool.
type Adapter struct {
    ID           string         `yaml:"id"`
    Name         string         `yaml:"name"`
    Type         string         `yaml:"type"`    // file-inject | clipboard-export | mcp-wire
    Targets      []Target       `yaml:"targets"`
    ClipFormat   string         `yaml:"format"`
    Template     string         `yaml:"template"`
    Instructions string         `yaml:"instructions"`
    MCPConfig    *MCPConfig     `yaml:"mcp_config,omitempty"`
    Detect       []DetectRule   `yaml:"detect"`
}

// Target is a single file injection target.
type Target struct {
    Scope   string `yaml:"scope"`   // global | project
    Path    string `yaml:"path"`
    Mode    string `yaml:"mode"`    // replace-section | append
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/adapter/... -v -run TestLoadAdapters`
Expected: PASS (3 tests)

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/adapter.go internal/adapter/adapter_test.go
git commit -m "feat: add adapter loader for AI tool injection definitions"
```

---

## Task 2: File-Inject Handler (replace-section logic)

**Files:**
- Create: `internal/adapter/file_inject.go`
- Create: `internal/adapter/file_inject_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/adapter/file_inject_test.go
package adapter_test

import (
    "os"
    "path/filepath"
    "strings"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.tools.sap/developer-relations/sap-devs-cli/internal/adapter"
)

func TestReplaceSection_FirstInject(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "CLAUDE.md")
    require.NoError(t, os.WriteFile(path, []byte("# My Notes\n\nExisting content.\n"), 0644))

    err := adapter.ReplaceSection(path, "SAP Developer Context", "## SAP Tips\n\nUse CAP.\n", false)
    require.NoError(t, err)

    got, err := os.ReadFile(path)
    require.NoError(t, err)
    content := string(got)

    assert.Contains(t, content, "# My Notes")
    assert.Contains(t, content, "Existing content.")
    assert.Contains(t, content, "<!-- sap-devs:start:SAP Developer Context -->")
    assert.Contains(t, content, "## SAP Tips")
    assert.Contains(t, content, "Use CAP.")
    assert.Contains(t, content, "<!-- sap-devs:end:SAP Developer Context -->")
}

func TestReplaceSection_Idempotent(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "CLAUDE.md")

    // First inject
    require.NoError(t, adapter.ReplaceSection(path, "SAP Developer Context", "v1 content", false))
    // Second inject with different content
    require.NoError(t, adapter.ReplaceSection(path, "SAP Developer Context", "v2 content", false))

    got, _ := os.ReadFile(path)
    content := string(got)

    // Only one section
    assert.Equal(t, 1, strings.Count(content, "<!-- sap-devs:start:SAP Developer Context -->"))
    assert.Contains(t, content, "v2 content")
    assert.NotContains(t, content, "v1 content")
}

func TestReplaceSection_CreatesFile(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "subdir", "CLAUDE.md")

    err := adapter.ReplaceSection(path, "SAP Developer Context", "content", false)
    require.NoError(t, err)

    _, err = os.Stat(path)
    assert.NoError(t, err)
}

func TestReplaceSection_DryRun(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "CLAUDE.md")

    err := adapter.ReplaceSection(path, "SAP Developer Context", "injected", true)
    require.NoError(t, err)

    // File should not be created in dry-run
    _, err = os.Stat(path)
    assert.True(t, os.IsNotExist(err))
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/adapter/... -v -run TestReplaceSection`
Expected: FAIL — `ReplaceSection` not defined

- [ ] **Step 3: Implement file_inject.go**

```go
// internal/adapter/file_inject.go
package adapter

import (
    "fmt"
    "os"
    "path/filepath"
    "strings"
)

const markerFmt = "<!-- sap-devs:start:%s -->"
const markerEndFmt = "<!-- sap-devs:end:%s -->"

// ReplaceSection writes `content` into `filePath` between HTML comment markers
// for the named section. If the section already exists it is replaced in-place;
// otherwise it is appended. Parent directories are created as needed.
// When dryRun is true the function prints what it would do but writes nothing.
func ReplaceSection(filePath, section, content string, dryRun bool) error {
    start := fmt.Sprintf(markerFmt, section)
    end := fmt.Sprintf(markerEndFmt, section)
    block := start + "\n" + strings.TrimRight(content, "\n") + "\n" + end + "\n"

    if dryRun {
        fmt.Printf("[dry-run] would write section %q to %s\n", section, filePath)
        return nil
    }

    // Read existing content (OK if file doesn't exist)
    existing := ""
    data, err := os.ReadFile(filePath)
    if err == nil {
        existing = string(data)
    } else if !os.IsNotExist(err) {
        return err
    }

    var result string
    startIdx := strings.Index(existing, start)
    endIdx := strings.Index(existing, end)
    if startIdx != -1 && endIdx != -1 && endIdx > startIdx {
        // Replace in-place; consume the trailing newline after the end marker if present
        afterEnd := endIdx + len(end)
        if afterEnd < len(existing) && existing[afterEnd] == '\n' {
            afterEnd++
        }
        result = existing[:startIdx] + block + existing[afterEnd:]
    } else {
        // Append with separator
        if existing != "" && !strings.HasSuffix(existing, "\n") {
            existing += "\n"
        }
        if existing != "" {
            existing += "\n"
        }
        result = existing + block
    }

    if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
        return err
    }
    return os.WriteFile(filePath, []byte(result), 0644)
}

// ExpandHome replaces a leading ~ with the user's home directory.
func ExpandHome(path string) (string, error) {
    if !strings.HasPrefix(path, "~/") && path != "~" {
        return path, nil
    }
    home, err := os.UserHomeDir()
    if err != nil {
        return "", err
    }
    return filepath.Join(home, path[1:]), nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/adapter/... -v -run TestReplaceSection`
Expected: PASS (4 tests)

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/file_inject.go internal/adapter/file_inject_test.go
git commit -m "feat: add idempotent replace-section file injection"
```

---

## Task 3: MCP-Wire Handler

**Files:**
- Create: `internal/adapter/mcp_wire.go`
- Create: `internal/adapter/mcp_wire_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/adapter/mcp_wire_test.go
package adapter_test

import (
    "encoding/json"
    "os"
    "path/filepath"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.tools.sap/developer-relations/sap-devs-cli/internal/adapter"
    "github.tools.sap/developer-relations/sap-devs-cli/internal/content"
)

func TestWriteMCPConfig_NewFile(t *testing.T) {
    dir := t.TempDir()
    settingsPath := filepath.Join(dir, "settings.json")

    server := content.MCPServer{
        ID:          "cap-mcp",
        Name:        "CAP MCP Server",
        Description: "CAP tools",
        Install: content.MCPInstall{
            Command: "npx",
            Args:    []string{"-y", "@sap/cap-mcp-server"},
        },
    }

    err := adapter.WriteMCPConfig(settingsPath, "mcpServers", server, false)
    require.NoError(t, err)

    data, err := os.ReadFile(settingsPath)
    require.NoError(t, err)

    var result map[string]interface{}
    require.NoError(t, json.Unmarshal(data, &result))

    servers, ok := result["mcpServers"].(map[string]interface{})
    require.True(t, ok)
    entry, ok := servers["cap-mcp"].(map[string]interface{})
    require.True(t, ok)
    assert.Equal(t, "npx", entry["command"])
}

func TestWriteMCPConfig_Idempotent(t *testing.T) {
    dir := t.TempDir()
    settingsPath := filepath.Join(dir, "settings.json")

    // Write existing settings
    require.NoError(t, os.WriteFile(settingsPath, []byte(`{"theme":"dark","mcpServers":{}}`), 0644))

    server := content.MCPServer{
        ID:      "cap-mcp",
        Install: content.MCPInstall{Command: "npx", Args: []string{"-y", "@sap/cap-mcp-server"}},
    }

    require.NoError(t, adapter.WriteMCPConfig(settingsPath, "mcpServers", server, false))
    require.NoError(t, adapter.WriteMCPConfig(settingsPath, "mcpServers", server, false))

    data, _ := os.ReadFile(settingsPath)
    var result map[string]interface{}
    require.NoError(t, json.Unmarshal(data, &result))

    // Existing key preserved
    assert.Equal(t, "dark", result["theme"])
    // Server is present once
    servers := result["mcpServers"].(map[string]interface{})
    assert.Len(t, servers, 1)
}

func TestWriteMCPConfig_DryRun(t *testing.T) {
    dir := t.TempDir()
    settingsPath := filepath.Join(dir, "settings.json")

    server := content.MCPServer{ID: "cap-mcp", Install: content.MCPInstall{Command: "npx"}}
    require.NoError(t, adapter.WriteMCPConfig(settingsPath, "mcpServers", server, true))

    _, err := os.Stat(settingsPath)
    assert.True(t, os.IsNotExist(err))
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/adapter/... -v -run TestWriteMCPConfig`
Expected: FAIL — `WriteMCPConfig` not defined

- [ ] **Step 3: Implement mcp_wire.go**

```go
// internal/adapter/mcp_wire.go
package adapter

import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"

    "github.tools.sap/developer-relations/sap-devs-cli/internal/content"
)

// WriteMCPConfig merges an MCP server entry into the host's settings JSON file.
// The file is created if it does not exist. Existing keys are preserved.
// When dryRun is true, prints what would happen and returns without writing.
func WriteMCPConfig(settingsPath, key string, server content.MCPServer, dryRun bool) error {
    if dryRun {
        fmt.Printf("[dry-run] would add MCP server %q to %s[%s]\n", server.ID, settingsPath, key)
        return nil
    }

    // Read existing JSON (or start empty)
    var root map[string]interface{}
    data, err := os.ReadFile(settingsPath)
    if err == nil {
        if err := json.Unmarshal(data, &root); err != nil {
            return fmt.Errorf("parse %s: %w", settingsPath, err)
        }
    } else if os.IsNotExist(err) {
        root = make(map[string]interface{})
    } else {
        return err
    }

    // Get or create the mcpServers map
    var servers map[string]interface{}
    if v, ok := root[key]; ok {
        if m, ok := v.(map[string]interface{}); ok {
            servers = m
        }
    }
    if servers == nil {
        servers = make(map[string]interface{})
    }

    // Build the server entry
    entry := map[string]interface{}{
        "command": server.Install.Command,
        "args":    server.Install.Args,
    }
    servers[server.ID] = entry
    root[key] = servers

    // Write back with indentation
    out, err := json.MarshalIndent(root, "", "  ")
    if err != nil {
        return err
    }
    if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
        return err
    }
    return os.WriteFile(settingsPath, out, 0644)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/adapter/... -v -run TestWriteMCPConfig`
Expected: PASS (3 tests)

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/mcp_wire.go internal/adapter/mcp_wire_test.go
git commit -m "feat: add MCP server config writer for mcp-wire adapters"
```

---

## Task 4: Clipboard-Export Handler

**Files:**
- Create: `internal/adapter/clipboard.go`

Note: The `golang.design/x/clipboard` package requires CGo on Linux and a display server (X11/Wayland) or macOS/Windows. On CI (headless Linux), clipboard operations are expected to fail gracefully. The function falls back to printing the content to stdout when clipboard access fails.

- [ ] **Step 1: Add clipboard dependency**

Run: `cd d:/projects/sap-devs-cli && go get golang.design/x/clipboard`

- [ ] **Step 2: Implement clipboard.go**

```go
// internal/adapter/clipboard.go
package adapter

import (
    "fmt"
    "strings"

    "golang.design/x/clipboard"
)

// ExportToClipboard writes content to the system clipboard.
// If clipboard access is unavailable (headless, no display), it falls back
// to printing the content to stdout with usage instructions.
func ExportToClipboard(content, instructions string, dryRun bool) error {
    if dryRun {
        fmt.Printf("[dry-run] would copy %d bytes to clipboard\n", len(content))
        fmt.Printf("[dry-run] instructions: %s\n", instructions)
        return nil
    }

    if err := clipboard.Init(); err != nil {
        // Clipboard unavailable — print to stdout as fallback
        fmt.Printf("--- SAP Developer Context ---\n%s\n--- End ---\n", strings.TrimSpace(content))
        if instructions != "" {
            fmt.Printf("\n%s\n", instructions)
        }
        return nil
    }

    clipboard.Write(clipboard.FmtText, []byte(content))
    fmt.Println("SAP developer context copied to clipboard.")
    if instructions != "" {
        fmt.Printf("%s\n", instructions)
    }
    return nil
}
```

- [ ] **Step 3: Verify it compiles**

Run: `go build ./...`
Expected: clean build (no errors)

- [ ] **Step 4: Commit**

```bash
git add internal/adapter/clipboard.go go.mod go.sum
git commit -m "feat: add clipboard-export handler with stdout fallback"
```

---

## Task 5: Adapter Engine

**Files:**
- Create: `internal/adapter/engine.go`

The engine connects everything: given a list of adapters and a rendered context string, it runs the appropriate handler for each adapter based on type and scope flags.

- [ ] **Step 1: Write failing tests**

```go
// Add to internal/adapter/adapter_test.go

func TestEngine_FileInject_DryRun(t *testing.T) {
    dir := t.TempDir()
    targetFile := filepath.Join(dir, "CLAUDE.md")

    adapters := []adapter.Adapter{
        {
            ID:   "test-tool",
            Type: "file-inject",
            Targets: []adapter.Target{
                {Scope: "global", Path: targetFile, Mode: "replace-section", Section: "SAP Dev"},
            },
        },
    }

    engine := adapter.NewEngine(adapters, "# SAP Context\nUse CAP.", adapter.Options{
        Scope:  "global",
        DryRun: true,
    })
    require.NoError(t, engine.Run())
    _, err := os.Stat(targetFile)
    assert.True(t, os.IsNotExist(err), "dry-run should not create file")
}

func TestEngine_SkipsWrongScope(t *testing.T) {
    dir := t.TempDir()
    projectFile := filepath.Join(dir, "proj.md")

    adapters := []adapter.Adapter{
        {
            ID:   "test-tool",
            Type: "file-inject",
            Targets: []adapter.Target{
                {Scope: "project", Path: projectFile, Mode: "replace-section", Section: "SAP Dev"},
            },
        },
    }

    // Running with global scope — project target should be skipped
    engine := adapter.NewEngine(adapters, "content", adapter.Options{Scope: "global"})
    require.NoError(t, engine.Run())
    _, err := os.Stat(projectFile)
    assert.True(t, os.IsNotExist(err), "global scope should skip project targets")
}

func TestEngine_FilterByTool(t *testing.T) {
    dir := t.TempDir()
    fileA := filepath.Join(dir, "a.md")
    fileB := filepath.Join(dir, "b.md")

    adapters := []adapter.Adapter{
        {
            ID:   "tool-a",
            Type: "file-inject",
            Targets: []adapter.Target{{Scope: "global", Path: fileA, Mode: "replace-section", Section: "S"}},
        },
        {
            ID:   "tool-b",
            Type: "file-inject",
            Targets: []adapter.Target{{Scope: "global", Path: fileB, Mode: "replace-section", Section: "S"}},
        },
    }

    engine := adapter.NewEngine(adapters, "content", adapter.Options{Scope: "global", ToolFilter: "tool-a"})
    require.NoError(t, engine.Run())

    _, errA := os.Stat(fileA)
    _, errB := os.Stat(fileB)
    assert.NoError(t, errA, "tool-a target should be written")
    assert.True(t, os.IsNotExist(errB), "tool-b target should be skipped")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/adapter/... -v -run TestEngine`
Expected: FAIL — `NewEngine` not defined

- [ ] **Step 3: Implement engine.go**

```go
// internal/adapter/engine.go
package adapter

import "fmt"

// Options controls inject scope, filtering, and dry-run behaviour.
type Options struct {
    Scope      string // "global" | "project"
    ToolFilter string // if non-empty, only run this adapter ID
    DryRun     bool
}

// Engine runs injection for a set of adapters with a given rendered context.
type Engine struct {
    adapters []Adapter
    context  string
    opts     Options
}

// NewEngine constructs an Engine.
func NewEngine(adapters []Adapter, renderedContext string, opts Options) *Engine {
    return &Engine{adapters: adapters, context: renderedContext, opts: opts}
}

// Run dispatches to the appropriate handler for each adapter.
func (e *Engine) Run() error {
    for _, a := range e.adapters {
        if e.opts.ToolFilter != "" && a.ID != e.opts.ToolFilter {
            continue
        }
        switch a.Type {
        case "file-inject":
            if err := e.runFileInject(a); err != nil {
                return fmt.Errorf("adapter %s: %w", a.ID, err)
            }
        case "clipboard-export":
            // clipboard-export is only for global scope
            if e.opts.Scope == "project" {
                continue
            }
            if err := ExportToClipboard(e.context, a.Instructions, e.opts.DryRun); err != nil {
                return fmt.Errorf("adapter %s: %w", a.ID, err)
            }
        case "mcp-wire":
            // mcp-wire is handled by the mcp command; inject skips it
        }
    }
    return nil
}

func (e *Engine) runFileInject(a Adapter) error {
    for _, target := range a.Targets {
        if target.Scope != e.opts.Scope {
            continue
        }
        path, err := ExpandHome(target.Path)
        if err != nil {
            return err
        }
        if target.Mode == "replace-section" {
            if err := ReplaceSection(path, target.Section, e.context, e.opts.DryRun); err != nil {
                return err
            }
        }
    }
    return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/adapter/... -v`
Expected: PASS (all tests)

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/engine.go
git commit -m "feat: add adapter engine dispatching file-inject and clipboard-export"
```

---

## Task 6: Context Rendering

Context rendering transforms loaded packs into a single Markdown string suitable for injection. It belongs in `internal/content/` as it operates on packs.

**Files:**
- Create: `internal/content/render.go`
- Create: `internal/content/render_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/content/render_test.go
package content_test

import (
    "strings"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.tools.sap/developer-relations/sap-devs-cli/internal/content"
)

func TestRenderContext_BasicPacks(t *testing.T) {
    packs := []*content.Pack{
        {ID: "cap", Name: "CAP", ContextMD: "## CAP\n\nUse @sap/cds."},
        {ID: "btp-core", Name: "BTP Core", ContextMD: "## BTP Core\n\nDeploy to Cloud Foundry."},
    }

    out := content.RenderContext(packs, nil)

    assert.Contains(t, out, "Use @sap/cds.")
    assert.Contains(t, out, "Deploy to Cloud Foundry.")
    // CAP should appear before BTP Core (order preserved)
    assert.Less(t, strings.Index(out, "Use @sap/cds."), strings.Index(out, "Deploy to Cloud Foundry."))
}

func TestRenderContext_WithProfile(t *testing.T) {
    packs := []*content.Pack{
        {ID: "cap", Name: "CAP", ContextMD: "CAP context."},
    }
    profile := &content.Profile{
        ID:          "cap-developer",
        Name:        "CAP Developer",
        Description: "Building cloud-native apps with SAP CAP on BTP",
    }

    out := content.RenderContext(packs, profile)

    assert.Contains(t, out, "CAP Developer")
    assert.Contains(t, out, "CAP context.")
}

func TestRenderContext_EmptyPacks(t *testing.T) {
    out := content.RenderContext(nil, nil)
    assert.NotEmpty(t, out) // Always emits the SAP header
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/content/... -v -run TestRenderContext`
Expected: FAIL — `RenderContext` not defined

- [ ] **Step 3: Implement render.go**

```go
// internal/content/render.go
package content

import (
    "fmt"
    "strings"
)

// RenderContext builds the Markdown string injected into AI tool configuration.
// Packs are rendered in the order provided (caller applies profile weights first).
func RenderContext(packs []*Pack, profile *Profile) string {
    var b strings.Builder

    b.WriteString("# SAP Developer Context\n\n")
    b.WriteString("This context is maintained by sap-devs and provides up-to-date SAP developer knowledge.\n\n")

    if profile != nil {
        b.WriteString(fmt.Sprintf("**Developer Profile:** %s — %s\n\n", profile.Name, profile.Description))
    }

    for _, p := range packs {
        if strings.TrimSpace(p.ContextMD) == "" {
            continue
        }
        b.WriteString(strings.TrimRight(p.ContextMD, "\n"))
        b.WriteString("\n\n")
    }

    return strings.TrimRight(b.String(), "\n") + "\n"
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/content/... -v -run TestRenderContext`
Expected: PASS (3 tests)

- [ ] **Step 5: Commit**

```bash
git add internal/content/render.go internal/content/render_test.go
git commit -m "feat: add context renderer that merges packs into injection-ready Markdown"
```

---

## Task 7: Adapter YAML Files

**Files:**
- Modify: `content/adapters/claude-code.yaml`
- Modify: `content/adapters/cursor.yaml`
- Create: `content/adapters/copilot.yaml`
- Create: `content/adapters/continue.yaml`
- Create: `content/adapters/jetbrains-ai.yaml`
- Create: `content/adapters/cody.yaml`
- Create: `content/adapters/chatgpt.yaml`
- Create: `content/adapters/gemini.yaml`
- Create: `content/adapters/claude-ai.yaml`
- Create: `content/adapters/sap-ai-core.yaml`
- Create: `content/adapters/sap-joule.yaml`

- [ ] **Step 1: Write claude-code.yaml (replaces stub)**

```yaml
# content/adapters/claude-code.yaml
id: claude-code
name: Claude Code
type: file-inject
targets:
  - scope: global
    path: "~/.claude/CLAUDE.md"
    mode: replace-section
    section: "SAP Developer Context"
  - scope: project
    path: "./CLAUDE.md"
    mode: replace-section
    section: "SAP Developer Context"
detect:
  - command: "claude --version"
  - path: "~/.claude"
```

- [ ] **Step 2: Write cursor.yaml (replaces stub)**

```yaml
# content/adapters/cursor.yaml
id: cursor
name: Cursor
type: file-inject
targets:
  - scope: global
    path: "~/.cursor/rules/sap-developer-context.mdc"
    mode: replace-section
    section: "SAP Developer Context"
  - scope: project
    path: ".cursorrules"
    mode: replace-section
    section: "SAP Developer Context"
detect:
  - path: "~/.cursor"
  - command: "cursor --version"
```

- [ ] **Step 3: Write copilot.yaml**

```yaml
# content/adapters/copilot.yaml
id: copilot
name: GitHub Copilot
type: file-inject
targets:
  - scope: project
    path: ".github/copilot-instructions.md"
    mode: replace-section
    section: "SAP Developer Context"
detect:
  - path: "~/.config/github-copilot"
  - command: "gh extension list"
```

- [ ] **Step 4: Write continue.yaml**

```yaml
# content/adapters/continue.yaml
id: continue
name: Continue.dev
type: file-inject
targets:
  - scope: global
    path: "~/.continue/config.md"
    mode: replace-section
    section: "SAP Developer Context"
  - scope: project
    path: ".continue/config.md"
    mode: replace-section
    section: "SAP Developer Context"
detect:
  - path: "~/.continue"
```

- [ ] **Step 5: Write jetbrains-ai.yaml**

```yaml
# content/adapters/jetbrains-ai.yaml
id: jetbrains-ai
name: JetBrains AI Assistant
type: file-inject
targets:
  - scope: project
    path: ".idea/ai-context.md"
    mode: replace-section
    section: "SAP Developer Context"
detect:
  - path: "~/.config/JetBrains"
```

- [ ] **Step 6: Write cody.yaml**

```yaml
# content/adapters/cody.yaml
id: cody
name: Sourcegraph Cody
type: file-inject
targets:
  - scope: project
    path: ".cody/context.md"
    mode: replace-section
    section: "SAP Developer Context"
detect:
  - path: "~/.config/cody"
```

- [ ] **Step 7: Write clipboard-export adapters (chatgpt, gemini, claude-ai, sap-ai-core, sap-joule)**

```yaml
# content/adapters/chatgpt.yaml
id: chatgpt
name: ChatGPT
type: clipboard-export
format: markdown
instructions: "Paste this into ChatGPT → Settings → Custom Instructions → 'What would you like ChatGPT to know about you?'"
```

```yaml
# content/adapters/gemini.yaml
id: gemini
name: Google Gemini
type: clipboard-export
format: markdown
instructions: "Paste this into Gemini → Settings → Custom Instructions or into your Gemini for Google Workspace prompt."
```

```yaml
# content/adapters/claude-ai.yaml
id: claude-ai
name: Claude.ai
type: clipboard-export
format: markdown
instructions: "Paste this into claude.ai → Settings → Custom Instructions."
```

```yaml
# content/adapters/sap-ai-core.yaml
id: sap-ai-core
name: SAP AI Core
type: clipboard-export
format: markdown
instructions: "Paste this as a system message in your SAP AI Core deployment prompt template."
```

```yaml
# content/adapters/sap-joule.yaml
id: sap-joule
name: SAP Joule
type: clipboard-export
format: markdown
instructions: "Paste this context into your SAP Joule workspace configuration prompt."
```

- [ ] **Step 8: Verify adapters load correctly**

Run: `go test ./internal/adapter/... -v`
Expected: PASS (all existing tests still pass)

- [ ] **Step 9: Commit**

```bash
git add content/adapters/
git commit -m "feat: add full adapter YAML definitions for 11 AI tools"
```

---

## Task 8: cmd/inject.go Command

**Files:**
- Create: `cmd/inject.go`
- Modify: `cmd/root.go` (add `newAdapterEngine` helper)

- [ ] **Step 1: Write the adapter engine helper in root.go**

Add to `cmd/root.go` after the `newContentLoader` function:

```go
// newAdapterEngine constructs an adapter engine from all configured adapter layers.
// It reads adapter YAML files from: official cache, company cache, and user data dir.
func newAdapterEngine(renderedContext string, opts adapter.Options) (*adapter.Engine, error) {
    paths, err := xdg.New()
    if err != nil {
        return nil, err
    }
    cfg, err := config.Load(paths.ConfigDir)
    if err != nil {
        return nil, err
    }

    var allAdapters []adapter.Adapter

    // Official adapters
    officialAdaptersDir := filepath.Join(paths.CacheDir, "official", "content", "adapters")
    if a, err := adapter.LoadAdapters(officialAdaptersDir); err == nil {
        allAdapters = append(allAdapters, a...)
    }

    // Company adapters (override official by ID)
    if cfg.CompanyRepo != "" {
        companyAdaptersDir := filepath.Join(paths.CacheDir, "company", "content", "adapters")
        if a, err := adapter.LoadAdapters(companyAdaptersDir); err == nil {
            allAdapters = mergeAdapters(allAdapters, a)
        }
    }

    // Fall back to bundled adapters in the binary's working dir (dev mode)
    if len(allAdapters) == 0 {
        if a, err := adapter.LoadAdapters("content/adapters"); err == nil {
            allAdapters = a
        }
    }

    return adapter.NewEngine(allAdapters, renderedContext, opts), nil
}

// mergeAdapters merges src into dst, overriding by adapter ID.
func mergeAdapters(dst, src []adapter.Adapter) []adapter.Adapter {
    index := make(map[string]int)
    for i, a := range dst {
        index[a.ID] = i
    }
    for _, a := range src {
        if i, ok := index[a.ID]; ok {
            dst[i] = a
        } else {
            dst = append(dst, a)
        }
    }
    return dst
}
```

Also add the import for `adapter` package to root.go:
```go
"github.tools.sap/developer-relations/sap-devs-cli/internal/adapter"
```

- [ ] **Step 2: Implement cmd/inject.go**

```go
// cmd/inject.go
package cmd

import (
    "fmt"

    "github.com/spf13/cobra"
    "github.tools.sap/developer-relations/sap-devs-cli/internal/adapter"
    "github.tools.sap/developer-relations/sap-devs-cli/internal/config"
    "github.tools.sap/developer-relations/sap-devs-cli/internal/content"
    "github.tools.sap/developer-relations/sap-devs-cli/internal/xdg"
)

var (
    injectGlobal  bool
    injectProject bool
    injectTool    string
    injectDryRun  bool
)

var injectCmd = &cobra.Command{
    Use:   "inject",
    Short: "Push SAP context to your AI tools",
    Long: `Inject up-to-date SAP developer context into all detected AI tools.

By default, injects at global (user) scope into tools such as Claude Code,
Cursor, and GitHub Copilot. Use --project to inject into project-level files
(CLAUDE.md, .cursorrules, etc.) in the current directory.`,
    RunE: func(cmd *cobra.Command, args []string) error {
        // Resolve scope
        scope := "global"
        if injectProject {
            scope = "project"
        }

        // Load content
        loader, err := newContentLoader()
        if err != nil {
            return err
        }

        paths, err := xdg.New()
        if err != nil {
            return err
        }
        configProfile, err := config.LoadProfile(paths.ConfigDir)
        if err != nil {
            return err
        }

        var activeProfile *content.Profile
        if configProfile.ID != "" {
            activeProfile, err = loader.FindProfile(configProfile.ID)
            if err != nil {
                return err
            }
        }

        packs, err := loader.LoadPacks(activeProfile)
        if err != nil {
            return err
        }

        rendered := content.RenderContext(packs, activeProfile)

        // Build and run engine
        opts := adapter.Options{
            Scope:      scope,
            ToolFilter: injectTool,
            DryRun:     injectDryRun,
        }
        engine, err := newAdapterEngine(rendered, opts)
        if err != nil {
            return err
        }

        if injectDryRun {
            fmt.Println("[dry-run] no files will be modified")
        }

        if err := engine.Run(); err != nil {
            return err
        }

        if !injectDryRun {
            fmt.Printf("SAP developer context injected (%s scope).\n", scope)
            if injectTool == "" {
                fmt.Println("Run 'sap-devs inject --dry-run' to preview changes before writing.")
            }
        }
        return nil
    },
}

func init() {
    injectCmd.Flags().BoolVar(&injectGlobal, "global", true, "inject at user (global) scope (default)")
    injectCmd.Flags().BoolVar(&injectProject, "project", false, "inject at project scope (current directory)")
    injectCmd.Flags().StringVar(&injectTool, "tool", "", "inject into a specific tool only (e.g. claude-code)")
    injectCmd.Flags().BoolVar(&injectDryRun, "dry-run", false, "preview changes without writing files")
    injectCmd.MarkFlagsMutuallyExclusive("global", "project")
    rootCmd.AddCommand(injectCmd)
}
```

- [ ] **Step 3: Verify it builds**

Run: `go build ./...`
Expected: clean build

- [ ] **Step 4: Commit**

```bash
git add cmd/inject.go cmd/root.go
git commit -m "feat: add inject command with global/project/tool/dry-run flags"
```

---

## Task 9: Wire inject into init wizard

The `init` wizard should offer to inject SAP context as step 3, pushing inject after profile selection. The shell hook step becomes step 4.

**Files:**
- Modify: `cmd/init.go`

- [ ] **Step 1: Update init.go to add inject step**

Replace the existing `Step 3/3` (shell hook) block with:

```go
// Step 3: Inject into AI tools
fmt.Println("\nStep 3/4: Inject SAP context into your AI tools?")
fmt.Println("  This writes SAP developer context to your AI tool configuration files.")
fmt.Print("  Inject now? [Y/n]: ")
if answer := strings.ToLower(strings.TrimSpace(readLine())); answer == "" || answer == "y" {
    if err := runInjectGlobal(); err != nil {
        fmt.Printf("  Warning: inject failed (%v). You can run 'sap-devs inject' manually.\n", err)
    } else {
        fmt.Println("  SAP context injected into your AI tools.")
    }
}

// Step 4: Shell profile hook
fmt.Println("\nStep 4/4: Add SAP tip to your terminal startup?")
```

Also update the final message:
```go
fmt.Println("\nSetup complete! Run 'sap-devs --help' to explore all commands.")
fmt.Println("Run 'sap-devs inject' to re-inject after syncing new content.")
```

Add `runInjectGlobal` helper at the bottom of init.go (before `func init()`):

```go
func runInjectGlobal() error {
    injectProject = false
    injectGlobal = true
    injectDryRun = false
    injectTool = ""
    return injectCmd.RunE(injectCmd, nil)
}
```

- [ ] **Step 2: Verify it builds**

Run: `go build ./...`
Expected: clean build

- [ ] **Step 3: Commit**

```bash
git add cmd/init.go
git commit -m "feat: wire inject step into init wizard"
```

---

## Task 10: Integration Smoke Test

Verify the full inject flow works end-to-end using a temp directory fixture.

**Files:**
- Create: `cmd/inject_test.go`

- [ ] **Step 1: Write the integration test**

```go
// cmd/inject_test.go
package cmd_test

import (
    "os"
    "path/filepath"
    "strings"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.tools.sap/developer-relations/sap-devs-cli/internal/adapter"
    "github.tools.sap/developer-relations/sap-devs-cli/internal/content"
)

// TestInjectEndToEnd tests ReplaceSection → file content round-trip
// without invoking the full cobra command (avoids XDG path dependencies in CI).
func TestInjectEndToEnd(t *testing.T) {
    dir := t.TempDir()
    claudeMD := filepath.Join(dir, "CLAUDE.md")

    // Simulate existing CLAUDE.md
    require.NoError(t, os.WriteFile(claudeMD, []byte("# My Project\n\nMy notes.\n"), 0644))

    // Build packs and render context
    packs := []*content.Pack{
        {ID: "cap", Name: "CAP", ContextMD: "## SAP CAP\n\nUse @sap/cds for data models."},
    }
    rendered := content.RenderContext(packs, nil)

    // Run engine with a file-inject adapter targeting our temp file
    adapters := []adapter.Adapter{
        {
            ID:   "claude-code",
            Type: "file-inject",
            Targets: []adapter.Target{
                {Scope: "global", Path: claudeMD, Mode: "replace-section", Section: "SAP Developer Context"},
            },
        },
    }
    engine := adapter.NewEngine(adapters, rendered, adapter.Options{Scope: "global"})
    require.NoError(t, engine.Run())

    // Verify output
    data, err := os.ReadFile(claudeMD)
    require.NoError(t, err)
    result := string(data)

    assert.Contains(t, result, "# My Project")
    assert.Contains(t, result, "My notes.")
    assert.Contains(t, result, "<!-- sap-devs:start:SAP Developer Context -->")
    assert.Contains(t, result, "Use @sap/cds for data models.")
    assert.Contains(t, result, "<!-- sap-devs:end:SAP Developer Context -->")

    // Second run — idempotent
    require.NoError(t, engine.Run())
    data2, _ := os.ReadFile(claudeMD)
    assert.Equal(t, 1, strings.Count(string(data2), "<!-- sap-devs:start:SAP Developer Context -->"))
}
```

- [ ] **Step 2: Run tests**

Run: `go test ./cmd/... -v -run TestInjectEndToEnd`
Expected: PASS

- [ ] **Step 3: Run all tests**

Run: `go test ./...`
Expected: all pass

- [ ] **Step 4: Commit**

```bash
git add cmd/inject_test.go
git commit -m "test: add end-to-end inject smoke test"
```

---

## Task 11: Final Build Verification

- [ ] **Step 1: Run the full test suite**

Run: `go test ./...`
Expected: all tests pass, zero failures

- [ ] **Step 2: Run vet**

Run: `go vet ./...`
Expected: no issues

- [ ] **Step 3: Build the binary**

Run: `go build -o sap-devs.exe .`
Expected: binary produced without errors

- [ ] **Step 4: Test the binary manually**

From your own terminal (not Claude Code's bash — Windows Defender blocks it there):
```
sap-devs.exe inject --dry-run
sap-devs.exe inject --tool claude-code --dry-run
sap-devs.exe inject --project --dry-run
```
Expected: dry-run output printed, no files written

- [ ] **Step 5: Final commit**

```bash
git add -A
git commit -m "chore: plan2 ai-injection complete — all tests passing"
```
