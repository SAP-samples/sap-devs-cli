# Inject Verbosity Modes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add semantic verbosity tagging to context.md files so adapters control injection density via `minimal`/`standard`/`full` levels.

**Architecture:** New `VerbositySections` type parses `<!-- verbosity:{level} -->` HTML comment markers in context.md/constraints.md at load time, splitting content into core/detail/extended buckets. `RenderContext` and `TrimPacks` accept a verbosity parameter to assemble only the appropriate tiers. Adapters declare their level in YAML; a CLI flag overrides.

**Tech Stack:** Go, cobra, stretchr/testify

**Spec:** `docs/superpowers/specs/2026-04-19-inject-verbosity-modes-design.md`

---

### Task 1: VerbositySections type and parser

**Files:**
- Create: `internal/content/verbosity.go`
- Create: `internal/content/verbosity_test.go`

- [ ] **Step 1: Write failing tests for ParseVerbositySections**

```go
package content_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/SAP-samples/sap-devs-cli/internal/content"
)

func TestParseVerbositySections_NoMarkers(t *testing.T) {
	md := "## Title\n\nAll content here.\n"
	v := content.ParseVerbositySections(md)
	assert.Equal(t, md, v.Core)
	assert.Empty(t, v.Detail)
	assert.Empty(t, v.Extended)
}

func TestParseVerbositySections_AllThreeTiers(t *testing.T) {
	md := "Core content.\n<!-- verbosity:detail -->\nDetail content.\n<!-- verbosity:extended -->\nExtended content.\n"
	v := content.ParseVerbositySections(md)
	assert.Equal(t, "Core content.\n", v.Core)
	assert.Equal(t, "Detail content.\n", v.Detail)
	assert.Equal(t, "Extended content.\n", v.Extended)
}

func TestParseVerbositySections_CoreReset(t *testing.T) {
	md := "A.\n<!-- verbosity:detail -->\nB.\n<!-- verbosity:core -->\nC.\n"
	v := content.ParseVerbositySections(md)
	assert.Equal(t, "A.\nC.\n", v.Core)
	assert.Equal(t, "B.\n", v.Detail)
	assert.Empty(t, v.Extended)
}

func TestParseVerbositySections_AdjacentMarkers(t *testing.T) {
	md := "Core.\n<!-- verbosity:detail -->\n<!-- verbosity:extended -->\nExtended.\n"
	v := content.ParseVerbositySections(md)
	assert.Equal(t, "Core.\n", v.Core)
	assert.Empty(t, v.Detail)
	assert.Equal(t, "Extended.\n", v.Extended)
}

func TestParseVerbositySections_EmptyInput(t *testing.T) {
	v := content.ParseVerbositySections("")
	assert.Empty(t, v.Core)
	assert.Empty(t, v.Detail)
	assert.Empty(t, v.Extended)
}

func TestParseVerbositySections_MarkersStrippedFromOutput(t *testing.T) {
	md := "Core.\n<!-- verbosity:detail -->\nDetail.\n"
	v := content.ParseVerbositySections(md)
	assert.NotContains(t, v.Core, "<!-- verbosity")
	assert.NotContains(t, v.Detail, "<!-- verbosity")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/content/ -run TestParseVerbositySections -v`
Expected: compilation error — `ParseVerbositySections` not defined

- [ ] **Step 3: Write failing tests for AtLevel**

Add to `internal/content/verbosity_test.go`:

```go
func TestVerbositySections_AtLevel_Minimal(t *testing.T) {
	v := content.VerbositySections{Core: "C.", Detail: "D.", Extended: "E."}
	assert.Equal(t, "C.", v.AtLevel("minimal"))
}

func TestVerbositySections_AtLevel_Standard(t *testing.T) {
	v := content.VerbositySections{Core: "C.", Detail: "D.", Extended: "E."}
	assert.Equal(t, "C.D.", v.AtLevel("standard"))
}

func TestVerbositySections_AtLevel_Full(t *testing.T) {
	v := content.VerbositySections{Core: "C.", Detail: "D.", Extended: "E."}
	assert.Equal(t, "C.D.E.", v.AtLevel("full"))
}

func TestVerbositySections_AtLevel_EmptyDefault(t *testing.T) {
	v := content.VerbositySections{Core: "C.", Detail: "D.", Extended: "E."}
	assert.Equal(t, "C.D.E.", v.AtLevel(""), "empty string defaults to full")
}

func TestVerbositySections_AtLevel_EmptyTiers(t *testing.T) {
	v := content.VerbositySections{Core: "C.", Detail: "", Extended: "E."}
	assert.Equal(t, "C.E.", v.AtLevel("full"))
	assert.Equal(t, "C.", v.AtLevel("standard"))
}
```

- [ ] **Step 4: Implement VerbositySections, ParseVerbositySections, and AtLevel**

Create `internal/content/verbosity.go`:

```go
package content

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

type VerbositySections struct {
	Core     string
	Detail   string
	Extended string
}

var reVerbosityMarker = regexp.MustCompile(`(?m)^<!--\s*verbosity:(\w+)\s*-->\n?`)

func ParseVerbositySections(md string) VerbositySections {
	var v VerbositySections
	currentLevel := "core"
	lastEnd := 0

	for _, loc := range reVerbosityMarker.FindAllStringSubmatchIndex(md, -1) {
		chunk := md[lastEnd:loc[0]]
		appendChunk(&v, currentLevel, chunk)

		level := md[loc[2]:loc[3]]
		switch level {
		case "core", "detail", "extended":
			currentLevel = level
		default:
			fmt.Fprintf(os.Stderr, "sap-devs: unknown verbosity level %q, treating as core\n", level)
			currentLevel = "core"
		}
		lastEnd = loc[1]
	}

	if lastEnd < len(md) {
		appendChunk(&v, currentLevel, md[lastEnd:])
	}
	return v
}

func appendChunk(v *VerbositySections, level, chunk string) {
	switch level {
	case "core":
		v.Core += chunk
	case "detail":
		v.Detail += chunk
	case "extended":
		v.Extended += chunk
	}
}

func (v VerbositySections) AtLevel(level string) string {
	switch level {
	case "minimal":
		return v.Core
	case "standard":
		return v.Core + v.Detail
	default:
		return v.Core + v.Detail + v.Extended
	}
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/content/ -run "TestParseVerbositySections|TestVerbositySections_AtLevel" -v`
Expected: all PASS

- [ ] **Step 6: Commit**

```bash
git add internal/content/verbosity.go internal/content/verbosity_test.go
git commit -m "feat: add VerbositySections type and parser for context.md markers"
```

---

### Task 2: Pack struct migration — ContextMD → Context, ConstraintsMD → Constraints

**Files:**
- Modify: `internal/content/pack.go:14-58` (Pack struct) and `internal/content/pack.go:326-394` (LoadPack)
- Modify: `internal/content/merge.go:27-43` (MergeWith context/constraints)
- Modify: `internal/content/render.go:28-151` (RenderContext) and `internal/content/render.go:221-269` (TrimPacks)
- Modify: `internal/content/render_test.go` (all tests referencing ContextMD/ConstraintsMD)
- Modify: `internal/content/merge_test.go` (tests referencing ContextMD/ConstraintsMD)
- Modify: `internal/content/pack_test.go` (tests referencing ContextMD/ConstraintsMD)

This task does NOT add the verbosity parameter yet — it only migrates the struct fields so the codebase compiles. RenderContext and TrimPacks will use `.AtLevel("full")` to preserve existing behaviour.

- [ ] **Step 1: Update Pack struct in pack.go**

In `internal/content/pack.go`, change the Pack struct:

```go
// Replace:
ContextMD  string
// With:
Context  VerbositySections

// Replace:
ConstraintsMD string
// With:
Constraints VerbositySections
```

- [ ] **Step 2: Update LoadPack in pack.go**

In `internal/content/pack.go` LoadPack function, change the context loading:

```go
// Replace (line ~381):
	pack.ContextMD = string(data)
// With:
	pack.Context = ParseVerbositySections(string(data))

// Replace (line ~393):
	pack.ConstraintsMD = string(data)
// With:
	pack.Constraints = ParseVerbositySections(string(data))
```

- [ ] **Step 3: Update MergeWith in merge.go**

Replace the ContextMD and ConstraintsMD merge blocks (lines 27-43) with per-tier merge:

```go
// Replace the ContextMD block (lines 28-34):
if a.Context != (VerbositySections{}) {
	if a.AdditivePosition == "before" {
		merged.Context = mergeVerbositySections(a.Context, base.Context)
	} else {
		merged.Context = mergeVerbositySections(base.Context, a.Context)
	}
}

// Replace the ConstraintsMD block (lines 37-43):
if a.Constraints != (VerbositySections{}) {
	if a.AdditivePosition == "before" {
		merged.Constraints = mergeVerbositySections(a.Constraints, base.Constraints)
	} else {
		merged.Constraints = mergeVerbositySections(base.Constraints, a.Constraints)
	}
}
```

Add helper in `internal/content/verbosity.go`:

```go
func mergeVerbositySections(first, second VerbositySections) VerbositySections {
	return VerbositySections{
		Core:     joinNonEmpty(first.Core, second.Core),
		Detail:   joinNonEmpty(first.Detail, second.Detail),
		Extended: joinNonEmpty(first.Extended, second.Extended),
	}
}

func joinNonEmpty(a, b string) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	return a + "\n\n" + b
}
```

- [ ] **Step 4: Update RenderContext in render.go — use AtLevel("full")**

In `internal/content/render.go`, update RenderContext to use the new struct (preserving existing behaviour with `"full"`):

```go
// Replace constraints gathering (lines 80-84):
var constraints []string
for _, p := range packs {
	if trimmed := strings.TrimSpace(p.Constraints.AtLevel("full")); trimmed != "" {
		constraints = append(constraints, trimmed)
	}
}

// Replace pack content loop (lines 91-97):
for _, p := range packs {
	ctx := strings.TrimSpace(p.Context.AtLevel("full"))
	if ctx == "" {
		continue
	}
	b.WriteString(ctx)
	b.WriteString("\n\n")
}
```

- [ ] **Step 5: Update TrimPacks in render.go — use AtLevel("full")**

In `internal/content/render.go`, update trimNonBase budget calculation (line 261):

```go
// Replace:
size := len(p.ContextMD) + len(p.ConstraintsMD)
// With:
size := len(p.Context.AtLevel("full")) + len(p.Constraints.AtLevel("full"))
```

- [ ] **Step 6: Update all tests referencing ContextMD → Context and ConstraintsMD → Constraints**

Two categories of replacement across **all** test files:

**Category A — Struct literal initializers:**
```go
ContextMD: "..."      →  Context: content.VerbositySections{Core: "..."}
ConstraintsMD: "..."  →  Constraints: content.VerbositySections{Core: "..."}
```

**Category B — Field access in assertions:**
```go
result.ContextMD       →  result.Context.Core
result.ConstraintsMD   →  result.Constraints.Core
pack.ContextMD         →  pack.Context.Core
p.ContextMD            →  p.Context.Core
p.ConstraintsMD        →  p.Constraints.Core
assert.Empty(t, x.ContextMD)  →  assert.Equal(t, content.VerbositySections{}, x.Context)
```

**Files to update** (all have direct references):
- `internal/content/render_test.go` — struct literals (Category A)
- `internal/content/merge_test.go` — struct literals (A) and assertion field access (B)
- `internal/content/pack_test.go` — struct literals (A) and assertion field access (B)
- `internal/content/loader_test.go` — assertion field access `p.ContextMD` (B)
- `internal/adapter/adapter_test.go` — struct literals (A)
- `internal/adapter/status_test.go` — struct literals (A)
- `cmd/inject_test.go` — struct literals (A)

