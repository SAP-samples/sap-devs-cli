# Additive Content Layers — Design

**Date:** 2026-04-16
**Status:** Approved

## Problem

The current content layer system merges packs by ID using last-wins replacement: if a company, user, or project layer defines a pack with the same ID as an official pack, the official pack is completely discarded. A company that wants to add two extra tips to the official `cap` pack must copy and maintain the entire pack — including context, resources, tools, and MCP servers — just to make a small addition.

## Goal

Allow higher layers (company, user, project) to mark a pack as *additive*, meaning it augments the lower-layer pack with the same ID rather than replacing it. Official pack content is preserved; the additive layer contributes only what it explicitly defines.

---

## Data Model

### New `pack.yaml` fields

```yaml
additive: true              # marks this pack as augmenting rather than replacing
additive_position: after    # where additive content appears — "before" or "after" (default: "after")
```

- `additive` defaults to `false` — existing packs are unaffected.
- `additive_position` defaults to `"after"` when omitted or set to an unrecognised value.
- `additive_position` is only meaningful when `additive: true`; it is ignored otherwise.
- An additive pack with no matching base in the map becomes the base (no-op merge, existing behaviour).

### `packMeta` struct additions (`internal/content/pack.go`)

```go
Additive         bool   `yaml:"additive"`
AdditivePosition string `yaml:"additive_position"` // "before" | "after"
```

Both fields are promoted from `packMeta` into the `Pack` struct during `LoadPack()`.

### Metadata override rules (when `additive: true`)

| Field | Behaviour |
| --- | --- |
| `name` | Overrides base if non-empty |
| `description` | Overrides base if non-empty |
| `weight` | Overrides base if non-zero |
| `tags` | Union-merged (deduplicated) |
| `profiles`, `base`, `overlaps` | Always taken from base pack |

---

## Merge Logic — `Pack.MergeWith`

A new method on `*Pack` in `internal/content/pack.go`. `packMap` holds `*Pack` values and `LoadPack` returns `*Pack`, so the method uses pointer receivers throughout.

```go
// MergeWith returns a new *Pack that augments base with the content of a.
// Precondition: a.Additive must be true. If called with Additive == false,
// MergeWith is a no-op and returns base unchanged (safe guard, not a panic).
func (a *Pack) MergeWith(base *Pack) *Pack {
    if !a.Additive {
        return base
    }
    merged := *base // shallow copy of scalar fields; slices replaced below

    // Metadata: override on non-empty
    if a.Name != ""        { merged.Name = a.Name }
    if a.Description != "" { merged.Description = a.Description }
    if a.Weight != 0       { merged.Weight = a.Weight }
    merged.Tags = unionStrings(base.Tags, a.Tags)
    // base, profiles, overlaps always taken from base pack (already in merged via shallow copy)

    // Context: position controls order; applies to the resolved ContextMD string
    // (after locale and context.expanded.md selection in LoadPack).
    // An empty additive ContextMD (additive pack has no context file) preserves base unchanged.
    if a.ContextMD != "" {
        if a.AdditivePosition == "before" {
            merged.ContextMD = a.ContextMD + "\n\n" + base.ContextMD
        } else {
            merged.ContextMD = base.ContextMD + "\n\n" + a.ContextMD
        }
    }

    // Tips: both kept (no deduplication by title); position controls order.
    // Always produce a fresh slice to avoid aliasing the base pack's backing array.
    // Additive packs cannot replace a specific base tip — only append/prepend.
    if a.AdditivePosition == "before" {
        merged.Tips = append(append([]Tip(nil), a.Tips...), base.Tips...)
    } else {
        merged.Tips = append(append([]Tip(nil), base.Tips...), a.Tips...)
    }

    // Structured lists: additive replaces on matching ID, appends new entries.
    // PackID on Resource and MCPServer entries is re-stamped to the base pack's ID
    // so that downstream display (resources, mcp list) groups correctly.
    // Always produce fresh slices (same aliasing concern as Tips).
    // Parameter order for all three helpers: (base, additive, packID).
    merged.Resources  = mergeResources(base.Resources, a.Resources, base.ID)
    merged.Tools      = mergeTools(base.Tools, a.Tools)
    merged.MCPServers = mergeMCPServers(base.MCPServers, a.MCPServers, base.ID)

    // Profiles and Overlaps are taken from base (via shallow copy) but must be
    // fresh slices to avoid aliasing base's backing arrays if callers ever append.
    merged.Profiles = append([]string(nil), base.Profiles...)
    merged.Overlaps = append([]string(nil), base.Overlaps...)

    merged.Additive = false // merged result is not itself additive;
                            // a subsequent additive layer will merge into this
    return &merged
}
```

