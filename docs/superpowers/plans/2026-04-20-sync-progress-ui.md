# Sync Progress UI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace silent sync phases with a unified Bubbletea inline progress display showing a phase status list and progress bar.

**Architecture:** A single Bubbletea model (`syncModel`) in `internal/ui/sync_progress.go` renders all sync progress. `cmd/sync.go` is refactored so `runSync` builds a phase plan, launches the Bubbletea program, and kicks off a `syncWorker` goroutine that sends typed messages as phases execute. Hand-rolled spinner + progress bar using lipgloss v1 — no new dependencies.

**Tech Stack:** Go, charmbracelet/bubbletea v1, charmbracelet/lipgloss v1, golang.org/x/term

**Spec:** `docs/superpowers/specs/2026-04-20-sync-progress-ui-design.md`

---

## File Map

| File | Action | Responsibility |
|------|--------|---------------|
| `internal/ui/sync_progress.go` | Create | Bubbletea model, message types, `View()`, `RunSyncProgress()` entry point, hand-rolled progress bar + spinner, lipgloss v1 styling |
| `internal/ui/sync_progress_test.go` | Create | Unit tests for `syncModel` Update/View logic |
| `internal/ui/progress.go` | Modify | Remove `RunMarkerExpansion()` and `progressModel`; keep `MarkerDoneMsg`; export `MarkerItem` with exported fields |
| `cmd/sync.go` | Modify | Refactor `runSync` into phase plan + Bubbletea launch + `syncWorker`; TTY detection; update `runMarkerExpansion` to accept `*tea.Program` |

---

## Task 1: Create the Bubbletea Model with Message Types and View

**Files:**
- Create: `internal/ui/sync_progress.go`
- Create: `internal/ui/sync_progress_test.go`

This task builds the core Bubbletea model that renders the phase list and progress bar. It is tested in isolation using programmatic message dispatch — no actual sync logic.

- [ ] **Step 1: Write the failing test — syncModel renders pending phases**

```go
// internal/ui/sync_progress_test.go
package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSyncModel_View_AllPending(t *testing.T) {
	phases := []PhaseID{PhaseContent, PhaseMarkers, PhaseEvents}
	m := newSyncModel(phases)
	view := m.View()
	assert.Contains(t, view, "Syncing SAP developer content")
	assert.Contains(t, view, "content")
	assert.Contains(t, view, "markers")
	assert.Contains(t, view, "events")
	assert.Contains(t, view, "0%")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go build ./internal/ui/...`
Expected: compile failure — `PhaseID`, `newSyncModel` not defined

- [ ] **Step 3: Write the syncModel, message types, PhaseID constants, and View()**

Create `internal/ui/sync_progress.go` with:

```go
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
	styleDone    = lipgloss.NewStyle().Foreground(colorGreen)
	styleActive  = lipgloss.NewStyle().Foreground(colorBlue)
	styleFailed  = lipgloss.NewStyle().Foreground(colorRed)
	styleMuted   = lipgloss.NewStyle().Foreground(colorMuted)
	styleBarFill = lipgloss.NewStyle().Foreground(colorBlue)
	styleBarEmpty = lipgloss.NewStyle().Foreground(colorMuted)
)

const spinnerChars = `⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏`
const barWidth = 20

type PhaseID int

const (
	PhaseContent PhaseID = iota
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

type PhaseStartMsg struct{ ID PhaseID }
type PhaseDoneMsg struct {
	ID      PhaseID
	Summary string
	Err     error
}
type PhaseSkipMsg struct{ ID PhaseID }
type SyncDoneMsg struct{ FatalErr error }
type spinnerTickMsg struct{}

type syncModel struct {
	phases   []phaseState
	markers  []MarkerItem
	frame    int
	done     int
	total    int
	fatalErr error
}

func newSyncModel(visiblePhases []PhaseID) *syncModel {
	phases := make([]phaseState, len(visiblePhases))
	for i, id := range visiblePhases {
		phases[i] = phaseState{id: id, status: statusPending}
	}
	return &syncModel{
		phases: phases,
		total:  len(visiblePhases),
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
		if p.id == PhaseMarkers && len(m.markers) > 0 {
			for _, item := range m.markers {
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
			}
		}
	}
	return b.String()
}

// SetMarkersMsg populates marker sub-items in the model.
type SetMarkersMsg struct{ Items []MarkerItem }

// FatalErr returns the fatal error from the sync worker, if any.
func (m *syncModel) FatalErr() error {
	return m.fatalErr
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go build ./internal/ui/... && go vet ./internal/ui/...`
Expected: compiles and passes vet

