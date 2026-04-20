# Discovery Center Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `sap-devs discovery` command surfacing SAP Discovery Center missions, BTP services, and guidance framework via live OData V2 APIs with curated YAML + cache.

**Architecture:** Thin custom OData V2 client fetches from two Discovery Center endpoints (`/platformx/$batch` for missions/guidance, `/servicecatalog/` for services). Curated refs in `discovery.yaml` per pack enriched with live API data. 7-day cache TTL. Profile-aware filtering.

**Tech Stack:** Go, cobra, OData V2 (custom client), tabwriter, `github.com/pkg/browser`

**Spec:** `docs/superpowers/specs/2026-04-18-discovery-center-design.md`

**Windows note:** `go test` fails locally due to Windows Defender. Use `go build ./...` + `go vet ./...` locally. CI (ubuntu-latest) is the authoritative test runner.

---

## File Map

| File | Action | Responsibility |
|------|--------|---------------|
| `internal/discovery/types.go` | Create | All API response types + search filters |
| `internal/discovery/cache.go` | Create | Generic TTL-based cache load/save |
| `internal/discovery/client.go` | Create | OData client: CSRF, batch, direct GET |
| `internal/discovery/discovery.go` | Create | Enrich curated refs, filter, orchestrate |
| `internal/content/pack.go` | Modify | Add discovery ref types + fields to Pack; load discovery.yaml in LoadPack |
| `internal/content/merge.go` | Modify | Add merge functions for discovery refs |
| `internal/content/discovery.go` | Create | Flatten/collect helpers |
| `cmd/discovery.go` | Create | Parent + missions subcommands |
| `cmd/discovery_services.go` | Create | Services subcommands |
| `cmd/discovery_guidance.go` | Create | Guidance subcommands |
| `content/packs/base/discovery.yaml` | Create | Base pack curated missions/services/guidance |
| `content/packs/cap/discovery.yaml` | Create | CAP pack curated refs + profile filters |
| `content/packs/btp-core/discovery.yaml` | Create | BTP pack curated refs + profile filters |
| `content/packs/abap/discovery.yaml` | Create | ABAP pack curated refs + profile filters |
| `content/schemas/discovery.schema.json` | Create | YAML validation schema |
| `.vscode/settings.json` | Modify | Wire discovery schema |
| `internal/i18n/catalogs/en.json` | Modify | English i18n keys |
| `internal/i18n/catalogs/de.json` | Modify | German i18n keys |
| `cmd/sync.go` | Modify | Add discovery sync category |
| `internal/config/config.go` | Modify | Add Discovery TTL field to SyncConfig |
| `CLAUDE.md` | Modify | Document discovery command + architecture |
| `docs/content-authoring.md` | Modify | Document discovery.yaml format |

---

### Task 1: API Response Types

**Files:**
- Create: `internal/discovery/types.go`

- [ ] **Step 1: Create types.go with all API response structs**

```go
// internal/discovery/types.go
package discovery

// Mission represents a single mission from GetMissionCatalogContentV2 or search.
type Mission struct {
	ID                 int    `json:"Id"`
	Name               string `json:"Name"`
	Category           string `json:"Category"`
	SubCategory        string `json:"SubCategory"`
	Product            string `json:"Product"`
	Industry           string `json:"Industry"`
	LoB                string `json:"LoB"`
	FocusTags          string `json:"FocusTags"`
	Type               string `json:"Type"`
	PartnerCompany     string `json:"PartnerCompany"`
	ReferenceCustomers string `json:"ReferenceCustomers"`
	UCId               int    `json:"UCId"`
	UCLongDescription  string `json:"UCLongDescription"`
	UCRibbonText       string `json:"UCRibbonText"`
	Effort             string `json:"Effort"`
	MissionCount       int    `json:"MissionCount"`
}

// MissionCatalogGroup is a named group returned by GetMissionCatalogContentV2.
type MissionCatalogGroup struct {
	Name     string    `json:"name"`
	Desc     string    `json:"desc"`
	Missions []Mission `json:"missions"`
}

// Service represents a BTP service from /servicecatalog/ServiceDetailss.
type Service struct {
	ID                  string `json:"Id"`
	Name                string `json:"Name"`
	ShortName           string `json:"ShortName"`
	Category            string `json:"Category"`
	ShortDescription    string `json:"ShortDescription"`
	LicenseModelType    string `json:"LicenseModelType"`
	IsDeprecatedService bool   `json:"IsDeprecatedService"`
}

// GuidanceNode is one node in the guidance framework tree.
type GuidanceNode struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Domain   *string        `json:"domain"`
	Order    int            `json:"order"`
	Children []GuidanceNode `json:"children"`
}

// ProductCategory is one entry in the products/categories taxonomy.
type ProductCategory struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Products []ProductCategory `json:"products,omitempty"`
}

// Categories wraps the GetProductsCategories response.
type Categories struct {
	Products []ProductCategory `json:"products"`
}

// Facets wraps the GetApplicationFocusTagsIndustryLob response.
type Facets struct {
	FocusTags  []FacetItem `json:"focusTags"`
	Industries []FacetItem `json:"industries"`
	Lobs       []FacetItem `json:"lobs"`
}

// FacetItem is a single tag/industry/LOB entry.
type FacetItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// SearchFilters controls server-side filtering for GetViewFuzzySearchesCustomV3.
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

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/discovery/...`
Expected: clean build (no output)

- [ ] **Step 3: Commit**

```bash
git add internal/discovery/types.go
git commit -m "feat(discovery): add API response types for Discovery Center OData"
```

---

### Task 2: Cache Layer

**Files:**
- Create: `internal/discovery/cache.go`

- [ ] **Step 1: Create cache.go with generic TTL-based cache**

```go
// internal/discovery/cache.go
package discovery

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// LoadCache reads a cached JSON file and unmarshals it into T.
// Returns the zero value of T and false if the cache is missing or older than ttl.
func LoadCache[T any](cacheDir, name string, ttl time.Duration) (T, bool) {
	var zero T
	p := cachePath(cacheDir, name)
	info, err := os.Stat(p)
	if err != nil {
		return zero, false
	}
	if time.Since(info.ModTime()) > ttl {
		return zero, false
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return zero, false
	}
	var v T
	if err := json.Unmarshal(data, &v); err != nil {
		return zero, false
	}
	return v, true
}

// LoadCacheStale reads a cached JSON file ignoring TTL.
// Used as fallback when the network is unavailable.
func LoadCacheStale[T any](cacheDir, name string) (T, bool) {
	var zero T
	p := cachePath(cacheDir, name)
	data, err := os.ReadFile(p)
	if err != nil {
		return zero, false
	}
	var v T
	if err := json.Unmarshal(data, &v); err != nil {
		return zero, false
	}
	return v, true
}

// SaveCache marshals data to JSON and writes it to the cache directory.
func SaveCache[T any](cacheDir, name string, data T) error {
	p := cachePath(cacheDir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return os.WriteFile(p, b, 0644)
}

// CacheAge returns the age of a cache file, or -1 if it doesn't exist.
func CacheAge(cacheDir, name string) time.Duration {
	info, err := os.Stat(cachePath(cacheDir, name))
	if err != nil {
		return -1
	}
	return time.Since(info.ModTime())
}

// SearchCacheKey returns a deterministic cache name for a search query + filters.
func SearchCacheKey(query string, filters SearchFilters) string {
	raw := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%d",
		query, filters.Category, filters.Product, filters.LoB,
		filters.Industry, filters.FocusTags, filters.Partners, filters.Top)
	h := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("search-%x", h[:8])
}

func cachePath(cacheDir, name string) string {
	return filepath.Join(cacheDir, "discovery", name+".json")
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/discovery/...`
Expected: clean build

- [ ] **Step 3: Commit**

```bash
git add internal/discovery/cache.go
git commit -m "feat(discovery): add TTL-based cache layer with generics"
```

---

### Task 3: OData Client

**Files:**
- Create: `internal/discovery/client.go`

- [ ] **Step 1: Create client.go with CSRF, batch, and direct GET support**

