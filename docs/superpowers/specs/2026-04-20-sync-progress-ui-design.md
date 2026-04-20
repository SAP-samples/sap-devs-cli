# Sync Progress UI Design

**Date:** 2026-04-20
**Status:** Approved
**Scope:** Unified Bubbletea progress display for the `sap-devs sync` command

## Problem

The sync command has 7 phases, but only the archive fetch (Phase 1) prints a status line and marker expansion (Phase 2) has a Bubbletea progress UI. Phases 3-7 (events, YouTube, Discovery Center, tutorials, learning) are completely silent and can each take seconds to complete, creating long pauses with no terminal feedback.

## Solution

Replace all sync output with a single Bubbletea inline program that renders a progress bar and phase status list throughout the entire sync lifecycle. The existing marker expansion detail integrates as sub-items under the markers phase.

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
- Progress bar gradient: FioriBlue → FioriGreen

## Architecture

### Message Protocol

Typed messages flow from a sync worker goroutine to the Bubbletea program:

```go
type PhaseID int
const (
    PhaseContent PhaseID = iota
    PhaseCompany
    PhaseMarkers
    PhaseEvents
    PhaseYouTube
    PhaseDiscovery
    PhaseTutorials
    PhaseLearning
)

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
    phases   []phaseState       // ordered, one per PhaseID
    markers  []markerItem       // sub-items under PhaseMarkers (reused)
    progress progress.Model     // bubbles progress bar
    spinner  spinner.Model      // bubbles spinner for active phases
    done     int                // phases completed + skipped
    total    int                // total phase count
    fatalErr error              // propagated back to caller
}
```

Runs inline (no alt-screen). Spinner tick keeps the display alive during long-running phases — this is what eliminates the "long pause" problem.

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

- `cmd/sync.go` — refactor `runSync` into setup + Bubbletea launch + `syncWorker`; update `runMarkerExpansion` to accept `*tea.Program`
- `internal/ui/progress.go` — remove `RunMarkerExpansion` (program logic moves to `sync_progress.go`); keep `MarkerDoneMsg` and marker item types for reuse
- `go.mod` / `go.sum` — add `github.com/charmbracelet/bubbles` as direct v1 dependency (for `progress` and `spinner`)

### Unchanged

- `internal/sync/*` — engine, fetcher, marker, state all untouched
- `internal/theme/fiori.go` — consumed, not modified
- All `run*Fetch` function internals — same behavior, just no longer print to stdout

## Edge Cases

- **Non-TTY (CI, pipe):** Bubbletea renders a single final frame. No spinner animation but the phase list still prints.
- **"Up to date" fast path:** If nothing is stale, skip Bubbletea entirely, print existing i18n message directly.
- **Fatal error (archive fetch fails):** `SyncDoneMsg{FatalErr: err}` quits the program, error propagates to caller.
- **Non-fatal phase errors:** Rendered as `✗ failed` with warning text; sync continues.

## Scope Boundary

Explicitly NOT included:
- Per-phase elapsed time display
- Parallel phase execution (phases stay sequential)
- Changes to `inject --sync` flow (it calls `runSync`, gets new UI automatically)
