# Content Editing UI Phase 2a: Undo/Redo + Pre-Save Diff View — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an undo/redo snapshot stack and a pre-save change confirmation screen to the array content editor so users can revert mistakes and review all changes before writing to disk.

**Architecture:** A single new file `internal/editor/history.go` holds the snapshot stack (`History`), change detection (`Changes()`), and a Bubbletea diff confirmation model (`diffModel`). The existing `runArrayEditor` loop in `editor.go` is modified to push snapshots before mutations and show the diff view on save. The list model in `list.go` gains `u`/`r` keybindings that signal undo/redo back to the loop.

**Tech Stack:** Go, charmbracelet/bubbletea, lipgloss v1 (github.com/charmbracelet/lipgloss), huh v2 (charm.land/huh/v2)

**Spec:** `docs/superpowers/specs/2026-04-19-content-editing-phase2a-design.md`

---

## File Structure

| File | Action | Responsibility |
|------|--------|----------------|
| `internal/editor/history.go` | Create | Snapshot stack (`History`), deep copy, change detection (`Changes`), diff Bubbletea model (`diffModel`) |
| `internal/editor/history_test.go` | Create | Unit tests for History push/undo/redo/changes |
| `internal/editor/editor.go` | Modify | Wire History into `runArrayEditor` loop; show diff view on save |
| `internal/editor/list.go` | Modify | Add `u`/`r` keybindings, `history` field, status line, undo/redo result flags |
| `internal/theme/fiori.go` | Modify | Add `DiffAdded()`, `DiffEdited()`, `DiffDeleted()` lipgloss v1 style functions |

---

### Task 1: Deep Copy Helper + Snapshot Stack

**Files:**
- Create: `internal/editor/history.go`
- Create: `internal/editor/history_test.go`

This task implements the core `History` struct with `Push`, `Undo`, `Redo`, `CanUndo`, `CanRedo`, and the `deepCopyItems` helper. No UI changes yet.

- [ ] **Step 1: Write failing tests for History**

Create `internal/editor/history_test.go` with the following tests. Use the `_test` external package pattern (`package editor_test`) and `testify/assert` + `testify/require`, matching the project's conventions (see `internal/schema/schema_test.go`).

