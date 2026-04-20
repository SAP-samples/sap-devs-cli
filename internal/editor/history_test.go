package editor_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.tools.sap/developer-relations/sap-devs-cli/internal/editor"
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
	h.Push(items, "edited Alpha")
	items[0].Data["title"] = "Alpha Edited"
	assert.True(t, h.CanUndo())
	restored, desc, ok := h.Undo(items)
	require.True(t, ok)
	assert.Equal(t, "edited Alpha", desc)
	assert.Equal(t, "Alpha", restored[0].Data["title"])
}

func TestHistory_Redo(t *testing.T) {
	items := sampleItems()
	h := editor.NewHistory(items)
	h.Push(items, "edited Alpha")
	items[0].Data["title"] = "Alpha Edited"
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
	h.Push(items, "edit 1")
	items[0].Data["title"] = "Edit 1"
	restored, _, _ := h.Undo(items)
	assert.True(t, h.CanRedo())
	h.Push(restored, "edit beta")
	restored[1].Data["title"] = "Beta Edited"
	assert.False(t, h.CanRedo())
}

func TestHistory_UndoAll(t *testing.T) {
	items := sampleItems()
	h := editor.NewHistory(items)
	h.Push(items, "edit 1")
	items[0].Data["title"] = "Edit 1"
	h.Push(items, "edit 2")
	items[0].Data["title"] = "Edit 2"
	h.Push(items, "edit 3")
	items[0].Data["title"] = "Edit 3"
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
	h.Push(items, "mutated")
	items[0].Data["title"] = "Mutated"
	restored, _, _ := h.Undo(items)
	assert.Equal(t, "Alpha", restored[0].Data["title"])
	restored[0].Data["title"] = "Mutated Again"
	redone, _, _ := h.Redo(restored)
	assert.Equal(t, "Mutated", redone[0].Data["title"])
}

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
	h.Push(items, "edit")
	items[0].Data["title"] = "Changed"
	restored, _, _ := h.Undo(items)
	assert.False(t, h.HasChanges(restored))
}

func TestHistory_DiscardLast(t *testing.T) {
	items := sampleItems()
	h := editor.NewHistory(items)

	h.Push(items, "edited Alpha")
	items[0].Data["title"] = "Edited"

	restored, ok := h.DiscardLast()
	require.True(t, ok)
	assert.Equal(t, "Alpha", restored[0].Data["title"])
	assert.False(t, h.CanUndo())
	assert.False(t, h.CanRedo())
}

func TestHistory_DiscardLast_EmptyStack(t *testing.T) {
	items := sampleItems()
	h := editor.NewHistory(items)
	_, ok := h.DiscardLast()
	assert.False(t, ok)
}

func TestHistory_DiscardLast_PreservesRedo(t *testing.T) {
	items := sampleItems()
	h := editor.NewHistory(items)

	h.Push(items, "edit 1")
	items[0].Data["title"] = "Edit 1"
	h.Push(items, "edit 2")
	items[0].Data["title"] = "Edit 2"

	h.Undo(items)
	assert.True(t, h.CanRedo())

	h.DiscardLast()
	assert.True(t, h.CanRedo())
}

func TestHistory_BulkUndoSingleSnapshot(t *testing.T) {
	items := []editor.MergedItem{
		{Data: map[string]any{"id": "a", "scope": "old"}, Layer: editor.LayerUser},
		{Data: map[string]any{"id": "b", "scope": "old"}, Layer: editor.LayerUser},
		{Data: map[string]any{"id": "c", "scope": "old"}, Layer: editor.LayerUser},
	}
	h := editor.NewHistory(items)

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