```go
// internal/discovery/client.go
package discovery

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	baseURL           = "https://discovery-center.cloud.sap"
	platformxPath     = "/platformx/"
	servicecatalogPath = "/servicecatalog/"
)

// Client talks to the Discovery Center OData V2 services.
type Client struct {
	baseURL   string
	http      *http.Client
	csrfToken string
}

// NewClient returns a Client ready to call Discovery Center APIs.
func NewClient() *Client {
	return &Client{
		baseURL: baseURL,
		http:    &http.Client{Timeout: 15 * time.Second},
	}
}

// FetchMissions calls GetMissionCatalogContentV2 and returns grouped missions.
func (c *Client) FetchMissions() ([]MissionCatalogGroup, error) {
	raw, err := c.batchGET("GetMissionCatalogContentV2?username=''")
	if err != nil {
		return nil, fmt.Errorf("fetch missions: %w", err)
	}
	var groups []MissionCatalogGroup
	if err := json.Unmarshal(raw, &groups); err != nil {
		return nil, fmt.Errorf("parse missions: %w", err)
	}
	return groups, nil
}

// SearchMissions calls GetViewFuzzySearchesCustomV3 with the given query and filters.
func (c *Client) SearchMissions(query string, f SearchFilters) ([]Mission, error) {
	top := f.Top
	if top <= 0 {
		top = 20
	}
	q := fmt.Sprintf(
		"GetViewFuzzySearchesCustomV3?searchString='%s'&filterCategory='%s'&filterType=mission-catalog-search&filterProduct='%s'&filterLob='%s'&filterIndustry='%s'&filterFocusTags='%s'&filterPartners='%s'&filterQuickFilter=''&top='%d'",
		query, f.Category, f.Product, f.LoB, f.Industry, f.FocusTags, f.Partners, top)
	raw, err := c.batchGET(q)
	if err != nil {
		return nil, fmt.Errorf("search missions: %w", err)
	}
	var missions []Mission
	if err := json.Unmarshal(raw, &missions); err != nil {
		return nil, fmt.Errorf("parse search results: %w", err)
	}
	return missions, nil
}

// FetchCategories calls GetProductsCategories.
func (c *Client) FetchCategories() (*Categories, error) {
	raw, err := c.batchGET("GetProductsCategories?version='1'")
	if err != nil {
		return nil, fmt.Errorf("fetch categories: %w", err)
	}
	var cats Categories
	if err := json.Unmarshal(raw, &cats); err != nil {
		return nil, fmt.Errorf("parse categories: %w", err)
	}
	return &cats, nil
}

// FetchFacets calls GetApplicationFocusTagsIndustryLob.
func (c *Client) FetchFacets() (*Facets, error) {
	raw, err := c.batchGET("GetApplicationFocusTagsIndustryLob?version='1'")
	if err != nil {
		return nil, fmt.Errorf("fetch facets: %w", err)
	}
	var facets Facets
	if err := json.Unmarshal(raw, &facets); err != nil {
		return nil, fmt.Errorf("parse facets: %w", err)
	}
	return &facets, nil
}

// FetchGuidanceTree calls GetGuidanceFrameworkTree.
func (c *Client) FetchGuidanceTree() ([]GuidanceNode, error) {
	raw, err := c.batchGET("GetGuidanceFrameworkTree")
	if err != nil {
		return nil, fmt.Errorf("fetch guidance tree: %w", err)
	}
	var nodes []GuidanceNode
	if err := json.Unmarshal(raw, &nodes); err != nil {
		return nil, fmt.Errorf("parse guidance tree: %w", err)
	}
	return nodes, nil
}

// FetchGuidanceContent calls GetGuidanceFrameworkContentById for a single node.
func (c *Client) FetchGuidanceContent(id string) (string, error) {
	raw, err := c.batchGET(fmt.Sprintf("GetGuidanceFrameworkContentById?id='%s'", id))
	if err != nil {
		return "", fmt.Errorf("fetch guidance content: %w", err)
	}
	// Raw is a JSON string (the content is returned as a plain string, not an array/object).
	var content string
	if err := json.Unmarshal(raw, &content); err != nil {
		// If unmarshal as string fails, return raw bytes as string.
		return string(raw), nil
	}
	return content, nil
}

// FetchServices calls GET /servicecatalog/ServiceDetailss (no batch needed).
func (c *Client) FetchServices() ([]Service, error) {
	url := c.baseURL + servicecatalogPath +
		"ServiceDetailss?$format=json&$select=Id,Name,ShortName,Category,ShortDescription,LicenseModelType,IsDeprecatedService"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("DataServiceVersion", "2.0")
	req.Header.Set("MaxDataServiceVersion", "2.0")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch services: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch services: HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var wrapper struct {
		D struct {
			Results []Service `json:"results"`
		} `json:"d"`
	}
	if err := json.Unmarshal(body, &wrapper); err != nil {
		return nil, fmt.Errorf("parse services: %w", err)
	}
	return wrapper.D.Results, nil
}

// fetchCSRF obtains a CSRF token from the /platformx/ endpoint.
func (c *Client) fetchCSRF() error {
	req, err := http.NewRequest("HEAD", c.baseURL+platformxPath, nil)
	if err != nil {
		return err
	}
	req.Header.Set("x-csrf-token", "Fetch")
	req.Header.Set("x-requested-with", "XMLHttpRequest")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("CSRF fetch: %w", err)
	}
	resp.Body.Close()
	token := resp.Header.Get("x-csrf-token")
	if token == "" {
		return fmt.Errorf("CSRF fetch: no token in response")
	}
	c.csrfToken = token
	return nil
}

// batchGET sends a single GET inside an OData $batch request to /platformx/$batch
// and returns the unwrapped inner JSON value.
//
// The /platformx/ endpoint returns function import results as JSON strings inside
// the OData wrapper: {"d":{"FunctionName":"[{...}]"}}. This method handles the
// double-unmarshal: outer OData envelope → extract string → return raw JSON bytes.
func (c *Client) batchGET(query string) ([]byte, error) {
	if c.csrfToken == "" {
		if err := c.fetchCSRF(); err != nil {
			return nil, err
		}
	}

	boundary := fmt.Sprintf("batch_%d", time.Now().UnixNano())
	lines := []string{
		"--" + boundary,
		"Content-Type: application/http",
		"Content-Transfer-Encoding: binary",
		"",
		"GET " + query + " HTTP/1.1",
		"sap-cancel-on-close: false",
		"sap-contextid-accept: header",
		"Accept: application/json",
		"Accept-Language: en",
		"DataServiceVersion: 2.0",
		"MaxDataServiceVersion: 2.0",
		"X-Requested-With: XMLHttpRequest",
		"x-csrf-token: " + c.csrfToken,
		"",
		"",
		"--" + boundary + "--",
		"",
	}
	body := strings.Join(lines, "\r\n")

	req, err := http.NewRequest("POST", c.baseURL+platformxPath+"$batch", strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "multipart/mixed;boundary="+boundary)
	req.Header.Set("x-csrf-token", c.csrfToken)
	req.Header.Set("x-requested-with", "XMLHttpRequest")
	req.Header.Set("DataServiceVersion", "2.0")
	req.Header.Set("MaxDataServiceVersion", "2.0")
	req.Header.Set("Accept", "multipart/mixed")
	req.Header.Set("Accept-Language", "en")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("batch request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("batch request: HTTP %d", resp.StatusCode)
	}

	return extractBatchJSON(respBody)
}

// extractBatchJSON pulls the JSON value out of a multipart OData batch response.
// It finds the JSON object in the response, then unwraps the "d" envelope.
// If the value is a string (double-encoded JSON), it returns the raw inner JSON.
func extractBatchJSON(body []byte) ([]byte, error) {
	// Find the start of the JSON object in the multipart response.
	text := string(body)
	idx := strings.Index(text, "{")
	if idx < 0 {
		return nil, fmt.Errorf("no JSON found in batch response")
	}
	jsonStr := text[idx:]

	// Unmarshal the outer OData envelope: {"d": {"FunctionName": <value>}}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal([]byte(jsonStr), &envelope); err != nil {
		return nil, fmt.Errorf("parse batch envelope: %w", err)
	}
	dRaw, ok := envelope["d"]
	if !ok {
		return nil, fmt.Errorf("no 'd' key in batch response")
	}

	// The "d" value is {"FunctionName": <value>} — extract the single value.
	var inner map[string]json.RawMessage
	if err := json.Unmarshal(dRaw, &inner); err != nil {
		return nil, fmt.Errorf("parse batch inner: %w", err)
	}

	// Get the first (only) value regardless of key name.
	for _, v := range inner {
		// Check if the value is a string (double-encoded JSON).
		var s string
		if err := json.Unmarshal(v, &s); err == nil {
			// It's a string — return the inner JSON bytes.
			return []byte(s), nil
		}
		// Not a string — return the raw JSON directly.
		return v, nil
	}

	return nil, fmt.Errorf("empty batch inner response")
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/discovery/...`
Expected: clean build

