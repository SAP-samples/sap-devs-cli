# Hook System Design

**Date:** 2026-04-17
**Status:** Draft

---

## Problem

AI agents (Claude Code, Cursor, Copilot) support hook/event systems that run shell commands at lifecycle points such as session start. There is currently no way to wire `sap-devs` commands into these hooks declaratively. Users who want a daily SAP tip greeted at session start must manually configure their tool's settings file — which is fragile, non-portable, and undiscoverable.

The inject adapter system already solves analogous problems for MCP server configuration. Hooks follow the same shape: a pack declares what should run, an adapter declares how to write it, and a CLI command installs/removes/reports status.

---

## Goals

1. Add a `hook.yaml` file to packs (initially only `base`) that declares hook commands for specific adapter events.
2. Add a `hook_config` block to adapter YAML that declares how to write hook entries into the tool's config file.
3. Add a `sap-devs hook list/install/uninstall/status` top-level command.
4. Add `--markdown` and `--plain` output flags to `sap-devs tip` to support hook-friendly output formats.
5. Document all of the above in `docs/content-authoring.md`, `CLAUDE.md` commands table, adapter YAML, and JSON schemas.

---

## Non-Goals

- Per-project hook installation — global scope only; hooks are personal tool configuration, not project files.
- Hook execution at runtime — `sap-devs` installs hooks, it does not run them.
- Hook support for non-file-based tools (clipboard-export adapters) — those have no settings file to write to.
- Locale variants for `hook.yaml` — hook commands are command names, not translatable strings.
- Multiple events per hook entry — one `id` maps to one event and one command.

---

## Design

### `--markdown` and `--plain` flags on `sap-devs tip`

Claude Code `sessionStart` hooks capture stdout. The current `tip` output is ANSI-rendered via glamour — not suitable for AI context. Two new flags:

- `--markdown`: emit raw Markdown (`## 💡 Title\n\nContent\n`) without glamour rendering.
- `--plain`: emit plain text (`Title\n\nContent\n`) without Markdown or ANSI.

Default (no flag): current glamour-rendered ANSI output. No existing behaviour changes.

Implementation: two boolean flags on `tipCmd`. A helper function selects the format branch.

### `hook.yaml` per pack

A new optional file `content/packs/<id>/hook.yaml` declares hook entries for this pack:

```yaml
- id: tip-on-session-start
  event: sessionStart
  command: "sap-devs tip --markdown"
  tools:
    - claude-code
```

Fields:

| Field     | Type     | Description                                                                    |
| --------- | -------- | ------------------------------------------------------------------------------ |
| `id`      | string   | Unique within the pack. Used by `hook install <id>` and `hook uninstall <id>`. |
| `event`   | string   | Tool-neutral event name. `sessionStart` is the first supported value.          |
| `command` | string   | Shell command to run when the event fires.                                     |
| `tools`   | []string | Adapter IDs this hook applies to. Controls which `hook_config` block is used.  |

The `event` field is a tool-neutral name. The adapter's `hook_config` block maps it to the tool-specific key (e.g. `hooks.SessionStart` in Claude Code's `settings.json`).

### `internal/content/pack.go` changes

Add `HookDef` struct and `Hooks []HookDef` field to `Pack`:

```go
type HookDef struct {
    ID      string   `yaml:"id"`
    Event   string   `yaml:"event"`
    Command string   `yaml:"command"`
    Tools   []string `yaml:"tools"`
    PackID  string   // set at load time
}
```

In `Pack` struct, add `Hooks []HookDef` (after `Tips []Tip`).

`LoadPack` loads `hook.yaml` from the pack directory (silent skip when absent), sets `PackID` on each entry, and populates `Pack.Hooks`.

### `internal/content/hook.go` (new file)

```go
// FlattenHooks returns all hook entries across all packs.
func FlattenHooks(packs []*Pack) []HookDef

// FindHookDef returns the first HookDef with the given ID across all packs, or nil.
func FindHookDef(packs []*Pack, id string) *HookDef
```

`FlattenHooks` mirrors `FlattenMCPServers`. `FindHookDef` mirrors `FindMCPServer`. Both are used by `cmd/hook.go`.

### `HookConfig` in `Adapter` struct

```go
type HookConfig struct {
    Path   string `yaml:"path"`
    Format string `yaml:"format"` // "json" only for now
    Key    string `yaml:"key"`    // dot-separated JSON path, e.g. "hooks.SessionStart"
}
```

`Adapter` gains a new field:

```go
HookConfig *HookConfig `yaml:"hook_config,omitempty"`
```

### `claude-code.yaml` adapter update

```yaml
hook_config:
  path: "~/.claude/settings.json"
  format: json
  key: "hooks.SessionStart"
```

