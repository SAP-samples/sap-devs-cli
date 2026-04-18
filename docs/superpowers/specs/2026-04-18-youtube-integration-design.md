# YouTube Integration Design

**Date:** 2026-04-18
**Status:** Approved
**Approach:** Event-types mirror (Approach A)

## Summary

Make YouTube videos a first-class content type in sap-devs, with multi-playlist support, individual video links, dual-path fetching (RSS + YouTube Data API v3), sync engine integration, per-pack caching, and a `sap-devs videos` CLI command. Follows the established events pattern: YAML declarations per pack, fetch-and-cache during sync, resolve from cache at display time.

## Key Decisions

- **Playlist definitions:** Both layers. Base pack declares main channel playlists; individual packs can add pack-specific playlists. Supports both playlists and individual video links.
- **Storage:** Separate `Videos` collection on Pack (not merged into Resources).
- **News command:** Stays separate. The news command retains its unique community blog correlation and Friday hook. The SAP Developer News playlist also appears in the videos system, but the two commands serve different purposes.
- **Fetching:** Both RSS and YouTube Data API v3 ship together. RSS is the zero-config fallback; API v3 provides richer metadata when an API key is configured.
- **Injection:** CLI only. Videos are not injected into AI tool context files. Accessed via `sap-devs videos` command.

## 1. Data Model

### YAML: `youtube.yaml` per pack

Declares video sources — either playlists (fetched via RSS or API) or individual video links (static, no fetching needed).

```yaml
# content/packs/base/youtube.yaml
- id: sap-dev-news
  type: playlist
  name: SAP Developer News
  playlist_id: PL6RpkC85SLQAVBSQXN9522_1jNvPavBgg
  tags: [news, weekly]

- id: tech-bytes
  type: playlist
  name: SAP Tech Bytes
  playlist_id: PLkzo92owKnVw3l4fqcLoQalyFi9K4-UdY
  tags: [tutorial, short-form]

# content/packs/cap/youtube.yaml
- id: cap-tutorials
  type: playlist
  name: CAP Tutorial Series
  playlist_id: PLxxxxxxxxxxx
  tags: [tutorial, cap]

- id: cap-hana-swapi-walkthrough
  type: video
  name: "Cloud CAP HANA SWAPI Full Walkthrough"
  video_id: dQw4w9WgXcQ
  tags: [tutorial, cap, hana]
```

### Go structs

```go
// YouTubeSource declares a playlist or individual video in youtube.yaml.
type YouTubeSource struct {
    ID         string   `yaml:"id"`
    Type       string   `yaml:"type"`       // "playlist" | "video"
    Name       string   `yaml:"name"`
    PlaylistID string   `yaml:"playlist_id,omitempty"`
    VideoID    string   `yaml:"video_id,omitempty"`
    Tags       []string `yaml:"tags,omitempty"`
    PackID     string   // set at load time
}

// Video is a resolved YouTube video (fetched from a playlist or declared individually).
type Video struct {
    ID          string    // "<pack>/<source-id>/<video-id>"
    Title       string
    URL         string
    VideoID     string
    Published   time.Time
    Description string
    Duration    string    // ISO 8601 from API, empty from RSS
    SourceID    string    // which YouTubeSource this came from
    Tags        []string  // merged: source tags + API tags
    PackID      string
}
```

**ID format and lookup:** The composite `Video.ID` (`base/sap-dev-news/dQw4w9WgXcQ`) is used internally and displayed in list output. The `videos open` command accepts either a composite ID or a positional index (`videos open 3`), matching the `news open` pattern. Numeric parsing and 1-based index lookup happen in the `videos open` command handler; `FindVideo` is strictly composite-ID-only.

Pack gains two new fields: `YouTubeSources []YouTubeSource` and `Videos []Video`. Sources are loaded from YAML at `LoadPack` time. Videos are populated at runtime by resolving sources against the cache.

## 2. YouTube Package (`internal/youtube`)

### Existing (generalized)

`ParseFeed` and `FetchPlaylist` already handle any YouTube playlist Atom RSS feed. No changes needed to the parser. Returns `[]Episode` which maps to `[]Video` at a higher level.

