# Tutorials Feature (Phase 1) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `sap-devs tutorial` commands to browse, search, and render tutorials from the sap-tutorials GitHub org with full index sync, on-demand content fetch, and progress tracking.

**Architecture:** GitHub-based two-phase pipeline: sync builds a metadata index from YAML frontmatter across ~21 sap-tutorials repos, with optional developers.sap.com API enrichment. Full tutorial content (parsed markdown with step navigation) is fetched on demand. Curated `tutorials.yaml` per pack provides profile-filtered browsing; the full index enables cross-cutting search.

**Tech Stack:** Go, cobra, charmbracelet/bubbletea, charmbracelet/glamour, net/http (GitHub raw + API), encoding/json, gopkg.in/yaml.v3

**Spec:** `docs/superpowers/specs/2026-04-18-tutorials-design.md`

---

## File Structure

### New files

| File | Responsibility |
|------|---------------|
| `internal/tutorials/types.go` | `TutorialMeta`, `Tutorial`, `TutorialStep`, `TutorialProgress`, `RepoInfo` structs |
| `internal/tutorials/parser.go` | Markdown→Tutorial parsing: v2 (H3 steps) and v1 (ACCORDION) parsers, frontmatter extraction, tag normalisation |
| `internal/tutorials/parser_test.go` | Unit tests for both parsers, frontmatter edge cases, OPTION blocks |
| `internal/tutorials/cache.go` | Load/save index, content, and repo info from `{cacheDir}/tutorials/` |
| `internal/tutorials/cache_test.go` | Round-trip cache tests |
| `internal/tutorials/client.go` | GitHub HTTP client: fetch repo list, tree listings, raw markdown |
| `internal/tutorials/client_test.go` | Tests with HTTP test server |
| `internal/tutorials/search.go` | Full-text search + level/tag filtering on `[]TutorialMeta` |
| `internal/tutorials/search_test.go` | Search and filter tests |
| `internal/tutorials/progress.go` | Load/save/update progress from XDG data dir |
| `internal/tutorials/progress_test.go` | Progress state machine tests |
| `internal/tutorials/enrichment.go` | Optional developers.sap.com Solr API enrichment with silent fallback |
| `internal/tutorials/enrichment_test.go` | Tests with HTTP test server (200 + 403 cases) |
| `internal/content/tutorials.go` | `FlattenTutorialRefs`, `FilterTutorialRefsByPack`, `FindTutorialRef` |
| `internal/content/tutorials_test.go` | Unit tests mirroring `samples_test.go` pattern |
| `cmd/tutorials.go` | `tutorial list/search/show/open` cobra commands |
| `content/schemas/tutorials.yaml.schema.json` | JSON Schema for `tutorials.yaml` |
| `content/packs/cap/tutorials.yaml` | Curated CAP tutorials |
| `content/packs/btp-core/tutorials.yaml` | Curated BTP tutorials |
| `content/packs/abap/tutorials.yaml` | Curated ABAP tutorials |

### Modified files

| File | Change |
|------|--------|
| `internal/content/pack.go` | Add `TutorialRef` struct, `TutorialRefs` field on `Pack`, load in `LoadPack()` |
| `internal/config/config.go` | Add `Tutorials` TTL to `SyncConfig`, `TutorialConfig` sub-struct, defaults |
| `cmd/sync.go` | Add `"tutorials"` category, `runTutorialsFetch()`, wire into independent phase |
| `.vscode/settings.json` | Wire `tutorials.yaml.schema.json` |
| `internal/i18n/catalogs/en.json` | Add `tutorial.*` i18n keys |
| `internal/i18n/catalogs/de.json` | Add `tutorial.*` i18n keys (German) |

---

## Task 1: Data Model — TutorialRef in pack.go

**Files:**
- Modify: `internal/content/pack.go`
- Test: `internal/content/pack_test.go`

- [ ] **Step 1: Add TutorialRef struct and Pack field**

In `internal/content/pack.go`, add after the `Sample` struct (line ~128):

```go
// TutorialRef is a curated tutorial reference in a pack's tutorials.yaml.
type TutorialRef struct {
	Slug     string `yaml:"slug"`
	Featured bool   `yaml:"featured,omitempty"`
	PackID   string // set at load time, not in YAML
}
```

Add to the `Pack` struct (after `Samples []Sample` around line 37):

```go
	TutorialRefs []TutorialRef
```

- [ ] **Step 2: Load tutorials.yaml in LoadPack()**

In `LoadPack()`, add after the samples loading block (after line ~343):

```go
	if data, err := os.ReadFile(filepath.Join(packDir, "tutorials.yaml")); err == nil {
		_ = yaml.Unmarshal(data, &pack.TutorialRefs)
		for i := range pack.TutorialRefs {
			pack.TutorialRefs[i].PackID = pack.ID
		}
	}
```

- [ ] **Step 3: Verify build compiles**

Run: `go build ./...`
Expected: clean build, no errors.

- [ ] **Step 4: Commit**

```bash
git add internal/content/pack.go
git commit -m "feat(tutorials): add TutorialRef struct and LoadPack integration"
```

---

## Task 2: Content Helper Functions — tutorials.go

**Files:**
- Create: `internal/content/tutorials.go`
- Create: `internal/content/tutorials_test.go`

- [ ] **Step 1: Write test file**

Create `internal/content/tutorials_test.go`:

```go
package content_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
)

func fixtureTutorialPacks() []*content.Pack {
	return []*content.Pack{
		{
			ID: "cap",
			TutorialRefs: []content.TutorialRef{
				{Slug: "cap-getting-started", Featured: true, PackID: "cap"},
				{Slug: "cap-deploy-cf", Featured: false, PackID: "cap"},
			},
		},
		{
			ID: "abap",
			TutorialRefs: []content.TutorialRef{
				{Slug: "abap-rap-create", Featured: true, PackID: "abap"},
			},
		},
	}
}

func TestFlattenTutorialRefs(t *testing.T) {
	got := content.FlattenTutorialRefs(fixtureTutorialPacks())
	require.Len(t, got, 3)
	assert.Equal(t, "cap-getting-started", got[0].Slug)
	assert.Equal(t, "cap", got[0].PackID)
}

func TestFlattenTutorialRefs_NilInput(t *testing.T) {
	got := content.FlattenTutorialRefs(nil)
	assert.Empty(t, got)
}

func TestFilterTutorialRefsByPack(t *testing.T) {
	got := content.FilterTutorialRefsByPack(fixtureTutorialPacks(), "cap")
	require.Len(t, got, 2)
	assert.Equal(t, "cap-getting-started", got[0].Slug)
}

func TestFilterTutorialRefsByPack_NotFound(t *testing.T) {
	got := content.FilterTutorialRefsByPack(fixtureTutorialPacks(), "nonexistent")
	assert.Nil(t, got)
}

func TestFindTutorialRef(t *testing.T) {
	got := content.FindTutorialRef(fixtureTutorialPacks(), "abap-rap-create")
	require.NotNil(t, got)
	assert.True(t, got.Featured)
}

func TestFindTutorialRef_NotFound(t *testing.T) {
	got := content.FindTutorialRef(fixtureTutorialPacks(), "nonexistent")
	assert.Nil(t, got)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/content/... -run TestFlattenTutorialRefs -v` (CI only)
Expected: FAIL — functions not defined.

- [ ] **Step 3: Write implementation**

Create `internal/content/tutorials.go`:

```go
package content

// FlattenTutorialRefs collects all tutorial references from all packs.
func FlattenTutorialRefs(packs []*Pack) []TutorialRef {
	var out []TutorialRef
	for _, p := range packs {
		out = append(out, p.TutorialRefs...)
	}
	return out
}

// FilterTutorialRefsByPack returns tutorial refs from the matching pack.
func FilterTutorialRefsByPack(packs []*Pack, packID string) []TutorialRef {
	for _, p := range packs {
		if p.ID == packID {
			return p.TutorialRefs
		}
	}
	return nil
}

// FindTutorialRef returns the first tutorial ref matching slug, or nil.
func FindTutorialRef(packs []*Pack, slug string) *TutorialRef {
	for _, p := range packs {
		for i := range p.TutorialRefs {
			if p.TutorialRefs[i].Slug == slug {
				return &p.TutorialRefs[i]
			}
		}
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/content/... -run TestFlattenTutorial -v` (CI only)
Local: `go build ./... && go vet ./...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/content/tutorials.go internal/content/tutorials_test.go
git commit -m "feat(tutorials): add TutorialRef helper functions with tests"
```

