# Events Command — Design Spec

**Date:** 2026-04-18
**Status:** Approved
**Project:** sap-devs-cli

---

## Overview

Add a `sap-devs events` command to browse upcoming SAP community events from two data sources: RSS feeds (CodeJam, Devtoberfest) fetched live with cache fallback, and manually curated YAML instances (TechEd sessions). Event types are defined in pack-level `event-types.yaml`; manual instances in `event-instances.yaml`. Location-based filtering uses an embedded city coordinate database with Haversine distance and configurable radius thresholds. Includes iCal export, session-start hook reminders, and cross-platform OS notifications.

---

## Data Model

### `event-types.yaml` (per pack)

Defines event categories and their data sources:

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

Required fields: `id`, `name`, `source`, `default_scope`.
- `source`: `"rss"` | `"manual"`
- `rss_url`: required when `source` is `"rss"`
- `default_scope`: applied to RSS-parsed events when scope is not determinable

### `event-instances.yaml` (per pack)

Manually curated event instances (for `source: manual` types):

```yaml
- id: teched-2026-bangalore
  type: teched
  title: SAP TechEd 2026 Bangalore
  date: "2026-10-21"
  end_date: "2026-10-23"
  location: Bangalore, India
  scope: regional
  url: https://www.sap.com/events/teched.html
  room:
  speaker:
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

Required fields: `id`, `type`, `title`, `date`, `url`, `scope`.
Optional fields: `end_date`, `location`, `room`, `speaker`, `tags`.

### Go Structs

Added to `internal/content/pack.go`:

```go
type EventType struct {
    ID           string   `yaml:"id"`
    Name         string   `yaml:"name"`
    Description  string   `yaml:"description,omitempty"`
    Source       string   `yaml:"source"`
    RSSURL       string   `yaml:"rss_url,omitempty"`
    DefaultScope string   `yaml:"default_scope"`
    Tags         []string `yaml:"tags,omitempty"`
    PackID       string   // set at load time
}

type EventInstance struct {
    ID       string   `yaml:"id"`
    Type     string   `yaml:"type"`
    Title    string   `yaml:"title"`
    Date     time.Time `yaml:"-"`     // parsed from string; YAML uses custom unmarshal
    DateStr  string    `yaml:"date"` // ISO 8601 date: "YYYY-MM-DD" or "YYYY-MM-DDTHH:MM:SSZ"
    EndDate  time.Time `yaml:"-"`
    EndDateStr string  `yaml:"end_date,omitempty"`
    Location string   `yaml:"location,omitempty"`
    Scope    string   `yaml:"scope"`
    URL      string   `yaml:"url"`
    Room     string   `yaml:"room,omitempty"`
    Speaker  string   `yaml:"speaker,omitempty"`
    Tags     []string `yaml:"tags,omitempty"`
    PackID   string   // set at load time
}
```

`Pack` gains two new fields: `EventTypes []EventType` and `EventInstances []EventInstance`.

**Date parsing contract:** All dates are normalised to `time.Time` at parse time. YAML manual instances use `DateStr`/`EndDateStr` (ISO 8601 `YYYY-MM-DD` format), unmarshaled into `Date`/`EndDate` via a custom `UnmarshalYAML`. RSS-sourced events parse `<pubDate>` (RFC 1123Z) into `Date` via `time.Parse`. The canonical in-memory representation is always `time.Time`, used for sorting and iCal export. JSON cache files store dates as RFC 3339 strings.

---

## Events Package: `internal/events/`

### `rss.go` — RSS Fetching

`FetchRSS(rssURL string, typeID string, defaultScope string) ([]EventInstance, error)`

Fetches SAP Community RSS, parses `<item>` elements into `[]EventInstance`. Uses the same HTTP pattern as `internal/community/` (custom user agent, 10s timeout, 1 MiB limit). Maps RSS fields: `<title>` → Title, `<link>` → URL, `<pubDate>` → Date. Generates deterministic IDs from type + URL hash. Sets `Type` to `typeID` and `Scope` to `defaultScope`.

### `cache.go` — File-Based Cache

- `LoadCache(cacheDir string, typeID string) ([]EventInstance, error)` — reads `~/.cache/sap-devs/events/<typeID>.json`
- `SaveCache(cacheDir string, typeID string, events []EventInstance) error` — writes cache file as JSON

### `events.go` — Orchestration

`Resolve(eventType EventType, cacheDir string) ([]EventInstance, error)` — main entrypoint for RSS-sourced types:

1. Check cache freshness: if cache exists and is less than 4 hours old, return cached events (no fetch)
2. If cache is stale or missing: attempt live fetch with 3-second timeout
3. On success: update cache, return fresh events
4. On failure: load from stale cache if available, return those events
5. If no cache either: return nil (not an error)

This makes `Resolve()` the single fetch path — the sync command calls `Resolve()` for each RSS type (which populates the cache), and the `events` command also calls `Resolve()` (which reads fresh cache or re-fetches if stale). There is no separate sync-only fetch path.

`FilterByLocation(events []EventInstance, userLat, userLon float64, localRadius, regionalRadius int) []EventInstance`:
- `virtual` / `global` scope: always included
- `regional` scope: included if event location is within `regionalRadius` km
- `local` scope: included if event location is within `localRadius` km
- Events with empty location or `location: "virtual"`: always included
- Geocoding via `geo.Lookup()` on each event's location string

`MergeAndSort(rss []EventInstance, manual []EventInstance) []EventInstance`:
- Combines both slices, deduplicates by ID, sorts by date ascending (upcoming first)

### `ical.go` — iCal Export

`ExportICS(events []EventInstance, w io.Writer) error` — writes standard iCal format:
- `BEGIN:VCALENDAR` / `END:VCALENDAR` wrapper
- Per event: `VEVENT` with `DTSTART`, `DTEND` (if end_date), `SUMMARY`, `LOCATION`, `URL`, `DESCRIPTION`
- `PRODID:-//sap-devs//events//EN`
- Date format: `YYYYMMDD` (all-day) or `YYYYMMDDTHHMMSSZ` if time is present

