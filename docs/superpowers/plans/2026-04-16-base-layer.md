# Base Layer Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `base: true` field to `pack.yaml` so a pack is auto-injected into every AI tool context regardless of the active developer profile, always rendered first, and exempt from byte-budget trimming.

**Architecture:** Add `Base bool` to the `Pack` struct and `packMeta` YAML schema. In `LoadPacks()`, partition base packs from non-base packs after sorting and pin base packs at the front of the returned slice. In `TrimPacks()`, extract base packs before both trimming passes and append them unconditionally to the result. Create `content/packs/base/` with shared SAP ecosystem content.

**Tech Stack:** Go 1.21, `gopkg.in/yaml.v3`, `github.com/stretchr/testify`

**Build note (Windows):** `go test` is blocked by Windows Defender. Use `go build ./...` + `go vet ./...` for local verification. CI (ubuntu-latest GitHub Actions) is the authoritative test runner.

---

## File Map

| File | Action | Responsibility |
| --- | --- | --- |
| `internal/content/pack.go` | Modify | Add `Base bool` to `Pack` and `packMeta`; assign in `LoadPack()` |
| `internal/content/loader.go` | Modify | Partition base/non-base and pin base packs first after sorting |
| `internal/content/render.go` | Modify | Extract base packs before TrimPacks passes; add `trimNonBase()` helper |
| `internal/adapter/engine.go` | Modify | Add comment documenting behaviour of budget guard with base packs |
| `internal/content/loader_test.go` | Modify | Add tests: base packs appear first regardless of weight; nil profile |
| `internal/content/render_test.go` | Modify | Add tests: base packs survive budget trimming; survive dedup pass |
| `content/packs/base/pack.yaml` | Create | New base pack metadata with `base: true` |
| `content/packs/base/context.md` | Create | Shared SAP ecosystem entry points |
| `docs/content/content-guide.md` | Modify | Document `base` field in pack.yaml schema; update "Creating a New Pack" |
| `docs/content-authoring.md` | Modify | Add "Base Layer" section |

---

## Task 1: Add `Base` field to Pack struct

**Files:**

- Modify: `internal/content/pack.go`
- Modify: `internal/content/loader_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/content/loader_test.go` after line 54:

```go
func TestLoadPack_BaseField_TrueWhenSet(t *testing.T) {
	dir := makeTempPacksDir(t, map[string]string{
		"base": "id: base\nname: Base\nweight: 0\nbase: true\n",
	})
	pack, err := content.LoadPack(filepath.Join(dir, "packs", "base"), "")
	require.NoError(t, err)
	assert.True(t, pack.Base, "pack.Base should be true when base: true in pack.yaml")
}

func TestLoadPack_BaseField_FalseByDefault(t *testing.T) {
	dir := makeTempPacksDir(t, map[string]string{
		"cap": "id: cap\nname: CAP\nweight: 100\n",
	})
	pack, err := content.LoadPack(filepath.Join(dir, "packs", "cap"), "")
	require.NoError(t, err)
	assert.False(t, pack.Base, "pack.Base should be false when base field is absent")
}
```

- [ ] **Step 2: Verify build fails (field missing)**

```bash
go build ./...
```

Expected: compile error — `pack.Base undefined`

- [ ] **Step 3: Add `Base bool` to `Pack` struct**

In `internal/content/pack.go`, add `Base bool` between `Overlaps` and `ContextMD` in the `Pack` struct (line 19, after `Overlaps []string`):

```go
	Overlaps    []string
	Base        bool
	ContextMD   string
```

The doc comment on line 11 (`// Pack is a named bundle...`) must be preserved — do not replace the whole struct block, just insert the field.

- [ ] **Step 4: Add `Base` to `packMeta` struct**

In `internal/content/pack.go`, edit the `packMeta` struct (lines 82–91). Add `Base` after `Overlaps`:

```go
type packMeta struct {
	ID          string                    `yaml:"id"`
	Name        string                    `yaml:"name"`
	Description string                    `yaml:"description"`
	Tags        []string                  `yaml:"tags"`
	Profiles    []string                  `yaml:"profiles"`
	Weight      int                       `yaml:"weight"`
	Overlaps    []string                  `yaml:"overlaps,omitempty"`
	Base        bool                      `yaml:"base,omitempty"`
	Locales     map[string]packMetaLocale `yaml:"locales,omitempty"`
}
```