---

## Task 3: Types and Cache — internal/tutorials package

**Files:**
- Create: `internal/tutorials/types.go`
- Create: `internal/tutorials/cache.go`
- Create: `internal/tutorials/cache_test.go`

- [ ] **Step 1: Write types.go**

Create `internal/tutorials/types.go`:

```go
package tutorials

import "time"

// TutorialMeta is a resolved tutorial in the full index (cached from GitHub).
type TutorialMeta struct {
	Slug       string   `json:"slug"`
	Title      string   `json:"title"`
	Description string  `json:"description"`
	Time       int      `json:"time"`
	Level      string   `json:"level"`
	Tags       []string `json:"tags"`
	PrimaryTag string   `json:"primary_tag"`
	Author     string   `json:"author,omitempty"`
	Repo       string   `json:"repo"`
	URL        string   `json:"url"`
	Parser     string   `json:"parser"`
}

// Tutorial is a fully parsed tutorial with step content.
type Tutorial struct {
	TutorialMeta
	Prerequisites string         `json:"prerequisites,omitempty"`
	YouWillLearn  []string       `json:"you_will_learn,omitempty"`
	Steps         []TutorialStep `json:"steps"`
}

// TutorialStep is a single step within a tutorial.
type TutorialStep struct {
	Number  int    `json:"number"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

