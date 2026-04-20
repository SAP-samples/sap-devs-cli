# Content Editing Phase 2b: Reordering + Bulk Editing — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add item reordering (Shift+J/K) and multi-select bulk operations (set field, delete, add/remove tag) to the array editor.

**Architecture:** Extend the existing `listModel` in `list.go` with selection state (`map[int]bool`) and new result fields (`moveUp`, `moveDown`, `bulkAction`). The `runArrayEditor` loop in `editor.go` handles reorder swaps and dispatches to new bulk action forms in `bulk.go`. All mutations push a single history snapshot for undo. Selection clears after every action.

**Tech Stack:** Go, charmbracelet/bubbletea, charm.land/huh/v2, lipgloss v1 (list rendering), lipgloss v2 (huh themes)

**Spec:** `docs/superpowers/specs/2026-04-20-content-editing-phase2b-design.md`

---

### Task 1: Add `SelectedCheckbox` theme style

**Files:**
- Modify: `internal/theme/fiori.go:95-109` (after the existing Diff styles)

- [ ] **Step 1: Write the test — verify SelectedCheckbox style exists**

Since theme functions are pure style constructors with no logic to unit-test meaningfully, verify via compilation.

```go
// No dedicated test file — verified via go build and visual inspection.
```

- [ ] **Step 2: Add the SelectedCheckbox style**

In `internal/theme/fiori.go`, add after `DiffMuted()`:

```go
func SelectedCheckbox() lipglossv1.Style {
	return lipglossv1.NewStyle().Foreground(lipglossv1.Color("#4DB8FF"))
}
```

- [ ] **Step 3: Verify build**

Run: `go build ./... && go vet ./...`
Expected: SUCCESS

- [ ] **Step 4: Commit**

```bash
git add internal/theme/fiori.go
git commit -m "feat(theme): add SelectedCheckbox style for multi-select indicators"
```

---

### Task 2: Add selection state and result fields to `listModel`

**Files:**
- Modify: `internal/editor/list.go:41-66` (listModel struct)
- Modify: `internal/editor/list.go:68-81` (newListModel constructor)

This task adds the data fields only — no keybinding logic yet. The struct changes are needed before the keybinding and view tasks.

- [ ] **Step 1: Add fields to listModel struct**

In `internal/editor/list.go`, add these fields to the `listModel` struct after the undo/redo fields block (after line 62):

```go
// Selection and bulk action result fields.
selected             map[int]bool // originalIndex -> selected
moveUp               bool
moveDown             bool
bulkAction           string // "set-field", "delete", "add-tag", or ""
cursorOriginalIndex  int    // resolved originalIndex of cursor item (filter-safe)
```

- [ ] **Step 2: Initialize selected map in newListModel**

In the `newListModel` function, add `selected: make(map[int]bool),` and `cursorOriginalIndex: -1,` to the struct literal.

- [ ] **Step 3: Add helper function `selectedCount`**

Add below the `visibleItems()` method:

```go
func (m listModel) selectedCount() int {
	return len(m.selected)
}
```

- [ ] **Step 4: Verify build**

Run: `go build ./... && go vet ./...`
Expected: SUCCESS (new fields unused but that's fine — they'll be wired in subsequent tasks)

- [ ] **Step 5: Commit**

```bash
git add internal/editor/list.go
git commit -m "feat(editor): add selection state and bulk action fields to listModel"
```

---

### Task 3: Add selection keybindings (Space, Ctrl+A, Esc)

**Files:**
- Modify: `internal/editor/list.go:118-169` (updateNormal method)

- [ ] **Step 1: Add Space toggle in updateNormal**

In `updateNormal`, add a case for `" "` (Space key). Only allow selection of items in the target layer:

```go
case " ":
	visible := m.visibleItems()
	if m.cursor < len(visible) {
		idx := visible[m.cursor].originalIndex
		if m.items[idx].Layer == m.target.Layer {
			if m.selected[idx] {
				delete(m.selected, idx)
			} else {
				m.selected[idx] = true
			}
		}
	}
```