- [ ] **Step 5: Assign `meta.Base` in `LoadPack()`**

In `internal/content/pack.go`, edit the `pack := &Pack{...}` literal (lines 106–114). Add `Base: meta.Base,`:

```go
pack := &Pack{
	ID:          meta.ID,
	Name:        meta.Name,
	Description: meta.Description,
	Tags:        meta.Tags,
	Profiles:    meta.Profiles,
	Weight:      meta.Weight,
	Overlaps:    meta.Overlaps,
	Base:        meta.Base,
}
```

- [ ] **Step 6: Verify build and vet pass**

```bash
go build ./...
go vet ./...
```

Expected: no errors

- [ ] **Step 7: Commit**

```bash
git add internal/content/pack.go internal/content/loader_test.go
git commit -m "feat(content): add Base field to Pack struct and pack.yaml schema"
```

---

## Task 2: TrimPacks — exempt base packs from trimming

**Files:**

- Modify: `internal/content/render.go`
- Modify: `internal/content/render_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `internal/content/render_test.go` after the last test (line 330):

```go
func TestTrimPacks_BasePackSurvivesBudget(t *testing.T) {
	// Base pack content is 20 bytes; budget is 5 — base pack must survive anyway
	packs := []*content.Pack{
		{ID: "base", Base: true, ContextMD: "12345678901234567890"},
		{ID: "cap", ContextMD: "CAP content"},
	}
	result := content.TrimPacks(packs, 5)
	require.Len(t, result, 1)
	assert.Equal(t, "base", result[0].ID, "base pack must survive even when its content exceeds the budget")
}

func TestTrimPacks_BasePackSurvivesDeduplication(t *testing.T) {
	// Non-base pack declares overlaps: [base] — base pack must NOT be dropped
	packs := []*content.Pack{
		{ID: "base", Base: true, ContextMD: "base content"},
		{ID: "cap", ContextMD: "CAP content", Overlaps: []string{"base"}},
	}
	result := content.TrimPacks(packs, 0)
	// base pack survives; cap is not dropped either (its overlap target was separated out)
	require.Len(t, result, 2)
	assert.Equal(t, "base", result[0].ID)
	assert.Equal(t, "cap", result[1].ID)
}

func TestTrimPacks_BasePackFirst_NonBasePacksAfter(t *testing.T) {
	packs := []*content.Pack{
		{ID: "cap", ContextMD: "CAP content"},
		{ID: "base", Base: true, ContextMD: "base content"},
		{ID: "abap", ContextMD: "ABAP content"},
	}
	result := content.TrimPacks(packs, 0)
	require.Len(t, result, 3)
	assert.Equal(t, "base", result[0].ID, "base pack must be first in output")
}

func TestTrimPacks_BreakOnOversizePreservedForNonBase(t *testing.T) {
	// base pack always included (even though its 16 bytes exceeds the 10-byte budget);
	// first non-base pack is too large → break; second non-base pack (small) is never reached.
	// This verifies: (a) base pack is budget-exempt, (b) break-on-first-oversize preserved for non-base.
	packs := []*content.Pack{
		{ID: "base", Base: true, ContextMD: "base content here"}, // 17 bytes > budget
		{ID: "big", ContextMD: "this is way too large for budget"},
		{ID: "small", ContextMD: "hi"},
	}
	result := content.TrimPacks(packs, 10)
	require.Len(t, result, 1, "only base pack survives; big breaks the loop; small never reached")
	assert.Equal(t, "base", result[0].ID)
}

