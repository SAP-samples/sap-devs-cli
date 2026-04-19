# Error Pattern Library Design

**Date:** 2026-04-19
**Status:** Approved

## Problem

When an AI agent encounters a SAP error like `No 'default' database configured` or `AMDP method must be static`, it guesses or web-searches. Most SAP errors have well-known, stable fixes that belong in the tool as structured data.

## Solution

Add `known_errors.yaml` per pack listing common SAP error messages with their causes and fixes. The data is:

1. **Injected** as a compact `## Known Errors` table in the AI context block
2. **Queryable** via `sap-devs errors list` and `sap-devs errors search <query>`
3. **Merged** across content layers using the standard ID-based replace-or-append pattern

## Approach

Flat array YAML — same pattern as `resources.yaml`, `samples.yaml`, `tools.yaml`. No categories or grouping beyond tags.

## Data Model

### Struct

```go
type KnownError struct {
    ID      string   `yaml:"id"`
    Pattern string   `yaml:"pattern"`
    Cause   string   `yaml:"cause"`
    Fix     string   `yaml:"fix"`
    Docs    string   `yaml:"docs,omitempty"`
    Tags    []string `yaml:"tags,omitempty"`
    PackID  string
}
```

### Pack Field

```go
KnownErrors []KnownError
```

### YAML File

`known_errors.yaml` per pack, flat array:

```yaml
- id: cap/no-default-db
  pattern: "No 'default' database configured"
  cause: No database binding in cds.requires; common in new projects or missing .env
  fix: Add `cds.requires.db.kind = sqlite` for local dev, or bind a HANA service for BTP
  docs: https://cap.cloud.sap/docs/node.js/databases
  tags: [database, local-dev]
```

**ID convention:** `pack/slug` (e.g., `cap/no-default-db`), matching resources and samples.

## Loading

In `LoadPack` (internal/content/pack.go), after existing YAML loaders:

```go
if data, err := os.ReadFile(filepath.Join(packDir, "known_errors.yaml")); err == nil {
    _ = yaml.Unmarshal(data, &pack.KnownErrors)
    for i := range pack.KnownErrors {
        pack.KnownErrors[i].PackID = pack.ID
    }
}
```

## Merging

In `MergeWith` (internal/content/merge.go), ID-based replace-or-append:

```go
func mergeKnownErrors(base, additive []KnownError, packID string) []KnownError {
    result := make([]KnownError, len(base))
    copy(result, base)
    for _, a := range additive {
        replaced := false
        for i, b := range result {
            if b.ID == a.ID {
                result[i] = a
                replaced = true
                break
            }
        }
        if !replaced {
            result = append(result, a)
        }
    }
    for i := range result {
        result[i].PackID = packID
    }
    return result
}
```

Called in `MergeWith`:

```go
merged.KnownErrors = mergeKnownErrors(base.KnownErrors, a.KnownErrors, base.ID)
```

## Schema

`content/schemas/known_errors.schema.json` — JSON Schema Draft-07, array of objects.

**Required:** `id`, `pattern`, `cause`, `fix`
**Optional:** `docs`, `tags`

Wire into `.vscode/settings.json` for YAML validation.

## Injection Rendering

In `RenderContext` (internal/content/render.go), after Canonical Patterns and Learning Journeys, append:

```markdown
## Known Errors

| Error Pattern | Cause | Fix |
|---|---|---|
| No 'default' database configured | No database binding in cds.requires | Add `cds.requires.db.kind = sqlite` for local dev |
```

**Rules:**
- Only rendered if any `KnownErrors` exist across all packs
- `docs` URL omitted from table to save tokens (available via CLI)
- Errors ordered by pack weight (already sorted)
- Pipe characters (`|`) in cell values must be escaped to `\|` to prevent table breakage
- No verbosity gating for now

## CLI Command

**File:** `cmd/known_errors.go`

### `sap-devs errors list`

Lists all known errors from active packs as a table (pattern, cause, pack). Supports `--pack` and `--tags` filtering flags for consistency with other list commands.

### `sap-devs errors search <query>`

Case-insensitive substring match against `pattern`, `cause`, and `fix` fields. Displays full detail:

```
$ sap-devs errors search "database configured"

  No 'default' database configured
  Pack:  cap
  Cause: No database binding in cds.requires; common in new projects or missing .env
  Fix:   Add `cds.requires.db.kind = sqlite` for local dev, or bind a HANA service for BTP
  Docs:  https://cap.cloud.sap/docs/node.js/databases
```

No interactive TUI — filtered text output only.

## Seed Data

### `content/packs/cap/known_errors.yaml` (~5-8 entries)

| ID | Pattern |
|---|---|
| cap/no-default-db | No 'default' database configured |
| cap/eisdir | EISDIR: illegal operation on a directory |
| cap/no-service-def | No service definition found |
| cap/cannot-find-cds | Cannot find module '@sap/cds' |
| cap/hdi-not-bound | HDI container not bound |

### `content/packs/abap/known_errors.yaml` (~3-5 entries)

| ID | Pattern |
|---|---|
| abap/amdp-static | AMDP method must be static |
| abap/clean-core-access | Access to object not permitted (clean core) |
| abap/cds-activation | CDS view activation failed |

## Files Changed

| File | Change |
|---|---|
| `internal/content/pack.go` | Add `KnownError` struct, `KnownErrors` field on `Pack` |
| `internal/content/pack.go` (`LoadPack`) | Load `known_errors.yaml` |
| `internal/content/known_errors.go` | `FlattenKnownErrors`, `FilterKnownErrors` helpers |
| `internal/content/merge.go` | Add `mergeKnownErrors`, call in `MergeWith` |
| `internal/content/render.go` | Render `## Known Errors` table in `RenderContext` |
| `content/schemas/known_errors.schema.json` | JSON Schema for validation |
| `.vscode/settings.json` | Wire schema for `known_errors.yaml` |
| `cmd/known_errors.go` | `errors list` and `errors search` commands |
| `cmd/root.go` | Register `errorsCmd` |
| `content/packs/cap/known_errors.yaml` | Seed CAP errors |
| `content/packs/abap/known_errors.yaml` | Seed ABAP errors |
| `internal/i18n/catalogs/en.json` | Add `errors.*` i18n keys |
| `internal/i18n/catalogs/de.json` | Add `errors.*` German translations |
| `CLAUDE.md` | Update CLI Commands table and Content Layer docs |
