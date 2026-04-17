# inject --uninstall Design

**Date:** 2026-04-17  
**Project:** sap-devs-cli  
**Status:** Approved

## Problem

There is no clean way to reverse `inject`. Users who want to stop using sap-devs, switch tools, or debug a clean state must manually find and delete fenced sections from files like `~/.claude/CLAUDE.md`. This is error-prone and creates a bad off-boarding experience.

## Solution

Add `--uninstall` as a boolean flag to the existing `inject` command. When set, the engine iterates all `file-inject` adapters and removes previously injected content instead of writing it.

## Scope

- `replace-section` adapters: remove the fenced `<!-- sap-devs:start:… -->` / `<!-- sap-devs:end:… -->` block from the target file.
- `replace-file` adapters: delete the entire file (sap-devs owns these files entirely).
- `clipboard-export`, `file-export`, `mcp-wire` adapters: silently skipped.
- Supports `--tool` to limit removal to a single adapter ID.
- Supports `--project` to only remove project-scope injections.
- Supports `--dry-run` to preview what would be removed without modifying files.
- Prints a per-file summary; no-ops cleanly if nothing is found.

## Architecture

### Approach

Option A: uninstall logic lives inside the existing engine. `engine.Run()` checks `opts.Uninstall` and dispatches to a new `runFileUninstall()` method instead of `runFileInject()`. All existing `--tool` and scope filtering applies unchanged.

### New primitives in `internal/adapter/file_inject.go`

**`findSection(content, start, end string) (startIdx, endIdx int, found bool)`**

Extracts the marker-search logic currently inlined in `ReplaceSection`. Returns byte offsets of the start marker's beginning through the end marker's end (inclusive of trailing newline). Used by both `ReplaceSection` and the new `RemoveSection`.

**`RemoveSection(path, section string, dryRun bool) (removed bool, err error)`**

- Reads the file; returns `removed=false, err=nil` if file doesn't exist.
- Calls `findSection` to locate the fenced block.
- Returns `removed=false, err=nil` if no markers found (clean no-op).
- Slices the block out of the file content, collapsing any resulting double blank lines.
- Writes the modified content back to the file.
- Dry-run: prints `[dry-run] would remove section %q from %s`, no file write.

**`DeleteFile(path string, dryRun bool) (deleted bool, err error)`**

- Returns `deleted=false, err=nil` if file does not exist.
- Removes the file.
- Dry-run: prints `[dry-run] would delete %s`, no deletion.

### Engine changes in `internal/adapter/engine.go`

Add `Uninstall bool` to the `Options` struct.

In `engine.Run()`, at the adapter-type dispatch switch, add a check:

```go
if e.opts.Uninstall {
    if a.Type == "file-inject" {
        if err := e.runFileUninstall(a); err != nil { ... }
    }
    continue
}
```

**`runFileUninstall(a Adapter)`** mirrors `runFileInject` in structure:
- Iterates `a.Targets`.
- Applies scope filtering (skips targets whose scope doesn't match `opts.Scope`).
- Expands `~/` paths via `ExpandHome()`.
- Dispatches by mode:
  - `replace-section` → `RemoveSection(path, target.Section, opts.DryRun)`
  - `replace-file` → `DeleteFile(path, opts.DryRun)`
- Collects per-file results (path, action taken / skipped) and returns them for summary output.

### Command changes in `cmd/inject.go`

Add `--uninstall` boolean flag alongside existing inject flags.

Mutual exclusion: validated at the top of `RunE` — `--uninstall` is incompatible with `--sync` and `--no-sync`.

When `--uninstall` is set, skip:
- Staleness check
- Dynamic context gathering

Build engine with `Options{Uninstall: true, Scope: scope, ToolFilter: injectTool, DryRun: injectDryRun}` and run.

**Summary output:**

```
Uninstalled SAP developer context:
  ~/.claude/CLAUDE.md                              — section removed
  ~/.cursor/rules/sap-developer-context.mdc        — file deleted
```

If nothing was found:

```
No injected sections found.
```

With `--dry-run`, each action line is prefixed with `[dry-run]`.

Summary strings are added to the i18n catalog under `inject.uninstall.*` keys (both `en` and `de`).

## Data Flow

```
inject --uninstall [--tool X] [--project] [--dry-run]
  └─ cmd/inject.go: validates flags, builds Options{Uninstall:true, ...}
       └─ engine.Run()
            └─ for each file-inject adapter (filtered by --tool / scope):
                 ├─ replace-section target → RemoveSection() → removed bool
                 └─ replace-file target    → DeleteFile()    → deleted bool
                      └─ summary printed to stdout
```

## Error Handling

- File not found: no-op, not an error.
- Section markers not found: no-op, not an error (logged in summary as "not found").
- Orphaned/mismatched markers (start without end or vice versa): `RemoveSection` returns an error, same behaviour as `ReplaceSection` today.
- File permission errors: returned as errors, printed to stderr.

## Testing

- Unit test `RemoveSection`: section present → removed; section absent → no-op; file absent → no-op; orphaned marker → error; dry-run → no write.
- Unit test `DeleteFile`: file present → deleted; file absent → no-op; dry-run → no deletion.
- Unit test `findSection`: extracted helper, tested independently.
- Engine integration test: `--uninstall` removes a previously injected section; skips non-file-inject adapters; respects `--tool` filter; respects `--project` scope; `--dry-run` makes no changes.
- Command-level test: `--uninstall` with `--sync` returns an error.

## Out of Scope

- MCP server registration cleanup (handled by the `mcp` command separately).
- `clipboard-export` and `file-export` cleanup.
- Hook configuration cleanup.
