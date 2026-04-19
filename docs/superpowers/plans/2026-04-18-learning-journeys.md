# Learning Journeys Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `sap-devs learning` command to browse, search, show, and open SAP Learning Journeys from learning.sap.com, with pack/profile integration and context injection.

**Architecture:** Catalog-hybrid approach. Sync downloads the full catalog JSON from `learning.sap.com/service/catalog-download/json`, filters to learning journeys (~351 items), and caches the index locally. The `search` subcommand uses the live search API (`getCards`) for fuzzy matching with a local fallback. Pack integration via `learning.yaml` per pack follows the discovery pattern: profile filters + curated refs with featured flags, resolved through the three-tier algorithm (featured → pack-referenced → profile-filtered).

**Tech Stack:** Go, Cobra (CLI), `text/tabwriter` (table output), `net/http` (API calls), `encoding/json` (serialization), `pkg/browser` (open URLs), `charmbracelet/glamour` (markdown rendering)

**Spec:** `docs/superpowers/specs/2026-04-18-learning-journeys-design.md`

**Windows note:** `go test` fails locally due to Windows Defender. Use `go build ./...` and `go vet ./...` for local validation. CI (ubuntu-latest) is the authoritative test runner.

---

### Task 1: Types & Constants (`internal/learning/types.go`)

**Files:**
- Create: `internal/learning/types.go`

- [ ] **Step 1: Create the types file**

```go
package learning

import "time"

const (
	CacheTTL       = 7 * 24 * time.Hour // 7 days
	SearchCacheTTL = 1 * time.Hour

	CatalogURL = "https://learning.sap.com/service/catalog-download/json"
	SearchURL  = "https://learning.sap.com/service/learning/search/getCards"
	BaseURL    = "https://learning.sap.com/learning-journeys/"
)

// LearningJourney is the cached index entry for a single learning journey.
type LearningJourney struct {
	ObjectID        string   `json:"objectId"`
	Title           string   `json:"title"`
	Slug            string   `json:"slug"`
	Description     string   `json:"description"`
	Level           string   `json:"level"`
	DurationHours   string   `json:"durationHours"`
	Roles           []string `json:"roles"`
	Product         string   `json:"product"`
	ProductCategory string   `json:"productCategory"`
	ProductSubcat   string   `json:"productSubcat"`
	Objectives      string   `json:"objectives"`
	AvailableFrom   string   `json:"availableFrom"`
	URL             string   `json:"url"`
}

// catalogItem is the raw JSON shape from the catalog download endpoint.
type catalogItem struct {
	LearningType     string      `json:"Learning_type"`
	LearningObjectID string      `json:"Learning_object_ID"`
	Title            string      `json:"Title"`
	Description      string      `json:"Description"`
	Level            string      `json:"Level"`
	DurationInHours  string      `json:"Duration_in_hours"`
	Role             string      `json:"Role"`
	Product          string      `json:"LSC_product"`
	ProductCategory  string      `json:"LSC_product_category"`
	ProductSubcat    string      `json:"LSC_product_subcategory"`
	Objectives       string      `json:"Learning_objectives"`
	AvailableFrom    string      `json:"Content_available_from"`
	DirectLink       directLink  `json:"Direct_link"`
}

type directLink struct {
	Hyperlink string `json:"hyperlink"`
}

// searchResponse is the envelope from the getCards search API.
type searchResponse struct {
	Value searchValue `json:"value"`
}

type searchValue struct {
	Results    []searchResult `json:"results"`
	TotalCount int            `json:"totalCount"`
	NextPage   *int           `json:"nextPage"`
}

type searchResult struct {
	Title           string   `json:"title"`
	Slug            string   `json:"slug"`
	Description     string   `json:"description"`
	ExperienceLevel string   `json:"experienceLevel"`
	Duration        float64  `json:"duration"`
	Roles           []string `json:"roles"`
	ObjType         string   `json:"objType"`
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/learning/...`
Expected: success (no output)

- [ ] **Step 3: Commit**

```bash
git add internal/learning/types.go
git commit -m "feat(learning): add types and constants"
```

---

### Task 2: Cache Layer (`internal/learning/cache.go`)

**Files:**
- Create: `internal/learning/cache.go`

This reuses the same generic cache pattern as `internal/discovery/cache.go` but with `learning/`-scoped paths.

- [ ] **Step 1: Create the cache file**

