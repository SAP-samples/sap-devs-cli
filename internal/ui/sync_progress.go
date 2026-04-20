package ui

import (
	"fmt"
	"io"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	lipgloss "github.com/charmbracelet/lipgloss"
)

// Fiori Horizon Evening palette — local lipgloss v1 constants to avoid
// importing internal/theme which mixes lipgloss v1 and v2.
var (
	colorGreen = lipgloss.Color("#00D68F")
	colorBlue  = lipgloss.Color("#4DB8FF")
	colorRed   = lipgloss.Color("#FF5C5C")
	colorMuted = lipgloss.Color("#8C9BAA")
)

var (
	styleDone     = lipgloss.NewStyle().Foreground(colorGreen)
	styleActive   = lipgloss.NewStyle().Foreground(colorBlue)
	styleFailed   = lipgloss.NewStyle().Foreground(colorRed)
	styleMuted    = lipgloss.NewStyle().Foreground(colorMuted)
	styleBarFill  = lipgloss.NewStyle().Foreground(colorBlue)
	styleBarEmpty = lipgloss.NewStyle().Foreground(colorMuted)
)

const spinnerChars = `⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏`
const barWidth = 20

// PhaseID identifies a sync phase.
type PhaseID int

const (
	PhaseContent   PhaseID = iota
	PhaseCompany
	PhaseMarkers
	PhaseChangelog
	PhaseEvents
	PhaseYouTube
	PhaseDiscovery
	PhaseTutorials
	PhaseLearning
)

var phaseLabels = map[PhaseID]string{
	PhaseContent:   "content",
	PhaseCompany:   "company",
	PhaseMarkers:   "markers",
	PhaseChangelog: "changelog",
	PhaseEvents:    "events",
	PhaseYouTube:   "youtube",
	PhaseDiscovery: "discovery",
	PhaseTutorials: "tutorials",
	PhaseLearning:  "learning",
}

type phaseStatus int

const (
	statusPending phaseStatus = iota
	statusActive
	statusDone
	statusFailed
	statusSkipped
)

type phaseState struct {
	id      PhaseID
	status  phaseStatus
	summary string
}

// PhaseStartMsg is sent when a sync phase begins.
type PhaseStartMsg struct{ ID PhaseID }

// PhaseDoneMsg is sent when a sync phase completes (successfully or with error).
type PhaseDoneMsg struct {
	ID      PhaseID
	Summary string
	Err     error
}

// PhaseSkipMsg is sent when a sync phase is skipped (e.g. content is fresh).
type PhaseSkipMsg struct{ ID PhaseID }

// SyncDoneMsg signals that all sync work is complete.
type SyncDoneMsg struct{ FatalErr error }

// WarnMsg queues a warning to be printed after the progress display exits.
type WarnMsg struct{ Text string }

type spinnerTickMsg struct{}

// SetMarkersMsg populates marker sub-items in the model.
type SetMarkersMsg struct{ Items []MarkerItem }

type syncModel struct {
	phases      []phaseState
	markers     []MarkerItem
	markerSlots int
	frame       int
	done        int
	total       int
	fatalErr    error
	warnings    []string
}

func newSyncModel(visiblePhases []PhaseID, markerSlots int) *syncModel {
	phases := make([]phaseState, len(visiblePhases))
	for i, id := range visiblePhases {
		phases[i] = phaseState{id: id, status: statusPending}
	}
	return &syncModel{
		phases:      phases,
		total:       len(visiblePhases),
		markerSlots: markerSlots,
	}
}

func spinnerTick() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg {
		return spinnerTickMsg{}
	})
}

func (m *syncModel) Init() tea.Cmd {
	return spinnerTick()
}

func (m *syncModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinnerTickMsg:
		m.frame++
		return m, spinnerTick()
	case PhaseStartMsg:
		for i := range m.phases {
			if m.phases[i].id == msg.ID {
				m.phases[i].status = statusActive
				break
			}
		}
	case PhaseDoneMsg:
		for i := range m.phases {
			if m.phases[i].id == msg.ID {
				if msg.Err != nil {
					m.phases[i].status = statusFailed
					m.phases[i].summary = msg.Err.Error()
				} else {
					m.phases[i].status = statusDone
					m.phases[i].summary = msg.Summary
				}
				m.done++
				break
			}
		}
	case PhaseSkipMsg:
		for i := range m.phases {
			if m.phases[i].id == msg.ID {
				m.phases[i].status = statusSkipped
				m.phases[i].summary = "skipped (fresh)"
				m.done++
				break
			}
		}
	case MarkerDoneMsg:
		for i, item := range m.markers {
			if item.PackID == msg.PackID && item.Index == msg.Index {
				if msg.Err != nil {
					m.markers[i].State = "failed"
				} else {
					m.markers[i].State = "done"
					m.markers[i].Lines = msg.Lines
				}
				break
			}
		}
	case SetMarkersMsg:
		m.markers = msg.Items
	case WarnMsg:
		m.warnings = append(m.warnings, msg.Text)
	case SyncDoneMsg:
		m.fatalErr = msg.FatalErr
		return m, tea.Quit
	}
	return m, nil
}

