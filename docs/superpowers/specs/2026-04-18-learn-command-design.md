# Learn Command Design

**Date:** 2026-04-18
**Status:** Approved
**Approach:** Full Internal Package (Approach B)

## Overview

`sap-devs learn` is an umbrella command that aggregates content from three existing sources — learning journeys (learning.sap.com), tutorials (developers.sap.com), and Discovery Center missions — into a unified learning recommendation experience. It provides profile-filtered, experience-level-aware recommendations and curated learning paths.

## Subcommands

```
sap-devs learn                          → defaults to recommend
sap-devs learn recommend                → sectioned output (Journeys / Tutorials / Missions)
sap-devs learn search <query>           → cross-type search, unified table
sap-devs learn path                     → defaults to path list
sap-devs learn path list                → show available paths (curated + auto-filled)
sap-devs learn path show <path-id>      → show steps of a specific path (glamour-rendered)
sap-devs learn path open <path-id>      → open first URL in browser
```

### Shared Flags (PersistentFlags on parent)

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--level` | string | config `experience_level` | Filter by level (beginner, intermediate, advanced) |
| `--all` | bool | false | Bypass profile filtering |
| `--count` / `-n` | int | 10 | Max items per section (recommend) or total (search) |

### Subcommand-specific Flags

| Flag | Command | Type | Description |
|------|---------|------|-------------|
| `--pack` | recommend, path list | string | Filter to a specific pack's content |

## Data Model

### `internal/learn/types.go`

```go
type ItemType string

const (
    ItemJourney  ItemType = "journey"
    ItemTutorial ItemType = "tutorial"
    ItemMission  ItemType = "mission"
)

type LearnItem struct {
    Type     ItemType
    Title    string
    Slug     string   // unique ID (slug for journeys/tutorials, stringified ID for missions)
    Level    string   // normalized lowercase: "beginner", "intermediate", "advanced"
    Duration string   // human-readable: "4 hr", "45 min", ""
    URL      string
    Featured bool
    PackID   string
    Product  string
}

type Recommendations struct {
    Journeys  []LearnItem
    Tutorials []LearnItem
    Missions  []LearnItem
}

type RecommendOptions struct {
    Level  string
    PackID string
    All    bool
    Limit  int
}

type LearningPath struct {
    ID          string
    Name        string
    Description string
    Level       string
    PackID      string
    Steps       []PathStep
    Generated   bool // true = auto-filled, false = curated
}

type PathStep struct {
    Type ItemType
    Slug string
    Item *LearnItem // resolved at runtime
}
```

### Pack YAML: `paths.yaml`

New file alongside `tutorials.yaml`, `learning.yaml`, `discovery.yaml`:

```yaml
paths:
  - id: cap-getting-started
    name: Getting Started with CAP
    level: beginner
    description: Build your first CAP application from scratch
    steps:
      - type: journey
        slug: developing-with-sap-cloud-application-programming-model
      - type: tutorial
        slug: cap-getting-started
      - type: tutorial
        slug: hana-cloud-cap-create-project
      - type: mission
        slug: "4327"
```

### Pack struct addition

```go
// In content.Pack:
LearningPaths []LearningPathDef // raw YAML shape, before resolution
```

```go
// content.LearningPathDef is the YAML-parsed shape
type LearningPathDef struct {
    ID          string           `yaml:"id"`
    Name        string           `yaml:"name"`
    Description string           `yaml:"description,omitempty"`
    Level       string           `yaml:"level,omitempty"`
    Steps       []LearningPathStepDef `yaml:"steps"`
    PackID      string           // set at load time
}

type LearningPathStepDef struct {
    Type string `yaml:"type"` // "journey", "tutorial", "mission"
    Slug string `yaml:"slug"`
}
```

## Package Structure: `internal/learn/`

### `types.go`
Types listed above.

### `recommend.go`

```go
func Recommend(
    journeys []learning.LearningJourney,
    tutorials []tutorials.TutorialMeta,
    missions []discovery.Mission,
    packs []*content.Pack,
    opts RecommendOptions,
) *Recommendations
```

Each content type runs its own three-tier resolution:
1. Featured items from pack refs
2. Non-featured pack refs
3. Profile-filtered items from the full index (or all if `opts.All`)

Results are normalized into `LearnItem` and filtered by `opts.Level` if set. Each section is capped at `opts.Limit`.

**Level normalization:**
- Learning journeys: `"BEGINNER"` → `"beginner"` (API returns uppercase)
- Tutorials: already lowercase
- Missions: effort `"0"`/`"1"` → `"beginner"`, `"2"` → `"intermediate"`, `"3"` → `"advanced"`

### `search.go`

```go
func Search(
    journeys []learning.LearningJourney,
    tutorials []tutorials.TutorialMeta,
    missions []discovery.Mission,
    query string,
    opts RecommendOptions,
) []LearnItem
```

Searches all three indexes locally using case-insensitive substring match on title and description. Results are interleaved by relevance (match in title ranks higher than match in description). Level filtering applied if set. Capped at `opts.Limit`.

### `paths.go`

```go
func LoadPaths(packs []*content.Pack) []LearningPath

