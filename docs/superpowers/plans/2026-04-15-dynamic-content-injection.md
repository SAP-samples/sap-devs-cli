# Dynamic Content Injection Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend the sync + inject pipeline so pack authors can embed `<!-- sync:fetch url="..." -->` markers in `context.md` that are expanded with live fetched content at sync time, with a Bubbletea progress display, pre-inject freshness check, and tiered agent instruction pattern.

**Architecture:** A new `internal/sync/state.go` migrates `sync-state.json` to a versioned `SyncState` struct; `internal/sync/marker.go` scans and fetches markers; `internal/ui/progress.go` drives a Bubbletea inline progress display. `cmd/sync.go` gains Phase 2 marker expansion via a shared `runSync()` helper; `cmd/inject.go` checks freshness and calls `runSync()` inline when needed. `LoadPack` in `internal/content/pack.go` prefers `context.expanded.md` over `context.md` when present.

**Tech Stack:** Go, Cobra, Bubbletea (`github.com/charmbracelet/bubbletea` — new direct dep; Charm sub-libs already present as indirect deps via glamour)

**Verification commands (Windows — no `go test` locally):**

- `go build ./...` — must produce no errors
- `go vet ./...` — must produce no warnings
- CI (`ubuntu-latest`) runs `go test ./...` as the authoritative test runner

**Spec:** `docs/superpowers/specs/2026-04-15-dynamic-content-injection-design.md`

---

## File Map

| File | Action | Responsibility |
| --- | --- | --- |
| `internal/sync/state.go` | **Create** | `SyncState`, `PackState`, `MarkerState` types; `loadSyncState`, `saveSyncState`, `markerKey` |
| `internal/sync/state_test.go` | **Create** | Migration tests: old flat format → reset; new format round-trips correctly |
| `internal/sync/engine.go` | **Modify** | Remove `loadState`/`saveState`; update `IsStale`/`MarkSynced`/`MarkAllSynced` to use `SyncState.Categories`; add `SetPackHasMarkers`, `RecordMarkerState`, `GetMarkerState` |
| `internal/sync/engine_test.go` | **Modify** | Update helpers that write old flat format to write new `SyncState` JSON |
| `internal/sync/marker.go` | **Create** | `Marker` type; `ScanMarkers` (parser + code-fence tracking); `FetchMarker` (HTTP + truncation); `ExpandMarkers` (string substitution) |
| `internal/sync/marker_test.go` | **Create** | Unit tests for scanner (valid, malformed, in-fence) and expander; HTTP tests via `httptest.NewServer` |
| `internal/ui/progress.go` | **Create** | Bubbletea inline `progressModel`; exported `RunMarkerExpansion(markers)` that drives parallel fetches and live display |
| `cmd/sync.go` | **Modify** | Extract `runSync(ctx, force, out)` helper; wire Phase 2 (scan → expand → `SetPackHasMarkers` + `RecordMarkerState`); call `RunMarkerExpansion` |
| `cmd/inject.go` | **Modify** | Add `--sync`/`--no-sync` flags; `checkStaleness(engine, packs)` helper; non-TTY detection; reload packs after inline sync |
| `internal/content/pack.go` | **Modify** | `LoadPack`: add expanded-file preference — `context.<lang>.md` → `context.expanded.md` → `context.md` |
| `internal/content/pack_test.go` | **Modify** | Add tests for expanded file preference and locale-vs-expanded precedence |
| `content/packs/cap/context.md` | **Modify** | Add first `sync:fetch` marker (feb26 release notes) + `### Agent Instructions` section |
| `docs/content-authoring.md` | **Create** | Pack authoring guide: marker syntax, attributes, parser rules, agent instructions, token budget guidance |
| `go.mod` / `go.sum` | **Modify** | Add `github.com/charmbracelet/bubbletea` direct dependency |

---

## Task 1: SyncState schema — new state.go

**Files:**

- Create: `internal/sync/state.go`
- Create: `internal/sync/state_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/sync/state_test.go`:

```go
package sync_test

import (
    "encoding/json"
    "os"
    "path/filepath"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    sapSync "github.tools.sap/developer-relations/sap-devs-cli/internal/sync"
)

func TestLoadSyncState_EmptyDir(t *testing.T) {
    dir := t.TempDir()
    state := sapSync.LoadSyncStateForTest(dir)
    assert.Equal(t, 1, state.Version)
    assert.NotNil(t, state.Categories)
    assert.NotNil(t, state.Packs)
    assert.NotNil(t, state.Markers)
}

func TestLoadSyncState_OldFlatFormat_Resets(t *testing.T) {
    dir := t.TempDir()
    // Write old flat map[string]time.Time format
    old := map[string]time.Time{"tips": time.Now().Add(-1 * time.Hour)}
    data, _ := json.Marshal(old)
    require.NoError(t, os.WriteFile(filepath.Join(dir, "sync-state.json"), data, 0600))

    state := sapSync.LoadSyncStateForTest(dir)
    // Old format cannot unmarshal into SyncState → reset
    assert.Equal(t, 1, state.Version)
    assert.Empty(t, state.Categories)
}
```

> Note: `LoadSyncStateForTest` is a test-exported wrapper around the unexported `loadSyncState`. Add it at the bottom of `state.go`: `func LoadSyncStateForTest(dir string) SyncState { return loadSyncState(dir) }` — only exists so tests can reach the internal function.
>
> Note: `TestLoadSyncState_NewFormatRoundTrips` (which uses `SetPackHasMarkers`) is in Task 2 — it depends on engine methods not yet defined.

