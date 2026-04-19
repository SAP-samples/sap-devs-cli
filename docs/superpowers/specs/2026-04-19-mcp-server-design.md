# MCP Server Design — `sap-devs mcp serve`

## Summary

Expose `sap-devs` as a live MCP server so AI agents can query SAP developer knowledge on demand, instead of relying solely on static injected text. The server runs as a stdio-based child process spawned by the AI tool, using the `mark3labs/mcp-go` SDK for protocol handling.

## Motivation

Static injection pushes everything upfront and hopes the agent reads it. With an MCP server, the agent pulls specific context when it needs it — no token budget pressure, always fresh from the cache, and topically relevant to the current task.

## Architecture

### Approach: Monolithic package with thin adapter handlers

A single `internal/mcpserver/` package. Each handler is a thin adapter calling existing functions from `internal/content/`, `internal/news/`, `internal/youtube/`, `internal/tutorials/`, `internal/learning/`. No new abstractions, no registry pattern.

### Package structure

| File | Purpose |
|------|---------|
| `server.go` | `NewServer(deps Deps) *server.MCPServer` — creates the mcp-go server, registers all tools |
| `tools_content.go` | Handlers: `list_packs`, `get_context`, `get_tip` |
| `tools_resources.go` | Handlers: `search_resources` |
| `tools_errors.go` | Handlers: `get_known_errors` |
| `tools_news.go` | Handlers: `get_recent_news` |
| `tools_learn.go` | Handlers: `search_tutorials`, `search_learning_journeys` |
| `tools_samples.go` | Handlers: `get_samples` |

### Command

New file: `cmd/mcp_serve.go`

```
sap-devs mcp serve [--profile <id>]
```

The command:

1. Creates a `ContentLoader` (same as `inject` does)
2. Loads packs, applies profile weights
3. Loads tutorial and learning journey indexes from cache (empty if not synced)
4. Passes everything to `mcpserver.NewServer()` — news is fetched lazily on first `get_recent_news` call, not at startup
5. Calls `server.ServeStdio(s)` — blocks until the client disconnects

The `mcp serve` command skips the background update check (same pattern as the `update` command in `root.go`) to avoid the 3-second post-run delay on server shutdown.

### Dependencies struct

```go
type Deps struct {
    Packs            []content.Pack
    Profile          *content.Profile
    NewsItems        []news.NewsItem              // may be empty; fetched lazily on first get_recent_news call
    TutorialIndex    []tutorials.TutorialMeta     // may be empty if not synced
    LearningIndex    []learning.LearningJourney   // may be empty if not synced
    Version          string                       // build version for server metadata
}
```

### Server metadata

- Name: `"sap-devs"`
- Version: injected from build ldflags (same `cmd.Version`)
- Capabilities: tools only (no resources or prompts)
- Instructions: `"SAP developer knowledge server. Use these tools to get SAP-specific context, tips, resources, error patterns, news, tutorials, and learning journeys on demand."`

## Tool Catalog (9 tools)

### Core content tools

| Tool | Parameters | Returns | Backing function |
|------|-----------|---------|-----------------|
| `list_packs` | *(none)* | JSON array of `{id, name, description, tags}` | iterates `Deps.Packs` |
| `get_context` | `pack` (optional string) | Rendered markdown context at `"full"` verbosity for active profile, or a specific pack | `pack.Context.AtLevel("full")` |
| `get_tip` | `topic` (optional string) | One tip as markdown — random per call (`time.Now().UnixNano()` seed). If `topic` given, passed as single-element `profileTags` to filter | `content.SelectTip(packs, tags, seed)` |

### Resource & error tools

| Tool | Parameters | Returns | Backing function |
|------|-----------|---------|-----------------|
| `search_resources` | `query` (required string), `pack` (optional string) | JSON array of `{id, title, url, type, tags}` | `content.FlattenResources` + `content.FilterResources` |
| `get_known_errors` | `query` (required string) | JSON array of `{id, pattern, cause, fix, docs, tags}` | `content.FlattenKnownErrors` + `content.FilterKnownErrors` |

### News tool

| Tool | Parameters | Returns | Backing function |
|------|-----------|---------|-----------------|
| `get_recent_news` | `count` (optional number, default 5) | JSON array of `{title, url, published, community_url}`. `community_url` is `""` when no matching blog post exists | Lazy-fetched on first call with 5s timeout, then cached for server lifetime |

### Learning & tutorial tools