```go
package learning

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// SaveIndex writes the learning journey index to cache.
func SaveIndex(cacheDir string, journeys []LearningJourney) error {
	p := indexPath(cacheDir)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}
	b, err := json.Marshal(journeys)
	if err != nil {
		return err
	}
	return os.WriteFile(p, b, 0644)
}

// LoadIndex reads the cached learning journey index.
// Returns nil, false if the cache is missing or older than ttl.
func LoadIndex(cacheDir string, ttl time.Duration) ([]LearningJourney, bool) {
	p := indexPath(cacheDir)
	info, err := os.Stat(p)
	if err != nil {
		return nil, false
	}
	if ttl > 0 && time.Since(info.ModTime()) > ttl {
		return nil, false
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, false
	}
	var v []LearningJourney
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, false
	}
	return v, true
}

// LoadIndexStale reads the cached index ignoring TTL (offline fallback).
func LoadIndexStale(cacheDir string) ([]LearningJourney, bool) {
	return LoadIndex(cacheDir, 0)
}

// IndexCacheAge returns the age of the index cache, or -1 if missing.
func IndexCacheAge(cacheDir string) time.Duration {
	info, err := os.Stat(indexPath(cacheDir))
	if err != nil {
		return -1
	}
	return time.Since(info.ModTime())
}

// SaveSearchCache writes search results with a deterministic key.
func SaveSearchCache(cacheDir, key string, results []LearningJourney) error {
	p := filepath.Join(cacheDir, "learning", key+".json")
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}
	b, err := json.Marshal(results)
	if err != nil {
		return err
	}
	return os.WriteFile(p, b, 0644)
}

// LoadSearchCache reads cached search results if fresh.
func LoadSearchCache(cacheDir, key string) ([]LearningJourney, bool) {
	p := filepath.Join(cacheDir, "learning", key+".json")
	info, err := os.Stat(p)
	if err != nil {
		return nil, false
	}
	if time.Since(info.ModTime()) > SearchCacheTTL {
		return nil, false
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, false
	}
	var v []LearningJourney
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, false
	}
	return v, true
}

// SearchCacheKey returns a deterministic cache name for a search query.
func SearchCacheKey(query string, level, role string) string {
	raw := fmt.Sprintf("%s|%s|%s", query, level, role)
	h := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("search-%x", h[:8])
}

func indexPath(cacheDir string) string {
	return filepath.Join(cacheDir, "learning", "index.json")
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/learning/...`
Expected: success

- [ ] **Step 3: Commit**

```bash
git add internal/learning/cache.go
git commit -m "feat(learning): add cache layer"
```

---

### Task 3: Catalog Fetcher (`internal/learning/catalog.go`)

**Files:**
- Create: `internal/learning/catalog.go`

Downloads the full catalog JSON, filters to `Learning_type == "Learning Journey"`, and converts to `[]LearningJourney`.

- [ ] **Step 1: Create the catalog file**

```go
package learning

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// FetchCatalog downloads the full catalog and returns only learning journeys.
func FetchCatalog() ([]LearningJourney, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(CatalogURL)
	if err != nil {
		return nil, fmt.Errorf("fetch catalog: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch catalog: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read catalog: %w", err)
	}

	var items []catalogItem
	if err := json.Unmarshal(body, &items); err != nil {
		return nil, fmt.Errorf("parse catalog: %w", err)
	}

	var journeys []LearningJourney
	for _, item := range items {
		if item.LearningType != "Learning Journey" {
			continue
		}
		j := convertCatalogItem(item)
		if j.Slug == "" {
			continue
		}
		journeys = append(journeys, j)
	}
	return journeys, nil
}

func convertCatalogItem(item catalogItem) LearningJourney {
	slug := extractSlug(item.DirectLink.Hyperlink)
	var roles []string
	for _, r := range strings.Split(item.Role, ",") {
		r = strings.TrimSpace(r)
		if r != "" {
			roles = append(roles, r)
		}
	}
	return LearningJourney{
		ObjectID:        item.LearningObjectID,
		Title:           item.Title,
		Slug:            slug,
		Description:     item.Description,
		Level:           item.Level,
		DurationHours:   item.DurationInHours,
		Roles:           roles,
		Product:         item.Product,
		ProductCategory: item.ProductCategory,
		ProductSubcat:   item.ProductSubcat,
		Objectives:      item.Objectives,
		AvailableFrom:   item.AvailableFrom,
		URL:             item.DirectLink.Hyperlink,
	}
}

func extractSlug(url string) string {
	const prefix = "https://learning.sap.com/learning-journeys/"
	if strings.HasPrefix(url, prefix) {
		return strings.TrimSuffix(strings.TrimPrefix(url, prefix), "/")
	}
	return ""
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/learning/...`
Expected: success

- [ ] **Step 3: Commit**

```bash
git add internal/learning/catalog.go
git commit -m "feat(learning): add catalog fetcher"
```

---

### Task 4: Local Search & Filters (`internal/learning/search.go`)

**Files:**
- Create: `internal/learning/search.go`

- [ ] **Step 1: Create the search file**