- [ ] **Step 3: Commit**

```bash
git add internal/discovery/client.go
git commit -m "feat(discovery): add OData V2 client for Discovery Center APIs"
```

---

### Task 4: Content Types + Pack Integration

**Files:**
- Modify: `internal/content/pack.go` — add discovery ref types, Pack fields, and discovery.yaml loading in LoadPack
- Modify: `internal/content/merge.go` — add merge functions

- [ ] **Step 1: Add discovery ref types to pack.go**

Add after the existing `YouTubeSource` type definition in `internal/content/pack.go`:

```go
// DiscoveryYAML is the intermediate struct for unmarshaling discovery.yaml.
// Unlike most pack YAML files (top-level arrays), this is a top-level object.
type DiscoveryYAML struct {
	ProfileFilters *DiscoveryProfileFilters `yaml:"profile_filters,omitempty"`
	Missions       []DiscoveryMissionRef    `yaml:"missions,omitempty"`
	Services       []DiscoveryServiceRef    `yaml:"services,omitempty"`
	Guidance       []DiscoveryGuidanceRef   `yaml:"guidance,omitempty"`
}

// DiscoveryMissionRef is a curated mission reference in discovery.yaml.
type DiscoveryMissionRef struct {
	ID       int    `yaml:"id"`
	Name     string `yaml:"name"`
	Featured bool   `yaml:"featured,omitempty"`
	PackID   string // set at load time
}

// DiscoveryServiceRef is a curated service reference in discovery.yaml.
type DiscoveryServiceRef struct {
	ID       string `yaml:"id"`
	Name     string `yaml:"name"`
	Featured bool   `yaml:"featured,omitempty"`
	PackID   string
}

// DiscoveryGuidanceRef is a curated guidance reference in discovery.yaml.
type DiscoveryGuidanceRef struct {
	ID     string `yaml:"id"`
	Name   string `yaml:"name"`
	PackID string
}

// DiscoveryProfileFilters maps a pack to Discovery Center filter values.
type DiscoveryProfileFilters struct {
	Products   []string `yaml:"products,omitempty"`
	Categories []string `yaml:"categories,omitempty"`
	FocusTags  []string `yaml:"focus_tags,omitempty"`
}
```

Add fields to the `Pack` struct (after the `YouTubeSources`/`Videos` fields):

```go
	DiscoveryMissions []DiscoveryMissionRef
	DiscoveryServices []DiscoveryServiceRef
	DiscoveryGuidance []DiscoveryGuidanceRef
	DiscoveryFilters  *DiscoveryProfileFilters
```

- [ ] **Step 2: Add discovery.yaml loading to pack.go**

Add the following block in `LoadPack` (in `internal/content/pack.go`) after the `youtube.yaml` loading block (follow the exact same pattern):

```go
	if data, err := os.ReadFile(filepath.Join(packDir, "discovery.yaml")); err == nil {
		var disc DiscoveryYAML
		_ = yaml.Unmarshal(data, &disc)
		pack.DiscoveryMissions = disc.Missions
		for i := range pack.DiscoveryMissions {
			pack.DiscoveryMissions[i].PackID = pack.ID
		}
		pack.DiscoveryServices = disc.Services
		for i := range pack.DiscoveryServices {
			pack.DiscoveryServices[i].PackID = pack.ID
		}
		pack.DiscoveryGuidance = disc.Guidance
		for i := range pack.DiscoveryGuidance {
			pack.DiscoveryGuidance[i].PackID = pack.ID
		}
		pack.DiscoveryFilters = disc.ProfileFilters
	}
```

- [ ] **Step 3: Add merge functions to merge.go**

Add to the `MergeWith` method, after the `mergeYouTubeSources` line:

```go
	merged.DiscoveryMissions = mergeDiscoveryMissions(base.DiscoveryMissions, a.DiscoveryMissions, base.ID)
	merged.DiscoveryServices = mergeDiscoveryServices(base.DiscoveryServices, a.DiscoveryServices, base.ID)
	merged.DiscoveryGuidance = mergeDiscoveryGuidance(base.DiscoveryGuidance, a.DiscoveryGuidance, base.ID)
	if a.DiscoveryFilters != nil {
		merged.DiscoveryFilters = a.DiscoveryFilters
	}
```

Add the merge functions at the bottom of `merge.go`:

```go
func mergeDiscoveryMissions(base, additive []DiscoveryMissionRef, packID string) []DiscoveryMissionRef {
	result := make([]DiscoveryMissionRef, len(base))
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

func mergeDiscoveryServices(base, additive []DiscoveryServiceRef, packID string) []DiscoveryServiceRef {
	result := make([]DiscoveryServiceRef, len(base))
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

func mergeDiscoveryGuidance(base, additive []DiscoveryGuidanceRef, packID string) []DiscoveryGuidanceRef {
	result := make([]DiscoveryGuidanceRef, len(base))
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

- [ ] **Step 4: Verify it compiles**

Run: `go build ./internal/content/...`
Expected: clean build

- [ ] **Step 5: Commit**

```bash
git add internal/content/pack.go internal/content/loader.go internal/content/merge.go
git commit -m "feat(content): add discovery ref types, loader, and merge support"
```

---

### Task 5: Content Flatten/Collect Helpers

**Files:**
- Create: `internal/content/discovery.go`

- [ ] **Step 1: Create discovery.go with flatten and filter helpers**

```go
// internal/content/discovery.go
package content

// FlattenDiscoveryMissionRefs collects all curated mission refs from all packs.
func FlattenDiscoveryMissionRefs(packs []*Pack) []DiscoveryMissionRef {
	var out []DiscoveryMissionRef
	for _, p := range packs {
		out = append(out, p.DiscoveryMissions...)
	}
	return out
}

// FlattenDiscoveryServiceRefs collects all curated service refs from all packs.
func FlattenDiscoveryServiceRefs(packs []*Pack) []DiscoveryServiceRef {
	var out []DiscoveryServiceRef
	for _, p := range packs {
		out = append(out, p.DiscoveryServices...)
	}
	return out
}

// FlattenDiscoveryGuidanceRefs collects all curated guidance refs from all packs.
func FlattenDiscoveryGuidanceRefs(packs []*Pack) []DiscoveryGuidanceRef {
	var out []DiscoveryGuidanceRef
	for _, p := range packs {
		out = append(out, p.DiscoveryGuidance...)
	}
	return out
}

// CollectProfileFilters unions all DiscoveryProfileFilters across active packs.
func CollectProfileFilters(packs []*Pack) DiscoveryProfileFilters {
	products := make(map[string]bool)
	categories := make(map[string]bool)
	focusTags := make(map[string]bool)

	for _, p := range packs {
		if p.DiscoveryFilters == nil {
			continue
		}
		for _, v := range p.DiscoveryFilters.Products {
			products[v] = true
		}
		for _, v := range p.DiscoveryFilters.Categories {
			categories[v] = true
		}
		for _, v := range p.DiscoveryFilters.FocusTags {
			focusTags[v] = true
		}
	}

	return DiscoveryProfileFilters{
		Products:   setToSlice(products),
		Categories: setToSlice(categories),
		FocusTags:  setToSlice(focusTags),
	}
}

