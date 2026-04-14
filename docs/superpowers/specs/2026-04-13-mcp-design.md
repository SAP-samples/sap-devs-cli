# sap-devs mcp — Design Specification

## Goal

Add `sap-devs mcp` so developers can discover, install, and check the status of SAP MCP servers in their AI tools. Detection of installed host tools narrows the install prompt to only tools present on the machine.

## Commands

```sh
sap-devs mcp list              # list servers from active profile
sap-devs mcp list --all        # list all servers from all packs

sap-devs mcp install <id>      # detect installed hosts, prompt to pick, wire
sap-devs mcp install --all     # detect hosts once, prompt once, wire all profile servers
sap-devs mcp install --dry-run # preview without writing

sap-devs mcp status            # read each mcp-wire adapter config; show what is registered
```

## Content Schema

`mcp.yaml` is already parsed by `LoadPack` into `pack.MCPServers`. The structs in `internal/content/pack.go`:

```go
type MCPServer struct {
    ID          string     `yaml:"id"`
    Name        string     `yaml:"name"`
    Description string     `yaml:"description"`
    Install     MCPInstall `yaml:"install"`
    Hosts       []string   `yaml:"hosts"` // adapter IDs that support this server
}

type MCPInstall struct {
    Command string   `yaml:"command"`
    Args    []string `yaml:"args"`
}
```

Adapter YAML files in `content/adapters/` already define `MCPConfig` for mcp-wire adapters:

```go
type MCPConfig struct {
    Path   string `yaml:"path"`   // e.g. "~/.claude/settings.json"
    Format string `yaml:"format"` // "json"
    Key    string `yaml:"key"`    // e.g. "mcpServers"
}
```

## Architecture

### What already exists (do not re-implement)

- `MCPServer`, `MCPInstall` structs — in `internal/content/pack.go`
- `mcp.yaml` loading — in `LoadPack` (populates `pack.MCPServers`)
- `Adapter`, `MCPConfig`, `DetectRule` structs — in `internal/adapter/adapter.go`
- `LoadAdapters(dir)` — in `internal/adapter/adapter.go`
- `WriteMCPConfig(path, key, server, dryRun)` — in `internal/adapter/mcp_wire.go`
- `ExpandHome(path)` — in `internal/adapter/file_inject.go`
- `LoadPacks(nil)` / `LoadPacks(profile)` — in `internal/content/loader.go`
- Profile resolution — `config.LoadProfile` + `loader.FindProfile`
- Multi-layer adapter loading logic — in `newAdapterEngine` in `cmd/root.go` (loads official cache, optional company cache, optional `SAP_DEVS_DEV=1` local fallback; uses `mergeAdapters` for override-by-ID)

### New code: `internal/content/mcp.go`

```go
// FlattenMCPServers returns all MCPServer entries across the given packs in order.
func FlattenMCPServers(packs []*Pack) []MCPServer

// FindMCPServer returns the first MCPServer with the given ID across packs, or nil.
func FindMCPServer(packs []*Pack, id string) *MCPServer
```

No deduplication is needed for `FlattenMCPServers`; the same server ID will not appear in multiple packs in practice. `FindMCPServer` returns the first match.

### New code: `internal/adapter/detect.go`

```go
// Detect returns true if the adapter is present on this machine.
// It iterates the adapter's Detect rules and returns true on the first passing rule.
// A "command" rule passes if the command exits with code 0.
// A "path" rule passes if the expanded path exists on the filesystem.
// Returns false if Detect is empty or all rules fail.
func Detect(a Adapter) bool
```

`Detect` uses `exec.Command` for command rules (splits on spaces: `parts[0]` is executable, `parts[1:]` are args) and `os.Stat` + `ExpandHome` for path rules.

### New code: `internal/adapter/mcp_wire.go` (addition)

```go
// ReadMCPConfig reads the mcpServers map from a JSON settings file.
// Returns an empty map (not an error) if the file does not exist.
// Returns an error if the file exists but cannot be parsed as JSON,
// or if the key exists but is not a JSON object.
func ReadMCPConfig(settingsPath, key string) (map[string]interface{}, error)
```

### New code: `cmd/mcp.go`

Thin presentation layer only.

#### Profile resolution for `list` and `install --all`

