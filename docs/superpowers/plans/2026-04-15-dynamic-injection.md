# Dynamic Injection Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prepend a runtime-generated `## sap-devs Runtime Context` section to every inject output, surfacing CLI version, active profile, loaded packs, last sync time, detected project type, wired SAP MCP servers, and available commands.

**Architecture:** Add a `DynamicContext` struct to `internal/content`, a new `internal/dynamic` package that gathers it at inject time, and thread it through `adapter.Options` → `Engine.Run` → `RenderContext`. Import direction: `internal/dynamic` → `internal/adapter` → `internal/content`; no cycles. The `cmd` package populates cobra command metadata and passes it in via `GatherOpts` to avoid a `cmd` ↔ `internal` cycle.

**Tech Stack:** Go 1.21+, cobra (command introspection), standard library (`encoding/json`, `os`, `strings`, `time`)

---

## File Map

| Action | Path | Responsibility |
|---|---|---|
| Modify | `internal/sync/engine.go` | Add `MostRecentSync(stateDir string) *time.Time` exported function |
| Create | `internal/content/dynamic.go` | `DynamicContext`, `WiredMCPEntry`, `CommandInfo` types |
| Modify | `internal/content/render.go` | `RenderContext` gains `*DynamicContext` third param; renders runtime section |
| Modify | `internal/content/render_test.go` | Add dynamic section tests; update existing callers to pass `nil` |
| Create | `internal/dynamic/gather.go` | `GatherOpts`, `GatherDynamic` — collects all four dynamic items |
| Create | `internal/dynamic/gather_test.go` | Unit tests for each gather step |
| Modify | `internal/adapter/engine.go` | `Options` gets `Dynamic *content.DynamicContext`; thread into `RenderContext` call |
| Modify | `cmd/inject.go` | Build cobra command list; call `GatherDynamic`; wire into `Options` |

---

## Task 1: Export `MostRecentSync` from `internal/sync`

`GatherDynamic` needs to read the last sync time without directly calling the unexported `loadSyncState`. Add a thin exported function to the sync package.

**Files:**
- Modify: `internal/sync/engine.go`
- Modify: `internal/sync/engine_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/sync/engine_test.go` (same file as existing tests — the `writeCategoryTimestamps` helper defined there is needed by the second test case):

```go
func TestMostRecentSync_ReturnsNilWhenNoState(t *testing.T) {
    dir := t.TempDir()
    result := MostRecentSync(dir)
    assert.Nil(t, result)
}

func TestMostRecentSync_ReturnsMostRecentCategory(t *testing.T) {
    dir := t.TempDir()
    older := time.Now().Add(-2 * time.Hour).Truncate(time.Second)
    newer := time.Now().Add(-1 * time.Hour).Truncate(time.Second)
    writeCategoryTimestamps(t, dir, map[string]time.Time{
        "tips":    older,
        "context": newer,
    })
    result := MostRecentSync(dir)
    require.NotNil(t, result)
    assert.Equal(t, newer, result.Truncate(time.Second))
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go build ./internal/sync/...`
Expected: compile error — `MostRecentSync` undefined

- [ ] **Step 3: Implement `MostRecentSync` in `internal/sync/engine.go`**

Add after the `PacksBlock` function:

```go
// MostRecentSync returns a pointer to the most recent non-zero category sync time
// recorded in stateDir, or nil if no syncs have been recorded.
func MostRecentSync(stateDir string) *time.Time {
    state := loadSyncState(stateDir)
    var most time.Time
    for _, ts := range state.Categories {
        if ts.After(most) {
            most = ts
        }
    }
    if most.IsZero() {
        return nil
    }
    return &most
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go build ./internal/sync/... && go vet ./internal/sync/...`

- [ ] **Step 5: Commit**

```bash
git add internal/sync/engine.go internal/sync/engine_test.go
git commit -m "feat(sync): export MostRecentSync for dynamic injection"
```

---

## Task 2: Create `DynamicContext` types in `internal/content`

Pure type declarations — no logic. Establishes the data contract that both `internal/dynamic` (writer) and `internal/content/render.go` (reader) depend on.

**Files:**
- Create: `internal/content/dynamic.go`

- [ ] **Step 1: Create the file**