### `notify.go` — Notification Logic

`CheckUpcoming(events []EventInstance, withinDays int) []EventInstance` — returns events whose date is within N days from now.

`FormatHookMessage(upcoming []EventInstance) string` — formats a terminal-friendly reminder:
```
📅 3 upcoming SAP events this week:
  • May 15 — CodeJam Hamburg (local)
  • May 18 — CodeJam Munich (local)
  • May 20 — Devtoberfest Webinar (virtual)
Run 'sap-devs events' for details or 'sap-devs events open <id>' to register.
```

---

## Geo Package: `internal/geo/`

### `cities.go` — Embedded City Database

```go
//go:embed cities.json
var citiesJSON []byte
```

`cities.json` contains ~500 cities with coordinates:
```json
[
  {"name": "Hamburg", "country": "Germany", "lat": 53.5511, "lon": 9.9937},
  {"name": "Bangalore", "country": "India", "lat": 12.9716, "lon": 77.5946},
  ...
]
```

`Lookup(location string) (lat, lon float64, ok bool)` — parses "City, Country" format. Tries exact city+country match first, then city-only match. Case-insensitive. Returns `ok=false` if no match found.

### `distance.go` — Haversine Distance

```go
func DistanceKm(lat1, lon1, lat2, lon2 float64) float64
func IsNearby(lat1, lon1, lat2, lon2 float64, radiusKm float64) bool
```

Standard Haversine formula. Earth radius: 6371 km.

---

## Notification System: `internal/notify/`

### `notify.go` — Cross-Platform OS Notifications

`Send(title, body string) error` — dispatches OS-level notification:

- **Windows:** PowerShell `[Windows.UI.Notifications.ToastNotificationManager]` (built-in on Windows 10+, no third-party module needed). Falls back to `msg.exe` console message if toast API unavailable.
- **macOS:** `osascript -e 'display notification "body" with title "title"'`
- **Linux:** `notify-send "title" "body"`

`Available() bool` — returns true if the current platform has a working notification method. Used by `events notify` to skip gracefully on unsupported systems.

Returns an error if the notification method is unavailable. Callers handle gracefully — notification failure is never fatal. On unsupported platforms, `events notify` prints the reminder to stdout instead (same as `events hook` output).

---

## Sync Integration

`cmd/sync.go` adds `"events"` to `allCategories()`. The `runSync()` function gains an events phase after the zip extraction:

1. Load event types from the freshly-synced official content packs
2. For each type with `source: rss`, call `events.Resolve()` (which fetches and caches)
3. Track in `sync-state.json` under `events` key via existing TTL mechanism

`internal/sync/engine.go` is not modified — the events fetch is orchestrated in `cmd/sync.go` using `events.Resolve()` directly (same pattern as how `sync` currently calls `expansion.Process()` for dynamic content). `SyncConfig` in `internal/config/config.go` gains an `Events time.Duration` field for the TTL (default: 4 hours). `--force` bypasses as usual.

---

## Config Integration

### `EventsConfig` sub-struct

```go
type EventsConfig struct {
    LocalRadius    int    `yaml:"local_radius,omitempty"`    // km, default 200
    RegionalRadius int    `yaml:"regional_radius,omitempty"` // km, default 800
    NotifyDays     int    `yaml:"notify_days,omitempty"`     // default 7
    NotifyMethod   string `yaml:"notify_method,omitempty"`   // "hook" | "os" | "both"
}
```

Added to `Config` struct: `Events EventsConfig `yaml:"events,omitempty"``

Runtime defaults: `LocalRadius=200`, `RegionalRadius=800`, `NotifyDays=7`, `NotifyMethod="hook"` when values are zero/empty.

### Config subcommands

`configEventsCmd` is a parent command registered under `configCmd` (like `configCmd.AddCommand(configEventsCmd)`). Running `sap-devs config events` with no arguments displays all current events settings. Subcommands set individual values:

```
sap-devs config events                         # show current events config
sap-devs config events local-radius [km]       # get/set local radius
sap-devs config events regional-radius [km]    # get/set regional radius
sap-devs config events notify-days [days]      # get/set notification window
sap-devs config events notify-method [method]  # get/set: hook | os | both
```

Each subcommand: with an argument sets the value and prints confirmation; without an argument prints the current value. Follows the `configTipRotationCmd` pattern. `configShowCmd` is updated to print the events sub-struct values.

---

## Command Structure

### Parent: `events` (defaults to list)

```
sap-devs events                              # upcoming, location-filtered
sap-devs events --all                        # no location filter
sap-devs events --type codejam               # filter by event type
sap-devs events --count 20                   # limit results (default 10)
```

**Flags:**

| Flag | Short | Type | Default | Description |
| --- | --- | --- | --- | --- |
| `--all` | `-a` | bool | false | Show all events regardless of location |
| `--type` | `-t` | string | "" | Filter by event type ID |
| `--count` | `-n` | int | 10 | Max events to display |

### Subcommands

| Subcommand | Description |
| --- | --- |
| `open <id>` | Open event URL in browser |
| `types` | List available event types |
| `export [--type X] [--output path]` | Export events to .ics file |
| `hook` | Print upcoming event reminders (session-start hook) |
| `notify` | Send OS notification for upcoming events |

### List Output (tabwriter)

```
DATE         TYPE       SCOPE     LOCATION              TITLE
2026-05-15   codejam    local     Hamburg, Germany       CodeJam Hamburg — CAP Hands-on
2026-06-02   codejam    local     Munich, Germany        CodeJam Munich — ABAP Cloud
2026-10-01   devtober   global    virtual                Devtoberfest 2026
2026-10-21   teched     regional  Bangalore, India       SAP TechEd 2026 Bangalore
```

### Types Output

```
ID             SOURCE   NAME
codejam        rss      SAP CodeJam
devtoberfest   rss      Devtoberfest
teched         manual   SAP TechEd
```

### List Flow

1. Load event types from all packs via `FlattenEventTypes()`
2. For each RSS-sourced type: call `events.Resolve()` (live fetch → cache fallback)
3. Load manual instances via `FlattenEventInstances()`
4. `MergeAndSort()` RSS + manual
5. Apply `--type` filter if set
6. Apply location filter unless `--all` (geocode user location, apply radius thresholds)
7. Limit to `--count`
8. Display with tabwriter

---

## Location Filtering

