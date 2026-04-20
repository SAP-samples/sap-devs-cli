package editor_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/editor"
)

func TestBulkDelete(t *testing.T) {
	items := []editor.MergedItem{
		{Data: map[string]any{"id": "a"}, Layer: editor.LayerUser},
		{Data: map[string]any{"id": "b"}, Layer: editor.LayerUser},
		{Data: map[string]any{"id": "c"}, Layer: editor.LayerUser},
		{Data: map[string]any{"id": "d"}, Layer: editor.LayerUser},
	}
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
