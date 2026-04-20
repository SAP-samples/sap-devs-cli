# Content Editing UI — Phase 2b: Reordering + Bulk Editing

## Context

Phase 1 shipped an interactive terminal-based YAML editor (`sap-devs content edit`) with a schema-driven form view, list view with layer badges, and multi-layer merge/save logic. Phase 2a added undo/redo and a pre-save diff confirmation screen. Phase 2b adds two features that share the same selection infrastructure:

- **Reordering:** Move items up/down in the list with `Shift+J`/`Shift+K`
- **Bulk editing:** Multi-select items, then apply an action (set field, delete, add/remove tag)

## Goal

Add item reordering and multi-select bulk operations to the array editor, enabling users to curate content order and make batch changes efficiently.

## Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Selection UI | In-list multi-select | Single view, no mode switch. Selection state always visible. Familiar file-manager pattern. Shared between reorder and bulk edit. |
| Reorder keybindings | Shift+J / Shift+K | Direct, no mode entry. Standard vim-with-modifier pattern. |
| Selection toggle | Space | Standard toggle key in TUI lists. |
| Select all | Ctrl+A | Common shortcut. Respects active filter. |
| Clear selection | Esc (when items selected) | Esc quits only when nothing is selected. |
| Bulk actions | Set field, delete, add/remove tag | All four requested; set-enum-from-schema handled via the set-field flow using schema metadata. |
| Layer constraint | Target layer only | Cannot select, reorder, or bulk-edit inherited items. |
| Multi-item reorder | Move all selected items | Each selected item shifts one position in the move direction, preserving relative order. |
| Undo granularity | One snapshot per bulk operation | Single undo reverts entire bulk action (e.g. "set scope on 3 items"). |
| Diff view for reorder | No change detection for position-only changes | Key-based comparison is unaffected by order. Order is editorial, not data. |
| Scope | Array editor only | Consistent with Phase 2a — object editor is a single-form flow. |

## Architecture

### Modified file: `internal/editor/list.go`

The `listModel` struct gains selection state and new result fields.

**New fields on `listModel`:**

```go
selected  map[int]bool // originalIndex -> selected
moveUp    bool         // Shift+K pressed
moveDown  bool         // Shift+J pressed
bulkAction string      // "set-field", "delete", "add-tag", "" (none)
```

**Selection mechanics:**

- `Space` toggles `selected[originalIndex]`. Only items in `target.Layer` can be selected.
- `Ctrl+A` selects all visible items (respects filter) that are in the target layer.
- `Esc` clears selection when `len(selected) > 0`; otherwise quits as before.

**Reorder keybindings:**

- `Shift+J` (capital `J`): sets `moveDown = true`, quits to loop.
- `Shift+K` (capital `K`): sets `moveUp = true`, quits to loop.

**Contextual footer:**

When `len(selected) > 0`, footer switches to:
```
N selected: e set field  d delete  t add/remove tag  Esc clear
```

When no items selected, footer is the existing:
```
↑/↓ navigate  Enter edit  a add  d delete  u undo  r redo  / filter  q save  Esc quit
```

**Bulk action triggers:**

When items are selected:
- `e` → sets `bulkAction = "set-field"`, quits to loop.
- `d` → sets `bulkAction = "delete"`, quits to loop.
- `t` → sets `bulkAction = "add-tag"`, quits to loop.

**View changes:**

- Each item row renders `[x]` or `[ ]` before the cursor indicator when any items are selected.
- Selected items rendered with a subtle highlight (muted foreground or background tint).

### New file: `internal/editor/bulk.go`

Contains the bulk action form logic, separate from list.go (navigation/selection) and editor.go (loop orchestration).

**Functions:**

```go
// BulkSetField opens a form to pick a field and value, returns the field key and new value.
func BulkSetField(spec *schema.ObjectSpec) (string, any, error)

// BulkAddRemoveTag opens a form to add or remove a tag value on an array field.
func BulkAddRemoveTag(spec *schema.ObjectSpec) (action string, field string, value string, err error)
```

**`BulkSetField` flow:**

1. Build a huh select dropdown of string and enum schema fields (key + type label). Exclude array, object, and map fields — those are not meaningful for bulk set. Boolean and integer fields use a text input; the raw string value is stored as-is and validated by schema validation downstream.
2. User picks a field.
3. If the field is an enum (has `Enum` values in schema), show a select dropdown of valid options.
4. If the field is a string/URI/etc., show a text input.
5. Return the field key and typed value.

**`BulkAddRemoveTag` flow:**

1. Build a huh select: "Add" or "Remove".
2. Build a select of array-type fields from the schema (e.g., `tags`).
3. Text input for the tag value.
4. Return action ("add"/"remove"), field key, and tag value.

### Modified file: `internal/editor/editor.go`

The `runArrayEditor` loop gains handlers for the new result fields.

**Reorder handling:**

```go
if result.moveUp || result.moveDown {
    // Collect indices to move (selected items, or just cursor item if none selected).
    // Validate all are in target layer.
    // Push history snapshot.
    // Perform swap(s) in the items slice.
    // Set statusMsg.
    continue
}
```

**Single-item move:** Swap `items[idx]` with `items[idx-1]` (move up) or `items[idx+1]` (move down). Bounds-checked.

**Multi-item move:** Sort selected indices. For move-up, iterate ascending: each selected index swaps with its predecessor if the predecessor is not also selected. For move-down, iterate descending: each selected index swaps with its successor if the successor is not also selected. This preserves relative order and handles adjacent selected items correctly.

**Bulk action handling:**

```go
if result.bulkAction != "" {
    indices := selectedIndices(result.selected)
    switch result.bulkAction {
    case "set-field":
        field, value, err := BulkSetField(s.ItemSpec)
        // if err (user abort), continue
        history.Push(items, desc)
        for _, idx := range indices {
            items[idx].Data[field] = value
            // Clone to target layer if inherited: copy Data map, set Layer = target.Layer,
            // IsOverride = true — same pattern as the single-item edit path in editor.go.
        }
    case "delete":
        history.Push(items, desc)
        // delete in reverse index order to avoid shifting
        for i := len(indices) - 1; i >= 0; i-- {
            items = append(items[:indices[i]], items[indices[i]+1:]...)
        }
    case "add-tag":
        action, field, value, err := BulkAddRemoveTag(s.ItemSpec)
        // if err, continue
        history.Push(items, desc)
        for _, idx := range indices {
            // add or remove value from items[idx].Data[field] ([]any slice)
        }
    }
    statusMsg = desc
    continue
}
```

**Post-action behavior:** Selection is cleared after every bulk action (set-field, delete, add-tag) and after every reorder. The user starts fresh for the next operation. This avoids accidental double-application.

### Modified file: `internal/theme/fiori.go`

Add one new style for selected items:

```go
func SelectedCheckbox() lipglossv1.Style {
    return lipglossv1.NewStyle().Foreground(lipglossv1.Color("#4DB8FF"))
}
```

## What this does NOT include

- Drag-and-drop with mouse (terminal has no mouse drag support in this architecture)
- Reorder across layers (items can only be reordered within the target layer)
- Bulk edit of inherited items (would need to clone each to target layer first — out of scope)
- Column sorting (sort by name, id, etc.) — could be a future enhancement
- Regex-based bulk find-and-replace

## Testing

- `bulk_test.go` — unit tests for multi-item move logic (swap algorithm), bulk delete index handling
- `history_test.go` — additional tests for bulk undo (push once, multiple items change, single undo restores all)
- `go build ./...` and `go vet ./...` locally (Windows Defender blocks `go test`)
- CI runs full test suite on ubuntu-latest
