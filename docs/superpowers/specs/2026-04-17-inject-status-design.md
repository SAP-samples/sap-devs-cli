# inject --status Design

**Goal:** Give users visibility into the state of all sap-devs injections across detected AI tools — whether content is present, well-formed, current, and what share of each config file sap-devs occupies.

**Architecture:** A new `Status()` method on `Engine` iterates `file-inject` adapters, reads each target file, and returns a `[]StatusRow` slice. The command layer renders the rows as either a tabwriter table or JSON. Staleness is detected by rendering the current content pipeline and comparing it to the on-disk section content.

**Tech Stack:** Go stdlib (`os`, `strings`, `regexp`, `encoding/json`, `text/tabwriter`); reuses existing `findSection`, `ExpandHome`, and pack-render helpers already in `internal/adapter/`.

---

## Command Interface

`inject --status` is a new flag on the existing `inject` command.

**Compatible flags:**
- `--tool <id>` — limit scan to one adapter (e.g. `--tool claude-code`)
- `--project` — scan project-scope targets only (default: global)
- `--json` — emit JSON array instead of tabwriter table
- `--verbose` — show stretch-goal columns (file size, token breakdown, other sections)

**Mutually exclusive with `--status`:**
- `--uninstall`
- `--dry-run`
- `--sync` / `--no-sync`
- `--stats`

**Human-readable output (default):**
```
Tool            Scope    File                        Status
Claude Code     global   ~/.claude/CLAUDE.md         ✓ current
Claude Code     project  .claude/CLAUDE.md           ✗ stale
Cursor          global   ~/.cursor/rules/sap.mdc     ✗ not found
Copilot         global   ~/.github/copilot.md        ✓ current
```

**With `--verbose` (stretch-goal columns appended):**
```
Tool            Scope    File                    Status      Size    Tokens  SAP%  Other sections
Claude Code     global   ~/.claude/CLAUDE.md     ✓ current   14 KB   3200    42%   cursor(1)
```

**With `--json`:**
```json
[
  {
    "adapter":         "claude-code",
    "name":            "Claude Code",
    "scope":           "global",
    "path":            "~/.claude/CLAUDE.md",
    "file_exists":     true,
    "injected":        true,
    "orphaned":        false,
    "stale":           false,
    "file_size_bytes": 14200,
    "file_token_est":  3200,
    "sap_devs_tokens": 1350,
    "other_sections":  [{"name": "cursor", "tokens": 400}]
  }
]
```

`--json` always includes all fields (including stretch-goal fields), regardless of `--verbose`.

---

## Data Model

New file: `internal/adapter/status.go`

```go
// SectionInfo describes a non-sap-devs fenced block found in a target file.
type SectionInfo struct {
    Name   string // tool prefix, e.g. "cursor" from <!-- cursor:start:Rules -->
    Tokens int
}

// StatusRow is the result of inspecting one adapter target.
type StatusRow struct {
    AdapterName string
    AdapterID   string
    Scope       string
    TargetPath  string // unexpanded (~-form)

    FileExists bool
    Injected   bool // sap-devs section present and well-formed
    Orphaned   bool // markers found but mismatched/reversed

    // Stale is true when the on-disk section content differs from what
    // inject would write today. Always false when FileExists=false or
    // Injected=false, or when the engine has no packs loaded.
    Stale bool

    // Stretch-goal fields — always populated when FileExists=true.
    FileSizeBytes int
    FileTokenEst  int           // word count × 1.3
    SapDevsTokens int           // token estimate for sap-devs section only
    OtherSections []SectionInfo // non-sap-devs fenced blocks in the file
}
```

JSON tags are added to all fields (snake_case). `OtherSections` marshals as `[]` not `null` when empty.

---

## Engine Method

`Status() ([]StatusRow, error)` is added to `Engine` in `internal/adapter/engine.go`.

**Algorithm per adapter target:**

