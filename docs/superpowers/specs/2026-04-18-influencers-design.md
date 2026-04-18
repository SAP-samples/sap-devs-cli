# Influencers Command — Design Spec

**Date:** 2026-04-18
**Status:** Approved
**Project:** sap-devs-cli

---

## Overview

Add a `sap-devs influencers` command to browse SAP community influencers and thought leaders. Influencers are stored as `influencers.yaml` files within content packs, following the same pack-based content pattern as resources. The default view filters by active profile (via pack tag matching); flags provide `--all`, `--pack`, `--tags`, and `--random` modes. An `open` subcommand launches an influencer's link in the browser.

Seed data: SAP Developer Advocates in the `base` pack with focus tags for cross-profile discovery.

---

## Data Model

### YAML Format: `influencers.yaml`

Each pack may contain an `influencers.yaml` file. Each influencer lives in exactly one pack — focus tags handle cross-profile matching (no deduplication needed).

```yaml
- id: dj-adams
  name: DJ Adams
  role: Developer Advocate
  org: SAP
  focus: [cap, fiori, nodejs, community, btp]
  links:
    blog: https://qmacro.org
    github: https://github.com/qmacro
    twitter: https://x.com/qmacro
    youtube: https://youtube.com/@qmacro
```

Required fields: `id`, `name`, `role`, `org`, `focus`, `links`.

- `id` — unique kebab-case identifier
- `name` — display name
- `role` — job title or community role
- `org` — organisation
- `focus` — list of topic tags (matched against pack tags for profile filtering)
- `links` — map of link-type to URL (common types: `blog`, `github`, `twitter`, `youtube`, `linkedin`, `community`)

### Go Struct

Added to `internal/content/pack.go`:

```go
type Influencer struct {
    ID    string            `yaml:"id"`
    Name  string            `yaml:"name"`
    Role  string            `yaml:"role"`
    Org   string            `yaml:"org"`
    Focus []string          `yaml:"focus"`
    Links map[string]string `yaml:"links"`
}
```

`Pack` gains a new field:

```go
type Pack struct {
    // ... existing fields ...
    Influencers []Influencer
}
```

### Content Loading

