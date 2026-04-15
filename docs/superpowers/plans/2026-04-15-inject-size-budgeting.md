# Inject Size Budgeting Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add per-adapter token budgets, overlap deduplication, and `--stats` profiling to the `inject` pipeline so content is trimmed to fit each AI tool before injection.

**Architecture:** A new `TrimPacks` function filters packs in two passes (dedup then budget) before rendering. The `Engine` struct is refactored to hold packs+profile instead of a pre-rendered string, rendering per-adapter with each adapter's own budget. A `--stats` flag surfaces what was injected and how much budget was used.

**Tech Stack:** Go, cobra, `text/tabwriter` (stdlib), testify (existing)

> **Windows note:** `go test` always fails locally (Windows Defender). Use `go build ./...` + `go vet ./...` for local verification. CI (`ubuntu-latest`) is the authoritative test runner.

---

## File Map

| File | What changes |
|---|---|
| `internal/content/pack.go` | Add `Overlaps []string` to `Pack` and `packMeta`; copy in `LoadPack` |
| `internal/content/pack_test.go` | Add test for `overlaps` field loading from YAML |
| `internal/content/render.go` | Add `TrimPacks(packs []*Pack, maxBytes int) []*Pack` |
| `internal/content/render_test.go` | Add `TrimPacks` unit tests |
| `internal/adapter/adapter.go` | Add `MaxTokens int` to `Adapter` |
| `internal/adapter/engine.go` | Refactor `Engine` struct, `Options`, `NewEngine`, `Run()`, `runFileInject`; add `adapterStats`, `printStats` |
| `internal/adapter/adapter_test.go` | Update 4 `NewEngine` call sites to new signature; add stats + budget tests |
| `cmd/inject.go` | Remove pre-render line; wire `--stats` flag into `Options` |
| `cmd/inject_test.go` | Update `NewEngine` call site and remove `rendered` pre-step |
| `cmd/root.go` | Update `newAdapterEngine` signature |

---

## Task 1: Pack.Overlaps data model

**Files:**
- Modify: `internal/content/pack.go`
- Modify: `internal/content/pack_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/content/pack_test.go`:

```go
func TestLoadPack_OverlapsField(t *testing.T) {
	dir := t.TempDir()
	yaml := `id: btp-core
name: BTP Core
description: BTP basics
tags: []
profiles: []
weight: 80
overlaps:
  - cap
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(yaml), 0644))

	p, err := content.LoadPack(dir, "")
	require.NoError(t, err)
	assert.Equal(t, []string{"cap"}, p.Overlaps)
}

func TestLoadPack_NoOverlaps(t *testing.T) {
	dir := t.TempDir()
	yaml := "id: cap\nname: CAP\ndescription: CAP\ntags: []\nprofiles: []\nweight: 100\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(yaml), 0644))

	p, err := content.LoadPack(dir, "")
	require.NoError(t, err)
	assert.Empty(t, p.Overlaps)
}
```

- [ ] **Step 2: Verify it fails to build**

```bash
cd d:/projects/sap-devs-cli && go build ./internal/content/...
```

Expected: compile error — `p.Overlaps` does not exist yet.

- [ ] **Step 3: Add `Overlaps` to `packMeta` and `Pack` in `internal/content/pack.go`**

In `packMeta`, add after the `Weight` field:
```go
Overlaps []string `yaml:"overlaps,omitempty"`
```

In `Pack`, add after the `Weight` field:
```go
Overlaps []string
```

In `LoadPack`, inside the `pack := &Pack{...}` literal, add:
```go
Overlaps: meta.Overlaps,
```

- [ ] **Step 4: Build and vet**

```bash
cd d:/projects/sap-devs-cli && go build ./... && go vet ./...
```

Expected: clean.

- [ ] **Step 5: Commit**

```bash
cd d:/projects/sap-devs-cli && git add internal/content/pack.go internal/content/pack_test.go && git commit -m "feat(content): add Overlaps field to Pack for deduplication"
```

---

## Task 2: TrimPacks function

**Files:**
- Modify: `internal/content/render.go`
- Modify: `internal/content/render_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `internal/content/render_test.go`:

