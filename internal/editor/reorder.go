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