**Helper functions** (new, in `internal/content/pack.go` or a small `internal/content/merge.go`):

- `unionStrings(a, b []string) []string` — returns a fresh deduplicated slice: all elements of `a`, then elements of `b` not already in `a`.
- `mergeResources(base, additive []Resource, packID string) []Resource` — builds a fresh slice: starts with a copy of base entries (parameter 1), replaces any entry whose `ID` matches an additive entry (parameter 2), appends additive entries with no match; re-stamps `PackID = packID` on every entry in the result.
- `mergeTools(base, additive []ToolDef) []ToolDef` — same ID-replace-or-append logic for `ToolDef` (parameter 1 = base, parameter 2 = additive; no `PackID` field).
- `mergeMCPServers(base, additive []MCPServer, packID string) []MCPServer` — same as `mergeResources` (parameter 1 = base, parameter 2 = additive) with `PackID` re-stamping.

`Resource`, `ToolDef`, and `MCPServer` share no common interface, so three concrete helpers are used rather than a generic — this avoids adding `GetID()` methods to all three structs.

---

## Loader Change (`internal/content/loader.go`)

One conditional replaces the current single-line replace. `pack` and `existing` are both `*Pack`:

```go
if pack.Additive {
    if existing, ok := packMap[pack.ID]; ok {
        packMap[pack.ID] = pack.MergeWith(existing)
    } else {
        // No base pack found. The additive pack becomes the base as-is.
        // LoadPack has already stamped PackID on Resource and MCPServer entries,
        // and the pack's own Base/Profiles/Overlaps values apply directly.
        // Clear Additive so the stored entry does not appear additive to future
        // layers or to any code that inspects Pack.Additive at runtime.
        pack.Additive = false
        packMap[pack.ID] = pack
    }
} else {
    packMap[pack.ID] = pack // existing replace behaviour unchanged
}
```

**Multi-layer additive stacking** works correctly without extra logic: after `MergeWith`, the result has `Additive = false`. If a subsequent layer (e.g. project on top of company+official) also has `additive: true` for the same pack ID, it finds the already-merged pack as its base and applies another merge. Each layer in the chain augments the cumulative result.

No other changes to `LoadPacks()` are required.

---

## YAML Schemas

A new `content/schemas/` directory with one JSON Schema file per YAML content type. These provide inline validation and autocomplete in editors that support the YAML Language Server (e.g. VS Code with the Red Hat YAML extension).

### Files

```text
content/schemas/
  pack.schema.json
  resources.schema.json
  tools.schema.json
  mcp.schema.json
  profile.schema.json
```

### VS Code wiring (`.vscode/settings.json`)

```json
"yaml.schemas": {
  "./content/schemas/pack.schema.json":      "**/packs/*/pack.yaml",
  "./content/schemas/resources.schema.json": "**/packs/*/resources.yaml",
  "./content/schemas/tools.schema.json":     "**/packs/*/tools.yaml",
  "./content/schemas/mcp.schema.json":       "**/packs/*/mcp.yaml",
  "./content/schemas/profile.schema.json":   "**/profiles/*.yaml"
}
```

### Schema coverage

All schemas use `"additionalProperties": false` to catch typos in field names.

**`pack.schema.json`** covers all existing fields plus:

- `additive` (boolean, default false)
- `additive_position` (enum: `"before"` | `"after"`, default `"after"`)
- An `if/then` constraint enforcing that `additive_position` is only meaningful when `additive: true`