- [ ] **Step 2: Verify tests fail to build** (function doesn't exist yet)

```bash
go build ./internal/sync/...
```

Expected: compile error referencing missing types/functions.

- [ ] **Step 3: Create `internal/sync/state.go`**

```go
package sync

import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "strconv"
    "time"
)

// SyncState is the versioned on-disk format of sync-state.json.
type SyncState struct {
    Version    int                    `json:"version"`
    Categories map[string]time.Time   `json:"categories"`
    Packs      map[string]PackState   `json:"packs"`
    Markers    map[string]MarkerState `json:"markers"`
}

// PackState records whether a pack has dynamic fetch markers.
type PackState struct {
    HasMarkers bool `json:"has_markers"`
}

// MarkerState records the result of the last fetch for a single marker.
type MarkerState struct {
    URL         string    `json:"url"`
    LastFetched time.Time `json:"last_fetched"`
    TTLHours    int       `json:"ttl_hours"`
    OK          bool      `json:"ok"`
}

func newSyncState() SyncState {
    return SyncState{
        Version:    1,
        Categories: make(map[string]time.Time),
        Packs:      make(map[string]PackState),
        Markers:    make(map[string]MarkerState),
    }
}

func loadSyncState(stateDir string) SyncState {
    data, err := os.ReadFile(filepath.Join(stateDir, "sync-state.json"))
    if err != nil {
        return newSyncState()
    }
    var state SyncState
    if err := json.Unmarshal(data, &state); err != nil {
        // Old flat format (map[string]time.Time) or corrupt file — reset.
        fmt.Fprintf(os.Stderr, "sap-devs: sync state reset after format upgrade\n")
        _ = os.Remove(filepath.Join(stateDir, "sync-state.json"))
        return newSyncState()
    }
    if state.Categories == nil {
        state.Categories = make(map[string]time.Time)
    }
    if state.Packs == nil {
        state.Packs = make(map[string]PackState)
    }
    if state.Markers == nil {
        state.Markers = make(map[string]MarkerState)
    }
    return state
}

func saveSyncState(stateDir string, state SyncState) error {
    if err := os.MkdirAll(stateDir, 0755); err != nil {
        return err
    }
    data, err := json.MarshalIndent(state, "", "  ")
    if err != nil {
        return err
    }
    return os.WriteFile(filepath.Join(stateDir, "sync-state.json"), data, 0600)
}

// markerKey returns the sync-state.json key for a marker identified by pack + position index.
func markerKey(packID string, index int) string {
    return packID + "::" + strconv.Itoa(index)
}

// LoadSyncStateForTest exposes the internal state loader for tests.
func LoadSyncStateForTest(dir string) SyncState { return loadSyncState(dir) }
```

- [ ] **Step 4: Verify tests pass**

```bash
go build ./internal/sync/... && go vet ./internal/sync/...
```

Expected: no errors (tests run in CI).

- [ ] **Step 5: Commit**

```bash
git add internal/sync/state.go internal/sync/state_test.go
git commit -m "feat(sync): add versioned SyncState with migration from flat format"
```

---

## Task 2: Migrate Engine to SyncState + add new methods

**Files:**

- Modify: `internal/sync/engine.go`
- Modify: `internal/sync/engine_test.go`

- [ ] **Step 1: Update engine_test.go to write new format**

`TestEngine_IsStale_TrueWhenExpired` and `TestEngine_IsStale_HonoursPerCategoryTTL` both write the old flat format directly, which will now trigger a reset. Update them to write the new `SyncState` JSON. Add a shared helper at the top of the file:

```go
// writeCategoryTimestamps writes a SyncState with the given category timestamps.
// Use this instead of writing the old flat map[string]time.Time format.
func writeCategoryTimestamps(t *testing.T, dir string, cats map[string]time.Time) {
    t.Helper()
    type syncStateShape struct {
        Version    int                  `json:"version"`
        Categories map[string]time.Time `json:"categories"`
    }
    data, _ := json.Marshal(syncStateShape{Version: 1, Categories: cats})
    require.NoError(t, os.WriteFile(filepath.Join(dir, "sync-state.json"), data, 0600))
}
```

Replace the two direct `json.Marshal(ts)` calls in `TestEngine_IsStale_TrueWhenExpired` and `TestEngine_IsStale_HonoursPerCategoryTTL` with `writeCategoryTimestamps(t, dir, ts)`.

- [ ] **Step 2: Add failing tests for new engine methods**

Add to `internal/sync/engine_test.go` (before the engine.go rewrite — these will fail to build against the old engine since `SetPackHasMarkers` etc. don't exist yet, which is the required red state):

```go
func TestEngine_SetAndGetPackHasMarkers(t *testing.T) {
    dir := t.TempDir()
    eng := sapSync.NewEngine(dir, 24*time.Hour, nil)
    require.NoError(t, eng.SetPackHasMarkers("cap", true))
    packs := eng.PacksBlock()
    require.NotNil(t, packs)
    assert.True(t, packs["cap"].HasMarkers)
}

func TestEngine_RecordAndGetMarkerState(t *testing.T) {
    dir := t.TempDir()
    eng := sapSync.NewEngine(dir, 24*time.Hour, nil)
    ms := sapSync.MarkerState{
        URL:         "https://example.com",
        LastFetched: time.Now(),
        TTLHours:    168,
        OK:          true,
    }
    require.NoError(t, eng.RecordMarkerState("cap", 0, ms))
    got, ok := eng.GetMarkerState("cap", 0)
    require.True(t, ok)
    assert.Equal(t, ms.URL, got.URL)
    assert.True(t, got.OK)
}

func TestEngine_GetMarkerState_NotFound(t *testing.T) {
    dir := t.TempDir()
    eng := sapSync.NewEngine(dir, 24*time.Hour, nil)
    _, ok := eng.GetMarkerState("cap", 0)
    assert.False(t, ok)
}

func TestLoadSyncState_NewFormatRoundTrips(t *testing.T) {
    dir := t.TempDir()
    eng := sapSync.NewEngine(dir, 24*time.Hour, nil)
    require.NoError(t, eng.SetPackHasMarkers("cap", true))
    ms := sapSync.MarkerState{URL: "https://x.com", TTLHours: 168, OK: true, LastFetched: time.Now()}
    require.NoError(t, eng.RecordMarkerState("cap", 0, ms))
    // Reload from disk and verify
    got, ok := eng.GetMarkerState("cap", 0)
    require.True(t, ok)
    assert.Equal(t, ms.URL, got.URL)
    packs := eng.PacksBlock()
    assert.True(t, packs["cap"].HasMarkers)
}
```

- [ ] **Step 2a: Verify tests fail to build** (methods not yet defined in old engine.go)

```bash
go build ./internal/sync/...
```

Expected: compile error referencing missing methods.

- [ ] **Step 3: Rewrite engine.go**

Replace all of `internal/sync/engine.go` with the version below. It removes `loadState`/`saveState` (now in `state.go`) and adds the three new methods. `PacksBlock()` returns nil when no packs have been recorded — callers use `len(packsBlock) == 0` to detect first-run:

```go
package sync

import (
    "time"
)

// Engine tracks sync timestamps and marker state.
type Engine struct {
    stateDir   string
    ttls       map[string]time.Duration
    defaultTTL time.Duration
}

// NewEngine creates an Engine that stores state in stateDir.
func NewEngine(stateDir string, defaultTTL time.Duration, ttls map[string]time.Duration) *Engine {
    return &Engine{stateDir: stateDir, defaultTTL: defaultTTL, ttls: ttls}
}

// IsStale reports whether the given category needs a refresh.
func (e *Engine) IsStale(category string) bool {
    ttl := e.defaultTTL
    if t, ok := e.ttls[category]; ok && t > 0 {
        ttl = t
    }
    state := loadSyncState(e.stateDir)
    last, ok := state.Categories[category]
    if !ok {
        return true
    }
    return time.Since(last) > ttl
}

// MarkSynced records the current time as the last sync time for category.
func (e *Engine) MarkSynced(category string) error {
    state := loadSyncState(e.stateDir)
    state.Categories[category] = time.Now()
    return saveSyncState(e.stateDir, state)
}

// MarkAllSynced records the current time for all given categories in a single write.
func (e *Engine) MarkAllSynced(categories []string) error {
    state := loadSyncState(e.stateDir)
    now := time.Now()
    for _, cat := range categories {
        state.Categories[cat] = now
    }
    return saveSyncState(e.stateDir, state)
}

// SetPackHasMarkers records whether a pack contains sync:fetch markers.
func (e *Engine) SetPackHasMarkers(packID string, hasMarkers bool) error {
    state := loadSyncState(e.stateDir)
    state.Packs[packID] = PackState{HasMarkers: hasMarkers}
    return saveSyncState(e.stateDir, state)
}

// RecordMarkerState persists the result of a marker fetch.
func (e *Engine) RecordMarkerState(packID string, index int, ms MarkerState) error {
    state := loadSyncState(e.stateDir)
    state.Markers[markerKey(packID, index)] = ms
    return saveSyncState(e.stateDir, state)
}

// GetMarkerState retrieves the last fetch result for a marker.
func (e *Engine) GetMarkerState(packID string, index int) (MarkerState, bool) {
    state := loadSyncState(e.stateDir)
    ms, ok := state.Markers[markerKey(packID, index)]
    return ms, ok
}

// PacksBlock returns the full packs map, or nil if no packs have been recorded yet.
func (e *Engine) PacksBlock() map[string]PackState {
    p := loadSyncState(e.stateDir).Packs
    if len(p) == 0 {
        return nil
    }
    return p
}
```

- [ ] **Step 4: Verify**

```bash
go build ./internal/sync/... && go vet ./internal/sync/...
```

Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add internal/sync/engine.go internal/sync/engine_test.go
git commit -m "feat(sync): migrate Engine to SyncState; add SetPackHasMarkers, RecordMarkerState, GetMarkerState"
```

---

## Task 3: Marker scanner

**Files:**

- Create: `internal/sync/marker.go` (scanner only — fetcher added in Task 4)
- Create: `internal/sync/marker_test.go`

- [ ] **Step 1: Write failing tests for the scanner**

Create `internal/sync/marker_test.go`:

```go
package sync_test

import (
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    sapSync "github.tools.sap/developer-relations/sap-devs-cli/internal/sync"
)

func TestScanMarkers_Basic(t *testing.T) {
    content := `## Section
<!-- sync:fetch url="https://example.com/notes" max_lines="50" label="Release Notes" -->
## Other
`
    markers, warns := sapSync.ScanMarkers("cap", content)
    require.Empty(t, warns)
    require.Len(t, markers, 1)
    assert.Equal(t, "cap", markers[0].PackID)
    assert.Equal(t, 0, markers[0].Index)
    assert.Equal(t, "https://example.com/notes", markers[0].URL)
    assert.Equal(t, 50, markers[0].MaxLines)
    assert.Equal(t, "Release Notes", markers[0].Label)
}

func TestScanMarkers_SkipsInsideCodeFence(t *testing.T) {
    content := "```markdown\n<!-- sync:fetch url=\"https://example.com\" -->\n```\n"
    markers, warns := sapSync.ScanMarkers("cap", content)
    assert.Empty(t, warns)
    assert.Empty(t, markers)
}

func TestScanMarkers_MalformedMissingURL(t *testing.T) {
    content := `<!-- sync:fetch max_lines="10" -->` + "\n"
    markers, warns := sapSync.ScanMarkers("cap", content)
    assert.Empty(t, markers)
    require.Len(t, warns, 1)
    assert.Contains(t, warns[0], "missing required 'url'")
}

func TestScanMarkers_BothBudgetsWarnMaxLinesWins(t *testing.T) {
    content := `<!-- sync:fetch url="https://x.com" max_lines="20" max_tokens="500" -->` + "\n"
    markers, warns := sapSync.ScanMarkers("cap", content)
    require.Len(t, markers, 1)
    assert.Equal(t, 20, markers[0].MaxLines)
    require.Len(t, warns, 1)
    assert.Contains(t, warns[0], "max_lines takes precedence")
}

func TestScanMarkers_MultipleMarkers(t *testing.T) {
    content := "<!-- sync:fetch url=\"https://a.com\" -->\n## Mid\n<!-- sync:fetch url=\"https://b.com\" -->\n"
    markers, warns := sapSync.ScanMarkers("cap", content)
    assert.Empty(t, warns)
    require.Len(t, markers, 2)
    assert.Equal(t, 0, markers[0].Index)
    assert.Equal(t, 1, markers[1].Index)
    assert.Equal(t, "https://a.com", markers[0].URL)
    assert.Equal(t, "https://b.com", markers[1].URL)
}

func TestExpandMarkers_ReplacesAtPosition(t *testing.T) {
    content := "## Before\n<!-- sync:fetch url=\"https://x.com\" -->\n## After\n"
    markers, _ := sapSync.ScanMarkers("cap", content)
    results := map[int]string{0: "Fetched content here"}
    expanded := sapSync.ExpandMarkers(content, markers, results)
    assert.Contains(t, expanded, "Fetched content here")
    assert.Contains(t, expanded, "## Before")
    assert.Contains(t, expanded, "## After")
    assert.NotContains(t, expanded, "sync:fetch")
}

func TestExpandMarkers_SkipsInsideCodeFence(t *testing.T) {
    content := "```\n<!-- sync:fetch url=\"https://x.com\" -->\n```\n"
    markers, _ := sapSync.ScanMarkers("cap", content)
    results := map[int]string{0: "should not appear"}
    expanded := sapSync.ExpandMarkers(content, markers, results)
    // No markers found (inside fence) → no substitution
    assert.NotContains(t, expanded, "should not appear")
}
```

- [ ] **Step 2: Create `internal/sync/marker.go`** (scanner + expander; fetcher added next task)

```go
package sync

import (
    "fmt"
    "regexp"
    "strconv"
    "strings"
)

// Marker represents a parsed <!-- sync:fetch ... --> directive.
type Marker struct {
    PackID    string
    Index     int    // zero-based position in the file
    URL       string
    MaxLines  int    // 0 = no limit
    MaxTokens int    // 0 = no limit; MaxLines takes precedence when both set
    Label     string
    TTLHours  int // 0 = use pack/engine default
    LineNum   int
}

var markerRE = regexp.MustCompile(`<!--\s*sync:fetch\s+(.*?)\s*-->`)
var attrRE = regexp.MustCompile(`(\w+)="([^"]*)"`)

// ScanMarkers parses content for sync:fetch markers.
// Markers inside fenced code blocks (``` delimiters) are skipped.
// Returns parsed markers and any parse warnings (not errors — sync continues regardless).
func ScanMarkers(packID, content string) ([]Marker, []string) {
    var markers []Marker
    var warnings []string

    lines := strings.Split(content, "\n")
    inFence := false
    index := 0

    for lineNum, line := range lines {
        if strings.HasPrefix(strings.TrimSpace(line), "```") {
            inFence = !inFence
            continue
        }
        if inFence {
            continue
        }
        match := markerRE.FindStringSubmatch(line)
        if match == nil {
            continue
        }
        attrs := parseAttrs(match[1])
        url := attrs["url"]
        if url == "" {
            warnings = append(warnings, fmt.Sprintf(
                "%s: line %d: sync:fetch missing required 'url' attribute", packID, lineNum+1,
            ))
            continue
        }
        m := Marker{
            PackID:  packID,
            Index:   index,
            URL:     url,
            Label:   attrs["label"],
            LineNum: lineNum + 1,
        }
        if v := attrs["max_lines"]; v != "" {
            m.MaxLines, _ = strconv.Atoi(v)
        }
        if v := attrs["max_tokens"]; v != "" {
            m.MaxTokens, _ = strconv.Atoi(v)
        }
        if m.MaxLines > 0 && m.MaxTokens > 0 {
            warnings = append(warnings, fmt.Sprintf(
                "%s: line %d: both max_lines and max_tokens set; max_lines takes precedence", packID, lineNum+1,
            ))
        }
        if v := attrs["ttl_hours"]; v != "" {
            m.TTLHours, _ = strconv.Atoi(v)
        }
        markers = append(markers, m)
        index++
    }
    return markers, warnings
}

// ExpandMarkers substitutes sync:fetch marker lines in content with fetched results.
// results maps marker.Index → replacement string. Markers with no result entry are left unchanged.
func ExpandMarkers(content string, markers []Marker, results map[int]string) string {
    if len(markers) == 0 {
        return content
    }
    // Build a line-number → marker index map for O(1) lookup.
    lineToMarker := make(map[int]int, len(markers))
    for _, m := range markers {
        lineToMarker[m.LineNum-1] = m.Index // LineNum is 1-based
    }

    lines := strings.Split(content, "\n")
    inFence := false
    for i, line := range lines {
        if strings.HasPrefix(strings.TrimSpace(line), "```") {
            inFence = !inFence
            continue
        }
        if inFence {
            continue
        }
        if idx, ok := lineToMarker[i]; ok {
            if fetched, hasResult := results[idx]; hasResult {
                lines[i] = fetched
            }
        }
    }
    return strings.Join(lines, "\n")
}

func parseAttrs(s string) map[string]string {
    attrs := make(map[string]string)
    for _, m := range attrRE.FindAllStringSubmatch(s, -1) {
        attrs[m[1]] = m[2]
    }
    return attrs
}
```

- [ ] **Step 3: Verify**

```bash
go build ./internal/sync/... && go vet ./internal/sync/...
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add internal/sync/marker.go internal/sync/marker_test.go
git commit -m "feat(sync): add marker scanner and expander (ScanMarkers, ExpandMarkers)"
```

---

## Task 4: Marker fetcher

**Files:**

- Modify: `internal/sync/marker.go` (add `FetchMarker`, `truncateLines`, `truncateTokens`)
- Modify: `internal/sync/marker_test.go` (add HTTP fetch tests)

- [ ] **Step 1: Write failing fetch tests**

Add `"fmt"`, `"net/http"`, `"net/http/httptest"`, and `"strings"` to the existing import block at the top of `internal/sync/marker_test.go` (the one already containing `"testing"`, `assert`, `require`, and `sapSync`). Do not add a second import block — Go only allows one per file.

Then add to `internal/sync/marker_test.go`:

```go
func TestFetchMarker_Success(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        for i := 0; i < 10; i++ {
            fmt.Fprintf(w, "line %d\n", i+1)
        }
    }))
    defer srv.Close()

    m := sapSync.Marker{URL: srv.URL, MaxLines: 5}
    content, err := sapSync.FetchMarker(m, srv.Client())
    require.NoError(t, err)
    lines := strings.Split(strings.TrimSpace(content), "\n")
    assert.Len(t, lines, 5)
    assert.Equal(t, "line 1", lines[0])
}

