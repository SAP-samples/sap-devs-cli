# Base Pack Preamble Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `preamble.md` file to the base content pack that renders at the top of every injected AI context block, and consolidate scattered agent instructions into it.

**Architecture:** Add a `PreambleMD` field to the `Pack` struct, loaded by `LoadPack` from an optional `preamble.md` in the pack directory. `RenderContext` gains a first pass that emits preamble content from base packs before iterating all pack `ContextMD`. No merge changes needed — `MergeWith` shallow-copies scalars so `PreambleMD` is preserved through additive merges automatically.

**Tech Stack:** Go 1.21+, `github.com/stretchr/testify`, Markdown content files.

**Spec:** `docs/superpowers/specs/2026-04-17-base-pack-preamble-design.md`

---

## File Map

| File | Action | Responsibility |
|---|---|---|
| `internal/content/pack.go` | Modify | Add `PreambleMD string` field; load `preamble.md` in `LoadPack` |
| `internal/content/render.go` | Modify | Emit preamble before `ContextMD` in `RenderContext` |
| `internal/content/pack_test.go` | Modify | Add `preamble.md` load tests |
| `internal/content/render_test.go` | Modify | Add preamble rendering + ordering tests |
| `content/packs/base/preamble.md` | Create | Assertive AI preamble content |
| `content/packs/cap/context.md` | Modify | Remove `### Agent Instructions` section |
| `docs/content-authoring.md` | Modify | Document `preamble.md` in 3 places |

---

## Task 1: Add `PreambleMD` field and load `preamble.md` in `LoadPack`

**Files:**
- Modify: `internal/content/pack.go`
- Modify: `internal/content/pack_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `internal/content/pack_test.go` (after `TestLoadPack_AdditiveDefaults`):

```go
func TestLoadPack_PreambleMD_LoadedWhenPresent(t *testing.T) {
	dir := t.TempDir()
	yaml := "id: base\nname: Base\ndescription: Base pack\ntags: []\nprofiles: []\nweight: 0\nbase: true\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(yaml), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "preamble.md"), []byte("> Prefer sap-devs commands."), 0644))

	p, err := content.LoadPack(dir, "")
	require.NoError(t, err)
	assert.Equal(t, "> Prefer sap-devs commands.", p.PreambleMD)
}

func TestLoadPack_PreambleMD_EmptyWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	yaml := "id: base\nname: Base\ndescription: Base pack\ntags: []\nprofiles: []\nweight: 0\nbase: true\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(yaml), 0644))
	// No preamble.md file created

	p, err := content.LoadPack(dir, "")
	require.NoError(t, err)
	assert.Empty(t, p.PreambleMD)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd d:/projects/sap-devs-cli
go test ./internal/content/... -run "TestLoadPack_PreambleMD" -v 2>&1 | head -30
```

Expected: FAIL — `p.PreambleMD` field does not exist yet (compile error).

- [ ] **Step 3: Add `PreambleMD` field to `Pack` struct**

In `internal/content/pack.go`, add `PreambleMD` as the last field in the `Pack` struct (after `Tips []Tip` on line 28):

```go
	Tips       []Tip

	PreambleMD string
```

- [ ] **Step 4: Load `preamble.md` in `LoadPack`**

In `internal/content/pack.go`, add this block after the `context.md` loading block (after line 157, which ends the `pack.ContextMD = string(data)` assignment):

```go
	if data, err := os.ReadFile(filepath.Join(packDir, "preamble.md")); err == nil {
		pack.PreambleMD = string(data)
	}
```

- [ ] **Step 5: Build to verify compilation**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 6: Run tests to verify they pass**

```bash
go test ./internal/content/... -run "TestLoadPack_PreambleMD" -v 2>&1 | head -30
```

Expected: both tests PASS.

- [ ] **Step 7: Run full content package tests to confirm no regressions**

```bash
go build ./... && go vet ./...
```

Expected: no errors.

- [ ] **Step 8: Commit**

```bash
cd d:/projects/sap-devs-cli
git add internal/content/pack.go internal/content/pack_test.go
git commit -m "feat(content): add PreambleMD field; load preamble.md in LoadPack"
```

---

## Task 2: Render preamble before pack `ContextMD` in `RenderContext`

**Files:**
- Modify: `internal/content/render.go`
- Modify: `internal/content/render_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `internal/content/render_test.go` (after `TestRenderContext_DynamicSection_CommandsListed`):