```go
package editor_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/SAP-samples/sap-devs-cli/internal/editor"
)

func sampleItems() []editor.MergedItem {
	return []editor.MergedItem{
		{Data: map[string]any{"id": "a", "title": "Alpha"}, Layer: editor.LayerOfficial},
		{Data: map[string]any{"id": "b", "title": "Beta"}, Layer: editor.LayerOfficial},
	}
}

func TestHistory_NewHistory(t *testing.T) {
	items := sampleItems()
	h := editor.NewHistory(items)
	assert.False(t, h.CanUndo())
	assert.False(t, h.CanRedo())
}

func TestHistory_PushAndUndo(t *testing.T) {
	items := sampleItems()
	h := editor.NewHistory(items)

	// Simulate editing item "a".
	items[0].Data["title"] = "Alpha Edited"
	h.Push(items, "edited Alpha")

	assert.True(t, h.CanUndo())

	restored, desc, ok := h.Undo(items)
	require.True(t, ok)
	assert.Equal(t, "edited Alpha", desc)
	assert.Equal(t, "Alpha", restored[0].Data["title"])
}

func TestHistory_Redo(t *testing.T) {
	items := sampleItems()
	h := editor.NewHistory(items)

	items[0].Data["title"] = "Alpha Edited"
	h.Push(items, "edited Alpha")

	restored, _, _ := h.Undo(items)
	assert.True(t, h.CanRedo())

	redone, desc, ok := h.Redo(restored)
	require.True(t, ok)
	assert.Equal(t, "edited Alpha", desc)
	assert.Equal(t, "Alpha Edited", redone[0].Data["title"])
}

func TestHistory_PushClearsRedo(t *testing.T) {
	items := sampleItems()
	h := editor.NewHistory(items)

	items[0].Data["title"] = "Edit 1"
	h.Push(items, "edit 1")

	restored, _, _ := h.Undo(items)
	assert.True(t, h.CanRedo())

	// New push after undo should clear the redo stack.
	restored[1].Data["title"] = "Beta Edited"
	h.Push(restored, "edit beta")
	assert.False(t, h.CanRedo())
}

func TestHistory_UndoAll(t *testing.T) {
	items := sampleItems()
	h := editor.NewHistory(items)

	items[0].Data["title"] = "Edit 1"
	h.Push(items, "edit 1")
	items[0].Data["title"] = "Edit 2"
	h.Push(items, "edit 2")
	items[0].Data["title"] = "Edit 3"
	h.Push(items, "edit 3")

	// Undo all three.
	current := items
	for h.CanUndo() {
		var ok bool
		current, _, ok = h.Undo(current)
		require.True(t, ok)
	}
	assert.Equal(t, "Alpha", current[0].Data["title"])
	assert.False(t, h.CanUndo())
}

func TestHistory_DeepCopyIsolation(t *testing.T) {
	items := sampleItems()
	h := editor.NewHistory(items)

	items[0].Data["title"] = "Mutated"
	h.Push(items, "mutated")

	// Undo should return the original, unaffected by the mutation.
	restored, _, _ := h.Undo(items)
	assert.Equal(t, "Alpha", restored[0].Data["title"])

	// Further mutation of restored should not affect the stack.
	restored[0].Data["title"] = "Mutated Again"
	redone, _, _ := h.Redo(restored)
	assert.Equal(t, "Mutated", redone[0].Data["title"])
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go build ./internal/editor/... 2>&1` (will fail — `NewHistory` etc. don't exist yet)

Expected: compilation error referencing undefined `NewHistory`, `History`

- [ ] **Step 3: Implement History struct and operations**

Create `internal/editor/history.go`:

```go
package editor

// Snapshot captures the full item list and a human-readable description.
type Snapshot struct {
	Items []MergedItem
	Desc  string
}

// History tracks undo/redo state for the array editor.
type History struct {
	baseline  []MergedItem
	undoStack []Snapshot
	redoStack []Snapshot
}

// NewHistory creates a History with the given items as the immutable baseline.
func NewHistory(items []MergedItem) *History {
	return &History{
		baseline: deepCopyItems(items),
	}
}

// Push records the current state before a mutation. Clears the redo stack.
func (h *History) Push(items []MergedItem, desc string) {
	h.undoStack = append(h.undoStack, Snapshot{
		Items: deepCopyItems(items),
		Desc:  desc,
	})
	h.redoStack = nil
}

// Undo restores the previous state. current is the live item list (pushed
// onto the redo stack). Returns the restored items, the description of the
// undone action, and whether the undo succeeded.
func (h *History) Undo(current []MergedItem) ([]MergedItem, string, bool) {
	if len(h.undoStack) == 0 {
		return nil, "", false
	}
	top := h.undoStack[len(h.undoStack)-1]
	h.undoStack = h.undoStack[:len(h.undoStack)-1]
	h.redoStack = append(h.redoStack, Snapshot{
		Items: deepCopyItems(current),
		Desc:  top.Desc,
	})
	return deepCopyItems(top.Items), top.Desc, true
}

// Redo restores a previously undone state. current is the live item list
// (pushed onto the undo stack).
func (h *History) Redo(current []MergedItem) ([]MergedItem, string, bool) {
	if len(h.redoStack) == 0 {
		return nil, "", false
	}
	top := h.redoStack[len(h.redoStack)-1]
	h.redoStack = h.redoStack[:len(h.redoStack)-1]
	h.undoStack = append(h.undoStack, Snapshot{
		Items: deepCopyItems(current),
		Desc:  top.Desc,
	})
	return deepCopyItems(top.Items), top.Desc, true
}

// CanUndo reports whether the undo stack is non-empty.
func (h *History) CanUndo() bool { return len(h.undoStack) > 0 }

// CanRedo reports whether the redo stack is non-empty.
func (h *History) CanRedo() bool { return len(h.redoStack) > 0 }

// UndoDepth returns the number of operations that can be undone.
func (h *History) UndoDepth() int { return len(h.undoStack) }

// Baseline returns a deep copy of the original items.
func (h *History) Baseline() []MergedItem { return deepCopyItems(h.baseline) }

func deepCopyItems(items []MergedItem) []MergedItem {
	cp := make([]MergedItem, len(items))
	for i, item := range items {
		data := make(map[string]any, len(item.Data))
		for k, v := range item.Data {
			data[k] = v
		}
		cp[i] = MergedItem{
			Data:       data,
			Layer:      item.Layer,
			IsOverride: item.IsOverride,
		}
	}
	return cp
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/editor/... -v -run TestHistory`

Expected: all 6 tests PASS

- [ ] **Step 5: Verify build**

Run: `go build ./... && go vet ./...`

Expected: clean build, no vet warnings

- [ ] **Step 6: Commit**

```bash
git add internal/editor/history.go internal/editor/history_test.go
git commit -m "feat(editor): add undo/redo snapshot stack with History struct"
```

---

### Task 2: Change Detection

**Files:**
- Modify: `internal/editor/history.go`
- Modify: `internal/editor/history_test.go`

Add `Changes()` and `HasChanges()` that compare the baseline against the current item list.

- [ ] **Step 1: Write failing tests for change detection**

Append to `internal/editor/history_test.go`:

```go
func TestHistory_HasChanges_NoChanges(t *testing.T) {
	items := sampleItems()
	h := editor.NewHistory(items)
	assert.False(t, h.HasChanges(items))
}

func TestHistory_HasChanges_AfterEdit(t *testing.T) {
	items := sampleItems()
	h := editor.NewHistory(items)
	items[0].Data["title"] = "Changed"
	assert.True(t, h.HasChanges(items))
}

func TestHistory_Changes_Edit(t *testing.T) {
	items := sampleItems()
	h := editor.NewHistory(items)
	items[0].Data["title"] = "Alpha Edited"
	changes := h.Changes(items)
	require.Len(t, changes, 1)
	assert.Equal(t, editor.ChangeEdited, changes[0].Kind)
	assert.Equal(t, "a", changes[0].ItemID)
	require.Len(t, changes[0].Fields, 1)
	assert.Equal(t, "title", changes[0].Fields[0].Key)
	assert.Equal(t, "Alpha", changes[0].Fields[0].OldValue)
	assert.Equal(t, "Alpha Edited", changes[0].Fields[0].NewValue)
}

func TestHistory_Changes_Add(t *testing.T) {
	items := sampleItems()
	h := editor.NewHistory(items)
	items = append(items, editor.MergedItem{
		Data:  map[string]any{"id": "c", "title": "Gamma"},
		Layer: editor.LayerUser,
	})
	changes := h.Changes(items)
	require.Len(t, changes, 1)
	assert.Equal(t, editor.ChangeAdded, changes[0].Kind)
	assert.Equal(t, "c", changes[0].ItemID)
}

func TestHistory_Changes_Delete(t *testing.T) {
	items := sampleItems()
	h := editor.NewHistory(items)
	items = items[1:] // remove first item
	changes := h.Changes(items)
	require.Len(t, changes, 1)
	assert.Equal(t, editor.ChangeDeleted, changes[0].Kind)
	assert.Equal(t, "a", changes[0].ItemID)
}

func TestHistory_Changes_UndoAllNoChanges(t *testing.T) {
	items := sampleItems()
	h := editor.NewHistory(items)
	items[0].Data["title"] = "Changed"
	h.Push(items, "edit")
	restored, _, _ := h.Undo(items)
	assert.False(t, h.HasChanges(restored))
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go build ./internal/editor/... 2>&1`

Expected: compilation error — `Changes`, `HasChanges`, `ChangeEdited` etc. undefined

- [ ] **Step 3: Implement change detection**

Add to `internal/editor/history.go`:

```go
// ChangeKind categorises a difference between baseline and current.
type ChangeKind int

const (
	ChangeAdded ChangeKind = iota
	ChangeEdited
	ChangeDeleted
)

// FieldDiff describes a single field that changed between baseline and current.
type FieldDiff struct {
	Key      string
	OldValue string
	NewValue string
}

// Change describes one item-level difference.
type Change struct {
	Kind   ChangeKind
	ItemID string
	Fields []FieldDiff
}

// HasChanges reports whether the current items differ from the baseline.
func (h *History) HasChanges(current []MergedItem) bool {
	return len(h.Changes(current)) > 0
}

// Changes compares the baseline against current and returns all differences.
func (h *History) Changes(current []MergedItem) []Change {
	baseByKey := make(map[string]map[string]any)
	baseOrder := make([]string, 0)
	for i, item := range h.baseline {
		key := itemKey(item.Data)
		if key == "" {
			key = positionalKey(i)
		}
		baseByKey[key] = item.Data
		baseOrder = append(baseOrder, key)
	}

	var changes []Change
	currentKeys := make(map[string]bool)

	for i, item := range current {
		key := itemKey(item.Data)
		if key == "" {
			key = positionalKey(i)
		}
		currentKeys[key] = true

		baseData, existed := baseByKey[key]
		if !existed {
			changes = append(changes, Change{Kind: ChangeAdded, ItemID: key})
			continue
		}

		diffs := diffFields(baseData, item.Data)
		if len(diffs) > 0 {
			changes = append(changes, Change{Kind: ChangeEdited, ItemID: key, Fields: diffs})
		}
	}

	for _, key := range baseOrder {
		if !currentKeys[key] {
			changes = append(changes, Change{Kind: ChangeDeleted, ItemID: key})
		}
	}

	return changes
}

func positionalKey(i int) string {
	return fmt.Sprintf("item #%d", i+1)
}

func diffFields(old, new map[string]any) []FieldDiff {
	var diffs []FieldDiff
	allKeys := make(map[string]bool)
	for k := range old {
		allKeys[k] = true
	}
	for k := range new {
		allKeys[k] = true
	}
	for k := range allKeys {
		oldVal := fmt.Sprintf("%v", old[k])
		newVal := fmt.Sprintf("%v", new[k])
		if old[k] == nil {
			oldVal = ""
		}
		if new[k] == nil {
			newVal = ""
		}
		if oldVal != newVal {
			diffs = append(diffs, FieldDiff{Key: k, OldValue: oldVal, NewValue: newVal})
		}
	}
	return diffs
}
```

Note: `itemKey()` is already defined in `merge.go` and is package-private, so it's accessible from `history.go` within the same package.

Add `"fmt"` to the imports in `history.go` if not already present.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/editor/... -v -run "TestHistory_(HasChanges|Changes)"`

Expected: all 6 new tests PASS

- [ ] **Step 5: Verify build**

Run: `go build ./... && go vet ./...`

Expected: clean

- [ ] **Step 6: Commit**

```bash
git add internal/editor/history.go internal/editor/history_test.go
git commit -m "feat(editor): add change detection comparing baseline vs current items"
```

---

### Task 3: Diff View Theme Styles

**Files:**
- Modify: `internal/theme/fiori.go`

Add three lipgloss v1 style functions for the diff confirmation view.

- [ ] **Step 1: Add diff style functions**

Add to the bottom of `internal/theme/fiori.go`, after `OverrideSuffix()`:

```go
func DiffAdded() lipglossv1.Style {
	return lipglossv1.NewStyle().Foreground(lipglossv1.Color("#00D68F"))
}

func DiffEdited() lipglossv1.Style {
	return lipglossv1.NewStyle().Foreground(lipglossv1.Color("#F58B00"))
}

func DiffDeleted() lipglossv1.Style {
	return lipglossv1.NewStyle().Foreground(lipglossv1.Color("#FF5C5C"))
}

func DiffMuted() lipglossv1.Style {
	return lipglossv1.NewStyle().Foreground(lipglossv1.Color("#8C9BAA"))
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./... && go vet ./...`

Expected: clean

- [ ] **Step 3: Commit**

```bash
git add internal/theme/fiori.go
git commit -m "feat(theme): add diff view styles for added/edited/deleted/muted"
```

---

### Task 4: Diff Confirmation Bubbletea Model

**Files:**
- Create: `internal/editor/diff.go`

A standalone Bubbletea model that renders the change summary and handles save/cancel/discard keybindings. Separated from `history.go` for clarity — history is data logic, diff is UI.

- [ ] **Step 1: Create diff.go with the diffModel**

Create `internal/editor/diff.go`:

```go
package editor

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/SAP-samples/sap-devs-cli/internal/theme"
)

type diffAction int

const (
	diffSave    diffAction = iota
	diffCancel             // back to list
	diffDiscard            // quit without saving
)

type diffModel struct {
	changes []Change
	action  diffAction
	cursor  int
	width   int
	height  int
}

func newDiffModel(changes []Change) diffModel {
	return diffModel{
		changes: changes,
		action:  diffCancel,
		width:   80,
		height:  24,
	}
}

func (m diffModel) Init() tea.Cmd { return nil }

func (m diffModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			m.action = diffSave
			return m, tea.Quit
		case "esc":
			m.action = diffCancel
			return m, tea.Quit
		case "d":
			m.action = diffDiscard
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < m.totalLines()-1 {
				m.cursor++
			}
		}
	}
	return m, nil
}

