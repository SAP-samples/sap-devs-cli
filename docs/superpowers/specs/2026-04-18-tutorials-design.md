# Tutorials Feature Design (Phase 1)

**Date:** 2026-04-18
**Status:** Approved
**Approach:** Hybrid GitHub + API fallback (Approach A — two-phase sync)

## Summary

Add a `sap-devs tutorial` command that fetches, indexes, and renders tutorials from the `sap-tutorials` GitHub organization (~1,290 tutorials across 22 repos). Phase 1 covers content ingestion, full-text search, terminal rendering (both full-document and interactive step-by-step TUI), and per-user progress tracking. Curated `tutorials.yaml` per pack provides profile-filtered browsing; the full GitHub index enables cross-cutting search.

## Key Decisions

- **Content source:** Tutorial markdown from `github.com/sap-tutorials/*` repos (public, no auth needed). Raw markdown is more efficient than parsing developers.sap.com HTML.
- **Metadata source:** YAML frontmatter in each tutorial's markdown file. Enriched by developers.sap.com Solr API when accessible; silent fallback to GitHub-only when WAF-blocked (403).
- **Scope:** Full index of all ~1,290 tutorials cached locally. Curated `tutorials.yaml` per pack for profile-matched display in `tutorial list`. `tutorial search` searches the entire index.
- **Index strategy:** Frontmatter-only scan. Fetch `repository-groups.json` for slug→repo mapping, GitHub Trees API for folder listing (22 API calls), `raw.githubusercontent.com` for markdown content (no API rate limit). Full tutorial content fetched on demand for `tutorial show`.
- **Terminal UX:** Flag-toggled. Default: full glamour-rendered markdown. `-i` flag: interactive Bubbletea TUI with step navigation. Configurable default via `tutorial.interactive` config key.
- **Progress tracking:** JSON file in XDG data directory. Tracks current step, completed steps, timestamps per tutorial.
- **Injection:** Not in Phase 1. Data model is designed so Phase 2 can inject active-tutorial context into AI tools.

## 1. Data Model

### 1.1 Curated References — `tutorials.yaml` per pack

Minimal YAML declaring which tutorials are relevant to a pack. All metadata resolved from the cached index.

```yaml
# content/packs/cap/tutorials.yaml
- slug: cap-getting-started
  featured: true
- slug: hana-cloud-cap-create-project
- slug: cap-service-deploy-to-cf
  featured: true
```

### 1.2 Structs

#### TutorialRef (in `internal/content/pack.go`)

Loaded from `tutorials.yaml` per pack. Follows the same pattern as `Sample`, `Influencer`, etc.

```go
type TutorialRef struct {
    Slug     string `yaml:"slug"`
    Featured bool   `yaml:"featured,omitempty"`
    PackID   string // set at load time
}
```

Added to the `Pack` struct:

```go
type Pack struct {
    // ... existing fields ...
    TutorialRefs []TutorialRef
}
```

#### TutorialMeta (in `internal/tutorials/types.go`)

A resolved tutorial entry in the full index. Built from GitHub frontmatter + optional API enrichment.

```go
type TutorialMeta struct {
    Slug        string   `json:"slug"`
    Title       string   `json:"title"`
    Description string   `json:"description"`
    Time        int      `json:"time"`          // minutes
    Level       string   `json:"level"`         // beginner, intermediate, advanced
    Tags        []string `json:"tags"`
    PrimaryTag  string   `json:"primary_tag"`
    Author      string   `json:"author,omitempty"`
    Repo        string   `json:"repo"`          // sap-tutorials repo name
    URL         string   `json:"url"`           // https://developers.sap.com/tutorials/{slug}.html
    Parser      string   `json:"parser"`        // "v2" or ""
}
```

#### Tutorial (in `internal/tutorials/types.go`)

Fully parsed tutorial with step content. Created on demand when `tutorial show` is called.

```go
type Tutorial struct {
    TutorialMeta
    Prerequisites string         `json:"prerequisites,omitempty"`
    YouWillLearn  []string       `json:"you_will_learn,omitempty"`
    Steps         []TutorialStep `json:"steps"`
}

type TutorialStep struct {
    Number  int    `json:"number"`
    Title   string `json:"title"`
    Content string `json:"content"` // markdown for this step
}
```

