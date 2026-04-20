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
	restored, _, _ := h.Undo(items)
	assert.Equal(t, "Alpha", restored[0].Data["title"])
	restored[0].Data["title"] = "Mutated Again"
	redone, _, _ := h.Redo(restored)
	assert.Equal(t, "Mutated", redone[0].Data["title"])
}