// TutorialProgress tracks a user's position within a tutorial.
type TutorialProgress struct {
	Slug           string     `json:"slug"`
	CurrentStep    int        `json:"current_step"`
	CompletedSteps []int      `json:"completed_steps"`
	TotalSteps     int        `json:"total_steps"`
	StartedAt      time.Time  `json:"started_at"`
	LastAccessed   time.Time  `json:"last_accessed"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
}

// RepoInfo stores cached metadata about a sap-tutorials repo.
type RepoInfo struct {
	Name          string `json:"name"`
	DefaultBranch string `json:"default_branch"`
	TreeSHA       string `json:"tree_sha,omitempty"`
}
```

- [ ] **Step 2: Write cache test**

Create `internal/tutorials/cache_test.go`:

```go
package tutorials_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/tutorials"
)

func TestIndexCache_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	index := []tutorials.TutorialMeta{
		{Slug: "cap-getting-started", Title: "Getting Started with CAP", Time: 30, Level: "beginner", Repo: "Tutorials"},
		{Slug: "abap-rap-create", Title: "Create a RAP BO", Time: 20, Level: "intermediate", Repo: "abap-core-development"},
	}

	require.NoError(t, tutorials.SaveIndex(dir, index))
	loaded, err := tutorials.LoadIndex(dir)
	require.NoError(t, err)
	require.Len(t, loaded, 2)
	assert.Equal(t, "cap-getting-started", loaded[0].Slug)
	assert.Equal(t, 30, loaded[0].Time)
}

func TestIndexCache_NotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := tutorials.LoadIndex(dir)
	assert.Error(t, err)
}

func TestContentCache_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	tut := &tutorials.Tutorial{
		TutorialMeta: tutorials.TutorialMeta{Slug: "test-tutorial", Title: "Test"},
		Steps: []tutorials.TutorialStep{
			{Number: 1, Title: "Step One", Content: "Do this."},
		},
	}

	require.NoError(t, tutorials.SaveContent(dir, tut))
	loaded, err := tutorials.LoadContent(dir, "test-tutorial")
	require.NoError(t, err)
	require.Len(t, loaded.Steps, 1)
	assert.Equal(t, "Step One", loaded.Steps[0].Title)
}

func TestContentCache_NotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := tutorials.LoadContent(dir, "nonexistent")
	assert.Error(t, err)
}

func TestRepoInfoCache_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	repos := []tutorials.RepoInfo{
		{Name: "Tutorials", DefaultBranch: "master", TreeSHA: "abc123"},
		{Name: "abap-core-development", DefaultBranch: "main", TreeSHA: "def456"},
	}

	require.NoError(t, tutorials.SaveRepoInfo(dir, repos))
	loaded, err := tutorials.LoadRepoInfo(dir)
	require.NoError(t, err)
	require.Len(t, loaded, 2)
	assert.Equal(t, "master", loaded[0].DefaultBranch)
}

func TestCacheAge_Exists(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "tutorials"), 0755)
	os.WriteFile(filepath.Join(dir, "tutorials", "index.json"), []byte("[]"), 0644)
	age := tutorials.IndexCacheAge(dir)
	assert.True(t, age >= 0)
}

func TestCacheAge_Missing(t *testing.T) {
	dir := t.TempDir()
	age := tutorials.IndexCacheAge(dir)
	assert.True(t, age < 0)
}
```

- [ ] **Step 3: Write cache implementation**

Create `internal/tutorials/cache.go`:

```go
package tutorials

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

func tutorialsDir(cacheDir string) string {
	return filepath.Join(cacheDir, "tutorials")
}

// SaveIndex writes the tutorial index to the cache.
func SaveIndex(cacheDir string, index []TutorialMeta) error {
	dir := tutorialsDir(cacheDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.Marshal(index)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "index.json"), data, 0644)
}

// LoadIndex reads the tutorial index from the cache.
func LoadIndex(cacheDir string) ([]TutorialMeta, error) {
	data, err := os.ReadFile(filepath.Join(tutorialsDir(cacheDir), "index.json"))
	if err != nil {
		return nil, err
	}
	var index []TutorialMeta
	return index, json.Unmarshal(data, &index)
}

// IndexCacheAge returns the age of the index cache file, or a negative duration if missing.
func IndexCacheAge(cacheDir string) time.Duration {
	info, err := os.Stat(filepath.Join(tutorialsDir(cacheDir), "index.json"))
	if err != nil {
		return -1
	}
	return time.Since(info.ModTime())
}

// SaveContent writes a parsed tutorial to the content cache.
func SaveContent(cacheDir string, tut *Tutorial) error {
	dir := filepath.Join(tutorialsDir(cacheDir), "content")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.Marshal(tut)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, tut.Slug+".json"), data, 0644)
}

// LoadContent reads a parsed tutorial from the content cache.
func LoadContent(cacheDir, slug string) (*Tutorial, error) {
	data, err := os.ReadFile(filepath.Join(tutorialsDir(cacheDir), "content", slug+".json"))
	if err != nil {
		return nil, err
	}
	var tut Tutorial
	return &tut, json.Unmarshal(data, &tut)
}

// SaveRepoInfo writes cached repo metadata.
func SaveRepoInfo(cacheDir string, repos []RepoInfo) error {
	dir := tutorialsDir(cacheDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.Marshal(repos)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "repos.json"), data, 0644)
}

// LoadRepoInfo reads cached repo metadata.
func LoadRepoInfo(cacheDir string) ([]RepoInfo, error) {
	data, err := os.ReadFile(filepath.Join(tutorialsDir(cacheDir), "repos.json"))
	if err != nil {
		return nil, err
	}
	var repos []RepoInfo
	return repos, json.Unmarshal(data, &repos)
}
```

- [ ] **Step 4: Verify build and tests**

Run: `go build ./... && go vet ./...`
CI: `go test ./internal/tutorials/... -v`

- [ ] **Step 5: Commit**

```bash
git add internal/tutorials/
git commit -m "feat(tutorials): add types and cache layer"
```

---

## Task 4: Markdown Parser

**Files:**
- Create: `internal/tutorials/parser.go`
- Create: `internal/tutorials/parser_test.go`

- [ ] **Step 1: Write parser tests**

Create `internal/tutorials/parser_test.go`:

```go
package tutorials_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/tutorials"
)

const sampleV2Markdown = `---
parser: v2
time: 20
tags: [ tutorial>beginner, software-product>sap-cloud-application-programming-model ]
primary_tag: software-product>sap-cloud-application-programming-model
author_name: Test Author
---

# Getting Started with CAP
<!-- description --> Learn how to create your first CAP project

## Prerequisites
- Node.js 18+
- @sap/cds-dk installed

## You will learn
- How to init a CAP project
- How to define a data model

### Create a new CAP project
Open a terminal and run:
` + "```bash\ncds init my-bookshop\n```" + `

### Define a data model
Create ` + "`db/schema.cds`" + `:
` + "```cds\nentity Books { key ID : Integer; title : String; }\n```" + `
`

func TestParseV2_BasicStructure(t *testing.T) {
	tut, err := tutorials.Parse(sampleV2Markdown, "cap-getting-started", "Tutorials")
	require.NoError(t, err)
	assert.Equal(t, "Getting Started with CAP", tut.Title)
	assert.Equal(t, "Learn how to create your first CAP project", tut.Description)
	assert.Equal(t, 20, tut.Time)
	assert.Equal(t, "beginner", tut.Level)
	assert.Equal(t, "Test Author", tut.Author)
	assert.Contains(t, tut.Prerequisites, "Node.js 18+")
	require.Len(t, tut.YouWillLearn, 2)
	assert.Equal(t, "How to init a CAP project", tut.YouWillLearn[0])
	require.Len(t, tut.Steps, 2)
	assert.Equal(t, 1, tut.Steps[0].Number)
	assert.Equal(t, "Create a new CAP project", tut.Steps[0].Title)
	assert.Contains(t, tut.Steps[0].Content, "cds init my-bookshop")
	assert.Equal(t, 2, tut.Steps[1].Number)
	assert.Equal(t, "Define a data model", tut.Steps[1].Title)
}

func TestParseV2_TitleFromFrontmatter(t *testing.T) {
	md := "---\nparser: v2\ntitle: Explicit Title\ntime: 10\ntags: [tutorial>advanced]\nprimary_tag: topic>cloud\n---\n\n### Step One\nContent\n"
	tut, err := tutorials.Parse(md, "test-slug", "Tutorials")
	require.NoError(t, err)
	assert.Equal(t, "Explicit Title", tut.Title)
}

func TestParseV2_URLGeneration(t *testing.T) {
	md := "---\nparser: v2\ntime: 10\ntags: [tutorial>beginner]\nprimary_tag: topic>cloud\n---\n\n# Title\n\n### Step\nContent\n"
	tut, err := tutorials.Parse(md, "my-tutorial", "Tutorials")
	require.NoError(t, err)
	assert.Equal(t, "https://developers.sap.com/tutorials/my-tutorial.html", tut.URL)
}

const sampleV1Markdown = `---
time: 15
tags: [ tutorial>intermediate ]
primary_tag: topic>abap
---

# Legacy Tutorial

[ACCORDION-BEGIN [Step 1: ](Create Something)]
Do this first.
[ACCORDION-END]

[ACCORDION-BEGIN [Step 2: ](Configure Something)]
Then do this.
[ACCORDION-END]
`

func TestParseV1_AccordionSteps(t *testing.T) {
	tut, err := tutorials.Parse(sampleV1Markdown, "legacy-tutorial", "Tutorials")
	require.NoError(t, err)
	assert.Equal(t, "Legacy Tutorial", tut.Title)
	assert.Equal(t, "intermediate", tut.Level)
	require.Len(t, tut.Steps, 2)
	assert.Equal(t, "Create Something", tut.Steps[0].Title)
	assert.Contains(t, tut.Steps[0].Content, "Do this first.")
	assert.Equal(t, "Configure Something", tut.Steps[1].Title)
}

func TestParseFrontmatterOnly(t *testing.T) {
	md := "---\nparser: v2\ntime: 30\ntags: [tutorial>beginner, topic>cloud]\nprimary_tag: topic>cloud\nauthor_name: Jane Doe\n---\n\n# My Tutorial\n<!-- description --> A description\n\n### Step 1\nContent\n"
	meta, err := tutorials.ParseFrontmatterOnly(md, "my-slug", "TestRepo")
	require.NoError(t, err)
	assert.Equal(t, "my-slug", meta.Slug)
	assert.Equal(t, "My Tutorial", meta.Title)
	assert.Equal(t, "A description", meta.Description)
	assert.Equal(t, 30, meta.Time)
	assert.Equal(t, "beginner", meta.Level)
	assert.Equal(t, "Jane Doe", meta.Author)
	assert.Equal(t, "TestRepo", meta.Repo)
	assert.Equal(t, "v2", meta.Parser)
}

func TestParseFrontmatterOnly_SlugFallbackTitle(t *testing.T) {
	md := "---\ntime: 10\ntags: [tutorial>beginner]\nprimary_tag: x\n---\nNo headings here.\n"
	meta, err := tutorials.ParseFrontmatterOnly(md, "my-slug", "Repo")
	require.NoError(t, err)
	assert.Equal(t, "my-slug", meta.Title)
}

func TestParseV2_OptionBlocks(t *testing.T) {
	md := "---\nparser: v2\ntime: 10\ntags: [tutorial>beginner]\nprimary_tag: x\n---\n\n# Test\n\n### Install dependencies\n\n[OPTION BEGIN [Node.js]]\nRun `npm install`.\n[OPTION END]\n\n[OPTION BEGIN [Java]]\nRun `mvn install`.\n[OPTION END]\n"
	tut, err := tutorials.Parse(md, "test-options", "Tutorials")
	require.NoError(t, err)
	require.Len(t, tut.Steps, 1)
	assert.Contains(t, tut.Steps[0].Content, "#### Option: Node.js")
	assert.Contains(t, tut.Steps[0].Content, "#### Option: Java")
	assert.NotContains(t, tut.Steps[0].Content, "[OPTION BEGIN")
	assert.NotContains(t, tut.Steps[0].Content, "[OPTION END]")
}

func TestExtractLevel(t *testing.T) {
	tests := []struct {
		tags  []string
		level string
	}{
		{[]string{"tutorial>beginner", "topic>cloud"}, "beginner"},
		{[]string{"tutorial>intermediate"}, "intermediate"},
		{[]string{"tutorial>advanced"}, "advanced"},
		{[]string{"topic>cloud"}, ""},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.level, tutorials.ExtractLevel(tt.tags))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go build ./internal/tutorials/...` — expected to fail (functions not defined).

- [ ] **Step 3: Write parser implementation**

Create `internal/tutorials/parser.go`:

```go
package tutorials

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type frontmatter struct {
	Parser     string   `yaml:"parser"`
	Title      string   `yaml:"title"`
	Desc       string   `yaml:"description"`
	Time       any      `yaml:"time"`
	Tags       []string `yaml:"tags"`
	PrimaryTag string   `yaml:"primary_tag"`
	AuthorName string   `yaml:"author_name"`
}

// Parse parses a full tutorial markdown into a Tutorial struct.
func Parse(md, slug, repo string) (*Tutorial, error) {
	fm, body, err := splitFrontmatter(md)
	if err != nil {
		return nil, err
	}

	meta := buildMeta(fm, body, slug, repo)
	tut := &Tutorial{TutorialMeta: meta}

	tut.Prerequisites = extractSection(body, "Prerequisites")
	tut.YouWillLearn = extractBulletList(body, "You will learn")

	if fm.Parser == "v2" {
		tut.Steps = parseV2Steps(body)
	} else {
		tut.Steps = parseV1Steps(body)
	}

	return tut, nil
}

// ParseFrontmatterOnly extracts metadata without parsing steps.
func ParseFrontmatterOnly(md, slug, repo string) (*TutorialMeta, error) {
	fm, body, err := splitFrontmatter(md)
	if err != nil {
		return nil, err
	}
	meta := buildMeta(fm, body, slug, repo)
	return &meta, nil
}

// ExtractLevel derives the experience level from tutorial tags.
func ExtractLevel(tags []string) string {
	for _, t := range tags {
		lower := strings.ToLower(t)
		if strings.HasPrefix(lower, "tutorial>") {
			level := strings.TrimPrefix(lower, "tutorial>")
			switch level {
			case "beginner", "intermediate", "advanced":
				return level
			}
		}
	}
	return ""
}

func splitFrontmatter(md string) (*frontmatter, string, error) {
	if !strings.HasPrefix(strings.TrimSpace(md), "---") {
		return &frontmatter{}, md, nil
	}
	trimmed := strings.TrimSpace(md)
	parts := strings.SplitN(trimmed[3:], "---", 2)
	if len(parts) < 2 {
		return &frontmatter{}, md, nil
	}
	var fm frontmatter
	if err := yaml.Unmarshal([]byte(parts[0]), &fm); err != nil {
		return nil, "", fmt.Errorf("parse frontmatter: %w", err)
	}
	return &fm, strings.TrimSpace(parts[1]), nil
}

func buildMeta(fm *frontmatter, body, slug, repo string) TutorialMeta {
	meta := TutorialMeta{
		Slug:       slug,
		Tags:       fm.Tags,
		PrimaryTag: fm.PrimaryTag,
		Author:     fm.AuthorName,
		Repo:       repo,
		URL:        fmt.Sprintf("https://developers.sap.com/tutorials/%s.html", slug),
		Parser:     fm.Parser,
		Level:      ExtractLevel(fm.Tags),
	}

	switch v := fm.Time.(type) {
	case int:
		meta.Time = v
	case float64:
		meta.Time = int(v)
	case string:
		meta.Time, _ = strconv.Atoi(v)
	}

	// Title: frontmatter → H1 → slug
	if fm.Title != "" {
		meta.Title = fm.Title
	} else {
		meta.Title = extractH1(body)
		if meta.Title == "" {
			meta.Title = slug
		}
	}

	// Description: frontmatter → <!-- description --> comment
	if fm.Desc != "" {
		meta.Description = fm.Desc
	} else {
		meta.Description = extractDescriptionComment(body)
	}

	return meta
}

func extractH1(body string) string {
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") && !strings.HasPrefix(line, "## ") {
			return strings.TrimPrefix(line, "# ")
		}
	}
	return ""
}

var descCommentRE = regexp.MustCompile(`<!--\s*description\s*-->\s*(.+)`)

func extractDescriptionComment(body string) string {
	m := descCommentRE.FindStringSubmatch(body)
	if len(m) >= 2 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

func extractSection(body, heading string) string {
	marker := "## " + heading
	idx := strings.Index(body, marker)
	if idx < 0 {
		return ""
	}
	rest := body[idx+len(marker):]
	rest = strings.TrimLeft(rest, " \t\r\n")
	end := strings.Index(rest, "\n## ")
	if end < 0 {
		end = strings.Index(rest, "\n### ")
	}
	if end < 0 {
		return strings.TrimSpace(rest)
	}
	return strings.TrimSpace(rest[:end])
}

func extractBulletList(body, heading string) []string {
	section := extractSection(body, heading)
	if section == "" {
		return nil
	}
	var items []string
	for _, line := range strings.Split(section, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "- ") {
			items = append(items, strings.TrimPrefix(line, "- "))
		} else if strings.HasPrefix(line, "* ") {
			items = append(items, strings.TrimPrefix(line, "* "))
		}
	}
	return items
}

func parseV2Steps(body string) []TutorialStep {
	var steps []TutorialStep
	parts := strings.Split("\n"+body, "\n### ")
	for i, part := range parts {
		if i == 0 {
			continue // preamble
		}
		lines := strings.SplitN(part, "\n", 2)
		title := strings.TrimSpace(lines[0])
		content := ""
		if len(lines) > 1 {
			content = strings.TrimSpace(lines[1])
		}
		content = normalizeOptionBlocks(content)
		steps = append(steps, TutorialStep{
			Number:  i,
			Title:   title,
			Content: content,
		})
	}
	return steps
}

var accordionBeginRE = regexp.MustCompile(`\[ACCORDION-BEGIN\s+\[Step\s+\d+:\s*\]\((.+?)\)\]`)

func parseV1Steps(body string) []TutorialStep {
	var steps []TutorialStep
	matches := accordionBeginRE.FindAllStringSubmatchIndex(body, -1)
	for i, match := range matches {
		title := body[match[2]:match[3]]
		contentStart := match[1]
		var contentEnd int
		endTag := "[ACCORDION-END]"
		endIdx := strings.Index(body[contentStart:], endTag)
		if endIdx >= 0 {
			contentEnd = contentStart + endIdx
		} else if i+1 < len(matches) {
			contentEnd = matches[i+1][0]
		} else {
			contentEnd = len(body)
		}
		content := strings.TrimSpace(body[contentStart:contentEnd])
		content = normalizeOptionBlocks(content)
		steps = append(steps, TutorialStep{
			Number:  i + 1,
			Title:   title,
			Content: content,
		})
	}
	return steps
}

var optionBeginRE = regexp.MustCompile(`\[OPTION BEGIN \[(.+?)\]\]`)

// normalizeOptionBlocks converts [OPTION BEGIN [Tab Name]] / [OPTION END] pairs
// into markdown subheadings for terminal display.
func normalizeOptionBlocks(content string) string {
	content = optionBeginRE.ReplaceAllString(content, "\n#### Option: $1\n")
	content = strings.ReplaceAll(content, "[OPTION END]", "")
	return strings.TrimSpace(content)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go build ./... && go vet ./...`
CI: `go test ./internal/tutorials/... -run TestParse -v`

- [ ] **Step 5: Commit**

```bash
git add internal/tutorials/parser.go internal/tutorials/parser_test.go
git commit -m "feat(tutorials): add markdown parser with v1/v2 support"
```

---

## Task 5: Search and Filtering

**Files:**
- Create: `internal/tutorials/search.go`
- Create: `internal/tutorials/search_test.go`

- [ ] **Step 1: Write search tests**

Create `internal/tutorials/search_test.go`:

```go
package tutorials_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/tutorials"
)

func fixtureIndex() []tutorials.TutorialMeta {
	return []tutorials.TutorialMeta{
		{Slug: "cap-getting-started", Title: "Getting Started with CAP", Description: "Learn CAP basics", Level: "beginner", Tags: []string{"tutorial>beginner", "software-product>cap"}},
		{Slug: "fiori-elements-create", Title: "Create a Fiori Elements App", Description: "Build Fiori UI", Level: "intermediate", Tags: []string{"tutorial>intermediate", "topic>fiori"}},
		{Slug: "abap-rap-bo", Title: "RAP Business Object", Description: "Create a RAP BO", Level: "advanced", Tags: []string{"tutorial>advanced", "topic>abap", "topic>rap"}},
	}
}

func TestSearch_TitleMatch(t *testing.T) {
	got := tutorials.Search(fixtureIndex(), "Fiori")
	require.Len(t, got, 1)
	assert.Equal(t, "fiori-elements-create", got[0].Slug)
}

func TestSearch_DescriptionMatch(t *testing.T) {
	got := tutorials.Search(fixtureIndex(), "RAP BO")
	require.Len(t, got, 1)
	assert.Equal(t, "abap-rap-bo", got[0].Slug)
}

func TestSearch_SlugMatch(t *testing.T) {
	got := tutorials.Search(fixtureIndex(), "cap-getting")
	require.Len(t, got, 1)
}

func TestSearch_TagMatch(t *testing.T) {
	got := tutorials.Search(fixtureIndex(), "rap")
	require.Len(t, got, 1)
	assert.Equal(t, "abap-rap-bo", got[0].Slug)
}

func TestSearch_CaseInsensitive(t *testing.T) {
	got := tutorials.Search(fixtureIndex(), "CAP")
	assert.True(t, len(got) >= 1)
}

func TestSearch_NoResults(t *testing.T) {
	got := tutorials.Search(fixtureIndex(), "zzznomatch")
	assert.Empty(t, got)
}

func TestFilterByLevel(t *testing.T) {
	got := tutorials.FilterByLevel(fixtureIndex(), "beginner")
	require.Len(t, got, 1)
	assert.Equal(t, "cap-getting-started", got[0].Slug)
}

func TestFilterByTags(t *testing.T) {
	got := tutorials.FilterByTags(fixtureIndex(), []string{"fiori"})
	require.Len(t, got, 1)
	assert.Equal(t, "fiori-elements-create", got[0].Slug)
}

func TestFindBySlug(t *testing.T) {
	got := tutorials.FindBySlug(fixtureIndex(), "abap-rap-bo")
	require.NotNil(t, got)
	assert.Equal(t, "RAP Business Object", got.Title)
}

func TestFindBySlug_NotFound(t *testing.T) {
	got := tutorials.FindBySlug(fixtureIndex(), "nonexistent")
	assert.Nil(t, got)
}
```

- [ ] **Step 2: Write search implementation**

Create `internal/tutorials/search.go`:

```go
package tutorials

import "strings"

// Search returns tutorials matching query against title, description, slug, and tags.
func Search(index []TutorialMeta, query string) []TutorialMeta {
	q := strings.ToLower(query)
	var out []TutorialMeta
	for _, m := range index {
		if strings.Contains(strings.ToLower(m.Title), q) ||
			strings.Contains(strings.ToLower(m.Description), q) ||
			strings.Contains(strings.ToLower(m.Slug), q) {
			out = append(out, m)
			continue
		}
		for _, tag := range m.Tags {
			if strings.Contains(strings.ToLower(tag), q) {
				out = append(out, m)
				break
			}
		}
	}
	return out
}

// FilterByLevel returns tutorials matching the given level.
func FilterByLevel(index []TutorialMeta, level string) []TutorialMeta {
	l := strings.ToLower(level)
	var out []TutorialMeta
	for _, m := range index {
		if strings.ToLower(m.Level) == l {
			out = append(out, m)
		}
	}
	return out
}

// FilterByTags returns tutorials with at least one tag matching (OR, case-insensitive, substring).
func FilterByTags(index []TutorialMeta, tags []string) []TutorialMeta {
	needles := make([]string, len(tags))
	for i, t := range tags {
		needles[i] = strings.ToLower(strings.TrimSpace(t))
	}
	var out []TutorialMeta
	for _, m := range index {
		if matchesAnyTag(m.Tags, needles) {
			out = append(out, m)
		}
	}
	return out
}

func matchesAnyTag(tags, needles []string) bool {
	for _, t := range tags {
		lower := strings.ToLower(t)
		for _, n := range needles {
			if strings.Contains(lower, n) {
				return true
			}
		}
	}
	return false
}

// FindBySlug returns the first tutorial matching slug, or nil.
func FindBySlug(index []TutorialMeta, slug string) *TutorialMeta {
	for i := range index {
		if index[i].Slug == slug {
			return &index[i]
		}
	}
	return nil
}
```

- [ ] **Step 3: Run tests**

Run: `go build ./... && go vet ./...`
CI: `go test ./internal/tutorials/... -run TestSearch -v`

- [ ] **Step 4: Commit**

```bash
git add internal/tutorials/search.go internal/tutorials/search_test.go
git commit -m "feat(tutorials): add search and filtering"
```

---

## Task 6: Progress Tracking

**Files:**
- Create: `internal/tutorials/progress.go`
- Create: `internal/tutorials/progress_test.go`

- [ ] **Step 1: Write progress tests**

Create `internal/tutorials/progress_test.go`:

```go
package tutorials_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/tutorials"
)

func TestProgress_NewTutorial(t *testing.T) {
	dir := t.TempDir()
	err := tutorials.UpdateProgress(dir, "test-tut", 1, 5, false)
	require.NoError(t, err)

	p, err := tutorials.GetProgress(dir, "test-tut")
	require.NoError(t, err)
	require.NotNil(t, p)
	assert.Equal(t, 1, p.CurrentStep)
	assert.Equal(t, 5, p.TotalSteps)
	assert.Empty(t, p.CompletedSteps)
	assert.False(t, p.StartedAt.IsZero())
}

func TestProgress_MarkStepDone(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, tutorials.UpdateProgress(dir, "test-tut", 1, 3, true))
	require.NoError(t, tutorials.UpdateProgress(dir, "test-tut", 2, 3, false))

	p, err := tutorials.GetProgress(dir, "test-tut")
	require.NoError(t, err)
	assert.Equal(t, 2, p.CurrentStep)
	assert.Equal(t, []int{1}, p.CompletedSteps)
	assert.Nil(t, p.CompletedAt)
}

func TestProgress_AllStepsDone(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, tutorials.UpdateProgress(dir, "test-tut", 1, 2, true))
	require.NoError(t, tutorials.UpdateProgress(dir, "test-tut", 2, 2, true))

	p, err := tutorials.GetProgress(dir, "test-tut")
	require.NoError(t, err)
	assert.NotNil(t, p.CompletedAt)
	assert.True(t, p.CompletedAt.Before(time.Now().Add(time.Second)))
}

func TestProgress_NoDuplicateCompletedSteps(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, tutorials.UpdateProgress(dir, "test-tut", 1, 3, true))
	require.NoError(t, tutorials.UpdateProgress(dir, "test-tut", 1, 3, true))

	p, err := tutorials.GetProgress(dir, "test-tut")
	require.NoError(t, err)
	assert.Equal(t, []int{1}, p.CompletedSteps)
}

func TestProgress_NotStarted(t *testing.T) {
	dir := t.TempDir()
	p, err := tutorials.GetProgress(dir, "nonexistent")
	require.NoError(t, err)
	assert.Nil(t, p)
}

func TestProgress_LoadAll(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, tutorials.UpdateProgress(dir, "tut-a", 1, 3, false))
	require.NoError(t, tutorials.UpdateProgress(dir, "tut-b", 2, 5, false))

	all, err := tutorials.LoadProgress(dir)
	require.NoError(t, err)
	assert.Len(t, all, 2)
}
```

- [ ] **Step 2: Write progress implementation**

Create `internal/tutorials/progress.go`:

```go
package tutorials

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

const progressFile = "tutorial-progress.json"

func progressPath(dataDir string) string {
	return filepath.Join(dataDir, progressFile)
}

// LoadProgress reads all tutorial progress from the data directory.
func LoadProgress(dataDir string) (map[string]TutorialProgress, error) {
	data, err := os.ReadFile(progressPath(dataDir))
	if os.IsNotExist(err) {
		return make(map[string]TutorialProgress), nil
	}
	if err != nil {
		return nil, err
	}
	var progress map[string]TutorialProgress
	if err := json.Unmarshal(data, &progress); err != nil {
		return nil, err
	}
	return progress, nil
}

// SaveProgress writes all tutorial progress to the data directory.
func SaveProgress(dataDir string, progress map[string]TutorialProgress) error {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(progress, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(progressPath(dataDir), data, 0644)
}

// GetProgress returns progress for a single tutorial, or nil if not started.
func GetProgress(dataDir, slug string) (*TutorialProgress, error) {
	all, err := LoadProgress(dataDir)
	if err != nil {
		return nil, err
	}
	if p, ok := all[slug]; ok {
		return &p, nil
	}
	return nil, nil
}

// UpdateProgress updates progress for a tutorial, creating a new entry if needed.
func UpdateProgress(dataDir, slug string, currentStep, totalSteps int, markDone bool) error {
	all, err := LoadProgress(dataDir)
	if err != nil {
		return err
	}

	now := time.Now()
	p, exists := all[slug]
	if !exists {
		p = TutorialProgress{
			Slug:       slug,
			TotalSteps: totalSteps,
			StartedAt:  now,
		}
	}

	p.CurrentStep = currentStep
	p.LastAccessed = now
	p.TotalSteps = totalSteps

	if markDone {
		found := false
		for _, s := range p.CompletedSteps {
			if s == currentStep {
				found = true
				break
			}
		}
		if !found {
			p.CompletedSteps = append(p.CompletedSteps, currentStep)
		}
	}

	if len(p.CompletedSteps) >= totalSteps && p.CompletedAt == nil {
		p.CompletedAt = &now
	}

	all[slug] = p
	return SaveProgress(dataDir, all)
}
```

- [ ] **Step 3: Run tests**

Run: `go build ./... && go vet ./...`
CI: `go test ./internal/tutorials/... -run TestProgress -v`

- [ ] **Step 4: Commit**

```bash
git add internal/tutorials/progress.go internal/tutorials/progress_test.go
git commit -m "feat(tutorials): add progress tracking"
```

---

## Task 7: GitHub Client

**Files:**
- Create: `internal/tutorials/client.go`
- Create: `internal/tutorials/client_test.go`

- [ ] **Step 1: Write client tests**

Create `internal/tutorials/client_test.go` with httptest-based tests covering:
- `FetchRepoList()` — parses repository-groups.json array
- `FetchRepoTree()` — extracts tutorial slugs from tree API response
- `FetchFrontmatter()` — fetches raw markdown and parses frontmatter
- Error handling — 404, network errors, malformed JSON

```go
package tutorials_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/tutorials"
)

func TestFetchRepoList(t *testing.T) {
	repos := []map[string]string{
		{"name": "Tutorials", "urlBase": "https://github.com/sap-tutorials/Tutorials"},
		{"name": "abap-core-development", "urlBase": "https://github.com/sap-tutorials/abap-core-development"},
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(repos)
	}))
	defer ts.Close()

	client := tutorials.NewClient(tutorials.ClientConfig{RepoListURL: ts.URL})
	got, err := client.FetchRepoList()
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "Tutorials", got[0])
}

func TestFetchRepoTree(t *testing.T) {
	treeResp := map[string]any{
		"sha": "abc123",
		"tree": []map[string]string{
			{"path": "tutorials/cap-getting-started/cap-getting-started.md", "type": "blob"},
			{"path": "tutorials/abap-rap/abap-rap.md", "type": "blob"},
			{"path": "README.md", "type": "blob"},
		},
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(treeResp)
	}))
	defer ts.Close()

	client := tutorials.NewClient(tutorials.ClientConfig{APIBaseURL: ts.URL})
	slugs, sha, err := client.FetchRepoTree("Tutorials", "master")
	require.NoError(t, err)
	assert.Equal(t, "abc123", sha)
	require.Len(t, slugs, 2)
	assert.Contains(t, slugs, "cap-getting-started")
	assert.Contains(t, slugs, "abap-rap")
}

func TestFetchDefaultBranch(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"default_branch": "master"})
	}))
	defer ts.Close()

	client := tutorials.NewClient(tutorials.ClientConfig{APIBaseURL: ts.URL})
	branch, err := client.FetchDefaultBranch("Tutorials")
	require.NoError(t, err)
	assert.Equal(t, "master", branch)
}

func TestFetchRawMarkdown(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("---\nparser: v2\ntime: 20\ntags: [tutorial>beginner]\nprimary_tag: x\n---\n\n# Test\n\n### Step 1\nContent\n"))
	}))
	defer ts.Close()

	client := tutorials.NewClient(tutorials.ClientConfig{RawBaseURL: ts.URL})
	md, err := client.FetchRawMarkdown("Tutorials", "main", "test-slug")
	require.NoError(t, err)
	assert.Contains(t, md, "# Test")
}
```

- [ ] **Step 2: Write client implementation**

Create `internal/tutorials/client.go`:

```go
package tutorials

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	defaultRepoListURL = "https://raw.githubusercontent.com/sap-tutorials/Tutorials/master/config/repository-groups.json"
	defaultAPIBaseURL  = "https://api.github.com"
	defaultRawBaseURL  = "https://raw.githubusercontent.com"
)

// ClientConfig allows overriding base URLs for testing.
type ClientConfig struct {
	RepoListURL string
	APIBaseURL  string
	RawBaseURL  string
	Token       string
	UserAgent   string
}

// Client handles GitHub API interactions for tutorials.
type Client struct {
	http   *http.Client
	config ClientConfig
}

// NewClient creates a new tutorial GitHub client.
func NewClient(cfg ClientConfig) *Client {
	if cfg.RepoListURL == "" {
		cfg.RepoListURL = defaultRepoListURL
	}
	if cfg.APIBaseURL == "" {
		cfg.APIBaseURL = defaultAPIBaseURL
	}
	if cfg.RawBaseURL == "" {
		cfg.RawBaseURL = defaultRawBaseURL
	}
	if cfg.UserAgent == "" {
		cfg.UserAgent = "sap-devs-cli"
	}
	return &Client{
		http:   &http.Client{Timeout: 30 * time.Second},
		config: cfg,
	}
}

type repoGroupEntry struct {
	Name string `json:"name"`
}

// FetchRepoList fetches the list of tutorial repo names.
func (c *Client) FetchRepoList() ([]string, error) {
	body, err := c.get(c.config.RepoListURL)
	if err != nil {
		return nil, fmt.Errorf("fetch repo list: %w", err)
	}
	var entries []repoGroupEntry
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, fmt.Errorf("parse repo list: %w", err)
	}
	var names []string
	for _, e := range entries {
		if e.Name != "" {
			names = append(names, e.Name)
		}
	}
	return names, nil
}

// FetchDefaultBranch returns the default branch for a repo.
func (c *Client) FetchDefaultBranch(repo string) (string, error) {
	url := fmt.Sprintf("%s/repos/sap-tutorials/%s", c.config.APIBaseURL, repo)
	body, err := c.get(url)
	if err != nil {
		return "", fmt.Errorf("fetch repo info %s: %w", repo, err)
	}
	var info struct {
		DefaultBranch string `json:"default_branch"`
	}
	if err := json.Unmarshal(body, &info); err != nil {
		return "", err
	}
	return info.DefaultBranch, nil
}

// FetchRepoTree fetches the tree for a repo and returns tutorial slugs + tree SHA.
func (c *Client) FetchRepoTree(repo, branch string) (slugs []string, sha string, err error) {
	url := fmt.Sprintf("%s/repos/sap-tutorials/%s/git/trees/%s?recursive=1", c.config.APIBaseURL, repo, branch)
	body, err := c.get(url)
	if err != nil {
		return nil, "", fmt.Errorf("fetch tree %s: %w", repo, err)
	}
	var tree struct {
		SHA  string `json:"sha"`
		Tree []struct {
			Path string `json:"path"`
			Type string `json:"type"`
		} `json:"tree"`
	}
	if err := json.Unmarshal(body, &tree); err != nil {
		return nil, "", err
	}

	seen := make(map[string]bool)
	for _, entry := range tree.Tree {
		if !strings.HasPrefix(entry.Path, "tutorials/") {
			continue
		}
		parts := strings.Split(entry.Path, "/")
		if len(parts) >= 2 && parts[1] != "" {
			slug := parts[1]
			if !seen[slug] {
				seen[slug] = true
				slugs = append(slugs, slug)
			}
		}
	}
	return slugs, tree.SHA, nil
}

// FetchRawMarkdown fetches the raw markdown content for a tutorial.
func (c *Client) FetchRawMarkdown(repo, branch, slug string) (string, error) {
	url := fmt.Sprintf("%s/sap-tutorials/%s/%s/tutorials/%s/%s.md", c.config.RawBaseURL, repo, branch, slug, slug)
	body, err := c.get(url)
	if err != nil {
		return "", fmt.Errorf("fetch markdown %s/%s: %w", repo, slug, err)
	}
	return string(body), nil
}

func (c *Client) get(url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.config.UserAgent)
	if c.config.Token != "" {
		req.Header.Set("Authorization", "token "+c.config.Token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d for %s", resp.StatusCode, url)
	}
	return io.ReadAll(resp.Body)
}
```

- [ ] **Step 3: Run tests**

Run: `go build ./... && go vet ./...`
CI: `go test ./internal/tutorials/... -run TestFetch -v`

- [ ] **Step 4: Commit**

```bash
git add internal/tutorials/client.go internal/tutorials/client_test.go
git commit -m "feat(tutorials): add GitHub client for repo list, tree, and content fetching"
```

---

## Task 8: API Enrichment (Optional)

**Files:**
- Create: `internal/tutorials/enrichment.go`
- Create: `internal/tutorials/enrichment_test.go`

- [ ] **Step 1: Write enrichment tests**

Create `internal/tutorials/enrichment_test.go` testing:
- Successful 200 response merges featured/mission data
- 403 response returns original index unchanged (no error)
- Malformed JSON returns original index unchanged

- [ ] **Step 2: Write enrichment implementation**

Create `internal/tutorials/enrichment.go`:

```go
package tutorials

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const solrSearchURL = "https://developers.sap.com/bin/sapdx/v3/solr/search"

// Enrich attempts to augment the index with data from developers.sap.com.
// Returns the original index unchanged if the API is unavailable (403, timeout, etc.).
func Enrich(index []TutorialMeta, userAgent string) []TutorialMeta {
	payload := fmt.Sprintf(`{"rows":"2000","start":0,"searchField":"","pagePath":"/content/developers/website/languages/en/tutorial-navigator","language":"en_us","addDefaultLanguage":true,"filters":[]}`)
	url := fmt.Sprintf("%s?json=%s", solrSearchURL, payload)

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return index
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return index
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return index
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return index
	}

	var result struct {
		Result []struct {
			PublicURL string `json:"publicUrl"`
			Featured  bool   `json:"featured"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return index
	}

	featuredSlugs := make(map[string]bool)
	for _, r := range result.Result {
		slug := extractSlugFromURL(r.PublicURL)
		if slug != "" && r.Featured {
			featuredSlugs[slug] = true
		}
	}

	// Currently we only extract featured flags.
	// Mission/group membership can be added later.
	_ = featuredSlugs

	return index
}

