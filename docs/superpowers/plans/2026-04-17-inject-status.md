# inject --status Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `sap-devs inject --status` to report whether SAP context is present, well-formed, and current in each AI tool config file, with an optional `--verbose` table and `--json` output.

**Architecture:** A new `Status()` method on `Engine` (Option A from the design) iterates `file-inject` adapters, reads each target file, calls the existing `findSection` helper to detect markers, renders current content via a new `renderSectionContent` helper for staleness comparison, and returns `[]StatusRow`. The command layer formats rows as a tabwriter table or JSON. A new `internal/adapter/status.go` holds the types and helpers; `engine.go` gets the `Status()` and `renderSectionContent` methods.

**Tech Stack:** Go stdlib only (`os`, `strings`, `regexp`, `encoding/json`, `text/tabwriter`); reuses `findSection`, `ExpandHome`, `content.TrimPacks`, `content.RenderContext`, `content.FormatOutput` already in the repo.

---

## File Map

| File | Change |
|---|---|
| `internal/adapter/status.go` | **Create** — `StatusRow`, `SectionInfo`, `estimateTokens`, `scanOtherSections` |
| `internal/adapter/engine.go` | **Modify** — add `renderSectionContent` method + `Status()` method |
| `internal/adapter/status_test.go` | **Create** — integration tests for `Status()` and helpers |
| `internal/i18n/catalogs/en.json` | **Modify** — add `inject.status.*` keys |
| `internal/i18n/catalogs/de.json` | **Modify** — add `inject.status.*` keys (German) |
| `cmd/inject.go` | **Modify** — add `--status`, `--json`, `--verbose` flags and early-return block |
| `cmd/inject_status_test.go` | **Create** — cmd-level tests (white-box, `package cmd`) |

---

## Key Context for Every Task

**Module path:** `github.tools.sap/developer-relations/sap-devs-cli`

**Working directory for all commands:** the worktree root (e.g. `.worktrees/feat/inject-status/`)

**Build/vet (no `go test` on Windows):**
```bash
go build ./...
go vet ./...
```

**Run tests (CI authoritative; use on Linux/CI):**
```bash
go test ./internal/adapter/...
go test ./cmd/...
```

**Pattern precedents to follow:**
- `runFileUninstall` in `engine.go` — `errors.Join` collect-all pattern, `e.opts.Lang` for i18n
- `TestEngineUninstall_*` in `adapter_test.go` — how engine integration tests are written (uses `adapter_test` package with `adapter.NewEngine`)
- `TestInjectUninstall_*` in `cmd/inject_uninstall_test.go` — white-box cmd tests using unexported vars

**`findSection` signature (in `file_inject.go`):**
```go
func findSection(content, start, end string) (startIdx, endIdx int, status sectionStatus)
// startIdx/endIdx are byte offsets of the first char of each marker string
// Only meaningful when status == sectionFound
```

**Marker format constants (in `file_inject.go`):**
```go
const markerFmt    = "<!-- sap-devs:start:%s -->"
const markerEndFmt = "<!-- sap-devs:end:%s -->"
```

**`ReplaceFile` writes:** `preamble + "\n" + content` when preamble non-empty, else just `content`

**Render pipeline in `engine.go` `Run()`:**
```go
maxBytes := a.MaxBytes
if maxBytes == 0 && a.MaxTokens > 0 {
    maxBytes = a.MaxTokens * 4
}
trimmed := content.TrimPacks(e.packs, maxBytes)
ctx := content.RenderContext(trimmed, e.profile, e.opts.Dynamic)
formattedCtx := content.FormatOutput(ctx, a.Format)
```

---

## Task 1: Data types and helpers in `status.go`

**Files:**
- Create: `internal/adapter/status.go`
- Test: `internal/adapter/status_test.go` (helper unit tests only in this task)

**What to build:** `StatusRow`, `SectionInfo`, `estimateTokens`, `scanOtherSections`. No engine logic yet.

- [ ] **Step 1: Write the failing tests for helpers**

Create `internal/adapter/status_test.go`:

```go
package adapter_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/adapter"
)

func TestEstimateTokens_Empty(t *testing.T) {
	assert.Equal(t, 0, adapter.EstimateTokens(""))
}

func TestEstimateTokens_KnownString(t *testing.T) {
	// "hello world foo bar" = 4 words → 4 * 13 / 10 = 5
	assert.Equal(t, 5, adapter.EstimateTokens("hello world foo bar"))
}

func TestScanOtherSections_Empty(t *testing.T) {
	result := adapter.ScanOtherSections("")
	assert.NotNil(t, result)
	assert.Empty(t, result)
}

func TestScanOtherSections_IgnoresSapDevs(t *testing.T) {
	content := "<!-- sap-devs:start:SAP Dev -->\nhello\n<!-- sap-devs:end:SAP Dev -->\n"
	result := adapter.ScanOtherSections(content)
	assert.Empty(t, result)
}

func TestScanOtherSections_OneMatch(t *testing.T) {
	content := "<!-- cursor:start:Rules -->\nsome cursor rules here\n<!-- cursor:end:Rules -->\n"
	result := adapter.ScanOtherSections(content)
	assert.Len(t, result, 1)
	assert.Equal(t, "cursor", result[0].Name)
	assert.Greater(t, result[0].Tokens, 0)
}

func TestScanOtherSections_MultipleTools(t *testing.T) {
	content := `<!-- cursor:start:Rules -->