func AutoFillPaths(
    packs []*content.Pack,
    journeys []learning.LearningJourney,
    tutorials []tutorials.TutorialMeta,
    missions []discovery.Mission,
) []LearningPath

func ResolvePaths(
    paths []LearningPath,
    journeys []learning.LearningJourney,
    tutorials []tutorials.TutorialMeta,
    missions []discovery.Mission,
) []LearningPath
```

**LoadPaths:** reads `LearningPaths` from packs, converts `LearningPathDef` → `LearningPath` (unresolved).

**AutoFillPaths:** for packs without `paths.yaml`, collects featured tutorials/journeys/missions, groups by level, generates synthetic paths named `"{Pack Name} — {Level}"`. Only generates a path if ≥2 items exist at that level. Sets `Generated = true`.

**ResolvePaths:** populates each `PathStep.Item` by looking up the slug in the respective index. Steps whose items are no longer in the index get `Item = nil` (rendered with a "(not found)" marker).

## Config Changes

Add `ExperienceLevel` to `internal/config/config.go`:

```go
type Config struct {
    // ... existing fields ...
    ExperienceLevel string `yaml:"experience_level,omitempty"`
}
```

Valid values: `"beginner"`, `"intermediate"`, `"advanced"`, or `""` (no filtering). Set via `sap-devs config set experience_level beginner`. The `learn` commands read this as the default `--level` value; the `--level` flag overrides it.

## Content Loader Changes

In `LoadPack()` (internal/content/pack.go), add alongside existing YAML loading:

```go
if data, err := os.ReadFile(filepath.Join(packDir, "paths.yaml")); err == nil {
    var pathsYAML struct {
        Paths []LearningPathDef `yaml:"paths"`
    }
    _ = yaml.Unmarshal(data, &pathsYAML)
    pack.LearningPaths = pathsYAML.Paths
    for i := range pack.LearningPaths {
        pack.LearningPaths[i].PackID = pack.ID
    }
}
```

Content flattening helper in `internal/content/`:

```go
func FlattenLearningPaths(packs []*Pack) []LearningPathDef
```

## Command Layer

### `cmd/learn.go`

Parent command + `recommend` subcommand. Pattern matches `cmd/learning.go`:
- Uses `newContentLoader()` and `config.LoadProfile()` from root.go
- Loads packs via `loader.LoadPacks(profile, lang)`
- Loads indexes: `learning.LoadIndex()`, `tutorials.LoadIndex()`, `discovery.LoadMissionCache()`
- Missing indexes are tolerated (section skipped with hint message)
- If all three indexes missing: error "No content cached — run `sap-devs sync` first"
- Calls `learn.Recommend()`, renders three tabwriter sections

### `cmd/learn_search.go`

Cross-type search. Loads all three indexes, calls `learn.Search()`, renders unified table with TYPE column.

### `cmd/learn_path.go`

`path` parent (defaults to `path list`) + `list`, `show`, `open` subcommands.
- `list`: loads curated paths + auto-fills, renders table (NAME, LEVEL, STEPS, PACK, SOURCE)
- `show <path-id>`: resolves path, renders numbered step list with glamour markdown
- `open <path-id>`: opens first step's URL in browser via `pkg/browser`

### Registration (init)

```go
func init() {
    learnCmd.PersistentFlags().StringVar(&learnLevel, "level", "", "...")
    learnCmd.PersistentFlags().BoolVar(&learnAll, "all", false, "...")
    learnCmd.PersistentFlags().IntVarP(&learnCount, "count", "n", 10, "...")

    learnRecommendCmd.Flags().StringVar(&learnPack, "pack", "", "...")
    learnPathListCmd.Flags().StringVar(&learnPack, "pack", "", "...")

    learnPathCmd.AddCommand(learnPathListCmd, learnPathShowCmd, learnPathOpenCmd)
    learnCmd.AddCommand(learnRecommendCmd, learnSearchCmd, learnPathCmd)
    rootCmd.AddCommand(learnCmd)
}
```

## Output Formats

### `recommend`

```
Learning Journeys
  FEATURED  TITLE                                         LEVEL         DURATION
  ★         Developing with SAP CAP                       Beginner      4 hr
  ★         Becoming an SAP BTP Solution Architect        Intermediate  8 hr

