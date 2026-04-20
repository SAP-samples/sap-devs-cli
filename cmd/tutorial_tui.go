package cmd

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/SAP-samples/sap-devs-cli/internal/tutorials"
)

type tutorialTUI struct {
	tutorial   *tutorials.Tutorial
	step       int
	totalSteps int
	dataDir    string
	rendered   string
	width      int
	height     int
}

func newTutorialTUI(tut *tutorials.Tutorial, dataDir string, startStep int) tutorialTUI {
	total := len(tut.Steps)
	if startStep < 1 || startStep > total {
		startStep = 1
	}
	return tutorialTUI{
		tutorial:   tut,
		step:       startStep,
		totalSteps: total,
		dataDir:    dataDir,
		width:      80,
		height:     24,
	}
}

func (m tutorialTUI) Init() tea.Cmd {
	return nil
}

func (m tutorialTUI) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			// Save progress on quit
			_ = tutorials.UpdateProgress(m.dataDir, m.tutorial.Slug, m.step, m.totalSteps, false)
			return m, tea.Quit
		case "n", "right", "l":
			if m.step < m.totalSteps {
				m.step++
				m.rendered = ""
				_ = tutorials.UpdateProgress(m.dataDir, m.tutorial.Slug, m.step, m.totalSteps, false)
			}
		case "p", "left", "h":
			if m.step > 1 {
				m.step--
				m.rendered = ""
				_ = tutorials.UpdateProgress(m.dataDir, m.tutorial.Slug, m.step, m.totalSteps, false)
			}
		case "d":
			// Mark current step as done and auto-advance
			_ = tutorials.UpdateProgress(m.dataDir, m.tutorial.Slug, m.step, m.totalSteps, true)
			if m.step < m.totalSteps {
				m.step++
				m.rendered = ""
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.rendered = ""
	}
	return m, nil
}

func (m tutorialTUI) View() string {
	if m.rendered != "" {
		return m.rendered
	}

	step := m.tutorial.Steps[m.step-1]

	var sb strings.Builder

	// Header bar
	sb.WriteString(fmt.Sprintf("\n  %s — Step %d of %d: %s\n", m.tutorial.Title, m.step, m.totalSteps, step.Title))
	sb.WriteString(fmt.Sprintf("  Time: %dm | Level: %s\n", m.tutorial.Time, m.tutorial.Level))

	w := min(m.width, 100)
	sb.WriteString(strings.Repeat("─", w))
	sb.WriteString("\n\n")

	// Render step content with glamour
	r, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(w),
	)
	rendered, err := r.Render(step.Content)
	if err != nil {
		sb.WriteString(step.Content)
	} else {
		sb.WriteString(rendered)
	}

	// Footer with keybindings
	sb.WriteString("\n")
	sb.WriteString(strings.Repeat("─", w))
	sb.WriteString("\n")

	nav := "  "
	if m.step > 1 {
		nav += "← p/left prev  "
	}
	if m.step < m.totalSteps {
		nav += "→ n/right next  "
	}
	nav += "d = done  q = quit"
	sb.WriteString(nav)
	sb.WriteString("\n")

	m.rendered = sb.String()
	return m.rendered
}

func runTutorialTUI(tut *tutorials.Tutorial, dataDir string, startStep int) error {
	// Resume from saved progress when no explicit step requested
	if startStep == 0 {
		progress, err := tutorials.GetProgress(dataDir, tut.Slug)
		if err == nil && progress != nil && progress.CurrentStep > 0 {
			startStep = progress.CurrentStep
		} else {
			startStep = 1
		}
	}

	model := newTutorialTUI(tut, dataDir, startStep)
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