func TestFetchMarker_Non200(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusNotFound)
    }))
    defer srv.Close()

    m := sapSync.Marker{URL: srv.URL}
    _, err := sapSync.FetchMarker(m, srv.Client())
    require.Error(t, err)
    assert.Contains(t, err.Error(), "404")
}

func TestFetchMarker_NoLimit(t *testing.T) {
    body := "line1\nline2\nline3\n"
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
        fmt.Fprint(w, body)
    }))
    defer srv.Close()

    m := sapSync.Marker{URL: srv.URL} // no max_lines
    content, err := sapSync.FetchMarker(m, srv.Client())
    require.NoError(t, err)
    assert.Equal(t, body, content)
}
```

- [ ] **Step 2: Add `FetchMarker` and truncation helpers to `marker.go`**

Merge `"fmt"`, `"io"`, `"net/http"`, and `"time"` into the existing `import` block at the top of `marker.go`. Then append after `parseAttrs`:

```go
// FetchMarker fetches m.URL and returns the content, truncated per m.MaxLines / m.MaxTokens.
// client may be nil; a default 10-second timeout client is used in that case.
func FetchMarker(m Marker, client *http.Client) (string, error) {
    if client == nil {
        client = &http.Client{Timeout: 10 * time.Second}
    }
    resp, err := client.Get(m.URL) //nolint:gosec // URL comes from pack author, not untrusted input
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()
    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        return "", fmt.Errorf("HTTP %d fetching %s", resp.StatusCode, m.URL)
    }
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return "", err
    }
    content := string(body)
    if m.MaxLines > 0 {
        content = truncateLines(content, m.MaxLines)
    } else if m.MaxTokens > 0 {
        content = truncateTokens(content, m.MaxTokens)
    }
    return content, nil
}