#### TutorialProgress (in `internal/tutorials/progress.go`)

Per-user progress state.

```go
type TutorialProgress struct {
    Slug           string     `json:"slug"`
    CurrentStep    int        `json:"current_step"`
    CompletedSteps []int      `json:"completed_steps"`
    TotalSteps     int        `json:"total_steps"`
    StartedAt      time.Time  `json:"started_at"`
    LastAccessed   time.Time  `json:"last_accessed"`
    CompletedAt    *time.Time `json:"completed_at,omitempty"`
}
```

## 2. Content Pipeline

### 2.1 Sync Flow

New sync category `"tutorials"` added to `allCategories()`. Independent category (not archive-based). TTL: 7 days.

```
runTutorialsFetch(cacheDir, officialCache, force, ttl)
  │
  ├─ 1. Fetch repo list
  │     GET raw.githubusercontent.com/sap-tutorials/Tutorials/master/config/repository-groups.json
  │     → flat array of {name, urlBase, description} objects — lists repos in the org
  │     → does NOT contain slug mappings; slug discovery happens in step 2
  │     → cache as {cacheDir}/tutorials/repository-groups.json
  │
  ├─ 2. For each repo, resolve default branch + list tutorial folders
  │     GET api.github.com/repos/sap-tutorials/{repo}
  │     → read default_branch field (varies: "master" for Tutorials, "main" for others)
  │     GET api.github.com/repos/sap-tutorials/{repo}/git/trees/{default_branch}?recursive=1
  │     → extract paths matching tutorials/*/
  │     → ~42 API calls total (2 per repo × ~21 repos; within unauthenticated 60/hour limit)
  │     → cache default_branch per repo to avoid re-fetching on incremental syncs
  │
  ├─ 3. For each slug, fetch frontmatter
  │     GET raw.githubusercontent.com/sap-tutorials/{repo}/{default_branch}/tutorials/{slug}/{slug}.md
  │     → raw.githubusercontent.com is a CDN — no API rate limit
  │     → parse YAML frontmatter only (stop at closing ---)
  │     → extract: title, description, time, tags, primary_tag, author, parser
  │     → build TutorialMeta entry
  │
  ├─ 4. (Optional) API enrichment
  │     GET developers.sap.com/bin/sapdx/v3/solr/search?json=...
  │     → if 200: merge mission/group membership, featured flags
  │     → if 403: skip silently, debug log only
  │     → honest User-Agent header (sap-devs-cli/vX.Y.Z)
  │
  └─ 5. Save index
        []TutorialMeta → {cacheDir}/tutorials/index.json
```

**Parallelism:** Steps 2-3 parallelized across repos, bounded to ~5 concurrent goroutines. Raw content fetches within a repo are batched.

**Incremental sync:** Compare cached index against tree listings. GitHub Trees API returns a SHA per tree — skip repos whose SHA hasn't changed since last sync. Only re-fetch tutorials in changed repos.

### 2.2 On-Demand Content Fetch

When `tutorial show <slug>` needs full content:

```
tutorials.FetchContent(cacheDir, meta TutorialMeta) (*Tutorial, error)
  │
  ├─ Check cache: {cacheDir}/tutorials/content/{slug}.json
  │   → if exists and fresh, unmarshal and return
  │
  ├─ GET raw.githubusercontent.com/sap-tutorials/{repo}/{default_branch}/tutorials/{slug}/{slug}.md
  ├─ Parse full markdown → Tutorial struct (parser v1 or v2)
  ├─ Cache as {cacheDir}/tutorials/content/{slug}.json
  └─ Return *Tutorial
```

### 2.3 Cache Layout

```
~/.cache/sap-devs/
  tutorials/
    repository-groups.json         # repo list from GitHub
    repos.json                     # repo → {default_branch, tree_sha} for incremental sync
    index.json                     # []TutorialMeta (~1,290 entries)
    content/
      cap-getting-started.json     # full parsed Tutorial (on demand)
      hana-cloud-cap-create-project.json
```

