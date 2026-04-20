# Content Editing UI — Phase 2a: Undo/Redo + Pre-Save Diff View

## Context

Phase 1 shipped an interactive terminal-based YAML editor (`sap-devs content edit`) with a schema-driven form view, list view with layer badges, and multi-layer merge/save logic. Phase 2 adds six enhancements, decomposed into three sub-projects:

- **Group A (this spec):** Undo/redo within editor session + diff view before save
- **Group B (future):** Drag-and-drop reordering + bulk editing
- **Group C (future):** Content creation wizard for new packs

## Goal

Add an undo/redo stack and a pre-save change confirmation screen to the array editor, so users can confidently experiment with edits knowing they can revert mistakes and review all changes before writing to disk.

## Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Undo granularity | Entry-level (whole operations) | Aligns with the existing loop-based architecture where each action exits Bubbletea; avoids hooking into huh's internal form state |
| Undo depth | Unlimited | Content files have 5–50 entries; even deep stacks are a few KB |
| Implementation strategy | Snapshot stack (deep copy of `[]MergedItem` before each mutation) | Simpler than command pattern; no inverse-operation bugs; diff view falls out as snapshot[0] vs current |
| Diff view trigger | Pre-save confirmation on `q` (save & quit) | Acts as a safety net before writing to disk; similar to `git diff --stat` before commit |
| Scope | Array editor only | Object editor is a single-form flow with no list loop; undo/redo not meaningful there |
| Git integration | Dropped | Developers already have their own git workflow; recreating it in the TUI is overkill |

## Architecture

### New file: `internal/editor/history.go`

The only new file. Contains the snapshot stack, change detection, and diff view model.

### Data structures

```go
type Snapshot struct {
    Items []MergedItem
    Desc  string // e.g. "edited SAP TechEd", "added new item", "deleted Devtoberfest"
}

type History struct {
    baseline  []MergedItem // initial state from LoadMergedItems — never mutated
    undoStack []Snapshot   // past states
    redoStack []Snapshot   // future states (after undo)
}
```

**Operations:**

- `NewHistory(items []MergedItem) *History` — stores a deep copy as baseline
- `Push(items []MergedItem, desc string)` — deep-copies items onto undoStack, clears redoStack
- `Undo() ([]MergedItem, string, bool)` — pops undoStack, pushes current to redoStack, returns restored items + description
- `Redo() ([]MergedItem, string, bool)` — pops redoStack, pushes current to undoStack, returns restored items + description
- `CanUndo() bool` / `CanRedo() bool` — stack depth checks
- `Changes(current []MergedItem) []Change` — compares baseline vs current
- `HasChanges(current []MergedItem) bool` — quick check

### Deep copy

A `deepCopyItems(items []MergedItem) []MergedItem` helper clones the slice and each item's `Data` map (shallow copy of map values is sufficient since YAML values are strings, bools, ints, or `[]any` of strings).

### Change detection

```go
type ChangeKind int

const (
    ChangeAdded ChangeKind = iota
    ChangeEdited
    ChangeDeleted
)

type FieldDiff struct {
    Key      string
    OldValue string
    NewValue string
}

type Change struct {
    Kind   ChangeKind
    ItemID string      // id or name of the entry
    Fields []FieldDiff // populated only for ChangeEdited
}
```

**Algorithm in `Changes()`:**

1. Build a map of baseline items by `itemKey()` (reuses existing function from `merge.go`)
2. Walk current items: if key exists in baseline map, compare fields; if any differ, emit `ChangeEdited` with `FieldDiff` entries. If key not in baseline, emit `ChangeAdded`
3. Remaining baseline keys not seen in current → emit `ChangeDeleted`
4. Items without an id/name key are matched by positional index and labeled as "item #N"

### Diff confirmation view

A Bubbletea model `diffModel` rendered when the user presses `q`:

```
  Review Changes (3 modifications)

  ~ SAP TechEd (edited)
    scope: regional → global
    tags:  +developer

  + SAP Inside Track (new)

  - Devtoberfest (deleted)

  Enter save  Esc back to list  d discard all
```

**Styling (Fiori palette):**

- `~` prefix and edited entry names in orange (`#F58B00`)
- `+` prefix and added entry names in green (`#00D68F`)
- `-` prefix and deleted entry names in red (`#FF5C5C`)
- Field diffs indented under edited entries, old value in muted (`#8C9BAA`), arrow, new value in text color (`#EDEDED`)
- Scrollable via up/down if changes exceed terminal height

**Keybindings:**

- `Enter` — confirm and save
- `Esc` — back to list (continue editing)
- `d` — discard all changes and quit without saving

**Edge cases:**

- No changes → skip diff view entirely, print "No changes." and exit
- Undo all changes back to baseline → `HasChanges()` returns false → clean exit

## Changes to existing files

### `internal/editor/editor.go`

Modify `runArrayEditor()`:

1. After `LoadMergedItems`, create `history := NewHistory(items)`
2. Pass `history` pointer into each `newListModel()` call
3. Before each mutation (edit/add/delete), call `history.Push(items, desc)`
4. When `result.save` is true: check `history.HasChanges(items)`. If true, run `diffModel`. If user confirms, save. If user cancels, continue loop. If no changes, exit.
5. On undo/redo results from the list model, replace `items` with the returned snapshot

### `internal/editor/list.go`

Modify `listModel`:

1. Add `history *History` field
2. Add `statusMsg string` field for action feedback
3. Add `undone bool` and `redone bool` result fields (signals back to the loop)
4. New keybindings in `updateNormal()`:
   - `u` — if `history.CanUndo()`, set `undone = true`, quit to loop
   - `r` — if `history.CanRedo()`, set `redone = true`, quit to loop
5. Update footer: `↑/↓ navigate  Enter edit  a add  d delete  u undo  r redo  / filter  q save  Esc quit`
6. Add status line in `View()` below the header showing `statusMsg` and undo stack depth

### `internal/theme/fiori.go`

Add three lipgloss v1 style functions for diff view:

- `DiffAdded() Style` — green foreground (`#00D68F`)
- `DiffEdited() Style` — orange foreground (`#F58B00`)
- `DiffDeleted() Style` — red foreground (`#FF5C5C`)

## What this does NOT include

- Field-level undo inside huh forms
- Per-entry layer diff (comparing user override vs official version)
- Git commit/push integration (dropped from Phase 2)
- Object editor undo (single-form, not applicable)

## Testing

- `history_test.go` — unit tests for Push/Undo/Redo/Changes/HasChanges with various scenarios (edit, add, delete, undo-all, redo-after-undo, push-clears-redo)
- `go build ./...` and `go vet ./...` locally (Windows Defender blocks `go test`)
- CI runs full test suite on ubuntu-latest