### Episode struct extension

The existing `Episode` struct gains two optional fields to carry API v3 metadata:

```go
type Episode struct {
    ID          string
    Title       string
    URL         string
    Published   time.Time
    Description string
    Duration    string   // ISO 8601; populated by API v3, empty from RSS
    Tags        []string // populated by API v3, nil from RSS
}
```

The `news` command ignores these fields (zero values). The `videos` package maps `Episode` → `Video`, merging source-level tags with any API-provided tags.

### New: API v3 path

`FetchPlaylistAPI(playlistID, apiKey string) ([]Episode, error)` calls the YouTube Data API v3 `playlistItems.list` endpoint (paginating with `nextPageToken` for playlists > 50 items), followed by a batched `videos.list` call for the video IDs to get duration and tags. Returns `[]Episode` with the richer fields populated.

### Resolution function

```go
// Resolve fetches videos for a YouTubeSource.
// Tries API v3 if apiKey is provided, otherwise falls back to RSS.
// For type:"video" sources, returns a single-element slice without fetching.
func Resolve(src content.YouTubeSource, apiKey string) ([]Episode, error)
```

- `type: "video"` — returns a synthetic `Episode` from the source fields. No network call.
- `type: "playlist"` + `apiKey != ""` — calls `FetchPlaylistAPI`. On failure (HTTP errors, quota exceeded 403, invalid key), falls back to RSS with a stderr warning.
- `type: "playlist"` + `apiKey == ""` — calls `FetchPlaylist` via RSS URL constructed from `playlist_id`.

**Quota awareness:** The sync phase checks cache freshness *before* calling `Resolve`. If cached data is fresh, no API/RSS call is made. This avoids unnecessary API quota consumption (YouTube Data API v3 free tier: 10,000 units/day).

## 3. Caching & Sync Integration

### Cache structure

```text
~/.cache/sap-devs/youtube/<pack-id>/<source-id>.json
```

Cache paths are namespaced by pack ID to avoid collisions when different packs declare sources with the same ID. Each file contains `[]Video` as JSON. New `internal/videos/cache.go`:

- `LoadCache(cacheDir, packID, sourceID) ([]content.Video, error)`
- `SaveCache(cacheDir, packID, sourceID, []content.Video) error`
- `CacheAge(cacheDir, packID, sourceID) time.Duration`

Default TTL: 6 hours. Cache-with-live-fallback pattern (same as events): try fresh fetch, fall back to stale cache on error.

### Sync category

`"youtube"` added to `allCategories()` in `cmd/sync.go`. New TTL entry:

```go
ttls["youtube"] = cfg.Sync.YouTube
```

**TTL independence:** `allCategories()` returns all categories, but `runSync` splits them into two groups:

```go
archiveCategories := []string{"tips", "tools", "resources", "context", "mcp", "advocates"}
independentCategories := []string{"events", "youtube"}
```

The `ttls` map includes entries for both groups — `ttls["youtube"]` feeds `engine.IsStale("youtube")` in phase 4, not the archive staleness check.

**Guard logic:** Two separate staleness checks:

```go
// Intersect with --category filter if set
activeArchive := intersect(archiveCategories, categories)
activeIndependent := intersect(independentCategories, categories)

archiveNeedsSync := force
for _, cat := range activeArchive {
    if engine.IsStale(cat) { archiveNeedsSync = true; break }
}

independentNeedsSync := force
for _, cat := range activeIndependent {
    if engine.IsStale(cat) { independentNeedsSync = true; break }
}

if !archiveNeedsSync && !independentNeedsSync {
    fmt.Fprintln(out, "up to date")
    return nil
}
```

The "up to date" early-exit fires only when both groups are fresh. Phase 1 (archive download) and phase 2 (marker expansion) only run when `archiveNeedsSync` is true. Phase 3 (events) runs when `"events"` is in `activeIndependent` and `engine.IsStale("events") || force`. Phase 4 (YouTube) runs when `"youtube"` is in `activeIndependent` and `engine.IsStale("youtube") || force`.

`MarkAllSynced` is called separately: archive categories after phase 2; `"events"` after phase 3; `"youtube"` after phase 4.