func TestTrimPacks_AllBasePacks_AllSurvive(t *testing.T) {
	packs := []*content.Pack{
		{ID: "base1", Base: true, ContextMD: "base one content"},
		{ID: "base2", Base: true, ContextMD: "base two content"},
	}
	result := content.TrimPacks(packs, 5) // tiny budget — ignored for base packs
	require.Len(t, result, 2)
}
```

- [ ] **Step 2: Verify build passes, tests would fail**

```bash
go build ./...
go vet ./...
```

Expected: builds fine. The new tests would fail (base packs currently get trimmed) — verified by CI.

- [ ] **Step 3: Refactor `TrimPacks` to extract base packs and add `trimNonBase` helper**

Replace the entire `TrimPacks` function in `internal/content/render.go` (lines 106–143):

```go
// TrimPacks filters packs to fit within maxBytes, applying overlap deduplication
// and pack-level budget enforcement. Pass maxBytes=0 for unconstrained.
// Packs must already be sorted by weight descending (LoadPacks guarantees this).
// Base packs (Pack.Base == true) are exempt from both trimming passes and always
// appear first in the returned slice.
func TrimPacks(packs []*Pack, maxBytes int) []*Pack {
	// Separate base packs — always included, never trimmed, always first.
	var base, nonBase []*Pack
	for _, p := range packs {
		if p.Base {
			base = append(base, p)
		} else {
			nonBase = append(nonBase, p)
		}
	}
	return append(base, trimNonBase(nonBase, maxBytes)...)
}

// trimNonBase applies deduplication and byte-budget enforcement to non-base packs.
func trimNonBase(packs []*Pack, maxBytes int) []*Pack {
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

- [ ] **Step 4: Verify build and vet pass**

```bash
go build ./...
go vet ./...
```

Expected: no errors

- [ ] **Step 5: Commit**

```bash
git add internal/content/render.go internal/content/render_test.go
git commit -m "feat(content): exempt base packs from TrimPacks budget and deduplication"
```

---

## Task 3: LoadPacks — pin base packs first

**Files:**

- Modify: `internal/content/loader.go`
- Modify: `internal/content/loader_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `internal/content/loader_test.go` after line 54:

```go
func TestContentLoader_LoadPacks_BasePackFirst_RegardlessOfWeight(t *testing.T) {
	// base pack has weight 0 (lowest), but must always appear first
	dir := makeTempPacksDir(t, map[string]string{
		"base": "id: base\nname: Base\nweight: 0\nbase: true\n",
		"cap":  "id: cap\nname: CAP\nweight: 100\n",
		"abap": "id: abap\nname: ABAP\nweight: 90\n",
	})
	loader := &content.ContentLoader{OfficialDir: dir}
	packs, err := loader.LoadPacks(nil, "")
	require.NoError(t, err)
	require.Len(t, packs, 3)
	assert.Equal(t, "base", packs[0].ID, "base pack must be first regardless of weight")
}

func TestContentLoader_LoadPacks_MultipleBasePacks_AllFirst(t *testing.T) {
	dir := makeTempPacksDir(t, map[string]string{
		"base1": "id: base1\nname: Base 1\nweight: 50\nbase: true\n",
		"base2": "id: base2\nname: Base 2\nweight: 10\nbase: true\n",
		"cap":   "id: cap\nname: CAP\nweight: 100\n",
	})
	loader := &content.ContentLoader{OfficialDir: dir}
	packs, err := loader.LoadPacks(nil, "")
	require.NoError(t, err)
	require.Len(t, packs, 3)
	assert.True(t, packs[0].Base, "first pack must be base")
	assert.True(t, packs[1].Base, "second pack must be base")
	assert.False(t, packs[2].Base, "third pack must be non-base")
	// base packs are ordered by weight among themselves
	assert.Equal(t, "base1", packs[0].ID, "higher-weight base pack first")
}

func TestContentLoader_LoadPacks_NoBasePacks_OrderUnchanged(t *testing.T) {
	dir := makeTempPacksDir(t, map[string]string{
		"cap":  "id: cap\nname: CAP\nweight: 100\n",
		"abap": "id: abap\nname: ABAP\nweight: 90\n",
	})
	loader := &content.ContentLoader{OfficialDir: dir}
	packs, err := loader.LoadPacks(nil, "")
	require.NoError(t, err)
	require.Len(t, packs, 2)
	// weight ordering unchanged when no base packs
	assert.Equal(t, "cap", packs[0].ID)
	assert.Equal(t, "abap", packs[1].ID)
}
```

- [ ] **Step 2: Verify build passes, tests would fail**

```bash
go build ./...
go vet ./...
```

Expected: builds fine. The new ordering tests would fail in CI — base pack currently sorts last (weight 0).

- [ ] **Step 3: Add partition step at end of `LoadPacks()`**

In `internal/content/loader.go`, replace line 50:

```go
	return ApplyWeights(packs, profile), nil
```

With:

```go
	weighted := ApplyWeights(packs, profile)

	// Pin base packs first. Base packs are exempt from profile weight ordering —
	// they always appear before non-base packs regardless of their weight value.
	// Among multiple base packs, relative order is preserved from the weight sort above.
	var base, nonBase []*Pack
	for _, p := range weighted {
		if p.Base {
			base = append(base, p)
		} else {
			nonBase = append(nonBase, p)
		}
	}
	return append(base, nonBase...), nil
```

- [ ] **Step 4: Verify build and vet pass**

```bash
go build ./...
go vet ./...
```

Expected: no errors

- [ ] **Step 5: Commit**

```bash
git add internal/content/loader.go internal/content/loader_test.go
git commit -m "feat(content): pin base packs first in LoadPacks output"
```

---

## Task 4: engine.go — document budget guard behaviour

**Files:**

- Modify: `internal/adapter/engine.go`

No new tests — this is a documentation-only change to a stats/warning code path.

- [ ] **Step 1: Add comment to the budget-too-small guard and note the Trimmed flag**

In `internal/adapter/engine.go`, replace lines 62–75 (the full block from `trimmed :=` through the closing `}` of the early-continue block):

```go
		trimmed := content.TrimPacks(e.packs, maxBytes)
		// Note: when base packs exist, TrimPacks always returns at least those packs,
		// so len(trimmed) == 0 only occurs when no base packs are configured and the
		// budget is too small for all non-base packs.
		if len(trimmed) == 0 && maxBytes > 0 {
			fmt.Fprintf(os.Stderr, "sap-devs: adapter %s: budget too small to include any pack content\n", a.ID)
			if e.opts.Stats {
				stats = append(stats, adapterStats{
					AdapterID:   a.ID,
					PackIDs:     nil,
					BudgetBytes: maxBytes, // resolved value
					Format:      a.Format,
					Trimmed:     true,
				})
			}
			continue
		}