```go
package learning

import "strings"

// Search performs case-insensitive substring matching across title, description, slug, and product.
func Search(journeys []LearningJourney, query string) []LearningJourney {
	q := strings.ToLower(query)
	var out []LearningJourney
	for _, j := range journeys {
		if strings.Contains(strings.ToLower(j.Title), q) ||
			strings.Contains(strings.ToLower(j.Description), q) ||
			strings.Contains(strings.ToLower(j.Slug), q) ||
			strings.Contains(strings.ToLower(j.Product), q) {
			out = append(out, j)
		}
	}
	return out
}

// FilterByLevel returns journeys matching the given level (case-insensitive exact match).
func FilterByLevel(journeys []LearningJourney, level string) []LearningJourney {
	l := strings.ToUpper(level)
	var out []LearningJourney
	for _, j := range journeys {
		if strings.EqualFold(j.Level, l) {
			out = append(out, j)
		}
	}
	return out
}

// FilterByRole returns journeys where at least one role matches (case-insensitive).
func FilterByRole(journeys []LearningJourney, role string) []LearningJourney {
	r := strings.ToLower(role)
	var out []LearningJourney
	for _, j := range journeys {
		for _, jr := range j.Roles {
			if strings.EqualFold(jr, r) {
				out = append(out, j)
				break
			}
		}
	}
	return out
}

// FindBySlug returns the first journey with the given slug, or nil.
func FindBySlug(journeys []LearningJourney, slug string) *LearningJourney {
	for i := range journeys {
		if journeys[i].Slug == slug {
			return &journeys[i]
		}
	}
	return nil
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/learning/...`
Expected: success

- [ ] **Step 3: Commit**

```bash
git add internal/learning/search.go
git commit -m "feat(learning): add local search and filters"
```

---

### Task 5: Search API Client (`internal/learning/api.go`)

**Files:**
- Create: `internal/learning/api.go`

Calls the `getCards` endpoint for fuzzy server-side search. Falls back to local search.

- [ ] **Step 1: Create the API file**

```go
package learning

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// SearchAPI calls the learning.sap.com search endpoint and returns results.
func SearchAPI(query string, limit int) ([]LearningJourney, error) {
	if limit <= 0 {
		limit = 15
	}
	filters := fmt.Sprintf(`{"locale":"en-US","query":"%s"}`, query)
	types := `["learning-journey"]`

	u := fmt.Sprintf("%s(types='%s',filters='%s',sort='',limit=%d,page=1)",
		SearchURL,
		url.PathEscape(types),
		url.PathEscape(filters),
		limit,
	)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(u)
	if err != nil {
		return nil, fmt.Errorf("search API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search API: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read search response: %w", err)
	}

	var sr searchResponse
	if err := json.Unmarshal(body, &sr); err != nil {
		return nil, fmt.Errorf("parse search response: %w", err)
	}

	var results []LearningJourney
	for _, r := range sr.Value.Results {
		results = append(results, LearningJourney{
			Title:         r.Title,
			Slug:          r.Slug,
			Description:   r.Description,
			Level:         r.ExperienceLevel,
			DurationHours: strconv.FormatFloat(r.Duration, 'f', 2, 64),
			URL:           BaseURL + r.Slug,
		})
	}
	return results, nil
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/learning/...`
Expected: success

- [ ] **Step 3: Commit**

```bash
git add internal/learning/api.go
git commit -m "feat(learning): add search API client"
```

---

### Task 6: Pack Integration Types & Loading

**Files:**
- Modify: `internal/content/pack.go` (add types + loading)
- Modify: `internal/content/merge.go` (add learning refs merge)
- Create: `internal/content/learning.go`

- [ ] **Step 1: Add types to `pack.go`**

After the existing `DiscoveryProfileFilters` struct (around line 200), add the learning types:

```go
// LearningYAML is the intermediate struct for unmarshaling learning.yaml.
type LearningYAML struct {
	ProfileFilters *LearningProfileFilters `yaml:"profile_filters,omitempty"`
	Journeys       []LearningRef           `yaml:"journeys,omitempty"`
}

// LearningRef is a curated learning journey reference in learning.yaml.
type LearningRef struct {
	Slug     string `yaml:"slug"`
	Featured bool   `yaml:"featured,omitempty"`
	PackID   string // set at load time
}

// LearningProfileFilters maps a pack to learning.sap.com filter values.
type LearningProfileFilters struct {
	Products          []string `yaml:"products,omitempty"`
	ProductCategories []string `yaml:"product_categories,omitempty"`
	Roles             []string `yaml:"roles,omitempty"`
}
```

Add fields to the `Pack` struct (after `DiscoveryFilters`):

```go
LearningRefs    []LearningRef
LearningFilters *LearningProfileFilters
```

- [ ] **Step 2: Add loading in `loadPack()`**

After the `discovery.yaml` loading block (around line 380), add:

```go
if data, err := os.ReadFile(filepath.Join(packDir, "learning.yaml")); err == nil {
	var learn LearningYAML
	_ = yaml.Unmarshal(data, &learn)
	pack.LearningRefs = learn.Journeys
	for i := range pack.LearningRefs {
		pack.LearningRefs[i].PackID = pack.ID
	}
	pack.LearningFilters = learn.ProfileFilters
}
```

- [ ] **Step 3: Add merge logic in `merge.go`**

In `internal/content/merge.go`, after the `merged.DiscoveryFilters` block (around line 59), add:

```go
merged.LearningRefs = mergeLearningRefs(base.LearningRefs, a.LearningRefs, base.ID)
if a.LearningFilters != nil {
	merged.LearningFilters = a.LearningFilters
}
```