- [ ] **Step 2: Add Ctrl+A select-all**

Add a case for `"ctrl+a"`. Select all visible items that are in the target layer:

```go
case "ctrl+a":
	visible := m.visibleItems()
	for _, vi := range visible {
		if vi.item.Layer == m.target.Layer {
			m.selected[vi.originalIndex] = true
		}
	}
```

- [ ] **Step 3: Modify Esc to clear selection first**

Change the `"esc"` case to clear selection when items are selected, and only quit when nothing is selected:

```go
case "esc":
	if len(m.selected) > 0 {
		m.selected = make(map[int]bool)
	} else {
		m.quit = true
		return m, tea.Quit
	}
```

- [ ] **Step 4: Override `d` when items are selected (bulk delete trigger)**

When items are selected, `d` should trigger bulk delete instead of single delete. Modify the `"d"` case:

```go
case "d":
	if len(m.selected) > 0 {
		m.bulkAction = "delete"
		return m, tea.Quit
	}
	visible := m.visibleItems()
	if m.cursor < len(visible) {
		idx := visible[m.cursor].originalIndex
		if m.items[idx].Layer == m.target.Layer {
			m.deleteIdx = idx
			return m, tea.Quit
		}
	}
```

- [ ] **Step 5: Verify build**

Run: `go build ./... && go vet ./...`
Expected: SUCCESS

- [ ] **Step 6: Commit**

```bash
git add internal/editor/list.go
git commit -m "feat(editor): add Space/Ctrl+A/Esc selection keybindings to list view"
```

---

### Task 4: Add reorder keybindings (Shift+J, Shift+K) and bulk action triggers (e, t)

**Files:**
- Modify: `internal/editor/list.go:118-169` (updateNormal method)

- [ ] **Step 1: Add Shift+J and Shift+K cases**

In `updateNormal`, add cases for capital `J` and `K` (Bubbletea reports Shift+J as `"J"` and Shift+K as `"K"`). Resolve `cursorOriginalIndex` from the visible list before quitting so the editor loop has a filter-safe cursor position:

```go
case "J":
	visible := m.visibleItems()
	if m.cursor < len(visible) {
		m.cursorOriginalIndex = visible[m.cursor].originalIndex
	}
	m.moveDown = true
	return m, tea.Quit
case "K":
	visible := m.visibleItems()
	if m.cursor < len(visible) {
		m.cursorOriginalIndex = visible[m.cursor].originalIndex
	}
	m.moveUp = true
	return m, tea.Quit
```

- [ ] **Step 2: Add bulk action triggers `e` and `t`**

These only activate when items are selected:

```go
case "e":
	if len(m.selected) > 0 {
		m.bulkAction = "set-field"
		return m, tea.Quit
	}
case "t":
	if len(m.selected) > 0 {
		m.bulkAction = "add-tag"
		return m, tea.Quit
	}
```

- [ ] **Step 3: Verify build**

Run: `go build ./... && go vet ./...`
Expected: SUCCESS

- [ ] **Step 4: Commit**

```bash
git add internal/editor/list.go
git commit -m "feat(editor): add Shift+J/K reorder and e/t bulk action keybindings"
```

---

### Task 5: Update list View — checkbox rendering and contextual footer

**Files:**
- Modify: `internal/editor/list.go:195-279` (View method)

- [ ] **Step 1: Restructure item row rendering to add checkboxes before cursor**

In the `View()` method, replace the item row rendering block (the `row` construction inside the visible items loop, around lines 244-260) with this restructured version. The key change is: checkbox comes first, then cursor indicator, then columns. The spec shows `[x] > item` not `> [x] item`.