When user has location configured:
1. `geo.Lookup(cfg.Location)` → user coordinates
2. If lookup fails: show all events (can't filter without coordinates)
3. For each event:
   - `virtual` / `global` scope: **always shown**
   - `regional` scope: shown if `geo.Lookup(event.Location)` is within `cfg.Events.RegionalRadius` km (default 800)
   - `local` scope: shown if within `cfg.Events.LocalRadius` km (default 200)
   - Events with `location: "virtual"` or empty location: **always shown**
4. Events whose location can't be geocoded: **shown** (fail-open)

No location configured = show all (equivalent to `--all`).

---

## i18n Keys

Add to `en.json` and `de.json`:

| Key | English |
| --- | --- |
| `events.short` | `Browse upcoming SAP community events` |
| `events.long` | `Browse upcoming SAP community events. Filters by your location when configured.` |
| `events.none` | `No upcoming events found.` |
| `events.none_type` | `No events found for type "{{.Type}}".` |
| `events.open.short` | `Open an event URL in the browser` |
| `events.types.short` | `List available event types` |
| `events.export.short` | `Export events to an iCal (.ics) file` |
| `events.export.done` | `Exported {{.Count}} events to {{.Path}}` |
| `events.hook.short` | `Print upcoming event reminders (session-start hook)` |
| `events.notify.short` | `Send OS notification for upcoming events` |
| `events.notify.sent` | `Notification sent: {{.Count}} upcoming events` |
| `events.notify.none` | `No upcoming events within {{.Days}} days.` |
| `events.notify.unsupported` | `OS notifications not available on this system.` |
| `events.not_found` | `Event "{{.ID}}" not found.` |
| `events.col_date` | `DATE` |
| `events.col_type` | `TYPE` |
| `events.col_scope` | `SCOPE` |
| `events.col_location` | `LOCATION` |
| `events.col_title` | `TITLE` |
| `events.types.col_id` | `ID` |
| `events.types.col_source` | `SOURCE` |
| `events.types.col_name` | `NAME` |

---

## Files Changed

| File | Change |
| --- | --- |
| `internal/content/pack.go` | Add `EventType` and `EventInstance` structs; add fields to `Pack`; load both YAML files in `LoadPack()` |
| `internal/content/merge.go` | Add `mergeEventTypes()` and `mergeEventInstances()`; wire into `MergeWith()` |
| `internal/content/events.go` | New: `FlattenEventTypes`, `FlattenEventInstances`, `FilterEventsByType`, `FindEvent` |
| `internal/geo/cities.go` | New: embedded city database, `Lookup()` |
| `internal/geo/cities.json` | New: ~500 cities with lat/lon coordinates |
| `internal/geo/distance.go` | New: `DistanceKm()`, `IsNearby()` |
| `internal/events/rss.go` | New: `FetchRSS()` — RSS fetch and parse into `[]EventInstance` |
| `internal/events/cache.go` | New: `LoadCache()`, `SaveCache()` |
| `internal/events/events.go` | New: `Resolve()`, `FilterByLocation()`, `MergeAndSort()` |
| `internal/events/ical.go` | New: `ExportICS()` — iCal/ICS generation |
| `internal/events/notify.go` | New: `CheckUpcoming()`, `FormatHookMessage()` |
| `internal/notify/notify.go` | New: cross-platform OS notification dispatch |
| `cmd/events.go` | New: `eventsCmd`, `eventsOpenCmd`, `eventsTypesCmd`, `eventsExportCmd`, `eventsHookCmd`, `eventsNotifyCmd` |
| `cmd/config.go` | Add `configEventsCmd` parent with show-all default; add subcommands for each setting; update `configShowCmd` to print events values |
| `internal/config/config.go` | Add `EventsConfig` sub-struct; add `Events` field to `Config`; add `Events time.Duration` to `SyncConfig` |
| `cmd/sync.go` | Add `"events"` to `allCategories()`; add events fetch phase calling `events.Resolve()` for each RSS-sourced type |
| `content/packs/base/event-types.yaml` | Seed: CodeJam, TechEd, Devtoberfest |
| `content/packs/base/event-instances.yaml` | Seed: TechEd 2026 instances |
| `content/schemas/event-types.schema.json` | JSON Schema |
| `content/schemas/event-instances.schema.json` | JSON Schema |
| `.vscode/settings.json` | Wire both schemas |
| `internal/i18n/catalogs/en.json` | Add `events.*` i18n keys |
| `internal/i18n/catalogs/de.json` | Add German `events.*` i18n keys |
| `CLAUDE.md` | Add `events` to CLI commands table |