```go
// internal/content/dynamic.go
package content

import "time"

// DynamicContext holds runtime-gathered information injected before pack content.
// All fields are optional; zero values mean "not available".
type DynamicContext struct {
    CLIVersion      string
    ActiveProfile   string // profile.Name, or profile.ID, or ""
    LoadedPackIDs   []string
    LastSynced      *time.Time
    ProjectType     string
    WiredMCPServers []WiredMCPEntry
    Commands        []CommandInfo
}

// WiredMCPEntry records SAP MCP servers registered in a specific AI tool's config.
type WiredMCPEntry struct {
    AdapterName string
    ServerIDs   []string
}

// CommandInfo describes a single CLI command for injection into AI context.
type CommandInfo struct {
    Name  string
    Short string
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/content/...`

- [ ] **Step 3: Commit**

```bash
git add internal/content/dynamic.go
git commit -m "feat(content): add DynamicContext types"
```

---

## Task 3: Update `RenderContext` to render the dynamic section

`RenderContext` gains a `*DynamicContext` third parameter. When nil, output is identical to today (all existing tests pass unchanged). When non-nil, a `## sap-devs Runtime Context` section is prepended before pack content.

**Files:**
- Modify: `internal/content/render.go`
- Modify: `internal/content/render_test.go`

- [ ] **Step 1: Write failing tests for the dynamic section**

Add to `internal/content/render_test.go`:

```go
func TestRenderContext_DynamicSection_NilIsBackwardCompatible(t *testing.T) {
    packs := []*content.Pack{{ID: "cap", ContextMD: "CAP content."}}
    out := content.RenderContext(packs, nil, nil)
    assert.Contains(t, out, "CAP content.")
    assert.NotContains(t, out, "sap-devs Runtime Context")
}

func TestRenderContext_DynamicSection_AppearsBeforePackContent(t *testing.T) {
    packs := []*content.Pack{{ID: "cap", ContextMD: "CAP content."}}
    dyn := &content.DynamicContext{
        CLIVersion:    "1.2.3",
        ActiveProfile: "CAP Developer",
        LoadedPackIDs: []string{"cap"},
    }
    out := content.RenderContext(packs, nil, dyn)
    dynIdx := strings.Index(out, "sap-devs Runtime Context")
    packIdx := strings.Index(out, "CAP content.")
    assert.Greater(t, packIdx, dynIdx, "runtime section must appear before pack content")
}

func TestRenderContext_DynamicSection_VersionAndProfile(t *testing.T) {
    dyn := &content.DynamicContext{
        CLIVersion:    "1.2.3",
        ActiveProfile: "CAP Developer",
        LoadedPackIDs: []string{"cap", "btp"},
    }
    out := content.RenderContext(nil, nil, dyn)
    assert.Contains(t, out, "sap-devs v1.2.3")
    assert.Contains(t, out, "CAP Developer")
    assert.Contains(t, out, "cap, btp")
}

func TestRenderContext_DynamicSection_LastSyncedShown(t *testing.T) {
    synced := time.Now().Add(-2 * time.Hour)
    dyn := &content.DynamicContext{LastSynced: &synced}
    out := content.RenderContext(nil, nil, dyn)
    assert.Contains(t, out, "Last synced:")
    assert.NotContains(t, out, "never")
}

func TestRenderContext_DynamicSection_NeverSyncedWhenNil(t *testing.T) {
    dyn := &content.DynamicContext{}
    out := content.RenderContext(nil, nil, dyn)
    assert.Contains(t, out, "never")
}

func TestRenderContext_DynamicSection_ProjectTypeShownWhenSet(t *testing.T) {
    dyn := &content.DynamicContext{ProjectType: "CAP (Node.js)"}
    out := content.RenderContext(nil, nil, dyn)
    assert.Contains(t, out, "**Project type:** CAP (Node.js)")
}

func TestRenderContext_DynamicSection_ProjectTypeOmittedWhenEmpty(t *testing.T) {
    dyn := &content.DynamicContext{ProjectType: ""}
    out := content.RenderContext(nil, nil, dyn)
    assert.NotContains(t, out, "Project type")
}

func TestRenderContext_DynamicSection_MCPServersShown(t *testing.T) {
    dyn := &content.DynamicContext{
        WiredMCPServers: []content.WiredMCPEntry{
            {AdapterName: "Claude Code", ServerIDs: []string{"sap-cap-mcp"}},
        },
    }
    out := content.RenderContext(nil, nil, dyn)
    assert.Contains(t, out, "Claude Code")
    assert.Contains(t, out, "sap-cap-mcp")
}

func TestRenderContext_DynamicSection_MCPServersOmittedWhenNone(t *testing.T) {
    dyn := &content.DynamicContext{}
    out := content.RenderContext(nil, nil, dyn)
    assert.NotContains(t, out, "Wired SAP MCP servers")
}

func TestRenderContext_DynamicSection_CommandsListed(t *testing.T) {
    dyn := &content.DynamicContext{
        Commands: []content.CommandInfo{
            {Name: "inject", Short: "Push SAP context to your AI tools"},
            {Name: "sync", Short: "Pull latest SAP developer content"},
        },
    }
    out := content.RenderContext(nil, nil, dyn)
    assert.Contains(t, out, "`inject`")
    assert.Contains(t, out, "Push SAP context to your AI tools")
    assert.Contains(t, out, "`sync`")
}
```