func truncateLines(s string, max int) string {
    lines := strings.SplitN(s, "\n", max+1)
    if len(lines) > max {
        lines = lines[:max]
    }
    return strings.Join(lines, "\n")
}

func truncateTokens(s string, max int) string {
    // Rough approximation: 1 token ≈ 4 characters.
    limit := max * 4
    if len(s) <= limit {
        return s
    }
    s = s[:limit]
    if idx := strings.LastIndex(s, "\n"); idx > 0 {
        s = s[:idx]
    }
    return s
}
```

- [ ] **Step 3: Verify**

```bash
go build ./internal/sync/... && go vet ./internal/sync/...
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add internal/sync/marker.go internal/sync/marker_test.go
git commit -m "feat(sync): add FetchMarker with line/token truncation"
```

---

## Task 5: Add Bubbletea dependency + progress UI

**Files:**

- Modify: `go.mod`, `go.sum`
- Create: `internal/ui/progress.go`

- [ ] **Step 1: Add Bubbletea**

```bash
go get github.com/charmbracelet/bubbletea
```

Expected: `go.mod` updated with `require github.com/charmbracelet/bubbletea v1.x.x`.

- [ ] **Step 2: Create `internal/ui/progress.go`**

This is the Bubbletea inline progress model for marker expansion. It receives `MarkerDoneMsg` from concurrent fetch goroutines via `program.Send()` and renders a live-updating list.

```go
package ui