func setToSlice(m map[string]bool) []string {
	if len(m) == 0 {
		return nil
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/content/...`
Expected: clean build

- [ ] **Step 3: Commit**

```bash
git add internal/content/discovery.go
git commit -m "feat(content): add flatten/collect helpers for discovery refs"
```

---

### Task 6: Discovery Orchestration Layer

**Files:**
- Create: `internal/discovery/discovery.go`

- [ ] **Step 1: Create discovery.go with enrich, filter, and resolve logic**

```go
// internal/discovery/discovery.go
package discovery

import (
	"strings"
	"time"

	"github.com/SAP-samples/sap-devs-cli/internal/content"
)

const (
	CacheTTL       = 7 * 24 * time.Hour // 7 days
	SearchCacheTTL = 1 * time.Hour
)

// CategoryMapping maps pack category codes to Discovery Center service category names.
var CategoryMapping = map[string]string{
	"appdev":        "Application Development and Automation",
	"intgn":         "Integration",
	"dataanalytics": "Data and Analytics",
	"aicatg":        "Artificial Intelligence",
}

// EffortLabels maps API effort codes to display labels.
var EffortLabels = map[string]string{
	"0": "<1h",
	"1": "1h",
	"2": "2h",
	"3": "3h+",
}

// ResolveMissions loads curated refs enriched with API data.
// Curated featured refs appear first, then API missions matching profile filters.
func ResolveMissions(
	refs []content.DiscoveryMissionRef,
	filters content.DiscoveryProfileFilters,
	cacheDir string,
	force bool,
	client *Client,
) ([]Mission, error) {
	allMissions, err := loadOrFetchMissions(cacheDir, force, client)
	if err != nil {
		return nil, err
	}

	missionByID := make(map[int]Mission, len(allMissions))
	for _, m := range allMissions {
		missionByID[m.ID] = m
	}

	var result []Mission
	seen := make(map[int]bool)

	// Featured curated refs first.
	for _, ref := range refs {
		if !ref.Featured {
			continue
		}
		if m, ok := missionByID[ref.ID]; ok {
			result = append(result, m)
			seen[ref.ID] = true
		}
	}

	// Non-featured curated refs next.
	for _, ref := range refs {
		if ref.Featured || seen[ref.ID] {
			continue
		}
		if m, ok := missionByID[ref.ID]; ok {
			result = append(result, m)
			seen[ref.ID] = true
		}
	}

	// Then API missions matching profile filters, deduped.
	for _, m := range allMissions {
		if seen[m.ID] {
			continue
		}
		if matchesFilters(m, filters) {
			result = append(result, m)
			seen[m.ID] = true
		}
	}

	return result, nil
}

// ResolveServices loads services enriched by API, filtered by profile categories.
func ResolveServices(
	refs []content.DiscoveryServiceRef,
	filters content.DiscoveryProfileFilters,
	cacheDir string,
	force bool,
	showDeprecated bool,
	client *Client,
) ([]Service, error) {
	allServices, err := loadOrFetchServices(cacheDir, force, client)
	if err != nil {
		return nil, err
	}

	svcByID := make(map[string]Service, len(allServices))
	for _, s := range allServices {
		svcByID[s.ID] = s
	}

	var result []Service
	seen := make(map[string]bool)

	// Featured curated refs first.
	for _, ref := range refs {
		if !ref.Featured {
			continue
		}
		if s, ok := svcByID[ref.ID]; ok {
			if !s.IsDeprecatedService || showDeprecated {
				result = append(result, s)
				seen[ref.ID] = true
			}
		}
	}

	// Non-featured curated refs.
	for _, ref := range refs {
		if ref.Featured || seen[ref.ID] {
			continue
		}
		if s, ok := svcByID[ref.ID]; ok {
			if !s.IsDeprecatedService || showDeprecated {
				result = append(result, s)
				seen[ref.ID] = true
			}
		}
	}

	// API services matching profile category filters.
	categorySet := buildCategorySet(filters.Categories)
	for _, s := range allServices {
		if seen[s.ID] {
			continue
		}
		if s.IsDeprecatedService && !showDeprecated {
			continue
		}
		if len(categorySet) > 0 && !categorySet[s.Category] {
			continue
		}
		result = append(result, s)
		seen[s.ID] = true
	}

	return result, nil
}

// ResolveGuidanceTree loads the guidance tree, optionally filtered by domain.
func ResolveGuidanceTree(
	cacheDir string,
	force bool,
	domainFilter string,
	client *Client,
) ([]GuidanceNode, error) {
	var tree []GuidanceNode
	var ok bool
	if !force {
		tree, ok = LoadCache[[]GuidanceNode](cacheDir, "guidance-tree", CacheTTL)
	}
	if !ok {
		var err error
		tree, err = client.FetchGuidanceTree()
		if err != nil {
			if stale, staleOK := LoadCacheStale[[]GuidanceNode](cacheDir, "guidance-tree"); staleOK {
				tree = stale
			} else {
				return nil, err
			}
		} else {
			_ = SaveCache(cacheDir, "guidance-tree", tree)
		}
	}

	if domainFilter != "" {
		tree = filterGuidanceByDomain(tree, domainFilter)
	}
	return tree, nil
}

// ResolveGuidanceContent loads content for a single guidance node.
func ResolveGuidanceContent(
	cacheDir string,
	force bool,
	id string,
	client *Client,
) (string, error) {
	cacheName := "guidance/" + id
	if !force {
		if c, ok := LoadCache[string](cacheDir, cacheName, CacheTTL); ok {
			return c, nil
		}
	}
	c, err := client.FetchGuidanceContent(id)
	if err != nil {
		if stale, ok := LoadCacheStale[string](cacheDir, cacheName); ok {
			return stale, nil
		}
		return "", err
	}
	_ = SaveCache(cacheDir, cacheName, c)
	return c, nil
}

func loadOrFetchMissions(cacheDir string, force bool, client *Client) ([]Mission, error) {
	if !force {
		if cached, ok := LoadCache[[]Mission](cacheDir, "missions", CacheTTL); ok {
			return cached, nil
		}
	}
	groups, err := client.FetchMissions()
	if err != nil {
		if stale, ok := LoadCacheStale[[]Mission](cacheDir, "missions"); ok {
			return stale, nil
		}
		return nil, err
	}
	var all []Mission
	for _, g := range groups {
		all = append(all, g.Missions...)
	}
	_ = SaveCache(cacheDir, "missions", all)
	return all, nil
}

func loadOrFetchServices(cacheDir string, force bool, client *Client) ([]Service, error) {
	if !force {
		if cached, ok := LoadCache[[]Service](cacheDir, "services", CacheTTL); ok {
			return cached, nil
		}
	}
	svcs, err := client.FetchServices()
	if err != nil {
		if stale, ok := LoadCacheStale[[]Service](cacheDir, "services"); ok {
			return stale, nil
		}
		return nil, err
	}
	_ = SaveCache(cacheDir, "services", svcs)
	return svcs, nil
}

func matchesFilters(m Mission, f content.DiscoveryProfileFilters) bool {
	if len(f.Products) == 0 && len(f.Categories) == 0 && len(f.FocusTags) == 0 {
		return true
	}
	for _, p := range f.Products {
		if containsCSV(m.Product, p) {
			return true
		}
	}
	for _, c := range f.Categories {
		if containsCSV(m.Category, c) {
			return true
		}
	}
	for _, t := range f.FocusTags {
		if containsCSV(m.FocusTags, t) {
			return true
		}
	}
	return false
}

func containsCSV(csv, val string) bool {
	for _, v := range strings.Split(csv, ",") {
		if strings.TrimSpace(v) == val {
			return true
		}
	}
	return false
}

func buildCategorySet(codes []string) map[string]bool {
	if len(codes) == 0 {
		return nil
	}
	set := make(map[string]bool)
	for _, code := range codes {
		if mapped, ok := CategoryMapping[code]; ok {
			set[mapped] = true
		} else {
			set[code] = true
		}
	}
	return set
}

func filterGuidanceByDomain(nodes []GuidanceNode, domain string) []GuidanceNode {
	d := strings.ToLower(domain)
	var result []GuidanceNode
	for _, phase := range nodes {
		var children []GuidanceNode
		for _, child := range phase.Children {
			if child.Domain != nil && strings.Contains(strings.ToLower(*child.Domain), d) {
				children = append(children, child)
			}
		}
		if len(children) > 0 {
			filtered := phase
			filtered.Children = children
			result = append(result, filtered)
		}
	}
	return result
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/discovery/...`
Expected: clean build

- [ ] **Step 3: Commit**

```bash
git add internal/discovery/discovery.go
git commit -m "feat(discovery): add resolve/enrich/filter orchestration layer"
```

---

### Task 7: i18n Keys

**Files:**
- Modify: `internal/i18n/catalogs/en.json`
- Modify: `internal/i18n/catalogs/de.json`

- [ ] **Step 1: Add English i18n keys to en.json**

Add the following keys (insert alphabetically among existing keys):

```json
"discovery.short": "Browse SAP Discovery Center content",
"discovery.long": "Browse missions, BTP services, and guidance from the SAP Discovery Center.",
"discovery.missions.short": "List curated missions for your active profile",
"discovery.missions.search.short": "Search Discovery Center missions",
"discovery.missions.search.no_results": "No missions found matching \"{{.Query}}\"",
"discovery.missions.open.short": "Open a mission in the browser",
"discovery.missions.open.not_found": "Mission \"{{.ID}}\" not found.",
"discovery.missions.open.opening": "Opening: {{.Name}} — {{.URL}}",
"discovery.missions.open.browser_fail": "Could not open browser: {{.Err}}. URL: {{.URL}}",
"discovery.missions.no_missions": "No missions found. Run `sap-devs sync` to refresh content.",
"discovery.services.short": "List BTP services for your active profile",
"discovery.services.search.short": "Search BTP services",
"discovery.services.search.no_results": "No services found matching \"{{.Query}}\"",
"discovery.services.open.short": "Open a service page in the browser",
"discovery.services.open.not_found": "Service \"{{.ID}}\" not found.",
"discovery.services.open.opening": "Opening: {{.Name}} — {{.URL}}",
"discovery.services.open.browser_fail": "Could not open browser: {{.Err}}. URL: {{.URL}}",
"discovery.services.no_services": "No services found. Run `sap-devs sync` to refresh content.",
"discovery.guidance.short": "Show BTP Guidance Framework",
"discovery.guidance.show.short": "Display guidance content in the terminal",
"discovery.guidance.open.short": "Open guidance content in the browser",
"discovery.guidance.open.not_found": "Guidance node \"{{.ID}}\" not found.",
"discovery.guidance.open.opening": "Opening: {{.Name}} — {{.URL}}",
"discovery.guidance.open.browser_fail": "Could not open browser: {{.Err}}. URL: {{.URL}}",
"discovery.col_num": "#",
"discovery.col_name": "NAME",
"discovery.col_category": "CATEGORY",
"discovery.col_effort": "EFFORT",
"discovery.col_featured": "FEATURED",
"discovery.col_pricing": "PRICING",
"discovery.col_phase": "PHASE",
"discovery.col_topic": "TOPIC",
"discovery.col_domain": "DOMAIN",
"discovery.col_partner": "PARTNER",
"discovery.err_fetch": "Could not fetch Discovery Center data: {{.Err}}"
```

- [ ] **Step 2: Add German i18n keys to de.json**

Add equivalent German translations (same key names, German values). Mirror the pattern from existing German translations in the file.

- [ ] **Step 3: Verify it compiles**

Run: `go build ./internal/i18n/...`
Expected: clean build

- [ ] **Step 4: Commit**

```bash
git add internal/i18n/catalogs/en.json internal/i18n/catalogs/de.json
git commit -m "feat(i18n): add discovery command translations for en and de"
```

---

### Task 8: Missions Command

**Files:**
- Create: `cmd/discovery.go`

- [ ] **Step 1: Create discovery.go with parent command + missions subcommands**

```go
// cmd/discovery.go
package cmd

import (
	"fmt"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/pkg/browser"
	"github.com/spf13/cobra"

	"github.com/SAP-samples/sap-devs-cli/internal/config"
	"github.com/SAP-samples/sap-devs-cli/internal/content"
	"github.com/SAP-samples/sap-devs-cli/internal/discovery"
	"github.com/SAP-samples/sap-devs-cli/internal/i18n"
	"github.com/SAP-samples/sap-devs-cli/internal/xdg"
)

var discoveryCmd = &cobra.Command{
	Use:   "discovery",
	Short: i18n.T(i18n.ActiveLang, "discovery.short"),
	Long:  i18n.T(i18n.ActiveLang, "discovery.long"),
	RunE: func(cmd *cobra.Command, args []string) error {
		return discoveryMissionsCmd.RunE(cmd, args)
	},
}

var discoveryMissionsCmd = &cobra.Command{
	Use:   "missions",
	Short: i18n.T(i18n.ActiveLang, "discovery.missions.short"),
	RunE: func(cmd *cobra.Command, args []string) error {
		return discoveryMissionsListCmd.RunE(cmd, args)
	},
}

var discoveryMissionsListCmd = &cobra.Command{
	Use:   "list",
	Short: i18n.T(i18n.ActiveLang, "discovery.missions.short"),
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
		if !discoveryAll {
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

		refs := content.FlattenDiscoveryMissionRefs(packs)
		filters := content.DiscoveryProfileFilters{}
		if !discoveryAll {
			filters = content.CollectProfileFilters(packs)
		}

		client := discovery.NewClient()
		missions, err := discovery.ResolveMissions(refs, filters, paths.CacheDir, discoveryForce, client)
		if err != nil {
			return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "discovery.err_fetch", map[string]any{"Err": err}))
		}

		if len(missions) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "discovery.missions.no_missions"))
			return nil
		}

		// Apply flags.
		if discoveryCategory != "" {
			missions = filterMissionsByCategory(missions, discoveryCategory)
		}
		if discoveryProduct != "" {
			missions = filterMissionsByProduct(missions, discoveryProduct)
		}
		if discoveryEffort != "" {
			missions = filterMissionsByEffort(missions, discoveryEffort)
		}

		n := discoveryCount
		if n <= 0 || n > len(missions) {
			n = len(missions)
		}
		missions = missions[:n]

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", "#", "FEATURED", "EFFORT", "NAME", "CATEGORY")
		for i, m := range missions {
			featured := ""
			for _, ref := range refs {
				if ref.ID == m.ID && ref.Featured {
					featured = "★"
					break
				}
			}
			effort := discovery.EffortLabels[m.Effort]
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n", i+1, featured, effort, m.Name, formatCategories(m.Category))
		}
		w.Flush()
		return nil
	},
}

var discoveryMissionsSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: i18n.T(i18n.ActiveLang, "discovery.missions.search.short"),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}

		filters := discovery.SearchFilters{Top: discoveryCount}
		if !discoveryAll {
			loader, err := newContentLoader()
			if err != nil {
				return err
			}
			profileCfg, _ := config.LoadProfile(paths.ConfigDir)
			if profileCfg.ID != "" {
				if p, _ := loader.FindProfile(profileCfg.ID); p != nil {
					packs, _ := loader.LoadPacks(p, i18n.ActiveLang)
					pf := content.CollectProfileFilters(packs)
					filters.Product = joinCSV(pf.Products)
					filters.Category = joinCSV(pf.Categories)
					filters.FocusTags = joinCSV(pf.FocusTags)
				}
			}
		}
		if discoveryCategory != "" {
			filters.Category = discoveryCategory
		}

		cacheKey := discovery.SearchCacheKey(args[0], filters)
		var missions []discovery.Mission
		if !discoveryForce {
			missions, _ = discovery.LoadCache[[]discovery.Mission](paths.CacheDir, cacheKey, discovery.SearchCacheTTL)
		}
		if missions == nil {
			client := discovery.NewClient()
			missions, err = client.SearchMissions(args[0], filters)
			if err != nil {
				return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "discovery.err_fetch", map[string]any{"Err": err}))
			}
			_ = discovery.SaveCache(paths.CacheDir, cacheKey, missions)
		}

		if len(missions) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "discovery.missions.search.no_results", map[string]any{"Query": args[0]}))
			return nil
		}

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", "#", "EFFORT", "NAME", "CATEGORY", "PARTNER")
		for i, m := range missions {
			effort := discovery.EffortLabels[m.Effort]
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n", i+1, effort, m.Name, formatCategories(m.Category), m.PartnerCompany)
		}
		w.Flush()
		return nil
	},
}

var discoveryMissionsOpenCmd = &cobra.Command{
	Use:   "open <id>",
	Short: i18n.T(i18n.ActiveLang, "discovery.missions.open.short"),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}
		client := discovery.NewClient()
		allMissions, err := discovery.ResolveMissions(nil, content.DiscoveryProfileFilters{}, paths.CacheDir, discoveryForce, client)
		if err != nil {
			return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "discovery.err_fetch", map[string]any{"Err": err}))
		}

		var target *discovery.Mission
		if id, err := strconv.Atoi(args[0]); err == nil {
			for i := range allMissions {
				if allMissions[i].ID == id {
					target = &allMissions[i]
					break
				}
			}
		}
		if target == nil {
			return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "discovery.missions.open.not_found", map[string]any{"ID": args[0]}))
		}

		url := fmt.Sprintf("https://discovery-center.cloud.sap/missiondetail/%d/", target.ID)
		if err := browser.OpenURL(url); err != nil {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "discovery.missions.open.browser_fail", map[string]any{"Err": err, "URL": url}))
			return nil
		}
		fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "discovery.missions.open.opening", map[string]any{"Name": target.Name, "URL": url}))
		return nil
	},
}