Note: these tests call `content.RenderContext(packs, profile, dyn)` — the new 3-argument signature. The existing tests in the file call the old 2-argument form and must be updated in the next step. Also add `"strings"` and `"time"` to the import block of `render_test.go` if not already present — the new tests use `strings.Index` and `time.Now()`.

- [ ] **Step 2: Update existing `RenderContext` callers in `render_test.go` to pass `nil` as third arg**

Find every call `content.RenderContext(` in `internal/content/render_test.go` and add `, nil` before the closing `)`.

There are 9 existing calls (lines 18, 37, 49, 57, 62, 69, 80, 87, 92 etc). Update each one, for example:

```go
// Before
out := content.RenderContext(packs, nil)
// After
out := content.RenderContext(packs, nil, nil)
```

- [ ] **Step 3: Run tests to verify they fail (compile error on old signature)**

Run: `go build ./internal/content/...`
Expected: compile errors — wrong number of arguments

- [ ] **Step 4: Update `RenderContext` signature and implementation in `internal/content/render.go`**

Replace the entire function with:

```go
// RenderContext builds the Markdown string injected into AI tool configuration.
// Packs are rendered in the order provided (caller applies profile weights first).
// dynamic may be nil; when non-nil a runtime context section is prepended.
func RenderContext(packs []*Pack, profile *Profile, dynamic *DynamicContext) string {
    var b strings.Builder

    b.WriteString("# SAP Developer Context\n\n")
    b.WriteString("This context is maintained by sap-devs and provides up-to-date SAP developer knowledge.\n\n")

    if profile != nil {
        b.WriteString(fmt.Sprintf("**Developer Profile:** %s — %s\n\n", profile.Name, profile.Description))
    }

    if dynamic != nil {
        b.WriteString(renderDynamic(dynamic))
        // renderDynamic ends with \n; add one more for blank line before pack content
        b.WriteString("\n")
    }

    for _, p := range packs {
        if strings.TrimSpace(p.ContextMD) == "" {
            continue
        }
        b.WriteString(strings.TrimSpace(p.ContextMD))
        b.WriteString("\n\n")
    }

    return strings.TrimRight(b.String(), "\n") + "\n"
}
```

Then add `renderDynamic` below it in the same file:

```go
// renderDynamic produces the ## sap-devs Runtime Context markdown section.
func renderDynamic(d *DynamicContext) string {
    var b strings.Builder
    b.WriteString("## sap-devs Runtime Context\n\n")

    // Status line: CLI version, profile, packs
    var statusParts []string
    if d.CLIVersion != "" {
        statusParts = append(statusParts, fmt.Sprintf("**CLI:** sap-devs v%s", d.CLIVersion))
    }
    if d.ActiveProfile != "" {
        statusParts = append(statusParts, fmt.Sprintf("**Profile:** %s", d.ActiveProfile))
    }
    if len(d.LoadedPackIDs) > 0 {
        statusParts = append(statusParts, fmt.Sprintf("**Packs:** %s", strings.Join(d.LoadedPackIDs, ", ")))
    }
    if len(statusParts) > 0 {
        b.WriteString(strings.Join(statusParts, " | "))
        b.WriteString("\n")
    }

    // Last synced
    if d.LastSynced != nil {
        ago := time.Since(*d.LastSynced).Truncate(time.Minute)
        b.WriteString(fmt.Sprintf("**Last synced:** %s (%s ago)\n",
            d.LastSynced.Format("2006-01-02 15:04"), ago))
    } else {
        b.WriteString("**Last synced:** never — run `sap-devs sync`\n")
    }

    // Project type (omit if empty)
    if d.ProjectType != "" {
        b.WriteString(fmt.Sprintf("**Project type:** %s\n", d.ProjectType))
    }

    // Wired MCP servers (omit if none)
    for _, entry := range d.WiredMCPServers {
        if len(entry.ServerIDs) > 0 {
            b.WriteString(fmt.Sprintf("**Wired SAP MCP servers (%s):** %s\n",
                entry.AdapterName, strings.Join(entry.ServerIDs, ", ")))
        }
    }

    // Commands
    if len(d.Commands) > 0 {
        b.WriteString("\n**Available commands:**\n")
        for _, c := range d.Commands {
            b.WriteString(fmt.Sprintf("- `%s` — %s\n", c.Name, c.Short))
        }
    }

    b.WriteString("\nRun `sap-devs inject` to refresh this context · `sap-devs sync --force` to update content\n")
    return b.String()
}
```