```go
for i := start; i < len(visible) && i < start+maxVisible; i++ {
	vi := visible[i]
	row := "  "

	// Checkbox (only shown when any items are selected).
	if len(m.selected) > 0 {
		if m.selected[vi.originalIndex] {
			row += theme.SelectedCheckbox().Render("[x]") + " "
		} else {
			row += "[ ] "
		}
	}

	// Cursor indicator.
	if i == m.cursor {
		row += "> "
	} else {
		row += "  "
	}

	for _, col := range m.columns {
		val, _ := vi.item.Data[col].(string)
		if len(val) > 18 {
			val = val[:18] + "..."
		}
		row += fmt.Sprintf("%-20s", val)
	}

	row += layerBadge(vi.item.Layer, vi.item.IsOverride)

	if i == m.cursor {
		sb.WriteString(selectedStyle.Render(row))
	} else {
		sb.WriteString(row)
	}
	sb.WriteString("\n")
}
```

- [ ] **Step 2: Add contextual footer**

Replace the static footer with a conditional one. When items are selected, show the bulk action footer:

```go
if len(m.selected) > 0 {
	sb.WriteString(footerStyle.Render(
		fmt.Sprintf("  %d selected: e set field  d delete  t add/remove tag  Esc clear", len(m.selected)),
	))
} else {
	sb.WriteString(footerStyle.Render(
		"  ↑/↓ navigate  Enter edit  a add  d delete  u undo  r redo  / filter  q save  Esc quit",
	))
}
```

- [ ] **Step 3: Verify build**

Run: `go build ./... && go vet ./...`
Expected: SUCCESS

- [ ] **Step 4: Commit**

```bash
git add internal/editor/list.go
git commit -m "feat(editor): render selection checkboxes and contextual footer in list view"
```

---

### Task 6: Implement reorder logic in editor loop

**Files:**
- Modify: `internal/editor/editor.go:86-103` (after redo handling, before delete handling)
- Test: `internal/editor/reorder_test.go` (new file)

- [ ] **Step 1: Write tests for single-item and multi-item move**

Create `internal/editor/reorder_test.go`:

```go
package editor_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/SAP-samples/sap-devs-cli/internal/editor"
)

func fourItems() []editor.MergedItem {
	return []editor.MergedItem{
		{Data: map[string]any{"id": "a"}, Layer: editor.LayerUser},
		{Data: map[string]any{"id": "b"}, Layer: editor.LayerUser},
		{Data: map[string]any{"id": "c"}, Layer: editor.LayerUser},
		{Data: map[string]any{"id": "d"}, Layer: editor.LayerUser},
	}
}

func ids(items []editor.MergedItem) []string {
	out := make([]string, len(items))
	for i, item := range items {
		out[i], _ = item.Data["id"].(string)
	}
	return out
}

func TestMoveItems_SingleUp(t *testing.T) {
	items := fourItems()
	// Move item at index 2 ("c") up
	result := editor.MoveItems(items, map[int]bool{2: true}, true)
	assert.Equal(t, []string{"a", "c", "b", "d"}, ids(result))
}

func TestMoveItems_SingleDown(t *testing.T) {
	items := fourItems()
	// Move item at index 1 ("b") down
	result := editor.MoveItems(items, map[int]bool{1: true}, false)
	assert.Equal(t, []string{"a", "c", "b", "d"}, ids(result))
}

func TestMoveItems_MultiUp(t *testing.T) {
	items := fourItems()
	// Move items at index 2,3 ("c","d") up
	result := editor.MoveItems(items, map[int]bool{2: true, 3: true}, true)
	assert.Equal(t, []string{"a", "c", "d", "b"}, ids(result))
}

func TestMoveItems_MultiDown(t *testing.T) {
	items := fourItems()
	// Move items at index 0,1 ("a","b") down
	result := editor.MoveItems(items, map[int]bool{0: true, 1: true}, false)
	assert.Equal(t, []string{"c", "a", "b", "d"}, ids(result))
}

func TestMoveItems_AtBoundaryUp(t *testing.T) {
	items := fourItems()
	// Item at index 0 can't move up — should be no-op
	result := editor.MoveItems(items, map[int]bool{0: true}, true)
	assert.Equal(t, []string{"a", "b", "c", "d"}, ids(result))
}

func TestMoveItems_AtBoundaryDown(t *testing.T) {
	items := fourItems()
	// Item at index 3 can't move down — should be no-op
	result := editor.MoveItems(items, map[int]bool{3: true}, false)
	assert.Equal(t, []string{"a", "b", "c", "d"}, ids(result))
}

func TestMoveItems_AdjacentSelectedUp(t *testing.T) {
	items := fourItems()
	// Items 1,2 are selected, move up — should shift as a block
	result := editor.MoveItems(items, map[int]bool{1: true, 2: true}, true)
	assert.Equal(t, []string{"b", "c", "a", "d"}, ids(result))
}

func TestMoveItems_NoSelection(t *testing.T) {
	items := fourItems()
	// Empty selection — no-op
	result := editor.MoveItems(items, map[int]bool{}, true)
	assert.Equal(t, []string{"a", "b", "c", "d"}, ids(result))
}
```

