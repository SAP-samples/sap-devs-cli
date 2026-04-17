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

`ReplaceSection` is refactored to call `findSection` and return an error when it gets `sectionOrphaned` (preserving existing behaviour). Used by both `ReplaceSection` and the new `RemoveSection`.

**`RemoveSection(path, section string, dryRun bool, w io.Writer) (removed bool, err error)`**

- Reads the file; returns `removed=false, err=nil` if file doesn't exist.
- Constructs `start = fmt.Sprintf("<!-- sap-devs:start:%s -->", section)` and `end = fmt.Sprintf("<!-- sap-devs:end:%s -->", section)`, then calls `findSection`.
- `sectionNotFound` → returns `removed=false, err=nil` (clean no-op).
- `sectionOrphaned` → returns `removed=false, err=<orphaned marker error>`.
- `sectionFound` → removes the block as follows:
  1. Start with `result = content[:startIdx] + content[endIdx+len(end):]`
  2. Consume the `\n` immediately after the end marker (i.e., advance `endIdx+len(end)` by 1 if that byte is `\n`) — same as `ReplaceSection` does today with `afterEnd++`.
  3. **Always** collapse any `\n\n\n` sequence in the result to `\n\n` — this step is unconditional, not optional cleanup. It is required to produce correct output when content surrounded the section with blank lines.
  4. Example: input `"before\n\n<!-- sap-devs:start:X -->\nbody\n<!-- sap-devs:end:X -->\n\nafter\n"` → after step 2 the trailing `\n` after the end marker is consumed → after step 3 no triple-newline remains → output `"before\n\nafter\n"`.
  5. Write the modified content back to the file.
- Dry-run: writes `fmt.Sprintf("[dry-run] would remove section %q from %s\n", section, path)` to `w`, no file write.
- Normal run: writes nothing to `w` (result lines are printed by the caller `runFileUninstall`).

**`DeleteFile(path string, dryRun bool, w io.Writer) (deleted bool, err error)`**

- Returns `deleted=false, err=nil` if file does not exist.
- Removes the file via `os.Remove`.
- Dry-run: writes `fmt.Sprintf("[dry-run] would delete %s\n", path)` to `w`, no deletion.
- Normal run: writes nothing to `w`.

**Refactoring note:** The existing `ReplaceSection` and `ReplaceFile` use `fmt.Printf` for dry-run output (writing to raw stdout). To keep the change set minimal, those functions are **not** migrated to `io.Writer` in this feature. Only the new `RemoveSection` and `DeleteFile` use the `w io.Writer` pattern, which allows their output to be captured in engine tests via `opts.Out`.

### Engine changes in `internal/adapter/engine.go`

Add to the `Options` struct:

```go
Uninstall bool
Lang      string  // populated from i18n.ActiveLang in cmd layer; used for i18n in runFileUninstall
```

`engine.Run()` returns a `RunResult` struct instead of bare `error`. This avoids a bare signature change while exposing the `Found` count to the command layer:

```go
type RunResult struct {
    Found    int   // sections/files actually removed; 0 during dry-run
    DryFound int   // targets that would be removed in dry-run (incremented per target when dryRun=true and section/file exists or would-be removed)
    Err      error
}

func (e *Engine) Run() RunResult { ... }
```

**All existing callers of `engine.Run()` must be updated.** There are 16 call sites:

- 13 in `internal/adapter/adapter_test.go` — change `require.NoError(t, eng.Run())` to `res := eng.Run(); require.NoError(t, res.Err)`.
- 1 in `cmd/inject.go` — change `if err := eng.Run(); err != nil` to `res := eng.Run(); if res.Err != nil`.
- 2 in `cmd/inject_test.go` — same pattern as the adapter tests.

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
- Expands `~/` paths via `ExpandHome()`. If `ExpandHome` returns an error for a target, collect it via `errors.Join` and continue to remaining targets (consistent with the collect-all-errors pattern, unlike the fail-fast behaviour in `runFileInject`).
- Dispatches by mode:
  - `replace-section` → `RemoveSection(path, target.Section, opts.DryRun, e.opts.Out)`
  - `replace-file` → `DeleteFile(path, opts.DryRun, e.opts.Out)`
  - `append` → writes `i18n.Tf(e.opts.Lang, "inject.uninstall.append_warning", map[string]any{"Path": path}) + "\n"` to `os.Stderr`; skips target (not counted as found, not an error).
  - Unknown mode → returns error immediately (same as `runFileInject`).
