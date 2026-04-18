# Events Command (Phase 1: Core) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `sap-devs events` command to browse upcoming SAP community events from RSS feeds and manually curated YAML, with geocoded location-based filtering.

**Architecture:** Two new packages (`internal/geo/` for city geocoding + Haversine distance, `internal/events/` for RSS fetching, caching, and event orchestration) feed a cobra command in `cmd/events.go`. Event types and manual instances are pack-level YAML loaded via the existing `LoadPack()` system. Sync integration populates the RSS cache. Location filtering uses an embedded ~500-city coordinate database.

**Tech Stack:** Go, Cobra, `encoding/xml`, `net/http`, `encoding/json`, tabwriter, `github.com/pkg/browser`

**Spec:** `docs/superpowers/specs/2026-04-18-events-design.md`

**Phase 2 (separate plan):** iCal export, notifications (hook + OS), config events subcommands for distances/notification preferences.

---

## File Map

| File | Action | Responsibility |
| --- | --- | --- |
| `internal/geo/distance.go` | Create | `DistanceKm()`, `IsNearby()` — Haversine formula |
| `internal/geo/cities.go` | Create | Embedded city DB, `Lookup()` |
| `internal/geo/cities.json` | Create | ~500 cities with lat/lon coordinates |
| `internal/content/pack.go` | Modify | Add `EventType`, `EventInstance` structs + fields on `Pack` + YAML loading |
| `internal/content/merge.go` | Modify | Add `mergeEventTypes()`, `mergeEventInstances()` |
| `internal/content/events.go` | Create | `FlattenEventTypes`, `FlattenEventInstances`, `FilterEventsByType`, `FindEvent` |
| `internal/events/rss.go` | Create | `FetchRSS()` — RSS fetch and parse into `[]EventInstance` |
| `internal/events/cache.go` | Create | `LoadCache()`, `SaveCache()`, `CacheAge()` |
| `internal/events/events.go` | Create | `Resolve()`, `FilterByLocation()`, `MergeAndSort()` |
| `internal/config/config.go` | Modify | Add `EventsConfig` sub-struct, `Events` field on `Config`, `Events` TTL on `SyncConfig` |
| `internal/i18n/catalogs/en.json` | Modify | Add `events.*` i18n keys |
| `internal/i18n/catalogs/de.json` | Modify | Add German `events.*` i18n keys |
| `cmd/events.go` | Create | `eventsCmd`, `eventsOpenCmd`, `eventsTypesCmd`, flags |
| `cmd/sync.go` | Modify | Add `"events"` to `allCategories()`, add events fetch phase |
| `content/packs/base/event-types.yaml` | Create | Seed: CodeJam, TechEd, Devtoberfest |
| `content/packs/base/event-instances.yaml` | Create | Seed: TechEd 2026 instances |
| `content/schemas/event-types.schema.json` | Create | JSON Schema |
| `content/schemas/event-instances.schema.json` | Create | JSON Schema |
| `.vscode/settings.json` | Modify | Wire both new schemas |
| `CLAUDE.md` | Modify | Add `events` to CLI commands table |

---

### Task 1: Geo Package — Haversine Distance

**Files:**
- Create: `internal/geo/distance.go`

- [ ] **Step 1: Create distance.go**

```go
package geo

import "math"

const earthRadiusKm = 6371.0

// DistanceKm returns the great-circle distance in kilometres between two points.
func DistanceKm(lat1, lon1, lat2, lon2 float64) float64 {
	dLat := degreesToRadians(lat2 - lat1)
	dLon := degreesToRadians(lon2 - lon1)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(degreesToRadians(lat1))*math.Cos(degreesToRadians(lat2))*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthRadiusKm * c
}

// IsNearby reports whether two points are within radiusKm of each other.
func IsNearby(lat1, lon1, lat2, lon2 float64, radiusKm float64) bool {
	return DistanceKm(lat1, lon1, lat2, lon2) <= radiusKm
}

func degreesToRadians(deg float64) float64 {
	return deg * math.Pi / 180
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./...`

- [ ] **Step 3: Commit**

```bash
git add internal/geo/distance.go
git commit -m "feat(events): add Haversine distance calculation"
```

---

### Task 2: Geo Package — City Database and Lookup

**Files:**
- Create: `internal/geo/cities.json`
- Create: `internal/geo/cities.go`

- [ ] **Step 1: Generate cities.json**

Create `internal/geo/cities.json` with ~500 cities. Each entry:

```json
[
  {"name": "Hamburg", "country": "Germany", "lat": 53.5511, "lon": 9.9937},
  {"name": "Berlin", "country": "Germany", "lat": 52.5200, "lon": 13.4050},
  ...
]
```