Make sure `"time"` is imported in `render.go`.

- [ ] **Step 5: Update the one other caller of `RenderContext` in `internal/adapter/engine.go`**

Find the line `ctx := content.RenderContext(trimmed, e.profile)` and change to:

```go
ctx := content.RenderContext(trimmed, e.profile, nil)
```

(Task 5 will change this to `e.opts.Dynamic`, but for now `nil` keeps it compiling.)

- [ ] **Step 6: Run tests to verify they pass**

Run: `go build ./... && go vet ./...`

- [ ] **Step 7: Commit**

```bash
git add internal/content/dynamic.go internal/content/render.go internal/content/render_test.go internal/adapter/engine.go
git commit -m "feat(content): render dynamic context section in RenderContext"
```

---

## Task 4: Implement `GatherDynamic` in `internal/dynamic`

The core gather logic. Each sub-step (project type, sync state, MCP, CLI self-awareness) is independent and silently skips on error.

**Files:**
- Create: `internal/dynamic/gather.go`
- Create: `internal/dynamic/gather_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/dynamic/gather_test.go`:

```go
package dynamic_test

import (
    "encoding/json"
    "os"
    "path/filepath"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.tools.sap/developer-relations/sap-devs-cli/internal/adapter"
    "github.tools.sap/developer-relations/sap-devs-cli/internal/content"
    "github.tools.sap/developer-relations/sap-devs-cli/internal/dynamic"
)

// --- Project type detection ---

func TestGatherDynamic_ProjectType_CdsrcJson(t *testing.T) {
    dir := t.TempDir()
    require.NoError(t, os.WriteFile(filepath.Join(dir, ".cdsrc.json"), []byte(`{}`), 0600))
    ctx := dynamic.GatherDynamic(dynamic.GatherOpts{CWD: dir})
    assert.Equal(t, "CAP (Node.js)", ctx.ProjectType)
}

func TestGatherDynamic_ProjectType_PackageJsonWithCDS(t *testing.T) {
    dir := t.TempDir()
    pkg := `{"dependencies":{"@sap/cds":"^7.0.0"}}`
    require.NoError(t, os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0600))
    ctx := dynamic.GatherDynamic(dynamic.GatherOpts{CWD: dir})
    assert.Equal(t, "CAP (Node.js)", ctx.ProjectType)
}

func TestGatherDynamic_ProjectType_PackageJsonWithCDSInDevDeps(t *testing.T) {
    dir := t.TempDir()
    pkg := `{"devDependencies":{"@sap/cds":"^7.0.0"}}`
    require.NoError(t, os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0600))
    ctx := dynamic.GatherDynamic(dynamic.GatherOpts{CWD: dir})
    assert.Equal(t, "CAP (Node.js)", ctx.ProjectType)
}

func TestGatherDynamic_ProjectType_MtaYaml(t *testing.T) {
    dir := t.TempDir()
    require.NoError(t, os.WriteFile(filepath.Join(dir, "mta.yaml"), []byte(`ID: myapp`), 0600))
    ctx := dynamic.GatherDynamic(dynamic.GatherOpts{CWD: dir})
    assert.Equal(t, "Multi-target Application (MTA)", ctx.ProjectType)
}

func TestGatherDynamic_ProjectType_XsAppJson(t *testing.T) {
    dir := t.TempDir()
    require.NoError(t, os.WriteFile(filepath.Join(dir, "xs-app.json"), []byte(`{}`), 0600))
    ctx := dynamic.GatherDynamic(dynamic.GatherOpts{CWD: dir})
    assert.Equal(t, "Fiori / BAS app", ctx.ProjectType)
}

func TestGatherDynamic_ProjectType_PlainPackageJson(t *testing.T) {
    dir := t.TempDir()
    require.NoError(t, os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"myapp"}`), 0600))
    ctx := dynamic.GatherDynamic(dynamic.GatherOpts{CWD: dir})
    assert.Equal(t, "Node.js", ctx.ProjectType)
}

