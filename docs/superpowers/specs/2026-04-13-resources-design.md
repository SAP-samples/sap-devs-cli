# sap-devs resources — Design Specification

## Goal

Add `sap-devs resources` with three subcommands — `list`, `search`, and `open` — so developers can discover and access curated SAP documentation, samples, and community links directly from their terminal.

## Commands

```
sap-devs resources list              Browse curated resources (profile-filtered)
sap-devs resources search <query>    Full-text search across all resources
sap-devs resources open <id>         Open a resource URL in the default browser
```

## Content Schema

Each pack may contain a `resources.yaml` file defining a list of resources:

```yaml
- id: cap/docs-official
  title: CAP Documentation
  url: https://cap.cloud.sap/docs
  type: official-docs
  tags: [reference, getting-started]

- id: cap/samples-github
  title: CAP Samples on GitHub
  url: https://github.com/SAP-samples/cloud-cap-samples
  type: sample
  tags: [examples, reference]
```

**Fields:**
- `id` — namespaced slug (`<pack>/<slug>`), globally unique across all packs
- `title` — human-readable name
- `url` — destination URL
- `type` — one of: `official-docs`, `sample`, `community`, `blog`, `tool`
- `tags` — string list used by search

`resources.yaml` is optional in a pack. Packs without it are silently skipped.

## Architecture

### Layer: `internal/content`

Add `LoadResources(packIDs []string) ([]Resource, error)` to the existing `ContentLoader`, following the same pattern as `LoadPacks()`, `LoadTips()`, etc.

**Loading logic:**
1. For each pack in `packIDs`, check for `resources.yaml` in each cache layer (official → company → local dev fallback when `SAP_DEVS_DEV=1`)
2. Merge by `id` — same override-by-ID semantics as adapters (inner layers override outer)
3. Attach `PackID` to each resource (the pack it was loaded from)
4. Return the deduplicated slice

**`Resource` struct:**
```go
type Resource struct {
    ID     string
    Title  string
    URL    string
    Type   string
    Tags   []string
    PackID string // set by loader, not in YAML
}
```

### Layer: `cmd/resources.go`

Thin presentation layer only. All data logic stays in `internal/content`.

## Subcommand Behaviour

### `resources list`

1. Load active profile via `config.LoadProfile` → `loader.FindProfile`
2. If no profile set → print `"No profile set. Run 'sap-devs profile set <name>' first."` and exit 1
3. Resolve pack IDs for the profile via `loader.LoadPacks`
4. Call `loader.LoadResources(packIDs)`
5. Print aligned table:

```
ID                          TYPE           TITLE
cap/docs-official           official-docs  CAP Documentation
cap/samples-github          sample         CAP Samples on GitHub
btp-core/discovery-center   official-docs  SAP Discovery Center
```

If no resources are found for the profile, print `"No resources found for your current profile."`.

### `resources search <query>`

1. Load resources from **all** packs (not profile-filtered) — pass all pack IDs from all loaded packs across the cache
2. Case-insensitive substring match against `title`, `type`, and each element of `tags`
3. Print same aligned table as `list`, with an added `PACK` column:

```
ID                          PACK      TYPE           TITLE
cap/docs-official           cap       official-docs  CAP Documentation
abap/adt-guide              abap      official-docs  ABAP Development Tools Guide
```

If no matches, print `"No resources found matching '<query>'."`.

Query argument is required; if missing, Cobra prints usage automatically.

### `resources open <id>`

1. Load resources from **all** packs (not profile-filtered)
2. Find resource with exact `id` match
3. If not found → print `"Resource '<id>' not found. Use 'sap-devs resources list' or 'sap-devs resources search' to browse."` and exit 1
4. Open URL using `github.com/pkg/browser`
5. Print `"Opening: <title> — <url>"`

## Dependencies

**New:** `github.com/pkg/browser` — opens URLs in the default system browser (`xdg-open` on Linux, `open` on macOS, `rundll32 url.dll,FileProtocolHandler` on Windows). No CGO, no heavy transitive dependencies.

## Error Handling

- Missing `resources.yaml` in a pack: silently skip (packs are not required to define resources)
- Malformed YAML: return a wrapped error with the file path
- Browser launch failure: print `"Could not open browser: <err>. URL: <url>"` — non-fatal, exits 0

## Testing

- `TestLoadResources_SinglePack` — loads a fixture resources.yaml, verifies slice contents
- `TestLoadResources_DeduplicatesById` — two packs with the same resource ID; inner layer wins
- `TestLoadResources_MissingFile` — pack without resources.yaml is skipped, no error
- `TestFilterResources_CaseInsensitive` — search matches title, type, and tag substrings
- `TestFilterResources_NoMatch` — returns empty slice, no error

Tests live in `internal/content/resources_test.go`. The `open` command's browser launch is not unit-tested (side effect); the lookup logic is covered by the search tests.

## Files

- **Create:** `cmd/resources.go`
- **Create:** `internal/content/resources.go` — `Resource` struct + `LoadResources()` + `FilterResources()`
- **Create:** `internal/content/resources_test.go`
- **Modify:** `go.mod` / `go.sum` — add `github.com/pkg/browser`