import (
    "fmt"
    "strings"
    "sync"

    tea "github.com/charmbracelet/bubbletea"
    sapSync "github.tools.sap/developer-relations/sap-devs-cli/internal/sync"
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
            if item.index == msg.Index {
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
// inline progress display, and returns results (index → content) and any fetch errors.
// If markers is empty it returns immediately with no output.
func RunMarkerExpansion(markers []sapSync.Marker) (map[int]string, map[int]error) {
    if len(markers) == 0 {
        return nil, nil
    }

    results := make(map[int]string)
    errs := make(map[int]error)
    var mu sync.Mutex

    model := newProgressModel(markers)
    p := tea.NewProgram(model, tea.WithoutAltScreen())

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
                errs[m.Index] = err
                mu.Unlock()
                p.Send(MarkerDoneMsg{PackID: m.PackID, Index: m.Index, Label: label, Err: err})
                return
            }
            mu.Lock()
            results[m.Index] = content
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
        fmt.Printf("progress display error: %v\n", err)
    }

    return results, errs
}
```

- [ ] **Step 3: Verify**

```bash
go build ./internal/ui/... && go vet ./internal/ui/...
```

Expected: no errors. (No tests for Bubbletea model — UI is presentational and hard to unit test.)

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum internal/ui/progress.go
git commit -m "feat(ui): add Bubbletea inline progress model for marker expansion"
```

---

## Task 6: Phase 2 sync integration

**Files:**

- Modify: `cmd/sync.go`

- [ ] **Step 1: Rewrite `cmd/sync.go`**

Extract `runSync` as a package-level helper. Wire Phase 2 after the archive fetch. The sync command's `RunE` becomes a thin wrapper.

