# Discovery Center Integration Design

**Date:** 2026-04-18
**Status:** Approved
**Approach:** Curated YAML + live OData API (Approach A: thin custom client)

## Summary

Add a `sap-devs discovery` command that surfaces SAP Discovery Center content: missions, BTP services, and the BTP Guidance Framework. Uses curated YAML references per pack enriched by live data from two undocumented but functional OData V2 services on `discovery-center.cloud.sap`. Profile-aware filtering shows relevant content first.

## Key Decisions

- **Command name:** `discovery` (not `missions`) — leaves room for services and guidance under one umbrella.
- **Data strategy:** Curated YAML (IDs + names per pack) enriched with live API data. Search always hits the live API. 7-day cache TTL.
- **OData client:** Thin custom client (~150 lines) in `internal/discovery/`. No external OData library — the ecosystem lacks mature Go V2 batch libraries.
- **Profile integration:** Auto-filter by active profile using product/category/focus_tag mappings in `discovery.yaml`. Override with `--all`.
- **Three content types:** Missions (from `/platformx/`), Services (from `/servicecatalog/`), Guidance Framework (from `/platformx/`).

## API Discovery

The Discovery Center is a UI5 SPA backed by two OData V2 services requiring no authentication for read operations:

### `/platformx/` — Mission Engine (OData V2 $batch)

Requires CSRF token (fetched via `HEAD /platformx/` with `x-csrf-token: Fetch` header). All calls go through `POST /platformx/$batch` with `multipart/mixed` body using `\r\n` line endings.

**Quirk:** Function imports return results as a JSON-encoded string inside the OData wrapper: `{"d":{"FunctionName":"[{\"Id\":...}]"}}` — requires double JSON unmarshal.

| Function Import | Purpose | Parameters |
|----------------|---------|------------|
| `GetMissionCatalogContentV2` | Full mission catalog grouped by category | `username=''` |
| `GetViewFuzzySearchesCustomV3` | Fuzzy search with filters | `searchString`, `filterCategory`, `filterType=mission-catalog-search`, `filterProduct`, `filterLob`, `filterIndustry`, `filterFocusTags`, `filterPartners`, `filterQuickFilter`, `top` |
| `GetProductsCategories` | Product/category taxonomy | `version='1'` |
| `GetApplicationFocusTagsIndustryLob` | Filter facets (tags, industries, LOBs) | `version='1'` |
| `GetGuidanceFrameworkTree` | Guidance phase tree | (none) |
| `GetGuidanceFrameworkContentById` | Guidance node detail (markdown) | `id='<node-id>'` |

### `/servicecatalog/` — BTP Service Catalog (Standard OData V2 GET)

No CSRF or batch required. Standard OData V2 entity set queries.

| Entity Set | Purpose | Key fields |
|-----------|---------|------------|
| `ServiceDetailss` | BTP service details | Id, Name, ShortName, Category, ShortDescription, LicenseModelType, IsDeprecatedService |
| `Services` | Service summary list | Id, Name, ShortName, Category, Icon, Ribbon, ShortDesc |

## 1. Command Structure

```
sap-devs discovery
├── missions                    # list curated + profile-filtered missions
│   ├── missions search <query> # fuzzy search via API
│   └── missions open <id>     # open mission in browser
├── services                    # list BTP services (profile-filtered)
│   ├── services search <query> # filter services by name/category
│   └── services open <id>     # open service page in browser
└── guidance                    # show guidance framework tree
    ├── guidance show <id>     # display guidance content in terminal
    └── guidance open <id>     # open in browser
```

### Shared flags

| Flag | Description |
|------|-------------|
| `--all` | Bypass profile filtering, show everything |
| `--force` | Bypass cache, fetch fresh data |
| `--count N` | Limit results (default 20 for missions/services) |

### Missions flags

| Flag | Description |
|------|-------------|
| `--category` | Filter by category code (appdev, intgn, aicatg, etc.) |
| `--product` | Filter by product ID or name |
| `--effort` | Filter by effort level (0-3) |

### Services flags

