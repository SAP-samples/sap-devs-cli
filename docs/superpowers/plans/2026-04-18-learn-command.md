# Learn Command Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build `sap-devs learn` — an umbrella command aggregating learning journeys, tutorials, and Discovery Center missions into unified recommendations, cross-type search, and curated learning paths.

**Architecture:** Full internal package (`internal/learn/`) with types, recommendation engine, cross-type search, and path resolution. Command layer (`cmd/learn*.go`) follows existing cobra patterns. Content layer extended with `paths.yaml` per pack and `ExperienceLevel` config field.

**Tech Stack:** Go, cobra, tabwriter, glamour, pkg/browser, gopkg.in/yaml.v3

**Spec:** `docs/superpowers/specs/2026-04-18-learn-command-design.md`

---

## File Structure

### New files
| File | Responsibility |
|------|---------------|
| `internal/learn/types.go` | `LearnItem`, `LearningPath`, `PathStep`, `Recommendations`, `RecommendOptions`, `ItemType` constants |
| `internal/learn/recommend.go` | `Recommend()` — three-tier resolution per content type, level normalization, filtering |
| `internal/learn/recommend_test.go` | Unit tests for `Recommend()` |
| `internal/learn/search.go` | `Search()` — cross-type substring search with title-priority ranking |
| `internal/learn/search_test.go` | Unit tests for `Search()` |
| `internal/learn/paths.go` | `LoadPaths()`, `AutoFillPaths()`, `ResolvePaths()` |
| `internal/learn/paths_test.go` | Unit tests for path loading, auto-fill, resolution |
| `internal/content/paths.go` | `FlattenLearningPaths()` helper, `LearningPathDef`/`LearningPathStepDef` types |
| `cmd/learn.go` | Parent command + `recommend` subcommand + shared flags + `--level` validation |
| `cmd/learn_search.go` | `learn search <query>` subcommand |
| `cmd/learn_path.go` | `learn path` parent + `list`, `show`, `open` subcommands |
| `content/packs/cap/paths.yaml` | Seed curated learning paths for CAP pack |
| `content/schemas/paths.yaml.schema.json` | JSON Schema for `paths.yaml` validation |

### Modified files
| File | Change |
|------|--------|
| `internal/config/config.go` | Add `ExperienceLevel string` field to `Config` struct |
| `internal/content/pack.go` | Add `LearningPaths []LearningPathDef` to `Pack`, load `paths.yaml` in `LoadPack()` |
| `internal/content/merge.go` | Add `mergeLearningPaths()` call in `MergeWith()` |
| `internal/i18n/catalogs/en.json` | Add `learn.*` i18n keys |
| `internal/i18n/catalogs/de.json` | Add `learn.*` i18n keys (mirror English) |
| `.vscode/settings.json` | Register paths.schema.json for `**/packs/*/paths.yaml` |
| `CLAUDE.md` | Add learn command to CLI commands table |

---

## Task 1: Config — Add ExperienceLevel

**Files:**
- Modify: `internal/config/config.go`

- [ ] **Step 1: Add ExperienceLevel field to Config struct**

In `internal/config/config.go`, add the field after `Tutorial`:

```go
type Config struct {
	CompanyRepo string         `yaml:"company_repo,omitempty"`
	Language    string         `yaml:"language,omitempty"`
	Location    string         `yaml:"location,omitempty"`
	Sync        SyncConfig     `yaml:"sync"`
	Tip         TipConfig      `yaml:"tip,omitempty"`
	Events      EventsConfig   `yaml:"events,omitempty"`
	Tutorial    TutorialConfig `yaml:"tutorial,omitempty"`
	ExperienceLevel string     `yaml:"experience_level,omitempty"`
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./...`
Expected: clean build, no errors

- [ ] **Step 3: Commit**

```bash
git add internal/config/config.go
git commit -m "feat(learn): add ExperienceLevel to config"
```

---

## Task 2: Content Layer — LearningPathDef types and pack loading

**Files:**
- Create: `internal/content/paths.go`
- Modify: `internal/content/pack.go`
- Modify: `internal/content/merge.go`

- [ ] **Step 1: Create internal/content/paths.go with types and flatten helper**

```go
package content

// LearningPathDef is the YAML-parsed shape from paths.yaml.
type LearningPathDef struct {
	ID          string                `yaml:"id"`
	Name        string                `yaml:"name"`
	Description string                `yaml:"description,omitempty"`
	Level       string                `yaml:"level,omitempty"`
	Steps       []LearningPathStepDef `yaml:"steps"`
	PackID      string                // set at load time
}

// LearningPathStepDef is a single step in a curated path.
type LearningPathStepDef struct {
	Type string `yaml:"type"`
	Slug string `yaml:"slug"`
}

// FlattenLearningPaths collects all curated learning paths from all packs.
func FlattenLearningPaths(packs []*Pack) []LearningPathDef {
	var out []LearningPathDef
	for _, p := range packs {
		out = append(out, p.LearningPaths...)
	}
	return out
}
```

- [ ] **Step 2: Add LearningPaths field to Pack struct in pack.go**

In `internal/content/pack.go`, add after the `LearningForInject` field:

```go
LearningPaths []LearningPathDef
```

- [ ] **Step 3: Add paths.yaml loading in LoadPack()**

In `internal/content/pack.go` `LoadPack()`, add after the `learning.yaml` loading block (after line ~422):

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

- [ ] **Step 4: Add mergeLearningPaths to merge.go**

In `internal/content/merge.go`, add the merge function:

```go
// mergeLearningPaths builds a fresh []LearningPathDef: starts with base entries,
// replaces any entry whose ID matches an additive entry, appends unmatched additive entries.
// PackID is re-stamped to packID on every entry in the result.
func mergeLearningPaths(base, additive []LearningPathDef, packID string) []LearningPathDef {
	result := make([]LearningPathDef, len(base))
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

Then add the call in `MergeWith()`, after the `LearningRefs` merge line:

```go
merged.LearningPaths = mergeLearningPaths(base.LearningPaths, a.LearningPaths, base.ID)
```

- [ ] **Step 5: Verify build**

Run: `go build ./...`
Expected: clean build

- [ ] **Step 6: Verify vet**

Run: `go vet ./...`
Expected: no issues

- [ ] **Step 7: Commit**

```bash
git add internal/content/paths.go internal/content/pack.go internal/content/merge.go
git commit -m "feat(learn): add LearningPathDef types, pack loading, and merge support"
```

---

## Task 3: Internal Package — Types

**Files:**
- Create: `internal/learn/types.go`

- [ ] **Step 1: Create internal/learn/types.go**

```go
package learn

// ItemType identifies the source content type for a learn item.
type ItemType string

const (
	ItemJourney  ItemType = "journey"
	ItemTutorial ItemType = "tutorial"
	ItemMission  ItemType = "mission"
)

// LearnItem is a unified wrapper around content from any source.
type LearnItem struct {
	Type     ItemType
	Title    string
	Slug     string
	Level    string
	Duration string
	URL      string
	Featured bool
	PackID   string
	Product  string
}

// Recommendations holds profile-filtered items grouped by type.
type Recommendations struct {
	Journeys  []LearnItem
	Tutorials []LearnItem
	Missions  []LearnItem
}

// RecommendOptions controls filtering for recommendations and search.
type RecommendOptions struct {
	Level  string
	PackID string
	All    bool
	Limit  int
}