func extractSlugFromURL(publicURL string) string {
	// publicUrl format: "/tutorials/slug.html"
	if len(publicURL) < 12 {
		return ""
	}
	s := publicURL
	if s[0] == '/' {
		s = s[1:]
	}
	if len(s) > 10 && s[:10] == "tutorials/" {
		slug := s[10:]
		if len(slug) > 5 && slug[len(slug)-5:] == ".html" {
			return slug[:len(slug)-5]
		}
	}
	return ""
}
```

- [ ] **Step 3: Run tests**

Run: `go build ./... && go vet ./...`
CI: `go test ./internal/tutorials/... -run TestEnrich -v`

- [ ] **Step 4: Commit**

```bash
git add internal/tutorials/enrichment.go internal/tutorials/enrichment_test.go
git commit -m "feat(tutorials): add optional developers.sap.com API enrichment"
```

---

## Task 9: Config and Sync Integration

**Files:**
- Modify: `internal/config/config.go`
- Modify: `cmd/sync.go`

- [ ] **Step 1: Add Tutorials TTL to SyncConfig**

In `internal/config/config.go`, add to `SyncConfig`:

```go
	Tutorials time.Duration `yaml:"tutorials"`
```

Add to `Config`:

```go
	Tutorial TutorialConfig `yaml:"tutorial,omitempty"`