**`--category` flag interaction:**

- `--category youtube` → `activeArchive` is empty, `archiveNeedsSync` is false, phases 1-2 skipped. Phase 4 runs if YouTube is stale. Phase 3 skipped (events not in filter).
- `--category tips` → `activeIndependent` is empty, phases 3-4 skipped. Archive downloads if `tips` is stale.
- No `--category` → all phases evaluated independently.

**Note:** This also fixes a pre-existing gap where `runEventsFetch` (phase 3) currently runs unconditionally after phase 2, even with `--category tips`. The new guard applies consistently to both events and YouTube.

### Sync phase 4

New phase in `runSync()`, after phase 3 (events RSS cache):

```go
// Phase 4: YouTube video cache
if err := runYouTubeFetch(paths.CacheDir, officialCache, paths.ConfigDir, force); err != nil {
    fmt.Fprintf(os.Stderr, "sap-devs: youtube sync warning: %v\n", err)
}
```

`runYouTubeFetch` scans packs from the official cache and company cache (if configured) for `youtube.yaml`, collects playlist sources, resolves API key from credentials, then calls `youtube.Resolve` for each source (checking per-source cache freshness first to avoid unnecessary fetches) and caches results via `videos.SaveCache`. User-layer and project-layer packs only support `type: video` entries (static, no fetching). Individual source failures are non-fatal (warning to stderr, skip).

### Video resolution at display time

`LoadPack` reads `youtube.yaml` into `YouTubeSources` only. A separate `videos.ResolveAll(sources, cacheDir)` function (called from the `videos` CLI command) reads cached JSON files and returns `[]Video`. This keeps `LoadPack` fast and cache-independent.

## 4. CLI Command (`sap-devs videos`)

### Subcommands

| Subcommand | Purpose |
| --- | --- |
| `videos list` | List videos for the active profile's packs. Default: most recent 20. |
| `videos search <query>` | Search across all packs by title, description, tags. |
| `videos open <id>` | Open a video URL in the browser. |

`videos` with no subcommand defaults to `videos list`.

### List output format

```text
#   DATE        PACK   SOURCE              TITLE
1   2026-04-11  base   sap-dev-news        SAP Developer News - Apr 11
2   2026-04-09  base   tech-bytes          SAP Tech Bytes: CDS Lint
3   2026-04-07  cap    cap-tutorials       Build a CAP App in 10 Minutes
```

Uses `tabwriter`. Videos sorted by `Published` descending (most recent first).

### Flags

- `--count/-n` (int, default 20) — number of videos to show
- `--source` (string, optional) — filter to a specific source ID
- `--pack` (string, optional) — filter to a specific pack

### Data flow for `videos list`

1. Load profile config, find active profile.
2. `loader.LoadPacks(profile, lang)` — packs with `YouTubeSources` populated.
3. For each pack, `videos.ResolveAll(pack.YouTubeSources, cacheDir)` — reads cached JSON, maps to `[]Video`.
4. Flatten across packs, sort by date, apply filters and `--count` limit, print table.

### Helper functions (`internal/videos/videos.go`)

- `ResolveAll(sources []content.YouTubeSource, cacheDir string) ([]content.Video, error)` — reads cached JSON for each source, maps to `[]Video`
- `FilterVideos(videos []content.Video, query string) []content.Video` — case-insensitive substring match on title, description, tags
- `FindVideo(videos []content.Video, id string) *content.Video` — exact match on composite ID

## 5. Credentials Extension & Config

### Service-keyed credential storage

New functions alongside the existing API (which remains untouched):

```go
func StoreService(configDir, service, token string) error
func LoadService(configDir, service string) (string, error)
func DeleteService(configDir, service string) error
func ResolveService(configDir, service string, envVars []string) string
```

Keyring: `keyringSvc = "sap-devs"` (same), `keyringUser = service` (variable). File fallback: `<configDir>/credentials-<service>` (e.g., `credentials-youtube`).