// LearningPath is a named, ordered sequence of learn items.
type LearningPath struct {
	ID          string
	Name        string
	Description string
	Level       string
	PackID      string
	Steps       []PathStep
	Generated   bool
}

// PathStep is a single step in a learning path.
type PathStep struct {
	Type ItemType
	Slug string
	Item *LearnItem
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./...`
Expected: clean build

- [ ] **Step 3: Commit**

```bash
git add internal/learn/types.go
git commit -m "feat(learn): add core types for learn package"
```

---

## Task 4: Internal Package — Recommend

**Files:**
- Create: `internal/learn/recommend.go`
- Create: `internal/learn/recommend_test.go`

- [ ] **Step 1: Write the failing test for Recommend()**

Create `internal/learn/recommend_test.go`:

```go
package learn

import (
	"fmt"
	"testing"

	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/discovery"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/learning"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/tutorials"
)

func TestRecommend_FeaturedFirst(t *testing.T) {
	journeys := []learning.LearningJourney{
		{Slug: "j1", Title: "Journey One", Level: "BEGINNER", DurationHours: "4", Product: "SAP BTP"},
		{Slug: "j2", Title: "Journey Two", Level: "INTERMEDIATE", DurationHours: "8", Product: "SAP BTP"},
	}
	tuts := []tutorials.TutorialMeta{
		{Slug: "t1", Title: "Tutorial One", Level: "beginner", Time: 45},
		{Slug: "t2", Title: "Tutorial Two", Level: "advanced", Time: 60},
	}
	missions := []discovery.Mission{
		{ID: 100, Name: "Mission One", Effort: "1", Product: "SAP BTP"},
	}
	packs := []*content.Pack{
		{
			ID: "cap",
			LearningRefs:    []content.LearningRef{{Slug: "j1", Featured: true, PackID: "cap"}},
			TutorialRefs:    []content.TutorialRef{{Slug: "t1", Featured: true, PackID: "cap"}},
			DiscoveryMissions: []content.DiscoveryMissionRef{{ID: 100, Featured: true, PackID: "cap"}},
			LearningFilters: &content.LearningProfileFilters{Products: []string{"SAP BTP"}},
		},
	}

	recs := Recommend(journeys, tuts, missions, packs, RecommendOptions{Limit: 10, All: true})

	if len(recs.Journeys) != 2 {
		t.Fatalf("expected 2 journeys, got %d", len(recs.Journeys))
	}
	if recs.Journeys[0].Slug != "j1" || !recs.Journeys[0].Featured {
		t.Errorf("expected first journey to be featured j1, got %s (featured=%v)", recs.Journeys[0].Slug, recs.Journeys[0].Featured)
	}
	if recs.Journeys[0].Level != "beginner" {
		t.Errorf("expected normalized level 'beginner', got %q", recs.Journeys[0].Level)
	}

	if len(recs.Tutorials) != 2 {
		t.Fatalf("expected 2 tutorials, got %d", len(recs.Tutorials))
	}
	if !recs.Tutorials[0].Featured {
		t.Errorf("expected first tutorial to be featured")
	}

	if len(recs.Missions) != 1 {
		t.Fatalf("expected 1 mission, got %d", len(recs.Missions))
	}
	if recs.Missions[0].Level != "beginner" {
		t.Errorf("expected mission effort 1 → beginner, got %q", recs.Missions[0].Level)
	}
}

func TestRecommend_LevelFilter(t *testing.T) {
	journeys := []learning.LearningJourney{
		{Slug: "j1", Title: "Journey One", Level: "BEGINNER", Product: "SAP BTP"},
		{Slug: "j2", Title: "Journey Two", Level: "ADVANCED", Product: "SAP BTP"},
	}
	packs := []*content.Pack{{ID: "cap"}}

	recs := Recommend(journeys, nil, nil, packs, RecommendOptions{Level: "beginner", Limit: 10, All: true})

	if len(recs.Journeys) != 1 {
		t.Fatalf("expected 1 journey after level filter, got %d", len(recs.Journeys))
	}
	if recs.Journeys[0].Slug != "j1" {
		t.Errorf("expected j1, got %s", recs.Journeys[0].Slug)
	}
}

func TestRecommend_Limit(t *testing.T) {
	journeys := make([]learning.LearningJourney, 20)
	for i := range journeys {
		journeys[i] = learning.LearningJourney{Slug: fmt.Sprintf("j%d", i), Title: "J", Level: "BEGINNER", Product: "X"}
	}
	packs := []*content.Pack{{ID: "cap"}}

	recs := Recommend(journeys, nil, nil, packs, RecommendOptions{Limit: 5, All: true})

	if len(recs.Journeys) != 5 {
		t.Fatalf("expected 5 journeys after limit, got %d", len(recs.Journeys))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/learn/...`
Expected: FAIL — `Recommend` not defined

- [ ] **Step 3: Implement Recommend()**

Create `internal/learn/recommend.go`:

```go
package learn

import (
	"fmt"
	"strings"

	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/discovery"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/learning"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/tutorials"
)

func Recommend(
	journeys []learning.LearningJourney,
	tuts []tutorials.TutorialMeta,
	missions []discovery.Mission,
	packs []*content.Pack,
	opts RecommendOptions,
) *Recommendations {
	recs := &Recommendations{
		Journeys:  recommendJourneys(journeys, packs, opts),
		Tutorials: recommendTutorials(tuts, packs, opts),
		Missions:  recommendMissions(missions, packs, opts),
	}
	return recs
}

func recommendJourneys(journeys []learning.LearningJourney, packs []*content.Pack, opts RecommendOptions) []LearnItem {
	refs := content.FlattenLearningRefs(packs)
	filters := content.LearningProfileFilters{}
	if !opts.All {
		filters = content.CollectLearningFilters(packs)
	}

	bySlug := make(map[string]learning.LearningJourney, len(journeys))
	for _, j := range journeys {
		bySlug[j.Slug] = j
	}

	var items []LearnItem
	seen := make(map[string]bool)

	// Tier 1: featured refs
	for _, ref := range refs {
		if !ref.Featured {
			continue
		}
		if opts.PackID != "" && ref.PackID != opts.PackID {
			continue
		}
		if j, ok := bySlug[ref.Slug]; ok && !seen[ref.Slug] {
			items = append(items, journeyToItem(j, true, ref.PackID))
			seen[ref.Slug] = true
		}
	}

	// Tier 2: non-featured pack refs
	for _, ref := range refs {
		if ref.Featured || seen[ref.Slug] {
			continue
		}
		if opts.PackID != "" && ref.PackID != opts.PackID {
			continue
		}
		if j, ok := bySlug[ref.Slug]; ok {
			items = append(items, journeyToItem(j, false, ref.PackID))
			seen[ref.Slug] = true
		}
	}

	// Tier 3: profile-filtered or all
	for _, j := range journeys {
		if seen[j.Slug] {
			continue
		}
		if opts.All || content.MatchesLearningFilters(j.Product, j.ProductCategory, j.Roles, filters) {
			items = append(items, journeyToItem(j, false, ""))
			seen[j.Slug] = true
		}
	}

	items = filterByLevel(items, opts.Level)
	return capItems(items, opts.Limit)
}

func recommendTutorials(tuts []tutorials.TutorialMeta, packs []*content.Pack, opts RecommendOptions) []LearnItem {
	refs := content.FlattenTutorialRefs(packs)

	bySlug := make(map[string]tutorials.TutorialMeta, len(tuts))
	for _, t := range tuts {
		bySlug[t.Slug] = t
	}

	var items []LearnItem
	seen := make(map[string]bool)

	// Tier 1: featured refs
	for _, ref := range refs {
		if !ref.Featured {
			continue
		}
		if opts.PackID != "" && ref.PackID != opts.PackID {
			continue
		}
		if t, ok := bySlug[ref.Slug]; ok && !seen[ref.Slug] {
			items = append(items, tutorialToItem(t, true, ref.PackID))
			seen[ref.Slug] = true
		}
	}

	// Tier 2: non-featured pack refs
	for _, ref := range refs {
		if ref.Featured || seen[ref.Slug] {
			continue
		}
		if opts.PackID != "" && ref.PackID != opts.PackID {
			continue
		}
		if t, ok := bySlug[ref.Slug]; ok {
			items = append(items, tutorialToItem(t, false, ref.PackID))
			seen[ref.Slug] = true
		}
	}

	// Tier 3: all remaining (tutorials don't have profile filters, so show all if --all or pack-scoped)
	if opts.All {
		for _, t := range tuts {
			if !seen[t.Slug] {
				items = append(items, tutorialToItem(t, false, ""))
				seen[t.Slug] = true
			}
		}
	}

	items = filterByLevel(items, opts.Level)
	return capItems(items, opts.Limit)
}

func recommendMissions(missions []discovery.Mission, packs []*content.Pack, opts RecommendOptions) []LearnItem {
	refs := content.FlattenDiscoveryMissionRefs(packs)

	byID := make(map[int]discovery.Mission, len(missions))
	for _, m := range missions {
		byID[m.ID] = m
	}

	var items []LearnItem
	seen := make(map[int]bool)

	// Tier 1: featured refs
	for _, ref := range refs {
		if !ref.Featured {
			continue
		}
		if opts.PackID != "" && ref.PackID != opts.PackID {
			continue
		}
		if m, ok := byID[ref.ID]; ok && !seen[ref.ID] {
			items = append(items, missionToItem(m, true, ref.PackID))
			seen[ref.ID] = true
		}
	}

	// Tier 2: non-featured pack refs
	for _, ref := range refs {
		if ref.Featured || seen[ref.ID] {
			continue
		}
		if opts.PackID != "" && ref.PackID != opts.PackID {
			continue
		}
		if m, ok := byID[ref.ID]; ok {
			items = append(items, missionToItem(m, false, ref.PackID))
			seen[ref.ID] = true
		}
	}

	// Tier 3: all remaining if --all
	if opts.All {
		for _, m := range missions {
			if !seen[m.ID] {
				items = append(items, missionToItem(m, false, ""))
				seen[m.ID] = true
			}
		}
	}

	items = filterByLevel(items, opts.Level)
	return capItems(items, opts.Limit)
}

func journeyToItem(j learning.LearningJourney, featured bool, packID string) LearnItem {
	return LearnItem{
		Type:     ItemJourney,
		Title:    j.Title,
		Slug:     j.Slug,
		Level:    NormalizeLevel(j.Level),
		Duration: formatJourneyDuration(j.DurationHours),
		URL:      j.URL,
		Featured: featured,
		PackID:   packID,
		Product:  j.Product,
	}
}

func tutorialToItem(t tutorials.TutorialMeta, featured bool, packID string) LearnItem {
	return LearnItem{
		Type:     ItemTutorial,
		Title:    t.Title,
		Slug:     t.Slug,
		Level:    NormalizeLevel(t.Level),
		Duration: formatTutorialDuration(t.Time),
		URL:      t.URL,
		Featured: featured,
		PackID:   packID,
	}
}

func missionToItem(m discovery.Mission, featured bool, packID string) LearnItem {
	return LearnItem{
		Type:     ItemMission,
		Title:    m.Name,
		Slug:     fmt.Sprintf("%d", m.ID),
		Level:    effortToLevel(m.Effort),
		Duration: "",
		URL:      fmt.Sprintf("https://discovery-center.cloud.sap/missiondetail/%d/", m.ID),
		Featured: featured,
		PackID:   packID,
		Product:  m.Product,
	}
}

// NormalizeLevel converts any level string to lowercase canonical form.
func NormalizeLevel(level string) string {
	return strings.ToLower(strings.TrimSpace(level))
}

func effortToLevel(effort string) string {
	switch effort {
	case "0", "1":
		return "beginner"
	case "2":
		return "intermediate"
	case "3":
		return "advanced"
	default:
		return ""
	}
}

func formatJourneyDuration(hours string) string {
	if hours == "" {
		return ""
	}
	return hours + " hr"
}

func formatTutorialDuration(minutes int) string {
	if minutes <= 0 {
		return ""
	}
	return fmt.Sprintf("%d min", minutes)
}

func filterByLevel(items []LearnItem, level string) []LearnItem {
	if level == "" {
		return items
	}
	norm := NormalizeLevel(level)
	var out []LearnItem
	for _, item := range items {
		if item.Level == norm {
			out = append(out, item)
		}
	}
	return out
}

func capItems(items []LearnItem, limit int) []LearnItem {
	if limit <= 0 || limit >= len(items) {
		return items
	}
	return items[:limit]
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/learn/...`
Expected: PASS (all 3 tests)

- [ ] **Step 5: Verify vet**

Run: `go vet ./internal/learn/...`
Expected: no issues

- [ ] **Step 6: Commit**

```bash
git add internal/learn/recommend.go internal/learn/recommend_test.go
git commit -m "feat(learn): implement Recommend() with three-tier resolution"
```

---

## Task 5: Internal Package — Search

**Files:**
- Create: `internal/learn/search.go`
- Create: `internal/learn/search_test.go`

- [ ] **Step 1: Write the failing test for Search()**

Create `internal/learn/search_test.go`:

```go
package learn

import (
	"testing"

	"github.tools.sap/developer-relations/sap-devs-cli/internal/discovery"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/learning"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/tutorials"
)

func TestSearch_TitleMatchFirst(t *testing.T) {
	journeys := []learning.LearningJourney{
		{Slug: "j1", Title: "Getting Started with CAP", Level: "BEGINNER", Product: "SAP BTP"},
		{Slug: "j2", Title: "Advanced Integration", Level: "ADVANCED", Description: "Uses CAP framework", Product: "SAP BTP"},
	}
	tuts := []tutorials.TutorialMeta{
		{Slug: "t1", Title: "CAP Tutorial Basics", Level: "beginner", Time: 30},
	}
	missions := []discovery.Mission{
		{ID: 1, Name: "Build with CAP", Effort: "1", Product: "SAP BTP"},
	}

	results := Search(journeys, tuts, missions, "CAP", RecommendOptions{Limit: 10, All: true})

	if len(results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(results))
	}
	// Title matches should come before description-only matches
	for _, r := range results[:3] {
		if r.Slug == "j2" {
			t.Errorf("description-only match (j2) should not appear before title matches")
		}
	}
}

func TestSearch_LevelFilter(t *testing.T) {
	journeys := []learning.LearningJourney{
		{Slug: "j1", Title: "CAP Beginner", Level: "BEGINNER", Product: "SAP BTP"},
		{Slug: "j2", Title: "CAP Advanced", Level: "ADVANCED", Product: "SAP BTP"},
	}

	results := Search(journeys, nil, nil, "CAP", RecommendOptions{Level: "beginner", Limit: 10, All: true})

	if len(results) != 1 {
		t.Fatalf("expected 1 result after level filter, got %d", len(results))
	}
}

func TestSearch_CaseInsensitive(t *testing.T) {
	tuts := []tutorials.TutorialMeta{
		{Slug: "t1", Title: "HANA Cloud Setup", Level: "beginner", Time: 20},
	}

	results := Search(nil, tuts, nil, "hana", RecommendOptions{Limit: 10, All: true})

	if len(results) != 1 {
		t.Fatalf("expected 1 result for case-insensitive search, got %d", len(results))
	}
}

func TestSearch_Limit(t *testing.T) {
	journeys := make([]learning.LearningJourney, 20)
	for i := range journeys {
		journeys[i] = learning.LearningJourney{Slug: "j", Title: "Match", Level: "BEGINNER", Product: "X"}
	}

	results := Search(journeys, nil, nil, "Match", RecommendOptions{Limit: 3, All: true})

	if len(results) != 3 {
		t.Fatalf("expected 3 results after limit, got %d", len(results))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/learn/...`
Expected: FAIL — `Search` not defined

- [ ] **Step 3: Implement Search()**

Create `internal/learn/search.go`:

```go
package learn

import (
	"strings"

	"github.tools.sap/developer-relations/sap-devs-cli/internal/discovery"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/learning"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/tutorials"
)

const (
	scoreTitle = 2
	scoreDesc  = 1
)

type scoredItem struct {
	item  LearnItem
	score int
}

func Search(
	journeys []learning.LearningJourney,
	tuts []tutorials.TutorialMeta,
	missions []discovery.Mission,
	query string,
	opts RecommendOptions,
) []LearnItem {
	q := strings.ToLower(query)
	var scored []scoredItem

	for _, j := range journeys {
		s := matchScore(j.Title, j.Description, q)
		if s > 0 {
			scored = append(scored, scoredItem{journeyToItem(j, false, ""), s})
		}
	}
	for _, t := range tuts {
		s := matchScore(t.Title, t.Description, q)
		if s > 0 {
			scored = append(scored, scoredItem{tutorialToItem(t, false, ""), s})
		}
	}
	for _, m := range missions {
		s := matchScore(m.Name, m.UCLongDescription, q)
		if s > 0 {
			scored = append(scored, scoredItem{missionToItem(m, false, ""), s})
		}
	}

	// Stable sort: title matches first, then description matches
	sortScored(scored)

	var items []LearnItem
	for _, s := range scored {
		items = append(items, s.item)
	}

	items = filterByLevel(items, opts.Level)
	return capItems(items, opts.Limit)
}

func matchScore(title, description, query string) int {
	score := 0
	if strings.Contains(strings.ToLower(title), query) {
		score += scoreTitle
	}
	if strings.Contains(strings.ToLower(description), query) {
		score += scoreDesc
	}
	return score
}

func sortScored(items []scoredItem) {
	// Simple insertion sort (small N, stable)
	for i := 1; i < len(items); i++ {
		key := items[i]
		j := i - 1
		for j >= 0 && items[j].score < key.score {
			items[j+1] = items[j]
			j--
		}
		items[j+1] = key
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/learn/...`
Expected: PASS (all tests)

- [ ] **Step 5: Commit**

```bash
git add internal/learn/search.go internal/learn/search_test.go
git commit -m "feat(learn): implement cross-type Search() with relevance ranking"
```

---

## Task 6: Internal Package — Paths

**Files:**
- Create: `internal/learn/paths.go`
- Create: `internal/learn/paths_test.go`

- [ ] **Step 1: Write the failing tests for paths**

Create `internal/learn/paths_test.go`:

```go
package learn

import (
	"testing"

	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/learning"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/tutorials"
)

func TestLoadPaths(t *testing.T) {
	packs := []*content.Pack{
		{
			ID:   "cap",
			Name: "SAP CAP",
			LearningPaths: []content.LearningPathDef{
				{
					ID:    "cap-start",
					Name:  "Getting Started",
					Level: "beginner",
					Steps: []content.LearningPathStepDef{
						{Type: "journey", Slug: "j1"},
						{Type: "tutorial", Slug: "t1"},
					},
					PackID: "cap",
				},
			},
		},
	}

	paths := LoadPaths(packs)

	if len(paths) != 1 {
		t.Fatalf("expected 1 path, got %d", len(paths))
	}
	if paths[0].ID != "cap-start" {
		t.Errorf("expected id cap-start, got %s", paths[0].ID)
	}
	if paths[0].Generated {
		t.Errorf("expected curated path, got generated")
	}
	if len(paths[0].Steps) != 2 {
		t.Errorf("expected 2 steps, got %d", len(paths[0].Steps))
	}
}

func TestAutoFillPaths_MinTwoItems(t *testing.T) {
	packs := []*content.Pack{
		{
			ID:   "cap",
			Name: "SAP CAP",
			TutorialRefs: []content.TutorialRef{
				{Slug: "t1", Featured: true, PackID: "cap"},
				{Slug: "t2", Featured: true, PackID: "cap"},
			},
		},
	}
	tuts := []tutorials.TutorialMeta{
		{Slug: "t1", Title: "Tut 1", Level: "beginner", Time: 30},
		{Slug: "t2", Title: "Tut 2", Level: "beginner", Time: 45},
	}

	paths := AutoFillPaths(packs, nil, tuts, nil)

	if len(paths) != 1 {
		t.Fatalf("expected 1 auto-filled path, got %d", len(paths))
	}
	if !paths[0].Generated {
		t.Errorf("expected generated flag to be true")
	}
	if paths[0].Level != "beginner" {
		t.Errorf("expected level beginner, got %s", paths[0].Level)
	}
}

func TestAutoFillPaths_SkipsPackWithCuratedPaths(t *testing.T) {
	packs := []*content.Pack{
		{
			ID:   "cap",
			Name: "SAP CAP",
			LearningPaths: []content.LearningPathDef{
				{ID: "existing", Name: "Existing", PackID: "cap"},
			},
			TutorialRefs: []content.TutorialRef{
				{Slug: "t1", Featured: true, PackID: "cap"},
				{Slug: "t2", Featured: true, PackID: "cap"},
			},
		},
	}
	tuts := []tutorials.TutorialMeta{
		{Slug: "t1", Title: "Tut 1", Level: "beginner", Time: 30},
		{Slug: "t2", Title: "Tut 2", Level: "beginner", Time: 45},
	}

	paths := AutoFillPaths(packs, nil, tuts, nil)

	if len(paths) != 0 {
		t.Fatalf("expected 0 auto-filled paths (pack has curated), got %d", len(paths))
	}
}

func TestResolvePaths_PopulatesItems(t *testing.T) {
	paths := []LearningPath{
		{
			ID: "test",
			Steps: []PathStep{
				{Type: ItemJourney, Slug: "j1"},
				{Type: ItemTutorial, Slug: "t1"},
				{Type: ItemMission, Slug: "999"},
			},
		},
	}
	journeys := []learning.LearningJourney{
		{Slug: "j1", Title: "Journey", Level: "BEGINNER", DurationHours: "4"},
	}
	tuts := []tutorials.TutorialMeta{
		{Slug: "t1", Title: "Tutorial", Level: "beginner", Time: 30},
	}

	resolved := ResolvePaths(paths, journeys, tuts, nil)

	if resolved[0].Steps[0].Item == nil {
		t.Fatal("expected journey step to be resolved")
	}
	if resolved[0].Steps[1].Item == nil {
		t.Fatal("expected tutorial step to be resolved")
	}
	if resolved[0].Steps[2].Item != nil {
		t.Fatal("expected unmatched mission step to be nil")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/learn/...`
Expected: FAIL — `LoadPaths`, `AutoFillPaths`, `ResolvePaths` not defined

- [ ] **Step 3: Implement paths.go**

Create `internal/learn/paths.go`:

```go
package learn

import (
	"fmt"
	"strconv"
	"strings"

	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/discovery"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/learning"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/tutorials"
)

func LoadPaths(packs []*content.Pack) []LearningPath {
	var out []LearningPath
	for _, p := range packs {
		for _, def := range p.LearningPaths {
			path := LearningPath{
				ID:          def.ID,
				Name:        def.Name,
				Description: def.Description,
				Level:       NormalizeLevel(def.Level),
				PackID:      def.PackID,
				Generated:   false,
			}
			for _, s := range def.Steps {
				path.Steps = append(path.Steps, PathStep{
					Type: ItemType(s.Type),
					Slug: s.Slug,
				})
			}
			out = append(out, path)
		}
	}
	return out
}

func AutoFillPaths(
	packs []*content.Pack,
	journeys []learning.LearningJourney,
	tuts []tutorials.TutorialMeta,
	missions []discovery.Mission,
) []LearningPath {
	journeyBySlug := make(map[string]learning.LearningJourney, len(journeys))
	for _, j := range journeys {
		journeyBySlug[j.Slug] = j
	}
	tutBySlug := make(map[string]tutorials.TutorialMeta, len(tuts))
	for _, t := range tuts {
		tutBySlug[t.Slug] = t
	}
	missionByID := make(map[int]discovery.Mission, len(missions))
	for _, m := range missions {
		missionByID[m.ID] = m
	}

	var out []LearningPath
	for _, p := range packs {
		if len(p.LearningPaths) > 0 {
			continue
		}

		// Collect featured items grouped by level
		byLevel := make(map[string][]PathStep)

		for _, ref := range p.LearningRefs {
			if !ref.Featured {
				continue
			}
			if j, ok := journeyBySlug[ref.Slug]; ok {
				lvl := NormalizeLevel(j.Level)
				byLevel[lvl] = append(byLevel[lvl], PathStep{Type: ItemJourney, Slug: ref.Slug})
			}
		}
		for _, ref := range p.TutorialRefs {
			if !ref.Featured {
				continue
			}
			if t, ok := tutBySlug[ref.Slug]; ok {
				lvl := NormalizeLevel(t.Level)
				byLevel[lvl] = append(byLevel[lvl], PathStep{Type: ItemTutorial, Slug: ref.Slug})
			}
		}
		for _, ref := range p.DiscoveryMissions {
			if !ref.Featured {
				continue
			}
			if m, ok := missionByID[ref.ID]; ok {
				lvl := effortToLevel(m.Effort)
				byLevel[lvl] = append(byLevel[lvl], PathStep{Type: ItemMission, Slug: fmt.Sprintf("%d", ref.ID)})
			}
		}

		for _, level := range []string{"beginner", "intermediate", "advanced"} {
			steps := byLevel[level]
			if len(steps) < 2 {
				continue
			}
			out = append(out, LearningPath{
				ID:        fmt.Sprintf("%s-%s-auto", p.ID, level),
				Name:      fmt.Sprintf("%s — %s", p.Name, titleCase(level)),
				Level:     level,
				PackID:    p.ID,
				Steps:     steps,
				Generated: true,
			})
		}
	}
	return out
}

func ResolvePaths(
	paths []LearningPath,
	journeys []learning.LearningJourney,
	tuts []tutorials.TutorialMeta,
	missions []discovery.Mission,
) []LearningPath {
	journeyBySlug := make(map[string]learning.LearningJourney, len(journeys))
	for _, j := range journeys {
		journeyBySlug[j.Slug] = j
	}
	tutBySlug := make(map[string]tutorials.TutorialMeta, len(tuts))
	for _, t := range tuts {
		tutBySlug[t.Slug] = t
	}
	missionByID := make(map[int]discovery.Mission, len(missions))
	for _, m := range missions {
		missionByID[m.ID] = m
	}

	resolved := make([]LearningPath, len(paths))
	for i, p := range paths {
		resolved[i] = p
		resolved[i].Steps = make([]PathStep, len(p.Steps))
		for j, step := range p.Steps {
			resolved[i].Steps[j] = step
			switch step.Type {
			case ItemJourney:
				if jr, ok := journeyBySlug[step.Slug]; ok {
					item := journeyToItem(jr, false, p.PackID)
					resolved[i].Steps[j].Item = &item
				}
			case ItemTutorial:
				if t, ok := tutBySlug[step.Slug]; ok {
					item := tutorialToItem(t, false, p.PackID)
					resolved[i].Steps[j].Item = &item
				}
			case ItemMission:
				id, err := strconv.Atoi(step.Slug)
				if err == nil {
					if m, ok := missionByID[id]; ok {
						item := missionToItem(m, false, p.PackID)
						resolved[i].Steps[j].Item = &item
					}
				}
			}
		}
	}
	return resolved
}

func FindPath(paths []LearningPath, id string) *LearningPath {
	for _, p := range paths {
		if p.ID == id {
			return &p
		}
	}
	return nil
}

func titleCase(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/learn/...`
Expected: PASS (all tests)

- [ ] **Step 5: Commit**

```bash
git add internal/learn/paths.go internal/learn/paths_test.go
git commit -m "feat(learn): implement LoadPaths, AutoFillPaths, ResolvePaths"
```

---

## Task 7: i18n Strings

**Files:**
- Modify: `internal/i18n/catalogs/en.json`
- Modify: `internal/i18n/catalogs/de.json`

- [ ] **Step 1: Add learn keys to en.json**

Add these entries to the end of `en.json` (before the closing `}`):

```json
  "learn.short": "Guided learning recommendations",
  "learn.long": "Recommendations, search, and learning paths combining tutorials, learning journeys, and Discovery Center missions — filtered by your active profile and experience level.",
  "learn.recommend.short": "Show recommended learning content",
  "learn.search.short": "Search across tutorials, journeys, and missions",
  "learn.path.short": "Browse curated learning paths",
  "learn.path.list.short": "List available learning paths",
  "learn.path.show.short": "Show learning path details",
  "learn.path.open.short": "Open a learning path in the browser",
  "learn.section_journeys": "Learning Journeys",
  "learn.section_tutorials": "Tutorials",
  "learn.section_missions": "Discovery Center Missions",
  "learn.no_content": "No content cached — run 'sap-devs sync' first.",
  "learn.no_results": "No learning content found.",
  "learn.path_not_found": "Learning path {{.ID}} not found.",
  "learn.step_not_found": "(not found)",
  "learn.invalid_level": "Invalid --level value {{.Level}}. Must be beginner, intermediate, or advanced.",
  "learn.hint_sync": "Run 'sap-devs sync' to include {{.Type}}."
```

- [ ] **Step 2: Add matching keys to de.json**

Mirror the same keys in `de.json` (using English text — German translation deferred).

- [ ] **Step 3: Verify build**

Run: `go build ./...`
Expected: clean build

- [ ] **Step 4: Commit**

```bash
git add internal/i18n/catalogs/en.json internal/i18n/catalogs/de.json
git commit -m "feat(learn): add i18n strings for learn command"
```

---

## Task 8: Command Layer — learn + recommend

**Files:**
- Create: `cmd/learn.go`

- [ ] **Step 1: Create cmd/learn.go**

```go
package cmd

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.tools.sap/developer-relations/sap-devs-cli/internal/config"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/discovery"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/i18n"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/learn"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/learning"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/tutorials"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/xdg"
)

var (
	learnLevel string
	learnAll   bool
	learnCount int
	learnPack  string
)

var learnCmd = &cobra.Command{
	Use:   "learn",
	Short: i18n.T(i18n.ActiveLang, "learn.short"),
	Long:  i18n.T(i18n.ActiveLang, "learn.long"),
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if err := rootCmd.PersistentPreRunE(cmd, args); err != nil {
			return err
		}
		if learnLevel != "" {
			switch learnLevel {
			case "beginner", "intermediate", "advanced":
			default:
				return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "learn.invalid_level", map[string]any{"Level": learnLevel}))
			}
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return learnRecommendCmd.RunE(cmd, args)
	},
}

var learnRecommendCmd = &cobra.Command{
	Use:   "recommend",
	Short: i18n.T(i18n.ActiveLang, "learn.recommend.short"),
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}
		loader, err := newContentLoader()
		if err != nil {
			return err
		}

		packs, err := loadLearnPacks(paths, loader)
		if err != nil {
			return err
		}

		journeys, tuts, missions, anyLoaded := loadLearnIndexes(paths, cmd)
		if !anyLoaded {
			return fmt.Errorf("%s", i18n.T(i18n.ActiveLang, "learn.no_content"))
		}

		level := effectiveLevel(paths)
		opts := learn.RecommendOptions{
			Level:  level,
			PackID: learnPack,
			All:    learnAll,
			Limit:  learnCount,
		}

		recs := learn.Recommend(journeys, tuts, missions, packs, opts)

		printed := false
		if len(recs.Journeys) > 0 {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "learn.section_journeys"))
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "  %s\t%s\t%s\t%s\n", "FEATURED", "TITLE", "LEVEL", "DURATION")
			for _, item := range recs.Journeys {
				feat := ""
				if item.Featured {
					feat = "★"
				}
				title := truncate(item.Title, 55)
				fmt.Fprintf(w, "  %s\t%s\t%s\t%s\n", feat, title, titleCaseLevel(item.Level), item.Duration)
			}
			w.Flush()
			printed = true
		}

		if len(recs.Tutorials) > 0 {
			if printed {
				fmt.Fprintln(cmd.OutOrStdout())
			}
			fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "learn.section_tutorials"))
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "  %s\t%s\t%s\t%s\n", "FEATURED", "TITLE", "LEVEL", "TIME")
			for _, item := range recs.Tutorials {
				feat := ""
				if item.Featured {
					feat = "★"
				}
				title := truncate(item.Title, 55)
				fmt.Fprintf(w, "  %s\t%s\t%s\t%s\n", feat, title, titleCaseLevel(item.Level), item.Duration)
			}
			w.Flush()
			printed = true
		}

		if len(recs.Missions) > 0 {
			if printed {
				fmt.Fprintln(cmd.OutOrStdout())
			}
			fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "learn.section_missions"))
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "  %s\t%s\t%s\n", "FEATURED", "TITLE", "EFFORT")
			for _, item := range recs.Missions {
				feat := ""
				if item.Featured {
					feat = "★"
				}
				title := truncate(item.Title, 55)
				fmt.Fprintf(w, "  %s\t%s\t%s\n", feat, title, effortLabel(item.Level))
			}
			w.Flush()
		}

		if !printed && len(recs.Missions) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "learn.no_results"))
		}

		return nil
	},
}

