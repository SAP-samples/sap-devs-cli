# Sync Progress UI Design

**Date:** 2026-04-20
**Status:** Approved
**Scope:** Unified Bubbletea progress display for the `sap-devs sync` command

## Problem

The sync command has 8 logical phases (content, company, markers, events, YouTube, Discovery Center, tutorials, learning), but only the archive fetch prints a status line and marker expansion has a Bubbletea progress UI. Phases 3-7 (events, YouTube, Discovery Center, tutorials, learning) are completely silent and can each take seconds to complete, creating long pauses with no terminal feedback.

## Solution

Replace all sync output with a single Bubbletea inline program that renders a progress bar and phase status list throughout the entire sync lifecycle. The existing marker expansion detail integrates as sub-items under the markers phase. When stdout is not a TTY, fall back to plain text progress lines.

## Visual Design

```
  Syncing SAP developer content
  [████████░░░░░░░░] 50%
    content     ✓  fetched archive
    company     ─  skipped
    markers         expanding...
      cap      › CAP release notes             ✓  (42 lines)
      btp-core › BTP service updates           fetching...
    events      ✓  2 event types (0.4s)
    youtube     ●  syncing...
    discovery   ─  pending
    tutorials   ─  pending
    learning    ─  pending
```

### Styling (SAP Fiori Horizon Evening palette)