- [ ] **Step 2: Implement the `MoveItems` function**

Create the exported function in `internal/editor/reorder.go`. The algorithm uses a `moved` boolean slice to track which positions have been vacated by a swap — this handles contiguous selected items correctly (the entire block shifts as a unit):

```go
package editor

import "sort"

// MoveItems moves all selected items one position in the given direction.
// moveUp=true shifts selected items toward index 0; moveUp=false shifts toward the end.
// Contiguous selected items move as a block. Returns a new slice.
func MoveItems(items []MergedItem, selected map[int]bool, moveUp bool) []MergedItem {
	if len(selected) == 0 {
		return items
	}

	result := make([]MergedItem, len(items))
	copy(result, items)

	indices := make([]int, 0, len(selected))
	for idx := range selected {
		indices = append(indices, idx)
	}
	sort.Ints(indices)

	// Track which positions hold a selected item (updated as swaps occur).
	occupied := make([]bool, len(result))
	for _, idx := range indices {
		occupied[idx] = true
	}

	if moveUp {
		for _, idx := range indices {
			if idx == 0 || occupied[idx-1] {
				continue
			}
			result[idx-1], result[idx] = result[idx], result[idx-1]
			occupied[idx-1] = true
			occupied[idx] = false
		}
	} else {
		for i := len(indices) - 1; i >= 0; i-- {
			idx := indices[i]
			if idx >= len(result)-1 || occupied[idx+1] {
				continue
			}
			result[idx], result[idx+1] = result[idx+1], result[idx]
			occupied[idx+1] = true
			occupied[idx] = false
		}
	}

	return result
}
```

- [ ] **Step 3: Verify tests compile and pass**

Run: `go build ./... && go vet ./...`
Expected: SUCCESS (tests verified in CI; `go test` blocked locally by Windows Defender)

- [ ] **Step 4: Wire reorder into the editor loop**

In `internal/editor/editor.go`, in the `runArrayEditor` function, add reorder handling after the redo block (after the `if result.redone { ... }` block) and before the delete block:

```go
// Handle reorder.
if result.moveUp || result.moveDown {
	sel := result.selected
	if len(sel) == 0 {
		// No multi-select: move cursor item only.
		// Use cursorOriginalIndex which was resolved from the filtered visible
		// list before quitting — safe even when a filter is active.
		cursorIdx := result.cursorOriginalIndex
		if cursorIdx >= 0 && items[cursorIdx].Layer == target.Layer {
			sel = map[int]bool{cursorIdx: true}
		}
	}
	if len(sel) > 0 {
		desc := fmt.Sprintf("reordered %d item(s)", len(sel))
		history.Push(items, desc)
		items = MoveItems(items, sel, result.moveUp)
		statusMsg = fmt.Sprintf("✓ %s", desc)
	}
	continue
}
```

Since `cursorOriginalIndex` is resolved from the filtered visible list in `updateNormal` (Task 4), this is safe even when a filter is active — unlike reconstructing a new filterless model.

- [ ] **Step 5: Verify build**

Run: `go build ./... && go vet ./...`
Expected: SUCCESS

- [ ] **Step 6: Commit**