`SessionStart` fires exactly once when a Claude Code session begins or resumes. It is the correct event for delivering a session greeting tip.

**Claude Code `settings.json` hook entry structure:**

An empty `matcher` (`""`) is equivalent to `"*"` — both match all session start types (`startup`, `resume`, `clear`, `compact`). The spec uses `""` as the canonical form; `WriteHookConfig` writes `""`.

```json
{
  "hooks": {
    "SessionStart": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "sap-devs tip --markdown"
          }
        ]
      }
    ]
  }
}
```

An empty `matcher` matches all session start types (`startup`, `resume`, `clear`, `compact`). `WriteHookConfig` must write this nested structure. `HookConfigInstalled` must detect if the entry already exists (by checking if `command` appears in any hook entry) to support idempotent install and accurate `status`.

### `internal/adapter/hook_wire.go`

New file alongside `mcp_wire.go`:

```go
// WriteHookConfig adds a hook command entry to the tool's settings JSON.
// Idempotent: if the command is already present, it is a no-op.
func WriteHookConfig(settingsPath, key, command string, dryRun bool) error

// RemoveHookConfig removes a hook command entry from the tool's settings JSON.
// No-op if the entry is not present.
func RemoveHookConfig(settingsPath, key, command string, dryRun bool) error

// HookConfigInstalled reports whether the command appears in the settings JSON.
func HookConfigInstalled(settingsPath, key, command string) (bool, error)
```

The `key` parameter is a dot-separated JSON path (e.g. `"hooks.SessionStart"`). The functions split `key` on `.` and navigate or create the nested structure dynamically — they do not hardcode field names.

**`WriteHookConfig` algorithm:**

1. Read existing JSON (or start empty object).
2. Split `key` on `.` to get path segments (e.g. `["hooks", "SessionStart"]`). Navigate to the array at that path, creating intermediate objects and the array itself if absent.
3. Check if any existing entry already contains a `hooks` array with an entry whose `command` matches — if so, return nil (idempotent).
4. Append a new matcher entry: `{"matcher": "", "hooks": [{"type": "command", "command": <cmd>}]}`.
5. Write back with indentation.

**`RemoveHookConfig` algorithm:**

1. Read existing JSON — if file absent, return nil.
2. Split `key` on `.` and navigate to the array. If path does not exist, return nil.
3. Filter out any entry whose nested `hooks` array contains an entry with the matching `command`.
4. Write back. If the array becomes empty, remove the key entirely (clean uninstall).

### `cmd/hook.go`

Top-level `hook` command with four subcommands:

**`hook list`** — lists all hook entries from active profile's packs. Prints a table. Accepts `--all` flag to show hooks from all packs regardless of active profile (consistent with `mcp list --all`):

```
ID                        PACK    EVENT          COMMAND                    TOOLS
tip-on-session-start      base    sessionStart   sap-devs tip --markdown    claude-code
```

**`hook install [id]`** — installs a hook. If `id` is omitted, installs all hooks from active profile packs for detected adapters. For each hook:
1. Detect which adapters from the hook's `tools` list are installed (using `adapter.Detect()`, same as `mcp install`).
2. If multiple adapters detected, prompt the user to choose (same `pickAdapters` logic as `mcp install`). If only one adapter detected, proceed without prompting.
3. For each selected adapter that has a `hook_config` block, expand `hook_config.path` (tilde expansion).
4. Call `WriteHookConfig`.
5. Print confirmation or "already installed".

**`hook uninstall [id]`** — uninstalls a hook by ID. Resolves adapter, calls `RemoveHookConfig`. Prints confirmation or "not installed".

**`hook status`** — for each hook in active profile packs, checks `HookConfigInstalled` and prints a status table:

```
ID                        PACK    ADAPTER       STATUS
tip-on-session-start      base    claude-code   ✓ installed
```

### Event name → JSON key mapping

The adapter's `hook_config.key` field holds the tool-specific JSON path. The `event` field in `hook.yaml` is tool-neutral. The mapping is:

| `event` | Claude Code `hook_config.key` |
|---|---|
| `sessionStart` | `hooks.SessionStart` |

Future events (e.g. `sessionEnd`, `onSave`) would add new `hook_config.key` values. The current design only supports `sessionStart` via `SessionStart`; the architecture is open for extension.

---

## Content: `content/packs/base/hook.yaml`

```yaml
- id: tip-on-session-start
  event: sessionStart
  command: "sap-devs tip --markdown"
  tools:
    - claude-code
```

---

## Documentation Updates

### 1. `docs/content-authoring.md` — Pack directory structure

Add `hook.yaml` to the directory tree (after `mcp.yaml`):

```
└── hook.yaml           # Hook commands wired by `sap-devs hook install`
```

### 2. `docs/content-authoring.md` — New `## Hook Authoring` section