### 2.4 GitHub Rate Limiting

- **Repo metadata + Trees API:** ~42 requests per full sync (2 per repo × ~21 repos). Within the 60 req/hour unauthenticated limit. Incremental syncs skip unchanged repos (cached tree SHA), reducing to only changed repos.
- **Raw content:** Uses `raw.githubusercontent.com` CDN — no API rate limit.
- **With token:** If `GITHUB_TOKEN` is set, used for API calls (5,000 req/hour). Not required. Note: the existing `credentials.Resolve()` targets `github.com/SAP-samples`; tutorials use public `github.com`, so a separate token resolution path checks `GITHUB_TOKEN` / `GH_TOKEN` only.

## 3. Markdown Parser

### 3.1 Format Detection

Check `parser` field in YAML frontmatter. `parser: v2` → v2 parser. Otherwise → v1 (legacy ACCORDION).

### 3.2 v2 Parser (Current Standard)

Steps delimited by `### Heading` (H3). Preamble (everything before first `###`) parsed for prerequisites and "you will learn" sections.

- **Title:** `title` frontmatter → first `# H1` → slug as fallback
- **Description:** `description` frontmatter → `<!-- description -->` HTML comment
- **Prerequisites:** Content under `## Prerequisites` heading
- **You will learn:** Bullet list under `## You will learn` heading
- **Steps:** Each `### Title` starts a new step; content runs until the next `###` or EOF

### 3.3 v1 Parser (Legacy ACCORDION)

Steps delimited by `[ACCORDION-BEGIN [Step N: ](Title)]` / `[ACCORDION-END]`. Same output structure as v2.

### 3.4 OPTION Blocks

`[OPTION BEGIN [Tab Name]]` / `[OPTION END]` pairs preserved in step content as labeled sections. Phase 1 renders them inline with the tab name as a subheading. Future phases could present them as selectable tabs in the TUI.

### 3.5 Tag Normalization

GitHub tags use `category>subcategory` format:
- `tutorial>beginner` → Level: "beginner"
- `tutorial>intermediate` → Level: "intermediate"
- `tutorial>advanced` → Level: "advanced"
- Other tags (topic, product, tool) kept as-is for search/filtering

All parsing in `internal/tutorials/parser.go`.

## 4. CLI Commands

New file `cmd/tutorials.go`. Command name: `tutorial` (singular, matching the conceptual model of working with one tutorial at a time).

### 4.1 `tutorial list`

Browse tutorials curated for the active profile.

```
$ sap-devs tutorial list
$ sap-devs tutorial list --all           # ignore profile
$ sap-devs tutorial list --pack cap      # filter by pack
$ sap-devs tutorial list --level beginner
$ sap-devs tutorial list --tags cap,hana

SLUG                              TITLE                                    TIME   LEVEL
★ cap-getting-started             Getting Started with CAP                 30m    Beginner
★ cap-service-deploy-to-cf        Deploy a CAP Application to CF           45m    Intermediate
  hana-cloud-cap-create-project   Create a CAP Project with HANA Cloud     20m    Beginner
```

Stars indicate `featured: true` in `tutorials.yaml`. Data from joining `TutorialRef` (per pack) with `TutorialMeta` (cached index). If index not cached, prompts to run `sap-devs sync`.

### 4.2 `tutorial search`

Search the full index (~1,290 tutorials).

```
$ sap-devs tutorial search fiori
$ sap-devs tutorial search --level advanced
$ sap-devs tutorial search --tags abap,rap
```

Text search matches against title, description, tags, and slug. Same filtering flags as `list`.

### 4.3 `tutorial show <slug>`

Render a tutorial in the terminal.

```
$ sap-devs tutorial show cap-getting-started           # full markdown (default)
$ sap-devs tutorial show cap-getting-started -i         # interactive TUI
$ sap-devs tutorial show cap-getting-started --step 3   # jump to step 3
```

**Default mode:** Full glamour-rendered markdown with metadata header (title, time, level, URL). Steps are numbered and visually separated.