```

Add new struct:

```go
// TutorialConfig controls tutorial display behaviour.
type TutorialConfig struct {
	Interactive bool `yaml:"interactive,omitempty"`
}
```

Add to `Default()` in the `Sync` block:

```go
	Tutorials: 168 * time.Hour, // 7 days
```

- [ ] **Step 2: Wire tutorials into sync.go**

In `cmd/sync.go`:

1. Add `"tutorials"` to `allCategories()` return.
2. Add `"tutorials"` to `independentCats` slice.
3. Add `"tutorials": cfg.Sync.Tutorials,` to the `ttls` map.
4. Add Phase 6 block after the discovery phase:

```go
	// Phase 6: Tutorials index fetch
	if containsString(activeIndependent, "tutorials") && (force || engine.IsStale("tutorials")) {
		if err := runTutorialsFetch(paths.CacheDir, force); err != nil {
			fmt.Fprintf(os.Stderr, "sap-devs: tutorials sync: %v\n", err)
		}
		_ = engine.MarkSynced("tutorials")
	}
```

5. Add `runTutorialsFetch()` function:

```go
func runTutorialsFetch(cacheDir string, force bool) error {
	cachedRepos, _ := tutorials.LoadRepoInfo(cacheDir)
	cachedSHAs := make(map[string]string)
	cachedBranches := make(map[string]string)
	for _, r := range cachedRepos {
		cachedSHAs[r.Name] = r.TreeSHA
		cachedBranches[r.Name] = r.DefaultBranch
	}

	// Tutorials live on public github.com, not github.tools.sap.
	// Do NOT use credentials.Resolve() here — it targets the enterprise instance.
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		token = os.Getenv("GH_TOKEN")
	}

	client := tutorials.NewClient(tutorials.ClientConfig{Token: token})

	repoNames, err := client.FetchRepoList()
	if err != nil {
		return err
	}

	// Phase 1: resolve branches + trees (API calls, bounded concurrency)
	type repoResult struct {
		info  tutorials.RepoInfo
		slugs []string
		reuse bool // true if tree SHA unchanged
	}
	results := make([]repoResult, len(repoNames))

	var mu sync.Mutex
	g, _ := errgroup.WithContext(context.Background())
	g.SetLimit(5)

	for i, repo := range repoNames {
		i, repo := i, repo
		g.Go(func() error {
			branch, ok := cachedBranches[repo]
			if !ok || force {
				var err error
				branch, err = client.FetchDefaultBranch(repo)
				if err != nil {
					fmt.Fprintf(os.Stderr, "sap-devs: skip repo %s: %v\n", repo, err)
					return nil
				}
			}

			slugs, sha, err := client.FetchRepoTree(repo, branch)
			if err != nil {
				fmt.Fprintf(os.Stderr, "sap-devs: skip repo %s: %v\n", repo, err)
				return nil
			}

			mu.Lock()
			results[i] = repoResult{
				info:  tutorials.RepoInfo{Name: repo, DefaultBranch: branch, TreeSHA: sha},
				slugs: slugs,
				reuse: !force && cachedSHAs[repo] == sha && sha != "",
			}
			mu.Unlock()
			return nil
		})
	}
	_ = g.Wait()

	// Phase 2: fetch frontmatter for changed repos (CDN, bounded concurrency)
	var allMeta []tutorials.TutorialMeta
	var repoInfos []tutorials.RepoInfo
	existingIndex, _ := tutorials.LoadIndex(cacheDir)

	for _, r := range results {
		if r.info.Name == "" {
			continue // skipped repo
		}
		repoInfos = append(repoInfos, r.info)

		if r.reuse {
			for _, m := range existingIndex {
				if m.Repo == r.info.Name {
					allMeta = append(allMeta, m)
				}
			}
			continue
		}

		// Fetch frontmatter in parallel (CDN, no rate limit)
		var repoMeta []tutorials.TutorialMeta
		var metaMu sync.Mutex
		fg, _ := errgroup.WithContext(context.Background())
		fg.SetLimit(10)

		for _, slug := range r.slugs {
			slug := slug
			repo := r.info.Name
			branch := r.info.DefaultBranch
			fg.Go(func() error {
				md, err := client.FetchRawMarkdown(repo, branch, slug)
				if err != nil {
					return nil
				}
				meta, err := tutorials.ParseFrontmatterOnly(md, slug, repo)
				if err != nil {
					return nil
				}
				metaMu.Lock()
				repoMeta = append(repoMeta, *meta)
				metaMu.Unlock()
				return nil
			})
		}
		_ = fg.Wait()
		allMeta = append(allMeta, repoMeta...)
	}

	allMeta = tutorials.Enrich(allMeta, "sap-devs-cli")

	if err := tutorials.SaveIndex(cacheDir, allMeta); err != nil {
		return err
	}
	return tutorials.SaveRepoInfo(cacheDir, repoInfos)
}
```

6. Add imports: `"sync"`, `"context"`, `"golang.org/x/sync/errgroup"`, and the tutorials package. Run `go get golang.org/x/sync` if errgroup isn't already in go.mod.

- [ ] **Step 3: Verify build compiles**

Run: `go build ./... && go vet ./...`

- [ ] **Step 4: Commit**

```bash
git add internal/config/config.go cmd/sync.go
git commit -m "feat(tutorials): wire tutorials sync with TTL, incremental fetch, and enrichment"
```

---

## Task 10: i18n Keys

**Files:**
- Modify: `internal/i18n/catalogs/en.json`
- Modify: `internal/i18n/catalogs/de.json`

- [ ] **Step 1: Add English tutorial keys**

Add to `internal/i18n/catalogs/en.json` (before the closing `}`):

```json
  "tutorial.short": "Browse SAP tutorials",
  "tutorial.list.short": "List tutorials for your active profile",
  "tutorial.search.short": "Search across all SAP tutorials",
  "tutorial.show.short": "Show a tutorial in the terminal",
  "tutorial.open.short": "Open a tutorial on developers.sap.com",
  "tutorial.list.no_profile": "no profile set — run 'sap-devs profile set <name>' first",
  "tutorial.list.profile_not_found": "profile \"{{.ID}}\" not found — run 'sap-devs sync' to refresh content",
  "tutorial.none": "No tutorials found for your current profile.",
  "tutorial.none_pack": "No tutorials found for pack \"{{.Pack}}\".",
  "tutorial.none_tags": "No tutorials found for tags {{.Tags}}.",
  "tutorial.search.no_results": "No tutorials matching \"{{.Query}}\".",
  "tutorial.not_found": "Tutorial \"{{.Slug}}\" not found — run 'sap-devs tutorial search' to browse",
  "tutorial.no_index": "Tutorial index not found — run 'sap-devs sync' first",
  "tutorial.open.opening": "Opening: {{.Title}} — {{.URL}}",
  "tutorial.open.browser_fail": "Could not open browser: {{.Err}}. URL: {{.URL}}",
  "tutorial.col_slug": "SLUG",
  "tutorial.col_title": "TITLE",
  "tutorial.col_time": "TIME",
  "tutorial.col_level": "LEVEL",
  "tutorial.col_pack": "PACK"