// Flags
var (
	discoveryAll      bool
	discoveryForce    bool
	discoveryCount    int
	discoveryCategory string
	discoveryProduct  string
	discoveryEffort   string
)

func init() {
	// Shared flags on parent
	discoveryCmd.PersistentFlags().BoolVar(&discoveryAll, "all", false, "bypass profile filtering")
	discoveryCmd.PersistentFlags().BoolVar(&discoveryForce, "force", false, "bypass cache")
	discoveryCmd.PersistentFlags().IntVarP(&discoveryCount, "count", "n", 20, "limit results")

	// Missions flags
	discoveryMissionsCmd.PersistentFlags().StringVar(&discoveryCategory, "category", "", "filter by category code")
	discoveryMissionsCmd.PersistentFlags().StringVar(&discoveryProduct, "product", "", "filter by product")
	discoveryMissionsCmd.PersistentFlags().StringVar(&discoveryEffort, "effort", "", "filter by effort level (0-3)")

	// Also expose missions flags on parent (since parent defaults to missions list)
	discoveryCmd.Flags().StringVar(&discoveryCategory, "category", "", "filter by category code")
	discoveryCmd.Flags().StringVar(&discoveryEffort, "effort", "", "filter by effort level (0-3)")

	discoveryMissionsCmd.AddCommand(discoveryMissionsListCmd, discoveryMissionsSearchCmd, discoveryMissionsOpenCmd)
	discoveryCmd.AddCommand(discoveryMissionsCmd)
	rootCmd.AddCommand(discoveryCmd)
}

// Helpers

func filterMissionsByCategory(missions []discovery.Mission, cat string) []discovery.Mission {
	var out []discovery.Mission
	for _, m := range missions {
		if discovery.ContainsCSV(m.Category, cat) {
			out = append(out, m)
		}
	}
	return out
}

func filterMissionsByProduct(missions []discovery.Mission, product string) []discovery.Mission {
	var out []discovery.Mission
	for _, m := range missions {
		if discovery.ContainsCSV(m.Product, product) {
			out = append(out, m)
		}
	}
	return out
}

func filterMissionsByEffort(missions []discovery.Mission, effort string) []discovery.Mission {
	var out []discovery.Mission
	for _, m := range missions {
		if m.Effort == effort {
			out = append(out, m)
		}
	}
	return out
}

func formatCategories(csv string) string {
	parts := make([]string, 0)
	for _, code := range splitCSV(csv) {
		if name, ok := discovery.CategoryMapping[code]; ok {
			parts = append(parts, name)
		} else {
			parts = append(parts, code)
		}
	}
	if len(parts) > 2 {
		parts = parts[:2]
	}
	return joinCSV(parts)
}

func splitCSV(s string) []string {
	var out []string
	for _, v := range strings.Split(s, ",") {
		v = strings.TrimSpace(v)
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

func joinCSV(parts []string) string {
	return strings.Join(parts, ",")
}
```

Note: Export the `containsCSV` helper in `internal/discovery/discovery.go` by renaming it to `ContainsCSV` so the command can use it for filtering. Update the internal usage in `matchesFilters` accordingly.

- [ ] **Step 2: Export ContainsCSV in discovery.go**

In `internal/discovery/discovery.go`, rename `containsCSV` to `ContainsCSV` and update the call in `matchesFilters`.

- [ ] **Step 3: Verify it compiles**

Run: `go build ./...`
Expected: clean build

- [ ] **Step 4: Commit**

```bash
git add cmd/discovery.go internal/discovery/discovery.go
git commit -m "feat(cmd): add discovery command with missions list, search, open"
```

---

### Task 9: Services Command

**Files:**
- Create: `cmd/discovery_services.go`

- [ ] **Step 1: Create discovery_services.go**

```go
// cmd/discovery_services.go
package cmd

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/pkg/browser"
	"github.com/spf13/cobra"

	"github.com/SAP-samples/sap-devs-cli/internal/config"
	"github.com/SAP-samples/sap-devs-cli/internal/content"
	"github.com/SAP-samples/sap-devs-cli/internal/discovery"
	"github.com/SAP-samples/sap-devs-cli/internal/i18n"
	"github.com/SAP-samples/sap-devs-cli/internal/xdg"
)

var discoveryServicesCmd = &cobra.Command{
	Use:   "services",
	Short: i18n.T(i18n.ActiveLang, "discovery.services.short"),
	RunE: func(cmd *cobra.Command, args []string) error {
		return discoveryServicesListCmd.RunE(cmd, args)
	},
}

var discoveryServicesListCmd = &cobra.Command{
	Use:   "list",
	Short: i18n.T(i18n.ActiveLang, "discovery.services.short"),
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
		if !discoveryAll {
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

		refs := content.FlattenDiscoveryServiceRefs(packs)
		filters := content.DiscoveryProfileFilters{}
		if !discoveryAll {
			filters = content.CollectProfileFilters(packs)
		}

		client := discovery.NewClient()
		services, err := discovery.ResolveServices(refs, filters, paths.CacheDir, discoveryForce, svcShowDeprecated, client)
		if err != nil {
			return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "discovery.err_fetch", map[string]any{"Err": err}))
		}

		if svcCategoryFilter != "" {
			var filtered []discovery.Service
			for _, s := range services {
				if strings.Contains(strings.ToLower(s.Category), strings.ToLower(svcCategoryFilter)) {
					filtered = append(filtered, s)
				}
			}
			services = filtered
		}

		if len(services) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "discovery.services.no_services"))
			return nil
		}

		n := discoveryCount
		if n <= 0 || n > len(services) {
			n = len(services)
		}
		services = services[:n]

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", "#", "NAME", "CATEGORY", "PRICING")
		for i, s := range services {
			name := s.Name
			for _, ref := range refs {
				if ref.ID == s.ID && ref.Featured {
					name = "★ " + name
					break
				}
			}
			pricing := formatPricing(s.LicenseModelType)
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\n", i+1, name, s.Category, pricing)
		}
		w.Flush()
		return nil
	},
}

var discoveryServicesSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: i18n.T(i18n.ActiveLang, "discovery.services.search.short"),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}
		client := discovery.NewClient()
		allServices, err := discovery.ResolveServices(nil, content.DiscoveryProfileFilters{}, paths.CacheDir, discoveryForce, svcShowDeprecated, client)
		if err != nil {
			return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "discovery.err_fetch", map[string]any{"Err": err}))
		}

		q := strings.ToLower(args[0])
		var matched []discovery.Service
		for _, s := range allServices {
			if strings.Contains(strings.ToLower(s.Name), q) ||
				strings.Contains(strings.ToLower(s.ShortDescription), q) ||
				strings.Contains(strings.ToLower(s.Category), q) {
				matched = append(matched, s)
			}
		}

		if len(matched) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "discovery.services.search.no_results", map[string]any{"Query": args[0]}))
			return nil
		}

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", "#", "NAME", "CATEGORY", "PRICING")
		for i, s := range matched {
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\n", i+1, s.Name, s.Category, formatPricing(s.LicenseModelType))
		}
		w.Flush()
		return nil
	},
}

var discoveryServicesOpenCmd = &cobra.Command{
	Use:   "open <id>",
	Short: i18n.T(i18n.ActiveLang, "discovery.services.open.short"),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}
		client := discovery.NewClient()
		allServices, err := discovery.ResolveServices(nil, content.DiscoveryProfileFilters{}, paths.CacheDir, discoveryForce, true, client)
		if err != nil {
			return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "discovery.err_fetch", map[string]any{"Err": err}))
		}

		var target *discovery.Service
		for i := range allServices {
			if allServices[i].ID == args[0] || strings.EqualFold(allServices[i].ShortName, args[0]) {
				target = &allServices[i]
				break
			}
		}
		if target == nil {
			return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "discovery.services.open.not_found", map[string]any{"ID": args[0]}))
		}

		url := fmt.Sprintf("https://discovery-center.cloud.sap/serviceCatalog/%s", target.ID)
		if err := browser.OpenURL(url); err != nil {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "discovery.services.open.browser_fail", map[string]any{"Err": err, "URL": url}))
			return nil
		}
		fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "discovery.services.open.opening", map[string]any{"Name": target.Name, "URL": url}))
		return nil
	},
}

var (
	svcCategoryFilter string
	svcShowDeprecated bool
)

