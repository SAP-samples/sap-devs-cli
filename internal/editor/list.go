package editor

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/SAP-samples/sap-devs-cli/internal/schema"
	"github.com/SAP-samples/sap-devs-cli/internal/theme"
)

// Styles for the list view.
var (
	selectedStyle = theme.SelectedStyle()
	headerStyle   = theme.HeaderStyle()
)

// layerBadge returns a styled string label for a content layer.
func layerBadge(l Layer, isOverride bool) string {
	var badge string
	switch l {
	case LayerOfficial:
		badge = theme.LayerBadgeOfficial().Render("official")
	case LayerCompany:
		badge = theme.LayerBadgeCompany().Render("company")
	case LayerUser:
		badge = theme.LayerBadgeUser().Render("user")
	case LayerProject:
		badge = theme.LayerBadgeProject().Render("project")
	default:
		badge = l.String()
	}
	if isOverride {
		badge += theme.OverrideSuffix().Render(" (override)")
	}
	return badge
}

// listModel is a Bubbletea model for the array content list view.
type listModel struct {
	items     []MergedItem
	columns   []string
	cursor    int
	width     int
	height    int
	filter    string
	filtering bool
	target    *ResolvedFile
	schema    *schema.Schema

	// Result fields set before quitting.
	editIndex int  // index of item to edit, or -1
	addNew    bool // user wants to add a new item
	deleteIdx int  // index of item to delete, or -1
	quit      bool // user chose to quit
	save      bool // user chose to save before quitting

	// Undo/redo result fields.
	undone bool
	redone bool

	// History and status.
	history   *History
	statusMsg string

	// Selection and bulk action result fields.
	selected             map[int]bool // originalIndex -> selected
	moveUp               bool
	moveDown             bool
	bulkAction           string // "set-field", "delete", "add-tag", or ""
	cursorOriginalIndex  int    // resolved originalIndex of cursor item (filter-safe)
}

func newListModel(items []MergedItem, columns []string, target *ResolvedFile, s *schema.Schema, history *History, statusMsg string) listModel {
	return listModel{
		items:               items,
		columns:             columns,
		target:              target,
		schema:              s,
		editIndex:           -1,
		deleteIdx:           -1,
		width:               80,
		height:              24,
		history:             history,
		statusMsg:           statusMsg,
		selected:            make(map[int]bool),
		cursorOriginalIndex: -1,
	}
}

func (m listModel) Init() tea.Cmd {
	return nil
}

func (m listModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		if m.filtering {
			return m.updateFilter(msg)
		}
		return m.updateNormal(msg)
	}
	return m, nil
}

func (m listModel) updateFilter(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter", "esc":
		m.filtering = false
	case "backspace":
		if len(m.filter) > 0 {
			m.filter = m.filter[:len(m.filter)-1]
		}
	default:
		if len(msg.String()) == 1 {
			m.filter += msg.String()
		}
	}
	return m, nil
}

func (m listModel) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		m.save = true
		m.quit = true
		return m, tea.Quit
	case "esc":
		if len(m.selected) > 0 {
			m.selected = make(map[int]bool)
		} else {
			m.quit = true
			return m, tea.Quit
		}
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		visible := m.visibleItems()
		if m.cursor < len(visible)-1 {
			m.cursor++
		}
	case "enter":
		visible := m.visibleItems()
		if m.cursor < len(visible) {
			m.editIndex = visible[m.cursor].originalIndex
			return m, tea.Quit
		}
	case "a":
		m.addNew = true
		return m, tea.Quit
	case "d":
		if len(m.selected) > 0 {
			m.bulkAction = "delete"
			return m, tea.Quit
		}
		visible := m.visibleItems()
		if m.cursor < len(visible) {
			idx := visible[m.cursor].originalIndex
			// Only allow deletion of items in the target layer.
			if m.items[idx].Layer == m.target.Layer {
				m.deleteIdx = idx
				return m, tea.Quit
			}
		}
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
	case "ctrl+a":
		visible := m.visibleItems()
		for _, vi := range visible {
			if vi.item.Layer == m.target.Layer {
				m.selected[vi.originalIndex] = true
			}
		}
	case "/":
		m.filtering = true
		m.filter = ""
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
	}
	return m, nil
}

// visibleItem pairs a MergedItem with its original index in the full list.
type visibleItem struct {
	originalIndex int
	item          MergedItem
}

func (m listModel) visibleItems() []visibleItem {
	var out []visibleItem
	for i, item := range m.items {
		if m.filter != "" {
			id, _ := item.Data["id"].(string)
			name, _ := item.Data["name"].(string)
			title, _ := item.Data["title"].(string)
			text := strings.ToLower(id + " " + name + " " + title)
			if !strings.Contains(text, strings.ToLower(m.filter)) {
				continue
			}
		}
		out = append(out, visibleItem{originalIndex: i, item: item})
	}
	return out
}

func (m listModel) View() string {
	var sb strings.Builder

	// Header
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("114"))
	sb.WriteString(fmt.Sprintf("\n  %s  Pack: %s  Layer: %s  %d items\n",
		titleStyle.Render(m.target.Filename),
		m.target.PackID,
		m.target.Layer,
		len(m.items),
	))

	if m.statusMsg != "" {
		statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#00D68F")).Italic(true)
		depth := ""
		if m.history != nil && m.history.UndoDepth() > 0 {
			depth = fmt.Sprintf("  (%d in undo stack)", m.history.UndoDepth())
		}
		sb.WriteString(fmt.Sprintf("  %s%s\n", statusStyle.Render(m.statusMsg), depth))
	}

	if m.filtering {
		sb.WriteString(fmt.Sprintf("  / %s_\n", m.filter))
	}

	sb.WriteString("\n")

	// Column header row
	header := "    "
	for _, col := range m.columns {
		header += fmt.Sprintf("%-20s", strings.ToUpper(col))
	}
	header += "LAYER"
	sb.WriteString(headerStyle.Render(header))
	sb.WriteString("\n")

	// Items
	visible := m.visibleItems()
	maxVisible := m.height - 8
	if maxVisible < 5 {
		maxVisible = 5
	}

	start := 0
	if m.cursor >= maxVisible {
		start = m.cursor - maxVisible + 1
	}

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

	// Footer
	sb.WriteString("\n")
	footerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	if len(m.selected) > 0 {
		sb.WriteString(footerStyle.Render(
			fmt.Sprintf("  %d selected: e set field  d delete  t add/remove tag  J/K reorder  Esc clear", len(m.selected)),
		))
	} else {
		sb.WriteString(footerStyle.Render(
			"  ↑/↓ navigate  Enter edit  a add  d delete  u undo  r redo  / filter  q save  Esc quit",
		))
	}
	sb.WriteString("\n")

	return sb.String()
}