func TestGatherDynamic_ProjectType_EmptyWhenNoFiles(t *testing.T) {
    ctx := dynamic.GatherDynamic(dynamic.GatherOpts{CWD: t.TempDir()})
    assert.Empty(t, ctx.ProjectType)
}

func TestGatherDynamic_ProjectType_CdsrcTakesPriorityOverPackageJson(t *testing.T) {
    dir := t.TempDir()
    require.NoError(t, os.WriteFile(filepath.Join(dir, ".cdsrc.json"), []byte(`{}`), 0600))
    require.NoError(t, os.WriteFile(filepath.Join(dir, "mta.yaml"), []byte(`ID: myapp`), 0600))
    ctx := dynamic.GatherDynamic(dynamic.GatherOpts{CWD: dir})
    assert.Equal(t, "CAP (Node.js)", ctx.ProjectType)
}

// --- Pack freshness ---

func TestGatherDynamic_LastSynced_NilWhenNoStateFile(t *testing.T) {
    ctx := dynamic.GatherDynamic(dynamic.GatherOpts{SyncStateDir: t.TempDir()})
    assert.Nil(t, ctx.LastSynced)
}

func TestGatherDynamic_LastSynced_ReturnsMostRecent(t *testing.T) {
    dir := t.TempDir()
    older := time.Now().Add(-2 * time.Hour).UTC().Truncate(time.Second)
    newer := time.Now().Add(-30 * time.Minute).UTC().Truncate(time.Second)
    state := map[string]interface{}{
        "version": 1,
        "categories": map[string]string{
            "tips":    older.Format(time.RFC3339),
            "context": newer.Format(time.RFC3339),
        },
        "packs":   map[string]interface{}{},
        "markers": map[string]interface{}{},
    }
    data, _ := json.Marshal(state)
    require.NoError(t, os.WriteFile(filepath.Join(dir, "sync-state.json"), data, 0600))
    ctx := dynamic.GatherDynamic(dynamic.GatherOpts{SyncStateDir: dir})
    require.NotNil(t, ctx.LastSynced)
    assert.Equal(t, newer, ctx.LastSynced.UTC().Truncate(time.Second))
}

// --- CLI self-awareness ---

func TestGatherDynamic_CLIVersion_PassedThrough(t *testing.T) {
    ctx := dynamic.GatherDynamic(dynamic.GatherOpts{CLIVersion: "2.0.0"})
    assert.Equal(t, "2.0.0", ctx.CLIVersion)
}

func TestGatherDynamic_ActiveProfile_UsesName(t *testing.T) {
    p := &content.Profile{ID: "cap-developer", Name: "CAP Developer"}
    ctx := dynamic.GatherDynamic(dynamic.GatherOpts{Profile: p})
    assert.Equal(t, "CAP Developer", ctx.ActiveProfile)
}

func TestGatherDynamic_ActiveProfile_FallsBackToID(t *testing.T) {
    p := &content.Profile{ID: "cap-developer", Name: ""}
    ctx := dynamic.GatherDynamic(dynamic.GatherOpts{Profile: p})
    assert.Equal(t, "cap-developer", ctx.ActiveProfile)
}

func TestGatherDynamic_LoadedPackIDs_FromPacks(t *testing.T) {
    packs := []*content.Pack{{ID: "cap"}, {ID: "btp"}}
    ctx := dynamic.GatherDynamic(dynamic.GatherOpts{Packs: packs})
    assert.Equal(t, []string{"cap", "btp"}, ctx.LoadedPackIDs)
}

func TestGatherDynamic_Commands_PassedThrough(t *testing.T) {
    cmds := []content.CommandInfo{{Name: "inject", Short: "Push SAP context"}}
    ctx := dynamic.GatherDynamic(dynamic.GatherOpts{Commands: cmds})
    require.Len(t, ctx.Commands, 1)
    assert.Equal(t, "inject", ctx.Commands[0].Name)
}

// --- Wired MCP servers ---