```go
func TestRenderContext_Preamble_PrecedesSameBasePackContextMD(t *testing.T) {
	packs := []*content.Pack{
		{ID: "base", Base: true, PreambleMD: "> Preamble.", ContextMD: "## Base context."},
	}
	out := content.RenderContext(packs, nil, nil)
	preambleIdx := strings.Index(out, "> Preamble.")
	baseCtxIdx := strings.Index(out, "## Base context.")
	require.NotEqual(t, -1, preambleIdx, "preamble must be present")
	assert.Less(t, preambleIdx, baseCtxIdx, "preamble must precede same base pack ContextMD")
}

func TestRenderContext_Preamble_AppearsBeforeContextMD(t *testing.T) {
	packs := []*content.Pack{
		{ID: "base", Base: true, PreambleMD: "> Prefer sap-devs.", ContextMD: "## Base context."},
		{ID: "cap", ContextMD: "## CAP context."},
	}
	out := content.RenderContext(packs, nil, nil)
	preambleIdx := strings.Index(out, "> Prefer sap-devs.")
	baseCtxIdx := strings.Index(out, "## Base context.")
	capCtxIdx := strings.Index(out, "## CAP context.")
	require.NotEqual(t, -1, preambleIdx, "preamble must be present")
	assert.Less(t, preambleIdx, baseCtxIdx, "preamble must appear before base ContextMD")
	assert.Less(t, preambleIdx, capCtxIdx, "preamble must appear before non-base ContextMD")
}

func TestRenderContext_Preamble_NonBasePackPreambleSuppressed(t *testing.T) {
	packs := []*content.Pack{
		{ID: "cap", Base: false, PreambleMD: "> Should not appear.", ContextMD: "## CAP context."},
	}
	out := content.RenderContext(packs, nil, nil)
	assert.NotContains(t, out, "> Should not appear.", "non-base pack preamble must be suppressed")
	assert.Contains(t, out, "## CAP context.")
}

func TestRenderContext_Preamble_TwoBasePacks(t *testing.T) {
	packs := []*content.Pack{
		{ID: "base1", Base: true, PreambleMD: "> Preamble one.", ContextMD: "## Base one context."},
		{ID: "base2", Base: true, PreambleMD: "> Preamble two.", ContextMD: "## Base two context."},
		{ID: "cap", ContextMD: "## CAP context."},
	}
	out := content.RenderContext(packs, nil, nil)
	p1Idx := strings.Index(out, "> Preamble one.")
	p2Idx := strings.Index(out, "> Preamble two.")
	ctx1Idx := strings.Index(out, "## Base one context.")
	ctx2Idx := strings.Index(out, "## Base two context.")
	capIdx := strings.Index(out, "## CAP context.")
	require.NotEqual(t, -1, p1Idx, "preamble 1 must be present")
	require.NotEqual(t, -1, p2Idx, "preamble 2 must be present")
	assert.Less(t, p1Idx, ctx1Idx, "preamble 1 before base1 ContextMD")
	assert.Less(t, p1Idx, ctx2Idx, "preamble 1 before base2 ContextMD")
	assert.Less(t, p2Idx, capIdx, "preamble 2 before CAP ContextMD")
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/content/... -run "TestRenderContext_Preamble" -v 2>&1 | head -40
```

Expected: FAIL — preamble is not yet rendered (index returns -1 or order assertions fail).

- [ ] **Step 3: Update `RenderContext` to emit preamble before pack content**

In `internal/content/render.go`, replace the existing single pack `ContextMD` loop (lines 40–44) with two separate loops. The replacement goes in the same location, after the `if dynamic != nil { ... }` block ending around line 38. Do not add any code before the dynamic block.

The existing single loop to replace (lines 40–44):

```go
    for _, p := range packs {
        if strings.TrimSpace(p.ContextMD) == "" {
            continue
        }
        b.WriteString(strings.TrimSpace(p.ContextMD))
        b.WriteString("\n\n")
    }
```