Then add the merge function at the end of the file (after `mergeDiscoveryGuidance`):

```go
// mergeLearningRefs builds a fresh []LearningRef: starts with base entries,
// replaces any entry whose Slug matches an additive entry, appends unmatched additive entries.
// PackID is re-stamped to packID on every entry in the result.
func mergeLearningRefs(base, additive []LearningRef, packID string) []LearningRef {
	result := make([]LearningRef, len(base))
	copy(result, base)
	for _, a := range additive {
		replaced := false
		for i, b := range result {
			if b.Slug == a.Slug {
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

- [ ] **Step 4: Create `internal/content/learning.go`**

```go
package content

import "strings"

// FlattenLearningRefs collects all curated learning refs from all packs.
func FlattenLearningRefs(packs []*Pack) []LearningRef {
	var out []LearningRef
	for _, p := range packs {
		out = append(out, p.LearningRefs...)
	}
	return out
}

// CollectLearningFilters unions all LearningProfileFilters across active packs.
func CollectLearningFilters(packs []*Pack) LearningProfileFilters {
	products := make(map[string]bool)
	categories := make(map[string]bool)
	roles := make(map[string]bool)

	for _, p := range packs {
		if p.LearningFilters == nil {
			continue
		}
		for _, v := range p.LearningFilters.Products {
			products[v] = true
		}
		for _, v := range p.LearningFilters.ProductCategories {
			categories[v] = true
		}
		for _, v := range p.LearningFilters.Roles {
			roles[v] = true
		}
	}

	return LearningProfileFilters{
		Products:          setToSlice(products),
		ProductCategories: setToSlice(categories),
		Roles:             setToSlice(roles),
	}
}

// MatchesLearningFilters checks if a journey matches the profile filters.
func MatchesLearningFilters(product, productCategory string, roles []string, f LearningProfileFilters) bool {
	if len(f.Products) == 0 && len(f.ProductCategories) == 0 && len(f.Roles) == 0 {
		return true
	}
	for _, fp := range f.Products {
		if strings.Contains(product, fp) {
			return true
		}
	}
	for _, fc := range f.ProductCategories {
		if strings.Contains(productCategory, fc) {
			return true
		}
	}
	for _, fr := range f.Roles {
		for _, r := range roles {
			if strings.EqualFold(r, fr) {
				return true
			}
		}
	}
	return false
}
```

Note: `setToSlice` is already defined in `internal/content/discovery.go` (same package) — no import needed.

- [ ] **Step 5: Verify it compiles**

Run: `go build ./internal/content/...`
Expected: success

- [ ] **Step 6: Commit**

```bash
git add internal/content/pack.go internal/content/merge.go internal/content/learning.go
git commit -m "feat(learning): add pack integration types, merge logic, and content helpers"
```

---

### Task 7: Config TTL (`internal/config/config.go`)

**Files:**
- Modify: `internal/config/config.go`

- [ ] **Step 1: Add Learning field to SyncConfig**

Add `Learning time.Duration \`yaml:"learning"\`` after the `Tutorials` field in the `SyncConfig` struct.

- [ ] **Step 2: Add default value**

In the `Default()` function, add `Learning: 168 * time.Hour, // 7 days` after the `Tutorials` default.

- [ ] **Step 3: Verify it compiles**

Run: `go build ./internal/config/...`
Expected: success

- [ ] **Step 4: Commit**

```bash
git add internal/config/config.go
git commit -m "feat(learning): add sync TTL config field"
```

---

### Task 8: Sync Integration (`cmd/sync.go`)

**Files:**
- Modify: `cmd/sync.go`

- [ ] **Step 1: Register "learning" in `allCategories()`**

Add `"learning"` to the returned slice:

```go
func allCategories() []string {
	return []string{"tips", "tools", "resources", "context", "mcp", "advocates", "events", "youtube", "discovery", "tutorials", "learning"}
}
```

- [ ] **Step 2: Add "learning" to the ttls map and independentCats**

In the `ttls` map definition (around line 71), add:

```go
"learning": cfg.Sync.Learning,
```

In the `independentCats` slice (line 81), add `"learning"`:

```go
independentCats := []string{"events", "youtube", "discovery", "tutorials", "learning"}
```

- [ ] **Step 3: Add sync phase after tutorials**

After the tutorials sync phase (around line 168), add:

```go
// Phase 7: Learning journeys catalog fetch
if containsString(activeIndependent, "learning") && (force || engine.IsStale("learning")) {
	if err := runLearningFetch(paths.CacheDir, force); err != nil {
		fmt.Fprintf(os.Stderr, "sap-devs: learning sync: %v\n", err)
	}
	_ = engine.MarkSynced("learning")
}
```

- [ ] **Step 4: Add `runLearningFetch()` function**

Add at the end of the file (before any existing helper functions):

