package editor

// Snapshot holds a deep copy of the item list at a point in time, along with
// a human-readable description of the change that produced it.
type Snapshot struct {
	Items []MergedItem
	Desc  string
}

// History tracks undo/redo snapshots for the content editor.
// Push a snapshot before each mutation; Undo/Redo restore previous states.
type History struct {
	baseline  []MergedItem
	undoStack []Snapshot
	redoStack []Snapshot
}

// NewHistory creates a new History seeded with the initial item state.
func NewHistory(items []MergedItem) *History {
	return &History{
		baseline: deepCopyItems(items),
	}
}

// Push records the current item state onto the undo stack and clears the redo
// stack (a new edit invalidates any previously undone future).
func (h *History) Push(items []MergedItem, desc string) {
	h.undoStack = append(h.undoStack, Snapshot{
		Items: deepCopyItems(items),
		Desc:  desc,
	})
	h.redoStack = nil
}

// Undo pops the most recent snapshot from the undo stack and returns it.
// current is the caller's present state; it is pushed onto the redo stack so
// the change can be re-applied. Returns (nil, "", false) if nothing to undo.
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

// Redo pops the most recent snapshot from the redo stack and returns it.
// current is pushed onto the undo stack. Returns (nil, "", false) if nothing
// to redo.
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

// CanUndo reports whether there are states available to undo.
func (h *History) CanUndo() bool { return len(h.undoStack) > 0 }

// CanRedo reports whether there are states available to redo.
func (h *History) CanRedo() bool { return len(h.redoStack) > 0 }

// UndoDepth returns the number of undoable steps currently on the stack.
func (h *History) UndoDepth() int { return len(h.undoStack) }

// Baseline returns a deep copy of the initial item state passed to NewHistory.
func (h *History) Baseline() []MergedItem { return deepCopyItems(h.baseline) }

// deepCopyItems returns a new slice where each MergedItem has its own copy of
// the Data map. Shallow map values (scalars, strings) are copied by value;
// nested maps/slices are not deep-copied — content YAML values are always
// scalars or simple strings so this is sufficient.
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