Focus on: major SAP event cities (European capitals, German cities, Indian tech hubs, US/Canada tech cities, Middle East, Asia-Pacific, Latin America, Africa). Include all national capitals and cities with population > 500K globally. Use a web search or known reference to compile accurate lat/lon data.

**Note for implementer:** This file is ~500 entries. Generate it programmatically or use a known city list. Accuracy matters — these coordinates drive event filtering. Use 4 decimal places for lat/lon.

- [ ] **Step 2: Create cities.go with embedded DB and Lookup**

```go
package geo

import (
	_ "embed"
	"encoding/json"
	"strings"
	"sync"
)

//go:embed cities.json
var citiesJSON []byte

type city struct {
	Name    string  `json:"name"`
	Country string  `json:"country"`
	Lat     float64 `json:"lat"`
	Lon     float64 `json:"lon"`
}

var (
	cities     []city
	citiesOnce sync.Once
)

func loadCities() {
	citiesOnce.Do(func() {
		_ = json.Unmarshal(citiesJSON, &cities)
	})
}

// Lookup resolves a location string like "Hamburg, Germany" to coordinates.
// Tries exact city+country match first, then city-only. Case-insensitive.
func Lookup(location string) (lat, lon float64, ok bool) {
	loadCities()
	loc := strings.TrimSpace(location)
	if loc == "" || strings.EqualFold(loc, "virtual") {
		return 0, 0, false
	}

	parts := strings.SplitN(loc, ",", 2)
	cityName := strings.TrimSpace(parts[0])
	countryName := ""
	if len(parts) > 1 {
		countryName = strings.TrimSpace(parts[1])
	}

	// Exact city+country match
	if countryName != "" {
		for _, c := range cities {
			if strings.EqualFold(c.Name, cityName) && strings.EqualFold(c.Country, countryName) {
				return c.Lat, c.Lon, true
			}
		}
	}

	// City-only match
	for _, c := range cities {
		if strings.EqualFold(c.Name, cityName) {
			return c.Lat, c.Lon, true
		}
	}

	return 0, 0, false
}
```

- [ ] **Step 3: Verify build**

Run: `go build ./...`

- [ ] **Step 4: Commit**

```bash
git add internal/geo/cities.json internal/geo/cities.go
git commit -m "feat(events): add embedded city database with geocoding lookup"
```

---

### Task 3: Data Model — EventType, EventInstance structs, Pack loading, merge

**Files:**
- Modify: `internal/content/pack.go`
- Modify: `internal/content/merge.go`

- [ ] **Step 1: Add EventType struct to pack.go**

Add after the `Influencer` struct:

```go
// EventType defines a category of events and its data source.
type EventType struct {
	ID           string   `yaml:"id"`
	Name         string   `yaml:"name"`
	Description  string   `yaml:"description,omitempty"`
	Source       string   `yaml:"source"`        // "rss" | "manual"
	RSSURL       string   `yaml:"rss_url,omitempty"`
	DefaultScope string   `yaml:"default_scope"`
	Tags         []string `yaml:"tags,omitempty"`
	PackID       string   // set at load time
}

// EventInstance is a specific event occurrence.
type EventInstance struct {
	ID       string   `yaml:"id"`
	Type     string   `yaml:"type"`
	Title    string   `yaml:"title"`
	DateStr  string   `yaml:"date"`
	EndDateStr string `yaml:"end_date,omitempty"`
	Location string   `yaml:"location,omitempty"`
	Scope    string   `yaml:"scope"`
	URL      string   `yaml:"url"`
	Room     string   `yaml:"room,omitempty"`
	Speaker  string   `yaml:"speaker,omitempty"`
	Tags     []string `yaml:"tags,omitempty"`
	PackID   string   // set at load time
}

// ParseDate parses the DateStr field into time.Time.
func (e *EventInstance) ParseDate() (time.Time, error) {
	return parseEventDate(e.DateStr)
}

// ParseEndDate parses the EndDateStr field into time.Time.
func (e *EventInstance) ParseEndDate() (time.Time, error) {
	if e.EndDateStr == "" {
		return time.Time{}, nil
	}
	return parseEventDate(e.EndDateStr)
}

func parseEventDate(s string) (time.Time, error) {
	for _, layout := range []string{"2006-01-02", time.RFC3339, time.RFC1123Z} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse date: %s", s)
}
```

**Note:** This uses a method-based approach instead of the spec's `time.Time` field + custom `UnmarshalYAML`. The `DateStr`/`EndDateStr` strings are the canonical stored form. `ParseDate()` is called at runtime when sorting or filtering. This avoids the complexity of custom unmarshal while keeping the YAML and JSON cache serialisation clean.

- [ ] **Step 2: Add EventTypes and EventInstances fields to Pack**

