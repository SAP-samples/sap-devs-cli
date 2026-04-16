# Base Layer Design

**Date:** 2026-04-16
**Status:** Approved
**Feature:** Shared base pack auto-injected into every profile

## Problem

Every pack that references shared SAP developer resources (help.sap.com, developers.sap.com, BTP cockpit, SAP Developer YouTube, SAP Developer News) must duplicate those links. As new packs are added, this duplication grows. There is also no way to guarantee a minimal shared context is always present regardless of the active profile.

## Goals

1. Introduce a `base: true` field in `pack.yaml` that marks a pack as always-injected regardless of profile.
2. Base packs are always rendered first (before profile packs), exempt from byte-budget trimming, and exempt from overlap deduplication.
3. Create `content/packs/base/` with initial shared SAP developer resources.
4. Update `docs/content/content-guide.md` and `docs/content-authoring.md` to document the base layer concept.

## Out of Scope

- Deduplicating shared content from existing packs (abap, cap, btp-core) — left for a future pass.
- The `minimal` profile (base layer only) — separate feature.
- The `all` profile — separate feature.

## Design

### Schema Change — `pack.yaml`

Add an optional `base` boolean field to `pack.yaml`:

```yaml
id: base
name: SAP Developer Base
description: Shared SAP developer resources injected into every profile
tags: [sap, btp, developers]
weight: 0
base: true
```

`base: true` is the only signal required. The `profiles` field is irrelevant for base packs and should be omitted. `weight` is still present for consistency with the schema but has no effect on rendering order — base packs always appear first. Among multiple base packs, relative ordering is by weight descending (to allow company/user layers to add a base pack that sorts before or after the official one).

### Go Struct Changes — `internal/content/pack.go`

1. Add `Base bool` to the `Pack` struct.
2. Add `Base bool \`yaml:"base,omitempty"\`` to the `packMeta` struct.
3. Assign `meta.Base` to `p.Base` in `LoadPack()`.

### LoadPacks — `internal/content/loader.go`

After `ApplyWeights()` returns, partition packs into `base` and `nonBase` slices and concatenate them:

```go
var base, nonBase []*Pack
for _, p := range packs {
    if p.Base {
        base = append(base, p)
    } else {
        nonBase = append(nonBase, p)
    }
}
return append(base, nonBase...)
```

`LoadPacks()` runs two sort passes on the full unseparated slice before partitioning:

1. **`sort.Slice` by base weight** (loader.go line 47–49) — sorts all packs descending by `pack.Weight`. This is the primary sort for base pack relative ordering when no profile is active.
2. **`ApplyWeights(packs, profile)`** (loader.go line 50) — re-sorts by profile-overridden weights. When `profile == nil`, `ApplyWeights` is a no-op and returns packs unchanged (profile.go line 59). In that case, relative base pack ordering is determined solely by the `sort.Slice` in step 1.

After both sorts, the partition runs. Weight therefore controls relative ordering among base packs (both between themselves, and when `profile == nil`). The `profiles` field in `pack.yaml` is not consulted for base packs. Note: `ApplyWeights` runs on the full unseparated slice, so if a profile YAML incorrectly declared an entry for a base pack ID, it would affect the base pack's pre-partition sort position (i.e., its position relative to other base packs) but not its always-first guarantee. The authoring contract is to omit `profiles` from base pack YAML entirely.

### TrimPacks — `internal/content/render.go`

Base packs are extracted before both trimming passes and appended to the result unconditionally:

```go
func TrimPacks(packs []*Pack, maxBytes int) []*Pack {
    var base, nonBase []*Pack
    for _, p := range packs {
        if p.Base {
            base = append(base, p)
        } else {
            nonBase = append(nonBase, p)
        }
    }
    trimmed := trimNonBase(nonBase, maxBytes)
    return append(base, trimmed...)
}
```

The existing deduplication and byte-budget logic moves into a private `trimNonBase()` helper with the same signature. No behaviour change for non-base packs.

**Rationale for budget exemption:** Base pack content is intentionally kept small. If a base pack is large enough to cause token budget problems, that is an authoring problem, not a trimming problem. The authoring contract is: keep base pack context minimal.

**Rationale for dedup exemption:** Base packs should not declare `overlaps` with technology packs. The overlap relationship is one-directional in intent. Note: a non-base pack that declares `overlaps: [base]` will silently have no effect after this change — the base pack is separated out before the deduplication pass runs, so it is never in the `included` set that dedup checks against. This is a known limitation; `overlaps: [base]` on a tech pack is unsupported and should not be used.

### New Pack — `content/packs/base/`

Files:

- `pack.yaml` — `id: base`, `base: true`, minimal metadata
- `context.md` — shared SAP developer resources: developers.sap.com, help.sap.com, community.sap.com, SAP Developer YouTube channel, SAP Developer News show, BTP cockpit entry point, general API/SDK discovery pointers

Content is intentionally short — a single concise section covering ecosystem entry points. Technology-specific content stays in the appropriate technology packs.

### Documentation Updates

**`docs/content/content-guide.md`** — `pack.yaml` schema section:

- Add `base` field entry: optional boolean, default false, marks the pack as always-injected
- Note: `profiles` field is ignored for base packs
- Note: base packs are always rendered first, exempt from byte-budget trimming and overlap deduplication
- Authoring contract: keep base pack context minimal

**`docs/content-authoring.md`** — add a "Base Layer" section:

- What it is: a pack always injected regardless of the active profile
- When to use: for content that should be present in every context window (shared portals, community links, ecosystem entry points)
- When not to use: for anything technology-specific — use a regular pack
- Token budget note: base packs are exempt from adapter trimming, so keep them small

## Testing

- Unit tests for `TrimPacks`: verify base packs survive when `maxBytes` is set to a value smaller than the base pack's content size.
- Unit tests for `TrimPacks`: verify base packs are not dropped by the overlap deduplication pass.
- Unit tests for `TrimPacks`: verify that when a base pack is present and the first non-base pack exceeds the budget, subsequent non-base packs are still dropped (existing break-on-first-oversize behaviour preserved for non-base packs).
- Unit tests for `LoadPacks` (or integration via `makeTempPacksDir`): verify base packs appear before non-base packs regardless of weight values. The temp pack YAML for the base pack must include `base: true` or the test passes vacuously.
- Existing tests must continue to pass unchanged.

## File Changelist

| File | Change |
| --- | --- |
| `internal/content/pack.go` | Add `Base bool` to `Pack` and `packMeta`; assign in `LoadPack()` |
| `internal/content/loader.go` | Add partition + pin step at end of `LoadPacks()` |
| `internal/content/render.go` | Extract base packs before trimming passes; add `trimNonBase()` helper |
| `internal/adapter/engine.go` | Review `Trimmed` flag (line ~123: `len(trimmed) < len(e.packs)`) and "budget too small" guard (line ~63: `len(trimmed) == 0 && maxBytes > 0`). With base packs always surviving TrimPacks, `len(trimmed) == 0` is no longer possible when a base pack exists — the guard's warning path is unreachable in that case. Adjust or document as needed. |
| `internal/content/render_test.go` | Add TrimPacks tests for base pack exemptions |
| `internal/content/loader_test.go` | Add LoadPacks test for base-first ordering |
| `content/packs/base/pack.yaml` | New file |
| `content/packs/base/context.md` | New file |
| `docs/content/content-guide.md` | Document `base` field in pack.yaml schema section; update "Creating a New Pack" guide to note base packs do not need a `profiles` entry |
| `docs/content-authoring.md` | Add "Base Layer" section |