func (m *syncModel) View() string {
	var b strings.Builder
	b.WriteString("  Syncing SAP developer content\n")

	// Progress bar
	pct := 0.0
	if m.total > 0 {
		pct = float64(m.done) / float64(m.total)
	}
	filled := int(pct * barWidth)
	if filled > barWidth {
		filled = barWidth
	}
	bar := styleBarFill.Render(strings.Repeat("█", filled)) +
		styleBarEmpty.Render(strings.Repeat("░", barWidth-filled))
	fmt.Fprintf(&b, "  [%s] %d%%\n", bar, int(pct*100))

	// Phase list
	spinnerRunes := []rune(spinnerChars)
	spinChar := string(spinnerRunes[m.frame%len(spinnerRunes)])

	for _, p := range m.phases {
		label := phaseLabels[p.id]
		switch p.status {
		case statusDone:
			summary := p.summary
			if summary != "" {
				summary = "  " + summary
			}
			fmt.Fprintf(&b, "    %-12s %s%s\n", label, styleDone.Render("✓"), summary)
		case statusFailed:
			summary := p.summary
			if summary != "" {
				summary = "  " + summary
			}
			fmt.Fprintf(&b, "    %-12s %s%s\n", label, styleFailed.Render("✗"), summary)
		case statusActive:
			fmt.Fprintf(&b, "    %-12s %s  syncing...\n", label, styleActive.Render(spinChar))
		case statusSkipped:
			fmt.Fprintf(&b, "    %-12s %s  %s\n", label, styleMuted.Render("─"), styleMuted.Render("skipped (fresh)"))
		default:
			fmt.Fprintf(&b, "    %-12s %s  %s\n", label, styleMuted.Render("─"), styleMuted.Render("pending"))
		}

		// Render marker sub-items after the markers phase
		if p.id == PhaseMarkers && m.markerSlots > 0 {
			for i := 0; i < m.markerSlots; i++ {
				if i < len(m.markers) {
					item := m.markers[i]
					switch item.State {
					case "done":
						fmt.Fprintf(&b, "      %-8s › %-36s %s  (%d lines)\n",
							item.PackID, item.Label, styleDone.Render("✓"), item.Lines)
					case "failed":
						fmt.Fprintf(&b, "      %-8s › %-36s %s  fetch failed, using cached\n",
							item.PackID, item.Label, styleFailed.Render("✗"))
					default:
						fmt.Fprintf(&b, "      %-8s › %-36s fetching...\n",
							item.PackID, item.Label)
					}
				} else {
					b.WriteString("\n")
				}
			}
		}
	}
	return b.String()
}

// FatalErr returns the fatal error from the sync worker, if any.
func (m *syncModel) FatalErr() error {
	return m.fatalErr
}

// Warnings returns any warnings collected during sync.
func (m *syncModel) Warnings() []string {
	return m.warnings
}

// RunSyncProgress starts the Bubbletea inline program for sync progress.
// markerSlots pre-allocates lines for marker sub-items so the view height
// stays constant (prevents ghost lines from terminal scroll).
func RunSyncProgress(visiblePhases []PhaseID, markerSlots int) (*tea.Program, *syncModel) {
	m := newSyncModel(visiblePhases, markerSlots)
	p := tea.NewProgram(m)
	return p, m
}

// PrintPlainProgress writes a single non-TTY progress line for one phase.
func PrintPlainProgress(out io.Writer, id PhaseID, status string, summary string, err error) {
	label := phaseLabels[id]
	switch status {
	case "done":
		if summary != "" {
			fmt.Fprintf(out, "  ✓ %s  %s\n", label, summary)
		} else {
			fmt.Fprintf(out, "  ✓ %s\n", label)
		}
	case "failed":
		if err != nil {
			fmt.Fprintf(out, "  ✗ %s  %v\n", label, err)
		} else {
			fmt.Fprintf(out, "  ✗ %s\n", label)
		}
	case "skipped":
		fmt.Fprintf(out, "  ─ %s  skipped (fresh)\n", label)
	}
}