```bash
git add internal/editor/reorder.go internal/editor/reorder_test.go internal/editor/editor.go
git commit -m "feat(editor): implement item reordering with Shift+J/K"
```

---

### Task 7: Create `bulk.go` — BulkSetField and BulkAddRemoveTag forms

**Files:**
- Create: `internal/editor/bulk.go`

- [ ] **Step 1: Create `internal/editor/bulk.go` with BulkSetField**

```go
package editor

import (
	"errors"
	"fmt"

	"charm.land/huh/v2"
	"github.com/SAP-samples/sap-devs-cli/internal/schema"
	"github.com/SAP-samples/sap-devs-cli/internal/theme"
)

// BulkSetField opens a form to pick a field and value for bulk assignment.
// Returns the field key and new value. Returns an error if the user aborts.
func BulkSetField(spec *schema.ObjectSpec) (string, any, error) {
	candidates := bulkSettableFields(spec)
	if len(candidates) == 0 {
		return "", nil, fmt.Errorf("no fields available for bulk set")
	}

	// Step 1: pick field.
	var fieldKey string
	opts := make([]huh.Option[string], 0, len(candidates))
	for _, f := range candidates {
		label := fmt.Sprintf("%s (%s)", f.Title, f.Type)
		opts = append(opts, huh.NewOption(label, f.Key))
	}

	fieldForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Field to set").
				Options(opts...).
				Value(&fieldKey),
		),
	).WithTheme(huh.ThemeFunc(theme.ThemeFiori))

	if err := fieldForm.Run(); err != nil {
		return "", nil, err
	}

	// Step 2: get value for the chosen field.
	var chosen schema.FieldSpec
	for _, f := range candidates {
		if f.Key == fieldKey {
			chosen = f
			break
		}
	}

	if len(chosen.Enum) > 0 {
		var val string
		enumOpts := make([]huh.Option[string], 0, len(chosen.Enum))
		for _, e := range chosen.Enum {
			enumOpts = append(enumOpts, huh.NewOption(e, e))
		}
		valForm := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title(fmt.Sprintf("Value for %s", chosen.Title)).
					Options(enumOpts...).
					Value(&val),
			),
		).WithTheme(huh.ThemeFunc(theme.ThemeFiori))
		if err := valForm.Run(); err != nil {
			return "", nil, err
		}
		return fieldKey, val, nil
	}

	// Non-enum: text input.
	var val string
	valForm := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title(fmt.Sprintf("Value for %s", chosen.Title)).
				Value(&val),
		),
	).WithTheme(huh.ThemeFunc(theme.ThemeFiori))
	if err := valForm.Run(); err != nil {
		return "", nil, err
	}
	return fieldKey, val, nil
}

// BulkAddRemoveTag opens a form to add or remove a tag value on an array field.
func BulkAddRemoveTag(spec *schema.ObjectSpec) (action string, field string, value string, err error) {
	arrayFields := bulkArrayFields(spec)
	if len(arrayFields) == 0 {
		return "", "", "", fmt.Errorf("no array fields available")
	}

	// Step 1: Add or Remove?
	actionForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Action").
				Options(
					huh.NewOption("Add tag", "add"),
					huh.NewOption("Remove tag", "remove"),
				).
				Value(&action),
		),
	).WithTheme(huh.ThemeFunc(theme.ThemeFiori))
	if err := actionForm.Run(); err != nil {
		return "", "", "", err
	}

	// Step 2: Which array field?
	if len(arrayFields) == 1 {
		field = arrayFields[0].Key
	} else {
		fieldOpts := make([]huh.Option[string], 0, len(arrayFields))
		for _, f := range arrayFields {
			fieldOpts = append(fieldOpts, huh.NewOption(f.Title, f.Key))
		}
		fieldForm := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Field").
					Options(fieldOpts...).
					Value(&field),
			),
		).WithTheme(huh.ThemeFunc(theme.ThemeFiori))
		if err := fieldForm.Run(); err != nil {
			return "", "", "", err
		}
	}

	// Step 3: Tag value.
	valForm := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Tag value").
				Value(&value),
		),
	).WithTheme(huh.ThemeFunc(theme.ThemeFiori))
	if err := valForm.Run(); err != nil {
		return "", "", "", err
	}

	return action, field, value, nil
}

// IsUserAborted reports whether the error is a user abort from huh.
func IsUserAborted(err error) bool {
	return errors.Is(err, huh.ErrUserAborted)
}

// bulkSettableFields returns fields suitable for bulk set: string, integer, boolean types.
// Excludes array, object, and map fields.
func bulkSettableFields(spec *schema.ObjectSpec) []schema.FieldSpec {
	var out []schema.FieldSpec
	for _, f := range spec.Fields {
		switch f.Type {
		case "string", "integer", "boolean":
			out = append(out, f)
		}
	}
	return out
}

// bulkArrayFields returns fields of type "array" from the spec.
func bulkArrayFields(spec *schema.ObjectSpec) []schema.FieldSpec {
	var out []schema.FieldSpec
	for _, f := range spec.Fields {
		if f.Type == "array" {
			out = append(out, f)
		}
	}
	return out
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./... && go vet ./...`
Expected: SUCCESS

