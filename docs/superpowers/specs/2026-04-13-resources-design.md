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

Each pack may contain a `resources.yaml` file. The `Resource` struct already exists in `internal/content/pack.go`:

```go
type Resource struct {
    ID       string   `yaml:"id"`
    Title    string   `yaml:"title"`
    URL      string   `yaml:"url"`
    Type     string   `yaml:"type"`
    Tags     []string `yaml:"tags"`
    Advocate string   `yaml:"advocate,omitempty"`
    PackID   string   // set at load time, not in YAML
}
```

`resources.yaml` is already parsed by `LoadPack` into `pack.Resources`. The `PackID` field is **not** in YAML — it must be set by `LoadPack` after unmarshalling, by assigning `meta.ID` to each resource's `PackID`.

**Change to `LoadPack`:** After `yaml.Unmarshal(data, &pack.Resources)`, add:

```go
for i := range pack.Resources {
    pack.Resources[i].PackID = pack.ID
}
```

## Architecture

### What already exists (do not re-implement)

- `Resource` struct — in `internal/content/pack.go`
- `resources.yaml` loading — in `LoadPack` (populates `pack.Resources`)
- Layer deduplication — `LoadPacks` already handles this (later layers override by pack ID)
- `LoadPacks(nil)` — loads all packs unfiltered across all layers (nil profile = no weight filtering)

### New code: `internal/content/resources.go`

Three pure helper functions operating on an already-loaded `[]*Pack` slice:

```go
// FlattenResources collects all resources from all packs into a single slice.
func FlattenResources(packs []*Pack) []Resource

// FilterResources returns resources whose title, type, or any tag contains query
// (case-insensitive substring match).
func FilterResources(resources []Resource, query string) []Resource

// FindResource returns the first resource with an exact ID match, or nil.
func FindResource(resources []Resource, id string) *Resource
```

No filesystem access. No new loading. These operate on data already in memory.

### New code: `cmd/resources.go`

Thin presentation layer only. All data logic stays in `internal/content`.

## Subcommand Behaviour

### `resources list`

1. Load active profile: `config.LoadProfile` → `loader.FindProfile`
2. **No profile set → print `"No profile set. Run 'sap-devs profile set <name>' first."` and exit 1.**
   *(This intentionally diverges from `tip.go` which runs without a profile. `list` is profile-filtered by definition — showing all resources when no profile is set would be misleading.)*
3. Call `loader.LoadPacks(activeProfile)` — returns profile-weighted packs
4. Call `content.FlattenResources(packs)`
5. Print aligned table (no `PACK` column):

```
ID                          TYPE           TITLE
cap/docs-official           official-docs  CAP Documentation
cap/samples-github          sample         CAP Samples on GitHub
btp-core/discovery-center   official-docs  SAP Discovery Center
```

If no resources found: `"No resources found for your current profile."`.

### `resources search <query>`

1. Call `loader.LoadPacks(nil)` — all packs, no profile filtering
2. Call `content.FlattenResources(packs)` then `content.FilterResources(resources, query)`
3. Print aligned table with `PACK` column added:

```
ID                          PACK      TYPE           TITLE
cap/docs-official           cap       official-docs  CAP Documentation
abap/adt-guide              abap      official-docs  ABAP Development Tools Guide
```

If no matches: `"No resources found matching '<query>'."`.

Query argument is required; Cobra prints usage automatically if missing.

**Note:** `list` and `search` use separate table formatters — `list` has 3 columns, `search` has 4. They do not share a renderer.

### `resources open <id>`

1. Call `loader.LoadPacks(nil)` — all packs, not profile-filtered (user should not be blocked by profile when they know an ID)
2. Call `content.FindResource(content.FlattenResources(packs), id)` — exact ID match
3. If not found → `"Resource '<id>' not found. Use 'sap-devs resources list' or 'sap-devs resources search' to browse."` and exit 1
4. Open URL using `github.com/pkg/browser`
5. Print `"Opening: <title> — <url>"`

## Dependencies

**New:** `github.com/pkg/browser` — opens URLs in the default system browser (`xdg-open` on Linux, `open` on macOS, `rundll32 url.dll,FileProtocolHandler` on Windows). No CGO.

## Error Handling

- Missing `resources.yaml` in a pack: already silently skipped by `LoadPack` (existing behaviour)
- Malformed YAML: already silently skipped by `LoadPack` (existing behaviour — `_ = yaml.Unmarshal(...)`)
- Browser launch failure: print `"Could not open browser: <err>. URL: <url>"` and exit 0 (non-fatal)

## Testing

Tests in `internal/content/resources_test.go`:

- `TestFlattenResources` — two packs, verifies all resources collected and `PackID` set
- `TestFilterResources_TitleMatch` — substring in title returns resource
- `TestFilterResources_TagMatch` — substring in a tag returns resource
- `TestFilterResources_CaseInsensitive` — uppercase query matches lowercase title
- `TestFilterResources_NoMatch` — returns empty slice, no error
- `TestFindResource_Found` — exact ID returns correct resource
- `TestFindResource_NotFound` — unknown ID returns nil

The `open` command's browser launch is not unit-tested (side effect). Lookup logic is covered by `TestFindResource_*`.

## Files

- **Modify:** `internal/content/pack.go` — add `PackID string` field to `Resource`; set it in `LoadPack` after unmarshalling `resources.yaml`
- **Create:** `internal/content/resources.go` — `FlattenResources`, `FilterResources`, `FindResource`
- **Create:** `internal/content/resources_test.go`
- **Create:** `cmd/resources.go`
- **Modify:** `go.mod` / `go.sum` — add `github.com/pkg/browser`
