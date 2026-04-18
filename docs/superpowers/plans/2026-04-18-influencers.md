# Influencers Command Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `sap-devs influencers` command to browse SAP community influencers from pack-based `influencers.yaml` files, with profile-aware filtering and browser-open support.

**Architecture:** New `Influencer` struct on the `Pack` type, loaded from `influencers.yaml` in `LoadPack()`. Helper functions in `internal/content/influencers.go` follow the `resources.go` pattern. Cobra command in `cmd/influencers.go` uses flags (`--all`, `--pack`, `--tags`, `--random`) on the parent plus an `open` subcommand. Additive-layer merge support in `merge.go`.

**Tech Stack:** Go, Cobra, tabwriter, `github.com/pkg/browser`

**Spec:** `docs/superpowers/specs/2026-04-18-influencers-design.md`

---

## File Map

| File | Action | Responsibility |
| --- | --- | --- |
| `internal/content/pack.go` | Modify | Add `Influencer` struct and `Influencers []Influencer` field to `Pack` |
| `internal/content/pack.go` | Modify | Load `influencers.yaml` in `LoadPack()` |
| `internal/content/merge.go` | Modify | Add `mergeInfluencers()` and call from `MergeWith()` |
| `internal/content/influencers.go` | Create | `FlattenInfluencers`, `FilterInfluencersByTags`, `FilterInfluencersByPack`, `FindInfluencer`, `RandomInfluencer` |
| `cmd/influencers.go` | Create | `influencersCmd` (parent + list), `influencersOpenCmd`, flags |
| `cmd/root.go` | Modify | Register `influencersCmd` in `init()` — no, commands self-register |
| `content/packs/base/influencers.yaml` | Create | Seed data: SAP Developer Advocates |
| `content/schemas/influencers.schema.json` | Create | JSON Schema for YAML validation |
| `.vscode/settings.json` | Modify | Wire schema for `**/packs/*/influencers.yaml` |
| `internal/i18n/catalogs/en.json` | Modify | Add influencers i18n keys |
| `internal/i18n/catalogs/de.json` | Modify | Add influencers i18n keys (German) |
| `CLAUDE.md` | Modify | Add `influencers` to CLI commands table |

---

### Task 1: Data Model — Influencer struct and Pack field

**Files:**
- Modify: `internal/content/pack.go:77-91` (after `HookDef`)

- [ ] **Step 1: Add Influencer struct to pack.go**

Add after the `HookDef` struct (line 91):

```go
// Influencer is a community influencer or thought leader within a pack.
type Influencer struct {
	ID    string            `yaml:"id"`
	Name  string            `yaml:"name"`
	Role  string            `yaml:"role"`
	Org   string            `yaml:"org"`
	Focus []string          `yaml:"focus"`
	Links map[string]string `yaml:"links"`
	PackID string           // set at load time, not in YAML
}
```

- [ ] **Step 2: Add Influencers field to Pack struct**

Add to the `Pack` struct (after the `Hooks` field at line 31):

```go
Influencers []Influencer
```

- [ ] **Step 3: Load influencers.yaml in LoadPack()**

Add to `LoadPack()` after the `hook.yaml` loading block (after line 179), following the exact same pattern as `resources.yaml`:

```go
if data, err := os.ReadFile(filepath.Join(packDir, "influencers.yaml")); err == nil {
	_ = yaml.Unmarshal(data, &pack.Influencers)
	for i := range pack.Influencers {
		pack.Influencers[i].PackID = pack.ID
	}
}
```

- [ ] **Step 4: Add mergeInfluencers to merge.go**

Add `mergeInfluencers()` following the `mergeHooks()` pattern (replace on matching ID, append new, re-stamp PackID):

```go
func mergeInfluencers(base, additive []Influencer, packID string) []Influencer {
	result := make([]Influencer, len(base))
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

Call it in `MergeWith()` after the `merged.Hooks = mergeHooks(...)` line:

```go
merged.Influencers = mergeInfluencers(base.Influencers, a.Influencers, base.ID)
```

- [ ] **Step 5: Verify build**

Run: `go build ./...`
Expected: compiles successfully with new struct and loading code

- [ ] **Step 6: Commit**

```bash
git add internal/content/pack.go internal/content/merge.go
git commit -m "feat(influencers): add Influencer struct, Pack field, and YAML loading"
```

---

### Task 2: Helper Functions — internal/content/influencers.go

**Files:**
- Create: `internal/content/influencers.go`

- [ ] **Step 1: Create influencers.go with all helper functions**

Create `internal/content/influencers.go`:

```go
package content