| Flag | Description |
|------|-------------|
| `--category` | Filter by service category |
| `--deprecated` | Include deprecated services (hidden by default) |

### Guidance flags

| Flag | Description |
|------|-------------|
| `--domain` | Filter tree by domain (Extensibility, Integration, Data and Analytics) |

## 2. Data Model

### YAML: `discovery.yaml` per pack

```yaml
# content/packs/cap/discovery.yaml
profile_filters:
  products: ["1006"]        # BTP (numeric ID from GetProductsCategories)
  categories: ["appdev"]    # Category codes from mission data
  focus_tags: ["4"]         # Focus tag IDs (4 = Cloud Application Programming)

missions:
  - id: 4327
    name: Develop a Full-Stack CAP Application
    featured: true
  - id: 4371
    name: GenAI Mail Insights with CAP and RAG
  - id: 4064
    name: Develop a multitenant SaaS app using CAP

services:
  - id: 05e5c025-fcb9-4953-8489-7018aefe5aa7
    name: SAP Cloud Application Programming Model
    featured: true

guidance:
  - id: realize-application-dev-best-practices
    name: Application Development Best Practices
```

### Go structs — discovery.yaml wrapper (in `internal/content/pack.go`)

Unlike most pack YAML files (which are top-level arrays), `discovery.yaml` is a top-level object because it combines multiple content types and filter config. Loading uses an intermediate wrapper struct:

```go
type DiscoveryYAML struct {
    ProfileFilters *DiscoveryProfileFilters `yaml:"profile_filters,omitempty"`
    Missions       []DiscoveryMissionRef    `yaml:"missions,omitempty"`
    Services       []DiscoveryServiceRef    `yaml:"services,omitempty"`
    Guidance       []DiscoveryGuidanceRef   `yaml:"guidance,omitempty"`
}
```

### Go structs — content refs (in `internal/content/pack.go`)

```go
type DiscoveryMissionRef struct {
    ID       int    `yaml:"id"`
    Name     string `yaml:"name"`
    Featured bool   `yaml:"featured,omitempty"`
    PackID   string // set at load time
}

type DiscoveryServiceRef struct {
    ID       string `yaml:"id"`
    Name     string `yaml:"name"`
    Featured bool   `yaml:"featured,omitempty"`
    PackID   string
}

type DiscoveryGuidanceRef struct {
    ID     string `yaml:"id"`
    Name   string `yaml:"name"`
    PackID string
}

type DiscoveryProfileFilters struct {
    Products   []string `yaml:"products,omitempty"`
    Categories []string `yaml:"categories,omitempty"`
    FocusTags  []string `yaml:"focus_tags,omitempty"`
}
```

### Go structs — API response types (in `internal/discovery/types.go`)

```go
type Mission struct {
    ID                int    `json:"Id"`
    Name              string `json:"Name"`
    Category          string `json:"Category"`          // comma-separated codes
    SubCategory       string `json:"SubCategory"`
    Product           string `json:"Product"`           // comma-separated IDs
    Industry          string `json:"Industry"`
    LoB               string `json:"LoB"`
    FocusTags         string `json:"FocusTags"`         // comma-separated IDs
    Type              string `json:"Type"`              // "platform"
    PartnerCompany    string `json:"PartnerCompany"`
    ReferenceCustomers string `json:"ReferenceCustomers"`
    UCId              int    `json:"UCId"`              // use case ID
    UCLongDescription string `json:"UCLongDescription"`
    UCRibbonText      string `json:"UCRibbonText"`     // "featured" or null
    Effort            string `json:"Effort"`            // "0"-"3"
    MissionCount      int    `json:"MissionCount"`      // popularity metric
}

type MissionCatalogGroup struct {
    Name     string    `json:"name"`     // e.g., "Recommended by SAP"
    Desc     string    `json:"desc"`
    Missions []Mission `json:"missions"`
}

type Service struct {
    ID                string `json:"Id"`
    Name              string `json:"Name"`
    ShortName         string `json:"ShortName"`
    Category          string `json:"Category"`
    ShortDescription  string `json:"ShortDescription"`
    LicenseModelType  string `json:"LicenseModelType"`
    IsDeprecatedService bool `json:"IsDeprecatedService"`
}

type GuidanceNode struct {
    ID       string         `json:"id"`
    Name     string         `json:"name"`
    Domain   *string        `json:"domain"`   // nil for top-level phases
    Order    int            `json:"order"`
    Children []GuidanceNode `json:"children"`
}

type ProductCategory struct {
    ID       string            `json:"id"`
    Name     string            `json:"name"`
    Products []ProductCategory `json:"products,omitempty"` // nested sub-products
}

type Categories struct {
    Products []ProductCategory `json:"products"`
}

type Facets struct {
    FocusTags  []FacetItem `json:"focusTags"`
    Industries []FacetItem `json:"industries"`
    Lobs       []FacetItem `json:"lobs"`
}

type FacetItem struct {
    ID   string `json:"id"`
    Name string `json:"name"`
}

type SearchFilters struct {
    Category  string
    Product   string
    LoB       string
    Industry  string
    FocusTags string
    Partners  string
    Top       int
}
```