Tutorials
  FEATURED  TITLE                                         LEVEL         TIME
  ★         Get Started Using SAP CAP                     Beginner      45 min
  ★         Deploy Your CAP App to Cloud Foundry          Intermediate  30 min

Discovery Center Missions
  FEATURED  TITLE                                         EFFORT
  ★         Develop a Full-Stack CAP Application          Easy
            GenAI Mail Insights with CAP and RAG          Medium
```

Sections with zero results omitted entirely.

### `search`

```
  #  TYPE      TITLE                                      LEVEL         TIME
  1  Journey   Developing with SAP CAP                    Beginner      4 hr
  2  Tutorial  Get Started Using SAP CAP                  Beginner      45 min
  3  Mission   Develop a Full-Stack CAP Application       Easy          -
```

### `path list`

```
  NAME                          LEVEL         STEPS  PACK    SOURCE
  Getting Started with CAP      Beginner      4      cap     curated
  SAP CAP — Intermediate        Intermediate  3      cap     auto
```

### `path show`

Glamour-rendered markdown:

```markdown
# Getting Started with CAP
**Level:** Beginner | **Pack:** cap | **Source:** curated

Build your first CAP application from scratch

1. [Journey] Developing with SAP CAP (4 hr)
   https://learning.sap.com/learning-journeys/developing-with-sap-cloud-application-programming-model

2. [Tutorial] Get Started Using SAP CAP (45 min)
   https://developers.sap.com/tutorials/cap-getting-started.html

3. [Tutorial] Create a CAP Project with HANA Cloud (30 min)
   https://developers.sap.com/tutorials/hana-cloud-cap-create-project.html

4. [Mission] Develop a Full-Stack CAP Application
   https://discovery-center.cloud.sap/missiondetail/4327/
```

## i18n

Keys added to `internal/i18n/catalogs/en.json` and `de.json`:

- `learn.short`, `learn.long`
- `learn.recommend.short`, `learn.search.short`
- `learn.path.short`, `learn.path.list.short`, `learn.path.show.short`, `learn.path.open.short`
- `learn.col_type`, `learn.col_title`, `learn.col_level`, `learn.col_duration`
- `learn.col_name`, `learn.col_steps`, `learn.col_pack`, `learn.col_source`
- `learn.section_journeys`, `learn.section_tutorials`, `learn.section_missions`
- `learn.no_content`, `learn.path_not_found`, `learn.step_not_found`

## JSON Schema

New `content/schemas/paths.yaml.schema.json` for `paths.yaml` validation. Register in `.vscode/settings.json`.

## Edge Cases

- **Missing individual indexes:** gracefully degrade — show available sections, skip missing with hint
- **Empty `paths.yaml`:** treated as no curated paths; auto-fill kicks in
- **Duplicate slugs across packs:** ResolvePaths resolves against global indexes; same item may appear in multiple paths
- **`--level` with no `experience_level` config:** no level filtering (show all levels)
- **Unresolved path steps:** shown with "(not found)" marker
- **All indexes missing:** error with "run `sap-devs sync` first"

## Testing

- Unit tests in `internal/learn/` for `Recommend()`, `Search()`, `AutoFillPaths()`, `ResolvePaths()` with fixture data
- Local validation via `go build ./...` and `go vet ./...`
- CI (ubuntu-latest) runs `go test ./...`

## Files to Create/Modify

### New files:
- `internal/learn/types.go`
- `internal/learn/recommend.go`
- `internal/learn/search.go`
- `internal/learn/paths.go`
- `internal/learn/recommend_test.go`
- `internal/learn/search_test.go`
- `internal/learn/paths_test.go`
- `cmd/learn.go`
- `cmd/learn_search.go`
- `cmd/learn_path.go`
- `content/packs/cap/paths.yaml` (seed content)
- `content/schemas/paths.yaml.schema.json`

### Modified files:
- `internal/config/config.go` — add `ExperienceLevel` field
- `internal/content/pack.go` — add `LearningPaths` field, `LearningPathDef` type, load `paths.yaml`
- `internal/content/merge.go` — merge `LearningPaths` in `MergeWith()`
- `internal/content/learning.go` (or new `internal/content/paths.go`) — `FlattenLearningPaths()` helper
- `internal/i18n/catalogs/en.json` — learn command strings
- `internal/i18n/catalogs/de.json` — learn command strings (German)
- `.vscode/settings.json` — register paths.yaml schema
- `CLAUDE.md` — add learn command to CLI commands table and architecture section