import (
	"math/rand"
	"strings"
)

// FlattenInfluencers collects all influencers from all packs into a single slice.
func FlattenInfluencers(packs []*Pack) []Influencer {
	var out []Influencer
	for _, p := range packs {
		out = append(out, p.Influencers...)
	}
	return out
}

// FilterInfluencersByTags returns influencers with at least one focus tag
// matching any of the provided tags (OR semantics, case-insensitive).
func FilterInfluencersByTags(influencers []Influencer, tags []string) []Influencer {
	tagSet := make(map[string]bool, len(tags))
	for _, t := range tags {
		tagSet[strings.ToLower(strings.TrimSpace(t))] = true
	}
	var out []Influencer
	for _, inf := range influencers {
		for _, f := range inf.Focus {
			if tagSet[strings.ToLower(f)] {
				out = append(out, inf)
				break
			}
		}
	}
	return out
}

// FilterInfluencersByPack returns influencers from packs matching the given pack ID.
func FilterInfluencersByPack(packs []*Pack, packID string) []Influencer {
	for _, p := range packs {
		if p.ID == packID {
			return p.Influencers
		}
	}
	return nil
}

// FindInfluencer returns a pointer to the first influencer with an exact ID match, or nil.
func FindInfluencer(influencers []Influencer, id string) *Influencer {
	for i := range influencers {
		if influencers[i].ID == id {
			return &influencers[i]
		}
	}
	return nil
}