- `✓` done: FioriGreen (#00D68F)
- `●` active spinner: FioriBlue (#4DB8FF)
- `✗` failed: FioriRed (#FF5C5C)
- `─` pending/skipped: FioriMuted (#8C9BAA)
- Progress bar fill: FioriBlue (#4DB8FF), empty: FioriMuted (#8C9BAA)

All styling uses `github.com/charmbracelet/lipgloss` v1 (matching `internal/ui/progress.go`). Define local color constants in `sync_progress.go` rather than importing `internal/theme` (which mixes lipgloss v1 and v2 types).

### Non-TTY Fallback

Before launching the Bubbletea program, check `term.IsTerminal(int(os.Stdout.Fd()))` (same pattern as `cmd/inject.go`). When stdout is not a TTY:

- Skip Bubbletea entirely
- Print plain text progress lines to `out` as each phase starts/completes: `"  ✓ content"`, `"  ✓ events (2 types)"`, `"  ✗ discovery (fetch failed)"`
- This ensures `inject --sync`, CI pipelines, and piped output work correctly

### `--category` Filtering

When `--category` is set, the phase list shown in the UI is filtered to only the relevant phase(s). For example, `sync --category events` shows only the events phase row and the progress bar fills from 0% to 100% for that single phase. Skipped phases for other categories are not rendered.

## Architecture

### Message Protocol

Typed messages flow from a sync worker goroutine to the Bubbletea program:

```go
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
```

**Company + changelog coupling:** In the current code, company sync and changelog collection happen inside the `archiveNeedsSync` block, tightly coupled to the content phase. In the new model, `PhaseContent`, `PhaseCompany`, `PhaseMarkers`, and `PhaseChangelog` are all sub-phases of the archive sync block. The `syncWorker` runs them sequentially within the archive-needs-sync conditional, and skips all four together when the archive is fresh. Changelog collection receives both the official and company cache directories, exactly as today.

```go
type PhaseStartMsg struct{ ID PhaseID }
type PhaseDoneMsg  struct{ ID PhaseID; Summary string; Err error }
type PhaseSkipMsg  struct{ ID PhaseID }
type SyncDoneMsg   struct{ FatalErr error }
// MarkerDoneMsg — reused from existing code
```

### Phase States

Each phase transitions through: `pending` → `active` → `done`/`failed`, or directly to `skipped`.

Progress bar percent = `(done + skipped) / total`.

### Bubbletea Model

Single `syncModel` in `internal/ui/sync_progress.go`:

```go
type syncModel struct {
    phases   []phaseState       // ordered, one per visible phase
    markers  []markerItem       // sub-items under PhaseMarkers (reused)
    frame    int                // spinner frame counter (ticked by tea.Tick)
    done     int                // phases completed + skipped
    total    int                // total visible phase count
    fatalErr error              // propagated back to caller
}
```

No `bubbles` dependency. The progress bar and spinner are rendered manually using lipgloss v1 styled strings, matching the approach in the existing `progressModel` in `progress.go`:

- **Progress bar:** Hand-built from `█` (filled) and `░` (empty) characters, width 20, styled with lipgloss v1.
- **Spinner:** Rotating dot sequence (`⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏`), advanced by `tea.Tick` at 100ms intervals. This tick is what keeps the display alive during long-running phases.

### Sync Orchestration

`runSync` in `cmd/sync.go` is refactored into two parts:

1. **Setup** — load config, resolve TTLs, build phase plan (which phases run vs skip)
2. **Launch** — start Bubbletea program inline, kick off `syncWorker` goroutine

```go
func runSync(ctx context.Context, force bool, out io.Writer) error {
    // 1. Load config, resolve TTLs, determine phase plan
    // 2. If nothing stale: print "up to date", return (no Bubbletea)
    // 3. Launch Bubbletea program (inline)
    // 4. syncWorker goroutine sends messages as phases execute
    // 5. p.Run() blocks until SyncDoneMsg
    // 6. Return fatalErr if any
}
```

The `syncWorker` runs each phase sequentially, sending `PhaseStartMsg` before and `PhaseDoneMsg` after each:

```go
func syncWorker(p *tea.Program, plan syncPlan) {
    // For each active phase:
    //   p.Send(PhaseStartMsg{ID: ...})
    //   err := existingRunFunc(...)
    //   p.Send(PhaseDoneMsg{ID: ..., Summary: "...", Err: err})
    // For skipped phases (sent upfront):
    //   p.Send(PhaseSkipMsg{ID: ...})
    // Finally:
    //   p.Send(SyncDoneMsg{FatalErr: err})
}
```

### Marker Integration

`runMarkerExpansion` is updated to accept a `*tea.Program` parameter instead of creating its own. Marker fetch goroutines send `MarkerDoneMsg` directly to the parent program. The `syncModel` renders marker sub-items indented under the markers phase.

## File Changes

### New files

- `internal/ui/sync_progress.go` — `syncModel`, message types, `View()`, `RunSyncProgress()` entry point

### Modified files

- `cmd/sync.go` — refactor `runSync` into setup + Bubbletea launch + `syncWorker`; update `runMarkerExpansion` to accept `*tea.Program`; add TTY detection gating Bubbletea vs plain text fallback
- `internal/ui/progress.go` — remove `RunMarkerExpansion` (program logic moves to `sync_progress.go`); keep `MarkerDoneMsg` and marker item types for reuse

No new dependencies. Progress bar and spinner are hand-rendered with lipgloss v1 strings.

### Unchanged

- `internal/sync/*` — engine, fetcher, marker, state all untouched
- `internal/theme/fiori.go` — consumed, not modified
- All `run*Fetch` function internals — same behavior, just no longer print to stdout

## Edge Cases

- **Non-TTY (CI, pipe, `inject --sync`):** Detected via `term.IsTerminal(int(os.Stdout.Fd()))`. Falls back to plain `fmt.Fprintln(out, ...)` lines per phase — no Bubbletea, no spinner. This matches the existing pattern in `cmd/inject.go:374`.
- **`--category` filter:** Phase list shows only the targeted phase(s). Progress bar fills 0→100% for the subset.
- **"Up to date" fast path:** If nothing is stale, skip Bubbletea entirely, print existing i18n message directly.
- **Fatal error (archive fetch fails):** `SyncDoneMsg{FatalErr: err}` quits the program, error propagates to caller.
- **Non-fatal phase errors:** Rendered as `✗ failed` with warning text; sync continues.

## Scope Boundary

Explicitly NOT included:
- Per-phase elapsed time display
- Parallel phase execution (phases stay sequential)
- New external dependencies (no `bubbles` — progress bar and spinner are hand-rendered)
- Changes to `inject --sync` flow (it calls `runSync`, gets the new UI automatically when TTY; plain text fallback otherwise)