```go
func TestTrimPacks_Unconstrained(t *testing.T) {
	packs := []*content.Pack{
		{ID: "cap", ContextMD: "CAP content"},
		{ID: "btp-core", ContextMD: "BTP content"},
	}
	result := content.TrimPacks(packs, 0)
	require.Len(t, result, 2)
	assert.Equal(t, "cap", result[0].ID)
	assert.Equal(t, "btp-core", result[1].ID)
}

func TestTrimPacks_EmptyInput(t *testing.T) {
	result := content.TrimPacks(nil, 0)
	assert.Empty(t, result)
}

func TestTrimPacks_DeduplicatesOverlappingPack(t *testing.T) {
	// cap (high weight) is already included; btp-core declares it overlaps cap → dropped
	packs := []*content.Pack{
		{ID: "cap", ContextMD: "CAP content"},
		{ID: "btp-core", ContextMD: "BTP content", Overlaps: []string{"cap"}},
	}
	result := content.TrimPacks(packs, 0)
	require.Len(t, result, 1)
	assert.Equal(t, "cap", result[0].ID)
}

func TestTrimPacks_DeduplicatesOnlyWhenHigherWeightPresent(t *testing.T) {
	// btp-core declares overlaps: [cap], but cap is not loaded — btp-core is kept
	packs := []*content.Pack{
		{ID: "btp-core", ContextMD: "BTP content", Overlaps: []string{"cap"}},
	}
	result := content.TrimPacks(packs, 0)
	require.Len(t, result, 1)
	assert.Equal(t, "btp-core", result[0].ID)
}

func TestTrimPacks_BudgetDropsPackThatDoesNotFit(t *testing.T) {
	// cap (14 bytes) exceeds 10-byte budget → break; btp-core never reached → result is empty
	packs := []*content.Pack{
		{ID: "cap", ContextMD: "12 chars long!"},
		{ID: "btp-core", ContextMD: "short"},
	}
	result := content.TrimPacks(packs, 10)
	assert.Empty(t, result)
}

func TestTrimPacks_BudgetIncludesFittingPacksInOrder(t *testing.T) {
	// cap (11 bytes) fits in 20-byte budget; abap (6 bytes) would also fit but we break at first miss
	packs := []*content.Pack{
		{ID: "cap", ContextMD: "11 chars ok!"},   // 12 bytes
		{ID: "big", ContextMD: "this is too large for budget"}, // 28 bytes — doesn't fit → break
		{ID: "abap", ContextMD: "small"},           // never reached
	}
	result := content.TrimPacks(packs, 20)
	require.Len(t, result, 1)
	assert.Equal(t, "cap", result[0].ID)
}

func TestTrimPacks_EmptyContextMDAlwaysFits(t *testing.T) {
	// Pack with no context file (size 0) fits any budget
	packs := []*content.Pack{
		{ID: "meta", ContextMD: ""},
		{ID: "cap", ContextMD: "some content here"},
	}
	result := content.TrimPacks(packs, 5)
	// meta fits (0 bytes), cap doesn't fit (17 bytes > 5), breaks after cap
	require.Len(t, result, 1)
	assert.Equal(t, "meta", result[0].ID)
}

func TestTrimPacks_DeduplicateAndBudgetCombined(t *testing.T) {
	packs := []*content.Pack{
		{ID: "cap", ContextMD: "cap content here"},             // 16 bytes, included
		{ID: "btp-core", ContextMD: "btp", Overlaps: []string{"cap"}}, // deduped out
		{ID: "abap", ContextMD: "abap content"},                // 12 bytes, fits
	}
	result := content.TrimPacks(packs, 100)
	require.Len(t, result, 2)
	assert.Equal(t, "cap", result[0].ID)
	assert.Equal(t, "abap", result[1].ID)
}
```

- [ ] **Step 2: Verify it fails to build**

```bash
cd d:/projects/sap-devs-cli && go build ./internal/content/...
```

