# Constraints Injection Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `constraints.md` file per pack that injects a numbered "## Constraints" section into AI tool context, telling agents what NOT to do with SAP technologies.

**Architecture:** New `ConstraintsMD` string field on the `Pack` struct, loaded from `constraints.md` with locale fallback, merged via string concatenation in `MergeWith()`, and rendered as a single consolidated section in `RenderContext()` between preambles and context blocks. Budget trimming includes constraints size.

**Tech Stack:** Go, testify, existing content layer system

**Spec:** `docs/superpowers/specs/2026-04-19-constraints-injection-design.md`

---

### Task 1: Add `ConstraintsMD` field and loading

**Files:**
- Modify: `internal/content/pack.go:32` (add field after `PreambleMD`)
- Modify: `internal/content/pack.go:365-367` (add loading after `preamble.md`)
- Test: `internal/content/pack_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `internal/content/pack_test.go`:

```go
func TestLoadPack_ConstraintsMDLoadedWhenPresent(t *testing.T) {
	dir := t.TempDir()
	yaml := "id: cap\nname: CAP\ndescription: CAP\ntags: []\nprofiles: []\nweight: 100\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(yaml), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "constraints.md"), []byte("1. Never write raw SQL"), 0644))

	p, err := content.LoadPack(dir, "")
	require.NoError(t, err)
	assert.Equal(t, "1. Never write raw SQL", p.ConstraintsMD)
}

func TestLoadPack_ConstraintsMDEmptyWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	yaml := "id: cap\nname: CAP\ndescription: CAP\ntags: []\nprofiles: []\nweight: 100\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(yaml), 0644))

	p, err := content.LoadPack(dir, "")
	require.NoError(t, err)
	assert.Empty(t, p.ConstraintsMD)
}