## 3. OData Client

### `internal/discovery/client.go`

```go
type Client struct {
    baseURL   string       // https://discovery-center.cloud.sap
    cacheDir  string       // ~/.cache/sap-devs/discovery/
    http      *http.Client // 15s timeout
    csrfToken string       // lazily fetched
}

func NewClient(cacheDir string) *Client

// CSRF + batch internals
func (c *Client) fetchCSRF() error                       // HEAD /platformx/ with x-csrf-token: Fetch
func (c *Client) batchGET(query string) ([]byte, error)  // multipart framing, POST, extract+unwrap

// Missions
func (c *Client) FetchMissions() ([]MissionCatalogGroup, error)
func (c *Client) SearchMissions(q string, f SearchFilters) ([]Mission, error)
func (c *Client) FetchCategories() (*Categories, error)
func (c *Client) FetchFacets() (*Facets, error)

// Services
func (c *Client) FetchServices() ([]Service, error)      // direct GET, no batch

// Guidance
func (c *Client) FetchGuidanceTree() ([]GuidanceNode, error)
func (c *Client) FetchGuidanceContent(id string) (string, error)
```

### Batch request framing

The `/platformx/$batch` endpoint requires strict `\r\n` line endings:

```
--batch_<uuid>\r\n
Content-Type: application/http\r\n
Content-Transfer-Encoding: binary\r\n
\r\n
GET <function-import-with-params> HTTP/1.1\r\n
Accept: application/json\r\n
Accept-Language: en\r\n
DataServiceVersion: 2.0\r\n
MaxDataServiceVersion: 2.0\r\n
X-Requested-With: XMLHttpRequest\r\n
\r\n
\r\n
--batch_<uuid>--\r\n
```

Response is `multipart/mixed` containing an inner HTTP response with the JSON body.

### JSON-string unwrapping

API responses use a double-encoding pattern:

```json
{"d": {"GetMissionCatalogContentV2": "[{\"Id\":3258,...}]"}}
```

The `batchGET` method:
1. Extracts the JSON body from the multipart response
2. Unmarshals the outer OData wrapper
3. Extracts the string value
4. Unmarshals the inner JSON string into the target type

### `/servicecatalog/` — direct GET

No batch or CSRF needed. Standard OData V2 query:

```
GET /servicecatalog/ServiceDetailss?$format=json&$select=Id,Name,ShortName,Category,ShortDescription,LicenseModelType,IsDeprecatedService
```

Response: `{"d": {"results": [{...}]}}`

## 4. Cache Layer

### `internal/discovery/cache.go`

```go
func LoadCache[T any](cacheDir, name string, ttl time.Duration) (T, bool)
func SaveCache[T any](cacheDir, name string, data T) error
```

### Cache structure