```go
package cmd

import (
    "context"
    "fmt"
    "io"
    "os"
    "path/filepath"
    "strings"
    "time"

    "github.com/spf13/cobra"
    "github.tools.sap/developer-relations/sap-devs-cli/internal/config"
    "github.tools.sap/developer-relations/sap-devs-cli/internal/credentials"
    "github.tools.sap/developer-relations/sap-devs-cli/internal/i18n"
    sapSync "github.tools.sap/developer-relations/sap-devs-cli/internal/sync"
    "github.tools.sap/developer-relations/sap-devs-cli/internal/ui"
    "github.tools.sap/developer-relations/sap-devs-cli/internal/xdg"
)

const officialRepoArchive = "https://github.tools.sap/developer-relations/sap-devs-cli/archive/refs/heads/main.zip"

var syncForce bool
var syncCategory string

var syncCmd = &cobra.Command{
    Use:   "sync",
    Short: "Pull latest SAP developer content",
    Long:  `Syncs content from the official repo (and company repo if configured). Respects per-category TTLs unless --force is set.`,
    RunE: func(cmd *cobra.Command, args []string) error {
        return runSync(cmd.Context(), syncForce, cmd.OutOrStdout())
    },
}

// runSync is the shared sync implementation used by both the sync command and inline inject sync.
// out receives all progress messages; pass cmd.OutOrStdout() or os.Stdout as appropriate.
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
    // Apply --category filter when called directly from syncCmd (syncCategory is set by the flag)
    if syncCategory != "" {
        categories = []string{syncCategory}
    }

    officialCache := filepath.Join(paths.CacheDir, "official")
    ttls := map[string]time.Duration{
        "tips": cfg.Sync.Tips, "tools": cfg.Sync.Tools,
        "advocates": cfg.Sync.Advocates, "resources": cfg.Sync.Resources,
        "context": cfg.Sync.Context, "mcp": cfg.Sync.MCP,
    }
    engine := sapSync.NewEngine(paths.CacheDir, 24*time.Hour, ttls)

    // Phase 1: archive download (existing behaviour, fmt output)
    needsSync := false
    for _, cat := range categories {
        if force || engine.IsStale(cat) {
            needsSync = true
            break
        }
    }
    if !needsSync {
        fmt.Fprintln(out, i18n.T(i18n.ActiveLang, "sync.up_to_date"))
        return nil
    }

    fmt.Fprintln(out, i18n.T(i18n.ActiveLang, "sync.syncing"))
    if err := sapSync.FetchArchive(officialRepoArchive, officialCache, token); err != nil {
        return fmt.Errorf("sync official content: %w", err)
    }
    if err := engine.MarkAllSynced(categories); err != nil {
        return err
    }
    fmt.Fprintln(out, i18n.Tf(i18n.ActiveLang, "sync.updated", map[string]any{"Categories": categories}))

    // Phase 2: marker expansion (Bubbletea progress)
    if err := runMarkerExpansion(officialCache, engine); err != nil {
        fmt.Fprintf(os.Stderr, "sap-devs: marker expansion warning: %v\n", err)
        // Non-fatal: sync continues
    }

    // Sync company repo if configured
    if cfg.CompanyRepo != "" {
        if !strings.HasPrefix(cfg.CompanyRepo, "https://") {
            fmt.Fprintln(out, i18n.Tf(i18n.ActiveLang, "sync.warn_https", map[string]any{"URL": cfg.CompanyRepo}))
        } else {
            companyCache := filepath.Join(paths.CacheDir, "company")
            repoURL := strings.TrimRight(cfg.CompanyRepo, "/")
            companyArchive := repoURL + "/archive/refs/heads/main.zip"
            fmt.Fprintln(out, i18n.T(i18n.ActiveLang, "sync.syncing_company"))
            if err := sapSync.FetchArchive(companyArchive, companyCache, token); err != nil {
                fmt.Fprintln(out, i18n.Tf(i18n.ActiveLang, "sync.warn_company_failed", map[string]any{"Err": err}))
            }
        }
    }
    return nil
}

// runMarkerExpansion scans all official-layer packs for sync:fetch markers,
// fetches them in parallel with a Bubbletea progress display, and writes
// context.expanded.md alongside each context.md.
func runMarkerExpansion(officialCache string, engine *sapSync.Engine) error {
    packsDir := filepath.Join(officialCache, "content", "packs")
    entries, err := os.ReadDir(packsDir)
    if err != nil {
        return nil // No packs directory yet — first run before archive fetch
    }

    var allMarkers []sapSync.Marker
    packContexts := make(map[string]string) // packID → context.md content

    for _, entry := range entries {
        if !entry.IsDir() {
            continue
        }
        packID := entry.Name()
        contextPath := filepath.Join(packsDir, packID, "context.md")
        data, err := os.ReadFile(contextPath)
        if err != nil {
            continue
        }
        contextContent := string(data)
        markers, warns := sapSync.ScanMarkers(packID, contextContent)
        for _, w := range warns {
            fmt.Fprintf(os.Stderr, "sap-devs: %s\n", w)
        }
        hasMarkers := len(markers) > 0
        if err := engine.SetPackHasMarkers(packID, hasMarkers); err != nil {
            return err
        }
        if hasMarkers {
            packContexts[packID] = contextContent
            allMarkers = append(allMarkers, markers...)
        }
    }

    if len(allMarkers) == 0 {
        return nil
    }

    // Fetch all markers in parallel with progress display
    results, fetchErrs := ui.RunMarkerExpansion(allMarkers)

    // Record marker states and write expanded files
    for packID, contextContent := range packContexts {
        // Collect markers for this pack
        var packMarkers []sapSync.Marker
        for _, m := range allMarkers {
            if m.PackID == packID {
                packMarkers = append(packMarkers, m)
            }
        }

        // Record state for each marker
        for _, m := range packMarkers {
            ms := sapSync.MarkerState{
                URL:      m.URL,
                TTLHours: m.TTLHours,
                OK:       fetchErrs[m.Index] == nil,
            }
            if ms.OK {
                ms.LastFetched = time.Now()
            }
            if err := engine.RecordMarkerState(packID, m.Index, ms); err != nil {
                return err
            }
        }

        // Expand and write context.expanded.md
        expanded := sapSync.ExpandMarkers(contextContent, packMarkers, results)
        expandedPath := filepath.Join(packsDir, packID, "context.expanded.md")
        if err := os.WriteFile(expandedPath, []byte(expanded), 0644); err != nil {
            return fmt.Errorf("write %s: %w", expandedPath, err)
        }
    }
    return nil
}

func allCategories() []string {
    return []string{"tips", "tools", "resources", "context", "mcp", "advocates"}
}

func init() {
    syncCmd.Flags().BoolVar(&syncForce, "force", false, "Re-sync all categories regardless of TTL")
    syncCmd.Flags().StringVar(&syncCategory, "category", "", "Sync a single category only")
    rootCmd.AddCommand(syncCmd)
}
```

- [ ] **Step 2: Verify**

```bash
go build ./... && go vet ./...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add cmd/sync.go
git commit -m "feat(sync): extract runSync helper; add Phase 2 marker expansion with Bubbletea progress"
```

---

## Task 7: LoadPack expanded file preference

**Files:**

- Modify: `internal/content/pack.go` (lines 124–131)
- Modify: `internal/content/pack_test.go`

- [ ] **Step 1: Write failing tests**

Add to `internal/content/pack_test.go`:

```go
func TestLoadPack_PrefersExpandedOverBase(t *testing.T) {
    dir := t.TempDir()
    yaml := "id: cap\nname: CAP\ndescription: Test\ntags: []\nprofiles: []\nweight: 0\n"
    require.NoError(t, os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(yaml), 0644))
    require.NoError(t, os.WriteFile(filepath.Join(dir, "context.md"), []byte("static content"), 0644))
    require.NoError(t, os.WriteFile(filepath.Join(dir, "context.expanded.md"), []byte("expanded content"), 0644))

    p, err := content.LoadPack(dir, "")
    require.NoError(t, err)
    assert.Equal(t, "expanded content", p.ContextMD)
}

func TestLoadPack_FallsBackToBaseWhenNoExpanded(t *testing.T) {
    dir := t.TempDir()
    yaml := "id: cap\nname: CAP\ndescription: Test\ntags: []\nprofiles: []\nweight: 0\n"
    require.NoError(t, os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(yaml), 0644))
    require.NoError(t, os.WriteFile(filepath.Join(dir, "context.md"), []byte("static content"), 0644))

    p, err := content.LoadPack(dir, "")
    require.NoError(t, err)
    assert.Equal(t, "static content", p.ContextMD)
}

func TestLoadPack_LocaleBeatsExpanded(t *testing.T) {
    dir := t.TempDir()
    yaml := "id: cap\nname: CAP\ndescription: Test\ntags: []\nprofiles: []\nweight: 0\n"
    require.NoError(t, os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(yaml), 0644))
    require.NoError(t, os.WriteFile(filepath.Join(dir, "context.md"), []byte("base"), 0644))
    require.NoError(t, os.WriteFile(filepath.Join(dir, "context.de.md"), []byte("german"), 0644))
    require.NoError(t, os.WriteFile(filepath.Join(dir, "context.expanded.md"), []byte("expanded"), 0644))

    // German locale wins over expanded
    p, err := content.LoadPack(dir, "de")
    require.NoError(t, err)
    assert.Equal(t, "german", p.ContextMD)

    // No locale → expanded wins
    p, err = content.LoadPack(dir, "")
    require.NoError(t, err)
    assert.Equal(t, "expanded", p.ContextMD)
}
```

