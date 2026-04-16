# Dynamic Injection Design

**Date:** 2026-04-15  
**Status:** Approved

## Overview

Extend the `inject` pipeline to prepend a runtime-generated context section before pack content. This section surfaces information the AI tool cannot derive from static packs: the CLI version, active profile, which packs are loaded, when content was last synced, the detected project type, wired SAP MCP servers, and the full list of available `sap-devs` commands. New commands automatically appear as they are added to the cobra command tree — no manual maintenance required.

## Scope

Four categories of dynamic data are in scope:

1. **CLI self-awareness** — version, active profile name, all non-hidden cobra commands with their `Short` descriptions
2. **Pack freshness** — last sync timestamp, loaded pack IDs
3. **Project type** — file-based detection in CWD
4. **Wired SAP MCP servers** — cross-reference each adapter's MCP config file against known SAP server IDs from loaded packs

Out of scope for this feature: installed tool versions (doctor-style checks), active BTP context (CF/BTP config files), incremental inject, adapter-specific formatting.

## Architecture

### Package layout

| Package | Responsibility |
|---|---|
| `internal/content` | `DynamicContext` struct; `RenderContext` updated to accept it |
| `internal/dynamic` | `GatherDynamic(GatherOpts)` — collects all runtime data |
| `internal/adapter` | `Options` gains `Dynamic *content.DynamicContext` field |
| `cmd/inject.go` | Populates `GatherOpts` (including cobra command list), calls `GatherDynamic`, passes result into `adapter.Options` |

`internal/dynamic` imports `internal/content` and `internal/adapter`. Import direction: `internal/dynamic` → `internal/adapter` → `internal/content`. `internal/dynamic` exports nothing that `internal/adapter` imports, so there is no cycle. The `DynamicContext` struct lives in `internal/content` so `RenderContext` can take it without importing `internal/dynamic`.

### Data model

```go
// internal/content/dynamic.go

type DynamicContext struct {
    CLIVersion      string
    ActiveProfile   string       // profile.Name, or profile.ID, or ""
    LoadedPackIDs   []string     // IDs of packs passed to RenderContext
    LastSynced      *time.Time   // nil = never synced
    ProjectType     string       // "" if undetected
    WiredMCPServers []WiredMCPEntry
    Commands        []CommandInfo
}

type WiredMCPEntry struct {
    AdapterName string
    ServerIDs   []string // only SAP MCP server IDs (cross-referenced against packs)
}

type CommandInfo struct {
    Name  string // cobra Use field (first word)
    Short string // cobra Short field
}
```

### GatherOpts

```go
// internal/dynamic/gather.go

type GatherOpts struct {
    CWD          string
    CLIVersion   string
    Profile      *content.Profile
    Packs        []*content.Pack
    SyncStateDir string
    Adapters     []adapter.Adapter
    Commands     []content.CommandInfo // populated from rootCmd.Commands() in cmd/inject.go
}

func GatherDynamic(opts GatherOpts) *content.DynamicContext
```

### Gather logic per item

**CLI self-awareness**  
`CLIVersion` — passed in via `GatherOpts.CLIVersion`, populated from the `cmd.Version` build-time var in `cmd/inject.go`. (Direct import of the `cmd` package from `internal/dynamic` would create a cycle and must not be done.) `ActiveProfile` — `profile.Name` if set, else `profile.ID`, else `""`. `Commands` — passed in from `cmd/inject.go` by walking `rootCmd.Commands()` and filtering `c.Hidden == false`.

**Pack freshness**  
`LoadedPackIDs` — `[p.ID for p in opts.Packs]`, no I/O. This reflects the pre-trim pack list (all packs loaded for this session), not the per-adapter trimmed subset — the intent is to show the user which packs are active, not which survived a budget cut on a specific adapter. `LastSynced` — read `sync-state.json` from `opts.SyncStateDir` using `internal/sync`'s `loadSyncState`. The known sync categories are `"tips"`, `"tools"`, `"resources"`, `"context"`, `"mcp"`, `"advocates"`. Take the most recent non-zero timestamp across all present category entries. If the file is absent or all entries are zero, `LastSynced` is nil.

**Project type**  
Check files in `opts.CWD` in priority order (first match wins):

| File | Label |
|---|---|
| `.cdsrc.json` | `CAP (Node.js)` |
| `package.json` containing `"@sap/cds"` in dependencies | `CAP (Node.js)` |
| `pom.xml` containing `com.sap.cds` | `CAP (Java)` |
| `mta.yaml` | `Multi-target Application (MTA)` |
| `xs-app.json` | `Fiori / BAS app` |
| `package.json` (any) | `Node.js` |

No directory recursion. File read errors are silently skipped.

**Wired SAP MCP servers**  
For each adapter with `mcp_config.path` set:
1. Expand `~` and read the JSON file
2. Extract the object at `mcp_config.key` (e.g. `mcpServers`)
3. Collect top-level keys as installed server IDs
4. Cross-reference against all `Pack.MCPServers[i].ID` values from `opts.Packs` — only keep matches

This ensures only SAP-specific servers are surfaced, not the user's full MCP server list. The `Pack.MCPServers` field is a `[]content.MCPServer`; the relevant field is `MCPServer.ID`.