```

- [ ] **Step 2: Add German tutorial keys**

Add equivalent German translations to `internal/i18n/catalogs/de.json`.

- [ ] **Step 3: Verify build**

Run: `go build ./... && go vet ./...`

- [ ] **Step 4: Commit**

```bash
git add internal/i18n/catalogs/
git commit -m "feat(tutorials): add i18n keys for tutorial commands"
```

---

## Task 11: CLI Commands

**Files:**
- Create: `cmd/tutorials.go`

- [ ] **Step 1: Create cmd/tutorials.go with all subcommands**

Create `cmd/tutorials.go` following the `samples.go` pattern. Implement:
- `tutorialCmd` — parent command
- `tutorialListCmd` — loads packs, joins TutorialRef with index, displays table (★ for featured)
- `tutorialSearchCmd` — loads full index, applies text search + level/tag filters
- `tutorialShowCmd` — fetches on-demand content, renders with glamour (default) or interactive TUI (`-i`)
- `tutorialOpenCmd` — deterministic URL from slug, opens browser

Flags:
- `list`: `--all`, `--pack`, `--level`, `--tags`
- `search`: `--level`, `--tags`
- `show`: `-i`/`--interactive`, `--step`

Wire in `init()`: register subcommands, add flags, `rootCmd.AddCommand(tutorialCmd)`.

The `show` command should:
- Load index to find the `TutorialMeta` for the slug
- Call `tutorials.LoadContent()` (cache hit) or `tutorials.FetchRawMarkdown()` + `tutorials.Parse()` + `tutorials.SaveContent()` (cache miss)
- Default: render full markdown with glamour, prepend metadata header
- `-i` flag: launch Bubbletea TUI (basic step navigation — this is a larger subtask, see Task 12)

For Phase 1 of this task, implement the non-interactive `show` with glamour rendering. The TUI is Task 12.

- [ ] **Step 2: Verify build**

Run: `go build ./... && go vet ./...`

- [ ] **Step 3: Manual smoke test**

Run: `go run . tutorial list` (after sync)
Run: `go run . tutorial search cap`
Run: `go run . tutorial show <slug> --dry-run` (if implemented)

- [ ] **Step 4: Commit**

```bash
git add cmd/tutorials.go
git commit -m "feat(tutorials): add tutorial list/search/show/open commands"
```

---

## Task 12: Interactive TUI for tutorial show -i

**Files:**
- Modify: `cmd/tutorials.go` (add TUI model)

- [ ] **Step 1: Implement Bubbletea TUI model**

Add a Bubbletea model to `cmd/tutorials.go` (or a separate `cmd/tutorial_tui.go` if cleaner) that:
- Shows one step at a time with header (title, step N of M, time, level)
- Keybindings: `n`/right=next, `p`/left=prev, `j`=jump prompt, `d`=mark done, `q`=quit
- Calls `tutorials.UpdateProgress()` on navigation and on `d`
- Resumes from last step via `tutorials.GetProgress()`
- Renders step content with glamour inside the TUI viewport

Follow existing Bubbletea patterns in the codebase (check `internal/ui/` for reference).

- [ ] **Step 2: Wire -i flag**

In `tutorialShowCmd`, check the `-i` flag (or `cfg.Tutorial.Interactive` config default). If set, launch the TUI model instead of the static render.

- [ ] **Step 3: Manual test**

Run: `go run . tutorial show <slug> -i`
Test: navigate steps, mark done, quit, resume.

- [ ] **Step 4: Commit**

```bash
git add cmd/tutorials.go  # or cmd/tutorial_tui.go
git commit -m "feat(tutorials): add interactive step-by-step TUI"
```

---

## Task 13: Schema and Pack Content

**Files:**
- Create: `content/schemas/tutorials.yaml.schema.json`
- Modify: `.vscode/settings.json`
- Create: `content/packs/cap/tutorials.yaml`
- Create: `content/packs/btp-core/tutorials.yaml`
- Create: `content/packs/abap/tutorials.yaml`

- [ ] **Step 1: Create JSON Schema**

Create `content/schemas/tutorials.yaml.schema.json`:

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "Pack Tutorials",
  "description": "Schema for sap-devs tutorials.yaml files (top-level array)",
  "type": "array",
  "items": {
    "type": "object",
    "required": ["slug"],
    "additionalProperties": false,
    "properties": {
      "slug": {
        "type": "string",
        "pattern": "^[a-z][a-z0-9-]*$",
        "description": "Tutorial slug matching the folder name in sap-tutorials GitHub repos"
      },
      "featured": {
        "type": "boolean",
        "default": false,
        "description": "If true, highlighted in tutorial list output"
      }
    }
  }
}
```