Note: All test data currently has no verbosity markers, so all content is core. This preserves behaviour.

- [ ] **Step 7: Find and update any remaining references to ContextMD or ConstraintsMD**

Run: `grep -rn "ContextMD\|ConstraintsMD" internal/ cmd/`

Update every remaining reference. Key locations already covered:
- `internal/adapter/engine.go` — `renderSectionContent` does not reference pack fields directly (it calls `TrimPacks` + `RenderContext`), so no change needed here yet.
- `cmd/inject.go` — does not reference pack content fields directly.

- [ ] **Step 8: Verify the build compiles**

Run: `go build ./...`
Expected: clean build, no errors

- [ ] **Step 9: Run all tests**

Run: `go test ./internal/content/... -v`
Expected: all PASS (behaviour unchanged — everything uses `AtLevel("full")`)

- [ ] **Step 10: Commit**

```bash
git add internal/content/pack.go internal/content/merge.go internal/content/render.go internal/content/verbosity.go internal/content/render_test.go internal/content/merge_test.go internal/content/pack_test.go
git commit -m "refactor: migrate Pack.ContextMD/ConstraintsMD to VerbositySections"
```

---

### Task 3: Thread verbosity through RenderContext and TrimPacks

**Files:**
- Modify: `internal/content/render.go:28` (RenderContext signature)
- Modify: `internal/content/render.go:221` (TrimPacks signature)
- Modify: `internal/content/render_test.go` (update all callers)
- Modify: `internal/adapter/engine.go:86,103,249,253` (callers of TrimPacks and RenderContext)

- [ ] **Step 1: Write failing test for verbosity-filtered rendering**

Add to `internal/content/render_test.go`:

```go
func TestRenderContext_VerbosityMinimal_ExcludesDetailAndExtended(t *testing.T) {
	packs := []*content.Pack{
		{ID: "cap", Context: content.VerbositySections{
			Core:     "## CAP\n\nCore stuff.",
			Detail:   "\n\n### Best Practices\n\nDetail stuff.",
			Extended: "\n\n### Release Notes\n\nExtended stuff.",
		}},
	}
	out := content.RenderContext(packs, nil, nil, "minimal")
	assert.Contains(t, out, "Core stuff.")
	assert.NotContains(t, out, "Detail stuff.")
	assert.NotContains(t, out, "Extended stuff.")
}

func TestRenderContext_VerbosityStandard_IncludesDetailExcludesExtended(t *testing.T) {
	packs := []*content.Pack{
		{ID: "cap", Context: content.VerbositySections{
			Core:     "## CAP\n\nCore stuff.",
			Detail:   "\n\n### Best Practices\n\nDetail stuff.",
			Extended: "\n\n### Release Notes\n\nExtended stuff.",
		}},
	}
	out := content.RenderContext(packs, nil, nil, "standard")
	assert.Contains(t, out, "Core stuff.")
	assert.Contains(t, out, "Detail stuff.")
	assert.NotContains(t, out, "Extended stuff.")
}

func TestRenderContext_VerbosityFull_IncludesAll(t *testing.T) {
	packs := []*content.Pack{
		{ID: "cap", Context: content.VerbositySections{
			Core:     "## CAP\n\nCore stuff.",
			Detail:   "\n\n### Best Practices\n\nDetail stuff.",
			Extended: "\n\n### Release Notes\n\nExtended stuff.",
		}},
	}
	out := content.RenderContext(packs, nil, nil, "full")
	assert.Contains(t, out, "Core stuff.")
	assert.Contains(t, out, "Detail stuff.")
	assert.Contains(t, out, "Extended stuff.")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/content/ -run "TestRenderContext_Verbosity" -v`
Expected: compilation error — wrong number of arguments

- [ ] **Step 3: Add verbosity parameter to RenderContext and TrimPacks**

In `internal/content/render.go`:

```go
// Change RenderContext signature (line 28):
func RenderContext(packs []*Pack, profile *Profile, dynamic *DynamicContext, verbosity string) string {

// Replace the constraints gathering to use verbosity:
var constraints []string
for _, p := range packs {
	if trimmed := strings.TrimSpace(p.Constraints.AtLevel(verbosity)); trimmed != "" {
		constraints = append(constraints, trimmed)
	}
}

// Replace the pack content loop to use verbosity:
for _, p := range packs {
	ctx := strings.TrimSpace(p.Context.AtLevel(verbosity))
	if ctx == "" {
		continue
	}
	b.WriteString(ctx)
	b.WriteString("\n\n")
}
```

Gate synthetic sections by verbosity:

```go
// Canonical Patterns — detail tier
if len(injectable) > 0 && (verbosity == "standard" || verbosity == "full" || verbosity == "") {
	// ... existing code ...
}

// Learning Journeys — extended tier
if len(learningRows) > 0 && (verbosity == "full" || verbosity == "") {
	// ... existing code ...
}

// Known Errors — extended tier
if len(knownErrors) > 0 && (verbosity == "full" || verbosity == "") {
	// ... existing code ...
}
```

Change TrimPacks:

```go
// Change TrimPacks signature (line 221):
func TrimPacks(packs []*Pack, maxBytes int, verbosity string) []*Pack {
	// ... existing base/nonBase separation ...
	return append(base, trimNonBase(nonBase, maxBytes, verbosity)...)
}

// Change trimNonBase signature (line 235):
func trimNonBase(packs []*Pack, maxBytes int, verbosity string) []*Pack {
	// ... existing dedup code ...
	// Replace budget size calculation (line 261):
	size := len(p.Context.AtLevel(verbosity)) + len(p.Constraints.AtLevel(verbosity))
	// ...
}
```

- [ ] **Step 4: Update all existing callers**

In `internal/content/render_test.go`, update all existing `RenderContext(packs, profile, dyn)` calls to `RenderContext(packs, profile, dyn, "full")`.

In `internal/content/render_test.go`, update all existing `TrimPacks(packs, n)` calls to `TrimPacks(packs, n, "full")`.

In `internal/adapter/engine.go`, update callers:
- Line 86: `content.TrimPacks(e.packs, maxBytes)` → `content.TrimPacks(e.packs, maxBytes, "full")`
- Line 103: `content.RenderContext(trimmed, e.profile, e.opts.Dynamic)` → `content.RenderContext(trimmed, e.profile, e.opts.Dynamic, "full")`
- Line 249: `content.TrimPacks(e.packs, maxBytes)` → `content.TrimPacks(e.packs, maxBytes, "full")`
- Line 253: `content.RenderContext(trimmed, e.profile, e.opts.Dynamic)` → `content.RenderContext(trimmed, e.profile, e.opts.Dynamic, "full")`

Note: These use `"full"` as a placeholder — Task 4 will thread the actual per-adapter verbosity. The `renderSectionContent` calls at lines 249/253 will be fully rewritten in Task 4 Step 5, so the `"full"` placeholder here is intentionally temporary.

- [ ] **Step 5: Verify build compiles**

Run: `go build ./...`
Expected: clean build

- [ ] **Step 6: Run all tests**

Run: `go test ./internal/content/... -v`
Expected: all PASS (including new verbosity tests)

- [ ] **Step 7: Commit**

```bash
git add internal/content/render.go internal/content/render_test.go internal/adapter/engine.go
git commit -m "feat: thread verbosity parameter through RenderContext and TrimPacks"
```

---

### Task 4: Adapter verbosity field and per-adapter resolution in engine

**Files:**
- Modify: `internal/adapter/adapter.go:11-25` (Adapter struct)
- Modify: `internal/adapter/engine.go:24-34` (Options struct)
- Modify: `internal/adapter/engine.go:44-52` (adapterStats struct)
- Modify: `internal/adapter/engine.go:63-161` (Run method)
- Modify: `internal/adapter/engine.go:241-255` (renderSectionContent)
- Modify: `internal/adapter/engine.go:360-384` (printStats)
- No change needed: `internal/adapter/status.go` — staleness logic lives in `engine.go`'s `renderSectionContent`, not in `status.go`