func loadLearnPacks(paths *xdg.Paths, loader *content.ContentLoader) ([]*content.Pack, error) {
	if learnAll {
		return loader.LoadPacks(nil, i18n.ActiveLang)
	}
	profileCfg, err := config.LoadProfile(paths.ConfigDir)
	if err == nil && profileCfg.ID != "" {
		if p, _ := loader.FindProfile(profileCfg.ID); p != nil {
			return loader.LoadPacks(p, i18n.ActiveLang)
		}
	}
	return loader.LoadPacks(nil, i18n.ActiveLang)
}

func loadLearnIndexes(paths *xdg.Paths, cmd *cobra.Command) (
	[]learning.LearningJourney,
	[]tutorials.TutorialMeta,
	[]discovery.Mission,
	bool,
) {
	var journeys []learning.LearningJourney
	var tuts []tutorials.TutorialMeta
	var missions []discovery.Mission
	anyLoaded := false

	if j, ok := learning.LoadIndex(paths.CacheDir, learning.CacheTTL); ok {
		journeys = j
		anyLoaded = true
	} else {
		fmt.Fprintln(cmd.ErrOrStderr(), i18n.Tf(i18n.ActiveLang, "learn.hint_sync", map[string]any{"Type": "learning journeys"}))
	}

	if t, err := tutorials.LoadIndex(paths.CacheDir); err == nil {
		tuts = t
		anyLoaded = true
	} else {
		fmt.Fprintln(cmd.ErrOrStderr(), i18n.Tf(i18n.ActiveLang, "learn.hint_sync", map[string]any{"Type": "tutorials"}))
	}

	if m, ok := discovery.LoadCache[[]discovery.Mission](paths.CacheDir, "missions", discovery.CacheTTL); ok {
		missions = m
		anyLoaded = true
	} else {
		fmt.Fprintln(cmd.ErrOrStderr(), i18n.Tf(i18n.ActiveLang, "learn.hint_sync", map[string]any{"Type": "missions"}))
	}

	return journeys, tuts, missions, anyLoaded
}