```go
func runLearningFetch(cacheDir string, force bool) error {
	if !force {
		if age := learning.IndexCacheAge(cacheDir); age >= 0 && age <= learning.CacheTTL {
			return nil
		}
	}
	journeys, err := learning.FetchCatalog()
	if err != nil {
		if stale, ok := learning.LoadIndexStale(cacheDir); ok {
			_ = learning.SaveIndex(cacheDir, stale)
			return nil
		}
		return err
	}
	return learning.SaveIndex(cacheDir, journeys)
}
```

Add the import `"github.tools.sap/developer-relations/sap-devs-cli/internal/learning"` to the imports block.

- [ ] **Step 5: Verify it compiles**

Run: `go build ./cmd/...`
Expected: success

- [ ] **Step 6: Commit**

```bash
git add cmd/sync.go
git commit -m "feat(learning): add sync integration for catalog fetch"
```

---

### Task 9: i18n Keys (`internal/i18n/`)

> **Must be completed before Task 10 (CLI Commands)** — the CLI commands reference these i18n keys.

**Files:**
- Modify: `internal/i18n/catalog_en.go` (and `catalog_de.go` if it exists)

- [ ] **Step 1: Add English translations**

Find the translations map in `catalog_en.go` and add:

```go
"learning.short":        "Browse SAP Learning Journeys",
"learning.long":         "Browse, search, and open learning journeys from learning.sap.com",
"learning.list.short":   "List learning journeys from your active profile",
"learning.search.short": "Search learning journeys",
"learning.show.short":   "Show learning journey details",
"learning.open.short":   "Open a learning journey in the browser",
```

- [ ] **Step 2: Add German translations (same as English for now)**

Copy the same keys to `catalog_de.go` with English values as placeholders.

- [ ] **Step 3: Verify it compiles**

Run: `go build ./...`
Expected: success

- [ ] **Step 4: Commit**

```bash
git add internal/i18n/
git commit -m "feat(learning): add i18n translation keys"
```

---

### Task 10: CLI Commands (`cmd/learning.go`)

**Files:**
- Create: `cmd/learning.go`

- [ ] **Step 1: Create the command file**

