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
	result := editor.MoveItems(items, map[int]bool{2: true}, true)
	assert.Equal(t, []string{"a", "c", "b", "d"}, ids(result))
}

func TestMoveItems_SingleDown(t *testing.T) {
	items := fourItems()
	result := editor.MoveItems(items, map[int]bool{1: true}, false)
	assert.Equal(t, []string{"a", "c", "b", "d"}, ids(result))
}

func TestMoveItems_MultiUp(t *testing.T) {
	items := fourItems()
	result := editor.MoveItems(items, map[int]bool{2: true, 3: true}, true)
	assert.Equal(t, []string{"a", "c", "d", "b"}, ids(result))
}

func TestMoveItems_MultiDown(t *testing.T) {
	items := fourItems()
	result := editor.MoveItems(items, map[int]bool{0: true, 1: true}, false)
	assert.Equal(t, []string{"c", "a", "b", "d"}, ids(result))
}

func TestMoveItems_AtBoundaryUp(t *testing.T) {
	items := fourItems()
	result := editor.MoveItems(items, map[int]bool{0: true}, true)
	assert.Equal(t, []string{"a", "b", "c", "d"}, ids(result))
}

func TestMoveItems_AtBoundaryDown(t *testing.T) {
	items := fourItems()
	result := editor.MoveItems(items, map[int]bool{3: true}, false)
	assert.Equal(t, []string{"a", "b", "c", "d"}, ids(result))
}

func TestMoveItems_AdjacentSelectedUp(t *testing.T) {
	items := fourItems()
	result := editor.MoveItems(items, map[int]bool{1: true, 2: true}, true)
	assert.Equal(t, []string{"b", "c", "a", "d"}, ids(result))
}

func TestMoveItems_NoSelection(t *testing.T) {
	items := fourItems()
	result := editor.MoveItems(items, map[int]bool{}, true)
	assert.Equal(t, []string{"a", "b", "c", "d"}, ids(result))
}
