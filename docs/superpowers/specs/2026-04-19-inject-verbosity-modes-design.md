# Inject Verbosity Modes Design

**Date:** 2026-04-19
**Status:** Approved

## Problem

Claude Code's `CLAUDE.md` can absorb thousands of tokens; a clipboard export for ChatGPT is capped at ~1,400 bytes. Currently the same content blob is injected everywhere, either overwhelming small-context tools or under-serving large-context ones. The existing `MaxBytes`/`MaxTokens` budget truncates by pack — it drops entire packs but cannot thin a pack's content. Truncation by byte count is arbitrary and cuts mid-sentence.

## Solution

Semantic verbosity tagging: HTML comment markers inside `context.md` files assign each section a verbosity tier (`core`, `detail`, `extended`). Adapters declare a verbosity level (`minimal`, `standard`, `full`) that controls which tiers are included. A CLI flag allows per-run override.

## Verbosity Tiers

| Tier | Meaning | Example content |
|---|---|---|
| `core` | Essential identity and instructions — always included | Ecosystem portals, CLI reference, key tools, preambles, constraints |
| `detail` | Best practices, examples, moderate reference | Service definitions, best practices, canonical patterns |
| `extended` | Deep reference material | Release notes, known errors, learning journey tables |

## Adapter Verbosity Levels

| Adapter `verbosity` value | Includes tiers |
|---|---|
| `minimal` | core |
| `standard` | core + detail |
| `full` (default) | core + detail + extended |

Default is `full` — existing adapters with no `verbosity` field behave identically to today.

## Marker Syntax

HTML comments in `context.md` (and `constraints.md`) delineate verbosity boundaries:

```markdown
## SAP CAP (Cloud Application Programming Model)

CAP is SAP's primary framework...    ← core (no marker = default)

### Key Tools                        ← core
- @sap/cds-dk ...

<!-- verbosity:detail -->
### Service Definition               ← detail
### Best Practices                   ← detail

<!-- verbosity:extended -->
### Recent CAP Releases              ← extended
```

**Rules:**
- A marker applies to all content after it until the next marker or end-of-file.
- Content before any marker is `core`.
- Valid levels: `core`, `detail`, `extended`. Unrecognized levels produce a stderr warning and are treated as `core`.
- Markers are stripped from rendered output.
- A `<!-- verbosity:core -->` marker can reset the level back to core mid-file.

## Design

### New Type: `VerbositySections`

File: `internal/content/verbosity.go`

```go
type VerbositySections struct {
    Core     string
    Detail   string
    Extended string
}

func ParseVerbositySections(md string) VerbositySections

func (v VerbositySections) AtLevel(level string) string
```

`ParseVerbositySections` scans for `<!-- verbosity:{level} -->` markers and splits the markdown into three buckets. `AtLevel` concatenates the appropriate tiers for a given verbosity level.

### Pack Struct Changes

In `internal/content/pack.go`:

- `ContextMD string` → `Context VerbositySections`
- `ConstraintsMD string` → `Constraints VerbositySections`
- `PreambleMD` remains `string` (preambles are always core)

`LoadPack` calls `ParseVerbositySections` when loading `context.md` and `constraints.md`.

### Additive Merge

In `internal/content/merge.go`, `MergeWith` for additive packs merges per-tier: the additive pack's `Core` appends to (or prepends before) the base pack's `Core`, and likewise for `Detail` and `Extended`.

### Adapter Changes

In `internal/adapter/adapter.go`, the `Adapter` struct gains:

```go
Verbosity string `yaml:"verbosity"` // "minimal" | "standard" | "full"; default "full"
```

In `internal/adapter/engine.go`, `Options` gains:

```go
Verbosity string // CLI override; empty = use adapter default
```

Per-adapter resolution in `engine.Run()`: CLI flag → adapter YAML → `"full"`.

### Render Pipeline

`RenderContext` signature becomes:

```go
func RenderContext(packs []*Pack, profile *Profile, dynamic *DynamicContext, verbosity string) string
```

Pack content assembly uses `pack.Context.AtLevel(verbosity)` and `pack.Constraints.AtLevel(verbosity)`.