- [ ] **Step 2: Wire schema in .vscode/settings.json**

Add line:

```json
    "./content/schemas/tutorials.yaml.schema.json": "**/packs/*/tutorials.yaml"
```

- [ ] **Step 3: Create tutorials.yaml for CAP pack**

Create `content/packs/cap/tutorials.yaml` with 5-10 curated tutorials. Look up valid slugs from the sap-tutorials GitHub repos (CAP-related tutorials). Example:

```yaml
- slug: cap-getting-started
  featured: true
- slug: hana-cloud-cap-create-project
- slug: cap-service-deploy-to-cf
  featured: true
```

- [ ] **Step 4: Create tutorials.yaml for btp-core and abap packs**

Create `content/packs/btp-core/tutorials.yaml` and `content/packs/abap/tutorials.yaml` with 5-10 curated tutorials each. Look up valid slugs.

- [ ] **Step 5: Verify LoadPack reads them**

Run: `go build ./... && go vet ./...`

- [ ] **Step 6: Commit**

```bash
git add content/schemas/tutorials.yaml.schema.json .vscode/settings.json content/packs/*/tutorials.yaml
git commit -m "feat(tutorials): add schema, VS Code wiring, and initial curated tutorials"
```

---

## Task 14: Documentation Updates

**Files:**
- Modify: `CLAUDE.md`
- Modify: `TODO.md` (mark Phase 1 as complete)