1. Skip adapters where `ToolFilter` doesn't match (existing filter logic).
2. Skip non-`file-inject` adapters (MCP wire already has `mcp status`).
3. Skip targets where `target.Scope != e.opts.Scope`.
4. `ExpandHome(target.Path)` → absolute path.
5. `os.ReadFile(path)` — if `IsNotExist`, set `FileExists=false` and continue. Other errors are collected with `errors.Join` but don't abort.
6. `FileExists=true`. Run `findSection` for `replace-section` targets:
   - `sectionFound` → `Injected=true`
   - `sectionOrphaned` → `Orphaned=true`
   - `sectionNotFound` → neither flag set
7. For `replace-file` targets: file existing means `Injected=true`.
8. **Staleness** (only when `Injected=true` and `e.packs != nil`):
   - Call `renderSectionContent(a)` to get the current rendered string (see Render Helper — applies `TrimPacks` with the adapter's budget).
   - For `replace-section`: extract the on-disk bytes between the markers. `findSection` returns `startIdx`/`endIdx` pointing to the start of each marker string; the inner content slice is `fileBytes[startIdx+len(startMarker)+1 : endIdx]` (skip marker + trailing `\n`).
   - For `replace-file`: use the full file bytes (minus the preamble prefix).
   - `Stale = (rendered != onDisk)` — direct string equality after `TrimSpace`.
9. **Stretch-goal fields** (always populated when `FileExists=true`):
   - `FileSizeBytes = len(fileBytes)`
   - `FileTokenEst = estimateTokens(string(fileBytes))` where `estimateTokens(s) = len(strings.Fields(s)) * 13 / 10`
   - `SapDevsTokens` = `estimateTokens` of the sap-devs section slice (or 0 if not injected)
   - `OtherSections` = result of `scanOtherSections(string(fileBytes))`
10. Append row to `rows`.

**Error handling:** `errors.Join` collects all per-target errors. The partial `rows` slice is returned alongside the error so the caller can display whatever was found.

---

## Helper Functions (in `status.go`)

```go
// estimateTokens returns a rough token estimate: word count × 1.3.
func estimateTokens(s string) int {
    return len(strings.Fields(s)) * 13 / 10
}

// scanOtherSections finds non-sap-devs HTML-comment fenced blocks.
// Pattern: <!-- <prefix>:start:<name> --> where prefix != "sap-devs".
func scanOtherSections(content string) []SectionInfo
```

`scanOtherSections` uses a single compiled regexp: `<!-- ([^:]+):start:[^>]+ -->`. For each match where group 1 is not `"sap-devs"`, find the matching end marker and record the token estimate of the enclosed content. Returns `[]SectionInfo{}` (not nil) when no sections found.

---

## Render Helper

`renderSectionContent(a Adapter) string` is a new private method on `Engine` that mirrors the full render pipeline used in `Run()`: apply `content.TrimPacks(e.packs, maxBytes)` using the adapter's `MaxBytes`/`MaxTokens` budget, then render context and format output. It returns the string that would be written between markers (or as the full file for `replace-file`).

This must replicate `TrimPacks` to avoid false-positive staleness reports on budget-constrained adapters: if packs are rendered without trimming, the comparison content will exceed what `inject` actually wrote, making every budget-trimmed file appear stale.

The method is used by both `Status()` (staleness check) and `runFileInject()` (replacing inline rendering logic). This removes duplication from `runFileInject` as a secondary benefit, but the primary driver is correctness in `Status()`.

---

## Staleness Algorithm Detail

For `replace-section`:
```
rendered  = renderSectionContent(a)             // what inject would write today
onDisk    = bytes between start and end markers  // what's currently in the file
Stale     = strings.TrimSpace(rendered) != strings.TrimSpace(onDisk)
```

`TrimSpace` normalises trailing newlines to avoid false positives from whitespace-only differences.

For `replace-file`:
```
rendered  = preamble + renderSectionContent(a)   // full file that inject would write
onDisk    = string(fileBytes)
Stale     = strings.TrimSpace(rendered) != strings.TrimSpace(onDisk)
```

---

## Command Layer

In `cmd/inject.go`, the `--status` block follows the same early-return pattern as `--uninstall`:

```go
if injectStatus {
    // mutual exclusion check
    // load adapters + packs + profile
    // build engine with Status-appropriate options
    res, err := eng.Status()
    if err != nil { return err }
    if injectJSON {
        // json.MarshalIndent(res, ...) → stdout
    } else {
        // tabwriter table; if --verbose, include stretch-goal columns
    }
    return nil
}
```

New package-level vars: `injectStatus bool`, `injectJSON bool`, `injectVerbose bool`.

`--json` and `--verbose` are registered as flags but are silently valid only when `--status` is also set. If used without `--status`, they are ignored (no error) — this is intentional: keeping them as simple booleans avoids cross-flag validation complexity, and a user who accidentally passes `--json` to a normal `inject` run will not see broken output (inject produces no stdout anyway). `--stats` is similarly a different output mode and is listed as mutually exclusive with `--status` (see exclusion list above); this must be validated in the mutual-exclusion check, not just in the flag documentation.

---

## i18n Keys

New keys in `internal/i18n/catalogs/en.json` and `de.json`:

| Key | English value |
|---|---|
| `inject.status.header_tool` | `Tool` |
| `inject.status.header_scope` | `Scope` |
| `inject.status.header_file` | `File` |
| `inject.status.header_status` | `Status` |
| `inject.status.current` | `✓ current` |
| `inject.status.stale` | `✗ stale` |
| `inject.status.not_found` | `✗ not found` |
| `inject.status.orphaned` | `✗ orphaned` |
| `inject.status.not_injected` | `✗ not injected` |
| `inject.status.no_results` | `No file-inject adapters found for the given scope/tool.` |
| `inject.status.append_warning` | `sap-devs warning: {{.Path}} uses append mode — injection state cannot be determined` |

---

## Testing

**`internal/adapter/status_test.go`** (new file, `package adapter`):
- `TestStatus_Current` — write a file with a valid sap-devs section matching current render; assert `Injected=true, Stale=false`
- `TestStatus_Stale` — write a file with outdated content in the section; assert `Stale=true`
- `TestStatus_NotFound` — target file absent; assert `FileExists=false, Injected=false`
- `TestStatus_Orphaned` — file with start marker but no end marker; assert `Orphaned=true`
- `TestStatus_NotInjected` — file exists but has no sap-devs markers; assert `FileExists=true, Injected=false`
- `TestStatus_ToolFilter` — two adapters, `ToolFilter` set to one; assert only one row returned
- `TestStatus_ScopeFilter` — target has scope "project", engine scope "global"; assert no rows
- `TestStatus_ReplaceFile` — `replace-file` mode target; assert file-existence maps to `Injected=true`
- `TestStatus_OtherSections` — file with one sap-devs block + one cursor block; assert `OtherSections` has one entry
- `TestStatus_TokenEstimate` — known string; assert `FileTokenEst` matches expected value
- `TestStatus_ErrorContinues` — one target with unreadable path (permissions error); assert error returned but other rows still populated

**`cmd/inject_status_test.go`** (new file, `package cmd`):
- `TestInjectStatus_FlagExists` — `--status` flag registered
- `TestInjectStatus_MutualExclusion` — `--status` + `--uninstall` returns error
- `TestInjectStatus_JSONAndVerboseNoErrorWithoutStatus` — `--json` + `--verbose` alone don't error

**`internal/adapter/status_helpers_test.go`** (or inline in `status_test.go`):
- `TestEstimateTokens` — unit test for token estimator
- `TestScanOtherSections_Empty` — no non-sap-devs sections
- `TestScanOtherSections_OneMatch` — one cursor block found
- `TestScanOtherSections_IgnoresSapDevs` — sap-devs block not included in results

---

## Out of Scope

- MCP wire adapter status (already covered by `mcp status`)
- Clipboard-export adapters (ephemeral; no persistent state to check)
- `append`-mode targets (no markers to detect; emit a warning to stderr using a new i18n key `inject.status.append_warning`: `"sap-devs warning: {{.Path}} uses append mode — injection state cannot be determined"`)
- Automatic repair / re-injection on stale detection