- [ ] **Step 1: Add Verbosity field to Adapter struct**

In `internal/adapter/adapter.go`:

```go
// Add after MaxBytes field (line 20):
Verbosity    string       `yaml:"verbosity,omitempty"` // "minimal" | "standard" | "full"; default "full"
```

- [ ] **Step 2: Add Verbosity to Options and adapterStats**

In `internal/adapter/engine.go`:

```go
// Add to Options struct (after Lang field):
Verbosity  string // CLI override; empty = use adapter default

// Add to adapterStats struct (after Trimmed field):
Verbosity  string
```

- [ ] **Step 3: Add resolveVerbosity helper**

Add to `internal/adapter/engine.go`:

```go
func resolveVerbosity(opts Options, a Adapter) string {
	if opts.Verbosity != "" {
		return opts.Verbosity
	}
	if a.Verbosity != "" {
		return a.Verbosity
	}
	return "full"
}
```

- [ ] **Step 4: Thread verbosity through Run()**

In `engine.go` Run method, immediately before the `TrimPacks` call (after maxBytes is computed but before it is used), resolve verbosity and pass it:

```go
// After maxBytes calculation:
verbosity := resolveVerbosity(e.opts, a)
trimmed := content.TrimPacks(e.packs, maxBytes, verbosity)

// Zero-pack early exit — record resolved verbosity:
if len(trimmed) == 0 && maxBytes > 0 {
	// ... existing stderr message ...
	if e.opts.Stats {
		stats = append(stats, adapterStats{
			AdapterID:   a.ID,
			BudgetBytes: maxBytes,
			Format:      a.Format,
			Trimmed:     true,
			Verbosity:   verbosity,
		})
	}
	continue
}
ctx := content.RenderContext(trimmed, e.profile, e.opts.Dynamic, verbosity)

// Stats recording — add Verbosity field:
stats = append(stats, adapterStats{
	// ... existing fields ...
	Verbosity:    verbosity,
})
```

- [ ] **Step 5: Thread verbosity through renderSectionContent**

Update `renderSectionContent` in `engine.go`:

```go
func (e *Engine) renderSectionContent(a Adapter) string {
	if e.packs == nil {
		return ""
	}
	verbosity := resolveVerbosity(e.opts, a)
	maxBytes := a.MaxBytes
	if maxBytes == 0 && a.MaxTokens > 0 {
		maxBytes = a.MaxTokens * 4
	}
	trimmed := content.TrimPacks(e.packs, maxBytes, verbosity)
	if len(trimmed) == 0 && maxBytes > 0 {
		return ""
	}
	ctx := content.RenderContext(trimmed, e.profile, e.opts.Dynamic, verbosity)
	return content.FormatOutput(ctx, a.Format)
}
```

- [ ] **Step 6: Add Verbosity column to printStats**

Update `printStats` in `engine.go`:

```go
func printStats(w io.Writer, stats []adapterStats) {
	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	fmt.Fprintln(tw, "Adapter\tPacks included\tTokens (approx)\tBudget (bytes)\tVerbosity\tFormat\tStatus")
	for _, s := range stats {
		// ... existing budget/packs/format/status logic ...
		fmt.Fprintf(tw, "%s\t%s\t~%d\t%s\t%s\t%s\t%s\n",
			s.AdapterID, packs, s.ApproxTokens, budget, s.Verbosity, format, status)
	}
	tw.Flush()
}
```

- [ ] **Step 7: Verify build compiles**

Run: `go build ./...`
Expected: clean build

- [ ] **Step 8: Run all tests**

Run: `go test ./... 2>/dev/null; go vet ./...`
Expected: go vet clean. (Tests may fail on Windows — go vet is the local authority.)

- [ ] **Step 9: Commit**

```bash
git add internal/adapter/adapter.go internal/adapter/engine.go
git commit -m "feat: add adapter verbosity field and per-adapter resolution in engine"
```