func TestGatherDynamic_WiredMCP_OnlySAPServersReturned(t *testing.T) {
    dir := t.TempDir()
    mcpConfig := map[string]interface{}{
        "mcpServers": map[string]interface{}{
            "sap-cap-mcp":    map[string]interface{}{},
            "some-other-mcp": map[string]interface{}{},
        },
    }
    data, _ := json.Marshal(mcpConfig)
    cfgPath := filepath.Join(dir, "settings.json")
    require.NoError(t, os.WriteFile(cfgPath, data, 0600))

    adapters := []adapter.Adapter{{
        ID:   "claude-code",
        Name: "Claude Code",
        MCPConfig: &adapter.MCPConfig{
            Path:   cfgPath,
            Format: "json",
            Key:    "mcpServers",
        },
    }}
    packs := []*content.Pack{{
        ID: "cap",
        MCPServers: []content.MCPServer{{ID: "sap-cap-mcp"}},
    }}

    ctx := dynamic.GatherDynamic(dynamic.GatherOpts{Adapters: adapters, Packs: packs})
    require.Len(t, ctx.WiredMCPServers, 1)
    assert.Equal(t, "Claude Code", ctx.WiredMCPServers[0].AdapterName)
    assert.Equal(t, []string{"sap-cap-mcp"}, ctx.WiredMCPServers[0].ServerIDs)
}

func TestGatherDynamic_WiredMCP_EmptyWhenConfigFileMissing(t *testing.T) {
    adapters := []adapter.Adapter{{
        ID:   "claude-code",
        Name: "Claude Code",
        MCPConfig: &adapter.MCPConfig{
            Path: "/nonexistent/settings.json",
            Key:  "mcpServers",
        },
    }}
    ctx := dynamic.GatherDynamic(dynamic.GatherOpts{Adapters: adapters})
    assert.Empty(t, ctx.WiredMCPServers)
}

// --- Error resilience ---

func TestGatherDynamic_NeverPanics_AllZeroOpts(t *testing.T) {
    // Must not panic or return nil
    ctx := dynamic.GatherDynamic(dynamic.GatherOpts{})
    require.NotNil(t, ctx)
}

func TestGatherDynamic_NeverPanics_MissingCWD(t *testing.T) {
    ctx := dynamic.GatherDynamic(dynamic.GatherOpts{CWD: "/nonexistent/dir/xyz"})
    require.NotNil(t, ctx)
    assert.Empty(t, ctx.ProjectType)
}
```

- [ ] **Step 2: Run tests to verify they fail (package doesn't exist)**

Run: `go build ./internal/dynamic/...`
Expected: compile error — package not found

- [ ] **Step 3: Create `internal/dynamic/gather.go`**

```go
// internal/dynamic/gather.go
package dynamic

import (
    "encoding/json"
    "os"
    "path/filepath"
    "strings"

    "github.tools.sap/developer-relations/sap-devs-cli/internal/adapter"
    "github.tools.sap/developer-relations/sap-devs-cli/internal/content"
    "github.tools.sap/developer-relations/sap-devs-cli/internal/sync"
)

// GatherOpts holds all inputs needed to collect dynamic context at inject time.
type GatherOpts struct {
    CWD          string
    CLIVersion   string
    Profile      *content.Profile
    Packs        []*content.Pack
    SyncStateDir string
    Adapters     []adapter.Adapter
    Commands     []content.CommandInfo
}

// GatherDynamic collects runtime context from the local environment.
// All sub-steps silently skip on error; the returned pointer is never nil.
func GatherDynamic(opts GatherOpts) *content.DynamicContext {
    d := &content.DynamicContext{}

    // CLI self-awareness
    d.CLIVersion = opts.CLIVersion
    if opts.Profile != nil {
        if opts.Profile.Name != "" {
            d.ActiveProfile = opts.Profile.Name
        } else {
            d.ActiveProfile = opts.Profile.ID
        }
    }
    for _, p := range opts.Packs {
        d.LoadedPackIDs = append(d.LoadedPackIDs, p.ID)
    }
    d.Commands = opts.Commands

    // Pack freshness
    if opts.SyncStateDir != "" {
        d.LastSynced = sync.MostRecentSync(opts.SyncStateDir)
    }

    // Project type
    d.ProjectType = detectProjectType(opts.CWD)

    // Wired SAP MCP servers
    d.WiredMCPServers = detectWiredMCP(opts.Adapters, opts.Packs)

    return d
}