**Implementation approach:** Extract the common keyring+file logic from existing `Store`/`Load`/`Delete` into private helpers (`storeForUser`, `loadForUser`, `deleteForUser`) that take a `user` parameter. The existing public functions become thin wrappers passing `"github-token"` as the user. The new `*Service` functions pass the `service` argument as the user. This avoids code duplication while keeping the existing API untouched.

### YouTube API key resolution chain

`YOUTUBE_API_KEY` env var -> keychain (`sap-devs` / `youtube`) -> file (`credentials-youtube`) -> `""` (empty = RSS fallback).

### Config command extension

```bash
sap-devs config token <key> --service youtube   # stores YouTube API key
sap-devs config token <key>                      # existing: stores GitHub token
```

`config show` is updated to also display YouTube API key status (configured/not configured) alongside the existing GitHub token status.

The existing `--delete` flag works with `--service`: `config token --delete --service youtube` calls `DeleteService(configDir, "youtube")`. Without `--service`, `--delete` continues to call the existing `Delete` for GitHub tokens.

### Sync config

`config.SyncConfig` gains `YouTube time.Duration` field:

```go
type SyncConfig struct {
    // ...existing fields...
    YouTube time.Duration `yaml:"youtube,omitempty"`
}
```

Default: 6 hours. Configurable via `sap-devs config set sync.youtube 12h`.

## 6. Schema, Testing & Error Handling

### JSON Schema

New `content/schemas/youtube.schema.json` validating `youtube.yaml`. Covers both source types with conditional requirements: `playlist` type requires `playlist_id`; `video` type requires `video_id`. Wired in `.vscode/settings.json`.

### Testing

- `internal/youtube/youtube_test.go` — extend with API v3 response parsing tests; add testdata for API JSON responses.
- `internal/videos/videos_test.go` — test `ResolveAll`, `FilterVideos`, `FindVideo` with fixture JSON cache files.
- `internal/credentials/credentials_test.go` — test `StoreService`/`LoadService`/`ResolveService` using the existing mock keyring pattern.
- `internal/content/pack_test.go` — test that `LoadPack` reads `youtube.yaml` into `YouTubeSources`.

All tests follow existing patterns: table-driven, testdata fixtures, no mocks except the keyring (already established).

### Error handling

- **Sync:** Individual source failures are non-fatal. Warning to stderr, skip source. Matches events pattern.
- **`videos list`/`search`:** If cache is empty for a source, source is silently omitted. If all caches are empty: "No videos cached. Run `sap-devs sync` first."
- **API v3:** HTTP errors, quota exceeded (403), invalid key — fall back to RSS transparently. Log warning to stderr.
- **RSS:** Network failures fall back to stale cache. No cache at all — source omitted.

## Files Created/Modified

### New files

- `internal/videos/videos.go` — FilterVideos, FindVideo, ResolveAll
- `internal/videos/cache.go` — LoadCache, SaveCache, CacheAge
- `internal/videos/videos_test.go` — tests with fixture data
- `cmd/videos.go` — CLI command with list/search/open subcommands
- `content/schemas/youtube.schema.json` — YAML validation schema
- `content/packs/base/youtube.yaml` — base pack playlist declarations
- `content/packs/cap/youtube.yaml` — CAP pack playlist/video declarations

### Modified files

- `internal/content/pack.go` — add YouTubeSource, Video structs; add fields to Pack; load youtube.yaml in LoadPack
- `internal/youtube/youtube.go` — extend Episode struct with Duration/Tags; add FetchPlaylistAPI, Resolve function
- `internal/youtube/youtube_test.go` — add API v3 tests
- `internal/credentials/credentials.go` — refactor to private helpers; add StoreService, LoadService, DeleteService, ResolveService
- `internal/credentials/credentials_test.go` — test service-keyed storage
- `internal/config/config.go` — add YouTube field to SyncConfig
- `cmd/sync.go` — add "youtube" to allCategories(), split archive-dependent vs independent categories, add phase 4, add TTL entry
- `cmd/config.go` — add --service flag to token subcommand; update config show to display YouTube key status
- `.vscode/settings.json` — wire youtube.schema.json
- `CLAUDE.md` — document videos command
- `docs/content-authoring.md` — document youtube.yaml format