cursor rules
<!-- cursor:end:Rules -->
<!-- copilot:start:Instructions -->
copilot stuff
<!-- copilot:end:Instructions -->
`
	result := adapter.ScanOtherSections(content)
	assert.Len(t, result, 2)
	names := []string{result[0].Name, result[1].Name}
	assert.Contains(t, names, "cursor")
	assert.Contains(t, names, "copilot")
}
```

- [ ] **Step 2: Run tests — verify they fail**

```bash
go test ./internal/adapter/... -run "TestEstimateTokens|TestScanOtherSections" -v
```

Expected: FAIL (adapter.EstimateTokens and adapter.ScanOtherSections undefined)

- [ ] **Step 3: Create `internal/adapter/status.go`**

```go
package adapter

import (
	"regexp"
	"strings"
)

// SectionInfo describes a non-sap-devs fenced block found in a target file.
type SectionInfo struct {
	Name   string `json:"name"`
	Tokens int    `json:"tokens"`
}

// StatusRow is the result of inspecting one adapter target (one row per adapter+target pair).
// An adapter with both a global and a project target produces two StatusRows.
type StatusRow struct {
	AdapterName string `json:"adapter_name"`
	AdapterID   string `json:"adapter"`
	Scope       string `json:"scope"`
	TargetPath  string `json:"path"` // unexpanded (~-form)

	FileExists bool `json:"file_exists"`
	Injected   bool `json:"injected"`  // sap-devs section present and well-formed
	Orphaned   bool `json:"orphaned"`  // markers found but mismatched/reversed

	// Stale is true when the on-disk section content differs from what inject would write today.
	// Always false when FileExists=false, Injected=false, or engine has no packs loaded.
	Stale bool `json:"stale"`

	// Stretch-goal fields — always populated when FileExists=true.
	FileSizeBytes int           `json:"file_size_bytes"`
	FileTokenEst  int           `json:"file_token_est"`  // word count × 1.3
	SapDevsTokens int           `json:"sap_devs_tokens"` // token estimate for sap-devs section only
	OtherSections []SectionInfo `json:"other_sections"`  // non-sap-devs fenced blocks
}

// reOtherSection matches <!-- <prefix>:start:<name> --> where prefix != "sap-devs".
var reOtherSection = regexp.MustCompile(`<!-- ([^:>]+):start:([^>]+) -->`)

// EstimateTokens returns a rough token estimate: word count × 1.3.
// Exported for testing.
func EstimateTokens(s string) int {
	words := len(strings.Fields(s))
	return words * 13 / 10
}

// ScanOtherSections finds non-sap-devs HTML-comment fenced blocks in content.
// Returns []SectionInfo{} (never nil) so it marshals as [] in JSON.
func ScanOtherSections(content string) []SectionInfo {
	result := []SectionInfo{}
	matches := reOtherSection.FindAllStringSubmatchIndex(content, -1)
	for _, m := range matches {
		prefix := content[m[2]:m[3]]
		if prefix == "sap-devs" {
			continue
		}
		// Find matching end marker
		endMarker := "<!-- " + prefix + ":end:"
		startPos := m[1] // position after the start marker
		endPos := strings.Index(content[startPos:], endMarker)
		var tokens int
		if endPos >= 0 {
			inner := content[startPos : startPos+endPos]
			tokens = EstimateTokens(inner)
		}
		result = append(result, SectionInfo{Name: prefix, Tokens: tokens})
	}
	return result
}
```

- [ ] **Step 4: Run tests — verify they pass**

```bash
go test ./internal/adapter/... -run "TestEstimateTokens|TestScanOtherSections" -v
```

Expected: PASS (4 tests)

- [ ] **Step 5: Build and vet**

```bash
go build ./...
go vet ./...
```

Expected: no errors

- [ ] **Step 6: Commit**

```bash
git add internal/adapter/status.go internal/adapter/status_test.go
git commit -m "feat: add StatusRow, SectionInfo, estimateTokens, scanOtherSections"
```

---

## Task 2: `renderSectionContent` helper on Engine

**Files:**
- Modify: `internal/adapter/engine.go`
- Test: `internal/adapter/status_test.go` (add render helper tests)

**What to build:** Private method `renderSectionContent(a Adapter) string` on `*Engine`. It mirrors the render pipeline from `Run()` — TrimPacks + RenderContext + FormatOutput — but returns the string instead of passing it to an inject handler. `Status()` will call this for staleness checks.

- [ ] **Step 1: Add test for renderSectionContent via Status()**

We can't test `renderSectionContent` directly (unexported), but we'll test it indirectly through `TestStatus_Current` and `TestStatus_Stale` in Task 3. For now, add a smoke test that verifies `Status()` doesn't panic when called with packs:

Append to `internal/adapter/status_test.go`:

```go
func makePackWithContent(id, contextMD string) *content.Pack {
	return &content.Pack{
		ID:        id,
		Name:      id,
		ContextMD: contextMD,
	}
}
```

(This helper will be used by multiple tests in Task 3.)