// detectProjectType checks CWD for well-known SAP project indicators.
// Returns the first match; returns "" if nothing is detected.
func detectProjectType(cwd string) string {
    if cwd == "" {
        return ""
    }

    // .cdsrc.json — definitive CAP Node.js marker
    if fileExists(filepath.Join(cwd, ".cdsrc.json")) {
        return "CAP (Node.js)"
    }

    // package.json — check for @sap/cds before falling through to plain Node.js
    pkgPath := filepath.Join(cwd, "package.json")
    if data, err := os.ReadFile(pkgPath); err == nil {
        if hasSAPCDS(data) {
            return "CAP (Node.js)"
        }
    }

    // pom.xml — CAP Java
    if data, err := os.ReadFile(filepath.Join(cwd, "pom.xml")); err == nil {
        if strings.Contains(string(data), "com.sap.cds") {
            return "CAP (Java)"
        }
    }

    // mta.yaml — Multi-target Application
    if fileExists(filepath.Join(cwd, "mta.yaml")) {
        return "Multi-target Application (MTA)"
    }

    // xs-app.json — Fiori / BAS
    if fileExists(filepath.Join(cwd, "xs-app.json")) {
        return "Fiori / BAS app"
    }

    // Plain package.json — generic Node.js
    if fileExists(pkgPath) {
        return "Node.js"
    }

    return ""
}

// hasSAPCDS reports whether the package.json data contains @sap/cds in any dependency map.
func hasSAPCDS(data []byte) bool {
    var pkg struct {
        Dependencies    map[string]string `json:"dependencies"`
        DevDependencies map[string]string `json:"devDependencies"`
    }
    if err := json.Unmarshal(data, &pkg); err != nil {
        return false
    }
    if _, ok := pkg.Dependencies["@sap/cds"]; ok {
        return true
    }
    if _, ok := pkg.DevDependencies["@sap/cds"]; ok {
        return true
    }
    return false
}

// detectWiredMCP reads each adapter's MCP config file and cross-references
// installed server IDs against known SAP MCP server IDs from loaded packs.
func detectWiredMCP(adapters []adapter.Adapter, packs []*content.Pack) []content.WiredMCPEntry {
    // Build set of known SAP MCP server IDs from packs.
    sapIDs := make(map[string]bool)
    for _, p := range packs {
        for _, srv := range p.MCPServers {
            sapIDs[srv.ID] = true
        }
    }
    if len(sapIDs) == 0 {
        return nil
    }

    var entries []content.WiredMCPEntry
    for _, a := range adapters {
        if a.MCPConfig == nil || a.MCPConfig.Path == "" {
            continue
        }
        path, err := adapter.ExpandHome(a.MCPConfig.Path)
        if err != nil {
            continue
        }
        installed := readMCPServerIDs(path, a.MCPConfig.Key)
        var matched []string
        for _, id := range installed {
            if sapIDs[id] {
                matched = append(matched, id)
            }
        }
        if len(matched) > 0 {
            entries = append(entries, content.WiredMCPEntry{
                AdapterName: a.Name,
                ServerIDs:   matched,
            })
        }
    }
    return entries
}

// readMCPServerIDs reads the top-level keys of the object at root[key] from a JSON file.
// Returns nil on any error.
func readMCPServerIDs(path, key string) []string {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil
    }
    var root map[string]json.RawMessage
    if err := json.Unmarshal(data, &root); err != nil {
        return nil
    }
    raw, ok := root[key]
    if !ok {
        return nil
    }
    var servers map[string]json.RawMessage
    if err := json.Unmarshal(raw, &servers); err != nil {
        return nil
    }
    ids := make([]string, 0, len(servers))
    for id := range servers {
        ids = append(ids, id)
    }
    return ids
}