- [ ] **Step 2: Run to verify tests currently fail**

```bash
go build ./internal/content/... && go vet ./internal/content/...
```

The new tests compile but will fail in CI (expanded content not yet preferred).

- [ ] **Step 3: Update `LoadPack` in `internal/content/pack.go`**

Replace lines 124–132 (the `contextFile` selection block):

```go
// Context file: locale variant → expanded base → base
contextFile := filepath.Join(packDir, "context.md")
localeFound := false
if lang != "" && lang != "en" {
    if loc := filepath.Join(packDir, "context."+lang+".md"); fileExists(loc) {
        contextFile = loc
        localeFound = true
    }
}
// If no locale variant selected, prefer the sync-expanded file when present.
if !localeFound {
    if exp := filepath.Join(packDir, "context.expanded.md"); fileExists(exp) {
        contextFile = exp
    }
}
if data, err := os.ReadFile(contextFile); err == nil {
    pack.ContextMD = string(data)
}
```

- [ ] **Step 4: Verify**

```bash
go build ./... && go vet ./...
```

Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add internal/content/pack.go internal/content/pack_test.go
git commit -m "feat(content): LoadPack prefers context.expanded.md over context.md when present"
```

---

## Task 8: Pre-inject staleness check

**Files:**

- Modify: `cmd/inject.go`

- [ ] **Step 1: Write the updated `cmd/inject.go`**

Add `--sync`/`--no-sync` flags, the staleness check, and packs reload after inline sync:

```go
package cmd

import (
    "fmt"
    "os"
    "path/filepath"
    "time"

    "github.com/spf13/cobra"
    "golang.org/x/term"
    "github.tools.sap/developer-relations/sap-devs-cli/internal/adapter"
    "github.tools.sap/developer-relations/sap-devs-cli/internal/config"
    "github.tools.sap/developer-relations/sap-devs-cli/internal/content"
    "github.tools.sap/developer-relations/sap-devs-cli/internal/i18n"
    sapSync "github.tools.sap/developer-relations/sap-devs-cli/internal/sync"
    "github.tools.sap/developer-relations/sap-devs-cli/internal/xdg"
)

var (
    injectProject bool
    injectTool    string
    injectDryRun  bool
    injectSync    bool
    injectNoSync  bool
)

var injectCmd = &cobra.Command{
    Use:   "inject",
    Short: "Push SAP context to your AI tools",
    Long: `Inject up-to-date SAP developer context into all detected AI tools.

Injects at global (user) scope by default. Use --project for project scope.`,
    RunE: func(cmd *cobra.Command, args []string) error {
        if injectSync && injectNoSync {
            return fmt.Errorf("--sync and --no-sync are mutually exclusive")
        }

        scope := "global"
        if injectProject {
            scope = "project"
        }

        loader, err := newContentLoader()
        if err != nil {
            return err
        }

        paths, err := xdg.New()
        if err != nil {
            return err
        }
        configProfile, err := config.LoadProfile(paths.ConfigDir)
        if err != nil {
            return err
        }

        var activeProfile *content.Profile
        if configProfile.ID != "" {
            activeProfile, err = loader.FindProfile(configProfile.ID)
            if err != nil {
                return err
            }
            if activeProfile == nil {
                return fmt.Errorf("profile %q not found in any content layer", configProfile.ID)
            }
        }

        packs, err := loader.LoadPacks(activeProfile, i18n.ActiveLang)
        if err != nil {
            return err
        }

        // Staleness check — skip if --no-sync, run unconditionally if --sync
        if !injectNoSync {
            engine := sapSync.NewEngine(paths.CacheDir, 24*time.Hour, nil)
            if injectSync || isStaleDynamicContent(engine, packs, paths) {
                if injectSync || shouldSyncNow(cmd) {
                    if err := runSync(cmd.Context(), false, cmd.OutOrStdout()); err != nil {
                        fmt.Fprintf(os.Stderr, "sap-devs: sync failed: %v\n", err)
                        // Non-fatal: continue with cached content
                    } else {
                        // Reload packs to pick up newly expanded content
                        packs, err = loader.LoadPacks(activeProfile, i18n.ActiveLang)
                        if err != nil {
                            return err
                        }
                    }
                }
            }
        }

        rendered := content.RenderContext(packs, activeProfile)

        opts := adapter.Options{
            Scope:      scope,
            ToolFilter: injectTool,
            DryRun:     injectDryRun,
        }
        eng, err := newAdapterEngine(rendered, opts)
        if err != nil {
            return err
        }

        if injectDryRun {
            fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "inject.dry_run"))
        }
        if err := eng.Run(); err != nil {
            return err
        }
        if !injectDryRun {
            fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "inject.done", map[string]any{"Scope": scope}))
            if injectTool == "" {
                fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "inject.hint"))
            }
        }
        return nil
    },
}