- [ ] **Step 1: Update CLAUDE.md**

Add `tutorial` to the CLI Commands table:

```
| `tutorial` | Browse and render SAP tutorials; `tutorial list/search/show/open`; `-i` for interactive step-by-step TUI |
```

Add a section to Architecture Overview describing the tutorials pipeline (GitHub client, parser, cache, sync integration).

- [ ] **Step 2: Update TODO.md**

Mark Phase 1 tasks as completed. Leave Phase 2 and 3 as-is.

- [ ] **Step 3: Commit**

```bash
git add CLAUDE.md TODO.md
git commit -m "docs: document tutorials command and architecture"
```

---

## Task 15: Build Verification

- [ ] **Step 1: Full build and vet**

Run: `go build ./... && go vet ./...`
Expected: clean build, no errors, no vet warnings.

- [ ] **Step 2: Verify all tests pass (CI equivalent)**

Run: `go test ./internal/tutorials/... ./internal/content/... -v` (CI only; skip locally on Windows)

- [ ] **Step 3: End-to-end smoke test**

```bash
# Sync tutorials index
go run . sync --category tutorials

# List curated tutorials
go run . tutorial list

# Search full index
go run . tutorial search "getting started"

# Show a tutorial
go run . tutorial show cap-getting-started

# Interactive mode
go run . tutorial show cap-getting-started -i

# Open in browser
go run . tutorial open cap-getting-started
```

- [ ] **Step 4: Final commit if any fixups needed**

```bash
git add -A
git commit -m "fix: address tutorial feature smoke test issues"
```