func TestLoadPack_ConstraintsMDLocaleVariant(t *testing.T) {
	dir := t.TempDir()
	yaml := "id: cap\nname: CAP\ndescription: CAP\ntags: []\nprofiles: []\nweight: 100\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(yaml), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "constraints.md"), []byte("English constraints"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "constraints.de.md"), []byte("German constraints"), 0644))

	// German locale: locale file used
	p, err := content.LoadPack(dir, "de")
	require.NoError(t, err)
	assert.Equal(t, "German constraints", p.ConstraintsMD)

	// No locale: base file used
	p, err = content.LoadPack(dir, "")
	require.NoError(t, err)
	assert.Equal(t, "English constraints", p.ConstraintsMD)

	// "en": base file used
	p, err = content.LoadPack(dir, "en")
	require.NoError(t, err)
	assert.Equal(t, "English constraints", p.ConstraintsMD)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go build ./internal/content/... && go vet ./internal/content/...`
Expected: Build fails — `ConstraintsMD` field does not exist on Pack.

- [ ] **Step 3: Add `ConstraintsMD` field to Pack struct**

In `internal/content/pack.go`, after line 32 (`PreambleMD string`), add:

```go
	ConstraintsMD string
```

So lines 32-34 become:

```go
	PreambleMD    string
	ConstraintsMD string
	Hooks         []HookDef
	Influencers   []Influencer
```

(The remaining fields after `Influencers` are unchanged.)

- [ ] **Step 4: Add `constraints.md` loading to `LoadPack()`**

In `internal/content/pack.go`, after the `preamble.md` loading block (after line 367), add:

```go
	// Constraints file: locale variant → base (no expanded step)
	constraintsFile := filepath.Join(packDir, "constraints.md")
	if lang != "" && lang != "en" {
		if loc := filepath.Join(packDir, "constraints."+lang+".md"); fileExists(loc) {
			constraintsFile = loc
		}
	}
	if data, err := os.ReadFile(constraintsFile); err == nil {
		pack.ConstraintsMD = string(data)
	}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go build ./internal/content/... && go vet ./internal/content/...`
Expected: Build and vet succeed.

- [ ] **Step 6: Commit**

```bash
git add internal/content/pack.go internal/content/pack_test.go
git commit -m "feat: add ConstraintsMD field and loading from constraints.md"
```

---

### Task 2: Add additive merge for `ConstraintsMD`

**Files:**
- Modify: `internal/content/merge.go:27-34` (add after ContextMD merge block)
- Test: `internal/content/merge_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `internal/content/merge_test.go`:

```go
func TestMergeWith_ConstraintsMD_After(t *testing.T) {
	base := makePack("cap", "CAP", "", nil, nil)
	base.ConstraintsMD = "1. Base constraint"
	additive := &content.Pack{
		ID: "cap", ConstraintsMD: "2. Additive constraint",
		Additive: true, AdditivePosition: "after",
	}
	result := additive.MergeWith(base)
	assert.Equal(t, "1. Base constraint\n\n2. Additive constraint", result.ConstraintsMD)
}

func TestMergeWith_ConstraintsMD_Before(t *testing.T) {
	base := makePack("cap", "CAP", "", nil, nil)
	base.ConstraintsMD = "1. Base constraint"
	additive := &content.Pack{
		ID: "cap", ConstraintsMD: "2. Additive constraint",
		Additive: true, AdditivePosition: "before",
	}
	result := additive.MergeWith(base)
	assert.Equal(t, "2. Additive constraint\n\n1. Base constraint", result.ConstraintsMD)
}

func TestMergeWith_ConstraintsMD_EmptyAdditivePreservesBase(t *testing.T) {
	base := makePack("cap", "CAP", "", nil, nil)
	base.ConstraintsMD = "1. Base constraint"
	additive := &content.Pack{
		ID: "cap", ConstraintsMD: "",
		Additive: true, AdditivePosition: "after",
	}
	result := additive.MergeWith(base)
	assert.Equal(t, "1. Base constraint", result.ConstraintsMD)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go build ./internal/content/... && go vet ./internal/content/...`
Expected: Build succeeds (field exists from Task 1), but tests fail — `MergeWith` doesn't merge `ConstraintsMD`.

- [ ] **Step 3: Add ConstraintsMD merge logic**

In `internal/content/merge.go`, after the `ContextMD` merge block (after line 34), add:

```go
	// Constraints: same position-controlled concatenation as ContextMD.
	if a.ConstraintsMD != "" {
		if a.AdditivePosition == "before" {
			merged.ConstraintsMD = a.ConstraintsMD + "\n\n" + base.ConstraintsMD
		} else {
			merged.ConstraintsMD = base.ConstraintsMD + "\n\n" + a.ConstraintsMD
		}
	}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go build ./internal/content/... && go vet ./internal/content/...`
Expected: Build and vet succeed.

- [ ] **Step 5: Commit**

```bash
git add internal/content/merge.go internal/content/merge_test.go
git commit -m "feat: add ConstraintsMD additive merge in MergeWith()"
```

---

### Task 3: Render `## Constraints` section

**Files:**
- Modify: `internal/content/render.go:46-48` (insert constraints block between preamble and context loops)
- Test: `internal/content/render_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `internal/content/render_test.go`:

```go
func TestRenderContext_Constraints_AppearsWhenPresent(t *testing.T) {
	packs := []*content.Pack{
		{ID: "cap", ContextMD: "CAP context.", ConstraintsMD: "1. Never write raw SQL"},
	}
	out := content.RenderContext(packs, nil, nil)
	assert.Contains(t, out, "## Constraints")
	assert.Contains(t, out, "1. Never write raw SQL")
}

func TestRenderContext_Constraints_OmittedWhenEmpty(t *testing.T) {
	packs := []*content.Pack{
		{ID: "cap", ContextMD: "CAP context."},
	}
	out := content.RenderContext(packs, nil, nil)
	assert.NotContains(t, out, "## Constraints")
}

func TestRenderContext_Constraints_AfterPreambleBeforeContext(t *testing.T) {
	packs := []*content.Pack{
		{ID: "base", Base: true, PreambleMD: "> Preamble.", ContextMD: "## Base context.", ConstraintsMD: "1. Base constraint"},
		{ID: "cap", ContextMD: "## CAP context.", ConstraintsMD: "2. CAP constraint"},
	}
	out := content.RenderContext(packs, nil, nil)
	preambleIdx := strings.Index(out, "> Preamble.")
	constraintsIdx := strings.Index(out, "## Constraints")
	baseCtxIdx := strings.Index(out, "## Base context.")
	capCtxIdx := strings.Index(out, "## CAP context.")
	require.NotEqual(t, -1, preambleIdx)
	require.NotEqual(t, -1, constraintsIdx)
	assert.Less(t, preambleIdx, constraintsIdx, "preamble before constraints")
	assert.Less(t, constraintsIdx, baseCtxIdx, "constraints before base context")
	assert.Less(t, constraintsIdx, capCtxIdx, "constraints before cap context")
}

func TestRenderContext_Constraints_MultiplePacksMerged(t *testing.T) {
	packs := []*content.Pack{
		{ID: "cap", ContextMD: "CAP context.", ConstraintsMD: "1. CAP constraint"},
		{ID: "abap", ContextMD: "ABAP context.", ConstraintsMD: "1. ABAP constraint"},
	}
	out := content.RenderContext(packs, nil, nil)
	assert.Contains(t, out, "1. CAP constraint")
	assert.Contains(t, out, "1. ABAP constraint")
	// Only one ## Constraints heading
	assert.Equal(t, 1, strings.Count(out, "## Constraints"))
}

func TestRenderContext_Constraints_SkipsEmptyPacks(t *testing.T) {
	packs := []*content.Pack{
		{ID: "cap", ContextMD: "CAP context.", ConstraintsMD: "1. CAP constraint"},
		{ID: "btp", ContextMD: "BTP context.", ConstraintsMD: ""},
		{ID: "abap", ContextMD: "ABAP context.", ConstraintsMD: "  \n  "},
	}
	out := content.RenderContext(packs, nil, nil)
	assert.Contains(t, out, "## Constraints")
	assert.Contains(t, out, "1. CAP constraint")
	// Empty/whitespace-only packs should not produce extra blank lines
	assert.NotContains(t, out, "\n\n\n\n")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go build ./internal/content/... && go vet ./internal/content/...`
Expected: Build succeeds, but tests will fail because `RenderContext` doesn't render constraints yet.

- [ ] **Step 3: Add constraints rendering to `RenderContext()`**

In `internal/content/render.go`, after the preamble loop (after line 46, before the `for _, p := range packs {` ContextMD loop at line 48), insert:

```go
	// Render consolidated constraints from all packs
	var constraints []string
	for _, p := range packs {
		if trimmed := strings.TrimSpace(p.ConstraintsMD); trimmed != "" {
			constraints = append(constraints, trimmed)
		}
	}
	if len(constraints) > 0 {
		b.WriteString("## Constraints\n\n")
		b.WriteString(strings.Join(constraints, "\n\n"))
		b.WriteString("\n\n")
	}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go build ./internal/content/... && go vet ./internal/content/...`
Expected: Build and vet succeed.

- [ ] **Step 5: Commit**

```bash
git add internal/content/render.go internal/content/render_test.go
git commit -m "feat: render consolidated ## Constraints section in injected context"
```

---

### Task 4: Include `ConstraintsMD` in budget trimming

**Files:**
- Modify: `internal/content/render.go:195` (change size calculation in `trimNonBase`)
- Test: `internal/content/render_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/content/render_test.go`:

```go
func TestTrimPacks_BudgetIncludesConstraintsMD(t *testing.T) {
	// cap: ContextMD=5 bytes + ConstraintsMD=10 bytes = 15 bytes total
	// Budget is 10 bytes — pack should NOT fit
	packs := []*content.Pack{
		{ID: "cap", ContextMD: "hello", ConstraintsMD: "constraint"},
	}
	result := content.TrimPacks(packs, 10)
	assert.Empty(t, result, "pack with ContextMD+ConstraintsMD exceeding budget must be trimmed")
}

func TestTrimPacks_BudgetFitsWithConstraintsMD(t *testing.T) {
	// cap: ContextMD=5 bytes + ConstraintsMD=5 bytes = 10 bytes total
	// Budget is 10 bytes — pack should fit
	packs := []*content.Pack{
		{ID: "cap", ContextMD: "hello", ConstraintsMD: "world"},
	}
	result := content.TrimPacks(packs, 10)
	require.Len(t, result, 1)
	assert.Equal(t, "cap", result[0].ID)
}
```

- [ ] **Step 2: Run tests to verify the first test fails**

Run: `go build ./internal/content/... && go vet ./internal/content/...`
Expected: Build succeeds. The first test will fail because budget only counts `ContextMD` (5 bytes < 10, so pack is included).

- [ ] **Step 3: Update budget calculation**

In `internal/content/render.go`, in `trimNonBase()`, change line 195 from:

```go
		size := len(p.ContextMD)
```

to:

```go
		size := len(p.ContextMD) + len(p.ConstraintsMD)
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go build ./internal/content/... && go vet ./internal/content/...`
Expected: Build and vet succeed.

- [ ] **Step 5: Commit**

```bash
git add internal/content/render.go internal/content/render_test.go
git commit -m "feat: include ConstraintsMD in budget trimming calculation"
```

---

### Task 5: Create seed `constraints.md` files

**Files:**
- Create: `content/packs/base/constraints.md`
- Create: `content/packs/cap/constraints.md`
- Create: `content/packs/abap/constraints.md`
- Create: `content/packs/btp-core/constraints.md`

- [ ] **Step 1: Create `content/packs/base/constraints.md`**

```markdown
1. Never store credentials, API keys, or secrets in source code — always use service bindings, environment variables, or the Destination Service
2. Never rely on AI training data for SAP API signatures or configurations — always verify against official SAP documentation or `sap-devs` commands
```

- [ ] **Step 2: Create `content/packs/cap/constraints.md`**

```markdown
1. Never write raw SQL — always use `cds.ql` or CQL
2. Never use `req.user` without a `@requires` annotation on the service
3. Never depend on `@sap/` packages that are not publicly published on npmjs.com or not listed in the CAP released API documentation
4. Never bypass CAP's built-in authentication — use `@requires` and `@restrict` annotations
```

- [ ] **Step 3: Create `content/packs/abap/constraints.md`**

```markdown
1. Never use internal SAP function modules — only use released (Tier-1) APIs
2. Never modify SAP standard objects — extend via clean-core patterns
3. Never use direct table selects in ABAP Cloud — use CDS-based views
4. Never skip ABAP Test Cockpit (ATC) checks before transport
```

- [ ] **Step 4: Create `content/packs/btp-core/constraints.md`**

```markdown
1. Never hardcode BTP credentials or API keys — use the Destination Service or service bindings
2. Never use user-provided services when a managed service instance is available
3. Never deploy to production without environment-specific subaccount separation (dev/test/prod)
4. Never skip entitlement checks — verify quota allocation before provisioning services
```

- [ ] **Step 5: Verify with dry-run inject**

Run: `SAP_DEVS_DEV=1 go run . inject --dry-run --no-sync`
Expected: Output contains `## Constraints` section with numbered items from base + active profile packs.

- [ ] **Step 6: Commit**

```bash
git add content/packs/base/constraints.md content/packs/cap/constraints.md content/packs/abap/constraints.md content/packs/btp-core/constraints.md
git commit -m "content: seed constraints.md for base, cap, abap, btp-core packs"
```

---

### Task 6: Update documentation

**Files:**
- Modify: `CLAUDE.md` (Content Layer System section — mention constraints.md)
- Modify: `docs/content-authoring.md` (Pack Directory Structure — add constraints.md; add Constraints Authoring section)
- Modify: `docs/developer/developer-guide.md` (Content Layer System — mention constraints.md)
- Modify: `TODO.md` (mark feature as completed)

- [ ] **Step 1: Update `CLAUDE.md`**

In the Content Layer System section, where it describes what files each pack contains, add `constraints.md` to the list. Specifically, in the sentence that starts with "`LoadPacks()` reads all `content/packs/<name>/` directories; each pack contains:", add `constraints.md (AI constraint rules)` to the list.

- [ ] **Step 2: Update `docs/content-authoring.md`**

In the Pack Directory Structure tree (around line 14), add after `preamble.md`:

```
├── constraints.md     # Numbered constraint list — things agents should NOT do
├── constraints.<lang>.md  # Localised constraints
```

After the Base Layer section (after line 91, before the Editor Setup section), add a new `## Constraints` section:

```markdown
## Constraints

A pack may include an optional `constraints.md` file. Its content is a numbered markdown list of things AI agents should NOT do when working with that pack's technology domain.

### Format

```markdown
1. Never write raw SQL — always use `cds.ql` or CQL
2. Never use `req.user` without a `@requires` annotation
```

No YAML, no frontmatter — raw numbered markdown. Each line is one constraint.

### Rendered output

All constraints from all active packs are consolidated into a single `## Constraints` section, placed after the preamble and before the first pack's `context.md` content.

### Localization

Two-step resolution: `constraints.{lang}.md` → `constraints.md`. Unlike `context.md`, there is no `constraints.expanded.md` step.

### Additive layers

`constraints.md` participates in additive merge the same way as `context.md`: company/user/project layer constraints are appended (or prepended, based on `additive_position`) to the official constraints.

### Authoring constraints

- Keep each constraint to one line — they are rendered as a numbered list.
- Start each constraint with "Never" to make the prohibition clear.
- Include the correct alternative after "—" so agents know what to do instead.
- Universal constraints (e.g. credential storage) belong in the base pack's `constraints.md`.
- Technology-specific constraints belong in the domain pack.

### Merge behaviour update

Add to the merge behaviour table in the Additive Layers section:

| `constraints.md` | Your content is appended or prepended to the official constraints |
```

- [ ] **Step 3: Update `docs/developer/developer-guide.md`**

In the Content Layer System section (around line 121), update the description to mention `constraints.md` alongside `context.md` and `preamble.md`.

- [ ] **Step 4: Update `TODO.md`**

Mark the "Behavioral rules / anti-patterns injection" feature as completed.

- [ ] **Step 5: Commit**

```bash
git add CLAUDE.md docs/content-authoring.md docs/developer/developer-guide.md TODO.md
git commit -m "docs: document constraints.md authoring, update architecture references"
```

---

### Task 7: Final verification

- [ ] **Step 1: Build check**

Run: `go build ./... && go vet ./...`
Expected: Clean build, no vet warnings.

- [ ] **Step 2: Full dry-run inject with stats**

Run: `SAP_DEVS_DEV=1 go run . inject --dry-run --no-sync --stats`
Expected: `## Constraints` section visible in output. Stats show token count reflecting constraints content.

- [ ] **Step 3: Verify constraint ordering**

In the dry-run output, confirm:
1. `## Constraints` appears after the preamble (`> For SAP-specific information...`)
2. `## Constraints` appears before `## SAP Developer Ecosystem` (first context block)
3. Constraints from base pack appear first, then CAP/ABAP/BTP constraints