// RandomInfluencer returns one random influencer from the slice using the given seed.
func RandomInfluencer(influencers []Influencer, seed int64) *Influencer {
	if len(influencers) == 0 {
		return nil
	}
	r := rand.New(rand.NewSource(seed))
	return &influencers[r.Intn(len(influencers))]
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./...`
Expected: compiles successfully

- [ ] **Step 3: Commit**

```bash
git add internal/content/influencers.go
git commit -m "feat(influencers): add helper functions for flatten, filter, find, random"
```

---

### Task 3: i18n Keys — English and German catalogs

**Files:**
- Modify: `internal/i18n/catalogs/en.json`
- Modify: `internal/i18n/catalogs/de.json`

- [ ] **Step 1: Add English i18n keys**

Add the following entries to `en.json` (after the `resources.*` block, before `version.*`):

```json
"influencers.short": "Browse SAP community influencers",
"influencers.long": "Browse SAP community influencers and thought leaders relevant to your active profile.",
"influencers.no_profile": "no profile set — run 'sap-devs profile set <name>' first",
"influencers.profile_not_found": "profile \"{{.ID}}\" not found — run 'sap-devs sync' to refresh content",
"influencers.none": "No influencers found.",
"influencers.none_pack": "No influencers found for pack \"{{.Pack}}\".",
"influencers.none_tags": "No influencers match tags {{.Tags}}.",
"influencers.not_found": "Influencer \"{{.ID}}\" not found.",
"influencers.link_not_found": "Influencer \"{{.ID}}\" has no \"{{.Link}}\" link. Available: {{.Available}}",
"influencers.open.short": "Open an influencer's link in the browser",
"influencers.open.long": "Open an influencer's link in the browser. Defaults to the blog link; use --link to choose another type (github, twitter, youtube, etc.).",
"influencers.open.browser_fail": "Could not open browser: {{.Err}}. URL: {{.URL}}",
"influencers.open.opening": "Opening: {{.Name}} — {{.URL}}",
"influencers.col_name": "NAME",
"influencers.col_role": "ROLE",
"influencers.col_org": "ORG",
"influencers.col_focus": "FOCUS",
"influencers.col_links": "LINKS",
```

- [ ] **Step 2: Add German i18n keys**

Add the corresponding entries to `de.json`:

```json
"influencers.short": "SAP-Community-Influencer durchsuchen",
"influencers.long": "SAP-Community-Influencer und Vordenker passend zu deinem aktiven Profil durchsuchen.",
"influencers.no_profile": "Kein Profil gesetzt — 'sap-devs profile set <name>' zuerst ausführen",
"influencers.profile_not_found": "Profil \"{{.ID}}\" nicht gefunden — 'sap-devs sync' ausführen, um Inhalte zu aktualisieren",
"influencers.none": "Keine Influencer gefunden.",
"influencers.none_pack": "Keine Influencer für Pack \"{{.Pack}}\" gefunden.",
"influencers.none_tags": "Keine Influencer für Tags {{.Tags}} gefunden.",
"influencers.not_found": "Influencer \"{{.ID}}\" nicht gefunden.",
"influencers.link_not_found": "Influencer \"{{.ID}}\" hat keinen \"{{.Link}}\"-Link. Verfügbar: {{.Available}}",
"influencers.open.short": "Link eines Influencers im Browser öffnen",
"influencers.open.long": "Link eines Influencers im Browser öffnen. Standardmäßig der Blog-Link; mit --link einen anderen Typ wählen (github, twitter, youtube, etc.).",
"influencers.open.browser_fail": "Browser konnte nicht geöffnet werden: {{.Err}}. URL: {{.URL}}",
"influencers.open.opening": "Öffne: {{.Name}} — {{.URL}}",
"influencers.col_name": "NAME",
"influencers.col_role": "ROLLE",
"influencers.col_org": "ORG",
"influencers.col_focus": "FOKUS",
"influencers.col_links": "LINKS",
```

- [ ] **Step 3: Verify build**

Run: `go build ./...`
Expected: compiles (embedded JSON catalogs are valid)

- [ ] **Step 4: Commit**

```bash
git add internal/i18n/catalogs/en.json internal/i18n/catalogs/de.json
git commit -m "feat(influencers): add i18n keys for influencers command (en, de)"
```

---

### Task 4: Command Implementation — cmd/influencers.go

**Files:**
- Create: `cmd/influencers.go`

- [ ] **Step 1: Create cmd/influencers.go**

```go
package cmd

import (
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/config"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/i18n"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/xdg"
)

var (
	influencersAll    bool
	influencersPack   string
	influencersTags   string
	influencersRandom bool
	influencersLink   string
)

var influencersCmd = &cobra.Command{
	Use:   "influencers",
	Short: "Browse SAP community influencers",
	RunE: func(cmd *cobra.Command, args []string) error {
		loader, err := newContentLoader()
		if err != nil {
			return err
		}

		var packs []*content.Pack

		if influencersPack != "" || influencersAll {
			// --pack or --all: load all packs
			packs, err = loader.LoadPacks(nil, i18n.ActiveLang)
			if err != nil {
				return err
			}
		} else {
			// Default: profile-scoped
			paths, err := xdg.New()
			if err != nil {
				return err
			}
			profileCfg, err := config.LoadProfile(paths.ConfigDir)
			if err != nil {
				return err
			}
			if profileCfg.ID == "" {
				return fmt.Errorf("%s", i18n.T(i18n.ActiveLang, "influencers.no_profile"))
			}
			activeProfile, err := loader.FindProfile(profileCfg.ID)
			if err != nil {
				return err
			}
			if activeProfile == nil {
				return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "influencers.profile_not_found", map[string]any{"ID": profileCfg.ID}))
			}
			packs, err = loader.LoadPacks(activeProfile, i18n.ActiveLang)
			if err != nil {
				return err
			}
		}

		var influencers []content.Influencer
		if influencersPack != "" {
			influencers = content.FilterInfluencersByPack(packs, influencersPack)
		} else {
			influencers = content.FlattenInfluencers(packs)
		}

		if influencersTags != "" {
			tags := strings.Split(influencersTags, ",")
			influencers = content.FilterInfluencersByTags(influencers, tags)
		}

		if len(influencers) == 0 {
			if influencersPack != "" {
				fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "influencers.none_pack", map[string]any{"Pack": influencersPack}))
			} else if influencersTags != "" {
				fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "influencers.none_tags", map[string]any{"Tags": influencersTags}))
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "influencers.none"))
			}
			return nil
		}

		if influencersRandom {
			inf := content.RandomInfluencer(influencers, time.Now().UnixNano())
			if inf == nil {
				fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "influencers.none"))
				return nil
			}
			printInfluencerCard(cmd, inf)
			return nil
		}

		printInfluencerTable(cmd, influencers)
		return nil
	},
}

var influencersOpenCmd = &cobra.Command{
	Use:   "open <id>",
	Short: "Open an influencer's link in the browser",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		loader, err := newContentLoader()
		if err != nil {
			return err
		}
		packs, err := loader.LoadPacks(nil, i18n.ActiveLang)
		if err != nil {
			return err
		}
		inf := content.FindInfluencer(content.FlattenInfluencers(packs), args[0])
		if inf == nil {
			return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "influencers.not_found", map[string]any{"ID": args[0]}))
		}

		linkType := influencersLink
		var url string
		if linkType != "" {
			url = inf.Links[linkType]
			if url == "" {
				available := sortedLinkTypes(inf.Links)
				return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "influencers.link_not_found", map[string]any{
					"ID":        inf.ID,
					"Link":      linkType,
					"Available": strings.Join(available, ", "),
				}))
			}
		} else {
			url = primaryLink(inf.Links)
		}

		if err := browser.OpenURL(url); err != nil {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "influencers.open.browser_fail", map[string]any{"Err": err, "URL": url}))
			return nil
		}
		fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "influencers.open.opening", map[string]any{"Name": inf.Name, "URL": url}))
		return nil
	},
}