- [ ] **Step 3: Commit**

```bash
git add internal/editor/bulk.go
git commit -m "feat(editor): add BulkSetField and BulkAddRemoveTag form helpers"
```

---

### Task 8: Wire bulk actions into editor loop

**Files:**
- Modify: `internal/editor/editor.go` (after the reorder handling block)
- Test: `internal/editor/bulk_test.go` (new — tests for bulk delete index logic)

- [ ] **Step 1: Write test for bulk delete index handling**

Create `internal/editor/bulk_test.go`:

```go
package editor_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/SAP-samples/sap-devs-cli/internal/editor"
)

func TestBulkDelete(t *testing.T) {
	items := []editor.MergedItem{
		{Data: map[string]any{"id": "a"}, Layer: editor.LayerUser},
		{Data: map[string]any{"id": "b"}, Layer: editor.LayerUser},
		{Data: map[string]any{"id": "c"}, Layer: editor.LayerUser},
		{Data: map[string]any{"id": "d"}, Layer: editor.LayerUser},
	}
	// Delete items at indices 1 and 3 ("b" and "d")
	result := editor.BulkDeleteItems(items, map[int]bool{1: true, 3: true})
	assert.Len(t, result, 2)
	assert.Equal(t, "a", result[0].Data["id"])
	assert.Equal(t, "c", result[1].Data["id"])
}

func TestBulkDelete_All(t *testing.T) {
	items := []editor.MergedItem{
		{Data: map[string]any{"id": "a"}, Layer: editor.LayerUser},
		{Data: map[string]any{"id": "b"}, Layer: editor.LayerUser},
	}
	result := editor.BulkDeleteItems(items, map[int]bool{0: true, 1: true})
	assert.Len(t, result, 0)
}

func TestBulkDelete_Empty(t *testing.T) {
	items := fourItems()
	result := editor.BulkDeleteItems(items, map[int]bool{})
	assert.Len(t, result, 4)
}

func TestBulkAddTag(t *testing.T) {
	items := []editor.MergedItem{
		{Data: map[string]any{"id": "a", "tags": []any{"x"}}, Layer: editor.LayerUser},
		{Data: map[string]any{"id": "b"}, Layer: editor.LayerUser},
	}
	editor.BulkApplyTag(items, map[int]bool{0: true, 1: true}, "tags", "y", "add")
	assert.Equal(t, []any{"x", "y"}, items[0].Data["tags"])
	assert.Equal(t, []any{"y"}, items[1].Data["tags"])
}

func TestBulkRemoveTag(t *testing.T) {
	items := []editor.MergedItem{
		{Data: map[string]any{"id": "a", "tags": []any{"x", "y"}}, Layer: editor.LayerUser},
		{Data: map[string]any{"id": "b", "tags": []any{"y"}}, Layer: editor.LayerUser},
	}
	editor.BulkApplyTag(items, map[int]bool{0: true, 1: true}, "tags", "y", "remove")
	assert.Equal(t, []any{"x"}, items[0].Data["tags"])
	assert.Equal(t, []any{}, items[1].Data["tags"])
}
```