| Context | Behaviour |
|---------|-----------|
| `mcp list` (no flag) | resolve active profile → `loader.LoadPacks(profile)`; error if no profile set |
| `mcp list --all` | `loader.LoadPacks(nil)` |
| `mcp install <id>` | `loader.LoadPacks(nil)` (search all packs for the server) |
| `mcp install --all` | resolve active profile → `loader.LoadPacks(profile)` |
| `mcp status` | `loader.LoadPacks(nil)` + all mcp-wire adapters |

#### `mcp list` flow

1. Resolve packs per profile resolution table above
2. `content.FlattenMCPServers(packs)`
3. If empty: print `"No MCP servers found for your current profile."` and return nil
4. Print aligned table (columns: TOOL, PACK, HOSTS, NAME)

Example output:
```
ID                   PACK        HOSTS                    NAME
cap-mcp-server       cap         claude-code, cursor      SAP CAP MCP Server
```

#### `mcp install <id>` flow

1. `loader.LoadPacks(nil)` → `content.FindMCPServer(packs, id)` → if nil: error `"MCP server %q not found — use 'sap-devs mcp list --all' to browse"`
2. `adapter.LoadAdapters(adaptersDir)` → filter to adapters where `a.Type == "mcp-wire"` AND `a.ID` is in `server.Hosts` AND `a.MCPConfig != nil`
3. For each candidate: `adapter.Detect(a)` → keep only detected adapters
4. If none detected: error `"no compatible hosts detected for %q — install one of: %s"` (list all hosts from server.Hosts)
5. Print numbered list of detected hosts with their config paths
6. Read user input → parse selection (integers, comma-separated, or `"all"`)
7. For each chosen adapter: `adapter.WriteMCPConfig(expandedPath, a.MCPConfig.Key, *server, dryRun)`
8. Print confirmation: `"✓ Registered %s in %s"` per host, or dry-run preview

#### `mcp install --all` flow

1. Resolve active profile → `loader.LoadPacks(profile)` → `content.FlattenMCPServers(packs)`
2. If empty: print `"No MCP servers defined for your current profile."` and return nil
3. Collect union of all host adapter IDs across all servers
4. `adapter.LoadAdapters(adaptersDir)` → filter to `type: mcp-wire`, `MCPConfig != nil`, ID in union, and `Detect` passes
5. If none detected: error (same wording as single-server case, listing all unique host IDs)
6. Print numbered list of detected hosts → read user input → parse selection
7. For each server, for each chosen adapter where `a.ID` is in `server.Hosts`: `WriteMCPConfig`
8. Print summary: `"Registered N server(s) in X host(s)"`

#### `mcp status` flow

1. `adapter.LoadAdapters(adaptersDir)` → filter to `type: mcp-wire` AND `MCPConfig != nil`
2. `loader.LoadPacks(nil)` → `content.FlattenMCPServers(packs)`
3. If no adapters and no servers: print `"No MCP adapters or servers found."` and return nil
4. For each mcp-wire adapter: `adapter.ReadMCPConfig(expandedPath, a.MCPConfig.Key)` → registered server ID map
5. Print aligned table (columns: SERVER, HOST, STATUS)
   - STATUS: `installed` if server.ID is a key in the adapter's registered map, else `not installed`
   - Only show rows where the adapter's ID is in `server.Hosts`

Example output:
```
SERVER           HOST          STATUS
cap-mcp-server   claude-code   installed
cap-mcp-server   cursor        not installed
```

#### Interactive prompt helper

Define a package-level function `pickAdapters(adapters []adapter.Adapter) ([]adapter.Adapter, error)` in `cmd/mcp.go`. It prints the numbered list, reads a line from `os.Stdin` via `bufio.NewReader`, and parses `"all"` or comma/space-separated integers. Returns an error if the input cannot be parsed or all indices are out of range.

#### adaptersDir resolution

`cmd/mcp.go` needs a `[]adapter.Adapter` slice, not an `adapter.Engine`. The multi-layer loading logic (official cache → company cache → `SAP_DEVS_DEV` fallback, with `mergeAdapters` for override-by-ID) currently lives inline in `newAdapterEngine` in `cmd/root.go`.