func effectiveLevel(paths *xdg.Paths) string {
	if learnLevel != "" {
		return learnLevel
	}
	cfg, err := config.Load(paths.ConfigDir)
	if err != nil {
		return ""
	}
	return cfg.ExperienceLevel
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

func titleCaseLevel(level string) string {
	switch level {
	case "beginner":
		return "Beginner"
	case "intermediate":
		return "Intermediate"
	case "advanced":
		return "Advanced"
	default:
		return level
	}
}

func effortLabel(level string) string {
	switch level {
	case "beginner":
		return "Easy"
	case "intermediate":
		return "Medium"
	case "advanced":
		return "Hard"
	default:
		return level
	}
}

func init() {
	learnCmd.PersistentFlags().StringVar(&learnLevel, "level", "", "filter by level (beginner, intermediate, advanced)")
	learnCmd.PersistentFlags().BoolVar(&learnAll, "all", false, "bypass profile filtering")
	learnCmd.PersistentFlags().IntVarP(&learnCount, "count", "n", 10, "limit results per section")

	learnRecommendCmd.Flags().StringVar(&learnPack, "pack", "", "filter to a specific pack")

	learnCmd.AddCommand(learnRecommendCmd)
	rootCmd.AddCommand(learnCmd)
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./...`
Expected: clean build

- [ ] **Step 3: Smoke test**

Run: `go run . learn --help`
Expected: shows learn command help with recommend, search, path subcommands (search/path added in later tasks)

- [ ] **Step 4: Commit**

```bash
git add cmd/learn.go
git commit -m "feat(learn): add learn command with recommend subcommand"
```

---

## Task 9: Command Layer — learn search

**Files:**
- Create: `cmd/learn_search.go`

- [ ] **Step 1: Create cmd/learn_search.go**

```go
package cmd

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.tools.sap/developer-relations/sap-devs-cli/internal/i18n"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/learn"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/xdg"
)

var learnSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: i18n.T(i18n.ActiveLang, "learn.search.short"),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}

		journeys, tuts, missions, anyLoaded := loadLearnIndexes(paths, cmd)
		if !anyLoaded {
			return fmt.Errorf("%s", i18n.T(i18n.ActiveLang, "learn.no_content"))
		}

		level := effectiveLevel(paths)
		opts := learn.RecommendOptions{
			Level: level,
			All:   true,
			Limit: learnCount,
		}

		results := learn.Search(journeys, tuts, missions, args[0], opts)

		if len(results) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "learn.no_results"))
			return nil
		}

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", "#", "TYPE", "TITLE", "LEVEL", "TIME")
		for i, item := range results {
			typeName := typeLabel(item.Type)
			title := truncate(item.Title, 50)
			level := titleCaseLevel(item.Level)
			duration := item.Duration
			if duration == "" {
				duration = "-"
			}
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n", i+1, typeName, title, level, duration)
		}
		w.Flush()
		return nil
	},
}

