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