```go
package cmd

import (
	"fmt"
	"html"
	"regexp"
	"strings"
	"text/tabwriter"

	"github.com/charmbracelet/glamour"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"

	"github.tools.sap/developer-relations/sap-devs-cli/internal/config"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/i18n"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/learning"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/xdg"
)

var learningCmd = &cobra.Command{
	Use:   "learning",
	Short: i18n.T(i18n.ActiveLang, "learning.short"),
	Long:  i18n.T(i18n.ActiveLang, "learning.long"),
	RunE: func(cmd *cobra.Command, args []string) error {
		return learningListCmd.RunE(cmd, args)
	},
}

var learningListCmd = &cobra.Command{
	Use:   "list",
	Short: i18n.T(i18n.ActiveLang, "learning.list.short"),
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}
		loader, err := newContentLoader()
		if err != nil {
			return err
		}

		var packs []*content.Pack
		if !learningAll {
			profileCfg, err := config.LoadProfile(paths.ConfigDir)
			if err == nil && profileCfg.ID != "" {
				if p, _ := loader.FindProfile(profileCfg.ID); p != nil {
					packs, err = loader.LoadPacks(p, i18n.ActiveLang)
				}
			}
		}
		if packs == nil {
			packs, err = loader.LoadPacks(nil, i18n.ActiveLang)
			if err != nil {
				return err
			}
		}

		index, ok := learning.LoadIndex(paths.CacheDir, learning.CacheTTL)
		if !ok {
			return fmt.Errorf("learning index not cached — run 'sap-devs sync' first")
		}

		refs := content.FlattenLearningRefs(packs)
		filters := content.LearningProfileFilters{}
		if !learningAll {
			filters = content.CollectLearningFilters(packs)
		}

		journeys := resolveLearningJourneys(index, refs, filters, learningAll)

		if learningPack != "" {
			journeys = filterLearningByPack(journeys, refs, learningPack)
		}
		if learningLevel != "" {
			journeys = learning.FilterByLevel(journeys, learningLevel)
		}
		if learningRole != "" {
			journeys = learning.FilterByRole(journeys, learningRole)
		}

		if len(journeys) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No learning journeys found.")
			return nil
		}

		n := learningCount
		if n <= 0 || n > len(journeys) {
			n = len(journeys)
		}
		journeys = journeys[:n]

		featuredSlugs := make(map[string]bool)
		for _, ref := range refs {
			if ref.Featured {
				featuredSlugs[ref.Slug] = true
			}
		}

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", "FEATURED", "TITLE", "LEVEL", "DURATION")
		for _, j := range journeys {
			featured := ""
			if featuredSlugs[j.Slug] {
				featured = "★"
			}
			level := formatLevel(j.Level)
			duration := formatDuration(j.DurationHours)
			title := j.Title
			if len(title) > 55 {
				title = title[:52] + "..."
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", featured, title, level, duration)
		}
		w.Flush()
		return nil
	},
}

var learningSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: i18n.T(i18n.ActiveLang, "learning.search.short"),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}

		cacheKey := learning.SearchCacheKey(args[0], learningLevel, learningRole)
		var results []learning.LearningJourney

		results, ok := learning.LoadSearchCache(paths.CacheDir, cacheKey)
		if !ok {
			results, err = learning.SearchAPI(args[0], learningCount)
			if err != nil {
				// Fallback to local search
				index, indexOK := learning.LoadIndex(paths.CacheDir, learning.CacheTTL)
				if !indexOK {
					return fmt.Errorf("search API failed and no cached index: %w", err)
				}
				results = learning.Search(index, args[0])
			} else {
				_ = learning.SaveSearchCache(paths.CacheDir, cacheKey, results)
			}
		}

		if learningLevel != "" {
			results = learning.FilterByLevel(results, learningLevel)
		}
		if learningRole != "" {
			results = learning.FilterByRole(results, learningRole)
		}

		if len(results) == 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "No learning journeys found for %q.\n", args[0])
			return nil
		}

		n := learningCount
		if n <= 0 || n > len(results) {
			n = len(results)
		}
		results = results[:n]

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", "#", "TITLE", "LEVEL", "DURATION")
		for i, j := range results {
			level := formatLevel(j.Level)
			duration := formatDuration(j.DurationHours)
			title := j.Title
			if len(title) > 55 {
				title = title[:52] + "..."
			}
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\n", i+1, title, level, duration)
		}
		w.Flush()
		return nil
	},
}

var learningShowCmd = &cobra.Command{
	Use:   "show <slug>",
	Short: i18n.T(i18n.ActiveLang, "learning.show.short"),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}

		index, ok := learning.LoadIndex(paths.CacheDir, learning.CacheTTL)
		if !ok {
			return fmt.Errorf("learning index not cached — run 'sap-devs sync' first")
		}

		j := learning.FindBySlug(index, args[0])
		if j == nil {
			return fmt.Errorf("learning journey %q not found", args[0])
		}

		var b strings.Builder
		b.WriteString(fmt.Sprintf("# %s\n\n", j.Title))
		b.WriteString(fmt.Sprintf("**Level:** %s | **Duration:** %s | **Product:** %s\n\n",
			formatLevel(j.Level), formatDuration(j.DurationHours), j.Product))
		if len(j.Roles) > 0 {
			b.WriteString(fmt.Sprintf("**Roles:** %s\n\n", strings.Join(j.Roles, ", ")))
		}
		if j.Description != "" {
			b.WriteString("## Description\n\n")
			b.WriteString(j.Description + "\n\n")
		}
		if j.Objectives != "" {
			b.WriteString("## Learning Objectives\n\n")
			b.WriteString(htmlToMarkdown(j.Objectives) + "\n\n")
		}
		b.WriteString(fmt.Sprintf("**URL:** %s\n", j.URL))

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

var learningOpenCmd = &cobra.Command{
	Use:   "open <slug>",
	Short: i18n.T(i18n.ActiveLang, "learning.open.short"),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		url := learning.BaseURL + args[0]
		if err := browser.OpenURL(url); err != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "Could not open browser. Visit: %s\n", url)
			return nil
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Opening %s\n", url)
		return nil
	},
}

// Flags
var (
	learningAll   bool
	learningPack  string
	learningLevel string
	learningRole  string
	learningCount int
)

func init() {
	learningCmd.PersistentFlags().BoolVar(&learningAll, "all", false, "bypass profile filtering")
	learningCmd.PersistentFlags().StringVar(&learningLevel, "level", "", "filter by level (beginner, intermediate, advanced)")
	learningCmd.PersistentFlags().StringVar(&learningRole, "role", "", "filter by role")
	learningCmd.PersistentFlags().IntVarP(&learningCount, "count", "n", 20, "limit results")

	learningListCmd.Flags().StringVar(&learningPack, "pack", "", "filter to a specific pack's curated journeys")

	learningCmd.AddCommand(learningListCmd, learningSearchCmd, learningShowCmd, learningOpenCmd)
	rootCmd.AddCommand(learningCmd)
}

// resolveLearningJourneys implements the three-tier resolution algorithm.
func resolveLearningJourneys(
	index []learning.LearningJourney,
	refs []content.LearningRef,
	filters content.LearningProfileFilters,
	all bool,
) []learning.LearningJourney {
	bySlug := make(map[string]learning.LearningJourney, len(index))
	for _, j := range index {
		bySlug[j.Slug] = j
	}

	var result []learning.LearningJourney
	seen := make(map[string]bool)

	// Tier 1: featured refs
	for _, ref := range refs {
		if !ref.Featured {
			continue
		}
		if j, ok := bySlug[ref.Slug]; ok && !seen[ref.Slug] {
			result = append(result, j)
			seen[ref.Slug] = true
		}
	}

	// Tier 2: non-featured pack refs
	for _, ref := range refs {
		if ref.Featured || seen[ref.Slug] {
			continue
		}
		if j, ok := bySlug[ref.Slug]; ok {
			result = append(result, j)
			seen[ref.Slug] = true
		}
	}

	// Tier 3: profile-filtered (or all if --all)
	for _, j := range index {
		if seen[j.Slug] {
			continue
		}
		if all || content.MatchesLearningFilters(j.Product, j.ProductCategory, j.Roles, filters) {
			result = append(result, j)
			seen[j.Slug] = true
		}
	}

	return result
}

func filterLearningByPack(journeys []learning.LearningJourney, refs []content.LearningRef, packID string) []learning.LearningJourney {
	slugs := make(map[string]bool)
	for _, ref := range refs {
		if ref.PackID == packID {
			slugs[ref.Slug] = true
		}
	}
	var out []learning.LearningJourney
	for _, j := range journeys {
		if slugs[j.Slug] {
			out = append(out, j)
		}
	}
	return out
}

func formatLevel(level string) string {
	switch strings.ToUpper(level) {
	case "BEGINNER":
		return "Beginner"
	case "INTERMEDIATE":
		return "Intermediate"
	case "ADVANCED":
		return "Advanced"
	default:
		return level
	}
}

func formatDuration(hours string) string {
	if hours == "" {
		return ""
	}
	return hours + " hr"
}

var reHTMLTag = regexp.MustCompile(`<[^>]+>`)

func htmlToMarkdown(s string) string {
	s = strings.ReplaceAll(s, "<li>", "- ")
	s = strings.ReplaceAll(s, "</li>", "\n")
	s = strings.ReplaceAll(s, "<br>", "\n")
	s = strings.ReplaceAll(s, "<br/>", "\n")
	s = strings.ReplaceAll(s, "<br />", "\n")
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	s = reHTMLTag.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	return strings.TrimSpace(s)
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./...`
Expected: success (may need i18n keys — see Task 11)