```

Also add a comment to the `Trimmed` flag on line 122 (inside the stats block at the bottom of the loop):

```go
			// Trimmed is true when any pack was dropped. With base packs always
			// surviving TrimPacks, this correctly reflects whether non-base packs
			// were dropped by budget or deduplication.
			Trimmed:      len(trimmed) < len(e.packs),
```

- [ ] **Step 2: Verify build and vet pass**

```bash
go build ./...
go vet ./...
```

Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add internal/adapter/engine.go
git commit -m "docs(adapter): clarify budget guard behaviour with base packs"
```

---

## Task 5: Create the base pack content

**Files:**

- Create: `content/packs/base/pack.yaml`
- Create: `content/packs/base/context.md`

- [ ] **Step 1: Create `content/packs/base/pack.yaml`**

```yaml
id: base
name: SAP Developer Base
description: Shared SAP developer resources — ecosystem entry points injected into every profile
tags: [sap, btp, developers, community]
weight: 0
base: true
```

- [ ] **Step 2: Create `content/packs/base/context.md`**

```markdown
## SAP Developer Ecosystem

### Key Portals

- **SAP Developer Portal** — https://developers.sap.com — tutorials, missions, blog posts, events
- **SAP Help Portal** — https://help.sap.com — official product documentation
- **SAP Community** — https://community.sap.com — Q&A, blogs, groups
- **SAP BTP Cockpit** — https://cockpit.btp.cloud.sap — manage your BTP global account and subaccounts

### Learning & Discovery

- **SAP Learning** — https://learning.sap.com — free and paid learning journeys
- **SAP Discovery Center** — https://discovery-center.cloud.sap — BTP service catalog, missions, and pricing

### Developer News & Community

- **SAP Developers YouTube** — https://youtube.com/@sapdevs — tutorials, demos, and live streams
- **SAP Developer News** — weekly show on the SAP Developers YouTube channel; new episodes every Friday
- **SAP Tech Bytes** — short-form code-focused videos on the SAP Developers YouTube channel

### APIs & SDKs

- **SAP Business Accelerator Hub** — https://api.sap.com — browse and test SAP APIs
- **SAP NPM registry** — https://registry.npmjs.org — `@sap/*` packages for Node.js development
- **SAP Maven Central** — `com.sap.cloud.*` artifacts for Java/Spring development

### Support & Contribution

- Ask questions on SAP Community (tag the relevant product/topic)
- File bugs via the SAP support portal or product-specific GitHub repositories
- Contribute samples and tutorials via https://github.com/SAP-samples
```

- [ ] **Step 3: Verify the pack loads cleanly**

```bash
go build ./...
go vet ./...
```

Expected: no errors

- [ ] **Step 4: Commit**

```bash
git add content/packs/base/
git commit -m "feat(content): add base pack with shared SAP ecosystem entry points"
```

---

## Task 6: Documentation updates

**Files:**

- Modify: `docs/content/content-guide.md`
- Modify: `docs/content-authoring.md`

- [ ] **Step 1: Add `base` field to pack.yaml schema in `docs/content/content-guide.md`**

Find the `pack.yaml` schema section (around line 40–58 in that file). After the `overlaps` field entry, add:

```markdown
- **`base`** *(optional bool, default `false`)* — when `true`, this pack is a **base pack**: it is auto-injected into every profile regardless of the `profiles` field, always rendered first (before profile-specific packs), and exempt from adapter byte-budget trimming and overlap deduplication. The `profiles` field is irrelevant for base packs and should be omitted. **Authoring contract: keep base pack content minimal** — base packs consume tokens in every context window.

  Note: declaring `overlaps: [base]` on a non-base pack has no effect (the base pack is separated before the deduplication pass runs). This is a known limitation.
```

- [ ] **Step 2: Update "Creating a New Pack" guide in `docs/content/content-guide.md`**

Find the "Creating a New Pack" section. After the sample `pack.yaml` snippet, add a note:

```markdown
> **Base packs:** If your pack should be auto-injected into every profile (e.g. shared ecosystem links), add `base: true` and omit the `profiles` field. Keep base pack content short — it is included in every context window regardless of budget constraints.
```

- [ ] **Step 3: Add "Base Layer" section to `docs/content-authoring.md`**

Add a new section after the existing "Pack Directory Structure" section:

```markdown
## Base Layer

A **base pack** is injected into every AI tool context regardless of the active developer profile. It is always rendered first, before profile-specific packs, and is exempt from adapter byte-budget trimming.

**When to use base packs:**

- Shared ecosystem entry points every SAP developer needs (portals, community links, YouTube, BTP cockpit)
- Content that should always be present in the AI context window regardless of the user's technology focus

**When NOT to use base packs:**

- Technology-specific content (CAP, ABAP, Fiori, etc.) — use a regular pack with the appropriate `profiles` entry
- Large reference material — base packs are exempt from token budget trimming, so large base packs inflate every context window

**How to create a base pack:**

Add `base: true` to `pack.yaml`. Omit the `profiles` field — it is not consulted for base packs.

```yaml
id: my-base
name: My Base Pack
description: Shared content for all profiles
weight: 0
base: true
```

**Authoring contract:** Keep base pack content minimal. Every byte in a base pack is consumed in every inject, for every user, regardless of their configured token budget.
```

- [ ] **Step 4: Verify build and vet pass**

```bash
go build ./...
go vet ./...
```

Expected: no errors

- [ ] **Step 5: Commit**

```bash
git add docs/content/content-guide.md docs/content-authoring.md
git commit -m "docs(content): document base pack field and base layer authoring guidance"
```

---

## Verification

After all tasks are complete:

- [ ] **Final build and vet**

```bash
go build ./...
go vet ./...
```

Expected: no errors

- [ ] **Smoke test with dev mode**

```bash
SAP_DEVS_DEV=1 go run . inject --dry-run
```

Expected: output includes base pack content before any profile-specific pack content. No errors.
