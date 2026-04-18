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

Pack gains two new fields: `YouTubeSources []YouTubeSource` and `Videos []Video`. Sources are loaded from YAML at `LoadPack` time. Videos are populated at runtime by resolving sources against the cache.

## 2. YouTube Package (`internal/youtube`)

### Existing (generalized)

`ParseFeed` and `FetchPlaylist` already handle any YouTube playlist Atom RSS feed. No changes needed to the parser. Returns `[]Episode` which maps to `[]Video` at a higher level.

### New: API v3 path

`FetchPlaylistAPI(playlistID, apiKey string) ([]Episode, error)` calls the YouTube Data API v3 `playlistItems.list` endpoint, followed by a `videos.list` call for the batch of video IDs to get duration and additional metadata. Returns the same `[]Episode` struct with richer fields populated.

### Resolution function

```go
// Resolve fetches videos for a YouTubeSource.
// Tries API v3 if apiKey is provided, otherwise falls back to RSS.
// For type:"video" sources, returns a single-element slice without fetching.
func Resolve(src content.YouTubeSource, apiKey string) ([]Episode, error)
```

- `type: "video"` — returns a synthetic `Episode` from the source fields. No network call.
- `type: "playlist"` + `apiKey != ""` — calls `FetchPlaylistAPI`.
- `type: "playlist"` + `apiKey == ""` — calls `FetchPlaylist` via RSS URL constructed from `playlist_id`.

API v3 errors (HTTP failures, quota exceeded 403, invalid key) fall back to RSS transparently with a stderr warning.

## 3. Caching & Sync Integration

### Cache structure

```
~/.cache/sap-devs/youtube/<source-id>.json
```

Each file contains `[]Video` as JSON. New `internal/videos/cache.go`:

- `LoadCache(cacheDir, sourceID) ([]content.Video, error)`
- `SaveCache(cacheDir, sourceID, []content.Video) error`
- `CacheAge(cacheDir, sourceID) time.Duration`

Default TTL: 6 hours. Cache-with-live-fallback pattern (same as events): try fresh fetch, fall back to stale cache on error.

### Sync category

`"youtube"` added to `allCategories()` in `cmd/sync.go`. New TTL entry:

```go
ttls["youtube"] = cfg.Sync.YouTube
```

### Sync phase 4

New phase in `runSync()`, after phase 3 (events RSS cache):

```go
// Phase 4: YouTube video cache
if err := runYouTubeFetch(paths.CacheDir, officialCache, paths.ConfigDir, force); err != nil {
    fmt.Fprintf(os.Stderr, "sap-devs: youtube sync warning: %v\n", err)
}
```

`runYouTubeFetch` scans all packs for `youtube.yaml`, collects playlist sources, resolves API key from credentials, then calls `youtube.Resolve` for each source and caches results via `videos.SaveCache`. Individual source failures are non-fatal (warning to stderr, skip).

### Video resolution at display time

`LoadPack` reads `youtube.yaml` into `YouTubeSources` only. A separate `videos.ResolveAll(sources, cacheDir)` function (called from the `videos` CLI command) reads cached JSON files and returns `[]Video`. This keeps `LoadPack` fast and cache-independent.

## 4. CLI Command (`sap-devs videos`)

### Subcommands

| Subcommand | Purpose |
|---|---|
| `videos list` | List videos for the active profile's packs. Default: most recent 20. |
| `videos search <query>` | Search across all packs by title, description, tags. |
| `videos open <id>` | Open a video URL in the browser. |

`videos` with no subcommand defaults to `videos list`.

### List output format

```
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

- `ResolveAll(sources []content.YouTubeSource, cacheDir string) ([]content.Video, error)`
- `FlattenVideos(packs []*content.Pack) []content.Video`
- `FilterVideos(videos []content.Video, query string) []content.Video`
- `FindVideo(videos []content.Video, id string) *content.Video`

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

### YouTube API key resolution chain

`YOUTUBE_API_KEY` env var -> keychain (`sap-devs` / `youtube`) -> file (`credentials-youtube`) -> `""` (empty = RSS fallback).

### Config command extension

```bash
sap-devs config token <key> --service youtube   # stores YouTube API key
sap-devs config token <key>                      # existing: stores GitHub token
```

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
- `internal/videos/videos_test.go` — test `ResolveAll`, `FlattenVideos`, `FilterVideos`, `FindVideo` with fixture JSON cache files.
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
- `internal/videos/videos.go` — FlattenVideos, FilterVideos, FindVideo, ResolveAll
- `internal/videos/cache.go` — LoadCache, SaveCache, CacheAge
- `internal/videos/videos_test.go` — tests with fixture data
- `cmd/videos.go` — CLI command with list/search/open subcommands
- `content/schemas/youtube.schema.json` — YAML validation schema
- `content/packs/base/youtube.yaml` — base pack playlist declarations
- `content/packs/cap/youtube.yaml` — CAP pack playlist/video declarations

### Modified files
- `internal/content/pack.go` — add YouTubeSource, Video structs; add fields to Pack; load youtube.yaml in LoadPack
- `internal/youtube/youtube.go` — add FetchPlaylistAPI, Resolve function
- `internal/youtube/youtube_test.go` — add API v3 tests
- `internal/credentials/credentials.go` — add StoreService, LoadService, DeleteService, ResolveService
- `internal/credentials/credentials_test.go` — test service-keyed storage
- `internal/config/config.go` — add YouTube field to SyncConfig
- `cmd/sync.go` — add "youtube" to allCategories(), add phase 4, add TTL entry
- `cmd/config.go` — add --service flag to token subcommand
- `.vscode/settings.json` — wire youtube.schema.json
- `CLAUDE.md` — document videos command
- `docs/content-authoring.md` — document youtube.yaml format
