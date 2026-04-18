# Learning Journeys Feature Design

**Date:** 2026-04-18
**Status:** Approved
**Command:** `sap-devs learning`

## Overview

Add a `learning` command to browse, search, and open SAP Learning Journeys from learning.sap.com. Follows the catalog-hybrid architecture: a full catalog JSON download at sync time for the index, with the search API used for the `search` subcommand's server-side fuzzy matching.

## Data Source

### Catalog Download (Primary — Index)

**Endpoint:** `GET https://learning.sap.com/service/catalog-download/json`

Returns the full SAP Learning catalog (~5.4MB, ~5,100 items). We filter to `Learning_type == "Learning Journey"` (~351 items) and cache the filtered index.

**Catalog item schema (learning journey):**

```json
{
  "LSC_product": "SAP Business Technology Platform",
  "LSC_product_category": "Business Technology Platform",
  "LSC_product_subcategory": "Application Development and Automation",
  "Role": "developer,architect",
  "Description": "...",
  "Title": "Becoming an SAP BTP Solution Architect",
  "Duration_in_hours": "6.00",
  "Level": "INTERMEDIATE",
  "Learning_object_ID": "LSC00246",
  "Learning_objectives": "<p>After completing this course...</p><ul><li>...</li></ul>",
  "Learning_type": "Learning Journey",
  "Direct_link": {
    "text": "https://learning.sap.com/learning-journeys/becoming-an-sap-btp-solution-architect",
    "hyperlink": "https://learning.sap.com/learning-journeys/becoming-an-sap-btp-solution-architect"
  },
  "Content_available_from": "2026-02-25"
}
```

### Search API (Secondary — `search` subcommand)

**Endpoint:** `GET https://learning.sap.com/service/learning/search/getCards(types='["learning-journey"]',filters='{"locale":"en-US","query":"<q>"}',sort='',limit=<n>,page=1)`

Returns paginated, faceted results with richer metadata (UUID IDs, descendants, facet counts). Used only for the `search` subcommand to leverage server-side fuzzy matching. Results cached with 1-hour TTL.

**Fallback:** If the search API is unreachable, fall back to local substring search on the cached index.

## Data Model

### `LearningJourney` struct

```go
type LearningJourney struct {
    ObjectID        string   // "LSC00246" from Learning_object_ID
    Title           string
    Slug            string   // extracted from Direct_link URL
    Description     string
    Level           string   // BEGINNER, INTERMEDIATE, ADVANCED
    DurationHours   string   // "6.00"
    Roles           []string // split from CSV: developer, architect, etc.
    Product         string   // "SAP Business Technology Platform"
    ProductCategory string   // "Business Technology Platform"
    ProductSubcat   string   // "Application Development and Automation"
    Objectives      string   // HTML learning objectives
    AvailableFrom   string   // "2026-02-25"
    URL             string   // full direct link
}
```

Slug is extracted from the `Direct_link.hyperlink` field by stripping the `https://learning.sap.com/learning-journeys/` prefix.

## Sync & Caching

### Sync Integration

New category `"learning"` registered in the sync engine with a 7-day default TTL (same as discovery).

`runLearningFetch()` added to `cmd/sync.go`:
1. Download `https://learning.sap.com/service/catalog-download/json`
2. Filter to `Learning_type == "Learning Journey"`
3. Parse each item into `LearningJourney` struct
4. Save filtered index to cache

The raw 5.4MB catalog is NOT persisted — only the filtered ~351-item index.

**Config TTL:** Add a `Sync.Learning` field to `internal/config/` (duration, default 168h) alongside the existing `Sync.Tutorials` etc. Register `"learning"` in `allCategories()` and the `ttls` map in `runSync()`.

### Cache Layout

```
~/.cache/sap-devs/learning/
├── index.json                    # []LearningJourney (~351 items, 7-day TTL)
└── search-{sha256[:8]}.json      # cached search API results (1-hour TTL)
```

### Stale Fallback

If the catalog download fails during sync, return the existing cached index (same pattern as discovery). If the search API fails during `learning search`, fall back to local substring search on the cached index.

## Pack Integration

### `learning.yaml` per pack

```yaml
# content/packs/cap/learning.yaml
profile_filters:
  products: ["SAP Business Technology Platform"]
  product_categories: ["Business Technology Platform"]
  roles: ["developer", "architect"]

journeys:
  - slug: becoming-an-sap-btp-solution-architect
    featured: true
  - slug: developing-with-sap-cloud-application-programming-model
    featured: true
  - slug: modernizing-integration-with-sap-integration-suite
```

### Types

```go
type LearningRef struct {
    Slug     string `yaml:"slug"`
    Featured bool   `yaml:"featured,omitempty"`
    PackID   string // set at load time
}

type LearningProfileFilters struct {
    Products          []string `yaml:"products"`
    ProductCategories []string `yaml:"product_categories"`
    Roles             []string `yaml:"roles"`
}
```

### Resolution Algorithm (Three-Tier)

1. **Featured first** — journeys referenced in packs with `featured: true`
2. **Pack-referenced** — other journeys explicitly listed in packs
3. **Profile-filtered** — remaining journeys matching `profile_filters` across active packs

`--all` flag bypasses profile filtering and shows all ~351 journeys.

### Profile Filtering

Filters are collected across all active packs via `CollectLearningFilters()`. A journey matches if any of these conditions hold:
- Its `Product` matches any filter product (substring)
- Its `ProductCategory` matches any filter product category (substring)
- Any of its `Roles` match a filter role (exact, case-insensitive)

### Merge Logic