- [ ] **Step 2: Add `renderSectionContent` to `engine.go`**

Add this method after `runFileUninstall` in `internal/adapter/engine.go`:

```go
// renderSectionContent renders the content string that would be written by inject
// for the given adapter. It mirrors the full pipeline in Run(): TrimPacks →
// RenderContext → FormatOutput. Returns "" when e.packs is nil.
func (e *Engine) renderSectionContent(a Adapter) string {
	if e.packs == nil {
		return ""
	}
	maxBytes := a.MaxBytes
	if maxBytes == 0 && a.MaxTokens > 0 {
		maxBytes = a.MaxTokens * 4
	}
	trimmed := content.TrimPacks(e.packs, maxBytes)
	ctx := content.RenderContext(trimmed, e.profile, e.opts.Dynamic)
	return content.FormatOutput(ctx, a.Format)
}
```

Also add the import for `content` if not already present — it already is in `engine.go`.

- [ ] **Step 3: Build and vet**

```bash
go build ./...
go vet ./...
```

Expected: no errors

- [ ] **Step 4: Commit**

```bash
git add internal/adapter/engine.go internal/adapter/status_test.go
git commit -m "feat: add renderSectionContent helper to Engine"
```

---

## Task 3: `Status()` method on Engine

**Files:**
- Modify: `internal/adapter/engine.go`
- Modify: `internal/adapter/status_test.go`

**What to build:** `func (e *Engine) Status() ([]StatusRow, error)`. Iterates `file-inject` adapters, reads target files, populates `StatusRow` per target, detects staleness via `renderSectionContent`, fills stretch-goal fields.

- [ ] **Step 1: Write the failing integration tests**

Append to `internal/adapter/status_test.go` (after the helper tests from Task 1):

```go
import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/adapter"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
)
```

(Note: merge imports with the existing import block at the top of the file.)

Add these test functions:

```go
// writeSectionFile writes a file containing a sap-devs fenced section.
func writeSectionFile(t *testing.T, path, section, inner string) {
	t.Helper()
	data := fmt.Sprintf("<!-- sap-devs:start:%s -->\n%s\n<!-- sap-devs:end:%s -->\n", section, inner, section)
	require.NoError(t, os.WriteFile(path, []byte(data), 0644))
}

func TestStatus_FileNotFound(t *testing.T) {
	dir := t.TempDir()
	targetFile := filepath.Join(dir, "CLAUDE.md") // does not exist

	adapters := []adapter.Adapter{{
		ID:   "test-tool",
		Name: "Test Tool",
		Type: "file-inject",
		Targets: []adapter.Target{
			{Scope: "global", Path: targetFile, Mode: "replace-section", Section: "SAP Dev"},
		},
	}}

	eng := adapter.NewEngine(adapters, nil, nil, adapter.Options{Scope: "global"})
	rows, err := eng.Status()
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, targetFile, rows[0].TargetPath)
	assert.False(t, rows[0].FileExists)
	assert.False(t, rows[0].Injected)
	assert.False(t, rows[0].Stale)
}

func TestStatus_NotInjected(t *testing.T) {
	dir := t.TempDir()
	targetFile := filepath.Join(dir, "CLAUDE.md")
	require.NoError(t, os.WriteFile(targetFile, []byte("# No SAP markers here\n"), 0644))

	adapters := []adapter.Adapter{{
		ID:   "test-tool",
		Name: "Test Tool",
		Type: "file-inject",
		Targets: []adapter.Target{
			{Scope: "global", Path: targetFile, Mode: "replace-section", Section: "SAP Dev"},
		},
	}}

	eng := adapter.NewEngine(adapters, nil, nil, adapter.Options{Scope: "global"})
	rows, err := eng.Status()
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.True(t, rows[0].FileExists)
	assert.False(t, rows[0].Injected)
	assert.Greater(t, rows[0].FileSizeBytes, 0)
}

func TestStatus_Orphaned(t *testing.T) {
	dir := t.TempDir()
	targetFile := filepath.Join(dir, "CLAUDE.md")
	// Start marker only, no end marker
	require.NoError(t, os.WriteFile(targetFile, []byte("<!-- sap-devs:start:SAP Dev -->\norphan\n"), 0644))

	adapters := []adapter.Adapter{{
		ID:   "test-tool",
		Name: "Test Tool",
		Type: "file-inject",
		Targets: []adapter.Target{
			{Scope: "global", Path: targetFile, Mode: "replace-section", Section: "SAP Dev"},
		},
	}}

	eng := adapter.NewEngine(adapters, nil, nil, adapter.Options{Scope: "global"})
	rows, err := eng.Status()
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.True(t, rows[0].FileExists)
	assert.False(t, rows[0].Injected)
	assert.True(t, rows[0].Orphaned)
}

func TestStatus_Current(t *testing.T) {
	dir := t.TempDir()
	targetFile := filepath.Join(dir, "CLAUDE.md")

	pack := makePackWithContent("test-pack", "## CAP\nUse CDS for data models.\n")
	packs := []*content.Pack{pack}

	eng := adapter.NewEngine([]adapter.Adapter{{
		ID:   "test-tool",
		Name: "Test Tool",
		Type: "file-inject",
		Targets: []adapter.Target{
			{Scope: "global", Path: targetFile, Mode: "replace-section", Section: "SAP Dev"},
		},
	}}, packs, nil, adapter.Options{Scope: "global"})

	// Write the file with exactly what renderSectionContent would produce
	rendered := eng.RenderSectionContentForTest(adapter.Adapter{
		ID:   "test-tool",
		Type: "file-inject",
	})
	writeSectionFile(t, targetFile, "SAP Dev", rendered)

	rows, err := eng.Status()
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.True(t, rows[0].Injected)
	assert.False(t, rows[0].Stale)
}

func TestStatus_Stale(t *testing.T) {
	dir := t.TempDir()
	targetFile := filepath.Join(dir, "CLAUDE.md")
	writeSectionFile(t, targetFile, "SAP Dev", "old outdated content")

	pack := makePackWithContent("test-pack", "## CAP\nNew content.\n")
	adapters := []adapter.Adapter{{
		ID:   "test-tool",
		Name: "Test Tool",
		Type: "file-inject",
		Targets: []adapter.Target{
			{Scope: "global", Path: targetFile, Mode: "replace-section", Section: "SAP Dev"},
		},
	}}

	eng := adapter.NewEngine(adapters, []*content.Pack{pack}, nil, adapter.Options{Scope: "global"})
	rows, err := eng.Status()
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.True(t, rows[0].Injected)
	assert.True(t, rows[0].Stale)
}

func TestStatus_ScopeFilter(t *testing.T) {
	dir := t.TempDir()
	targetFile := filepath.Join(dir, "CLAUDE.md")

	adapters := []adapter.Adapter{{
		ID:   "test-tool",
		Name: "Test Tool",
		Type: "file-inject",
		Targets: []adapter.Target{
			{Scope: "project", Path: targetFile, Mode: "replace-section", Section: "SAP Dev"},
		},
	}}

	// Engine scope is global — project target must be skipped
	eng := adapter.NewEngine(adapters, nil, nil, adapter.Options{Scope: "global"})
	rows, err := eng.Status()
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestStatus_ToolFilter(t *testing.T) {
	dir := t.TempDir()
	fileA := filepath.Join(dir, "A.md")
	fileB := filepath.Join(dir, "B.md")

	adapters := []adapter.Adapter{
		{
			ID:   "tool-a",
			Name: "Tool A",
			Type: "file-inject",
			Targets: []adapter.Target{
				{Scope: "global", Path: fileA, Mode: "replace-section", Section: "SAP Dev"},
			},
		},
		{
			ID:   "tool-b",
			Name: "Tool B",
			Type: "file-inject",
			Targets: []adapter.Target{
				{Scope: "global", Path: fileB, Mode: "replace-section", Section: "SAP Dev"},
			},
		},
	}

	eng := adapter.NewEngine(adapters, nil, nil, adapter.Options{Scope: "global", ToolFilter: "tool-a"})
	rows, err := eng.Status()
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "tool-a", rows[0].AdapterID)
}

func TestStatus_ReplaceFile(t *testing.T) {
	dir := t.TempDir()
	targetFile := filepath.Join(dir, "context.md")
	require.NoError(t, os.WriteFile(targetFile, []byte("preamble\ncontent"), 0644))

	adapters := []adapter.Adapter{{
		ID:   "test-tool",
		Name: "Test Tool",
		Type: "file-inject",
		Targets: []adapter.Target{
			{Scope: "global", Path: targetFile, Mode: "replace-file"},
		},
	}}

	eng := adapter.NewEngine(adapters, nil, nil, adapter.Options{Scope: "global"})
	rows, err := eng.Status()
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.True(t, rows[0].FileExists)
	assert.True(t, rows[0].Injected) // replace-file: existing file = injected
}

func TestStatus_OtherSections(t *testing.T) {
	dir := t.TempDir()
	targetFile := filepath.Join(dir, "CLAUDE.md")
	data := `<!-- sap-devs:start:SAP Dev -->