`LoadPack()` in `internal/content/loader.go` reads `influencers.yaml` alongside `resources.yaml`, using the same pattern: `os.ReadFile` → `yaml.Unmarshal` → assign to `pack.Influencers`. Missing file is not an error (most packs won't have influencers initially).

### JSON Schema

New file `content/schemas/influencers.schema.json` validates the YAML structure. Wired in `.vscode/settings.json` for IDE autocomplete.

---

## Command Structure

### Parent Command: `influencers`

Defaults to list behaviour (same pattern as `resources` and `news`):

```
sap-devs influencers                          # profile-filtered list
sap-devs influencers --all                    # all influencers across all packs
sap-devs influencers --pack cap               # filter to one pack
sap-devs influencers --tags cap,nodejs        # filter by focus tags (OR match)
sap-devs influencers --random                 # one random influencer (after other filters)
```

### Subcommand: `open`

```
sap-devs influencers open dj-adams            # open primary link in browser
sap-devs influencers open dj-adams --link github  # open specific link type
```

Primary link priority: `blog` > first key in `links` map. If `--link <type>` is specified but not found, print available link types and return an error.

### Flag Definitions

| Flag | Short | Type | Default | Description |
| --- | --- | --- | --- | --- |
| `--all` | `-a` | bool | false | Show all influencers regardless of profile |
| `--pack` | `-p` | string | "" | Filter to a specific pack |
| `--tags` | `-t` | string | "" | Comma-separated focus tags (OR match) |
| `--random` | `-r` | bool | false | Show one random influencer |
| `--link` | `-l` | string | "" | Link type to open (on `open` subcommand only) |

---

## Filtering Logic

1. **Default (no flags):** Load packs for active profile → `FlattenInfluencers()` → display all influencers from those profile-scoped packs. The profile filtering happens at the pack-loading level — `LoadPacks(activeProfile, lang)` only returns packs matching the profile.
2. **`--all`:** Load packs with `nil` profile (all packs) → flatten all influencers.
3. **`--pack <name>`:** Load all packs, use `FilterInfluencersByPack()` to extract influencers from the named pack only.
4. **`--tags <csv>`:** After step 1/2/3, apply `FilterInfluencersByTags()` — an influencer matches if any of its focus tags overlap with any of the provided tags (OR semantics).
5. **`--random`:** After all other filtering, pick one random influencer using `time.Now().UnixNano()` seed.

`--pack` implies `--all` (loads all packs to find the named one).

---

## Output Format

Tabwriter table (following the `news` command pattern — `tabwriter` suits the variable-width LINKS column):

```
NAME              ROLE                  ORG   FOCUS                    LINKS
DJ Adams          Developer Advocate    SAP   cap,fiori,nodejs         blog github twitter youtube
Thomas Jung       Developer Advocate    SAP   abap,rap,hana,btp        blog github twitter youtube
```

The LINKS column shows available link type names (keys from the `links` map), space-separated.

When `--random` is used, output a single influencer in a more detailed card format:

```
DJ Adams — Developer Advocate @ SAP
Focus: cap, fiori, nodejs, community, btp
  blog:    https://qmacro.org
  github:  https://github.com/qmacro
  twitter: https://x.com/qmacro
  youtube: https://youtube.com/@qmacro
```

Empty result set prints a friendly message: "No influencers found." (or "No influencers found for pack <name>." / "No influencers match tags <tags>.").

---

## Helper Functions

New file `internal/content/influencers.go`:

```go
// FlattenInfluencers collects all influencers from all packs.
func FlattenInfluencers(packs []*Pack) []Influencer

// FilterInfluencersByTags returns influencers with at least one matching focus tag (OR).
func FilterInfluencersByTags(influencers []Influencer, tags []string) []Influencer

// FilterInfluencersByPack returns influencers from packs matching the given pack ID.
func FilterInfluencersByPack(packs []*Pack, packID string) []Influencer

// FindInfluencer returns the influencer with the given ID, or nil.
func FindInfluencer(influencers []Influencer, id string) *Influencer

// RandomInfluencer returns one random influencer from the slice.
func RandomInfluencer(influencers []Influencer) *Influencer
```

---

## Seed Data

All SAP Developer Advocates go in `content/packs/base/influencers.yaml` with broad focus tags. Initial set:

- DJ Adams — focus: cap, fiori, nodejs, community, btp
- Thomas Jung — focus: abap, rap, hana, btp, cap
- Marius Obert — focus: btp, fiori, ui5, javascript
- Rich Heilman — focus: abap, rap, steampunk
- Riley Rainey — focus: btp, cap, integration
- Ian Thain — focus: abap, btp, integration

Exact links will be researched during implementation.

---

## i18n Keys

Add to `internal/i18n/` catalogs for `en` and `de`:

| Key | English value |
| --- | --- |
| `influencers.short` | `Browse SAP community influencers` |
| `influencers.long` | `Browse SAP community influencers and thought leaders relevant to your active profile.` |
| `influencers.open.short` | `Open an influencer's link in the browser` |
| `influencers.open.long` | `Open an influencer's link in the browser. Defaults to the blog link; use --link to choose another type (github, twitter, youtube, etc.).` |
| `influencers.none` | `No influencers found.` |
| `influencers.none_pack` | `No influencers found for pack "{{.Pack}}".` |
| `influencers.none_tags` | `No influencers match tags {{.Tags}}.` |
| `influencers.link_not_found` | `Influencer "{{.ID}}" has no "{{.Link}}" link. Available: {{.Available}}` |
| `influencers.not_found` | `Influencer "{{.ID}}" not found.` |

---

## Files Changed

| File | Change |
| --- | --- |
| `internal/content/pack.go` | Add `Influencer` struct; add `Influencers []Influencer` to `Pack` |
| `internal/content/loader.go` | Load `influencers.yaml` in `LoadPack()` |
| `internal/content/influencers.go` | New file: `FlattenInfluencers`, `FilterInfluencersByTags`, `FilterInfluencersByPack`, `FindInfluencer`, `RandomInfluencer` |
| `cmd/influencers.go` | New file: `influencersCmd`, `influencersOpenCmd`, flag wiring |
| `cmd/root.go` | Register `influencersCmd` |
| `content/packs/base/influencers.yaml` | Seed data: SAP Developer Advocates |
| `content/schemas/influencers.schema.json` | JSON Schema for `influencers.yaml` validation |
| `.vscode/settings.json` | Wire `influencers.schema.json` for `**/influencers.yaml` |
| `internal/i18n/` | Add i18n keys for `en` and `de` catalogs |
| `CLAUDE.md` | Update CLI commands table with `influencers` entry |

---

## Out of Scope

- Injecting influencer data into AI tool context (could be added later as a section in `context.md` or via MCP server)
- Community-contributed influencers workflow (handled naturally by the user/project content layers)
- Influencer "follow" or bookmarking feature
- Social media integration beyond static links