**Synthetic sections — hardcoded tiers:**

| Section | Tier |
|---|---|
| Header + Profile line | core |
| What's New | core |
| Current Context (scratch notes) | core |
| Runtime Context (dynamic) | core |
| Preambles | core |
| Constraints | per-marker (within constraints.md) |
| Canonical Patterns table | detail |
| Recommended Learning table | extended |
| Known Errors table | extended |

### TrimPacks

`TrimPacks` gains a `verbosity` parameter. Budget calculation sums only the bytes that will be rendered at the given verbosity level (using `AtLevel`), not the full pack content.

```go
func TrimPacks(packs []*Pack, maxBytes int, verbosity string) []*Pack
```

### CLI Flag

`cmd/inject.go` gains `--verbosity` flag (string, default empty). Valid values: `minimal`, `standard`, `full`. Passed into `adapter.Options.Verbosity`.

### Stats Output

The `--stats` table adds a `Verbosity` column showing the effective verbosity for each adapter.

### Status / Staleness

`inject --status` renders the comparison content using each adapter's verbosity level for accurate staleness detection.

## Initial Content Tagging

### `content/packs/cap/context.md`

```markdown
## SAP CAP (Cloud Application Programming Model)
...intro...                          ← core

### Key Tools                        ← core
### CDS Data Modelling               ← core

<!-- verbosity:detail -->
### Service Definition               ← detail
### Best Practices                   ← detail

<!-- verbosity:extended -->
### Recent CAP Releases              ← extended
```

### `content/packs/base/context.md`

```markdown
## SAP Developer Ecosystem           ← core
### Key Portals                      ← core

<!-- verbosity:detail -->
### Learning & Discovery             ← detail
### Developer News & Community       ← detail
### APIs & SDKs                      ← detail
### Support & Contribution           ← detail

<!-- verbosity:core -->
## sap-devs CLI Reference            ← core (reset)
```

### Adapter YAML

`content/adapters/clipboard.yaml` gets `verbosity: minimal` as proof-of-concept. All other adapters default to `full` (no change needed).

## File Changes

**New files:**
- `internal/content/verbosity.go` — type, parser, `AtLevel`
- `internal/content/verbosity_test.go` — unit tests

**Modified files:**

| File | Change |
|---|---|
| `internal/content/pack.go` | `ContextMD` → `Context VerbositySections`; `ConstraintsMD` → `Constraints VerbositySections`; parse in `LoadPack` |
| `internal/content/render.go` | `RenderContext` gains `verbosity` param; uses `AtLevel`; gates synthetic sections |
| `internal/content/render.go` | `TrimPacks` gains `verbosity` param |
| `internal/content/merge.go` | Per-tier merge for additive packs |
| `internal/adapter/adapter.go` | `Adapter.Verbosity` field |
| `internal/adapter/engine.go` | `Options.Verbosity`; per-adapter resolution logic |
| `internal/adapter/status.go` | Verbosity-aware staleness rendering |
| `cmd/inject.go` | `--verbosity` flag; pass to `Options` |
| `content/adapters/clipboard.yaml` | Add `verbosity: minimal` |
| `content/packs/base/context.md` | Add verbosity markers |
| `content/packs/cap/context.md` | Add verbosity markers |

## Migration

- **Existing context.md with no markers:** All content is core. Behavior identical to today at every verbosity level. Zero breakage.
- **Existing adapters with no `verbosity` field:** Default `full`. Zero behavior change.
- **Authors opt in** by adding `<!-- verbosity:detail -->` / `<!-- verbosity:extended -->` markers to their content files.

## Testing

- **Unit tests** (`verbosity_test.go`): marker parsing, `AtLevel` at each tier, edge cases (no markers, adjacent markers, unknown markers, core-reset mid-file, empty sections)
- **Render tests:** Updated to pass `verbosity` param; `"full"` preserves existing behavior
- **Integration:** `SAP_DEVS_DEV=1 go run . inject --dry-run --verbosity minimal` to visually confirm reduced output vs `--verbosity full`