```
~/.cache/sap-devs/discovery/
  missions.json              (TTL: 7 days)
  services.json              (TTL: 7 days)
  guidance-tree.json         (TTL: 7 days)
  guidance/<id>.json         (TTL: 7 days, fetched on demand)
  categories.json            (TTL: 7 days)
  facets.json                (TTL: 7 days)
  search-<hash>.json         (TTL: 1 hour, per-query)
```

- `--force` flag bypasses all cache reads
- `sap-devs sync` refreshes discovery cache (added to `sync-state.json` TTL tracking)
- Search results cached by SHA-256 hash of `query + filters` concatenation

## 5. Profile Integration & Filtering

### Filter resolution flow

1. Load active profile → find matching packs → collect all `profile_filters` from `discovery.yaml` files
2. Union the product/category/focus_tag sets across all active packs
3. Apply filtering per content type

### Missions filtering

**List mode** (`sap-devs discovery missions`):
1. Show curated missions from active packs first (featured ones at the top)
2. Fetch full mission catalog from cache/API
3. Filter by unioned `profile_filters` (product, category, focus_tags)
4. Append filtered API missions, deduplicating by mission ID
5. Limit to `--count` (default 20)

**Search mode** (`sap-devs discovery missions search <query>`):
- Pass filters to `GetViewFuzzySearchesCustomV3` server-side: `filterProduct`, `filterCategory`, `filterFocusTags`
- With `--all`, pass empty filter strings

### Services filtering

- Filter `ServiceDetailss` by `Category` field matching profile's category mappings
- Category code mapping: `appdev` → "Application Development and Automation", `intgn` → "Integration", `dataanalytics` → "Data and Analytics", `aicatg` → "Artificial Intelligence"
- Unrecognized category codes are passed through as-is (no exclusion) to future-proof against new categories
- Hide deprecated services by default (`IsDeprecatedService: true`), show with `--deprecated`

### Guidance filtering

- Filter tree nodes by `domain` field matching pack relevance
- CAP/BTP packs: show "Extensibility", "Integration", "SAP BTP General"
- ABAP packs: show "Extensibility", "SAP BTP General"
- `--all` shows full unfiltered tree

### Flatten helpers (`internal/content/discovery.go`)

```go
func FlattenDiscoveryMissionRefs(packs []*Pack) []DiscoveryMissionRef
func FlattenDiscoveryServiceRefs(packs []*Pack) []DiscoveryServiceRef
func FlattenDiscoveryGuidanceRefs(packs []*Pack) []DiscoveryGuidanceRef
func CollectProfileFilters(packs []*Pack) DiscoveryProfileFilters
```

## 6. Output Formatting

### Missions list

```
#   FEATURED  EFFORT  NAME                                              CATEGORY
1   ★         2h      Develop a Full-Stack CAP Application              App Development
2   ★         2h      GenAI Mail Insights with CAP and RAG              AI, App Development
3             1h      Implement Observability in a Full-Stack CAP App   App Development
4             2h      Develop a multitenant SaaS app using CAP          App Development
```

### Missions search

```
#   EFFORT  NAME                                              CATEGORY         PARTNER
1   2h      Develop a Full-Stack CAP Application              App Development  SAP
2   2h      GenAI Mail Insights with CAP and RAG              AI, App Dev      SAP
3           Develop a multitenant SaaS app using CAP          App Development  SAP
```

### Services list

```
#   NAME                                        CATEGORY                              PRICING
1   ★ SAP Cloud Application Programming Model   Application Development and Automation  Free Tier
2   SAP HANA Cloud                              Data and Analytics                      Cloud Credits
3   SAP Integration Suite                       Integration                             Free Tier
```

### Guidance tree

```
PHASE       TOPIC                                    DOMAIN
Discover    Discover SAP BTP                         SAP BTP General
            SAP BTP Use Case Identification          SAP BTP General
Prepare     Organizational Readiness                 SAP BTP General
            Extension Use Case Assessment            Extensibility
Explore     Solution Planning                        SAP BTP General
            Extension Solution Architecture Design   Extensibility
Realize     Application Development Best Practices   Extensibility
Deploy      Solution Deployment and Delivery         SAP BTP General
Run         Solution Operation                       SAP BTP General
```

### Guidance show