Expected: compile error — `content.TrimPacks` undefined.

- [ ] **Step 3: Implement `TrimPacks` in `internal/content/render.go`**

Add after `RenderContext`:

```go
// TrimPacks filters packs to fit within maxBytes, applying overlap deduplication
// and pack-level budget enforcement. Pass maxBytes=0 for unconstrained.
// Packs must already be sorted by weight descending (LoadPacks guarantees this).
func TrimPacks(packs []*Pack, maxBytes int) []*Pack {
	// Pass 1 — deduplication
	// A pack is dropped if a higher-weight pack it overlaps with is already included.
	included := make(map[string]bool)
	var deduped []*Pack
	for _, p := range packs {
		dominated := false
		for _, overlapID := range p.Overlaps {
			if included[overlapID] {
				dominated = true
				break
			}
		}
		if !dominated {
			deduped = append(deduped, p)
			included[p.ID] = true
		}
	}

	// Pass 2 — budget enforcement
	if maxBytes <= 0 {
		return deduped
	}
	var result []*Pack
	used := 0
	for _, p := range deduped {
		size := len(p.ContextMD)
		if used+size > maxBytes {
			break
		}
		result = append(result, p)
		used += size
	}
	return result
}
```

- [ ] **Step 4: Build and vet**

```bash
cd d:/projects/sap-devs-cli && go build ./... && go vet ./...
```

Expected: clean.

- [ ] **Step 5: Commit**

```bash
cd d:/projects/sap-devs-cli && git add internal/content/render.go internal/content/render_test.go && git commit -m "feat(content): add TrimPacks for deduplication and budget enforcement"
```

---

## Task 3: Adapter.MaxTokens data model

**Files:**
- Modify: `internal/adapter/adapter.go`
- Modify: `internal/adapter/adapter_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/adapter/adapter_test.go`:

```go
func TestLoadAdapters_MaxTokens(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, filepath.Join(dir, "cursor.yaml"), `
id: cursor
name: Cursor
type: file-inject
max_tokens: 2000
targets:
  - scope: global
    path: "~/.cursor/rules/sap.mdc"
    mode: replace-section
    section: "SAP Developer Context"
`)
	adapters, err := adapter.LoadAdapters(dir)
	require.NoError(t, err)
	require.Len(t, adapters, 1)
	assert.Equal(t, 2000, adapters[0].MaxTokens)
}

func TestLoadAdapters_MaxTokensDefaultsToZero(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, filepath.Join(dir, "claude-code.yaml"), `
id: claude-code
name: Claude Code
type: file-inject
targets:
  - scope: global
    path: "~/.claude/CLAUDE.md"
    mode: replace-section
    section: "SAP Developer Context"
`)
	adapters, err := adapter.LoadAdapters(dir)
	require.NoError(t, err)
	require.Len(t, adapters, 1)
	assert.Equal(t, 0, adapters[0].MaxTokens)
}
```

- [ ] **Step 2: Verify it fails to build**

```bash
cd d:/projects/sap-devs-cli && go build ./internal/adapter/...
```

Expected: compile error — `adapters[0].MaxTokens` undefined.

- [ ] **Step 3: Add `MaxTokens` to `Adapter` in `internal/adapter/adapter.go`**

In the `Adapter` struct, add after `Instructions string`:
```go
MaxTokens    int          `yaml:"max_tokens,omitempty"` // 0 = unconstrained
```

- [ ] **Step 4: Build and vet**

```bash
cd d:/projects/sap-devs-cli && go build ./... && go vet ./...
```

Expected: clean.

- [ ] **Step 5: Commit**

```bash
cd d:/projects/sap-devs-cli && git add internal/adapter/adapter.go internal/adapter/adapter_test.go && git commit -m "feat(adapter): add MaxTokens field for per-adapter budget"
```

---

## Task 4: Engine refactor

This is the core change. The `Engine` struct drops `context string` and gains `packs + profile`. `Options` gains `Stats` and `Out`. `Run()` renders per-adapter. `runFileInject` gains a `ctx` parameter. Stats are collected and printed after all adapters run.