Add to the `Pack` struct after `Influencers`:

```go
EventTypes     []EventType
EventInstances []EventInstance
```

- [ ] **Step 3: Load event-types.yaml and event-instances.yaml in LoadPack()**

Add after the `influencers.yaml` loading block:

```go
if data, err := os.ReadFile(filepath.Join(packDir, "event-types.yaml")); err == nil {
	_ = yaml.Unmarshal(data, &pack.EventTypes)
	for i := range pack.EventTypes {
		pack.EventTypes[i].PackID = pack.ID
	}
}
if data, err := os.ReadFile(filepath.Join(packDir, "event-instances.yaml")); err == nil {
	_ = yaml.Unmarshal(data, &pack.EventInstances)
	for i := range pack.EventInstances {
		pack.EventInstances[i].PackID = pack.ID
	}
}
```

Also add `"fmt"` and `"time"` to the imports.

- [ ] **Step 4: Add merge functions to merge.go**

Add `mergeEventTypes()` and `mergeEventInstances()` following the `mergeHooks()` pattern. Wire both into `MergeWith()` after the `Influencers` merge line:

```go
merged.EventTypes = mergeEventTypes(base.EventTypes, a.EventTypes, base.ID)
merged.EventInstances = mergeEventInstances(base.EventInstances, a.EventInstances, base.ID)
```

```go
func mergeEventTypes(base, additive []EventType, packID string) []EventType {
	result := make([]EventType, len(base))
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

func mergeEventInstances(base, additive []EventInstance, packID string) []EventInstance {
	result := make([]EventInstance, len(base))
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

- [ ] **Step 5: Verify build**

Run: `go build ./...`

- [ ] **Step 6: Commit**

```bash
git add internal/content/pack.go internal/content/merge.go
git commit -m "feat(events): add EventType and EventInstance structs, loading, and merge"
```

---

### Task 4: Content Helpers — internal/content/events.go

**Files:**
- Create: `internal/content/events.go`

- [ ] **Step 1: Create events.go**

```go
package content

// FlattenEventTypes collects all event types from all packs.
func FlattenEventTypes(packs []*Pack) []EventType {
	var out []EventType
	for _, p := range packs {
		out = append(out, p.EventTypes...)
	}
	return out
}

// FlattenEventInstances collects all event instances from all packs.
func FlattenEventInstances(packs []*Pack) []EventInstance {
	var out []EventInstance
	for _, p := range packs {
		out = append(out, p.EventInstances...)
	}
	return out
}

// FilterEventsByType returns events matching the given type ID.
func FilterEventsByType(events []EventInstance, typeID string) []EventInstance {
	var out []EventInstance
	for _, e := range events {
		if e.Type == typeID {
			out = append(out, e)
		}
	}
	return out
}