**Modify `cmd/root.go`:** extract that loading into a new package-level helper:

```go
// loadAdapters returns the merged adapter list across all configured layers.
// Official cache first, company cache overrides by ID, dev fallback if SAP_DEVS_DEV=1.
func loadAdapters() ([]adapter.Adapter, error)
```

Then have `newAdapterEngine` call `loadAdapters` internally (no behaviour change), and have `cmd/mcp.go` call `loadAdapters()` directly.

Also clarify expanding paths: wherever `WriteMCPConfig` or `ReadMCPConfig` is called with `a.MCPConfig.Path`, first expand it via `adapter.ExpandHome(a.MCPConfig.Path)`. Same for `status` flow step 4.

## Output Format

### `mcp list` table

```
ID                   PACK        HOSTS                    NAME
cap-mcp-server       cap         claude-code, cursor      SAP CAP MCP Server
```

Column widths: ID 24, PACK 12, HOSTS 28, NAME remainder. Separator line of dashes to total width.

### `mcp status` table

```
SERVER           HOST          STATUS
cap-mcp-server   claude-code   installed
cap-mcp-server   cursor        not installed
```

Column widths: SERVER 20, HOST 14, STATUS remainder. Separator line of dashes.

### Install confirmation

```
✓ Registered cap-mcp-server in ~/.claude/settings.json
```

Or dry-run:
```
[dry-run] would add MCP server "cap-mcp-server" to ~/.claude/settings.json[mcpServers]
```

(The dry-run line is printed by `WriteMCPConfig` itself — `cmd/mcp.go` does not need to format it.)

## Error Handling

- Server ID not found: `error: MCP server %q not found — use 'sap-devs mcp list --all' to browse`
- No compatible hosts detected: `error: no compatible hosts detected for %q — install one of: %s`
- No profile set (list/install --all): same error as other commands: `"no profile set — run 'sap-devs profile set <name>' first"`
- `WriteMCPConfig` failure (bad JSON, permissions): surface error directly
- `ReadMCPConfig` on missing file: return empty map, no error
- Invalid interactive input: `error: invalid selection %q — enter numbers (e.g. 1,2) or "all"`

## Testing

### `internal/content/mcp_test.go`

- `TestFlattenMCPServers` — two packs each with one server → slice of length 2 in order
- `TestFlattenMCPServers_Empty` — packs with no servers → empty slice, no nil panic
- `TestFindMCPServer_Found` — finds server by ID across two packs
- `TestFindMCPServer_NotFound` — returns nil when ID absent

### `internal/adapter/detect_test.go`

- `TestDetect_PathRule_Exists` — creates a temp file, detect rule with that path → true
- `TestDetect_PathRule_Missing` — non-existent path → false
- `TestDetect_CommandRule_Success` — rule with `"go version"` (always present in CI) → true
- `TestDetect_CommandRule_Fail` — rule with `"sap-devs-nonexistent-binary"` → false
- `TestDetect_AnyPassesReturnsTrue` — two rules, first (bad command) fails, second (existing path) passes → true
- `TestDetect_Empty` — adapter with no detect rules → false

### `internal/adapter/mcp_wire_test.go` (addition)

- `TestReadMCPConfig_Missing` — file does not exist → empty map, no error
- `TestReadMCPConfig_Present` — file with `{"mcpServers":{"cap-mcp":{"command":"npx"}}}` → map with one entry
- `TestReadMCPConfig_BadKeyType` — key exists but is not an object → error

## Files

- **Create:** `internal/content/mcp.go` — `FlattenMCPServers`, `FindMCPServer`
- **Create:** `internal/content/mcp_test.go`
- **Create:** `internal/adapter/detect.go` — `Detect`
- **Create:** `internal/adapter/detect_test.go`
- **Modify:** `internal/adapter/mcp_wire.go` — add `ReadMCPConfig`
- **Modify:** `internal/adapter/mcp_wire_test.go` — add `ReadMCPConfig` tests
- **Modify:** `cmd/root.go` — extract `loadAdapters() ([]adapter.Adapter, error)` from `newAdapterEngine`; have `newAdapterEngine` call it
- **Create:** `cmd/mcp.go` — `mcp` Cobra command with `list`, `install`, `status` subcommands