func typeLabel(t learn.ItemType) string {
	switch t {
	case learn.ItemJourney:
		return "Journey"
	case learn.ItemTutorial:
		return "Tutorial"
	case learn.ItemMission:
		return "Mission"
	default:
		return string(t)
	}
}

func init() {
	learnCmd.AddCommand(learnSearchCmd)
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./...`
Expected: clean build

- [ ] **Step 3: Commit**

```bash
git add cmd/learn_search.go
git commit -m "feat(learn): add learn search subcommand"
```

---

## Task 10: Command Layer — learn path

**Files:**
- Create: `cmd/learn_path.go`

- [ ] **Step 1: Create cmd/learn_path.go**

```go
package cmd

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/charmbracelet/glamour"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"

	"github.tools.sap/developer-relations/sap-devs-cli/internal/i18n"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/learn"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/xdg"
)

var learnPathCmd = &cobra.Command{
	Use:   "path",
	Short: i18n.T(i18n.ActiveLang, "learn.path.short"),
	RunE: func(cmd *cobra.Command, args []string) error {
		return learnPathListCmd.RunE(cmd, args)
	},
}

var learnPathListCmd = &cobra.Command{
	Use:   "list",
	Short: i18n.T(i18n.ActiveLang, "learn.path.list.short"),
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}
		loader, err := newContentLoader()
		if err != nil {
			return err
		}

		packs, err := loadLearnPacks(paths, loader)
		if err != nil {
			return err
		}

		journeys, tuts, missions, _ := loadLearnIndexes(paths, cmd)

		curated := learn.LoadPaths(packs)
		auto := learn.AutoFillPaths(packs, journeys, tuts, missions)

		allPaths := append(curated, auto...)

		level := effectiveLevel(paths)
		if level != "" {
			var filtered []learn.LearningPath
			for _, p := range allPaths {
				if p.Level == "" || p.Level == level {
					filtered = append(filtered, p)
				}
			}
			allPaths = filtered
		}

		if learnPack != "" {
			var filtered []learn.LearningPath
			for _, p := range allPaths {
				if p.PackID == learnPack {
					filtered = append(filtered, p)
				}
			}
			allPaths = filtered
		}

		if len(allPaths) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "learn.no_results"))
			return nil
		}

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", "NAME", "LEVEL", "STEPS", "PACK", "SOURCE")
		for _, p := range allPaths {
			source := "curated"
			if p.Generated {
				source = "auto"
			}
			level := titleCaseLevel(p.Level)
			if level == "" {
				level = "-"
			}
			name := truncate(p.Name, 40)
			fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\n", name, level, len(p.Steps), p.PackID, source)
		}
		w.Flush()
		return nil
	},
}

