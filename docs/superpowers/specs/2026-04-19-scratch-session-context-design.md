# Scratch/Session Context Design

## Problem

The AI agent knows SAP best practices from pack content but has no awareness of what the developer is currently working on. Facts like "implementing draft enablement for Books entity" or "HANA service only bound in dev space" live only in the developer's head. This gap means the agent's responses are generic when they could be specifically informed.

## Solution

Add a `context` command that lets users append ephemeral working notes to a per-project scratch file. These notes are injected as a `## Current Context` section at the top of the project-scope injected block, making them the first thing the agent reads.

## Commands

```
sap-devs context add "note text"   # append a note
sap-devs context list              # show current notes (default subcommand)
sap-devs context clear             # delete all notes, no confirmation
```

## Storage

**File:** `.sap-devs/scratch.yaml` in the project root (current working directory).

```yaml
notes:
  - "currently implementing draft enablement for Books entity"
  - "HANA service only bound in dev space, not test"
```

- Created on first `context add`; parent `.sap-devs/` directory created as needed.
- `.sap-devs/` is the project content layer directory (already documented as gitignore-worthy).
- Notes are ephemeral, not synced, not versioned, not shared.
- `context clear` deletes the file entirely.

## New Package: `internal/scratch`

A focused package with three exported functions:

| Function | Signature | Behavior |
| --- | --- | --- |
| `Load` | `Load(dir string) ([]string, error)` | Reads `.sap-devs/scratch.yaml`, returns notes slice. Returns empty slice (no error) if file doesn't exist. |
| `Add` | `Add(dir, note string) error` | Trims whitespace; rejects empty/whitespace-only notes with an error. Loads existing notes, appends the new note, writes back. Creates `.sap-devs/` directory if needed. |
| `Clear` | `Clear(dir string) error` | Removes `.sap-devs/scratch.yaml`. No error if file doesn't exist. |

Internal YAML structure:

```go
type scratchFile struct {
    Notes []string `yaml:"notes"`
}
```

## New Command: `cmd/context.go`

Cobra command tree:

- `context` (aliases: none; parent `RunE` delegates to `list` logic)
  - `context add <note>` — validates non-empty after trimming, calls `scratch.Add(cwd, note)`, prints confirmation
  - `context list` — calls `scratch.Load(cwd)`, prints bullet list or "no notes" hint
  - `context clear` — calls `scratch.Clear(cwd)`, prints "no notes to clear" if file doesn't exist, otherwise prints confirmation

All subcommands use `os.Getwd()` for the project directory. No flags needed.

## Inject Integration

### DynamicContext extension

Add a new field to `content.DynamicContext`:

```go
ScratchNotes []string
```

### Gather phase (`cmd/inject.go`)

When `--project` scope is active, after building the dynamic context:

```go
if injectProject {
    notes, _ := scratch.Load(cwd)
    dynCtx.ScratchNotes = notes
}
```

Scratch loading is intentionally done in `cmd/inject.go` after `GatherDynamic` returns, not inside `internal/dynamic/gather.go`, because scratch is a project-scope-only concern that does not belong in the general dynamic-gathering pipeline. The adapter engine's scope filter ensures only project-scope adapters run when `--project` is passed, so there is no risk of scratch notes leaking into global-scope output.

Errors are silently ignored (consistent with all other dynamic context gathering).

### Render phase (`internal/content/render.go`)

In `RenderContext`, before the `renderDynamic` call, check for scratch notes:

```go
if dynamic != nil && len(dynamic.ScratchNotes) > 0 {
    b.WriteString("## Current Context\n\n")
    for _, note := range dynamic.ScratchNotes {
        // Sanitize: collapse newlines to spaces, cap length
        sanitized := strings.ReplaceAll(note, "\n", " ")
        sanitized = strings.ReplaceAll(sanitized, "\r", "")
        if len(sanitized) > 500 {
            sanitized = TrimToBytes(sanitized, 500) + "..."
        }
        b.WriteString("- " + sanitized + "\n")
    }
    b.WriteString("\n")
}
```

Notes are user-authored free text, so they are sanitized before injection: newlines are collapsed to spaces (to preserve bullet-list structure) and length is capped at 500 bytes per note using the existing `TrimToBytes` helper (which respects UTF-8 rune boundaries).

This places `## Current Context` as the first content section after the header and profile line — before `## sap-devs Runtime Context` and all pack content.

### Rendering order in injected output

(Matches actual `RenderContext` flow in `render.go`)

1. `# SAP Developer Context` (header)
2. `**Developer Profile:**` line
3. **`## Current Context`** (scratch notes, new — inserted before `renderDynamic` call)
4. `## sap-devs Runtime Context` (dynamic context from `renderDynamic`)
5. Preamble (from base pack `preamble.md`)
6. `## Constraints` (if any)
7. Pack context sections (each pack's `ContextMD`)
8. `## Canonical Patterns`, `## Recommended Learning Journeys`, `## Known Errors`

### Uninstall interaction

`inject --uninstall` removes injected sections from AI tool config files. It does **not** clear scratch notes. `context clear` is the explicit, separate action for that.

## CLI Manifest Update

Add a row to the CLI reference table in `content/packs/base/context.md`:

```
| `sap-devs context add "note"` | Developer wants to tell the agent about current work | Appends note to project scratch; visible in next `inject --project` |
| `sap-devs context list` | Check what scratch notes are set | Bullet list of current notes |
| `sap-devs context clear` | Done with current task, clear working notes | Removes all scratch notes |
```

## i18n Keys

Add keys to `en` and `de` catalogs:

| Key | English |
| --- | --- |
| `context.add.done` | `Added note to project context.` |
| `context.add.empty` | `Note cannot be empty.` |
| `context.list.empty` | `No scratch notes set. Use "sap-devs context add" to add one.` |
| `context.list.header` | `Current project context:` |
| `context.clear.done` | `Cleared all scratch notes.` |
| `context.clear.empty` | `No scratch notes to clear.` |

## Testing

- `internal/scratch`: unit tests for Load/Add/Clear with temp directories (empty file, missing file, existing notes, clear idempotent).
- `internal/content/render_test.go`: test that ScratchNotes render as `## Current Context` section, positioned before Runtime Context.
- `cmd/context.go`: integration tested via `go build ./...` and manual verification (consistent with other commands).

## Files to Create/Modify

| File | Action |
| --- | --- |
| `internal/scratch/scratch.go` | Create — Load/Add/Clear functions |
| `internal/scratch/scratch_test.go` | Create — unit tests |
| `cmd/context.go` | Create — cobra command with add/list/clear subcommands |
| `internal/content/dynamic.go` | Modify — add `ScratchNotes []string` to DynamicContext |
| `internal/content/render.go` | Modify — render Current Context section |
| `internal/content/render_test.go` | Modify — add test for scratch notes rendering |
| `cmd/inject.go` | Modify — load scratch notes when --project scope |
| `content/packs/base/context.md` | Modify — add context commands to CLI manifest table |
| `internal/i18n/catalogs/en.json` | Modify — add context.* keys |
| `internal/i18n/catalogs/de.json` | Modify — add context.* keys |