| Tool | Parameters | Returns | Backing function |
|------|-----------|---------|-----------------|
| `search_tutorials` | `query` (required string) | JSON array of `{slug, title, description, url, tags}` | `tutorials.Search` against cached index |
| `search_learning_journeys` | `query` (required string) | JSON array of `{slug, title, level, duration, url}` | `learning.Search` against cached index |

### Samples tool

| Tool | Parameters | Returns | Backing function |
|------|-----------|---------|-----------------|
| `get_samples` | `pack` (optional string), `query` (optional string) | JSON array of `{id, label, description, url, tags}` | `content.FlattenSamples` + filtering |

All tools return JSON text via `mcp.NewToolResultText()`. Bad input returns `mcp.NewToolResultError()`. Go-level errors are never returned — the MCP protocol stays clean.

## Self-Install Wiring

The server is defined in the base pack so it's available to every profile.

### `content/packs/base/mcp.yaml`

```yaml
- id: sap-devs-server
  name: SAP Developer Context Server
  description: Live MCP server exposing SAP tips, resources, error patterns, news, tutorials, and learning journeys
  install:
    command: sap-devs
    args: ["mcp", "serve"]
  hosts:
    - claude-code
    - cursor
    - continue
```

### Install flow

1. User runs `sap-devs mcp install sap-devs-server`
2. Existing `content.FindMCPServer(packs, "sap-devs-server")` finds the entry
3. Existing `mcpWireAdapters` filters to detected tools with `mcp_config`
4. Existing `adapter.WriteMCPConfig` writes into the tool's JSON config
5. AI tool spawns `sap-devs mcp serve` as a child process on stdio

### Resulting AI tool config (e.g., Claude Code)

```json
{
  "mcpServers": {
    "sap-devs-server": {
      "command": "sap-devs",
      "args": ["mcp", "serve"]
    }
  }
}
```

No new adapter code needed. The existing `mcp-wire` mechanism handles everything.

**Schema validation:** The existing JSON schema for `mcp.yaml` in `content/schemas/` already covers all fields used in the new entry (`id`, `name`, `description`, `install.command`, `install.args`, `hosts`). No schema changes needed.

## Error Handling

**Content loading failures:** If packs fail to load (cache empty, never synced), the command prints a helpful error to stderr and exits non-zero before starting the server. The AI tool sees the process die and reports the error.

**News fetch failure:** `get_recent_news` fetches news lazily on first call (5-second timeout), then caches the result for the server's lifetime. If the fetch fails, returns an empty JSON array with a note suggesting `sap-devs sync` — not a protocol error. This avoids blocking server startup with network I/O while keeping news reasonably fresh per server session.

**Tutorials & learning journeys cache miss:** If the index files from `sap-devs sync` don't exist, the tools return empty results with a message suggesting the user run `sap-devs sync`. No crash.

**Stderr only:** All diagnostic output goes to stderr. The mcp-go SDK's `server.ServeStdio` owns stdout for JSON-RPC.

**Profile resolution:** `--profile` flag is optional. If omitted, loads the user's active profile from config (same as `inject`).

## Testing Strategy

**Unit tests:** Each handler is a pure function `(context.Context, mcp.CallToolRequest) → (*mcp.CallToolResult, error)`. Tests construct a `Deps` with known packs and call handlers directly.

**Integration test:** Construct a full server with `NewServer(deps)`, verify `tools/list` returns all 9 tools with correct schemas.

**CI-only:** Per project convention — `go test` fails on Windows Defender. Use `go build ./...` + `go vet ./...` locally.

## New dependency

```
github.com/mark3labs/mcp-go  (latest stable)
```

## Files to create/modify

| Action | Path |
|--------|------|
| Create | `internal/mcpserver/server.go` |
| Create | `internal/mcpserver/tools_content.go` |
| Create | `internal/mcpserver/tools_resources.go` |
| Create | `internal/mcpserver/tools_errors.go` |
| Create | `internal/mcpserver/tools_news.go` |
| Create | `internal/mcpserver/tools_learn.go` |
| Create | `internal/mcpserver/tools_samples.go` |
| Create | `cmd/mcp_serve.go` |
| Create | `content/packs/base/mcp.yaml` |
| Modify | `go.mod` / `go.sum` (add mcp-go dependency) |
| Modify | `CLAUDE.md` (document the new command) |
| Modify | `content/packs/base/context.md` (add mcp serve to CLI reference table) |