func printInfluencerTable(cmd *cobra.Command, influencers []content.Influencer) {
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
		i18n.T(i18n.ActiveLang, "influencers.col_name"),
		i18n.T(i18n.ActiveLang, "influencers.col_role"),
		i18n.T(i18n.ActiveLang, "influencers.col_org"),
		i18n.T(i18n.ActiveLang, "influencers.col_focus"),
		i18n.T(i18n.ActiveLang, "influencers.col_links"),
	)
	for _, inf := range influencers {
		focus := strings.Join(inf.Focus, ",")
		links := strings.Join(sortedLinkTypes(inf.Links), " ")
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", inf.Name, inf.Role, inf.Org, focus, links)
	}
	w.Flush()
}

func printInfluencerCard(cmd *cobra.Command, inf *content.Influencer) {
	fmt.Fprintf(cmd.OutOrStdout(), "%s — %s @ %s\n", inf.Name, inf.Role, inf.Org)
	fmt.Fprintf(cmd.OutOrStdout(), "Focus: %s\n", strings.Join(inf.Focus, ", "))
	for _, k := range sortedLinkTypes(inf.Links) {
		fmt.Fprintf(cmd.OutOrStdout(), "  %-10s %s\n", k+":", inf.Links[k])
	}
}

// primaryLink returns the blog URL if present, otherwise the first sorted link.
func primaryLink(links map[string]string) string {
	if u, ok := links["blog"]; ok {
		return u
	}
	for _, k := range sortedLinkTypes(links) {
		return links[k]
	}
	return ""
}