- [ ] **Step 3: Commit**

```bash
git add cmd/learning.go
git commit -m "feat(learning): add CLI commands (list, search, show, open)"
```

---

### Task 11: Context Injection (`internal/content/render.go`)

**Files:**
- Modify: `internal/content/pack.go` (add injection struct + field)
- Modify: `internal/content/render.go` (render featured journeys)
- Modify: `cmd/inject.go` (populate injection data from index)

The `RenderContext` function only has access to `Pack` data, but `LearningRef` only carries slug/featured/packID — no title, URL, level, or duration. The approach: add a `LearningJourneyInjection` struct to Pack that's populated from the cached index at inject time (in `cmd/inject.go`), before `RenderContext` is called.

- [ ] **Step 1: Add injection type to `pack.go`**

After the `LearningProfileFilters` struct, add:

```go
// LearningJourneyInjection is a pre-resolved learning journey for context injection.
type LearningJourneyInjection struct {
	Title    string
	URL      string
	Level    string
	Duration string
}
```

Add field to `Pack` struct (after `LearningFilters`):

```go
LearningForInject []LearningJourneyInjection // populated at inject time
```

- [ ] **Step 2: Add rendering in `render.go`**

In `RenderContext()`, after the samples/canonical patterns section (around line 73), before the final `return`:

```go
var learningRows []string
for _, p := range packs {
	for _, lj := range p.LearningForInject {
		learningRows = append(learningRows, fmt.Sprintf("| [%s](%s) | %s | %s |",
			lj.Title, lj.URL, lj.Level, lj.Duration))
	}
}
if len(learningRows) > 0 {
	b.WriteString("## Recommended Learning Journeys\n\n")
	b.WriteString("| Journey | Level | Duration |\n")
	b.WriteString("|---------|-------|----------|\n")
	for _, row := range learningRows {
		b.WriteString(row + "\n")
	}
	b.WriteString("\n")
}
```

- [ ] **Step 3: Populate `LearningForInject` in `cmd/inject.go`**

In `cmd/inject.go`, after packs are loaded at line 175 (`packs, err := loader.LoadPacks(activeProfile, i18n.ActiveLang)`) and also after the reload at line 190, but before the `dynCtx := dynamic.GatherDynamic(...)` call at line 218, insert:

```go
// Resolve featured learning journeys for injection
learningIndex, _ := learning.LoadIndex(paths.CacheDir, learning.CacheTTL)
if learningIndex != nil {
	bySlug := make(map[string]learning.LearningJourney, len(learningIndex))
	for _, j := range learningIndex {
		bySlug[j.Slug] = j
	}
	for _, p := range packs {
		for _, ref := range p.LearningRefs {
			if ref.Featured {
				if j, ok := bySlug[ref.Slug]; ok {
					p.LearningForInject = append(p.LearningForInject, content.LearningJourneyInjection{
						Title:    j.Title,
						URL:      j.URL,
						Level:    j.Level,
						Duration: j.DurationHours + " hr",
					})
				}
			}
		}
	}
}
```

Add the import `"github.tools.sap/developer-relations/sap-devs-cli/internal/learning"` to `cmd/inject.go`.

- [ ] **Step 4: Verify it compiles**

Run: `go build ./...`
Expected: success

- [ ] **Step 5: Commit**