func fileExists(path string) bool {
    _, err := os.Stat(path)
    return err == nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go build ./... && go vet ./...`

- [ ] **Step 5: Commit**

```bash
git add internal/dynamic/gather.go internal/dynamic/gather_test.go
git commit -m "feat(dynamic): implement GatherDynamic for runtime context collection"
```

---

## Task 5: Thread `DynamicContext` through `adapter.Options` and `Engine`

Small wiring change. Replaces the temporary `nil` placeholder added in Task 3.

**Files:**
- Modify: `internal/adapter/engine.go`

- [ ] **Step 1: Add `Dynamic` field to `Options` and update `RenderContext` call**

In `internal/adapter/engine.go`, add the field to `Options`:

```go
// Options controls inject scope, filtering, dry-run, and stats behaviour.
type Options struct {
    Scope      string
    ToolFilter string
    DryRun     bool
    Stats      bool
    Out        io.Writer
    Dynamic    *content.DynamicContext // nil = no dynamic section
}
```

Then in `Engine.Run()`, change the existing `RenderContext` call from:

```go
ctx := content.RenderContext(trimmed, e.profile, nil)
```

to:

```go
ctx := content.RenderContext(trimmed, e.profile, e.opts.Dynamic)
```

- [ ] **Step 2: Verify it compiles and existing tests pass**

Run: `go build ./... && go vet ./...`

- [ ] **Step 3: Commit**

```bash
git add internal/adapter/engine.go
git commit -m "feat(adapter): thread DynamicContext through Options into RenderContext"
```

---

## Task 6: Wire `GatherDynamic` into `cmd/inject.go`

The final wiring. Gathers the cobra command list and dynamic context, then passes them into `adapter.Options`.

**Files:**
- Modify: `cmd/inject.go`

- [ ] **Step 1: Add the `dynamic` import and gather call**

In `cmd/inject.go`, add `"github.tools.sap/developer-relations/sap-devs-cli/internal/dynamic"` to the import block.

After the `packs, err = loader.LoadPacks(...)` reload block and before `opts := adapter.Options{...}`, insert:

```go
// Gather current working directory for project type detection.
cwd, _ := os.Getwd() // silently ignore error; GatherDynamic handles empty CWD

// Build command list from cobra for CLI self-awareness.
var cmdInfos []content.CommandInfo
for _, c := range rootCmd.Commands() {
    if !c.Hidden {
        cmdInfos = append(cmdInfos, content.CommandInfo{
            Name:  strings.SplitN(c.Use, " ", 2)[0],
            Short: c.Short,
        })
    }
}

adapters, err := loadAdapters()
if err != nil {
    return err
}

dynCtx := dynamic.GatherDynamic(dynamic.GatherOpts{
    CWD:          cwd,
    CLIVersion:   Version,
    Profile:      activeProfile,
    Packs:        packs,
    SyncStateDir: paths.CacheDir,
    Adapters:     adapters,
    Commands:     cmdInfos,
})
```

Then update the `opts` construction to include `Dynamic`:

```go
opts := adapter.Options{
    Scope:      scope,
    ToolFilter: injectTool,
    DryRun:     injectDryRun,
    Stats:      injectStats,
    Out:        cmd.OutOrStdout(),
    Dynamic:    dynCtx,
}
```

And update `newAdapterEngine` call — remove the `adapters` load that was inside it (if it was loading adapters internally, or check how `newAdapterEngine` works). Look at the current signature:

```go
eng, err := newAdapterEngine(packs, activeProfile, opts)
```

If `newAdapterEngine` loads adapters internally, the `adapters` variable above is only used for `GatherDynamic`. This is fine — `newAdapterEngine` handles its own adapter loading for the injection itself. The `adapters` gathered above is solely for MCP detection in `GatherDynamic`.

- [ ] **Step 2: Note on `loadAdapters` and double-loading**

`newAdapterEngine` in `cmd/root.go` calls `loadAdapters()` internally to run the injection. The `adapters` variable declared above is a separate local used only for `GatherDynamic` (MCP detection). They do not conflict — `loadAdapters()` is called twice: once for MCP detection, once inside `newAdapterEngine`. This is acceptable; adapter loading is cheap (it just reads YAML files from disk).

- [ ] **Step 3: Verify it compiles**

Run: `go build ./... && go vet ./...`

- [ ] **Step 4: Smoke test with dry-run**

```bash
SAP_DEVS_DEV=1 go run . inject --dry-run
```

Expected: `[dry-run] would write section "SAP Developer Context" to ~/.claude/CLAUDE.md`

For a real content preview:

```bash
SAP_DEVS_DEV=1 go run . inject --dry-run --stats
```

Then manually verify the `## sap-devs Runtime Context` section is present by checking the actual CLAUDE.md file after a non-dry-run:

```bash
SAP_DEVS_DEV=1 go run . inject
```

Open `~/.claude/CLAUDE.md` and confirm the runtime context section appears with the correct content.

- [ ] **Step 5: Commit**

```bash
git add cmd/inject.go
git commit -m "feat(cmd): wire GatherDynamic into inject for dynamic runtime context"
```

---

## Verification

After all tasks complete:

- [ ] `go build ./...` — clean build
- [ ] `go vet ./...` — no issues
- [ ] Run `SAP_DEVS_DEV=1 go run . inject` and inspect `~/.claude/CLAUDE.md` — confirm:
  - `## sap-devs Runtime Context` section present before pack content
  - CLI version shown
  - Loaded pack IDs shown
  - Last synced date shown (or "never" message)
  - Project type shown if in a CAP/MTA project directory, absent if not
  - Commands list present with correct `Short` descriptions