**Files:**
- Modify: `internal/adapter/engine.go`
- Modify: `internal/adapter/adapter_test.go`

- [ ] **Step 1: Write new engine tests**

Add to `internal/adapter/adapter_test.go`. This also requires adding `"bytes"` and `"strings"` to the import block, and importing `content`:

```go
import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/adapter"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
)
```

Add these test functions:

```go
func TestEngine_PerAdapterBudget(t *testing.T) {
	dir := t.TempDir()
	fileA := filepath.Join(dir, "a.md")

	packs := []*content.Pack{
		{ID: "cap", ContextMD: strings.Repeat("x", 1000)},  // 1000 bytes ≈ 250 tokens
		{ID: "btp", ContextMD: strings.Repeat("y", 1000)},  // 1000 bytes ≈ 250 tokens
	}

	// budget of 500 tokens = 2000 bytes: both packs fit
	adapters := []adapter.Adapter{
		{
			ID:        "tool-a",
			Type:      "file-inject",
			MaxTokens: 500,
			Targets:   []adapter.Target{{Scope: "global", Path: fileA, Mode: "replace-section", Section: "S"}},
		},
	}

	engine := adapter.NewEngine(adapters, packs, nil, adapter.Options{Scope: "global"})
	require.NoError(t, engine.Run())

	data, err := os.ReadFile(fileA)
	require.NoError(t, err)
	assert.Contains(t, string(data), strings.Repeat("x", 1000))
	assert.Contains(t, string(data), strings.Repeat("y", 1000))
}

func TestEngine_BudgetTooSmall_EmitsWarning(t *testing.T) {
	var buf bytes.Buffer
	packs := []*content.Pack{
		{ID: "cap", ContextMD: strings.Repeat("x", 1000)},
	}
	adapters := []adapter.Adapter{
		{
			ID:        "tiny-tool",
			Type:      "file-inject",
			MaxTokens: 1, // 4 bytes — too small for any pack
			Targets:   []adapter.Target{{Scope: "global", Path: t.TempDir() + "/f.md", Mode: "replace-section", Section: "S"}},
		},
	}
	engine := adapter.NewEngine(adapters, packs, nil, adapter.Options{Scope: "global", Out: &buf})
	require.NoError(t, engine.Run())
	assert.Contains(t, buf.String(), "budget too small")
}

func TestEngine_Stats(t *testing.T) {
	dir := t.TempDir()
	targetFile := filepath.Join(dir, "out.md")

	packs := []*content.Pack{
		{ID: "cap", ContextMD: "CAP content"},
		{ID: "btp", ContextMD: "BTP content"},
	}
	adapters := []adapter.Adapter{
		{
			ID:        "test-tool",
			Type:      "file-inject",
			MaxTokens: 0,
			Targets:   []adapter.Target{{Scope: "global", Path: targetFile, Mode: "replace-section", Section: "S"}},
		},
	}

	var buf bytes.Buffer
	engine := adapter.NewEngine(adapters, packs, nil, adapter.Options{
		Scope:  "global",
		DryRun: true,
		Stats:  true,
		Out:    &buf,
	})
	require.NoError(t, engine.Run())

	out := buf.String()
	assert.Contains(t, out, "Adapter")
	assert.Contains(t, out, "test-tool")
	assert.Contains(t, out, "cap")
}

func TestEngine_NilOutIsSafe(t *testing.T) {
	packs := []*content.Pack{{ID: "cap", ContextMD: "content"}}
	adapters := []adapter.Adapter{
		{
			ID:   "test",
			Type: "file-inject",
			Targets: []adapter.Target{
				{Scope: "global", Path: t.TempDir() + "/f.md", Mode: "replace-section", Section: "S"},
			},
		},
	}
	// Out is nil — NewEngine must normalise to io.Discard
	engine := adapter.NewEngine(adapters, packs, nil, adapter.Options{Scope: "global"})
	require.NoError(t, engine.Run())
}
```