- [ ] **Step 5: Write additional model tests — phase transitions and progress bar**

```go
// Append to internal/ui/sync_progress_test.go

func TestSyncModel_Update_PhaseStart(t *testing.T) {
	phases := []PhaseID{PhaseContent, PhaseEvents}
	m := newSyncModel(phases)
	m.Update(PhaseStartMsg{ID: PhaseContent})
	assert.Equal(t, statusActive, m.phases[0].status)
}

func TestSyncModel_Update_PhaseDone(t *testing.T) {
	phases := []PhaseID{PhaseContent, PhaseEvents}
	m := newSyncModel(phases)
	m.Update(PhaseStartMsg{ID: PhaseContent})
	m.Update(PhaseDoneMsg{ID: PhaseContent, Summary: "fetched archive"})
	assert.Equal(t, statusDone, m.phases[0].status)
	assert.Equal(t, "fetched archive", m.phases[0].summary)
	assert.Equal(t, 1, m.done)
}

func TestSyncModel_Update_PhaseSkip(t *testing.T) {
	phases := []PhaseID{PhaseContent, PhaseEvents}
	m := newSyncModel(phases)
	m.Update(PhaseSkipMsg{ID: PhaseEvents})
	assert.Equal(t, statusSkipped, m.phases[1].status)
	assert.Equal(t, 1, m.done)
}

func TestSyncModel_Update_PhaseFailed(t *testing.T) {
	phases := []PhaseID{PhaseDiscovery}
	m := newSyncModel(phases)
	m.Update(PhaseStartMsg{ID: PhaseDiscovery})
	m.Update(PhaseDoneMsg{ID: PhaseDiscovery, Err: fmt.Errorf("connection refused")})
	assert.Equal(t, statusFailed, m.phases[0].status)
	assert.Contains(t, m.phases[0].summary, "connection refused")
	assert.Equal(t, 1, m.done)
}

func TestSyncModel_ProgressBar_Percent(t *testing.T) {
	phases := []PhaseID{PhaseContent, PhaseEvents, PhaseDiscovery}
	m := newSyncModel(phases)
	m.Update(PhaseDoneMsg{ID: PhaseContent, Summary: "done"})
	view := m.View()
	assert.Contains(t, view, "33%")
}

func TestSyncModel_Update_SyncDone_Quits(t *testing.T) {
	phases := []PhaseID{PhaseContent}
	m := newSyncModel(phases)
	_, cmd := m.Update(SyncDoneMsg{})
	// tea.Quit returns a function; check it is non-nil
	assert.NotNil(t, cmd)
}

func TestSyncModel_View_MarkerSubItems(t *testing.T) {
	phases := []PhaseID{PhaseMarkers}
	m := newSyncModel(phases)
	m.markers = []MarkerItem{
		{PackID: "cap", Index: 0, Label: "CAP release notes", State: "done", Lines: 42},
		{PackID: "btp", Index: 1, Label: "BTP updates", State: "fetching"},
	}
	view := m.View()
	assert.Contains(t, view, "cap")
	assert.Contains(t, view, "42 lines")
	assert.Contains(t, view, "fetching...")
}
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `go build ./internal/ui/... && go vet ./internal/ui/...`
Expected: compiles clean

- [ ] **Step 7: Commit**

```bash
git add internal/ui/sync_progress.go internal/ui/sync_progress_test.go
git commit -m "feat(ui): add sync progress Bubbletea model with phase list and progress bar"
```

---

## Task 2: Add RunSyncProgress Entry Point and Plain Text Fallback

**Files:**
- Modify: `internal/ui/sync_progress.go`
- Modify: `internal/ui/sync_progress_test.go`

This task adds the public `RunSyncProgress()` function that `cmd/sync.go` will call, plus `PrintPlainProgress()` for non-TTY fallback.

- [ ] **Step 1: Write the failing test — plain text fallback**

```go
// Append to internal/ui/sync_progress_test.go

func TestPrintPlainProgress_Done(t *testing.T) {
	var buf strings.Builder
	PrintPlainProgress(&buf, PhaseEvents, "done", "2 event types", nil)
	out := buf.String()
	assert.Contains(t, out, "events")
	assert.Contains(t, out, "2 event types")
}

