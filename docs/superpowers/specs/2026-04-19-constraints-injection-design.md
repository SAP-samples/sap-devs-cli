# Constraints Injection Design

**Date:** 2026-04-19
**Status:** Draft
**Feature:** Behavioral rules / anti-patterns injection via `constraints.md` per pack

## Problem

`context.md` tells agents what to do but never tells them what NOT to do. Agents frequently suggest valid-but-wrong approaches (raw SQL in CAP, internal ABAP function modules, hardcoded BTP credentials) because the injected content doesn't prohibit them.

## Solution

Add an optional `constraints.md` file to each pack. Its content is loaded into a new `ConstraintsMD` string field on the `Pack` struct, merged via concatenation (same as `ContextMD`), and rendered as a single consolidated `## Constraints` section in the injected output.

## Pack File Format

Each pack may include `constraints.md` containing a numbered markdown list:

```markdown
1. Never write raw SQL — always use `cds.ql` or CQL
2. Never use `req.user` without a `@requires` annotation on the service
3. Never import internal `@sap/` packages that aren't in the released API list
4. Never store credentials in code — always use service bindings or environment variables
```

No YAML, no frontmatter — raw markdown. Each line is a constraint.

**Localization:** Two-step resolution: `constraints.{lang}.md` (if lang is set and not `"en"`) → `constraints.md`. Unlike `context.md`, there is no `constraints.expanded.md` step — constraints are authored once and are not dynamically expanded via sync markers.

## Data Model

New field on `Pack` struct in `internal/content/pack.go`:

```go
type Pack struct {
    // ... existing fields ...
    PreambleMD    string
    ConstraintsMD string  // NEW — loaded from constraints.md
    Hooks         []HookDef
    // ... rest of fields ...
}
```

The field is placed adjacent to `PreambleMD` to group all free-form markdown string fields together.

## Loading

In `LoadPack()` (`internal/content/pack.go`), after loading `preamble.md`:

1. Try `constraints.{lang}.md` (if `lang` is set and not `"en"`)
2. Fall back to `constraints.md`
3. Store in `pack.ConstraintsMD`

Two-step locale resolution only — no `constraints.expanded.md` step (unlike `context.md`).

## Additive Layer Merge

In `MergeWith()` (`internal/content/merge.go`), add concatenation logic identical to `ContextMD`:

```go
if a.ConstraintsMD != "" {
    if a.AdditivePosition == "before" {
        merged.ConstraintsMD = a.ConstraintsMD + "\n\n" + base.ConstraintsMD
    } else {
        merged.ConstraintsMD = base.ConstraintsMD + "\n\n" + a.ConstraintsMD
    }
}
```

A company layer can append additional corporate constraints on top of official ones. When base `ConstraintsMD` is empty and additive is non-empty, the result is the additive content alone (consistent with `ContextMD` behavior).

## Rendering

In `RenderContext()` (`internal/content/render.go`), collect all non-empty `ConstraintsMD` from active packs, join them, and emit a single `## Constraints` section. Insert this block **after the preamble loop (line 46) and before the ContextMD loop (line 48)** in the current `RenderContext()` implementation.

### Placement in injected output

```text
# SAP Developer Context
**Developer Profile:** ...

## sap-devs Runtime Context     (if dynamic != nil)
...

> Preamble from base packs...

## Constraints                   ← NEW
1. Never write raw SQL...
2. Never use req.user without @requires...
3. ...

## SAP Developer Ecosystem       (base context.md)
...

## SAP CAP                       (cap context.md)
...

## Canonical Patterns            (if injectable samples)
...

## Recommended Learning Journeys (if any)
...
```

### Rendering logic