var learnPathShowCmd = &cobra.Command{
	Use:   "show <path-id>",
	Short: i18n.T(i18n.ActiveLang, "learn.path.show.short"),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		xdgPaths, err := xdg.New()
		if err != nil {
			return err
		}
		loader, err := newContentLoader()
		if err != nil {
			return err
		}

		packs, err := loadLearnPacks(xdgPaths, loader)
		if err != nil {
			return err
		}

		journeys, tuts, missions, _ := loadLearnIndexes(xdgPaths, cmd)

		curated := learn.LoadPaths(packs)
		auto := learn.AutoFillPaths(packs, journeys, tuts, missions)
		allPaths := append(curated, auto...)
		allPaths = learn.ResolvePaths(allPaths, journeys, tuts, missions)

		p := learn.FindPath(allPaths, args[0])
		if p == nil {
			return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "learn.path_not_found", map[string]any{"ID": args[0]}))
		}

		source := "curated"
		if p.Generated {
			source = "auto"
		}

		var b strings.Builder
		b.WriteString(fmt.Sprintf("# %s\n\n", p.Name))
		b.WriteString(fmt.Sprintf("**Level:** %s | **Pack:** %s | **Source:** %s\n\n", titleCaseLevel(p.Level), p.PackID, source))
		if p.Description != "" {
			b.WriteString(p.Description + "\n\n")
		}

		for i, step := range p.Steps {
			if step.Item != nil {
				b.WriteString(fmt.Sprintf("%d. [%s] %s", i+1, typeLabel(step.Item.Type), step.Item.Title))
				if step.Item.Duration != "" {
					b.WriteString(fmt.Sprintf(" (%s)", step.Item.Duration))
				}
				b.WriteString("\n")
				if step.Item.URL != "" {
					b.WriteString(fmt.Sprintf("   %s\n", step.Item.URL))
				}
			} else {
				b.WriteString(fmt.Sprintf("%d. [%s] %s %s\n", i+1, string(step.Type), step.Slug, i18n.T(i18n.ActiveLang, "learn.step_not_found")))
			}
			b.WriteString("\n")
		}

		renderer, err := glamour.NewTermRenderer(glamour.WithAutoStyle(), glamour.WithWordWrap(80))
		if err != nil {
			fmt.Fprint(cmd.OutOrStdout(), b.String())
			return nil
		}
		rendered, err := renderer.Render(b.String())
		if err != nil {
			fmt.Fprint(cmd.OutOrStdout(), b.String())
			return nil
		}
		fmt.Fprint(cmd.OutOrStdout(), rendered)
		return nil
	},
}