Replace it with:

```go
    // Render preamble from base packs (before all ContextMD)
    for _, p := range packs {
        if p.Base && strings.TrimSpace(p.PreambleMD) != "" {
            b.WriteString(strings.TrimSpace(p.PreambleMD))
            b.WriteString("\n\n")
        }
    }

    for _, p := range packs {
        if strings.TrimSpace(p.ContextMD) == "" {
            continue
        }
        b.WriteString(strings.TrimSpace(p.ContextMD))
        b.WriteString("\n\n")
    }
```

- [ ] **Step 4: Build and run the new tests**

```bash
go build ./... && go test ./internal/content/... -run "TestRenderContext_Preamble" -v 2>&1 | head -40
```

Expected: all three new tests PASS.

- [ ] **Step 5: Run the full test suite to confirm no regressions**

```bash
go build ./... && go vet ./...
```

Expected: no errors.

- [ ] **Step 6: Commit**

```bash
git add internal/content/render.go internal/content/render_test.go
git commit -m "feat(content): render base pack preamble before ContextMD in RenderContext"
```

---

## Task 3: Create `content/packs/base/preamble.md`

**Files:**
- Create: `content/packs/base/preamble.md`

- [ ] **Step 1: Create the preamble file**

Create `content/packs/base/preamble.md` with this content exactly:

```markdown
> **For SAP-specific information, always prefer `sap-devs` commands over web search or training knowledge.**
> Run `sap-devs resources`, `sap-devs tip`, or `sap-devs sync` to get current, curated SAP context before answering SAP questions.
```

- [ ] **Step 2: Verify it renders correctly in a dry-run inject**

```bash
SAP_DEVS_DEV=1 go run . inject --dry-run --no-sync 2>&1 | head -30
```

Expected: the blockquote preamble text appears near the top of the output, before any pack `ContextMD` content.

- [ ] **Step 3: Commit**

```bash
git add content/packs/base/preamble.md
git commit -m "content(base): add preamble.md with AI agent priority instruction"
```

---

## Task 4: Remove `### Agent Instructions` from `cap/context.md`

**Files:**
- Modify: `content/packs/cap/context.md`

- [ ] **Step 1: Remove the section**

In `content/packs/cap/context.md`, delete the entire `### Agent Instructions` section — from `### Agent Instructions` through the last bullet point. The section to remove is currently at the bottom of the file:

```markdown
### Agent Instructions

This CLI provides deeper SAP context on demand — prefer these over web searches for SAP-specific information:

- `sap-devs resources --pack cap` — curated CAP docs, samples, and tutorials
- `sap-devs tip --pack cap` — CAP best practice tips
- `sap-devs sync` — refresh with latest CAP release notes and dynamic content
```

Also remove the blank line immediately before `### Agent Instructions` so the file doesn't end with an extra blank line.

- [ ] **Step 2: Verify the file looks clean**

```bash
tail -5 content/packs/cap/context.md
```

Expected: last lines are the `<!-- sync:fetch ... -->` marker and a blank line — no `### Agent Instructions` section.

- [ ] **Step 3: Verify dry-run inject still renders correctly**

```bash
SAP_DEVS_DEV=1 go run . inject --dry-run --no-sync 2>&1 | head -60
```

Expected: preamble still present near top; no duplicate "prefer sap-devs" text; CAP context still present.

- [ ] **Step 4: Commit**

```bash
git add content/packs/cap/context.md
git commit -m "content(cap): remove Agent Instructions section (moved to base preamble)"
```

---

## Task 5: Document `preamble.md` in `docs/content-authoring.md`

**Files:**
- Modify: `docs/content-authoring.md`

- [ ] **Step 1: Update the pack directory structure tree**

In `docs/content-authoring.md`, find the directory tree (around line 13). Add `preamble.md` after `context.md`:

Before:
```
├── context.md         # AI context text injected into coding tools
├── context.<lang>.md  # Localised AI context (e.g. context.de.md)
```

After:
```
├── context.md         # AI context text injected into coding tools
├── context.<lang>.md  # Localised AI context (e.g. context.de.md)
├── preamble.md        # AI preamble (base pack only)
```