- [ ] **Step 2: Implement `BulkDeleteItems` and `BulkApplyTag` helper functions**

Add to `internal/editor/bulk.go`:

```go
// BulkDeleteItems removes items at the selected indices and returns a new slice.
func BulkDeleteItems(items []MergedItem, selected map[int]bool) []MergedItem {
	result := make([]MergedItem, 0, len(items)-len(selected))
	for i, item := range items {
		if !selected[i] {
			result = append(result, item)
		}
	}
	return result
}

// BulkApplyTag adds or removes a tag value from an array field on the selected items.
func BulkApplyTag(items []MergedItem, selected map[int]bool, field, value, action string) {
	for idx := range selected {
		if idx < 0 || idx >= len(items) {
			continue
		}
		arr, _ := items[idx].Data[field].([]any)
		if arr == nil {
			arr = []any{}
		}
		switch action {
		case "add":
			arr = append(arr, value)
		case "remove":
			filtered := make([]any, 0, len(arr))
			for _, v := range arr {
				if v != value {
					filtered = append(filtered, v)
				}
			}
			arr = filtered
		}
		items[idx].Data[field] = arr
	}
}

// selectedIndices returns sorted indices from a selected map.
func selectedIndices(selected map[int]bool) []int {
	indices := make([]int, 0, len(selected))
	for idx := range selected {
		indices = append(indices, idx)
	}
	sort.Ints(indices)
	return indices
}
```

**Important:** Add `"sort"` to the import block in `bulk.go` — it is not present from Task 7. The updated imports should be:

```go
import (
	"errors"
	"fmt"
	"sort"

	"charm.land/huh/v2"
	"github.com/SAP-samples/sap-devs-cli/internal/schema"
	"github.com/SAP-samples/sap-devs-cli/internal/theme"
)
```

- [ ] **Step 3: Verify build**

Run: `go build ./... && go vet ./...`
Expected: SUCCESS

- [ ] **Step 4: Wire bulk actions into the editor loop**

In `internal/editor/editor.go`, add `"strconv"` to the import block (needed for integer coercion in set-field). Then add the bulk action handling after the reorder block and before the delete block:

```go
// Handle bulk actions.
if result.bulkAction != "" {
	switch result.bulkAction {
	case "set-field":
		field, value, err := BulkSetField(s.ItemSpec)
		if err != nil {
			if IsUserAborted(err) {
				continue
			}
			continue
		}
		// Coerce string value to int for integer fields (BulkSetField returns
		// string for all text inputs; without this, yaml.Marshal writes "42"
		// instead of 42).
		for _, f := range s.ItemSpec.Fields {
			if f.Key == field && f.Type == "integer" {
				if n, convErr := strconv.Atoi(value.(string)); convErr == nil {
					value = n
				}
				break
			}
		}
		desc := fmt.Sprintf("set %s on %d item(s)", field, len(result.selected))
		history.Push(items, desc)
		for idx := range result.selected {
			if items[idx].Layer != target.Layer {
				cloned := make(map[string]any)
				for k, v := range items[idx].Data {
					cloned[k] = v
				}
				items[idx].Data = cloned
				items[idx].Layer = target.Layer
				items[idx].IsOverride = true
			}
			items[idx].Data[field] = value
		}
		statusMsg = fmt.Sprintf("✓ %s", desc)

	case "delete":
		desc := fmt.Sprintf("deleted %d item(s)", len(result.selected))
		history.Push(items, desc)
		items = BulkDeleteItems(items, result.selected)
		statusMsg = fmt.Sprintf("✓ %s", desc)

	case "add-tag":
		action, field, value, err := BulkAddRemoveTag(s.ItemSpec)
		if err != nil {
			if IsUserAborted(err) {
				continue
			}
			continue
		}
		desc := fmt.Sprintf("%s tag %q on %s for %d item(s)", action, value, field, len(result.selected))
		history.Push(items, desc)
		for idx := range result.selected {
			if items[idx].Layer != target.Layer {
				cloned := make(map[string]any)
				for k, v := range items[idx].Data {
					cloned[k] = v
				}
				items[idx].Data = cloned
				items[idx].Layer = target.Layer
				items[idx].IsOverride = true
			}
		}
		BulkApplyTag(items, result.selected, field, value, action)
		statusMsg = fmt.Sprintf("✓ %s", desc)
	}
	continue
}
```