// isStaleDynamicContent returns true if any active pack has markers and its expanded
// content is missing, failed on last fetch, or past TTL.
// paths is used to locate context.expanded.md in the official cache.
func isStaleDynamicContent(engine *sapSync.Engine, packs []*content.Pack, paths *xdg.Paths) bool {
    packsBlock := engine.PacksBlock()
    if packsBlock == nil {
        // No packs block yet — treat as potentially stale
        return len(packs) > 0
    }
    for _, p := range packs {
        ps, known := packsBlock[p.ID]
        if !known || !ps.HasMarkers {
            continue
        }
        // Condition 1: context.expanded.md must exist
        expandedPath := filepath.Join(paths.CacheDir, "official", "content", "packs", p.ID, "context.expanded.md")
        if _, err := os.Stat(expandedPath); os.IsNotExist(err) {
            return true
        }
        // Conditions 2+3: iterate recorded marker states until no more found
        for i := 0; ; i++ {
            ms, ok := engine.GetMarkerState(p.ID, i)
            if !ok {
                break // no more markers recorded for this pack
            }
            if !ms.OK {
                return true
            }
            ttl := time.Duration(ms.TTLHours) * time.Hour
            if ttl <= 0 {
                ttl = 7 * 24 * time.Hour // default 7-day TTL for markers
            }
            if time.Since(ms.LastFetched) > ttl {
                return true
            }
        }
    }
    return false
}

// shouldSyncNow prompts the user interactively. Returns true if user answers Y or is non-TTY auto-proceed.
// Non-TTY: auto-proceeds with cached content (returns false) and warns to stderr.
func shouldSyncNow(cmd *cobra.Command) bool {
    if !term.IsTerminal(int(os.Stdin.Fd())) {
        fmt.Fprintln(os.Stderr, `sap-devs: dynamic content is stale; run "sap-devs sync" to refresh`)
        return false
    }
    fmt.Fprint(cmd.OutOrStdout(), "  Dynamic content may be stale. Sync now for latest content? [Y/n] ")
    var answer string
    fmt.Fscan(os.Stdin, &answer)
    return answer == "" || answer == "Y" || answer == "y"
}

func init() {
    injectCmd.Flags().BoolVar(&injectProject, "project", false, "inject at project scope (current directory)")
    injectCmd.Flags().StringVar(&injectTool, "tool", "", "inject into a specific tool only (e.g. claude-code)")
    injectCmd.Flags().BoolVar(&injectDryRun, "dry-run", false, "preview changes without writing files")
    injectCmd.Flags().BoolVar(&injectSync, "sync", false, "sync dynamic content before injecting (no prompt)")
    injectCmd.Flags().BoolVar(&injectNoSync, "no-sync", false, "skip freshness check; use cached content as-is")
    rootCmd.AddCommand(injectCmd)
}
```

- [ ] **Step 2: Verify**

```bash
go build ./... && go vet ./...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add cmd/inject.go
git commit -m "feat(inject): add staleness check, --sync/--no-sync flags, and inline sync with pack reload"
```

---

## Task 9: CAP pack — add first marker and agent instructions

**Files:**

- Modify: `content/packs/cap/context.md`

- [ ] **Step 1: Update `content/packs/cap/context.md`**

Add the `sync:fetch` marker and `### Agent Instructions` section:

````markdown
## SAP CAP (Cloud Application Programming Model)

CAP is SAP's primary framework for building cloud-native business applications on SAP BTP.
It uses CDS (Core Data Services) for data and service definitions, Node.js or Java for service logic.

### Key Tools
- `@sap/cds-dk` — CAP development kit (CLI: `cds`)
- `cds watch` — local dev server with live reload
- `cds deploy` — deploy to database / cloud

### CDS Data Modelling
```cds
entity Books : managed {
  key ID     : Integer;
  title      : localized String(111);
  author     : Association to Authors;
}
```

### Service Definition

```cds
service CatalogService @(path:'/browse') {
  @readonly entity Books as SELECT from my.Books;
}
```

### Best Practices

- Define entities in `db/schema.cds`, services in `srv/*.cds`
- Use `cds.ql` for type-safe CQL queries
- Leverage built-in authentication via `@requires` annotations
- Always run `cds lint` before committing

### Recent CAP Releases

<!-- sync:fetch url="https://cap.cloud.sap/docs/releases/2026/feb26" max_lines="80" label="CAP Release Notes (feb26)" -->

### Agent Instructions

This CLI provides deeper SAP context on demand — prefer these over web searches for SAP-specific information:

- `sap-devs resources --pack cap` — curated CAP docs, samples, and tutorials
- `sap-devs tip --pack cap` — CAP best practice tips
- `sap-devs sync` — refresh with latest CAP release notes and dynamic content
````

- [ ] **Step 2: Verify build**

```bash
go build ./... && go vet ./...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add content/packs/cap/context.md
git commit -m "feat(content/cap): add feb26 release notes marker and agent instructions"
```

---

## Task 10: Documentation

**Files:**

- Create: `docs/content-authoring.md`

- [ ] **Step 1: Create `docs/content-authoring.md`**

Write a guide covering:

- Pack directory structure
- Marker syntax: single-line `<!-- sync:fetch url="..." ... -->` with all attributes and their effects
- Parser rules: single-line only, skipped inside fenced code blocks, malformed handling
- Failure behaviour: fetch fails → marker left unchanged, previous cache preserved, sync continues
- Token budget guidance: use `max_lines` for release notes (60–100 lines recommended), `max_tokens` for longer docs
- `### Agent Instructions` pattern: what it is, example, why it's a second tier
- Testing a new marker locally: `sap-devs sync --force` then `sap-devs inject --dry-run`

The guide should be practical and focused. ~200 lines.

- [ ] **Step 2: Verify**

```bash
go build ./... && go vet ./...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add docs/content-authoring.md
git commit -m "docs: add content-authoring guide covering markers, agent instructions, and token budget"
```

---

## Final verification

- [ ] **Full build and vet**

```bash
go build ./... && go vet ./...
```

Expected: clean.

- [ ] **Manual smoke test**

```bash
# Force a sync to trigger Phase 2 marker expansion
SAP_DEVS_DEV=1 go run . sync --force

# Check context.expanded.md was written (adjust path for your OS)
# Windows: %LOCALAPPDATA%\sap-devs\cache\official\content\packs\cap\context.expanded.md
# Linux:   ~/.cache/sap-devs/official/content/packs/cap/context.expanded.md

# Dry-run inject to see expanded content in rendered output
SAP_DEVS_DEV=1 go run . inject --dry-run --no-sync
```

Expected: `context.expanded.md` contains the fetched CAP release notes in place of the marker comment; dry-run inject output includes the release notes content.

- [ ] **Final commit**

```bash
git add -A
git commit -m "chore: final cleanup and smoke-test verification"
```