- After each `replace-section` or `replace-file` call:
  - If `opts.DryRun == true`: increment `dryFound`; `RemoveSection`/`DeleteFile` have already written the `[dry-run]` line to `e.opts.Out`; also write `fmt.Sprintf("  %s  — %s\n", path, i18n.T(e.opts.Lang, "inject.uninstall.not_found"))` to `e.opts.Out`.
  - If `removed`/`deleted` is true (normal mode): increment `found`; write `fmt.Sprintf("  %s  — %s\n", path, i18n.T(e.opts.Lang, key))` to `e.opts.Out` where key is `inject.uninstall.section_removed` or `inject.uninstall.file_deleted` accordingly.
  - If `removed`/`deleted` is false and not dry-run (section not present in file): write `fmt.Sprintf("  %s  — %s\n", path, i18n.T(e.opts.Lang, "inject.uninstall.not_found"))` to `e.opts.Out`.
- **Error handling:** collects all errors across all targets via `errors.Join`, returns after processing all targets (does not stop on first error).

### Command changes in `cmd/inject.go`

Add `--uninstall` boolean flag alongside existing inject flags.

Mutual exclusion: validated at the top of `RunE` — `--uninstall` is incompatible with `--sync` and `--no-sync`. `--stats` must **not** be added to the validation block; it is silently ignored by not passing it to `Options` when `injectUninstall` is true.

When `--uninstall` is set, skip:

- Staleness check
- Dynamic context gathering
- Pack loading (pass `nil` packs to the engine)

Use a `bytes.Buffer` as the `Out` writer passed to the engine so the command can inspect what was written:

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
eng := adapter.NewEngine(gatheredAdapters, nil, nil, opts)  // adapters loaded same as normal inject path; nil packs and profile
res := eng.Run()
if res.Err != nil {
    return res.Err
}
```

**Summary output logic:**

- In **normal mode**: if `res.Found > 0`, print `i18n.T(lang, "inject.uninstall.header")` then `buf.String()` to `cmd.OutOrStdout()`. Otherwise print `i18n.T(lang, "inject.uninstall.nothing_found")`.
- In **dry-run mode**: `res.Found` is always 0. Use `res.DryFound > 0` as the criterion: if non-zero, print `i18n.T(lang, "inject.uninstall.dry_run_header")` then `buf.String()`; otherwise print `inject.uninstall.nothing_found`.
- Note: if all matched targets are `append`-mode (warnings to stderr, `found == 0`, buf empty), `nothing_found` is printed on stdout. The stderr warnings convey the manual action required.

**Summary output format (normal mode):**

```text
Uninstalled SAP developer context:
  ~/.claude/CLAUDE.md  — section removed
  ~/.cursor/rules/sap-developer-context.mdc  — file deleted
```

**Summary output format (dry-run mode):**

```text
Would uninstall SAP developer context:
  [dry-run] would remove section "SAP Developer Context" from ~/.claude/CLAUDE.md
  ~/.claude/CLAUDE.md  — not found
```

If nothing was found:

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

Call site for append warning inside `runFileUninstall`: `i18n.Tf(e.opts.Lang, "inject.uninstall.append_warning", map[string]any{"Path": path})`. Always use `e.opts.Lang` inside engine code, not `i18n.ActiveLang` — the latter is a cmd-layer global and must not be referenced from the engine package.

## Data Flow

```text
inject --uninstall [--tool X] [--project] [--dry-run]
  └─ cmd/inject.go: validates flags, builds Options{Uninstall:true, Out: &buf, Lang: i18n.ActiveLang}
       └─ engine.Run() → RunResult{Found, Err}
            └─ for each adapter (--tool filter applied):
                 ├─ non-file-inject → skip (continue)
                 └─ file-inject → runFileUninstall(a) → (found int, err error)
                      ├─ scope filter applied per target
                      ├─ replace-section → RemoveSection(…, &buf) → [dry-run] line + result line to buf
                      ├─ replace-file    → DeleteFile(…, &buf)    → [dry-run] line + result line to buf
                      └─ append          → warning to os.Stderr, skips
       └─ cmd: normal: Found>0 → header+buf
              dry-run: DryFound>0 → dry_run_header+buf
              else → nothing_found
