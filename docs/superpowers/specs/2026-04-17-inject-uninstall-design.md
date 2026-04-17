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
- `append`-mode targets: skip with a warning to `os.Stderr` — `append` mode does not use fenced markers and cannot be auto-removed. This is defensive future-proofing; `append` mode is not currently implemented in `runFileInject` either.
- `clipboard-export`, `file-export`, `mcp-wire` adapters: silently skipped.
- `--stats` flag: silently ignored when `--uninstall` is set (no pack rendering occurs). It must **not** be added to the validation block — it is silently ignored by simply not passing it to `Options` when `injectUninstall` is true.
- Supports `--tool` to limit removal to a single adapter ID.
- Supports `--project` to only remove project-scope injections.
- Supports `--dry-run` to preview what would be removed without modifying files.
- Prints a per-file summary; no-ops cleanly if nothing is found.

## Architecture

### Approach

Option A: uninstall logic lives inside the existing engine. `engine.Run()` checks `opts.Uninstall` at the top of the per-adapter loop — before any pack trimming, rendering, or budget work — and dispatches to a new `runFileUninstall()` method instead of `runFileInject()`. All existing `--tool` and scope filtering applies unchanged.

### New primitives in `internal/adapter/file_inject.go`

**`findSection(content, start, end string) (startIdx, endIdx int, status sectionStatus)`**

Extracts the marker-search logic currently inlined in `ReplaceSection`. `sectionStatus` is a new unexported type with three values:

- `sectionFound` — both markers present and `startIdx < endIdx`.
- `sectionNotFound` — neither marker is present.
- `sectionOrphaned` — exactly one marker is present.

`startIdx` is the byte offset of the first character of the start marker string (matching `strings.Index` convention). `endIdx` is the byte offset of the first character of the end marker string (same convention). Both are only meaningful when `status == sectionFound`.

`ReplaceSection` is refactored to call `findSection` and return an error when it gets `sectionOrphaned` (preserving existing behaviour). When `findSection` returns `sectionNotFound`, `ReplaceSection` continues to the append-on-create path unchanged. No existing `ReplaceSection` behaviour changes — the refactor is purely internal.

`findSection` never returns an error itself. The caller is responsible for converting `sectionOrphaned` into an error with an appropriate message.

**`RemoveSection(path, section string, dryRun bool, w io.Writer) (found, removed bool, err error)`**

Returns three values:

- `found=true, removed=true`: live mode, section was present and has been removed.
- `found=true, removed=false`: dry-run mode, section was present but not removed (dry-run only previews).
- `found=false, removed=false`: section not present in file (either mode); `err=nil`.
- Any of the above with `err!=nil`: an error occurred.

Behaviour:

- Reads the file; returns `found=false, removed=false, err=nil` if file doesn't exist.
- Constructs `start` and `end` marker strings, then calls `findSection`.
- `sectionNotFound` → returns `found=false, removed=false, err=nil` (clean no-op).
- `sectionOrphaned` → returns `found=false, removed=false, err=<orphaned marker error>`.
- `sectionFound` in **dry-run** mode → writes `fmt.Sprintf("[dry-run] would remove section %q from %s\n", section, path)` to `w`; returns `found=true, removed=false, err=nil`. No file write.
- `sectionFound` in **live** mode → removes the block as follows:
  1. The removed region is bytes `[startIdx, endIdx+len(end))` — inclusive of both markers. Formula: `result = content[:startIdx] + content[endIdx+len(end):]`.
  2. Consume the `\n` immediately following the end marker (advance `endIdx+len(end)` by 1 if that byte is `\n`) — same as `ReplaceSection` does today with `afterEnd++`.
  3. **Always** apply a global collapse of three or more consecutive newlines to exactly two — unconditional, applied to all occurrences (not just the first). Required to produce correct output when content surrounded the section with blank lines. Use a loop or regexp: `for strings.Contains(result, "\n\n\n") { result = strings.ReplaceAll(result, "\n\n\n", "\n\n") }`.
  4. Example: `"before\n\n<!-- sap-devs:start:X -->\nbody\n<!-- sap-devs:end:X -->\n\nafter\n"` → step 2 consumes trailing `\n` → step 3 collapses nothing (no triple-newline) → output `"before\n\nafter\n"`.
  5. Write modified content back to the file.
  6. Returns `found=true, removed=true, err=nil`. Writes nothing to `w` (result lines are printed by `runFileUninstall`).

**`DeleteFile(path string, dryRun bool, w io.Writer) (found, deleted bool, err error)`**