func (m diffModel) totalLines() int {
	n := 0
	for _, c := range m.changes {
		n++
		if c.Kind == ChangeEdited {
			n += len(c.Fields)
		}
	}
	return n
}

func (m diffModel) View() string {
	var sb strings.Builder

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#4DB8FF"))
	sb.WriteString(fmt.Sprintf("\n  %s (%d modification%s)\n\n",
		titleStyle.Render("Review Changes"),
		len(m.changes),
		plural(len(m.changes)),
	))

	addedStyle := theme.DiffAdded()
	editedStyle := theme.DiffEdited()
	deletedStyle := theme.DiffDeleted()
	mutedStyle := theme.DiffMuted()

	maxVisible := m.height - 6
	if maxVisible < 5 {
		maxVisible = 5
	}

	lineIdx := 0
	start := 0
	if m.cursor >= maxVisible {
		start = m.cursor - maxVisible + 1
	}

	for _, c := range m.changes {
		switch c.Kind {
		case ChangeAdded:
			if lineIdx >= start && lineIdx < start+maxVisible {
				sb.WriteString(fmt.Sprintf("  %s %s\n",
					addedStyle.Render("+"),
					addedStyle.Render(c.ItemID+" (new)"),
				))
			}
			lineIdx++

		case ChangeDeleted:
			if lineIdx >= start && lineIdx < start+maxVisible {
				sb.WriteString(fmt.Sprintf("  %s %s\n",
					deletedStyle.Render("-"),
					deletedStyle.Render(c.ItemID+" (deleted)"),
				))
			}
			lineIdx++

		case ChangeEdited:
			if lineIdx >= start && lineIdx < start+maxVisible {
				sb.WriteString(fmt.Sprintf("  %s %s\n",
					editedStyle.Render("~"),
					editedStyle.Render(c.ItemID+" (edited)"),
				))
			}
			lineIdx++
			for _, f := range c.Fields {
				if lineIdx >= start && lineIdx < start+maxVisible {
					sb.WriteString(fmt.Sprintf("    %s: %s → %s\n",
						f.Key,
						mutedStyle.Render(f.OldValue),
						f.NewValue,
					))
				}
				lineIdx++
			}
		}
	}

	sb.WriteString("\n")
	footerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#8C9BAA"))
	sb.WriteString(footerStyle.Render("  Enter save  Esc back to list  d discard all"))
	sb.WriteString("\n")

	return sb.String()
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./... && go vet ./...`

Expected: clean

- [ ] **Step 3: Commit**

```bash
git add internal/editor/diff.go
git commit -m "feat(editor): add diff confirmation Bubbletea model for pre-save review"
```

---

### Task 5: List Model — Undo/Redo Keybindings + Status Line

**Files:**
- Modify: `internal/editor/list.go:41-58` (listModel struct)
- Modify: `internal/editor/list.go:108-149` (updateNormal)
- Modify: `internal/editor/list.go:175-250` (View)

Add `history` field, `statusMsg`, undo/redo result flags, `u`/`r` keybindings, and update the footer and header rendering.

- [ ] **Step 1: Add fields to listModel struct**

In `internal/editor/list.go`, modify the `listModel` struct (line 41) to add these fields after the existing `save` field (line 57):

```go
	// Undo/redo result fields.
	undone bool
	redone bool

	// History and status.
	history   *History
	statusMsg string
```

- [ ] **Step 2: Update newListModel to accept history and statusMsg**

Change the `newListModel` function signature (line 60) to:

```go
func newListModel(items []MergedItem, columns []string, target *ResolvedFile, s *schema.Schema, history *History, statusMsg string) listModel {
```

And set the new fields in the returned struct:

```go
	return listModel{
		items:     items,
		columns:   columns,
		target:    target,
		schema:    s,
		editIndex: -1,
		deleteIdx: -1,
		width:     80,
		height:    24,
		history:   history,
		statusMsg: statusMsg,
	}
```

- [ ] **Step 3: Add u/r keybindings to updateNormal**

In `updateNormal()` (line 108), add these cases before the closing `}` of the switch, after the `"/"` case (line 147):

```go
	case "u":
		if m.history != nil && m.history.CanUndo() {
			m.undone = true
			return m, tea.Quit
		}
	case "r":
		if m.history != nil && m.history.CanRedo() {
			m.redone = true
			return m, tea.Quit
		}
```

- [ ] **Step 4: Add status line to View()**

In the `View()` method (line 175), after the header line that prints filename/pack/layer/items count (line 180-185), add:

```go
	if m.statusMsg != "" {
		statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#00D68F")).Italic(true)
		depth := ""
		if m.history != nil && m.history.UndoDepth() > 0 {
			depth = fmt.Sprintf("  (%d in undo stack)", m.history.UndoDepth())
		}
		sb.WriteString(fmt.Sprintf("  %s%s\n", statusStyle.Render(m.statusMsg), depth))
	}
```

- [ ] **Step 5: Update footer text**

Replace the existing footer string (line 244-246):

```go
	sb.WriteString(footerStyle.Render(
		"  up/down navigate  Enter edit  a add  d delete  / filter  q save & quit  Esc quit",
	))
```

With:

```go
	sb.WriteString(footerStyle.Render(
		"  ↑/↓ navigate  Enter edit  a add  d delete  u undo  r redo  / filter  q save  Esc quit",
	))
```

- [ ] **Step 6: Verify build**

Run: `go build ./... && go vet ./...`

Expected: compilation error in `editor.go` because `newListModel` now takes extra arguments. That's expected — Task 6 updates `editor.go` to match. **Do not commit yet** — Tasks 5 and 6 will be committed together to avoid a broken intermediate state.

Proceed directly to Task 6.

---

### Task 6: Wire Everything into runArrayEditor

**Files:**
- Modify: `internal/editor/editor.go:64-135` (runArrayEditor)

This is the integration task. Modify the main editor loop to create a History, push snapshots before mutations, handle undo/redo signals from the list model, and show the diff confirmation view on save.

- [ ] **Step 1: Rewrite runArrayEditor**

Replace the existing `runArrayEditor` function (lines 64–135 of `editor.go`) with:

```go
func runArrayEditor(target *ResolvedFile, s *schema.Schema) error {
	cwd, _ := os.Getwd()
	items, err := LoadMergedItems(cwd, target.PackID, target.Filename)
	if err != nil {
		return err
	}

	columns := ColumnsForSchema(s.ItemSpec)
	history := NewHistory(items)
	var statusMsg string

	for {
		listMdl := newListModel(items, columns, target, s, history, statusMsg)
		p := tea.NewProgram(listMdl, tea.WithAltScreen())
		finalModel, err := p.Run()
		if err != nil {
			return err
		}

		result := finalModel.(listModel)
		statusMsg = ""

		// Handle undo.
		if result.undone {
			if restored, desc, ok := history.Undo(items); ok {
				items = restored
				statusMsg = fmt.Sprintf("↩ undid: %s", desc)
			}
			continue
		}

		// Handle redo.
		if result.redone {
			if restored, desc, ok := history.Redo(items); ok {
				items = restored
				statusMsg = fmt.Sprintf("↪ redid: %s", desc)
			}
			continue
		}

		// Handle delete.
		if result.deleteIdx >= 0 {
			desc := descForItem("deleted", items[result.deleteIdx].Data)
			history.Push(items, desc)
			items = append(items[:result.deleteIdx], items[result.deleteIdx+1:]...)
			statusMsg = fmt.Sprintf("✓ %s", desc)
			continue
		}

		// Handle add new item.
		if result.addNew {
			newItem := make(map[string]any)
			if err := editItem(s.ItemSpec, newItem); err != nil {
				if errors.Is(err, huh.ErrUserAborted) {
					continue
				}
				continue
			}
			history.Push(items, descForItem("added", newItem))
			items = append(items, MergedItem{
				Data:  newItem,
				Layer: target.Layer,
			})
			statusMsg = fmt.Sprintf("✓ %s", descForItem("added", newItem))
			continue
		}

		// Handle edit existing item.
		if result.editIndex >= 0 {
			item := &items[result.editIndex]
			if item.Layer != target.Layer {
				cloned := make(map[string]any)
				for k, v := range item.Data {
					cloned[k] = v
				}
				item.Data = cloned
				item.Layer = target.Layer
				item.IsOverride = true
			}
			desc := descForItem("edited", item.Data)
			history.Push(items, desc)
			if err := editItem(s.ItemSpec, item.Data); err != nil {
				if errors.Is(err, huh.ErrUserAborted) {
					// User cancelled — pop the snapshot we just pushed.
					if restored, _, ok := history.Undo(items); ok {
						items = restored
					}
					continue
				}
				continue
			}
			statusMsg = fmt.Sprintf("✓ %s", desc)
			continue
		}

		// Handle save+quit.
		if result.save {
			if !history.HasChanges(items) {
				fmt.Fprintln(os.Stdout, "No changes.")
				return nil
			}

			changes := history.Changes(items)
			dm := newDiffModel(changes)
			dp := tea.NewProgram(dm, tea.WithAltScreen())
			diffResult, err := dp.Run()
			if err != nil {
				return err
			}

			switch diffResult.(diffModel).action {
			case diffSave:
				return SaveItems(target.FilePath, items, target.Layer)
			case diffDiscard:
				fmt.Fprintln(os.Stdout, "Changes discarded.")
				return nil
			case diffCancel:
				statusMsg = "Save cancelled — back to editing"
				continue
			}
		}

		// Quit without saving (Esc).
		return nil
	}
}

func descForItem(verb string, data map[string]any) string {
	id := itemKey(data)
	if id == "" {
		id = "item"
	}
	return fmt.Sprintf("%s %q", verb, id)
}
```

- [ ] **Step 2: Verify build (includes Task 5 list.go changes)**

Run: `go build ./... && go vet ./...`

Expected: clean build, no warnings (both `list.go` signature change and `editor.go` call site now match)

- [ ] **Step 3: Commit Task 5 + Task 6 together**

```bash
git add internal/editor/list.go internal/editor/editor.go
git commit -m "feat(editor): wire undo/redo history and diff view into array editor loop"
```

- [ ] **Step 4: Manual smoke test**

Run from the worktree directory:

```bash
SAP_DEVS_DEV=1 go run . content edit cap/resources.yaml
```

Verify:
1. List view shows resources with footer including `u undo  r redo`
2. Edit an item → status line shows "✓ edited ..."
3. Press `u` → item reverts, status shows "↩ undid: ..."
4. Press `r` → re-applies, status shows "↪ redid: ..."
5. Press `q` → diff view appears showing the change
6. Press `Esc` in diff view → returns to list
7. Press `q` again → diff view → `Enter` → saves
8. Press `q` with no changes → "No changes." printed, exits

- [ ] **Step 5: Commit**

```bash
git add internal/editor/editor.go
git commit -m "docs: update smoke test verification for undo/redo"
```

Note: The main commit was already done in Step 3 above. Only commit here if additional changes were needed from smoke testing.

---

### Task 7: Documentation Updates

**Files:**
- Modify: `TODO.md`
- Modify: `CLAUDE.md`

- [ ] **Step 1: Update TODO.md**

In `TODO.md`, under the `#### Phase 2 — TUI Enhancements` section, mark the undo/redo and diff view items as done:

Change:
```markdown
- Undo/redo support within the editor session
- Diff view showing changes against the lower-layer version before saving
```

To:
```markdown
- Undo/redo support within the editor session - DONE ✔️
- Diff view showing changes against the lower-layer version before saving - DONE ✔️
```

Also mark the git integration line as dropped:

Change:
```markdown
- Git commit/push integration — commit and push the edited file from within the TUI
```

To:
```markdown
- ~~Git commit/push integration~~ — dropped; developers use their own git tools
```

- [ ] **Step 2: Update CLAUDE.md**

In the `content` command description row in the CLI Commands table, update to mention undo/redo:

Current:
```markdown
| `content` | Manage content YAML files; `content edit/validate/list` with `--pack`/`--layer`/`--json` filtering |
```

Updated:
```markdown
| `content` | Manage content YAML files; `content edit/validate/list` with `--pack`/`--layer`/`--json` filtering; edit includes undo/redo and pre-save diff review |
```

- [ ] **Step 3: Verify build**

Run: `go build ./... && go vet ./...`

Expected: clean (no Go changes in this task, but good practice)

- [ ] **Step 4: Commit**

```bash
git add TODO.md CLAUDE.md
git commit -m "docs: mark undo/redo and diff view as done in TODO, update CLAUDE.md"
```