func init() {
	discoveryServicesCmd.PersistentFlags().StringVar(&svcCategoryFilter, "category", "", "filter by service category")
	discoveryServicesCmd.PersistentFlags().BoolVar(&svcShowDeprecated, "deprecated", false, "include deprecated services")
	discoveryServicesCmd.AddCommand(discoveryServicesListCmd, discoveryServicesSearchCmd, discoveryServicesOpenCmd)
	discoveryCmd.AddCommand(discoveryServicesCmd)
}

func formatPricing(licenseModel string) string {
	if licenseModel == "" {
		return ""
	}
	if strings.Contains(licenseModel, "free") {
		return "Free Tier"
	}
	if strings.Contains(licenseModel, "cloudcredits") || strings.Contains(licenseModel, "btpea") {
		return "Cloud Credits"
	}
	if strings.Contains(licenseModel, "subscription") {
		return "Subscription"
	}
	if strings.Contains(licenseModel, "payg") {
		return "Pay-as-you-go"
	}
	return licenseModel
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./...`
Expected: clean build

- [ ] **Step 3: Commit**

```bash
git add cmd/discovery_services.go
git commit -m "feat(cmd): add discovery services list, search, open subcommands"
```

---

### Task 10: Guidance Command

**Files:**
- Create: `cmd/discovery_guidance.go`

- [ ] **Step 1: Create discovery_guidance.go**

```go
// cmd/discovery_guidance.go
package cmd

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/pkg/browser"
	"github.com/spf13/cobra"

	"github.com/SAP-samples/sap-devs-cli/internal/discovery"
	"github.com/SAP-samples/sap-devs-cli/internal/i18n"
	"github.com/SAP-samples/sap-devs-cli/internal/xdg"
)

var discoveryGuidanceCmd = &cobra.Command{
	Use:   "guidance",
	Short: i18n.T(i18n.ActiveLang, "discovery.guidance.short"),
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}
		client := discovery.NewClient()
		tree, err := discovery.ResolveGuidanceTree(paths.CacheDir, discoveryForce, guidanceDomain, client)
		if err != nil {
			return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "discovery.err_fetch", map[string]any{"Err": err}))
		}

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "%s\t%s\t%s\n", "PHASE", "TOPIC", "DOMAIN")
		for _, phase := range tree {
			for i, child := range phase.Children {
				phaseLabel := ""
				if i == 0 {
					phaseLabel = phase.Name
				}
				domain := ""
				if child.Domain != nil {
					domain = *child.Domain
				}
				fmt.Fprintf(w, "%s\t%s\t%s\n", phaseLabel, child.Name, domain)
			}
		}
		w.Flush()
		return nil
	},
}

var discoveryGuidanceShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: i18n.T(i18n.ActiveLang, "discovery.guidance.show.short"),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}
		client := discovery.NewClient()
		content, err := discovery.ResolveGuidanceContent(paths.CacheDir, discoveryForce, args[0], client)
		if err != nil {
			return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "discovery.err_fetch", map[string]any{"Err": err}))
		}
		// Strip HTML <br> tags for terminal display.
		content = strings.ReplaceAll(content, "<br>", "\n")
		content = strings.ReplaceAll(content, "<br/>", "\n")
		content = strings.ReplaceAll(content, "<br />", "\n")
		fmt.Fprintln(cmd.OutOrStdout(), content)
		return nil
	},
}

var discoveryGuidanceOpenCmd = &cobra.Command{
	Use:   "open <id>",
	Short: i18n.T(i18n.ActiveLang, "discovery.guidance.open.short"),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		url := fmt.Sprintf("https://discovery-center.cloud.sap/guidance-framework/%s", args[0])
		if err := browser.OpenURL(url); err != nil {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "discovery.guidance.open.browser_fail", map[string]any{"Err": err, "URL": url}))
			return nil
		}
		fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "discovery.guidance.open.opening", map[string]any{"Name": args[0], "URL": url}))
		return nil
	},
}

var guidanceDomain string