Returns analogous three values: `found=true, deleted=true` (live, file existed and was removed), `found=true, deleted=false` (dry-run, file exists but not deleted), `found=false, deleted=false` (file absent).

- File absent → returns `found=false, deleted=false, err=nil`.
- File present, dry-run → writes `fmt.Sprintf("[dry-run] would delete %s\n", path)` to `w`; returns `found=true, deleted=false, err=nil`.
- File present, live → removes file via `os.Remove`; returns `found=true, deleted=true, err=nil` on success. Writes nothing to `w`.

**Refactoring note:** The existing `ReplaceSection` and `ReplaceFile` use `fmt.Printf` for dry-run output (writing to raw stdout). To keep the change set minimal, those functions are **not** migrated to `io.Writer` in this feature. Only the new `RemoveSection` and `DeleteFile` use the `w io.Writer` pattern.

### Engine changes in `internal/adapter/engine.go`

Add to the `Options` struct:

```go
Uninstall bool
// Lang is the active language for i18n in runFileUninstall.
// Populated from i18n.ActiveLang in the cmd layer.
// Always use e.opts.Lang inside engine code — do not reference i18n.ActiveLang directly
// from the engine package, as it is a cmd-layer global.
Lang string
```

`engine.Run()` returns a `RunResult` struct instead of bare `error`:

```go
type RunResult struct {
    Found    int   // sections/files actually removed (live mode only)
    DryFound int   // sections/files that would be removed (dry-run mode only; 0 in live mode)
    Err      error
}

func (e *Engine) Run() RunResult { ... }
```

**All existing callers of `engine.Run()` must be updated.** Use `grep -n '\.Run()' ./internal/adapter/ ./cmd/` to find all call sites before editing — do not rely on a hard-coded count. The pattern to apply is: change `require.NoError(t, eng.Run())` to `res := eng.Run(); require.NoError(t, res.Err)`, and `if err := eng.Run(); err != nil` to `res := eng.Run(); if res.Err != nil`. Call sites are in `internal/adapter/adapter_test.go`, `cmd/inject.go`, and `cmd/inject_test.go`.

In `engine.Run()`, the uninstall check is placed at the **top of the per-adapter loop body**, before any pack-trim, render, or budget operations:

```go
for _, a := range e.adapters {
    if e.opts.ToolFilter != "" && a.ID != e.opts.ToolFilter {
        continue
    }
    if e.opts.Uninstall {
        if a.Type == "file-inject" {
            n, dn, err := e.runFileUninstall(a)
            result.Found += n
            result.DryFound += dn
            if err != nil {
                result.Err = errors.Join(result.Err, err)
            }
        }
        continue  // skip all other adapter types and all budget/render work
    }
    // existing budget / trim / render path below
    ...
}
```

This avoids calling `TrimPacks` or `RenderContext` (which require loaded packs) during uninstall, where packs are not loaded.

**`runFileUninstall(a Adapter) (found, dryFound int, err error)`**:

- Iterates `a.Targets`.
- Applies scope filtering (skips targets whose scope doesn't match `opts.Scope`). An empty `opts.Scope` skips all targets (consistent with `runFileInject`). Tests must set `opts.Scope` explicitly.
- Expands `~/` paths via `ExpandHome()`. If `ExpandHome` returns an error for a target, collect it via `errors.Join` and continue to remaining targets (consistent with collect-all-errors, unlike fail-fast in `runFileInject`).
- Dispatches by mode:
  - `replace-section` → `RemoveSection(path, target.Section, opts.DryRun, e.opts.Out)` → `(found, removed bool, err)`
  - `replace-file` → `DeleteFile(path, opts.DryRun, e.opts.Out)` → `(found, deleted bool, err)`
  - `append` → writes `i18n.Tf(e.opts.Lang, "inject.uninstall.append_warning", map[string]any{"Path": path}) + "\n"` to `os.Stderr`; skips target (not an error, not counted).
  - Unknown mode → returns error immediately (same as `runFileInject`).
- After each `replace-section` or `replace-file` call, uses `found` (not `removed`/`deleted`) to determine output:
  - If `found && removed` (live mode, section/file was present and removed): increment the local `found` counter; write `fmt.Sprintf("  %s  — %s\n", path, i18n.T(e.opts.Lang, key))` to `e.opts.Out` where key is `inject.uninstall.section_removed` or `inject.uninstall.file_deleted`.
  - If `found && !removed` (dry-run mode, section/file is present): increment local `dryFound`; the `[dry-run]` line was already written to `e.opts.Out` by `RemoveSection`/`DeleteFile`. Write no additional line.
  - If `!found` (section/file not present, either mode): write `fmt.Sprintf("  %s  — %s\n", path, i18n.T(e.opts.Lang, "inject.uninstall.not_found"))` to `e.opts.Out`. Do not increment `found` or `dryFound`.
- **Error handling:** collects all errors via `errors.Join`, returns after processing all targets (does not stop on first error).

### Command changes in `cmd/inject.go`

Add `--uninstall` boolean flag alongside existing inject flags.

Mutual exclusion: validated at the top of `RunE` — `--uninstall` is incompatible with `--sync` and `--no-sync`. `--stats` must **not** be added to the validation block; it is silently ignored by not passing it to `Options` when `injectUninstall` is true.

When `--uninstall` is set, skip:

- Staleness check
- Dynamic context gathering
- Pack loading

Load adapters via `loadAdapters()` (same as the normal inject path — check the error, return it if non-nil). Pass `nil` packs and `nil` profile to `adapter.NewEngine` directly. Do not use `newAdapterEngine` — it requires loaded packs and performs layer merging that is unnecessary for uninstall.

Use a `bytes.Buffer` as the `Out` writer so the command can inspect what was written:

```go
var buf bytes.Buffer
opts := adapter.Options{
    Uninstall:  true,
    Scope:      scope,
    ToolFilter: injectTool,
    DryRun:     injectDryRun,
    Lang:       i18n.ActiveLang,
    Out:        &buf,
}
gatheredAdapters, err := loadAdapters()
if err != nil {
    return err
}
eng := adapter.NewEngine(gatheredAdapters, nil, nil, opts)
res := eng.Run()
if res.Err != nil {
    return res.Err
}
```

**Summary output logic:**

- In **normal mode**: if `res.Found > 0`, print `i18n.T(lang, "inject.uninstall.header")` then `buf.String()` to `cmd.OutOrStdout()`. Otherwise print `i18n.T(lang, "inject.uninstall.nothing_found")`.
- In **dry-run mode**: `res.Found` is always 0. Use `res.DryFound > 0`: if non-zero, print `i18n.T(lang, "inject.uninstall.dry_run_header")` then `buf.String()`; otherwise print `inject.uninstall.nothing_found`.
- Note: if all matched targets are `append`-mode (`found == 0`, `DryFound == 0`), `nothing_found` is printed on stdout alongside the stderr warnings. This combination — stderr append warnings + stdout nothing-found — is intentional: the stderr warnings convey the manual action required, and `nothing_found` accurately reflects that no automatic removal occurred.

**Summary output format (normal mode):**

```text
Uninstalled SAP developer context:
  ~/.claude/CLAUDE.md  — section removed
  ~/.cursor/rules/sap-developer-context.mdc  — file deleted
```

**Summary output format (dry-run mode, sections present):**

```text
Would uninstall SAP developer context:
  [dry-run] would remove section "SAP Developer Context" from ~/.claude/CLAUDE.md
  [dry-run] would delete ~/.cursor/rules/sap-developer-context.mdc
```

If nothing was found or would be found:

```text
No injected sections found.
```

**i18n keys** added to both `en` and `de` catalogs:

| Key | English | German |
| --- | ------- | ------ |
| `inject.uninstall.header` | `Uninstalled SAP developer context:` | `SAP-Entwicklerkontext deinstalliert:` |
| `inject.uninstall.dry_run_header` | `Would uninstall SAP developer context:` | `Würde SAP-Entwicklerkontext deinstallieren:` |
| `inject.uninstall.section_removed` | `section removed` | `Abschnitt entfernt` |
| `inject.uninstall.file_deleted` | `file deleted` | `Datei gelöscht` |
| `inject.uninstall.not_found` | `not found` | `nicht gefunden` |
| `inject.uninstall.nothing_found` | `No injected sections found.` | `Keine injizierten Abschnitte gefunden.` |
| `inject.uninstall.append_warning` | `warning: cannot auto-remove append-mode target {{.Path}} — remove manually` | `Warnung: Append-Ziel {{.Path}} kann nicht automatisch entfernt werden — bitte manuell löschen` |

Call site for append warning inside `runFileUninstall`: `i18n.Tf(e.opts.Lang, "inject.uninstall.append_warning", map[string]any{"Path": path})`. Always use `e.opts.Lang` inside engine code — do not reference `i18n.ActiveLang` from the engine package.

## Data Flow

```text
inject --uninstall [--tool X] [--project] [--dry-run]
  └─ cmd/inject.go: load adapters, build Options{Uninstall:true, Out:&buf, Lang:i18n.ActiveLang}
       └─ engine.Run() → RunResult{Found, DryFound, Err}
            └─ for each adapter (--tool filter applied):
                 ├─ non-file-inject → skip (continue)
                 └─ file-inject → runFileUninstall(a) → (found, dryFound int, err)
                      ├─ scope filter applied per target
                      ├─ replace-section → RemoveSection(…, &buf)
                      │     found+removed  → result line to buf (section removed)
                      │     found+!removed → [dry-run] line to buf (by primitive)
                      │     !found         → not-found line to buf
                      ├─ replace-file → DeleteFile(…, &buf)   [same pattern]
                      └─ append → warning to os.Stderr, skips
       └─ cmd: normal: Found>0   → header + buf
              dry-run: DryFound>0 → dry_run_header + buf
              else               → nothing_found
```

## Error Handling

- File not found: no-op, not an error.
- Section markers not found: no-op, not an error (printed in summary as "not found").
- Orphaned/mismatched markers (start without end or vice versa): `RemoveSection` returns an error; `runFileUninstall` collects it via `errors.Join` and continues to remaining targets.
- `append`-mode target: warning printed to `os.Stderr`, target skipped, not an error.
- `ExpandHome` error for a target: collected via `errors.Join`, continues to remaining targets.
- File permission errors: collected by `runFileUninstall` via `errors.Join`, returned after all targets processed.
- `loadAdapters()` error: returned immediately from `RunE`.
- `--uninstall` with `--sync` or `--no-sync`: validation error at the top of `RunE`.

## Testing

- Unit test `findSection`: given `content = "before\n<!-- sap-devs:start:X -->\nbody\n<!-- sap-devs:end:X -->\nafter\n"`, verify `startIdx` and `endIdx` using `strings.Index` directly in the test rather than hard-coding computed values (manual byte counting is error-prone); status = `sectionFound`. Also: start marker only → `sectionOrphaned`; end marker only → `sectionOrphaned`; neither → `sectionNotFound`.
- Unit test `RemoveSection` (live mode): section present → `found=true, removed=true`, content is `"before\n\nafter\n"` for input `"before\n\n<!-- sap-devs:start:X -->\nbody\n<!-- sap-devs:end:X -->\n\nafter\n"`, no triple-newline; section absent → `found=false, removed=false, err=nil`; file absent → `found=false, removed=false, err=nil`; orphaned start → `err!=nil`; orphaned end → `err!=nil`.
- Unit test `RemoveSection` (dry-run): section present → `found=true, removed=false`, `[dry-run]` message written to `w`, file unchanged; section absent → `found=false, removed=false`, nothing written to `w`.
- Unit test `DeleteFile`: file present, live → `found=true, deleted=true`; file absent → `found=false, deleted=false, err=nil`; dry-run, file present → `found=true, deleted=false`, `[dry-run]` message written to `w`, file not deleted.
- Engine integration test: `--uninstall` removes a previously injected `replace-section` target and `RunResult.Found == 1`; `--uninstall` deletes a `replace-file` target and `Found == 1`; skips non-`file-inject` adapters; respects `--tool` filter; respects `--project` scope; `--dry-run` makes no disk changes, `DryFound == 1`, buf contains `[dry-run]` line; `append`-mode target emits warning to stderr, `Found == 0`, `DryFound == 0`.
- Command-level test: `--uninstall` with `--sync` returns an error; `--uninstall` with `--no-sync` returns an error; `--uninstall` with `--stats` succeeds; nothing-found path prints `inject.uninstall.nothing_found`; dry-run with content prints dry-run header + `[dry-run]` lines.
- Regression: verify all existing `ReplaceSection` tests pass after extracting `findSection`; no new `ReplaceSection` tests needed beyond confirming the refactor does not change observable behaviour.
- Caller update: update all `engine.Run()` call sites in `internal/adapter/adapter_test.go`, `cmd/inject.go`, and `cmd/inject_test.go` to the `RunResult` pattern (`res := eng.Run(); require.NoError(t, res.Err)`). Use `grep -n '\.Run()' ./internal/adapter/ ./cmd/` to locate all sites before editing.

## Out of Scope

- MCP server registration cleanup (handled by the `mcp` command separately).
- `clipboard-export` and `file-export` cleanup.
- Hook configuration cleanup.
- Migrating existing `ReplaceSection`/`ReplaceFile` dry-run output to `io.Writer` (deferred to a future cleanup).
