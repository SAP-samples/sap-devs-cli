package ui

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	sapSync "github.com/SAP-samples/sap-devs-cli/internal/sync"
)

// MarkerDoneMsg is sent by fetch goroutines when a marker fetch completes.
type MarkerDoneMsg struct {
	PackID string
	Index  int
	Label  string
	Lines  int
	Err    error
}

type markerItem struct {
	packID string
	index  int
	label  string
	state  string // "fetching", "done", "failed"
	lines  int
}

type progressModel struct {
	items []markerItem
	total int
	done  int
}

func newProgressModel(markers []sapSync.Marker) progressModel {
	items := make([]markerItem, len(markers))
	for i, m := range markers {
		label := m.Label
		if label == "" {
			label = m.URL
		}
		items[i] = markerItem{
			packID: m.PackID,
			index:  m.Index,
			label:  label,
			state:  "fetching",
		}
	}
	return progressModel{items: items, total: len(markers)}
}

func (m progressModel) Init() tea.Cmd { return nil }

func (m progressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case MarkerDoneMsg:
		for i, item := range m.items {
			if item.packID == msg.PackID && item.index == msg.Index {
				if msg.Err != nil {
					m.items[i].state = "failed"
				} else {
					m.items[i].state = "done"
					m.items[i].lines = msg.Lines
				}
				m.done++
				break
			}
		}
		if m.done >= m.total {
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m progressModel) View() string {
	var b strings.Builder
	b.WriteString("  Expanding dynamic markers\n")
	for _, item := range m.items {
		switch item.state {
		case "done":
			fmt.Fprintf(&b, "    %-8s › %-40s ✓  (%d lines)\n", item.packID, item.label, item.lines)
		case "failed":
			fmt.Fprintf(&b, "    %-8s › %-40s ✗  fetch failed, using cached\n", item.packID, item.label)
		default:
			fmt.Fprintf(&b, "    %-8s › %-40s fetching...\n", item.packID, item.label)
		}
	}
	return b.String()
}

// RunMarkerExpansion fetches all markers in parallel (max 4 concurrent), drives a Bubbletea
// inline progress display, and returns results (packID::index → content) and any fetch errors.
// If markers is empty it returns immediately with no output.
func RunMarkerExpansion(markers []sapSync.Marker) (map[string]string, map[string]error) {
	if len(markers) == 0 {
		return nil, nil
	}

	results := make(map[string]string)
	errs := make(map[string]error)
	var mu sync.Mutex

	model := newProgressModel(markers)
	p := tea.NewProgram(model)

	sem := make(chan struct{}, 4)
	var wg sync.WaitGroup

	for _, m := range markers {
		wg.Add(1)
		go func(m sapSync.Marker) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			content, err := sapSync.FetchMarker(m, nil)
			label := m.Label
			if label == "" {
				label = m.URL
			}
			if err != nil {
				mu.Lock()
				errs[m.PackID+"::"+strconv.Itoa(m.Index)] = err
				mu.Unlock()
				p.Send(MarkerDoneMsg{PackID: m.PackID, Index: m.Index, Label: label, Err: err})
				return
			}
			mu.Lock()
			results[m.PackID+"::"+strconv.Itoa(m.Index)] = content
			mu.Unlock()
			lineCount := strings.Count(content, "\n") + 1
			p.Send(MarkerDoneMsg{PackID: m.PackID, Index: m.Index, Label: label, Lines: lineCount})
		}(m)
	}

	go func() {
		wg.Wait()
		p.Send(tea.QuitMsg{})
	}()

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "progress display error: %v\n", err)
	}

	return results, errs
}