**Interactive mode (`-i`):** Bubbletea TUI showing one step at a time.
- Keybindings: `n`/right = next, `p`/left = prev, `j` = jump to step, `d` = mark done, `q` = quit
- Progress saved on every navigation
- Resumes from last step on re-entry

**Config default:** `sap-devs config set tutorial.interactive true`

### 4.4 `tutorial open <slug>`

Open on developers.sap.com in the default browser.

```
$ sap-devs tutorial open cap-getting-started
# → https://developers.sap.com/tutorials/cap-getting-started.html
```

Deterministic URL from slug. Uses `browser.OpenURL()`.

## 5. Pack Integration

### 5.1 LoadPack Changes

In `LoadPack()` (internal/content/pack.go), add the same pattern as samples/influencers:

```go
if data, err := os.ReadFile(filepath.Join(packDir, "tutorials.yaml")); err == nil {
    _ = yaml.Unmarshal(data, &pack.TutorialRefs)
    for i := range pack.TutorialRefs {
        pack.TutorialRefs[i].PackID = pack.ID
    }
}
```

### 5.2 Content Helper Functions

New file `internal/content/tutorials.go`:

```go
FlattenTutorialRefs(packs []*Pack) []TutorialRef
FilterTutorialRefsByPack(packs []*Pack, packID string) []TutorialRef
FindTutorialRef(packs []*Pack, slug string) *TutorialRef
```

### 5.3 Schema

New JSON schema `content/schemas/tutorials.yaml.schema.json`. Wired into `.vscode/settings.json`.

### 5.4 Initial Content

Seed `tutorials.yaml` for `cap`, `btp-core`, and `abap` packs with 5-10 curated tutorials each.

## 6. Progress Storage

### 6.1 File Location

`{xdg.DataDir}/tutorial-progress.json`
- Linux: `~/.local/share/sap-devs/tutorial-progress.json`
- macOS: `~/Library/Application Support/sap-devs/tutorial-progress.json`
- Windows: `%LOCALAPPDATA%/sap-devs/data/tutorial-progress.json`

### 6.2 API

`internal/tutorials/progress.go`:

```go
LoadProgress(dataDir string) (map[string]TutorialProgress, error)
SaveProgress(dataDir string, progress map[string]TutorialProgress) error
GetProgress(dataDir string, slug string) (*TutorialProgress, error)
UpdateProgress(dataDir string, slug string, currentStep int, markDone bool) error
```

`UpdateProgress` handles:
- Creating new entry if tutorial not started
- Updating `current_step` and `last_accessed` on navigation
- Appending to `completed_steps` when marking a step done
- Setting `completed_at` when all steps are completed

## 7. Package Layout

```
internal/tutorials/
  types.go        # TutorialMeta, Tutorial, TutorialStep
  client.go       # GitHub fetching (index + content)
  parser.go       # Markdown → Tutorial parsing (v1 + v2)
  cache.go        # Index + content cache (load/save/age)
  progress.go     # TutorialProgress load/save/update
  search.go       # Full-text search against index
  enrichment.go   # developers.sap.com API enrichment (optional)

internal/content/
  pack.go         # + TutorialRef struct, Pack.TutorialRefs field
  tutorials.go    # FlattenTutorialRefs, FilterTutorialRefsByPack, FindTutorialRef

cmd/
  tutorials.go    # tutorial list/search/show/open commands

content/packs/cap/tutorials.yaml
content/packs/btp-core/tutorials.yaml
content/packs/abap/tutorials.yaml
content/schemas/tutorials.yaml.schema.json
```

## 8. Sync Integration

In `cmd/sync.go`:
- Add `"tutorials"` to `allCategories()` return value
- Add `tutorialsTTL` to the config struct (default: 7 days / 168h)
- Add `runTutorialsFetch()` function called in the independent-fetch phase
- Import `internal/tutorials` package

## 9. Phase 2 Prep (Not Implemented)

Data model supports future enhancements without structural changes:
- **Inject integration:** `TutorialProgress` has all fields needed for "user is on step 3 of tutorial X" context
- **Guided runner:** `Tutorial.Steps` + `TutorialProgress` provide the state machine for `tutorial run`
- **AI instructor:** `Tutorial` struct carries full step content for agent grounding context