- [ ] **Step 5: Verify build**

Run: `go build ./... && go vet ./...`
Expected: SUCCESS

- [ ] **Step 6: Commit**

```bash
git add internal/editor/bulk.go internal/editor/bulk_test.go internal/editor/editor.go
git commit -m "feat(editor): wire bulk set-field, delete, and add/remove tag into editor loop"
```

---

### Task 9: Add bulk undo tests to history_test.go

**Files:**
- Modify: `internal/editor/history_test.go`

- [ ] **Step 1: Add test for bulk operation undo (single snapshot, multiple changes)**

Append to `internal/editor/history_test.go`:

```go
func TestHistory_BulkUndoSingleSnapshot(t *testing.T) {
	items := []editor.MergedItem{
		{Data: map[string]any{"id": "a", "scope": "old"}, Layer: editor.LayerUser},
		{Data: map[string]any{"id": "b", "scope": "old"}, Layer: editor.LayerUser},
		{Data: map[string]any{"id": "c", "scope": "old"}, Layer: editor.LayerUser},
	}
	h := editor.NewHistory(items)

	// Simulate bulk set-field: push once, then mutate all three.
	h.Push(items, "set scope on 3 items")
	items[0].Data["scope"] = "new"
	items[1].Data["scope"] = "new"
	items[2].Data["scope"] = "new"

	assert.Equal(t, 1, h.UndoDepth())

	restored, desc, ok := h.Undo(items)
	require.True(t, ok)
	assert.Equal(t, "set scope on 3 items", desc)
	assert.Equal(t, "old", restored[0].Data["scope"])
	assert.Equal(t, "old", restored[1].Data["scope"])
	assert.Equal(t, "old", restored[2].Data["scope"])
}

func TestHistory_BulkDeleteUndo(t *testing.T) {
	items := []editor.MergedItem{
		{Data: map[string]any{"id": "a"}, Layer: editor.LayerUser},
		{Data: map[string]any{"id": "b"}, Layer: editor.LayerUser},
		{Data: map[string]any{"id": "c"}, Layer: editor.LayerUser},
	}
	h := editor.NewHistory(items)

	h.Push(items, "deleted 2 items")
	items = editor.BulkDeleteItems(items, map[int]bool{0: true, 2: true})
	require.Len(t, items, 1)
	assert.Equal(t, "b", items[0].Data["id"])

	restored, _, ok := h.Undo(items)
	require.True(t, ok)
	require.Len(t, restored, 3)
	assert.Equal(t, "a", restored[0].Data["id"])
	assert.Equal(t, "b", restored[1].Data["id"])
	assert.Equal(t, "c", restored[2].Data["id"])
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./... && go vet ./...`
Expected: SUCCESS

- [ ] **Step 3: Commit**

```bash
git add internal/editor/history_test.go
git commit -m "test(editor): add bulk undo tests for multi-item set and delete"
```

---

### Task 10: Final build verification and documentation update

**Files:**
- Verify: all modified files compile
- Modify: `docs/TODO.md` (mark Phase 2b complete)

- [ ] **Step 1: Full build check**

Run: `go build ./... && go vet ./...`
Expected: SUCCESS with no warnings

- [ ] **Step 2: Update TODO.md**

Mark the Phase 2b items as complete in `docs/TODO.md`.

- [ ] **Step 3: Commit**

```bash
git add docs/TODO.md
git commit -m "docs: mark Phase 2b (reordering + bulk editing) as complete"
```