```go
// Collect constraints from all packs (in order)
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

Constraints from all packs are joined with `"\n\n"` (matching the separator used between context blocks) into a single flat numbered list. `TrimSpace` on each element prevents trailing newlines in individual files from producing extra blank lines. No per-pack subheadings.

## Budget Trimming

In `TrimPacks()` (`internal/content/render.go`), the byte-budget calculation must include `ConstraintsMD` alongside `ContextMD`:

```go
size := len(p.ContextMD) + len(p.ConstraintsMD)
```

This ensures trimming decisions account for each pack's content bytes, consistent with how `ContextMD` bytes are accounted for today (section headers emitted by `RenderContext` are not counted, same as existing behavior).

## Seed Content

Create `constraints.md` for these packs:

### `content/packs/base/constraints.md`

Universal constraints that apply to all SAP developer personas:

```markdown
1. Never store credentials, API keys, or secrets in source code — always use service bindings, environment variables, or the Destination Service
2. Never rely on AI training data for SAP API signatures or configurations — always verify against official SAP documentation or `sap-devs` commands
```

### `content/packs/cap/constraints.md`

```markdown
1. Never write raw SQL — always use `cds.ql` or CQL
2. Never use `req.user` without a `@requires` annotation on the service
3. Never depend on `@sap/` packages that are not publicly published on npmjs.com or not listed in the CAP released API documentation
4. Never bypass CAP's built-in authentication — use `@requires` and `@restrict` annotations
```

### `content/packs/abap/constraints.md`

```markdown
1. Never use internal SAP function modules — only use released (Tier-1) APIs
2. Never modify SAP standard objects — extend via clean-core patterns
3. Never use direct table selects in ABAP Cloud — use CDS-based views
4. Never skip ABAP Test Cockpit (ATC) checks before transport
```

### `content/packs/btp-core/constraints.md`

```markdown
1. Never hardcode BTP credentials or API keys — use the Destination Service or service bindings
2. Never use user-provided services when a managed service instance is available
3. Never deploy to production without environment-specific subaccount separation (dev/test/prod)
4. Never skip entitlement checks — verify quota allocation before provisioning services
```

## Testing

Add tests in `internal/content/render_test.go`:

1. `TestRenderContext_Constraints_AppearsWhenPresent` — verify `## Constraints` section renders
2. `TestRenderContext_Constraints_OmittedWhenEmpty` — no section if no packs have constraints
3. `TestRenderContext_Constraints_AfterPreambleBeforeContext` — ordering check (requires a `Base: true` pack with `PreambleMD` to trigger preamble rendering)
4. `TestRenderContext_Constraints_MultiplePacksMerged` — flat list from multiple packs
5. `TestRenderContext_Constraints_SkipsEmptyPacks` — packs without constraints don't add blank lines

Add tests in `internal/content/merge_test.go`:

1. `TestMergeWith_ConstraintsMD_After` — additive append (default position)
2. `TestMergeWith_ConstraintsMD_Before` — additive prepend
3. `TestMergeWith_ConstraintsMD_EmptyAdditivePreservesBase` — no-op when additive is empty

Add test in `internal/content/pack_test.go`:

1. `TestLoadPack_ConstraintsMD` — verify loading from `constraints.md` file

Add test for budget in `internal/content/render_test.go`:

1. `TestTrimPacks_BudgetIncludesConstraintsMD` — trimming accounts for constraints size

## Files Modified

| File | Change |
| ---- | ------ |
| `internal/content/pack.go` | Add `ConstraintsMD` field to `Pack`; load `constraints.md` in `LoadPack()` |
| `internal/content/merge.go` | Add `ConstraintsMD` concatenation in `MergeWith()` |
| `internal/content/render.go` | Render `## Constraints` section; include `ConstraintsMD` in budget calc |
| `internal/content/render_test.go` | Tests for rendering, ordering, budget |
| `internal/content/merge_test.go` | Tests for additive merge |
| `internal/content/pack_test.go` | Test for file loading |
| `content/packs/base/constraints.md` | Seed universal constraints |
| `content/packs/cap/constraints.md` | Seed constraints for CAP |
| `content/packs/abap/constraints.md` | Seed constraints for ABAP |
| `content/packs/btp-core/constraints.md` | Seed constraints for BTP |

No schema changes required — `constraints.md` is a sidecar markdown file like `context.md` and `preamble.md`, not a `pack.yaml` field.

## Non-Goals

- No structured YAML format for constraints (kept as raw markdown)
- No per-constraint IDs or severity levels
- No CLI subcommand for listing/managing constraints
- No per-pack subheadings in the rendered output