**`resources.schema.json`** covers: `id`, `title`, `url`, `type` (enum), `tags`, `advocate` (optional string)

**`tools.schema.json`** covers: `id`, `name`, `required`, `detect` (command + pattern), `install` (windows/macos/linux/all), `docs`

**`mcp.schema.json`** covers: `id`, `name`, `description`, `install` (command + args), `hosts`

**`profile.schema.json`** covers: `id`, `name`, `description`, `packs` (array of id + weight), `tip_tags`

---

## Documentation (`docs/content-authoring.md`)

### Changes

1. **VS Code setup note** — near the top, one paragraph pointing authors at `.vscode/settings.json` and the Red Hat YAML extension.

2. **`pack.yaml` field reference table** — two new rows: `additive` and `additive_position` with types, defaults, and the conditional relationship.

3. **New section: Additive Layers** — covering:
   - When to use additive mode (augmenting official packs from a company/user/project layer)
   - `additive_position: before|after` with a use-case for each
   - Per-file-type merge behaviour summary table
   - Worked example: a minimal additive company pack adding two tips and one resource to the official `cap` pack
   - No-base fallback behaviour

### Merge behaviour reference table (for the docs section)

| File | Merge behaviour |
| --- | --- |
| `context.md` | Additive content appended or prepended per `additive_position`; base preserved |
| `tips.md` | Both kept; additive tips ordered per `additive_position` |
| `resources.yaml` | Additive replaces matching `id`; new IDs appended |
| `tools.yaml` | Additive replaces matching `id`; new IDs appended |
| `mcp.yaml` | Additive replaces matching `id`; new IDs appended |
| `pack.yaml` metadata | `name`/`description` override if non-empty; `weight` overrides if non-zero; `tags` union-merged |

---

## Out of Scope

- Locale variants of additive context (`context.<lang>.md`) — additive merge applies to the resolved `ContextMD` string after locale selection. Additive packs may or may not provide locale context files; if the locale file is absent and no base `context.md` exists, `ContextMD` is empty and the `if a.ContextMD != ""` guard in `MergeWith` preserves the base context unchanged.
- `context.expanded.md` in additive packs — `LoadPack` is called on the additive pack's directory before `MergeWith`; if the additive pack's directory contains `context.expanded.md`, that expanded content is what gets merged (no special-casing). Additive packs should not provide `context.expanded.md` as a matter of authoring convention, but no implementation guard is needed.
- `base` and `profiles` fields in the no-base path — when an additive pack has no matching base in the map, it becomes the base as-is; its own `base` and `profiles` values apply directly. In the merge path these fields always come from the base pack (via shallow copy). Note: an additive company pack with `base: true` will have that field overridden by the official pack's `base: false` in a merge, which may be surprising — content authors should not set `base: true` in additive packs.
- Targeted tip replacement by title — `Tip` has no `ID` field; additive tips are always appended (or prepended), never used to replace a specific base tip. Content authors needing to replace a base tip must use a full replace-mode pack (leave `additive` unset).
- Three-way or conflict-reporting merge modes — last-wins and union cover all identified use cases.
- Additive mode for official packs relative to each other — official is the lowest layer; no layer exists below it.

---

## Testing

- Unit tests for `Pack.MergeWith` covering:
  - before/after position for context and tips
  - empty additive `ContextMD` preserves base context unchanged
  - tip ordering — fresh slice, no backing-array aliasing with base
  - list ID replacement and list append for Resources, Tools, MCPServers
  - `PackID` re-stamped to base ID on merged Resources and MCPServers
  - metadata override: non-empty name/description, non-zero weight, tags union
  - `Additive == false` precondition guard — returns base unchanged
  - no-base fallback: additive pack with no existing entry is stored as-is with its own PackID intact
- Unit tests for `unionStrings`, `mergeResources`, `mergeTools`, `mergeMCPServers` helpers.
- Integration test in `loader_test.go`: a synthetic 3-layer setup (official → company additive → project additive) verifying that each layer's contributions accumulate correctly in the final merged pack.