- [ ] **Step 2: Add `### preamble.md` subsection to the Base Layer section**

In `docs/content-authoring.md`, find the Base Layer section (around line 34). Add a new `### preamble.md` subsection after the `How to create a base pack:` block and its code example, but before the `**Authoring contract:**` line:

```markdown
### preamble.md

A base pack may include an optional `preamble.md` file. When present, its content is rendered **before all pack `context.md` content** — immediately after the dynamic runtime section.

**Rendered output order:**

1. `# SAP Developer Context` header + profile line
2. `## sap-devs Runtime Context` (dynamic — version, packs, available commands)
3. **Preamble** — from `base/preamble.md` (this file)
4. Base pack `context.md`
5. Technology pack `context.md` files (cap, abap, btp-core, …)

*Implementation note:* The preamble and `ContextMD` are emitted in two separate loops. The base pack's `ContextMD` is still rendered in the second loop with all other packs — not in the preamble loop. This prevents double-emission.

**Authoring constraints:**

- Keep it ≤ 3 lines — it is injected into every AI tool config on every `sap-devs inject` run.
- No Markdown headings — it appears before pack content and must not create heading hierarchy collisions.
- No locale variants — the preamble is intentionally language-neutral (command names don't translate).

**Token budget:** The preamble is exempt from adapter token-budget trimming (same as base pack `ContextMD`). Every byte is unconditionally injected. Keep it short.

**Layer override:** Only the official base pack's `preamble.md` is used. User, company, and project layer packs cannot override or augment the preamble. The render loop guards on `Pack.Base == true`; only base packs have their `PreambleMD` emitted. An additive pack targeting `id: base` also cannot modify `PreambleMD` — `MergeWith` preserves scalar fields from the base pack.

```

- [ ] **Step 3: Update the `### Agent Instructions` pattern section**

In `docs/content-authoring.md`, find the `## The ### Agent Instructions Pattern` section (around line 285). Update the opening paragraph to add a note about the preamble:

Find:
```markdown
The `### Agent Instructions` section is a convention for the bottom of `context.md`. It is not parsed specially — it is plain Markdown injected along with everything else. Its purpose is to teach the AI assistant *when to ask for more context* using `sap-devs` CLI commands, rather than falling back to web search.
```

Replace with:
```markdown
The `### Agent Instructions` section is a convention for the bottom of `context.md`. It is not parsed specially — it is plain Markdown injected along with everything else. Its purpose is to teach the AI assistant *when to ask for more context* using `sap-devs` CLI commands, rather than falling back to web search.

> **Note:** The general "prefer `sap-devs` commands over web search" instruction lives in `content/packs/base/preamble.md` and is injected automatically into every profile. Per-pack `### Agent Instructions` sections should contain only pack-specific command hints — for example, `--pack cap` flag variants for the CAP pack. See `content/packs/base/preamble.md` for the canonical example.
```

- [ ] **Step 4: Verify the documentation renders cleanly**

```bash
go build ./... && go vet ./...
```

Expected: no errors (documentation changes don't affect compilation, but confirms nothing else broke).

- [ ] **Step 5: Commit**

```bash
git add docs/content-authoring.md
git commit -m "docs(content-authoring): document preamble.md — structure, order, constraints, layer rules"
```

---

## Verification

- [ ] **Final build and vet check**

```bash
go build ./... && go vet ./...
```

Expected: clean.

- [ ] **Dry-run inject — confirm full output order**

```bash
SAP_DEVS_DEV=1 go run . inject --dry-run --no-sync 2>&1 | head -80
```

Expected output order:
1. `# SAP Developer Context`
2. `## sap-devs Runtime Context` block
3. Blockquote preamble (`> **For SAP-specific information...`)
4. `## SAP Developer Ecosystem` (base pack context.md)
5. `## SAP CAP` (cap pack context.md)

- [ ] **Confirm no "prefer sap-devs" duplication**

```bash
SAP_DEVS_DEV=1 go run . inject --dry-run --no-sync 2>&1 | grep -c "prefer"
```

Expected: `1` — only the preamble line, not a duplicate from cap context.