// FindEvent returns a pointer to the first event with an exact ID match, or nil.
func FindEvent(events []EventInstance, id string) *EventInstance {
	for i := range events {
		if events[i].ID == id {
			return &events[i]
		}
	}
	return nil
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./...`

- [ ] **Step 3: Commit**

```bash
git add internal/content/events.go
git commit -m "feat(events): add content helper functions for events"
```

---

### Task 5: Events Package — RSS Fetching

**Files:**
- Create: `internal/events/rss.go`

- [ ] **Step 1: Create rss.go**

```go
package events

import (
	"crypto/sha256"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
)

const userAgent = "Mozilla/5.0 (compatible; sap-devs/1.0)"

type rssFeed struct {
	XMLName xml.Name   `xml:"rss"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Items []rssItem `xml:"item"`
}

type rssItem struct {
	Title   string `xml:"title"`
	Link    string `xml:"link"`
	PubDate string `xml:"pubDate"`
}

// FetchRSS fetches an RSS feed and returns parsed EventInstances.
func FetchRSS(rssURL, typeID, defaultScope string, timeout time.Duration) ([]content.EventInstance, error) {
	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequest(http.MethodGet, rssURL, nil) //nolint:gosec
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("events: HTTP %d fetching RSS", resp.StatusCode)
	}
	const maxBodyBytes = 1 << 20
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return nil, err
	}
	return ParseRSS(data, typeID, defaultScope)
}

// ParseRSS parses RSS XML into EventInstances.
func ParseRSS(data []byte, typeID, defaultScope string) ([]content.EventInstance, error) {
	var feed rssFeed
	if err := xml.Unmarshal(data, &feed); err != nil {
		return nil, fmt.Errorf("events: parse RSS: %w", err)
	}
	var events []content.EventInstance
	for _, item := range feed.Channel.Items {
		id := fmt.Sprintf("%s/%x", typeID, sha256.Sum256([]byte(item.Link)))[:typeIDLen(typeID)]
		dateStr := ""
		if pub, err := time.Parse(time.RFC1123Z, item.PubDate); err == nil {
			dateStr = pub.Format("2006-01-02")
		}
		events = append(events, content.EventInstance{
			ID:      id,
			Type:    typeID,
			Title:   item.Title,
			DateStr: dateStr,
			Scope:   defaultScope,
			URL:     item.Link,
		})
	}
	return events, nil
}

func typeIDLen(typeID string) int {
	return len(typeID) + 1 + 12 // "typeID/" + 12 hex chars
}
```

**Note for implementer:** The ID generation uses a SHA256 hash of the URL truncated to 12 hex chars, prefixed with the type ID. This gives deterministic, collision-resistant IDs. The `typeIDLen` helper computes the truncation point. You may need to adjust the ID generation — the key requirement is determinism from URL so the same RSS item always gets the same ID.

- [ ] **Step 2: Verify build**

Run: `go build ./...`

- [ ] **Step 3: Commit**

```bash
git add internal/events/rss.go
git commit -m "feat(events): add RSS fetching and parsing for events"
```

---

### Task 6: Events Package — Cache

**Files:**
- Create: `internal/events/cache.go`

- [ ] **Step 1: Create cache.go**

```go
package events

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
)

// LoadCache reads cached events for a given event type.
func LoadCache(cacheDir, typeID string) ([]content.EventInstance, error) {
	path := cachePath(cacheDir, typeID)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var events []content.EventInstance
	if err := json.Unmarshal(data, &events); err != nil {
		return nil, err
	}
	return events, nil
}

// SaveCache writes events to the cache file for a given event type.
func SaveCache(cacheDir, typeID string, events []content.EventInstance) error {
	dir := filepath.Join(cacheDir, "events")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.Marshal(events)
	if err != nil {
		return err
	}
	return os.WriteFile(cachePath(cacheDir, typeID), data, 0644)
}

// CacheAge returns the age of the cache file, or -1 if it doesn't exist.
func CacheAge(cacheDir, typeID string) time.Duration {
	info, err := os.Stat(cachePath(cacheDir, typeID))
	if err != nil {
		return -1
	}
	return time.Since(info.ModTime())
}

func cachePath(cacheDir, typeID string) string {
	return filepath.Join(cacheDir, "events", typeID+".json")
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./...`

- [ ] **Step 3: Commit**

```bash
git add internal/events/cache.go
git commit -m "feat(events): add file-based event cache"
```

---

### Task 7: Events Package — Resolve, Filter, MergeAndSort

**Files:**
- Create: `internal/events/events.go`

- [ ] **Step 1: Create events.go**

```go
package events

import (
	"sort"
	"strings"
	"time"

	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/geo"
)

const (
	defaultCacheTTL  = 4 * time.Hour
	liveFetchTimeout = 3 * time.Second
)

// Resolve returns events for an RSS-sourced event type.
// Checks cache freshness first; fetches live if stale; falls back to stale cache on failure.
// Pass force=true to bypass the TTL check (used by sync --force).
func Resolve(et content.EventType, cacheDir string, force bool) ([]content.EventInstance, error) {
	if et.Source != "rss" || et.RSSURL == "" {
		return nil, nil
	}

	if !force {
		age := CacheAge(cacheDir, et.ID)
		if age >= 0 && age < defaultCacheTTL {
			return LoadCache(cacheDir, et.ID)
		}
	}

	events, err := FetchRSS(et.RSSURL, et.ID, et.DefaultScope, liveFetchTimeout)
	if err == nil {
		_ = SaveCache(cacheDir, et.ID, events)
		return events, nil
	}

	if cached, cacheErr := LoadCache(cacheDir, et.ID); cacheErr == nil {
		return cached, nil
	}

	return nil, nil
}

// FilterByLocation filters events based on user location and scope-based radius thresholds.
func FilterByLocation(events []content.EventInstance, userLat, userLon float64, localRadius, regionalRadius int) []content.EventInstance {
	var out []content.EventInstance
	for _, e := range events {
		scope := strings.ToLower(e.Scope)
		if scope == "virtual" || scope == "global" {
			out = append(out, e)
			continue
		}
		if e.Location == "" || strings.EqualFold(e.Location, "virtual") {
			out = append(out, e)
			continue
		}
		eLat, eLon, ok := geo.Lookup(e.Location)
		if !ok {
			out = append(out, e) // fail-open: can't geocode → show it
			continue
		}
		switch scope {
		case "regional":
			if geo.IsNearby(userLat, userLon, eLat, eLon, float64(regionalRadius)) {
				out = append(out, e)
			}
		case "local":
			if geo.IsNearby(userLat, userLon, eLat, eLon, float64(localRadius)) {
				out = append(out, e)
			}
		default:
			out = append(out, e)
		}
	}
	return out
}

// MergeAndSort combines two event slices, deduplicates by ID, sorts by date ascending.
func MergeAndSort(a, b []content.EventInstance) []content.EventInstance {
	seen := make(map[string]bool)
	var merged []content.EventInstance
	for _, list := range [][]content.EventInstance{a, b} {
		for _, e := range list {
			if !seen[e.ID] {
				seen[e.ID] = true
				merged = append(merged, e)
			}
		}
	}
	sort.Slice(merged, func(i, j int) bool {
		return merged[i].DateStr < merged[j].DateStr
	})
	return merged
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./...`

- [ ] **Step 3: Commit**

```bash
git add internal/events/events.go
git commit -m "feat(events): add Resolve, FilterByLocation, and MergeAndSort"
```

---

### Task 8: Config — EventsConfig sub-struct

**Files:**
- Modify: `internal/config/config.go`

- [ ] **Step 1: Add EventsConfig struct and wire into Config**

Add the `EventsConfig` struct:

```go
// EventsConfig controls event filtering and notification behaviour.
type EventsConfig struct {
	LocalRadius    int    `yaml:"local_radius,omitempty"`    // km, default 200
	RegionalRadius int    `yaml:"regional_radius,omitempty"` // km, default 800
	NotifyDays     int    `yaml:"notify_days,omitempty"`     // default 7
	NotifyMethod   string `yaml:"notify_method,omitempty"`   // "hook" | "os" | "both"
}
```

Add to `Config`:

```go
Events  EventsConfig `yaml:"events,omitempty"`
```

Add `Events time.Duration` to `SyncConfig`:

```go
Events time.Duration `yaml:"events"`
```

Update `Default()` to set `Sync.Events: 4 * time.Hour`.

Add helper methods for runtime defaults:

```go
func (e EventsConfig) EffectiveLocalRadius() int {
	if e.LocalRadius > 0 {
		return e.LocalRadius
	}
	return 200
}

func (e EventsConfig) EffectiveRegionalRadius() int {
	if e.RegionalRadius > 0 {
		return e.RegionalRadius
	}
	return 800
}

func (e EventsConfig) EffectiveNotifyDays() int {
	if e.NotifyDays > 0 {
		return e.NotifyDays
	}
	return 7
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./...`

- [ ] **Step 3: Commit**

```bash
git add internal/config/config.go
git commit -m "feat(events): add EventsConfig sub-struct with distance and notification settings"
```

---

### Task 9: i18n Keys

**Files:**
- Modify: `internal/i18n/catalogs/en.json`
- Modify: `internal/i18n/catalogs/de.json`

- [ ] **Step 1: Add English i18n keys**

Add after the `influencers.*` block, before `version.*`:

```json
"events.short": "Browse upcoming SAP community events",
"events.long": "Browse upcoming SAP community events. Filters by your location when configured.",
"events.none": "No upcoming events found.",
"events.none_type": "No events found for type \"{{.Type}}\".",
"events.not_found": "Event \"{{.ID}}\" not found.",
"events.open.short": "Open an event URL in the browser",
"events.open.browser_fail": "Could not open browser: {{.Err}}. URL: {{.URL}}",
"events.open.opening": "Opening: {{.Title}} — {{.URL}}",
"events.types.short": "List available event types",
"events.types.none": "No event types defined.",
"events.col_date": "DATE",
"events.col_type": "TYPE",
"events.col_scope": "SCOPE",
"events.col_location": "LOCATION",
"events.col_title": "TITLE",
"events.types.col_id": "ID",
"events.types.col_source": "SOURCE",
"events.types.col_name": "NAME",
```

- [ ] **Step 2: Add German i18n keys**

```json
"events.short": "Kommende SAP-Community-Veranstaltungen durchsuchen",
"events.long": "Kommende SAP-Community-Veranstaltungen durchsuchen. Filtert nach deinem Standort, wenn konfiguriert.",
"events.none": "Keine kommenden Veranstaltungen gefunden.",
"events.none_type": "Keine Veranstaltungen für Typ \"{{.Type}}\" gefunden.",
"events.not_found": "Veranstaltung \"{{.ID}}\" nicht gefunden.",
"events.open.short": "Veranstaltungs-URL im Browser öffnen",
"events.open.browser_fail": "Browser konnte nicht geöffnet werden: {{.Err}}. URL: {{.URL}}",
"events.open.opening": "Öffne: {{.Title}} — {{.URL}}",
"events.types.short": "Verfügbare Veranstaltungstypen auflisten",
"events.types.none": "Keine Veranstaltungstypen definiert.",
"events.col_date": "DATUM",
"events.col_type": "TYP",
"events.col_scope": "BEREICH",
"events.col_location": "ORT",
"events.col_title": "TITEL",
"events.types.col_id": "ID",
"events.types.col_source": "QUELLE",
"events.types.col_name": "NAME",
```

- [ ] **Step 3: Verify build**

Run: `go build ./...`

- [ ] **Step 4: Commit**

```bash
git add internal/i18n/catalogs/en.json internal/i18n/catalogs/de.json
git commit -m "feat(events): add i18n keys for events command (en, de)"
```

---

### Task 10: Command Implementation — cmd/events.go

**Files:**
- Create: `cmd/events.go`

- [ ] **Step 1: Create cmd/events.go**

```go
package cmd

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/config"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/events"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/geo"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/i18n"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/xdg"
)

var (
	eventsAll   bool
	eventsType  string
	eventsCount int
)

var eventsCmd = &cobra.Command{
	Use:   "events",
	Short: "Browse upcoming SAP community events",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}
		cfg, err := config.Load(paths.ConfigDir)
		if err != nil {
			return err
		}
		loader, err := newContentLoader()
		if err != nil {
			return err
		}
		packs, err := loader.LoadPacks(nil, i18n.ActiveLang)
		if err != nil {
			return err
		}

		eventTypes := content.FlattenEventTypes(packs)
		var allEvents []content.EventInstance

		// Resolve RSS-sourced types
		for _, et := range eventTypes {
			if et.Source == "rss" {
				resolved, _ := events.Resolve(et, paths.CacheDir, false)
				allEvents = append(allEvents, resolved...)
			}
		}

		// Add manual instances
		manual := content.FlattenEventInstances(packs)
		allEvents = events.MergeAndSort(allEvents, manual)

		// Filter by type
		if eventsType != "" {
			allEvents = content.FilterEventsByType(allEvents, eventsType)
		}

		// Location filter
		if !eventsAll && cfg.Location != "" {
			userLat, userLon, ok := geo.Lookup(cfg.Location)
			if ok {
				allEvents = events.FilterByLocation(allEvents, userLat, userLon,
					cfg.Events.EffectiveLocalRadius(), cfg.Events.EffectiveRegionalRadius())
			}
		}

		if len(allEvents) == 0 {
			if eventsType != "" {
				fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "events.none_type", map[string]any{"Type": eventsType}))
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "events.none"))
			}
			return nil
		}

		// Limit
		if eventsCount > 0 && len(allEvents) > eventsCount {
			allEvents = allEvents[:eventsCount]
		}

		printEventTable(cmd, allEvents)
		return nil
	},
}

var eventsOpenCmd = &cobra.Command{
	Use:   "open <id>",
	Short: "Open an event URL in the browser",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}
		loader, err := newContentLoader()
		if err != nil {
			return err
		}
		packs, err := loader.LoadPacks(nil, i18n.ActiveLang)
		if err != nil {
			return err
		}

		eventTypes := content.FlattenEventTypes(packs)
		var allEvents []content.EventInstance
		for _, et := range eventTypes {
			if et.Source == "rss" {
				resolved, _ := events.Resolve(et, paths.CacheDir, false)
				allEvents = append(allEvents, resolved...)
			}
		}
		manual := content.FlattenEventInstances(packs)
		allEvents = events.MergeAndSort(allEvents, manual)

		e := content.FindEvent(allEvents, args[0])
		if e == nil {
			return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "events.not_found", map[string]any{"ID": args[0]}))
		}
		if err := browser.OpenURL(e.URL); err != nil {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "events.open.browser_fail", map[string]any{"Err": err, "URL": e.URL}))
			return nil
		}
		fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "events.open.opening", map[string]any{"Title": e.Title, "URL": e.URL}))
		return nil
	},
}

var eventsTypesCmd = &cobra.Command{
	Use:   "types",
	Short: "List available event types",
	RunE: func(cmd *cobra.Command, args []string) error {
		loader, err := newContentLoader()
		if err != nil {
			return err
		}
		packs, err := loader.LoadPacks(nil, i18n.ActiveLang)
		if err != nil {
			return err
		}
		types := content.FlattenEventTypes(packs)
		if len(types) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "events.types.none"))
			return nil
		}
		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "%s\t%s\t%s\n",
			i18n.T(i18n.ActiveLang, "events.types.col_id"),
			i18n.T(i18n.ActiveLang, "events.types.col_source"),
			i18n.T(i18n.ActiveLang, "events.types.col_name"),
		)
		for _, et := range types {
			fmt.Fprintf(w, "%s\t%s\t%s\n", et.ID, et.Source, et.Name)
		}
		w.Flush()
		return nil
	},
}

func printEventTable(cmd *cobra.Command, evts []content.EventInstance) {
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
		i18n.T(i18n.ActiveLang, "events.col_date"),
		i18n.T(i18n.ActiveLang, "events.col_type"),
		i18n.T(i18n.ActiveLang, "events.col_scope"),
		i18n.T(i18n.ActiveLang, "events.col_location"),
		i18n.T(i18n.ActiveLang, "events.col_title"),
	)
	for _, e := range evts {
		date := e.DateStr
		if len(date) > 10 {
			date = date[:10]
		}
		loc := e.Location
		if loc == "" {
			loc = "virtual"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", date, e.Type, e.Scope, loc, e.Title)
	}
	w.Flush()
}

func init() {
	eventsCmd.Flags().BoolVarP(&eventsAll, "all", "a", false, "show all events regardless of location")
	eventsCmd.Flags().StringVarP(&eventsType, "type", "t", "", "filter by event type ID")
	eventsCmd.Flags().IntVarP(&eventsCount, "count", "n", 10, "max events to display")
	eventsCmd.AddCommand(eventsOpenCmd, eventsTypesCmd)
	rootCmd.AddCommand(eventsCmd)
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./...`

- [ ] **Step 3: Commit**

```bash
git add cmd/events.go
git commit -m "feat(events): add events command with list, open, types, and filtering"
```

---

### Task 11: Seed Data and Schemas

**Files:**
- Create: `content/packs/base/event-types.yaml`
- Create: `content/packs/base/event-instances.yaml`
- Create: `content/schemas/event-types.schema.json`
- Create: `content/schemas/event-instances.schema.json`
- Modify: `.vscode/settings.json`

- [ ] **Step 1: Create content/packs/base/event-types.yaml**

```yaml
- id: codejam
  name: SAP CodeJam
  description: Hands-on workshop series hosted by SAP developer advocates
  source: rss
  rss_url: https://community.sap.com/t5/sap-codejam/bg-p/code-jam/rss
  default_scope: local
  tags: [workshop, hands-on, in-person]

- id: teched
  name: SAP TechEd
  description: Annual SAP technology conference
  source: manual
  default_scope: regional
  tags: [conference, annual]

- id: devtoberfest
  name: Devtoberfest
  description: Annual month-long SAP developer learning event
  source: rss
  rss_url: https://community.sap.com/t5/devtoberfest/bg-p/devtoberfest/rss
  default_scope: global
  tags: [learning, virtual, october]
```

Note for implementer: verify the RSS URLs are correct by fetching them manually. The SAP Community RSS URL pattern is `https://community.sap.com/t5/<board>/bg-p/<section>/rss`. Adjust if the actual board slugs differ.

- [ ] **Step 2: Create content/packs/base/event-instances.yaml**

```yaml
- id: teched-2026-bangalore
  type: teched
  title: SAP TechEd 2026 Bangalore
  date: "2026-10-21"
  end_date: "2026-10-23"
  location: Bangalore, India
  scope: regional
  url: https://www.sap.com/events/teched.html
  tags: [in-person, asia]

- id: teched-2026-virtual
  type: teched
  title: SAP TechEd 2026 Virtual
  date: "2026-11-05"
  end_date: "2026-11-06"
  location: virtual
  scope: virtual
  url: https://www.sap.com/events/teched.html
  tags: [virtual, global]
```

Note for implementer: these are placeholder dates. Update with actual TechEd 2026 dates when known.

- [ ] **Step 3: Create content/schemas/event-types.schema.json**

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "Event Types",
  "description": "Schema for sap-devs event-types.yaml files",
  "type": "array",
  "items": {
    "type": "object",
    "required": ["id", "name", "source", "default_scope"],
    "additionalProperties": false,
    "properties": {
      "id": { "type": "string", "pattern": "^[a-z0-9]+(-[a-z0-9]+)*$" },
      "name": { "type": "string" },
      "description": { "type": "string" },
      "source": { "type": "string", "enum": ["rss", "manual"] },
      "rss_url": { "type": "string", "format": "uri" },
      "default_scope": { "type": "string", "enum": ["local", "regional", "global", "virtual"] },
      "tags": { "type": "array", "items": { "type": "string" } }
    }
  }
}
```

- [ ] **Step 4: Create content/schemas/event-instances.schema.json**

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "Event Instances",
  "description": "Schema for sap-devs event-instances.yaml files",
  "type": "array",
  "items": {
    "type": "object",
    "required": ["id", "type", "title", "date", "url", "scope"],
    "additionalProperties": false,
    "properties": {
      "id": { "type": "string" },
      "type": { "type": "string" },
      "title": { "type": "string" },
      "date": { "type": "string", "pattern": "^\\d{4}-\\d{2}-\\d{2}" },
      "end_date": { "type": "string", "pattern": "^\\d{4}-\\d{2}-\\d{2}" },
      "location": { "type": "string" },
      "scope": { "type": "string", "enum": ["local", "regional", "global", "virtual"] },
      "url": { "type": "string", "format": "uri" },
      "room": { "type": "string" },
      "speaker": { "type": "string" },
      "tags": { "type": "array", "items": { "type": "string" } }
    }
  }
}
```

- [ ] **Step 5: Wire schemas in .vscode/settings.json**

Add to the `yaml.schemas` object:

```json
"./content/schemas/event-types.schema.json": "**/packs/*/event-types.yaml",
"./content/schemas/event-instances.schema.json": "**/packs/*/event-instances.yaml"
```

- [ ] **Step 6: Verify build**

Run: `go build ./...`

- [ ] **Step 7: Commit**

```bash
git add content/packs/base/event-types.yaml content/packs/base/event-instances.yaml content/schemas/event-types.schema.json content/schemas/event-instances.schema.json .vscode/settings.json
git commit -m "feat(events): add seed data, JSON schemas, and VS Code wiring"
```

---

### Task 12: Sync Integration

**Files:**
- Modify: `cmd/sync.go`

- [ ] **Step 1: Add "events" to allCategories()**

```go
func allCategories() []string {
	return []string{"tips", "tools", "resources", "context", "mcp", "advocates", "events"}
}
```

- [ ] **Step 2: Add events fetch phase after marker expansion**

Add after the `runMarkerExpansion` call (around line 93), before the company repo sync:

```go
// Phase 3: events RSS cache
if err := runEventsFetch(paths.CacheDir, officialCache, force); err != nil {
	fmt.Fprintf(os.Stderr, "sap-devs: events sync warning: %v\n", err)
}
```

Add the helper function:

```go
func runEventsFetch(cacheDir, officialCache string, force bool) error {
	packsDir := filepath.Join(officialCache, "content", "packs")
	entries, err := os.ReadDir(packsDir)
	if err != nil {
		return nil
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		typesPath := filepath.Join(packsDir, entry.Name(), "event-types.yaml")
		data, err := os.ReadFile(typesPath)
		if err != nil {
			continue
		}
		var types []content.EventType
		if err := yaml.Unmarshal(data, &types); err != nil {
			continue
		}
		for _, et := range types {
			if et.Source == "rss" && et.RSSURL != "" {
				events.Resolve(et, cacheDir, force)
			}
		}
	}
	return nil
}
```

Add imports: `"github.tools.sap/developer-relations/sap-devs-cli/internal/events"`, `"gopkg.in/yaml.v3"` (yaml may already be imported — check).

Also add `"events"` to the `ttls` map in `runSync()`:

```go
"events": cfg.Sync.Events,
```

- [ ] **Step 3: Verify build**

Run: `go build ./...`

- [ ] **Step 4: Commit**

```bash
git add cmd/sync.go
git commit -m "feat(events): integrate events RSS caching into sync"
```

---

### Task 13: Smoke Test and Documentation

**Files:**
- Modify: `CLAUDE.md`

- [ ] **Step 1: Smoke test**

```bash
SAP_DEVS_DEV=1 go run . events --all
SAP_DEVS_DEV=1 go run . events types
SAP_DEVS_DEV=1 go run . events --type teched
SAP_DEVS_DEV=1 go run . events --type nonexistent
SAP_DEVS_DEV=1 go run . events --help
SAP_DEVS_DEV=1 go run . events open --help
```

Expected:
- `events --all`: shows seed TechEd instances (RSS types may fail on first run without network)
- `events types`: lists codejam, teched, devtoberfest
- `events --type teched`: shows TechEd instances only
- `events --type nonexistent`: "No events found" message
- help: shows flags and subcommands

Note: If running on Windows from a worktree, `go run` may be blocked by Defender. Use `go build` + `go vet` instead and verify output by reading code.

- [ ] **Step 2: Run go vet**

Run: `go vet ./...`

- [ ] **Step 3: Update CLAUDE.md**

Add between `hook` and `influencers` in the CLI Commands table:

```markdown
| `events` | Browse upcoming SAP community events with location filtering; `events types` lists event categories |
```

- [ ] **Step 4: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: add events command to CLI commands table"
```

---

### Task 14: Final Verification

- [ ] **Step 1: Full build check**

Run: `go build ./... && go vet ./...`
Expected: clean build, no vet warnings

- [ ] **Step 2: Verify help output**

Run: `./sap-devs events --help`
Expected: shows --all, --type, --count flags and open/types subcommands

Run: `./sap-devs events types --help`
Expected: shows types subcommand description