var learnPathOpenCmd = &cobra.Command{
	Use:   "open <path-id>",
	Short: i18n.T(i18n.ActiveLang, "learn.path.open.short"),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		xdgPaths, err := xdg.New()
		if err != nil {
			return err
		}
		loader, err := newContentLoader()
		if err != nil {
			return err
		}

		packs, err := loadLearnPacks(xdgPaths, loader)
		if err != nil {
			return err
		}

		journeys, tuts, missions, _ := loadLearnIndexes(xdgPaths, cmd)

		curated := learn.LoadPaths(packs)
		auto := learn.AutoFillPaths(packs, journeys, tuts, missions)
		allPaths := append(curated, auto...)
		allPaths = learn.ResolvePaths(allPaths, journeys, tuts, missions)

		p := learn.FindPath(allPaths, args[0])
		if p == nil {
			return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "learn.path_not_found", map[string]any{"ID": args[0]}))
		}

		for _, step := range p.Steps {
			if step.Item != nil && step.Item.URL != "" {
				if err := browser.OpenURL(step.Item.URL); err != nil {
					fmt.Fprintf(cmd.OutOrStdout(), "Could not open browser. Visit: %s\n", step.Item.URL)
					return nil
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Opening %s\n", step.Item.URL)
				return nil
			}
		}

		fmt.Fprintln(cmd.OutOrStdout(), "No resolvable URLs in this path.")
		return nil
	},
}