Renders markdown content directly to terminal. Strips HTML `<br>` tags, renders links as `[text](url)`.

### URLs for `open` commands

| Content type | URL pattern |
|-------------|-------------|
| Missions | `https://discovery-center.cloud.sap/missiondetail/<id>/` |
| Services | `https://discovery-center.cloud.sap/serviceCatalog/<id>` |
| Guidance | `https://discovery-center.cloud.sap/guidance-framework/<id>` |

## 7. i18n

Keys added to `internal/i18n/catalogs/en.json` and `de.json`:

```
discovery.short, discovery.long
discovery.missions.short, discovery.missions.search.short, discovery.missions.open.short
discovery.services.short, discovery.services.search.short, discovery.services.open.short
discovery.guidance.short, discovery.guidance.show.short, discovery.guidance.open.short
discovery.col_name, discovery.col_category, discovery.col_effort, discovery.col_featured
discovery.col_pricing, discovery.col_phase, discovery.col_topic, discovery.col_domain
discovery.col_partner
discovery.err_no_missions, discovery.err_no_services, discovery.err_fetch
```

## 8. Content Loader Integration

### Pack loading (`internal/content/loader.go`)

`LoadPack` reads `discovery.yaml` alongside other YAML files, populating the `Pack` struct:

```go
// Added to Pack struct
DiscoveryMissions  []DiscoveryMissionRef  // from discovery.yaml
DiscoveryServices  []DiscoveryServiceRef
DiscoveryGuidance  []DiscoveryGuidanceRef
DiscoveryFilters   *DiscoveryProfileFilters
```

### Additive layer support

Packs with `additive: true` merge discovery refs additively (append/prepend by `AdditivePosition`), same as other content types.

## 9. Sync Integration

- Discovery cache refresh added to `sap-devs sync` flow
- New category in `sync-state.json`: `"discovery"` with 7-day TTL
- `sap-devs sync --force` refreshes all discovery cache files
- Fetches: missions catalog, services list, guidance tree, categories, facets

## 10. Schema & Documentation

### JSON Schema

`content/schemas/discovery.schema.json` — validates `discovery.yaml` per pack. Wired into `.vscode/settings.json`.

### Documentation updates

| File | Changes |
|------|---------|
| `CLAUDE.md` | Add `discovery` to CLI Commands table, add Discovery section to Architecture Overview |
| `docs/content-authoring.md` | Document `discovery.yaml` format, profile_filters, curated refs |

## 11. New Files

| File | Purpose |
|------|---------|
| `internal/discovery/client.go` | OData client (CSRF, batch, direct GET) |
| `internal/discovery/cache.go` | Generic cache load/save with TTL |
| `internal/discovery/types.go` | Mission, Service, GuidanceNode, Filters, Categories structs |
| `internal/discovery/discovery.go` | Enrich, filter, search orchestration |
| `internal/content/discovery.go` | Flatten/collect helpers for discovery refs from packs |
| `cmd/discovery.go` | Parent command + missions subcommands |
| `cmd/discovery_services.go` | Services subcommands |
| `cmd/discovery_guidance.go` | Guidance subcommands |
| `content/packs/base/discovery.yaml` | Base pack curated missions/services/guidance |
| `content/packs/cap/discovery.yaml` | CAP pack curated refs + profile filters |
| `content/packs/btp-core/discovery.yaml` | BTP pack curated refs + profile filters |
| `content/packs/abap/discovery.yaml` | ABAP pack curated refs + profile filters |
| `content/schemas/discovery.schema.json` | YAML validation schema |

## 12. Error Handling

- **Network failure:** Fall back to cached data if available (even if stale). Show warning: "Using cached data (last updated: <date>). Run with --force to retry."
- **CSRF token failure:** Retry once, then fail with clear error message.
- **Empty API response:** Show curated-only results from YAML. Warn that live data is unavailable.
- **Individual enrichment failure:** Show curated ref with name only (no description/effort). Don't block the full list.
- **Rate limiting / 429:** Respect Retry-After header. Cache aggressively to minimize requests.