Learning refs follow the tutorials pattern: refs are simply flattened across active packs (no slug-level merge/dedup in `merge.go`). If the same slug appears in multiple packs, duplicates are deduped by slug when resolving against the index. Additive packs append their refs normally.

## CLI Commands

### Command Tree

```
sap-devs learning
├── list    (default)   — profile-filtered learning journeys
├── search <query>      — server-side fuzzy search via search API
├── show <slug>         — detail view from cached index
└── open <slug>         — open in browser
```

### `learning list`

**Flags:**
- `--all` — bypass profile filtering, show all ~351 journeys
- `--pack <id>` — filter to a specific pack's curated journeys
- `--level <beginner|intermediate|advanced>` — filter by experience level
- `--role <role>` — filter by role
- `--count/-n <int>` — limit results (default 20)

**Output (table format):**

```
  ★  TITLE                                          LEVEL         DURATION
  ★  Becoming an SAP BTP Solution Architect          Intermediate  6 hr+
  ★  Developing with SAP CAP                         Beginner      4 hr+
     Modernizing Integration with SAP Integration..  Beginner      2 hr+
     Implementing Joule Across your Org Landscape    Intermediate  5 hr+
```

Note: The catalog JSON does not include a certification/achievement field. If this data is needed later, it can be sourced from the search API's richer response. For now, the table shows title, level, and duration.

Featured journeys (from pack curation) are marked with ★ and sorted first.

### `learning search <query>`

Calls the search API: `getCards(types='["learning-journey"]', filters='{"locale":"en-US","query":"<query>"}', limit=<n>, page=1)`. Results cached with 1-hour TTL using a SHA256 cache key derived from query + filters.

Falls back to local substring search on the cached index if the API is unreachable.

**Flags:** `--level`, `--role`, `--count/-n`

### `learning show <slug>`

Renders from cached index data:
- Title, level, duration, product, roles
- Description
- Learning objectives (HTML converted to terminal markdown via glamour)
- URL
- Available-from date

### `learning open <slug>`

Constructs `https://learning.sap.com/learning-journeys/<slug>` and opens in the default browser.

## Context Injection

When `sap-devs inject` runs, featured learning journeys from active packs are included in the injected context block.

### Injected Format

```markdown
### Recommended Learning Journeys

| Journey | Level | Duration |
|---------|-------|----------|
| [Becoming an SAP BTP Solution Architect](https://learning.sap.com/learning-journeys/becoming-an-sap-btp-solution-architect) | Intermediate | 6 hr+ |
| [Developing with SAP CAP](https://learning.sap.com/learning-journeys/developing-with-sap-cloud-application-programming-model) | Beginner | 4 hr+ |
```

Only featured journeys are injected (not the full profile-filtered list) to keep context concise — typically 3-5 per pack.

Rendered within the existing `file-inject` adapter output, appended after the pack's `context.md` content. No new adapter type needed.

## Package Structure

### New Package: `internal/learning/`

| File | Purpose |
|------|---------|
| `types.go` | `LearningJourney` struct, constants |
| `catalog.go` | `FetchCatalog()` — downloads JSON, filters to learning journeys, extracts slugs |
| `cache.go` | `SaveIndex()` / `LoadIndex()` / `IndexCacheAge()` — disk I/O with stale fallback |
| `search.go` | `Search()` (local substring), `FilterByLevel()`, `FilterByRole()`, `FindBySlug()` |
| `api.go` | `SearchAPI()` — calls `getCards` endpoint for `search` subcommand |

### New Files

| File | Purpose |
|------|---------|
| `cmd/learning.go` | All four subcommands |
| `internal/content/learning.go` | `FlattenLearningRefs()`, `CollectLearningFilters()` |
| `content/packs/cap/learning.yaml` | Curated CAP learning journeys |
| `content/packs/btp-core/learning.yaml` | Curated BTP learning journeys |
| `content/packs/abap/learning.yaml` | Curated ABAP learning journeys |
| `content/schemas/learning.yaml.schema.json` | JSON Schema for validation |

### Modified Files

| File | Change |
|------|--------|
| `internal/content/pack.go` | Add `LearningRefs` and `LearningFilters` fields to `Pack`; load `learning.yaml` in `LoadPack()` |
| `internal/content/merge.go` | Flatten learning refs across packs (no slug-level merge needed) |
| `internal/config/config.go` | Add `Sync.Learning` TTL field (default 168h) |
| `cmd/sync.go` | Add `runLearningFetch()`, register `"learning"` in `allCategories()` and TTL map |
| `cmd/root.go` | Register `learningCmd` |
| `internal/adapter/engine.go` | Render featured learning journeys in inject output |

## API Details

### Catalog Download

```
GET https://learning.sap.com/service/catalog-download/json
Accept: application/json

Response: JSON array of ~5,100 items
Filter: Learning_type == "Learning Journey" → ~351 items
No authentication required
```

### Search API

```
GET https://learning.sap.com/service/learning/search/getCards(
  types='["learning-journey"]',
  filters='{"locale":"en-US","query":"btp"}',
  sort='',
  limit=15,
  page=1
)

Response: {
  value: {
    results: [...],
    totalCount: N,
    facets: [...],
    prevPage: null,
    nextPage: 2,
    limit: 15
  }
}
No authentication required
```

## Schema (learning.yaml)

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "properties": {
    "profile_filters": {
      "type": "object",
      "properties": {
        "products": { "type": "array", "items": { "type": "string" } },
        "product_categories": { "type": "array", "items": { "type": "string" } },
        "roles": { "type": "array", "items": { "type": "string" } }
      }
    },
    "journeys": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["slug"],
        "properties": {
          "slug": { "type": "string" },
          "featured": { "type": "boolean" }
        }
      }
    }
  }
}
```