func init() {
	learnPathListCmd.Flags().StringVar(&learnPack, "pack", "", "filter to a specific pack")
	learnPathCmd.AddCommand(learnPathListCmd, learnPathShowCmd, learnPathOpenCmd)
	learnCmd.AddCommand(learnPathCmd)
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./...`
Expected: clean build

- [ ] **Step 3: Smoke test**

Run: `go run . learn path --help`
Expected: shows path subcommand help with list/show/open

- [ ] **Step 4: Commit**

```bash
git add cmd/learn_path.go
git commit -m "feat(learn): add learn path subcommand (list, show, open)"
```

---

## Task 11: Seed Content — CAP paths.yaml

**Files:**
- Create: `content/packs/cap/paths.yaml`

- [ ] **Step 1: Create the seed paths.yaml**

```yaml
paths:
  - id: cap-getting-started
    name: Getting Started with CAP
    level: beginner
    description: Build your first CAP application from project creation to deployment
    steps:
      - type: journey
        slug: developing-with-sap-cloud-application-programming-model
      - type: tutorial
        slug: cap-getting-started
      - type: tutorial
        slug: hana-cloud-cap-create-project
      - type: mission
        slug: "4327"

  - id: cap-deployment
    name: CAP Deployment & Operations
    level: intermediate
    description: Deploy and operate CAP applications on SAP BTP Cloud Foundry
    steps:
      - type: tutorial
        slug: cap-service-deploy-to-cf
      - type: mission
        slug: "4432"
      - type: journey
        slug: becoming-an-sap-btp-solution-architect
```

- [ ] **Step 2: Verify YAML is valid**

Run: `go run . learn path list`
Expected: shows the two curated paths (may show "sync" hints for missing indexes)

- [ ] **Step 3: Commit**

```bash
git add content/packs/cap/paths.yaml
git commit -m "content: add seed learning paths for CAP pack"
```

---

## Task 12: JSON Schema + VS Code Integration

**Files:**
- Create: `content/schemas/paths.yaml.schema.json`
- Modify: `.vscode/settings.json`

- [ ] **Step 1: Create the JSON schema**

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "Learning Paths",
  "description": "Schema for sap-devs paths.yaml files",
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "paths": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["id", "name", "steps"],
        "additionalProperties": false,
        "properties": {
          "id": {
            "type": "string",
            "pattern": "^[a-z][a-z0-9-]*[a-z0-9]$",
            "description": "Unique path identifier"
          },
          "name": {
            "type": "string",
            "description": "Human-readable path name"
          },
          "description": {
            "type": "string",
            "description": "Short description of the learning path"
          },
          "level": {
            "type": "string",
            "enum": ["beginner", "intermediate", "advanced"],
            "description": "Target experience level"
          },
          "steps": {
            "type": "array",
            "minItems": 1,
            "items": {
              "type": "object",
              "required": ["type", "slug"],
              "additionalProperties": false,
              "properties": {
                "type": {
                  "type": "string",
                  "enum": ["journey", "tutorial", "mission"],
                  "description": "Content source type"
                },
                "slug": {
                  "type": "string",
                  "description": "Content identifier (slug for journeys/tutorials, stringified ID for missions)"
                }
              }
            }
          }
        }
      }
    }
  }
}
```

- [ ] **Step 2: Register in .vscode/settings.json**

Add to the `yaml.schemas` object:

```json
"./content/schemas/paths.yaml.schema.json": "**/packs/*/paths.yaml"
```

- [ ] **Step 3: Commit**

```bash
git add content/schemas/paths.yaml.schema.json .vscode/settings.json
git commit -m "feat(learn): add paths.yaml JSON schema and VS Code integration"
```

---

## Task 13: Documentation — Update CLAUDE.md

**Files:**
- Modify: `CLAUDE.md`

- [ ] **Step 1: Add learn command to the CLI Commands table**

In the CLI Commands table in CLAUDE.md, add after the `learning` row:

```
| `learn` | Guided learning recommendations combining tutorials, journeys, and missions; `learn recommend/search`, `learn path list/show/open` |
```

- [ ] **Step 2: Add Learn section to Architecture Overview**

Add a new `### Learn` section after the `### Learning Journeys` section:

```markdown
### Learn

`sap-devs learn` ([cmd/learn.go](cmd/learn.go), [cmd/learn_search.go](cmd/learn_search.go), [cmd/learn_path.go](cmd/learn_path.go)) is an umbrella command aggregating content from learning journeys, tutorials, and Discovery Center missions. The `internal/learn` package handles cross-type recommendation (three-tier resolution per type), search (substring match with title-priority ranking), and learning path management (curated from `paths.yaml` + auto-filled from featured pack content).

**Subcommands:** `recommend` (default, sectioned output), `search <query>` (unified cross-type search), `path list/show/open` (curated + auto-generated learning paths).

**Experience level:** Stored in `experience_level` config field. Filters content across all three types using normalized levels (beginner/intermediate/advanced). Mission effort maps to levels: 0-1→beginner, 2→intermediate, 3→advanced.
```

- [ ] **Step 3: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: add learn command to CLAUDE.md architecture and CLI reference"
```

---

## Task 14: Final Verification

- [ ] **Step 1: Full build**

Run: `go build ./...`
Expected: clean build, no errors

- [ ] **Step 2: Full vet**

Run: `go vet ./...`
Expected: no issues

- [ ] **Step 3: Run tests (if on Linux/CI)**

Run: `go test ./internal/learn/...`
Expected: PASS — all tests in recommend_test.go, search_test.go, paths_test.go

- [ ] **Step 4: Smoke test — recommend**

Run: `go run . learn`
Expected: sectioned output or "sync" hints if indexes not cached

- [ ] **Step 5: Smoke test — search**

Run: `go run . learn search CAP`
Expected: unified search results or "sync" hints

- [ ] **Step 6: Smoke test — path list**

Run: `go run . learn path list`
Expected: shows "Getting Started with CAP" and "CAP Deployment & Operations"

- [ ] **Step 7: Smoke test — path show**

Run: `go run . learn path show cap-getting-started`
Expected: glamour-rendered path with numbered steps

- [ ] **Step 8: Smoke test — help**

Run: `go run . learn --help`
Expected: shows all subcommands (recommend, search, path) and flags (--level, --all, --count)