func TestPrintPlainProgress_Failed(t *testing.T) {
	var buf strings.Builder
	PrintPlainProgress(&buf, PhaseDiscovery, "failed", "", fmt.Errorf("timeout"))
	out := buf.String()
	assert.Contains(t, out, "discovery")
	assert.Contains(t, out, "timeout")
}

func TestPrintPlainProgress_Skipped(t *testing.T) {
	var buf strings.Builder
	PrintPlainProgress(&buf, PhaseTutorials, "skipped", "", nil)
	out := buf.String()
	assert.Contains(t, out, "tutorials")
	assert.Contains(t, out, "skipped")
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go build ./internal/ui/...`
Expected: compile failure — `PrintPlainProgress` not defined

- [ ] **Step 3: Implement RunSyncProgress and PrintPlainProgress**

Add to `internal/ui/sync_progress.go`:

```go
// RunSyncProgress starts the Bubbletea inline program for sync progress.
// Returns the syncModel (to extract fatalErr) after the program exits.
// The caller should launch a goroutine that sends PhaseStartMsg/PhaseDoneMsg/
// PhaseSkipMsg/MarkerDoneMsg/SyncDoneMsg to the returned *tea.Program.
func RunSyncProgress(visiblePhases []PhaseID) (*tea.Program, *syncModel) {
	m := newSyncModel(visiblePhases)
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
```

- [ ] **Step 4: Run tests**

Run: `go build ./internal/ui/... && go vet ./internal/ui/...`
Expected: compiles clean

- [ ] **Step 5: Commit**

```bash
git add internal/ui/sync_progress.go internal/ui/sync_progress_test.go
git commit -m "feat(ui): add RunSyncProgress entry point and plain text fallback"
```

---

## Task 3: Slim Down progress.go — Remove RunMarkerExpansion

**Files:**
- Modify: `internal/ui/progress.go`

The marker expansion Bubbletea program logic moves to the unified sync progress model. Keep `MarkerDoneMsg` and `markerItem` since `sync_progress.go` imports them.

- [ ] **Step 1: Remove `progressModel`, `newProgressModel`, and `RunMarkerExpansion` from `internal/ui/progress.go`**

The file should only retain:

```go
package ui

// MarkerDoneMsg is sent by fetch goroutines when a marker fetch completes.
type MarkerDoneMsg struct {
	PackID string
	Index  int
	Label  string
	Lines  int
	Err    error
}

// MarkerItem represents a single marker sub-item in the sync progress display.
// Exported so cmd/sync.go can construct items and send them via SetMarkersMsg.
type MarkerItem struct {
	PackID string
	Index  int
	Label  string
	State  string // "fetching", "done", "failed"
	Lines  int
}
```

Remove these imports that are no longer needed: `fmt`, `os`, `strconv`, `strings`, `sync`, `tea`, `sapSync`.

- [ ] **Step 2: Verify the project compiles**

Run: `go build ./...`
Expected: compile failure in `cmd/sync.go` because it still calls `ui.RunMarkerExpansion`. This is expected — Task 4 will fix it.

- [ ] **Step 3: Commit (will not compile yet — that's OK, Task 4 completes the refactor)**

```bash
git add internal/ui/progress.go
git commit -m "refactor(ui): slim progress.go to marker types only, remove standalone Bubbletea program"
```

---

## Task 4: Refactor cmd/sync.go — Phase Plan + syncWorker + TTY Detection

**Files:**
- Modify: `cmd/sync.go`

This is the main orchestration refactor. `runSync` builds a phase plan, detects TTY, and either launches the Bubbletea program with a `syncWorker` goroutine, or runs the plain text fallback path.

- [ ] **Step 1: Add the `syncPlan` type and `buildSyncPlan` function**

Add near the top of `cmd/sync.go`, after the existing imports:

```go
type syncPlan struct {
	visiblePhases  []ui.PhaseID
	archiveNeeded  bool
	companyNeeded  bool
	markersNeeded  bool
	activeArchive  []string
	activeIndep    []string
	indepPhases    map[string]ui.PhaseID
	force          bool

	// Shared config passed to worker
	officialCache string
	cfg           *config.Config
	paths         *xdg.Paths
	engine        *sapSync.Engine
	token         string
}

var categoryToPhase = map[string]ui.PhaseID{
	"events":    ui.PhaseEvents,
	"youtube":   ui.PhaseYouTube,
	"discovery": ui.PhaseDiscovery,
	"tutorials": ui.PhaseTutorials,
	"learning":  ui.PhaseLearning,
}

func buildSyncPlan(force bool, cfg *config.Config, paths *xdg.Paths, engine *sapSync.Engine, token string, categories []string) *syncPlan {
	archiveCats := []string{"tips", "tools", "resources", "context", "mcp", "advocates"}
	independentCats := []string{"events", "youtube", "discovery", "tutorials", "learning"}
	activeArchive := intersectStrings(archiveCats, categories)
	activeIndep := intersectStrings(independentCats, categories)

	archiveNeeded := force
	for _, cat := range activeArchive {
		if engine.IsStale(cat) {
			archiveNeeded = true
			break
		}
	}

	var visiblePhases []ui.PhaseID

	if archiveNeeded {
		visiblePhases = append(visiblePhases, ui.PhaseContent)
		if cfg.CompanyRepo != "" {
			visiblePhases = append(visiblePhases, ui.PhaseCompany)
		}
		visiblePhases = append(visiblePhases, ui.PhaseMarkers)
	}

	for _, cat := range independentCats {
		if !containsString(activeIndep, cat) {
			continue
		}
		pid, ok := categoryToPhase[cat]
		if !ok {
			continue
		}
		// Include in visible list if forced or stale; otherwise skip entirely
		// so the "up to date" fast path works when all phases are fresh.
		if force || engine.IsStale(cat) {
			visiblePhases = append(visiblePhases, pid)
		}
	}

	return &syncPlan{
		visiblePhases: visiblePhases,
		archiveNeeded: archiveNeeded,
		companyNeeded: archiveNeeded && cfg.CompanyRepo != "",
		markersNeeded: archiveNeeded,
		activeArchive: activeArchive,
		activeIndep:   activeIndep,
		indepPhases:   categoryToPhase,
		force:         force,
		officialCache: filepath.Join(paths.CacheDir, "official"),
		cfg:           cfg,
		paths:         paths,
		engine:        engine,
		token:         token,
	}
}
```

- [ ] **Step 2: Write the `syncWorker` function**

```go
func syncWorker(p *tea.Program, plan *syncPlan) {
	var fatalErr error

	// Archive block: content + company + markers + changelog
	if plan.archiveNeeded {
		p.Send(ui.PhaseStartMsg{ID: ui.PhaseContent})
		if err := sapSync.FetchArchive(officialRepoArchive, plan.officialCache, plan.token); err != nil {
			fatalErr = fmt.Errorf("sync official content: %w", err)
			p.Send(ui.PhaseDoneMsg{ID: ui.PhaseContent, Err: fatalErr})
			p.Send(ui.SyncDoneMsg{FatalErr: fatalErr})
			return
		}
		if err := plan.engine.MarkAllSynced(plan.activeArchive); err != nil {
			fatalErr = fmt.Errorf("mark synced: %w", err)
			p.Send(ui.SyncDoneMsg{FatalErr: fatalErr})
			return
		}
		p.Send(ui.PhaseDoneMsg{ID: ui.PhaseContent, Summary: "fetched archive"})

		// Company
		if plan.companyNeeded {
			p.Send(ui.PhaseStartMsg{ID: ui.PhaseCompany})
			companyCache := filepath.Join(plan.paths.CacheDir, "company")
			repoURL := strings.TrimRight(plan.cfg.CompanyRepo, "/")
			if !strings.HasPrefix(plan.cfg.CompanyRepo, "https://") {
				p.Send(ui.PhaseDoneMsg{ID: ui.PhaseCompany, Err: fmt.Errorf("company repo must use https://")})
			} else {
				companyArchive := repoURL + "/archive/refs/heads/main.zip"
				if err := sapSync.FetchArchive(companyArchive, companyCache, plan.token); err != nil {
					p.Send(ui.PhaseDoneMsg{ID: ui.PhaseCompany, Err: err})
				} else {
					p.Send(ui.PhaseDoneMsg{ID: ui.PhaseCompany, Summary: "fetched company"})
				}
			}
		}

		// Markers
		p.Send(ui.PhaseStartMsg{ID: ui.PhaseMarkers})
		if err := runMarkerExpansion(plan.officialCache, plan.engine, p); err != nil {
			p.Send(ui.PhaseDoneMsg{ID: ui.PhaseMarkers, Err: err})
		} else {
			p.Send(ui.PhaseDoneMsg{ID: ui.PhaseMarkers, Summary: "expanded"})
		}

		// Changelog (hidden — no phase messages)
		changelogDirs := []string{filepath.Join(plan.officialCache, "content", "packs")}
		if plan.companyNeeded {
			changelogDirs = append(changelogDirs, filepath.Join(plan.paths.CacheDir, "company", "content", "packs"))
		}
		syncedAt := time.Now()
		clEntries, _ := sapSync.CollectChangelog(changelogDirs)
		_ = sapSync.WriteChangelog(plan.paths.CacheDir, syncedAt, clEntries)
	}

	// Independent phases
	type indepPhase struct {
		cat string
		id  ui.PhaseID
		fn  func() (string, error)
	}

	indeps := []indepPhase{
		{"events", ui.PhaseEvents, func() (string, error) {
			return "", runEventsFetch(plan.paths.CacheDir, plan.officialCache, plan.force)
		}},
		{"youtube", ui.PhaseYouTube, func() (string, error) {
			return "", runYouTubeFetch(plan.paths.CacheDir, plan.officialCache, plan.cfg.CompanyRepo, plan.paths.ConfigDir, plan.force, plan.cfg.Sync.YouTube)
		}},
		{"discovery", ui.PhaseDiscovery, func() (string, error) {
			return "", runDiscoveryFetch(plan.paths.CacheDir, plan.force)
		}},
		{"tutorials", ui.PhaseTutorials, func() (string, error) {
			return "", runTutorialsFetch(plan.paths.CacheDir, plan.force)
		}},
		{"learning", ui.PhaseLearning, func() (string, error) {
			return "", runLearningFetch(plan.paths.CacheDir, plan.force)
		}},
	}

	for _, ip := range indeps {
		if !containsString(plan.activeIndep, ip.cat) {
			continue
		}
		// buildSyncPlan already excluded fresh phases from visiblePhases,
		// so every phase here is stale or forced — no re-check needed.
		p.Send(ui.PhaseStartMsg{ID: ip.id})
		summary, err := ip.fn()
		if err != nil {
			p.Send(ui.PhaseDoneMsg{ID: ip.id, Err: err})
		} else {
			p.Send(ui.PhaseDoneMsg{ID: ip.id, Summary: summary})
		}
		_ = plan.engine.MarkSynced(ip.cat)
	}

	p.Send(ui.SyncDoneMsg{FatalErr: fatalErr})
}
```

- [ ] **Step 3: Rewrite `runSync` to use phase plan + Bubbletea / plain text fallback**

Replace the body of `runSync` (lines 50-196 of the current file):

```go
func runSync(ctx context.Context, force bool, out io.Writer) error {
	paths, err := xdg.New()
	if err != nil {
		return err
	}
	cfg, err := config.Load(paths.ConfigDir)
	if err != nil {
		return err
	}
	if cfg.Sync.Disabled {
		fmt.Fprintln(out, i18n.T(i18n.ActiveLang, "sync.disabled"))
		return nil
	}

	token := credentials.Resolve(paths.ConfigDir)
	categories := allCategories()
	if syncCategory != "" {
		categories = []string{syncCategory}
	}

	ttls := map[string]time.Duration{
		"tips": cfg.Sync.Tips, "tools": cfg.Sync.Tools,
		"advocates": cfg.Sync.Advocates, "resources": cfg.Sync.Resources,
		"context": cfg.Sync.Context, "mcp": cfg.Sync.MCP,
		"events": cfg.Sync.Events, "youtube": cfg.Sync.YouTube,
		"discovery": cfg.Sync.Discovery, "tutorials": cfg.Sync.Tutorials,
		"learning": cfg.Sync.Learning,
	}
	engine := sapSync.NewEngine(paths.CacheDir, 24*time.Hour, ttls)

	plan := buildSyncPlan(force, cfg, paths, engine, token, categories)

	if len(plan.visiblePhases) == 0 {
		fmt.Fprintln(out, i18n.T(i18n.ActiveLang, "sync.up_to_date"))
		return nil
	}

	// TTY detection: Bubbletea for interactive, plain text for pipes/CI
	isTTY := term.IsTerminal(int(os.Stdout.Fd()))

	if isTTY {
		return runSyncTTY(plan)
	}
	return runSyncPlain(plan, out)
}

func runSyncTTY(plan *syncPlan) error {
	p, model := ui.RunSyncProgress(plan.visiblePhases)
	go syncWorker(p, plan)
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("progress display: %w", err)
	}
	return model.FatalErr()
}

func runSyncPlain(plan *syncPlan, out io.Writer) error {
	fmt.Fprintln(out, "  Syncing SAP developer content")
	// Run same worker logic but with plain text output instead of Bubbletea
	// (inline execution, sequential, same phase ordering)

	if plan.archiveNeeded {
		if err := sapSync.FetchArchive(officialRepoArchive, plan.officialCache, plan.token); err != nil {
			return fmt.Errorf("sync official content: %w", err)
		}
		if err := plan.engine.MarkAllSynced(plan.activeArchive); err != nil {
			return fmt.Errorf("mark synced: %w", err)
		}
		ui.PrintPlainProgress(out, ui.PhaseContent, "done", "fetched archive", nil)

		if plan.companyNeeded {
			companyCache := filepath.Join(plan.paths.CacheDir, "company")
			repoURL := strings.TrimRight(plan.cfg.CompanyRepo, "/")
			companyArchive := repoURL + "/archive/refs/heads/main.zip"
			if err := sapSync.FetchArchive(companyArchive, companyCache, plan.token); err != nil {
				ui.PrintPlainProgress(out, ui.PhaseCompany, "failed", "", err)
			} else {
				ui.PrintPlainProgress(out, ui.PhaseCompany, "done", "fetched company", nil)
			}
		}

		if err := runMarkerExpansion(plan.officialCache, plan.engine, nil); err != nil {
			ui.PrintPlainProgress(out, ui.PhaseMarkers, "failed", "", err)
		} else {
			ui.PrintPlainProgress(out, ui.PhaseMarkers, "done", "expanded", nil)
		}

		changelogDirs := []string{filepath.Join(plan.officialCache, "content", "packs")}
		if plan.companyNeeded {
			changelogDirs = append(changelogDirs, filepath.Join(plan.paths.CacheDir, "company", "content", "packs"))
		}
		syncedAt := time.Now()
		clEntries, _ := sapSync.CollectChangelog(changelogDirs)
		_ = sapSync.WriteChangelog(plan.paths.CacheDir, syncedAt, clEntries)
	}

	type indepPhase struct {
		cat string
		id  ui.PhaseID
		fn  func() error
	}
	indeps := []indepPhase{
		{"events", ui.PhaseEvents, func() error { return runEventsFetch(plan.paths.CacheDir, plan.officialCache, plan.force) }},
		{"youtube", ui.PhaseYouTube, func() error { return runYouTubeFetch(plan.paths.CacheDir, plan.officialCache, plan.cfg.CompanyRepo, plan.paths.ConfigDir, plan.force, plan.cfg.Sync.YouTube) }},
		{"discovery", ui.PhaseDiscovery, func() error { return runDiscoveryFetch(plan.paths.CacheDir, plan.force) }},
		{"tutorials", ui.PhaseTutorials, func() error { return runTutorialsFetch(plan.paths.CacheDir, plan.force) }},
		{"learning", ui.PhaseLearning, func() error { return runLearningFetch(plan.paths.CacheDir, plan.force) }},
	}

	for _, ip := range indeps {
		if !containsString(plan.activeIndep, ip.cat) {
			continue
		}
		if err := ip.fn(); err != nil {
			ui.PrintPlainProgress(out, ip.id, "failed", "", err)
		} else {
			ui.PrintPlainProgress(out, ip.id, "done", "", nil)
		}
		_ = plan.engine.MarkSynced(ip.cat)
	}

	return nil
}
```

Add `"golang.org/x/term"` and `tea "github.com/charmbracelet/bubbletea"` to the imports. Remove the now-unused `gosync "sync"` and `"golang.org/x/sync/errgroup"` imports if they are only used indirectly via the removed inline sync code (check — `errgroup` is still used by `runTutorialsFetch`, and `gosync` is still used there too, so keep them).

- [ ] **Step 4: Update `runMarkerExpansion` to accept `*tea.Program`**

Change the signature of `runMarkerExpansion` (currently at line 201):

```go
func runMarkerExpansion(officialCache string, engine *sapSync.Engine, p *tea.Program) error {
```

Replace the old `results, fetchErrs := ui.RunMarkerExpansion(allMarkers)` call with inline goroutine logic that sends `MarkerDoneMsg` to `p` (when non-nil) or runs silently (when `p == nil`, plain text path):

```go
	if len(allMarkers) == 0 {
		return nil
	}

	// Populate marker sub-items in the Bubbletea model
	if p != nil {
		items := make([]ui.MarkerItem, len(allMarkers))
		for i, m := range allMarkers {
			label := m.Label
			if label == "" {
				label = m.URL
			}
			items[i] = ui.MarkerItem{PackID: m.PackID, Index: m.Index, Label: label, State: "fetching"}
		}
		p.Send(ui.SetMarkersMsg{Items: items})
	}

	results := make(map[string]string)
	errs := make(map[string]error)
	var mu gosync.Mutex

	sem := make(chan struct{}, 4)
	var wg gosync.WaitGroup

	for _, m := range allMarkers {
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
			key := m.PackID + "::" + strconv.Itoa(m.Index)
			if err != nil {
				mu.Lock()
				errs[key] = err
				mu.Unlock()
				if p != nil {
					p.Send(ui.MarkerDoneMsg{PackID: m.PackID, Index: m.Index, Label: label, Err: err})
				}
				return
			}
			mu.Lock()
			results[key] = content
			mu.Unlock()
			lineCount := strings.Count(content, "\n") + 1
			if p != nil {
				p.Send(ui.MarkerDoneMsg{PackID: m.PackID, Index: m.Index, Label: label, Lines: lineCount})
			}
		}(m)
	}
	wg.Wait()
```

**Note:** This requires adding a `SetMarkersMsg` type and handling it in `syncModel.Update` (already added in Task 1). Add to `sync_progress.go` if not already present:

```go
type SetMarkersMsg struct{ Items []MarkerItem }
```

And in `Update`:
```go
case SetMarkersMsg:
    m.markers = msg.Items
```

- [ ] **Step 5: Remove the old `runSync` inline phase execution and old `runMarkerExpansion` standalone Bubbletea call**

Delete the old `runSync` body, the old `runMarkerExpansion` function. The new versions from steps 1-5 replace them.

- [ ] **Step 6: Verify the project compiles**

Run: `go build ./... && go vet ./...`
Expected: compiles and passes vet with no errors

- [ ] **Step 7: Commit**

```bash
git add cmd/sync.go internal/ui/sync_progress.go
git commit -m "feat(sync): unified progress UI with Bubbletea phase list, progress bar, and TTY fallback"
```

---

## Task 5: Manual Smoke Test

**Files:** None (testing only)

- [ ] **Step 1: Build the binary**

```bash
VERSION=$(git describe --tags --always --dirty)
go build -ldflags "-X github.com/SAP-samples/sap-devs-cli/cmd.Version=${VERSION}" -o sap-devs .
```

- [ ] **Step 2: Test interactive TTY sync**

```bash
./sap-devs sync --force
```

Expected: Full progress bar + phase status list with spinner animation. Each phase shows ✓/✗/─ as it completes. Progress bar fills from 0% to 100%.

- [ ] **Step 3: Test non-TTY fallback**

```bash
./sap-devs sync --force | cat
```

Expected: Plain text lines, one per phase, no ANSI escape sequence artifacts.

- [ ] **Step 4: Test --category filtering**

```bash
./sap-devs sync --force --category events
```

Expected: Only the events phase row appears. Progress bar goes 0% → 100%.

- [ ] **Step 5: Test up-to-date fast path**

```bash
./sap-devs sync
```

Expected: If just synced, prints "Content is up to date" with no progress bar.

- [ ] **Step 6: Commit any fixups from smoke testing**

```bash
git add -A
git commit -m "fix(sync): smoke test fixups for sync progress UI"
```

---

## Task 6: Final Verification and Cleanup

**Files:**
- All modified files

- [ ] **Step 1: Run full build + vet**

```bash
go build ./... && go vet ./...
```

Expected: clean

- [ ] **Step 2: Check for any stale references to `ui.RunMarkerExpansion`**

```bash
grep -r "RunMarkerExpansion" --include="*.go" .
```

Expected: no matches

- [ ] **Step 3: Check for any orphaned `fmt.Fprintf(os.Stderr` sync warnings in cmd/sync.go**

The old `fmt.Fprintf(os.Stderr, "sap-devs: ...")` warning lines in the phase execution blocks should be gone — errors flow through `PhaseDoneMsg.Err` now.

- [ ] **Step 4: Commit cleanup if needed**

```bash
git add -A
git commit -m "chore: clean up stale references after sync progress UI refactor"
```