sap content
<!-- sap-devs:end:SAP Dev -->
<!-- cursor:start:Rules -->
cursor rules
<!-- cursor:end:Rules -->
`
	require.NoError(t, os.WriteFile(targetFile, []byte(data), 0644))

	adapters := []adapter.Adapter{{
		ID:   "test-tool",
		Name: "Test Tool",
		Type: "file-inject",
		Targets: []adapter.Target{
			{Scope: "global", Path: targetFile, Mode: "replace-section", Section: "SAP Dev"},
		},
	}}

	eng := adapter.NewEngine(adapters, nil, nil, adapter.Options{Scope: "global"})
	rows, err := eng.Status()
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Len(t, rows[0].OtherSections, 1)
	assert.Equal(t, "cursor", rows[0].OtherSections[0].Name)
}

func TestStatus_SkipsNonFileInject(t *testing.T) {
	adapters := []adapter.Adapter{{
		ID:   "chatgpt",
		Name: "ChatGPT",
		Type: "clipboard-export",
	}}

	eng := adapter.NewEngine(adapters, nil, nil, adapter.Options{Scope: "global"})
	rows, err := eng.Status()
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestStatus_ErrorContinues(t *testing.T) {
	dir := t.TempDir()
	fileA := filepath.Join(dir, "A.md")
	fileB := filepath.Join(dir, "B.md")
	require.NoError(t, os.WriteFile(fileA, []byte("# hello"), 0644))
	// fileB does not exist — will yield FileExists=false but no error

	adapters := []adapter.Adapter{{
		ID:   "test-tool",
		Name: "Test Tool",
		Type: "file-inject",
		Targets: []adapter.Target{
			{Scope: "global", Path: fileA, Mode: "replace-section", Section: "SAP Dev"},
			{Scope: "global", Path: fileB, Mode: "replace-section", Section: "SAP Dev"},
		},
	}}

	eng := adapter.NewEngine(adapters, nil, nil, adapter.Options{Scope: "global"})
	rows, err := eng.Status()
	require.NoError(t, err) // not-exist is not an error
	assert.Len(t, rows, 2)
	assert.True(t, rows[0].FileExists)
	assert.False(t, rows[1].FileExists)
}

func TestStatus_MultipleTargets_TwoRows(t *testing.T) {
	dir := t.TempDir()
	globalFile := filepath.Join(dir, "global.md")
	projectFile := filepath.Join(dir, "project.md")

	adapters := []adapter.Adapter{{
		ID:   "test-tool",
		Name: "Test Tool",
		Type: "file-inject",
		Targets: []adapter.Target{
			{Scope: "global", Path: globalFile, Mode: "replace-section", Section: "SAP Dev"},
			{Scope: "project", Path: projectFile, Mode: "replace-section", Section: "SAP Dev"},
		},
	}}

	// global scope: only global target → 1 row
	eng := adapter.NewEngine(adapters, nil, nil, adapter.Options{Scope: "global"})
	rows, err := eng.Status()
	require.NoError(t, err)
	assert.Len(t, rows, 1)
	assert.Equal(t, "global", rows[0].Scope)
}
```

> **Note on `TestStatus_Current`:** It calls `eng.RenderSectionContentForTest(...)` — a thin exported wrapper you will add to `engine.go` in the next step to make the unexported method testable. This wrapper is test-only and will be added alongside `Status()`.

- [ ] **Step 2: Run tests — verify they fail**

```bash
go test ./internal/adapter/... -run "TestStatus_" -v
```

Expected: FAIL (adapter.Engine has no Status method)

- [ ] **Step 3: Add `Status()` and `RenderSectionContentForTest` to `engine.go`**

Add these methods after `renderSectionContent` in `internal/adapter/engine.go`:

```go
// RenderSectionContentForTest exposes renderSectionContent for white-box tests.
// Do not call this from production code.
func (e *Engine) RenderSectionContentForTest(a Adapter) string {
	return e.renderSectionContent(a)
}

// Status inspects each file-inject adapter target and returns one StatusRow per
// (adapter, target) pair for the configured scope.
func (e *Engine) Status() ([]StatusRow, error) {
	var rows []StatusRow
	var err error

	for _, a := range e.adapters {
		if e.opts.ToolFilter != "" && a.ID != e.opts.ToolFilter {
			continue
		}
		if a.Type != "file-inject" {
			continue
		}
		for _, target := range a.Targets {
			if target.Scope != e.opts.Scope {
				continue
			}
			row := StatusRow{
				AdapterName:   a.Name,
				AdapterID:     a.ID,
				Scope:         target.Scope,
				TargetPath:    target.Path,
				OtherSections: []SectionInfo{},
			}

			path, expandErr := ExpandHome(target.Path)
			if expandErr != nil {
				err = errors.Join(err, fmt.Errorf("target %s: %w", target.Path, expandErr))
				rows = append(rows, row)
				continue
			}

			fileBytes, readErr := os.ReadFile(path)
			if readErr != nil {
				if !os.IsNotExist(readErr) {
					err = errors.Join(err, fmt.Errorf("target %s: %w", target.Path, readErr))
				}
				rows = append(rows, row)
				continue
			}

			row.FileExists = true
			fileStr := string(fileBytes)

			switch target.Mode {
			case "replace-section":
				startMarker := fmt.Sprintf(markerFmt, target.Section)
				endMarker := fmt.Sprintf(markerEndFmt, target.Section)
				startIdx, endIdx, sStatus := findSection(fileStr, startMarker, endMarker)
				switch sStatus {
				case sectionFound:
					row.Injected = true
					// Staleness check
					if e.packs != nil {
						rendered := e.renderSectionContent(a)
						// Extract on-disk inner content: from after startMarker+"\n" to endIdx
						innerStart := startIdx + len(startMarker) + 1 // +1 for the \n after the marker
						if innerStart > endIdx {
							innerStart = endIdx
						}
						onDisk := fileStr[innerStart:endIdx]
						row.Stale = strings.TrimSpace(rendered) != strings.TrimSpace(onDisk)
					}
					// SapDevsTokens
					innerStart := startIdx + len(startMarker) + 1
					if innerStart > endIdx {
						innerStart = endIdx
					}
					row.SapDevsTokens = EstimateTokens(fileStr[innerStart:endIdx])
				case sectionOrphaned:
					row.Orphaned = true
				}
			case "replace-file":
				row.Injected = true
				if e.packs != nil {
					rendered := e.renderSectionContent(a)
					var expected string
					if target.Preamble != "" {
						expected = target.Preamble + "\n" + rendered
					} else {
						expected = rendered
					}
					row.Stale = strings.TrimSpace(expected) != strings.TrimSpace(fileStr)
				}
				row.SapDevsTokens = EstimateTokens(fileStr)
			case "append":
				fmt.Fprintf(os.Stderr, "%s\n", i18n.Tf(e.opts.Lang, "inject.status.append_warning", map[string]any{"Path": path}))
			}

			// Stretch-goal fields
			row.FileSizeBytes = len(fileBytes)
			row.FileTokenEst = EstimateTokens(fileStr)
			row.OtherSections = ScanOtherSections(fileStr)

			rows = append(rows, row)
		}
	}

	return rows, err
}
```