// sortedLinkTypes returns link type keys sorted alphabetically.
func sortedLinkTypes(links map[string]string) []string {
	keys := make([]string, 0, len(links))
	for k := range links {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func init() {
	influencersCmd.Flags().BoolVarP(&influencersAll, "all", "a", false, "show all influencers regardless of profile")
	influencersCmd.Flags().StringVarP(&influencersPack, "pack", "p", "", "filter to a specific pack")
	influencersCmd.Flags().StringVarP(&influencersTags, "tags", "t", "", "comma-separated focus tags (OR match)")
	influencersCmd.Flags().BoolVarP(&influencersRandom, "random", "r", false, "show one random influencer")
	influencersOpenCmd.Flags().StringVarP(&influencersLink, "link", "l", "", "link type to open (blog, github, twitter, etc.)")
	influencersCmd.AddCommand(influencersOpenCmd)
	rootCmd.AddCommand(influencersCmd)
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./...`
Expected: compiles successfully

- [ ] **Step 3: Commit**

```bash
git add cmd/influencers.go
git commit -m "feat(influencers): add influencers command with list, open, and filtering flags"
```

---

### Task 5: Seed Data and Schema

**Files:**
- Create: `content/packs/base/influencers.yaml`
- Create: `content/schemas/influencers.schema.json`
- Modify: `.vscode/settings.json`

- [ ] **Step 1: Create seed data in content/packs/base/influencers.yaml**

Research exact links for each advocate, then create:

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
    community: https://community.sap.com/t5/user/viewprofilepage/user-id/53

- id: thomas-jung
  name: Thomas Jung
  role: Developer Advocate
  org: SAP
  focus: [abap, rap, hana, btp, cap]
  links:
    blog: https://community.sap.com/t5/user/viewprofilepage/user-id/139
    github: https://github.com/jung-thomas
    twitter: https://x.com/thomas_jung
    youtube: https://youtube.com/@sapdevs
    linkedin: https://linkedin.com/in/thomasjungsap

- id: marius-obert
  name: Marius Obert
  role: Developer Advocate
  org: SAP
  focus: [btp, fiori, ui5, javascript]
  links:
    blog: https://community.sap.com/t5/user/viewprofilepage/user-id/381
    github: https://github.com/IObert
    twitter: https://x.com/IObert_
    linkedin: https://linkedin.com/in/marius-obert

- id: rich-heilman
  name: Rich Heilman
  role: Developer Advocate
  org: SAP
  focus: [abap, rap, steampunk, btp]
  links:
    blog: https://community.sap.com/t5/user/viewprofilepage/user-id/151
    github: https://github.com/rich-heilman
    twitter: https://x.com/nickel_chrome
    youtube: https://youtube.com/@sapdevs

- id: riley-rainey
  name: Riley Rainey
  role: Developer Advocate
  org: SAP
  focus: [btp, cap, integration, ai]
  links:
    blog: https://community.sap.com/t5/user/viewprofilepage/user-id/138734
    github: https://github.com/rrainey
    linkedin: https://linkedin.com/in/rileyrainey

- id: ian-thain
  name: Ian Thain
  role: Developer Advocate
  org: SAP
  focus: [abap, btp, integration]
  links:
    blog: https://community.sap.com/t5/user/viewprofilepage/user-id/225
    twitter: https://x.com/ithain
    linkedin: https://linkedin.com/in/ian-thain
```

Note: The implementer should verify all URLs are correct and update any that have changed.

- [ ] **Step 2: Create content/schemas/influencers.schema.json**

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "Pack Influencers",
  "description": "Schema for sap-devs influencers.yaml files (top-level array)",
  "type": "array",
  "items": {
    "type": "object",
    "required": ["id", "name", "role", "org", "focus", "links"],
    "additionalProperties": false,
    "properties": {
      "id": {
        "type": "string",
        "pattern": "^[a-z0-9]+(-[a-z0-9]+)*$",
        "description": "Unique kebab-case identifier for the influencer"
      },
      "name": { "type": "string", "description": "Display name" },
      "role": { "type": "string", "description": "Job title or community role" },
      "org": { "type": "string", "description": "Organisation" },
      "focus": {
        "type": "array",
        "items": { "type": "string" },
        "minItems": 1,
        "description": "Topic tags for profile-based discovery"
      },
      "links": {
        "type": "object",
        "additionalProperties": {
          "type": "string",
          "format": "uri"
        },
        "minProperties": 1,
        "description": "Map of link-type to URL (e.g. blog, github, twitter)"
      }
    }
  }
}
```

- [ ] **Step 3: Wire schema in .vscode/settings.json**

Add to the `yaml.schemas` object:

```json
"./content/schemas/influencers.schema.json": "**/packs/*/influencers.yaml"
```

- [ ] **Step 4: Verify build and YAML loads**

Run: `SAP_DEVS_DEV=1 go run . influencers --all`
Expected: lists the seed influencers from the base pack

- [ ] **Step 5: Commit**

```bash
git add content/packs/base/influencers.yaml content/schemas/influencers.schema.json .vscode/settings.json
git commit -m "feat(influencers): add seed data, JSON schema, and VS Code wiring"
```

---

### Task 6: Smoke Test and Documentation

**Files:**
- Modify: `CLAUDE.md` (CLI commands table)

- [ ] **Step 1: Smoke test all modes**

Run each command and verify output:

```bash
SAP_DEVS_DEV=1 go run . influencers
SAP_DEVS_DEV=1 go run . influencers --all
SAP_DEVS_DEV=1 go run . influencers --pack base
SAP_DEVS_DEV=1 go run . influencers --tags cap,abap
SAP_DEVS_DEV=1 go run . influencers --random
SAP_DEVS_DEV=1 go run . influencers --pack nonexistent
SAP_DEVS_DEV=1 go run . influencers open dj-adams --link github
SAP_DEVS_DEV=1 go run . influencers open nonexistent
```

Expected:
- Default list: tabwriter table with NAME/ROLE/ORG/FOCUS/LINKS columns
- `--all`: shows all seed influencers
- `--pack base`: same as --all for seed data
- `--tags cap,abap`: filters to influencers with cap or abap in focus
- `--random`: card format with one influencer
- `--pack nonexistent`: "No influencers found for pack" message
- `open dj-adams --link github`: opens browser
- `open nonexistent`: error message

- [ ] **Step 2: Run go vet**

Run: `go vet ./...`
Expected: no issues

- [ ] **Step 3: Update CLAUDE.md CLI commands table**

Add between `hook` and `inject` in the CLI Commands table:

```markdown
| `influencers` | Browse SAP community influencers and thought leaders |
```

- [ ] **Step 4: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: add influencers command to CLI commands table"
```

---

### Task 7: Final Verification

- [ ] **Step 1: Full build check**

Run: `go build -o sap-devs . && go vet ./...`
Expected: clean build, no vet warnings

- [ ] **Step 2: Verify help output**

Run: `./sap-devs influencers --help`
Expected: shows command description, flags (--all, --pack, --tags, --random), and open subcommand

Run: `./sap-devs influencers open --help`
Expected: shows open description with --link flag
