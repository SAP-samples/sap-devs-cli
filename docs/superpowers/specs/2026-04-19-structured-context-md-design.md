# Structured `context.md` Conventions

**Date:** 2026-04-19
**Status:** Approved

## Problem

Pack `context.md` files are free-form Markdown. This makes verbosity tagging harder to retrofit, prevents agents from referencing sections by name, and gives no guidance to content authors about what to include.

## Design

### Standard Sections

Each `context.md` keeps its pack-identity H2 (e.g., `## SAP CAP`), with these optional H3 sections in a defined order:

| # | Section | Purpose | Default Verbosity |
|---|---------|---------|-------------------|
| 1 | `### Overview` | What this technology is and when to use it | core |
| 2 | `### Key Concepts` | The 3-5 concepts an agent must understand | core |
| 3 | `### Best Practices` | What to do | core |
| 4 | `### Anti-patterns` | What not to do | detail |
| 5 | `### Code Examples` | Short, canonical inline snippets | detail |

Not all sections are required — packs include what fits. Sections not in this list are allowed but generate a warning at load time. `Known Errors` and `Resources` are omitted because they are already covered by `known_errors.yaml` and `resources.yaml`.

### Heading Hierarchy

```
## <Pack Name>           ← pack identity, typically one per context.md
### Overview             ← standard section (optional)
### Key Concepts         ← standard section (optional)
#### <tool or concept>   ← subsection within Key Concepts
### Best Practices       ← standard section (optional)
### Anti-patterns        ← standard section (optional)
### Code Examples        ← standard section (optional)
### <Custom Section>     ← allowed, generates lint warning
```

**Exception:** The `base` pack has two H2 headings (`## SAP Developer Ecosystem` and `## sap-devs CLI Reference`) because it serves a dual role (ecosystem overview + CLI reference). The CLI Reference section is not subject to the standard H3 convention. Other packs should use a single H2.

### Validation: `ValidateContextSections()`

A new function in `internal/content/sections.go`:

```go
func ValidateContextSections(packID string, content string)
```

**Behavior:**

- Parses all H3 headings (`### ...`) from the raw Markdown content (before verbosity parsing)
- Compares against the recognized set: `Overview`, `Key Concepts`, `Best Practices`, `Anti-patterns`, `Code Examples`
- Warns to stderr for unrecognized H3 headings: `sap-devs: pack %q: unrecognized section %q`
- Warns if recognized sections appear out of the canonical order
- Does NOT fail — warnings only, so custom sections remain possible
- Called from `LoadPack()` on the raw file bytes, **before** `ParseVerbositySections()`
- **Skipped for `context.expanded.md`** — expanded files contain machine-fetched remote content with arbitrary headings that would produce spurious warnings. Only authored `context.md` (or `context.<lang>.md`) files are validated.

**Recognized sections constant:**

```go
var RecognizedContextSections = []string{
    "Overview",
    "Key Concepts",
    "Best Practices",
    "Anti-patterns",
    "Code Examples",
}
```

### Verbosity Integration

The existing `<!-- verbosity:X -->` marker system is unchanged. The standard sections establish default verbosity assignments by convention:

- Overview, Key Concepts, Best Practices → authors should tag as `core`
- Anti-patterns, Code Examples → authors should tag as `detail`
- Pack-specific extended content (e.g., release notes) → `extended`

Authors place markers explicitly. The convention documents the expected defaults but does not enforce them programmatically.

### Migration Plan

**Atomicity:** The validator and all content migrations must land in the same commit to avoid spurious warnings during development.

Retrofit the 4 existing context.md files:

**`cap/context.md`:**
- Current single H2 `## SAP CAP (Cloud Application Programming Model)` stays
- Add H3 sections: Overview (intro paragraph), Key Concepts (tools list), Best Practices (existing bullets), Code Examples (existing CDS snippets)
- Move `### Recent CAP Releases` content to extended verbosity (already tagged)

**`btp-core/context.md`:**
- Current H2 `## SAP Business Technology Platform (BTP)` stays
- Add H3 sections: Overview (intro), Key Concepts (Global Account hierarchy), Best Practices (existing bullets), Code Examples (CF quick reference)

**`abap/context.md`:**
- Current H2 `## ABAP Cloud` stays
- Add H3 sections: Overview (intro), Key Concepts (ADT, RAP, Tier-1 APIs), Best Practices (existing bullets)

**`base/context.md`:**
- Special case: contains ecosystem info + CLI reference
- Keep `## SAP Developer Ecosystem` and `## sap-devs CLI Reference` as-is
- Under `## SAP Developer Ecosystem`, restructure into: Overview (portals), Key Concepts (learning/discovery/APIs sections)
- CLI Reference is a standalone section, not subject to standard heading convention

### Files Changed

| File | Change |
|------|--------|
| `internal/content/sections.go` | New: `ValidateContextSections()`, recognized sections list |
| `internal/content/sections_test.go` | New: unit tests for validation |
| `internal/content/pack.go` | Call `ValidateContextSections()` in `LoadPack()` |
| `content/packs/cap/context.md` | Retrofit with standard H3 sections |
| `content/packs/btp-core/context.md` | Retrofit with standard H3 sections |
| `content/packs/abap/context.md` | Retrofit with standard H3 sections |
| `content/packs/base/context.md` | Partial retrofit (ecosystem section) |

### Testing

- Unit tests for `ValidateContextSections()`: valid pack, unrecognized section, out-of-order sections, no H3 sections, mixed recognized/unrecognized
- `go build ./...` and `go vet ./...` for compile verification
- Manual: `SAP_DEVS_DEV=1 go run . inject --dry-run` to verify migrated content renders correctly

### What This Does NOT Do

- Does not change the verbosity parsing system
- Does not add a JSON Schema for Markdown files
- Does not make unrecognized sections an error
- Does not change how `constraints.md` is structured (future work)

### Locale Variant Files

Locale variant files (`context.<lang>.md`) are subject to the same section conventions. They are validated by `ValidateContextSections()` when selected as the active context file. Content authors writing locale variants should follow the same H3 section headings and ordering as the base `context.md`.