Also add `"strings"` to the imports in `engine.go` if not already present (it is already present).

- [ ] **Step 4: Run tests — verify they pass**

```bash
go test ./internal/adapter/... -run "TestStatus_" -v
```

Expected: PASS (all TestStatus_ tests)

- [ ] **Step 5: Build and vet**

```bash
go build ./...
go vet ./...
```

Expected: no errors

- [ ] **Step 6: Commit**

```bash
git add internal/adapter/engine.go internal/adapter/status_test.go
git commit -m "feat: add Status() method to Engine"
```

---

## Task 4: i18n keys

**Files:**
- Modify: `internal/i18n/catalogs/en.json`
- Modify: `internal/i18n/catalogs/de.json`

**What to build:** All `inject.status.*` keys needed by the command layer and engine.

- [ ] **Step 1: Add keys to `en.json`**

In `internal/i18n/catalogs/en.json`, after the `inject.uninstall.*` block, add:

```json
  "inject.status.header_tool": "Tool",
  "inject.status.header_scope": "Scope",
  "inject.status.header_file": "File",
  "inject.status.header_status": "Status",
  "inject.status.header_size": "Size",
  "inject.status.header_tokens": "Tokens",
  "inject.status.header_sap_pct": "SAP%",
  "inject.status.header_other": "Other sections",
  "inject.status.current": "✓ current",
  "inject.status.stale": "✗ stale",
  "inject.status.not_found": "✗ not found",
  "inject.status.orphaned": "✗ orphaned",
  "inject.status.not_injected": "✗ not injected",
  "inject.status.no_results": "No file-inject adapters found for the given scope/tool.",
  "inject.status.append_warning": "sap-devs warning: {{.Path}} uses append mode — injection state cannot be determined",
```

- [ ] **Step 2: Add keys to `de.json`**

In `internal/i18n/catalogs/de.json`, add the same keys with German values:

```json
  "inject.status.header_tool": "Tool",
  "inject.status.header_scope": "Geltungsbereich",
  "inject.status.header_file": "Datei",
  "inject.status.header_status": "Status",
  "inject.status.header_size": "Größe",
  "inject.status.header_tokens": "Token",
  "inject.status.header_sap_pct": "SAP%",
  "inject.status.header_other": "Andere Abschnitte",
  "inject.status.current": "✓ aktuell",
  "inject.status.stale": "✗ veraltet",
  "inject.status.not_found": "✗ nicht gefunden",
  "inject.status.orphaned": "✗ verwaist",
  "inject.status.not_injected": "✗ nicht injiziert",
  "inject.status.no_results": "Keine file-inject-Adapter für den angegebenen Geltungsbereich/das Tool gefunden.",
  "inject.status.append_warning": "sap-devs Warnung: {{.Path}} verwendet den Anhängemodus — Injektionsstatus kann nicht ermittelt werden",
```

- [ ] **Step 3: Build and vet**

```bash
go build ./...
go vet ./...
```

Expected: no errors

- [ ] **Step 4: Commit**

```bash
git add internal/i18n/catalogs/en.json internal/i18n/catalogs/de.json
git commit -m "feat: add inject.status i18n keys (en + de)"
```

---

## Task 5: Command layer — `--status` flag and output

**Files:**
- Modify: `cmd/inject.go`
- Create: `cmd/inject_status_test.go`

**What to build:** `--status`, `--json`, `--verbose` flags; mutual exclusion check; early-return block that calls `eng.Status()` and renders the result as a tabwriter table or JSON.

- [ ] **Step 1: Write the failing cmd tests**

Create `cmd/inject_status_test.go`:

```go
package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInjectStatus_FlagExists(t *testing.T) {
	require.NotNil(t, injectCmd.Flags().Lookup("status"), "--status flag must be registered")
	require.NotNil(t, injectCmd.Flags().Lookup("json"), "--json flag must be registered")
	require.NotNil(t, injectCmd.Flags().Lookup("verbose"), "--verbose flag must be registered")
}

func TestInjectStatus_MutualExclusion_WithUninstall(t *testing.T) {
	injectStatus = true
	injectUninstall = true
	t.Cleanup(func() {
		injectStatus = false
		injectUninstall = false
	})
	err := injectCmd.RunE(injectCmd, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--status is incompatible")
}

func TestInjectStatus_MutualExclusion_WithSync(t *testing.T) {
	injectStatus = true
	injectSync = true
	t.Cleanup(func() {
		injectStatus = false
		injectSync = false
	})
	err := injectCmd.RunE(injectCmd, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--status is incompatible")
}

func TestInjectStatus_MutualExclusion_WithNoSync(t *testing.T) {
	injectStatus = true
	injectNoSync = true
	t.Cleanup(func() {
		injectStatus = false
		injectNoSync = false
	})
	err := injectCmd.RunE(injectCmd, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--status is incompatible")
}

func TestInjectStatus_MutualExclusion_WithStats(t *testing.T) {
	injectStatus = true
	injectStats = true
	t.Cleanup(func() {
		injectStatus = false
		injectStats = false
	})
	err := injectCmd.RunE(injectCmd, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--status is incompatible")
}

func TestInjectStatus_MutualExclusion_WithDryRun(t *testing.T) {
	injectStatus = true
	injectDryRun = true
	t.Cleanup(func() {
		injectStatus = false
		injectDryRun = false
	})
	err := injectCmd.RunE(injectCmd, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--status is incompatible")
}

func TestInjectStatus_JSONWithoutStatusNoError(t *testing.T) {
	// --json alone (no --status) must not trigger the mutual-exclusion error.
	injectJSON = true
	t.Cleanup(func() { injectJSON = false })
	err := injectCmd.RunE(injectCmd, nil)
	if err != nil {
		assert.NotContains(t, err.Error(), "--status is incompatible")
		assert.NotContains(t, err.Error(), "mutually exclusive")
	}
}
```

- [ ] **Step 2: Run tests — verify they fail**

```bash
go test ./cmd/... -run "TestInjectStatus_" -v
```

Expected: FAIL (injectStatus undefined)

- [ ] **Step 3: Add flags and mutual exclusion check to `cmd/inject.go`**

In the `var (...)` block at the top of `cmd/inject.go`, add after `injectUninstall bool`:

```go
injectStatus  bool
injectJSON    bool
injectVerbose bool
```

In `RunE`, after the existing `--uninstall` incompatibility check, add:

```go
if injectStatus && (injectUninstall || injectSync || injectNoSync || injectDryRun || injectStats) {
    return fmt.Errorf("--status is incompatible with --uninstall, --sync, --no-sync, --dry-run, and --stats")
}
```

Then, after the existing `injectUninstall` early-return block and before the `scope := "global"` line, add the status early-return block:

```go
if injectStatus {
    lang := i18n.ActiveLang
    gatheredAdapters, err := loadAdapters()
    if err != nil {
        return err
    }
    scope := "global"
    if injectProject {
        scope = "project"
    }

    // Load packs for staleness check (errors are non-fatal — status still works without packs)
    loader, loaderErr := newContentLoader()
    var packs []*content.Pack
    var activeProfile *content.Profile
    if loaderErr == nil {
        paths, pathsErr := xdg.New()
        if pathsErr == nil {
            configProfile, _ := config.LoadProfile(paths.ConfigDir)
            if configProfile.ID != "" {
                activeProfile, _ = loader.FindProfile(configProfile.ID)
            }
        }
        packs, _ = loader.LoadPacks(activeProfile, lang)
    }

    opts := adapter.Options{
        Scope:      scope,
        ToolFilter: injectTool,
        Lang:       lang,
    }
    eng := adapter.NewEngine(gatheredAdapters, packs, activeProfile, opts)
    rows, statusErr := eng.Status()
    if statusErr != nil {
        return statusErr
    }

    if injectJSON {
        return printStatusJSON(cmd, rows)
    }
    printStatusTable(cmd, rows, lang, injectVerbose)
    return nil
}
```

- [ ] **Step 4: Add `printStatusJSON` and `printStatusTable` helpers**

Add these functions at the bottom of `cmd/inject.go` (before the `init()` function):

```go
func printStatusJSON(cmd *cobra.Command, rows []adapter.StatusRow) error {
    enc := json.NewEncoder(cmd.OutOrStdout())
    enc.SetIndent("", "  ")
    return enc.Encode(rows)
}

func printStatusTable(cmd *cobra.Command, rows []adapter.StatusRow, lang string, verbose bool) {
    w := cmd.OutOrStdout()
    if len(rows) == 0 {
        fmt.Fprintln(w, i18n.T(lang, "inject.status.no_results"))
        return
    }
    tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
    if verbose {
        fmt.Fprintln(tw,
            i18n.T(lang, "inject.status.header_tool")+"\t"+
                i18n.T(lang, "inject.status.header_scope")+"\t"+
                i18n.T(lang, "inject.status.header_file")+"\t"+
                i18n.T(lang, "inject.status.header_status")+"\t"+
                i18n.T(lang, "inject.status.header_size")+"\t"+
                i18n.T(lang, "inject.status.header_tokens")+"\t"+
                i18n.T(lang, "inject.status.header_sap_pct")+"\t"+
                i18n.T(lang, "inject.status.header_other"))
    } else {
        fmt.Fprintln(tw,
            i18n.T(lang, "inject.status.header_tool")+"\t"+
                i18n.T(lang, "inject.status.header_scope")+"\t"+
                i18n.T(lang, "inject.status.header_file")+"\t"+
                i18n.T(lang, "inject.status.header_status"))
    }
    for _, row := range rows {
        status := statusLabel(row, lang)
        if verbose {
            pct := 0
            if row.FileTokenEst > 0 {
                pct = row.SapDevsTokens * 100 / row.FileTokenEst
            }
            other := fmt.Sprintf("%d", len(row.OtherSections))
            fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%d B\t%d\t%d%%\t%s\n",
                row.AdapterName, row.Scope, row.TargetPath, status,
                row.FileSizeBytes, row.FileTokenEst, pct, other)
        } else {
            fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
                row.AdapterName, row.Scope, row.TargetPath, status)
        }
    }
    tw.Flush()
}

func statusLabel(row adapter.StatusRow, lang string) string {
    if !row.FileExists {
        return i18n.T(lang, "inject.status.not_found")
    }
    if row.Orphaned {
        return i18n.T(lang, "inject.status.orphaned")
    }
    if !row.Injected {
        return i18n.T(lang, "inject.status.not_injected")
    }
    if row.Stale {
        return i18n.T(lang, "inject.status.stale")
    }
    return i18n.T(lang, "inject.status.current")
}
```