```

## Error Handling

- File not found: no-op, not an error.
- Section markers not found: no-op, not an error (printed in summary as "not found").
- Orphaned/mismatched markers (start without end or vice versa): `RemoveSection` returns an error; `runFileUninstall` collects it via `errors.Join` and continues to remaining targets.
- `append`-mode target: warning printed to `os.Stderr`, target skipped, not an error.
- `ExpandHome` error for a target: collected via `errors.Join`, continues to remaining targets.
- File permission errors: collected by `runFileUninstall` via `errors.Join`, returned after all targets processed.
- All-append adapter (only adapter matched has all `append` targets): `found == 0`, `buf` empty, stdout prints `nothing_found`. Stderr warnings convey the manual action required.
- `--uninstall` with `--sync` or `--no-sync`: validation error at the top of `RunE`.

## Testing

- Unit test `findSection`: given `content = "before\n<!-- sap-devs:start:X -->\nbody\n<!-- sap-devs:end:X -->\nafter\n"`, `startIdx = 7` (index of `<` of start marker; `"before\n"` = 7 bytes), `endIdx = 38` (index of `<` of end marker; `7 + len("<!-- sap-devs:start:X -->") + len("\nbody\n") = 7 + 25 + 6 = 38`; note the end marker `<!-- sap-devs:end:X -->` is 23 bytes — 2 fewer than the start marker due to `end` vs `start`); status = `sectionFound`. Also: start marker only → `sectionOrphaned`; end marker only → `sectionOrphaned`; neither → `sectionNotFound`.
- Unit test `RemoveSection`: section present → `removed=true`, content correct (`"before\n\n<!-- sap-devs:start:X -->\nbody\n<!-- sap-devs:end:X -->\n\nafter\n"` → `"before\n\nafter\n"`), no triple-newline left; section absent → `removed=false, err=nil`; file absent → `removed=false, err=nil`; orphaned start marker → `err!=nil`; orphaned end marker → `err!=nil`; dry-run → no file write, expected `[dry-run]` message written to buffer `w`.
- Unit test `DeleteFile`: file present → `deleted=true`; file absent → `deleted=false, err=nil`; dry-run → no deletion, expected `[dry-run]` message written to buffer `w`.
- Engine integration test: `--uninstall` removes a previously injected `replace-section` target and `RunResult.Found == 1`; `--uninstall` deletes a `replace-file` target and `Found == 1`; skips non-`file-inject` adapters; respects `--tool` filter; respects `--project` scope; `--dry-run` makes no disk changes and buffer contains `[dry-run]` lines; `append`-mode target emits warning to stderr, not counted in `Found`.
- Command-level test: `--uninstall` with `--sync` returns an error; `--uninstall` with `--no-sync` returns an error; `--uninstall` with `--stats` succeeds (stats ignored); nothing-found path prints `inject.uninstall.nothing_found`; dry-run with content prints dry-run header + `[dry-run]` lines.
- Caller update test: all 15 existing `engine.Run()` call sites updated to `RunResult` pattern (`res := eng.Run(); require.NoError(t, res.Err)`) — 13 in `internal/adapter/adapter_test.go`, 1 in `cmd/inject.go`, 2 in `cmd/inject_test.go`.

## Out of Scope

- MCP server registration cleanup (handled by the `mcp` command separately).
- `clipboard-export` and `file-export` cleanup.
- Hook configuration cleanup.
- Migrating existing `ReplaceSection`/`ReplaceFile` dry-run output to `io.Writer` (deferred to a future cleanup).