Add a new top-level section after the `## The \`### Agent Instructions\` Pattern` section. It should cover:

- What `hook.yaml` is and what it does
- The `hook.yaml` schema (fields table)
- The `event` values and their tool equivalents
- Authoring constraints: hooks run on every event fire — keep `command` fast (< 200ms); avoid commands that write to stdout in unexpected formats
- The `tools` field: must match a known adapter ID that has `hook_config` configured
- Example: the `base` pack's `tip-on-session-start` hook
- How to install: `sap-devs hook install` (installs all), `sap-devs hook install tip-on-session-start` (installs one)

### 3. `CLAUDE.md` — CLI Commands table

Add row after the `mcp` row:

```
| `hook list/install/uninstall/status` | Wire AI tool lifecycle hooks from pack definitions |
```

### 4. `content/schemas/hook.schema.json` — New JSON Schema

New schema file validating `hook.yaml`:

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "Hook definitions",
  "type": "array",
  "items": {
    "type": "object",
    "required": ["id", "event", "command", "tools"],
    "additionalProperties": false,
    "properties": {
      "id": { "type": "string" },
      "event": { "type": "string", "enum": ["sessionStart"] },
      "command": { "type": "string" },
      "tools": { "type": "array", "items": { "type": "string" } }
    }
  }
}
```

### 5. `.vscode/settings.json` — Wire schema

Add `hook.yaml` to the YAML schema associations alongside `mcp.yaml`.

### 6. Adapter YAML documentation

The `HookConfig` struct fields should be documented in `docs/content-authoring.md` (or the referenced `content-guide.md`) alongside the existing `MCPConfig` documentation so adapter authors know how to configure `hook_config` for new tools.

---

## Files Changed

| File | Change |
|---|---|
| `cmd/tip.go` | Add `--markdown` and `--plain` flags |
| `internal/content/pack.go` | Add `HookDef` struct, `Hooks []HookDef` field to `Pack`, load `hook.yaml` in `LoadPack` |
| `internal/content/hook.go` | New — `FlattenHooks(packs []*Pack) []HookDef` helper |
| `internal/adapter/adapter.go` | Add `HookConfig` struct, `HookConfig *HookConfig` field to `Adapter` |
| `internal/adapter/hook_wire.go` | New — `WriteHookConfig`, `RemoveHookConfig`, `HookConfigInstalled` |
| `cmd/hook.go` | New — `hook list/install/uninstall/status` command |
| `content/packs/base/hook.yaml` | New — `tip-on-session-start` hook entry |
| `content/adapters/claude-code.yaml` | Add `hook_config` block |
| `content/schemas/hook.schema.json` | New — JSON Schema for `hook.yaml` |
| `.vscode/settings.json` | Wire `hook.schema.json` to `hook.yaml` |
| `docs/content-authoring.md` | Directory tree + new Hook Authoring section |
| `CLAUDE.md` | Add `hook` row to CLI commands table |

---

## Testing

### `cmd/tip.go`
- `--markdown` flag: output starts with `## 💡`, no ANSI escape sequences.
- `--plain` flag: output has no `#`, no ANSI sequences, no blockquote `>`.
- Default (no flag): output contains ANSI (glamour renders it); existing test coverage unchanged.

### `internal/content/pack.go`
- `LoadPack` on a pack dir with `hook.yaml` populates `Pack.Hooks`.
- `LoadPack` on a pack dir without `hook.yaml` leaves `Pack.Hooks` nil/empty.
- `PackID` is set on each loaded `HookDef`.

### `internal/content/hook.go`
- `FlattenHooks` returns all hook entries across multiple packs.
- `FlattenHooks` with empty packs returns empty slice.
- `FindHookDef` returns the correct `HookDef` when the ID exists across packs.
- `FindHookDef` returns `nil` when no hook with the given ID exists.

### `internal/adapter/hook_wire.go`
- `WriteHookConfig` creates the file when absent.
- `WriteHookConfig` is idempotent (second call with same command does not duplicate).
- `WriteHookConfig` preserves existing keys in the JSON file.
- `RemoveHookConfig` removes the entry; file still valid JSON after removal.
- `RemoveHookConfig` on a file with no matching entry is a no-op.
- `HookConfigInstalled` returns `true` when installed, `false` when not.

### `cmd/hook.go`
- `hook list` prints the hook table (integration: uses temp packs).
- `hook install` calls `WriteHookConfig` for each applicable adapter.
- `hook uninstall` calls `RemoveHookConfig` for each applicable adapter.
- `hook status` prints installed/not-installed per hook.

### Dry-run verification
```bash
SAP_DEVS_DEV=1 go run . hook install --dry-run
```
Expected: prints what would be written to `~/.claude/settings.json` without modifying it.