- [ ] **Step 2: Verify it fails to build**

```bash
cd d:/projects/sap-devs-cli && go build ./internal/adapter/...
```

Expected: compile errors — `NewEngine` has wrong number/type of arguments.

- [ ] **Step 3: Rewrite `internal/adapter/engine.go`**

Replace the entire file:

```go
// internal/adapter/engine.go
package adapter

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
)

// Options controls inject scope, filtering, dry-run, and stats behaviour.
type Options struct {
	Scope      string    // "global" | "project"
	ToolFilter string    // if non-empty, only run this adapter ID
	DryRun     bool
	Stats      bool
	Out        io.Writer // for stats/warning output; nil → io.Discard
}

// Engine runs injection for a set of adapters, rendering per-adapter with its own budget.
type Engine struct {
	adapters []Adapter
	packs    []*content.Pack
	profile  *content.Profile
	opts     Options
}

// adapterStats records what was injected for one adapter.
type adapterStats struct {
	AdapterID    string
	PackIDs      []string // IDs of packs included after TrimPacks
	ApproxTokens int      // len(rendered) / 4
	BudgetTokens int      // adapter.MaxTokens; 0 = unconstrained
	Trimmed      bool     // true if any packs were dropped by TrimPacks
}

// NewEngine constructs an Engine. A nil Out is normalised to io.Discard.
func NewEngine(adapters []Adapter, packs []*content.Pack, profile *content.Profile, opts Options) *Engine {
	if opts.Out == nil {
		opts.Out = io.Discard
	}
	return &Engine{adapters: adapters, packs: packs, profile: profile, opts: opts}
}

// Run dispatches to the appropriate handler for each adapter.
func (e *Engine) Run() error {
	var stats []adapterStats

	for _, a := range e.adapters {
		if e.opts.ToolFilter != "" && a.ID != e.opts.ToolFilter {
			continue
		}
		maxBytes := a.MaxTokens * 4
		trimmed := content.TrimPacks(e.packs, maxBytes)
		if len(trimmed) == 0 && maxBytes > 0 {
			fmt.Fprintf(e.opts.Out, "sap-devs: adapter %s: budget too small to include any pack content\n", a.ID)
			continue
		}
		ctx := content.RenderContext(trimmed, e.profile)

		if e.opts.Stats {
			packIDs := make([]string, len(trimmed))
			for i, p := range trimmed {
				packIDs[i] = p.ID
			}
			stats = append(stats, adapterStats{
				AdapterID:    a.ID,
				PackIDs:      packIDs,
				ApproxTokens: len(ctx) / 4,
				BudgetTokens: a.MaxTokens,
				Trimmed:      len(trimmed) < len(e.packs),
			})
		}

		switch a.Type {
		case "file-inject":
			if err := e.runFileInject(a, ctx); err != nil {
				return fmt.Errorf("adapter %s: %w", a.ID, err)
			}
		case "clipboard-export":
			// clipboard-export is only for global scope
			if e.opts.Scope == "project" {
				continue
			}
			if err := ExportToClipboard(ctx, a.Instructions, e.opts.DryRun); err != nil {
				return fmt.Errorf("adapter %s: %w", a.ID, err)
			}
		case "mcp-wire":
			// mcp-wire is handled by the mcp command; inject skips it
		}
	}

	if e.opts.Stats && len(stats) > 0 {
		printStats(e.opts.Out, stats)
	}
	return nil
}

func (e *Engine) runFileInject(a Adapter, ctx string) error {
	for _, target := range a.Targets {
		if target.Scope != e.opts.Scope {
			continue
		}
		path, err := ExpandHome(target.Path)
		if err != nil {
			return fmt.Errorf("target %s: %w", target.Path, err)
		}
		switch target.Mode {
		case "replace-section":
			if err := ReplaceSection(path, target.Section, ctx, e.opts.DryRun); err != nil {
				return fmt.Errorf("target %s: %w", target.Path, err)
			}
		default:
			return fmt.Errorf("target %s: unknown mode %q", target.Path, target.Mode)
		}
	}
	return nil
}

func printStats(w io.Writer, stats []adapterStats) {
	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	fmt.Fprintln(tw, "Adapter\tPacks included\tTokens (approx)\tBudget")
	for _, s := range stats {
		budget := "unconstrained"
		if s.BudgetTokens > 0 {
			budget = fmt.Sprintf("%d tokens", s.BudgetTokens)
		}
		packs := strings.Join(s.PackIDs, ", ")
		if packs == "" {
			packs = "(none)"
		}
		fmt.Fprintf(tw, "%s\t%s\t~%d\t%s\n", s.AdapterID, packs, s.ApproxTokens, budget)
	}
	tw.Flush()
}
```