**Error handling**  
All gather steps silently skip on any error (file not found, parse failure, etc.). `GatherDynamic` always returns a non-nil `*DynamicContext`; missing data is represented as zero values (nil pointer, empty string, empty slice).

### RenderContext changes

`RenderContext` gains a third parameter:

```go
func RenderContext(packs []*Pack, profile *Profile, dynamic *DynamicContext) string
```

If `dynamic` is nil, the function behaves exactly as before (backward compatible — all existing tests pass without modification).

When non-nil, a `## sap-devs Runtime Context` section is prepended immediately after the top-level heading and profile line, before any pack content:

```markdown
# SAP Developer Context

This context is maintained by sap-devs and provides up-to-date SAP developer knowledge.

**Developer Profile:** CAP Developer — Building cloud-native apps with SAP CAP on BTP

## sap-devs Runtime Context

**CLI:** sap-devs v1.2.3 | **Profile:** CAP Developer | **Packs:** cap, btp
**Last synced:** 2026-04-15 10:30 (2 hours ago)
**Project type:** CAP (Node.js)
**Wired SAP MCP servers (Claude Code):** sap-cap-mcp

**Available commands:**
- `inject` — Push SAP context to your AI tools
- `sync` — Fetch latest content from official/company repos
- `tip` — Show a random SAP developer tip
- `doctor` — Check local tool versions against pack requirements
- `mcp` — Browse and wire SAP MCP servers into AI tool configs

Run `sap-devs inject` to refresh this context · `sap-devs sync --force` to update content

## SAP CAP (Cloud Application Programming Model)
...
```

Omitted lines when data is absent:
- `Last synced: never — run sap-devs sync` replaces the timestamp line when `LastSynced` is nil
- `Project type` line omitted entirely when `ProjectType` is `""`
- `Wired SAP MCP servers` line omitted entirely when no matches found for any adapter

### Engine changes

`adapter.Options` gains one field:

```go
Dynamic *content.DynamicContext // nil = no dynamic section
```

`Engine.Run()` passes `opts.Dynamic` to `content.RenderContext`:

```go
ctx := content.RenderContext(trimmed, e.profile, e.opts.Dynamic)
```

### inject command changes

In `cmd/inject.go`, after loading packs and before constructing `adapter.Options`:

```go
var cmdInfos []content.CommandInfo
for _, c := range rootCmd.Commands() {
    if !c.Hidden {
        cmdInfos = append(cmdInfos, content.CommandInfo{
            Name:  strings.SplitN(c.Use, " ", 2)[0],
            Short: c.Short,
        })
    }
}

dynCtx := dynamic.GatherDynamic(dynamic.GatherOpts{
    CWD:          cwd,
    CLIVersion:   cmd.Version,
    Profile:      activeProfile,
    Packs:        packs,
    SyncStateDir: paths.CacheDir,
    Adapters:     adapters,
    Commands:     cmdInfos,
})

opts := adapter.Options{
    Scope:      scope,
    ToolFilter: injectTool,
    DryRun:     injectDryRun,
    Stats:      injectStats,
    Out:        cmd.OutOrStdout(),
    Dynamic:    dynCtx,
}
```

## Testing

- `internal/content`: unit tests for `RenderContext` with a non-nil `DynamicContext` — verify section present, formatting, nil-safety when `dynamic` is nil
- `internal/dynamic/gather_test.go`:
  - **Project type**: write files (`.cdsrc.json`, `package.json` with/without `@sap/cds`, `mta.yaml`, etc.) to a `t.TempDir()` and assert the correct label
  - **Pack freshness**: write a `sync-state.json` to a temp dir with `{"categories":{"context":"<RFC3339 time>"}}`; assert `LastSynced` is non-nil and correct; assert nil when file absent
  - **MCP detection**: write a temp JSON file `{"mcpServers":{"sap-cap-mcp":{},"some-other-mcp":{}}}` and set `Adapters[0].MCPConfig.Path` to that file; pass a pack with `MCPServers[0].ID = "sap-cap-mcp"`; assert only `"sap-cap-mcp"` appears in output (not `"some-other-mcp"`)
  - **Error handling**: missing CWD, missing sync state, missing MCP config file — assert `GatherDynamic` returns a non-nil context with zero-value fields, not a panic
- All existing `RenderContext` callers pass `nil` as the third argument — no changes needed to existing tests

## Files to create / modify

| Action | Path |
|---|---|
| Create | `internal/content/dynamic.go` — `DynamicContext`, `WiredMCPEntry`, `CommandInfo` types |
| Modify | `internal/content/render.go` — update `RenderContext` signature and implementation |
| Modify | `internal/content/render_test.go` — add dynamic section tests |
| Create | `internal/dynamic/gather.go` — `GatherOpts`, `GatherDynamic` |
| Create | `internal/dynamic/gather_test.go` |
| Modify | `internal/adapter/engine.go` — add `Dynamic` field to `Options`, thread into `RenderContext` |
| Modify | `cmd/inject.go` — build `CommandInfo` slice, call `GatherDynamic`, wire into `Options` |