Add `"encoding/json"` and `"text/tabwriter"` to the imports in `cmd/inject.go`. (`text/tabwriter` is not currently imported there.)

- [ ] **Step 5: Register flags in `init()`**

In the `init()` function at the bottom of `cmd/inject.go`, add after the `--uninstall` line:

```go
injectCmd.Flags().BoolVar(&injectStatus, "status", false, "report injection state for all detected AI tools")
injectCmd.Flags().BoolVar(&injectJSON, "json", false, "output status as JSON (only with --status)")
injectCmd.Flags().BoolVar(&injectVerbose, "verbose", false, "show file size and token breakdown (only with --status)")
```

- [ ] **Step 6: Run tests — verify they pass**

```bash
go test ./cmd/... -run "TestInjectStatus_" -v
```

Expected: PASS (all TestInjectStatus_ tests)

- [ ] **Step 7: Build and vet**

```bash
go build ./...
go vet ./...
```

Expected: no errors

- [ ] **Step 8: Commit**

```bash
git add cmd/inject.go cmd/inject_status_test.go
git commit -m "feat: add inject --status command with --json and --verbose flags"
```

---

## Task 6: Documentation

**Files:**
- Modify: `docs/user/user-guide.md`
- Modify: `docs/developer/developer-guide.md`

**What to build:** Document the new flags in user-facing and developer-facing guides.

- [ ] **Step 1: Update `docs/user/user-guide.md`**

In the **Core Workflow → Inject context into AI tools** section (around line 98), add two new examples after the existing `--uninstall --dry-run` example:

```bash
# Report injection state across all AI tools
sap-devs inject --status

# Report with file size and token breakdown
sap-devs inject --status --verbose

# Report as JSON (for scripting / CI)
sap-devs inject --status --json
```

In the **Command Reference → inject** flag table (around line 134), add after the `--uninstall` row:

| `--status` | Report injection state (present/stale/not found) for all AI tool config files |
| `--json` | Output status as JSON array (only with `--status`) |
| `--verbose` | Show file size and token breakdown columns (only with `--status`) |

After the existing `--uninstall` output example, add a `--status` output example:

```
**`--status` output example:**

```text
Tool            Scope    File                        Status
Claude Code     global   ~/.claude/CLAUDE.md         ✓ current
Cursor          global   ~/.cursor/rules/sap.mdc     ✗ not found
```

**`--status --verbose` output example:**

```text
Tool            Scope    File                    Status      Size     Tokens  SAP%  Other sections
Claude Code     global   ~/.claude/CLAUDE.md     ✓ current   14200 B  3200    42%   1
```
```

- [ ] **Step 2: Update `docs/developer/developer-guide.md`**

In the **Architecture Overview → Adapter System** section, after the description of `Run()` returning `RunResult`, add:

> `Status() ([]StatusRow, error)` — inspects all `file-inject` targets for the configured scope and returns one `StatusRow` per `(adapter, target)` pair. Each row reports file existence, injection state, staleness (via content-hash comparison using `renderSectionContent`), and stretch-goal file-analysis fields. Defined alongside its types and helpers in `internal/adapter/status.go`.

- [ ] **Step 3: Build and vet**

```bash
go build ./...
go vet ./...
```

Expected: no errors

- [ ] **Step 4: Commit**

```bash
git add docs/user/user-guide.md docs/developer/developer-guide.md
git commit -m "docs: document inject --status, --verbose, --json flags"
```

---

## Task 7: Mark backlog item done in TODO.md

**Files:**
- Modify: `TODO.md`

- [ ] **Step 1: Update TODO.md**

Find the `### \`sap-devs inject --status\`` backlog section (around line 513) and replace it with:

```markdown
### ✅ DONE: `sap-devs inject --status`

Implemented in `feat/inject-status`. See spec: `docs/superpowers/specs/2026-04-17-inject-status-design.md`.
```

- [ ] **Step 2: Build and vet**

```bash
go build ./...
go vet ./...
```

Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add TODO.md
git commit -m "chore: mark inject --status as done in TODO.md"
```

---

## Final Verification

After all tasks are complete, run the full build and vet:

```bash
go build ./...
go vet ./...
```

On Linux/CI, run tests:

```bash
go test ./internal/adapter/... -v
go test ./cmd/... -v
```

Expected: all tests pass, no vet warnings.