---

### Task 5: CLI flag --verbosity

**Files:**
- Modify: `cmd/inject.go:28-39` (flag variables)
- Modify: `cmd/inject.go:49-61` (validation)
- Modify: `cmd/inject.go:130-134` (status opts)
- Modify: `cmd/inject.go:292-299` (inject opts)
- Modify: `cmd/inject.go:443-454` (flag registration)

- [ ] **Step 1: Add flag variable**

In `cmd/inject.go`, add to the var block:

```go
injectVerbosity string
```

- [ ] **Step 2: Register the flag**

In the `init()` function:

```go
injectCmd.Flags().StringVar(&injectVerbosity, "verbosity", "", "verbosity level: minimal, standard, full (overrides adapter default)")
```

- [ ] **Step 3: Add validation**

In the `RunE` function, after existing validation:

```go
if injectVerbosity != "" && injectVerbosity != "minimal" && injectVerbosity != "standard" && injectVerbosity != "full" {
	return fmt.Errorf("--verbosity must be minimal, standard, or full")
}
```

- [ ] **Step 4: Thread into Options for inject path**

In `cmd/inject.go`, where `adapter.Options` is constructed (~line 292):

```go
opts := adapter.Options{
	Scope:      scope,
	ToolFilter: injectTool,
	DryRun:     injectDryRun,
	Stats:      injectStats,
	Out:        cmd.OutOrStdout(),
	Dynamic:    dynCtx,
	Verbosity:  injectVerbosity,
}
```

- [ ] **Step 5: Thread into Options for status path**

In `cmd/inject.go`, where status `adapter.Options` is constructed (~line 130):

```go
opts := adapter.Options{
	Scope:      scope,
	ToolFilter: injectTool,
	Lang:       lang,
	Verbosity:  injectVerbosity,
}
```

- [ ] **Step 6: Verify build compiles**

Run: `go build ./...`
Expected: clean build

- [ ] **Step 7: Smoke test**

Run: `SAP_DEVS_DEV=1 go run . inject --dry-run --verbosity minimal 2>/dev/null | head -5`
Expected: renders context output (even though content.md has no markers yet — all content is core at this point)

- [ ] **Step 8: Commit**

```bash
git add cmd/inject.go
git commit -m "feat: add --verbosity flag to inject command"
```

---

### Task 6: Tag content.md files with verbosity markers

**Files:**
- Modify: `content/packs/base/context.md`
- Modify: `content/packs/cap/context.md`

- [ ] **Step 1: Tag base/context.md**

Edit `content/packs/base/context.md` to add markers:

```markdown
## SAP Developer Ecosystem

### Key Portals
...existing content...

<!-- verbosity:detail -->
### Learning & Discovery
...existing content...

### Developer News & Community
...existing content...

### APIs & SDKs
...existing content...

### Support & Contribution
...existing content...

<!-- verbosity:core -->
## sap-devs CLI Reference (for AI agents)
...existing content (table stays core)...
```

- [ ] **Step 2: Tag cap/context.md**

Edit `content/packs/cap/context.md` to add markers:

```markdown
## SAP CAP (Cloud Application Programming Model)
...intro paragraph...

### Key Tools
...existing content...

### CDS Data Modelling
...existing content...

<!-- verbosity:detail -->
### Service Definition
...existing content...

### Best Practices
...existing content...

<!-- verbosity:extended -->
### Recent CAP Releases
...existing content (includes sync:fetch marker)...
```

- [ ] **Step 3: Smoke test — verbosity minimal**

Run: `SAP_DEVS_DEV=1 go run . inject --dry-run --verbosity minimal 2>/dev/null | grep -c "###"`
Expected: fewer H3 sections than full

- [ ] **Step 4: Smoke test — verbosity full**

Run: `SAP_DEVS_DEV=1 go run . inject --dry-run --verbosity full 2>/dev/null | grep -c "###"`
Expected: all H3 sections present

- [ ] **Step 5: Commit**