- [ ] **Step 4: Fix the four existing `NewEngine` call sites in `internal/adapter/adapter_test.go`**

The four old calls all pass a `string` as the second argument. Replace them with `nil` packs and `nil` profile (the tests use file-inject with no content assertions, so nil packs produce a header-only render which is fine):

Line 88 (`TestEngine_FileInject_DryRun`):
```go
// Before:
engine := adapter.NewEngine(adapters, "# SAP Context\nUse CAP.", adapter.Options{
	Scope:  "global",
	DryRun: true,
})
// After:
engine := adapter.NewEngine(adapters, nil, nil, adapter.Options{
	Scope:  "global",
	DryRun: true,
})
```

Line 112 (`TestEngine_SkipsWrongScope`):
```go
// Before:
engine := adapter.NewEngine(adapters, "content", adapter.Options{Scope: "global"})
// After:
engine := adapter.NewEngine(adapters, nil, nil, adapter.Options{Scope: "global"})
```

Line 136 (`TestEngine_FilterByTool`):
```go
// Before:
engine := adapter.NewEngine(adapters, "content", adapter.Options{Scope: "global", ToolFilter: "tool-a"})
// After:
engine := adapter.NewEngine(adapters, nil, nil, adapter.Options{Scope: "global", ToolFilter: "tool-a"})
```

Line 155 (`TestEngine_ClipboardSkippedForProject`):
```go
// Before:
engine := adapter.NewEngine(adapters, "content", adapter.Options{Scope: "project"})
// After:
engine := adapter.NewEngine(adapters, nil, nil, adapter.Options{Scope: "project"})
```

- [ ] **Step 5: Build and vet**

```bash
cd d:/projects/sap-devs-cli && go build ./... && go vet ./...
```

Expected: clean. All adapter tests and existing content/cmd tests compile.

- [ ] **Step 6: Commit**

```bash
cd d:/projects/sap-devs-cli && git add internal/adapter/engine.go internal/adapter/adapter_test.go && git commit -m "feat(adapter): refactor Engine to render per-adapter with budget trimming and stats"
```

---

## Task 5: cmd wiring

Update the cmd layer to remove the pre-render step, update `newAdapterEngine`'s signature, and wire the `--stats` flag.

**Files:**
- Modify: `cmd/inject.go`
- Modify: `cmd/inject_test.go`
- Modify: `cmd/root.go`

- [ ] **Step 1: Update `cmd/inject_test.go`**

The existing test pre-renders and passes the string. Replace with the new signature that passes packs directly:

```go
// cmd/inject_test.go
package cmd_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/adapter"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
)

// TestInjectEndToEnd tests ReplaceSection → file content round-trip
// without invoking the full cobra command (avoids XDG path dependencies in CI).
func TestInjectEndToEnd(t *testing.T) {
	dir := t.TempDir()
	claudeMD := filepath.Join(dir, "CLAUDE.md")

	// Simulate existing CLAUDE.md
	require.NoError(t, os.WriteFile(claudeMD, []byte("# My Project\n\nMy notes.\n"), 0644))

	packs := []*content.Pack{
		{ID: "cap", Name: "CAP", ContextMD: "## SAP CAP\n\nUse @sap/cds for data models."},
	}

	adapters := []adapter.Adapter{
		{
			ID:   "claude-code",
			Type: "file-inject",
			Targets: []adapter.Target{
				{Scope: "global", Path: claudeMD, Mode: "replace-section", Section: "SAP Developer Context"},
			},
		},
	}
	engine := adapter.NewEngine(adapters, packs, nil, adapter.Options{Scope: "global"})
	require.NoError(t, engine.Run())

	// Verify output
	data, err := os.ReadFile(claudeMD)
	require.NoError(t, err)
	result := string(data)

	assert.Contains(t, result, "# My Project")
	assert.Contains(t, result, "My notes.")
	assert.Contains(t, result, "<!-- sap-devs:start:SAP Developer Context -->")
	assert.Contains(t, result, "Use @sap/cds for data models.")
	assert.Contains(t, result, "<!-- sap-devs:end:SAP Developer Context -->")

	// Second run — idempotent
	require.NoError(t, engine.Run())
	data2, err := os.ReadFile(claudeMD)
	require.NoError(t, err)
	result2 := string(data2)
	assert.Equal(t, 1, strings.Count(result2, "<!-- sap-devs:start:SAP Developer Context -->"))
	assert.Contains(t, result2, "# My Project")
	assert.Contains(t, result2, "My notes.")
}
```

- [ ] **Step 2: Verify it fails to build**

```bash
cd d:/projects/sap-devs-cli && go build ./cmd/...
```

Expected: compile error — `newAdapterEngine` still has old signature.

- [ ] **Step 3: Update `newAdapterEngine` in `cmd/root.go`**

Find the current `newAdapterEngine` function (around line 168–175) and replace it:

```go
// newAdapterEngine constructs an adapter engine from all configured adapter layers.
func newAdapterEngine(packs []*content.Pack, profile *content.Profile, opts adapter.Options) (*adapter.Engine, error) {
	allAdapters, err := loadAdapters()
	if err != nil {
		return nil, err
	}
	return adapter.NewEngine(allAdapters, packs, profile, opts), nil
}
```

Ensure `"github.tools.sap/developer-relations/sap-devs-cli/internal/content"` is in the `root.go` imports (it should already be present).

- [ ] **Step 4: Update `cmd/inject.go`**

**Remove** the pre-render line (around line 96):
```go
rendered := content.RenderContext(packs, activeProfile)  // DELETE THIS LINE
```

**Add** `injectStats` variable alongside the existing inject vars:
```go
var injectStats bool
```

**Replace** the `opts` block and `newAdapterEngine` call:
```go
opts := adapter.Options{
	Scope:      scope,
	ToolFilter: injectTool,
	DryRun:     injectDryRun,
	Stats:      injectStats,
	Out:        cmd.OutOrStdout(),
}
eng, err := newAdapterEngine(packs, activeProfile, opts)
```

**Add** the flag registration in `init()`:
```go
injectCmd.Flags().BoolVar(&injectStats, "stats", false, "show injection stats per adapter")
```

Remove the now-unused `content` import if `RenderContext` was the only usage. Check whether `content` is still used elsewhere in `inject.go` (it is used for `[]*content.Pack` in the type, and `i18n.ActiveLang` from `i18n`). The `content` package is used indirectly via `loader.LoadPacks` — the import should remain since `packs` is `[]*content.Pack`.

- [ ] **Step 5: Build and vet**

```bash
cd d:/projects/sap-devs-cli && go build ./... && go vet ./...
```

Expected: clean.

- [ ] **Step 6: Commit**

```bash
cd d:/projects/sap-devs-cli && git add cmd/inject.go cmd/inject_test.go cmd/root.go && git commit -m "feat(cmd): wire per-adapter packs/profile into engine and add --stats flag"
```

---

## Done

All five tasks complete. Verify the full build one final time:

```bash
cd d:/projects/sap-devs-cli && go build ./... && go vet ./...
```

The feature is ready for CI to run the full test suite. Follow-on work (adding `max_tokens` to adapter YAMLs and `overlaps` to pack YAMLs where real overlaps exist) is tracked in `TODO.md` under "Inject Enhancements" and requires no code changes.