```bash
git add internal/content/pack.go internal/content/render.go cmd/inject.go
git commit -m "feat(learning): add context injection of featured learning journeys"
```

---

### Task 12: Pack Content Files

**Files:**
- Create: `content/packs/cap/learning.yaml`
- Create: `content/packs/btp-core/learning.yaml`
- Create: `content/packs/abap/learning.yaml`
- Create: `content/schemas/learning.schema.json`
- Modify: `.vscode/settings.json`

- [ ] **Step 1: Create `content/packs/cap/learning.yaml`**

```yaml
profile_filters:
  products: ["SAP Business Technology Platform"]
  product_categories: ["Business Technology Platform"]
  roles: ["developer", "architect"]

journeys:
  - slug: developing-with-sap-cloud-application-programming-model
    featured: true
  - slug: becoming-an-sap-btp-solution-architect
    featured: true
  - slug: modernizing-integration-with-sap-integration-suite
```

Note: Verify these slugs exist on learning.sap.com. The first two are confirmed from our research; adjust if needed after running sync.

- [ ] **Step 2: Create `content/packs/btp-core/learning.yaml`**

```yaml
profile_filters:
  products: ["SAP Business Technology Platform"]
  product_categories: ["Business Technology Platform"]

journeys:
  - slug: becoming-an-sap-btp-solution-architect
    featured: true
```

- [ ] **Step 3: Create `content/packs/abap/learning.yaml`**

```yaml
profile_filters:
  product_categories: ["Application Development and Automation"]
  roles: ["developer", "consultant"]

journeys: []
```

Note: Populate with actual ABAP learning journey slugs after reviewing the catalog.

- [ ] **Step 4: Create `content/schemas/learning.schema.json`**

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "Learning Journey References",
  "description": "Schema for sap-devs learning.yaml files",
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "profile_filters": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "products": {
          "type": "array",
          "items": { "type": "string" }
        },
        "product_categories": {
          "type": "array",
          "items": { "type": "string" }
        },
        "roles": {
          "type": "array",
          "items": { "type": "string" }
        }
      }
    },
    "journeys": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["slug"],
        "additionalProperties": false,
        "properties": {
          "slug": {
            "type": "string",
            "pattern": "^[a-z][a-z0-9-]*[a-z0-9]$",
            "description": "URL slug from learning.sap.com/learning-journeys/{slug}"
          },
          "featured": {
            "type": "boolean",
            "default": false,
            "description": "If true, highlighted in list output and injected into AI context"
          }
        }
      }
    }
  }
}
```

- [ ] **Step 5: Wire schema in `.vscode/settings.json`**

Add this line to the `yaml.schemas` object:

```json
"./content/schemas/learning.schema.json": "**/packs/*/learning.yaml"
```

- [ ] **Step 6: Commit**

```bash
git add content/packs/cap/learning.yaml content/packs/btp-core/learning.yaml content/packs/abap/learning.yaml content/schemas/learning.schema.json .vscode/settings.json
git commit -m "feat(learning): add pack content files and JSON schema"
```

---

### Task 13: Update CLAUDE.md

**Files:**
- Modify: `CLAUDE.md`

- [ ] **Step 1: Add `learning` to the CLI Commands table**

In the CLI Commands table, add a row after `discovery`:

```
| `learning` | Browse SAP Learning Journeys; `learning list/search/show/open` |
```

- [ ] **Step 2: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: add learning command to CLI reference"
```

---

### Task 14: Build Verification & Smoke Test

**Files:** None (verification only)

- [ ] **Step 1: Full build check**

Run: `go build ./...`
Expected: success

- [ ] **Step 2: Static analysis**

Run: `go vet ./...`
Expected: success

- [ ] **Step 3: Smoke test — sync learning**

Run: `go run . sync --category learning --force`
Expected: catalog is fetched, filtered, and cached. No errors on stderr.

- [ ] **Step 4: Smoke test — learning list**

Run: `go run . learning list --all -n 5`
Expected: table output with 5 learning journeys showing FEATURED, TITLE, LEVEL, DURATION columns.

- [ ] **Step 5: Smoke test — learning search**

Run: `go run . learning search "btp" -n 3`
Expected: table output with search results.

- [ ] **Step 6: Smoke test — learning show**

Run: `go run . learning show becoming-an-sap-btp-solution-architect`
Expected: rendered detail view with title, level, duration, description, learning objectives.

- [ ] **Step 7: Smoke test — learning open**

Run: `go run . learning open becoming-an-sap-btp-solution-architect`
Expected: browser opens to `https://learning.sap.com/learning-journeys/becoming-an-sap-btp-solution-architect`.

- [ ] **Step 8: Smoke test — profile-filtered list**

Run: `go run . learning list -n 5` (with cap profile active)
Expected: table shows CAP-relevant journeys, featured ones marked with ★.

- [ ] **Step 9: Smoke test — inject includes learning**

Run: `SAP_DEVS_DEV=1 go run . inject --dry-run`
Expected: output contains "## Recommended Learning Journeys" section with a markdown table of featured journeys.