```bash
git add content/packs/base/context.md content/packs/cap/context.md
git commit -m "feat: add verbosity markers to base and cap context.md files"
```

---

### Task 7: Set verbosity on budget-constrained adapters

**Files:**
- Modify: `content/adapters/chatgpt.yaml`

- [ ] **Step 1: Add verbosity to chatgpt.yaml**

The ChatGPT adapter has `max_bytes: 1400` and uses `plain-prose` format. Set `verbosity: minimal` so the budget-constrained adapter gets only core content.

```yaml
id: chatgpt
name: ChatGPT
type: file-export
export_path: "~/sap-devs-chatgpt-context.md"
max_bytes: 1400
format: plain-prose
verbosity: minimal
instructions: "Paste into ChatGPT → Settings → Custom Instructions → 'What would you like ChatGPT to know about you?'"
```

- [ ] **Step 2: Commit**

```bash
git add content/adapters/chatgpt.yaml
git commit -m "feat: set verbosity: minimal on ChatGPT adapter"
```

---

### Task 8: Update CLAUDE.md documentation

**Files:**
- Modify: `CLAUDE.md`

- [ ] **Step 1: Update the Inject CLI command table**

In the CLAUDE.md CLI Commands table, update the `inject` row to mention `--verbosity`:

```
| `inject` | Push rendered context into detected AI tools (`--project` for project scope); `--sync` forces fresh sync, `--no-sync` skips staleness check, `--verbosity minimal\|standard\|full` overrides adapter verbosity |
```

- [ ] **Step 2: Add verbosity to the Content Layer System section**

Add a brief note after the Additive Layers paragraph:

```markdown
**Verbosity Tagging:** Sections within `context.md` (and `constraints.md`) can be tagged with `<!-- verbosity:core -->`, `<!-- verbosity:detail -->`, or `<!-- verbosity:extended -->` HTML comments. Adapters declare a `verbosity` field (`minimal`/`standard`/`full`, default `full`) that controls which tiers are included. `ParseVerbositySections` ([internal/content/verbosity.go](internal/content/verbosity.go)) splits content at load time; `RenderContext` assembles only the requested tiers.
```

- [ ] **Step 3: Update the Adapter System section**

Add `verbosity` to the adapter types description after the Format mention:

```
Adapters optionally specify a `verbosity` field to control content density: `minimal` (core only), `standard` (core + detail), or `full` (core + detail + extended, the default).
```

- [ ] **Step 4: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: document verbosity modes in CLAUDE.md"
```

---

### Task 9: Update TODO.md — mark feature as done

**Files:**
- Modify: `TODO.md`

- [ ] **Step 1: Move the verbosity modes item from backlog to done (or remove it)**

Mark the "Inject verbosity modes with semantic section tagging" item as complete.

- [ ] **Step 2: Commit**

```bash
git add TODO.md
git commit -m "docs: mark inject verbosity modes as completed in TODO.md"
```

---

### Task 10: Final verification

- [ ] **Step 1: Full build check**

Run: `go build ./... && go vet ./...`
Expected: clean build and vet

- [ ] **Step 2: Full integration smoke test**

Run: `SAP_DEVS_DEV=1 go run . inject --dry-run --verbosity minimal --tool claude-code 2>/dev/null`
Expected: reduced output — no "Best Practices", no "Release Notes", no "Learning & Discovery" sections

Run: `SAP_DEVS_DEV=1 go run . inject --dry-run --verbosity full --tool claude-code 2>/dev/null`
Expected: all sections present (identical to today's output)

- [ ] **Step 3: Stats smoke test**

Run: `SAP_DEVS_DEV=1 go run . inject --dry-run --stats --verbosity minimal --tool claude-code 2>/dev/null`
Expected: stats table shows Verbosity column with "minimal"

- [ ] **Step 4: Verify no regressions — default behaviour unchanged**

Run: `SAP_DEVS_DEV=1 go run . inject --dry-run --tool claude-code 2>/dev/null`
Expected: output identical to pre-change behaviour (all content included, default "full")