func init() {
	discoveryGuidanceCmd.PersistentFlags().StringVar(&guidanceDomain, "domain", "", "filter by domain (e.g., Extensibility, Integration)")
	discoveryGuidanceCmd.AddCommand(discoveryGuidanceShowCmd, discoveryGuidanceOpenCmd)
	discoveryCmd.AddCommand(discoveryGuidanceCmd)
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./...`
Expected: clean build

- [ ] **Step 3: Commit**

```bash
git add cmd/discovery_guidance.go
git commit -m "feat(cmd): add discovery guidance tree, show, open subcommands"
```

---

### Task 11: Content YAML Files

**Files:**
- Create: `content/packs/base/discovery.yaml`
- Create: `content/packs/cap/discovery.yaml`
- Create: `content/packs/btp-core/discovery.yaml`
- Create: `content/packs/abap/discovery.yaml`

- [ ] **Step 1: Create base pack discovery.yaml**

```yaml
# content/packs/base/discovery.yaml
missions:
  - id: 3258
    name: Get Started with SAP Integration Suite
    featured: true
  - id: 4441
    name: Get Started with SAP Build Code and Joule using Generative AI
    featured: true
  - id: 4338
    name: Get Started with SAP Business AI
    featured: true

services:
  - id: 05e5c025-fcb9-4953-8489-7018aefe5aa7
    name: SAP Document Management service, application option

guidance:
  - id: discover-discover-sap-btp
    name: Discover SAP BTP
  - id: prepare-sap-btp-landscape-set-up-and-planning
    name: SAP BTP Landscape Set Up and Planning
```

- [ ] **Step 2: Create CAP pack discovery.yaml**

```yaml
# content/packs/cap/discovery.yaml
profile_filters:
  products: ["1006"]
  categories: ["appdev"]
  focus_tags: ["4"]

missions:
  - id: 4327
    name: Develop a Full-Stack CAP Application
    featured: true
  - id: 4371
    name: GenAI Mail Insights with CAP and RAG
  - id: 4064
    name: Develop a multitenant SaaS app using CAP
  - id: 4432
    name: Implement Observability in a Full-Stack CAP Application
  - id: 4426
    name: Develop a Side-by-Side CAP-Based Extension Application

services:
  - id: 73554e5a-6885-4e50-8388-2d8a5e52f3ba
    name: SAP Cloud Application Programming Model
    featured: true

guidance:
  - id: realize-application-dev-best-practices
    name: Application Development Best Practices
```

Note: The CAP service ID (`73554e5a-...`) is a placeholder. During implementation, verify the actual service ID by querying `/servicecatalog/ServiceDetailss?$filter=substringof('Cloud Application Programming',Name)&$format=json`.

- [ ] **Step 3: Create BTP pack discovery.yaml**

```yaml
# content/packs/btp-core/discovery.yaml
profile_filters:
  products: ["1006"]
  categories: ["appdev", "intgn"]

missions:
  - id: 4538
    name: Establish a Unified Joule Instance
    featured: true
  - id: 3260
    name: Process and approve your invoices with SAP Build Process Automation
  - id: 4024
    name: Keep the Core Clean Using SAP Build Apps with SAP S/4HANA

guidance:
  - id: prepare-organizational-readiness
    name: Organizational Readiness
  - id: deploy-deployment-and-delivery
    name: Solution Deployment and Delivery
  - id: run-governance-model-btp
    name: Governance Model for SAP BTP
```

- [ ] **Step 4: Create ABAP pack discovery.yaml**

```yaml
# content/packs/abap/discovery.yaml
profile_filters:
  products: ["1002", "1006"]
  categories: ["appdev"]
  focus_tags: ["17"]

missions: []

guidance:
  - id: prepare-extension-use-case-assessment
    name: Extension Use Case Assessment
  - id: prepare-extension-technology-assessment
    name: Extension Technology Assessment
```

- [ ] **Step 5: Verify packs load (build check)**

Run: `go build ./...`
Expected: clean build

- [ ] **Step 6: Commit**

```bash
git add content/packs/base/discovery.yaml content/packs/cap/discovery.yaml content/packs/btp-core/discovery.yaml content/packs/abap/discovery.yaml
git commit -m "content: add curated discovery.yaml for base, cap, btp-core, abap packs"
```

---

### Task 12: JSON Schema + VSCode Wiring

**Files:**
- Create: `content/schemas/discovery.schema.json`
- Modify: `.vscode/settings.json`

- [ ] **Step 1: Create discovery.schema.json**

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "Discovery Center References",
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
        "categories": {
          "type": "array",
          "items": { "type": "string" }
        },
        "focus_tags": {
          "type": "array",
          "items": { "type": "string" }
        }
      }
    },
    "missions": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["id", "name"],
        "additionalProperties": false,
        "properties": {
          "id": { "type": "integer" },
          "name": { "type": "string" },
          "featured": { "type": "boolean" }
        }
      }
    },
    "services": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["id", "name"],
        "additionalProperties": false,
        "properties": {
          "id": { "type": "string" },
          "name": { "type": "string" },
          "featured": { "type": "boolean" }
        }
      }
    },
    "guidance": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["id", "name"],
        "additionalProperties": false,
        "properties": {
          "id": { "type": "string" },
          "name": { "type": "string" }
        }
      }
    }
  }
}
```

- [ ] **Step 2: Wire schema in .vscode/settings.json**

Add to the `yaml.schemas` object:

```json
"./content/schemas/discovery.schema.json": "**/packs/*/discovery.yaml"
```

- [ ] **Step 3: Commit**

```bash
git add content/schemas/discovery.schema.json .vscode/settings.json
git commit -m "feat(schema): add discovery.yaml JSON schema and VSCode wiring"
```

---

### Task 13: Sync Integration

**Files:**
- Modify: `internal/config/config.go` — add Discovery TTL field
- Modify: `cmd/sync.go` — add discovery category, TTL map entry, fetch function

- [ ] **Step 1: Add Discovery field to SyncConfig**

In `internal/config/config.go`, add `Discovery` field to `SyncConfig` (after the `YouTube` field):

```go
	Discovery time.Duration `yaml:"discovery"`
```

In the `Default()` function, add the default TTL (after the `YouTube` line):

```go
	Discovery: 168 * time.Hour, // 7 days
```

- [ ] **Step 2: Add discovery to sync categories and TTL map**

In `cmd/sync.go`:

1. Add `"discovery"` to the `independentCats` slice (around line 75):

```go
independentCats := []string{"events", "youtube", "discovery"}
```

2. Add `"discovery"` to the `allCategories()` function (around line 271):

```go
func allCategories() []string {
	return []string{"tips", "tools", "resources", "context", "mcp", "advocates", "events", "youtube", "discovery"}
}
```

3. Add `"discovery"` to the TTL map (around line 70):

```go
	"events": cfg.Sync.Events, "youtube": cfg.Sync.YouTube,
	"discovery": cfg.Sync.Discovery,
```

- [ ] **Step 3: Add discovery fetch function**

Add a `runDiscoveryFetch` function following the pattern of `runYouTubeFetch`:

```go
func runDiscoveryFetch(cacheDir string, force bool) error {
	client := discovery.NewClient()

	if force || discovery.CacheAge(cacheDir, "missions") < 0 || discovery.CacheAge(cacheDir, "missions") > discovery.CacheTTL {
		groups, err := client.FetchMissions()
		if err != nil {
			fmt.Fprintf(os.Stderr, "sap-devs: fetch discovery missions: %v\n", err)
		} else {
			var all []discovery.Mission
			for _, g := range groups {
				all = append(all, g.Missions...)
			}
			_ = discovery.SaveCache(cacheDir, "missions", all)
		}
	}

	if force || discovery.CacheAge(cacheDir, "services") < 0 || discovery.CacheAge(cacheDir, "services") > discovery.CacheTTL {
		svcs, err := client.FetchServices()
		if err != nil {
			fmt.Fprintf(os.Stderr, "sap-devs: fetch discovery services: %v\n", err)
		} else {
			_ = discovery.SaveCache(cacheDir, "services", svcs)
		}
	}

	if force || discovery.CacheAge(cacheDir, "guidance-tree") < 0 || discovery.CacheAge(cacheDir, "guidance-tree") > discovery.CacheTTL {
		tree, err := client.FetchGuidanceTree()
		if err != nil {
			fmt.Fprintf(os.Stderr, "sap-devs: fetch discovery guidance: %v\n", err)
		} else {
			_ = discovery.SaveCache(cacheDir, "guidance-tree", tree)
		}
	}

	if force || discovery.CacheAge(cacheDir, "categories") < 0 || discovery.CacheAge(cacheDir, "categories") > discovery.CacheTTL {
		cats, err := client.FetchCategories()
		if err != nil {
			fmt.Fprintf(os.Stderr, "sap-devs: fetch discovery categories: %v\n", err)
		} else {
			_ = discovery.SaveCache(cacheDir, "categories", cats)
		}
	}

	return nil
}
```

- [ ] **Step 4: Call runDiscoveryFetch in the sync RunE**

Add the call alongside the existing YouTube fetch, gated by staleness check:

```go
if containsString(activeIndependent, "discovery") && (force || engine.IsStale("discovery")) {
    if err := runDiscoveryFetch(paths.CacheDir, force); err != nil {
        fmt.Fprintf(os.Stderr, "sap-devs: discovery sync: %v\n", err)
    }
    _ = engine.MarkSynced("discovery")
}
```

- [ ] **Step 5: Add discovery import**

Add `"github.com/SAP-samples/sap-devs-cli/internal/discovery"` to the imports in `cmd/sync.go`.

- [ ] **Step 6: Verify it compiles**

Run: `go build ./...`
Expected: clean build

- [ ] **Step 7: Commit**

```bash
git add internal/config/config.go cmd/sync.go
git commit -m "feat(sync): add Discovery Center data to sync flow"
```

---

### Task 14: Documentation

**Files:**
- Modify: `CLAUDE.md`
- Modify: `docs/content-authoring.md` (if exists, otherwise note in CLAUDE.md)

- [ ] **Step 1: Update CLAUDE.md CLI Commands table**

Add to the CLI Commands table:

```
| `discovery` | Browse SAP Discovery Center missions, BTP services, and guidance framework; `discovery missions list/search/open`, `discovery services list/search/open`, `discovery guidance/show/open` |
```

- [ ] **Step 2: Add Discovery section to Architecture Overview in CLAUDE.md**

Add after the "### Sync" section:

```markdown
### Discovery Center

`sap-devs discovery` ([cmd/discovery.go](cmd/discovery.go), [cmd/discovery_services.go](cmd/discovery_services.go), [cmd/discovery_guidance.go](cmd/discovery_guidance.go)) surfaces content from the SAP Discovery Center via two OData V2 services. Curated references in `discovery.yaml` per pack are enriched with live API data cached at `~/.cache/sap-devs/discovery/` (7-day TTL). The `internal/discovery` package ([internal/discovery/client.go](internal/discovery/client.go)) handles CSRF tokens, OData `$batch` requests, and the double-JSON-encoding quirk of the `/platformx/` endpoint.

Three content types: **missions** (guided learning paths), **services** (BTP service catalog), and **guidance** (BTP Guidance Framework phases). Profile-aware filtering uses `profile_filters` in `discovery.yaml` to auto-filter by product/category/focus tags.
```

- [ ] **Step 3: Update docs/content-authoring.md**

If `docs/content-authoring.md` exists, add a section documenting `discovery.yaml` format, `profile_filters`, and curated ref structure. If it doesn't exist, skip this step (the CLAUDE.md update is sufficient).

- [ ] **Step 4: Commit**

```bash
git add CLAUDE.md docs/content-authoring.md
git commit -m "docs: document discovery command and discovery.yaml format"
```

---

### Task 15: Final Build Verification + Static Analysis

- [ ] **Step 1: Full build**

Run: `go build ./...`
Expected: clean build

- [ ] **Step 2: Vet**

Run: `go vet ./...`
Expected: no issues

- [ ] **Step 3: Manual smoke test**

Run: `SAP_DEVS_DEV=1 go run . discovery missions --count 5`
Expected: table output with 5 missions (fetches from live API on first run)

Run: `SAP_DEVS_DEV=1 go run . discovery services --count 5`
Expected: table output with 5 BTP services

Run: `SAP_DEVS_DEV=1 go run . discovery guidance`
Expected: guidance tree with phases and topics

Run: `SAP_DEVS_DEV=1 go run . discovery missions search CAP`
Expected: search results for CAP-related missions

- [ ] **Step 4: Fix any issues found during smoke testing**

Address compile errors, runtime panics, or formatting problems.

- [ ] **Step 5: Final commit if fixes needed**

```bash
git add -A
git commit -m "fix: address issues found during discovery smoke testing"
```
