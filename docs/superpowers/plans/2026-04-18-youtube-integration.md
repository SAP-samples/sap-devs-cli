# YouTube Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make YouTube videos a first-class content type in sap-devs with multi-playlist support, dual-path fetching (RSS + API v3), sync engine integration, and a `sap-devs videos` CLI command.

**Architecture:** Mirrors the events system pattern — YAML declarations per pack, fetch-and-cache during sync, resolve from cache at display time. Videos are a separate collection on Pack (not merged into resources). The `news` command stays independent.

**Tech Stack:** Go 1.22+, cobra CLI, YouTube Atom RSS, YouTube Data API v3, zalando/go-keyring, testify

**Spec:** `docs/superpowers/specs/2026-04-18-youtube-integration-design.md`

---

## File Map

### New files

| File | Responsibility |
| --- | --- |
| `internal/videos/videos.go` | ResolveAll, FilterVideos, FindVideo — resolve cached data, search, lookup |
| `internal/videos/cache.go` | LoadCache, SaveCache, CacheAge — JSON cache at `youtube/<pack>/<source>.json` |
| `internal/videos/videos_test.go` | Tests for all videos package functions |
| `internal/videos/testdata/base/sap-dev-news.json` | Fixture: cached video list for tests |
| `internal/youtube/apiv3.go` | FetchPlaylistAPI — YouTube Data API v3 client |
| `internal/youtube/resolve.go` | Resolve — dispatch to API v3, RSS, or synthetic episode |
| `internal/youtube/apiv3_test.go` | Tests for API v3 parsing |
| `internal/youtube/resolve_test.go` | Tests for Resolve dispatch logic |
| `internal/youtube/testdata/apiv3_playlistitems.json` | Fixture: API v3 playlistItems response |
| `internal/youtube/testdata/apiv3_videos.json` | Fixture: API v3 videos.list response |
| `cmd/videos.go` | CLI command: videos list/search/open |
| `content/schemas/youtube.schema.json` | JSON Schema for youtube.yaml validation |
| `content/packs/base/youtube.yaml` | Base pack playlist declarations (SAP Dev News, Tech Bytes) |
| `content/packs/cap/youtube.yaml` | CAP pack playlist/video declarations |

### Modified files

| File | Changes |
| --- | --- |
| `internal/content/pack.go` | Add YouTubeSource, Video structs; add fields to Pack; load youtube.yaml in LoadPack |
| `internal/content/pack_test.go` | Test youtube.yaml loading |
| `internal/youtube/youtube.go` | Extend Episode struct with Duration, Tags fields |
| `internal/youtube/youtube_test.go` | Verify existing tests still pass with extended Episode |
| `internal/credentials/credentials.go` | Refactor to private helpers; add StoreService/LoadService/DeleteService/ResolveService |
| `internal/credentials/credentials_test.go` | Test service-keyed storage |
| `internal/config/config.go` | Add YouTube field to SyncConfig; add to Default() |
| `cmd/sync.go` | Split archive/independent categories; add phase 4; add YouTube TTL |
| `cmd/config.go` | Add --service flag to token; update config show; update config set |
| `.vscode/settings.json` | Wire youtube.schema.json |
| `internal/i18n/catalogs/en.json` | Add videos.* and config.show.sync_youtube i18n keys |
| `internal/i18n/catalogs/de.json` | Add German translations for same keys |
| `CLAUDE.md` | Document videos command in CLI Commands table |
| `docs/content-authoring.md` | Document youtube.yaml format |

---

## Task 1: Data Model — YouTubeSource and Video structs

**Files:**
- Modify: `internal/content/pack.go:12-39` (Pack struct), `internal/content/pack.go:193-310` (LoadPack)
- Modify: `internal/content/pack_test.go`

- [ ] **Step 1: Write the failing test for youtube.yaml loading**

Add to `internal/content/pack_test.go`:

```go
func TestLoadPack_YouTubeSourcesLoaded(t *testing.T) {
	dir := t.TempDir()
	packYAML := "id: cap\nname: CAP\ndescription: CAP\ntags: []\nprofiles: []\nweight: 100\n"
	youtubeYAML := `- id: cap-tutorials
  type: playlist
  name: CAP Tutorial Series
  playlist_id: PLxxxxxxxxxxx
  tags: [tutorial, cap]
- id: cap-walkthrough
  type: video
  name: CAP Full Walkthrough
  video_id: dQw4w9WgXcQ
  tags: [tutorial]
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(packYAML), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "youtube.yaml"), []byte(youtubeYAML), 0644))

	p, err := content.LoadPack(dir, "")
	require.NoError(t, err)
	require.Len(t, p.YouTubeSources, 2)
	assert.Equal(t, "cap-tutorials", p.YouTubeSources[0].ID)
	assert.Equal(t, "playlist", p.YouTubeSources[0].Type)
	assert.Equal(t, "PLxxxxxxxxxxx", p.YouTubeSources[0].PlaylistID)
	assert.Equal(t, "cap", p.YouTubeSources[0].PackID)
	assert.Equal(t, "cap-walkthrough", p.YouTubeSources[1].ID)
	assert.Equal(t, "video", p.YouTubeSources[1].Type)
	assert.Equal(t, "dQw4w9WgXcQ", p.YouTubeSources[1].VideoID)
}

func TestLoadPack_YouTubeSourcesEmptyWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	packYAML := "id: cap\nname: CAP\ndescription: CAP\ntags: []\nprofiles: []\nweight: 100\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(packYAML), 0644))

	p, err := content.LoadPack(dir, "")
	require.NoError(t, err)
	assert.Empty(t, p.YouTubeSources)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go build ./internal/content/... && go vet ./internal/content/...`
Expected: Compile error — `p.YouTubeSources` undefined

- [ ] **Step 3: Add YouTubeSource and Video structs to pack.go**

Add after the `Sample` struct (around line 120):

```go
// YouTubeSource declares a playlist or individual video in youtube.yaml.
type YouTubeSource struct {
	ID         string   `yaml:"id"`
	Type       string   `yaml:"type"`
	Name       string   `yaml:"name"`
	PlaylistID string   `yaml:"playlist_id,omitempty"`
	VideoID    string   `yaml:"video_id,omitempty"`
	Tags       []string `yaml:"tags,omitempty"`
	PackID     string
}

// Video is a resolved YouTube video (fetched from a playlist or declared individually).
type Video struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	URL         string    `json:"url"`
	VideoID     string    `json:"video_id"`
	Published   time.Time `json:"published"`
	Description string    `json:"description"`
	Duration    string    `json:"duration,omitempty"`
	SourceID    string    `json:"source_id"`
	Tags        []string  `json:"tags,omitempty"`
	PackID      string    `json:"pack_id"`
}
```

Add fields to the `Pack` struct:

```go
YouTubeSources []YouTubeSource
Videos         []Video
```

- [ ] **Step 4: Add youtube.yaml loading to LoadPack**

Add after the `samples.yaml` loading block (around line 271), following the same pattern:

```go
if data, err := os.ReadFile(filepath.Join(packDir, "youtube.yaml")); err == nil {
	_ = yaml.Unmarshal(data, &pack.YouTubeSources)
	for i := range pack.YouTubeSources {
		pack.YouTubeSources[i].PackID = pack.ID
	}
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go build ./internal/content/... && go vet ./internal/content/...`
Expected: Clean build

- [ ] **Step 6: Commit**

```bash
git add internal/content/pack.go internal/content/pack_test.go
git commit -m "feat(content): add YouTubeSource and Video structs, load youtube.yaml in LoadPack"
```

---

## Task 2: Episode struct extension

**Files:**
- Modify: `internal/youtube/youtube.go:12-18`
- Verify: `internal/youtube/youtube_test.go`

- [ ] **Step 1: Extend Episode struct with Duration and Tags**

In `internal/youtube/youtube.go`, add two fields to `Episode`:

```go
type Episode struct {
	ID          string
	Title       string
	URL         string
	Published   time.Time
	Description string
	Duration    string
	Tags        []string
}
```

- [ ] **Step 2: Verify existing tests still pass**

Run: `go build ./internal/youtube/... && go vet ./internal/youtube/...`
Expected: Clean build (existing tests only check fields that still exist)

- [ ] **Step 3: Commit**

```bash
git add internal/youtube/youtube.go
git commit -m "feat(youtube): extend Episode struct with Duration and Tags fields"
```

---

## Task 3: Credentials service-keyed storage

**Files:**
- Modify: `internal/credentials/credentials.go`
- Modify: `internal/credentials/credentials_test.go`

- [ ] **Step 1: Write failing tests for service-keyed storage**

Add to `internal/credentials/credentials_test.go`:

```go
func TestStoreLoadService_KeychainRoundtrip(t *testing.T) {
	kb := &fakeKeyring{}
	keyringBackend = kb
	dir := t.TempDir()
	require.NoError(t, StoreService(dir, "youtube", "yt-key"))
	tok, err := LoadService(dir, "youtube")
	require.NoError(t, err)
	assert.Equal(t, "yt-key", tok)
}

func TestStoreLoadService_FileRoundtrip(t *testing.T) {
	keyringBackend = unavailableKeyring{err: errors.New("no keychain")}
	dir := t.TempDir()
	require.NoError(t, StoreService(dir, "youtube", "yt-key"))
	tok, err := LoadService(dir, "youtube")
	require.NoError(t, err)
	assert.Equal(t, "yt-key", tok)
	// Verify service-specific file name
	_, statErr := os.Stat(filepath.Join(dir, "credentials-youtube"))
	assert.NoError(t, statErr)
}

func TestDeleteService_RemovesToken(t *testing.T) {
	kb := &fakeKeyring{token: "yt-key"}
	keyringBackend = kb
	dir := t.TempDir()
	require.NoError(t, DeleteService(dir, "youtube"))
	assert.Equal(t, "", kb.token)
}

func TestResolveService_EnvVarWins(t *testing.T) {
	keyringBackend = notFoundKeyring{}
	dir := t.TempDir()
	t.Setenv("YOUTUBE_API_KEY", "env-key")
	assert.Equal(t, "env-key", ResolveService(dir, "youtube", []string{"YOUTUBE_API_KEY"}))
}

func TestResolveService_EmptyWhenNothing(t *testing.T) {
	keyringBackend = notFoundKeyring{}
	dir := t.TempDir()
	t.Setenv("YOUTUBE_API_KEY", "")
	assert.Equal(t, "", ResolveService(dir, "youtube", []string{"YOUTUBE_API_KEY"}))
}

func TestExistingStore_StillWorks(t *testing.T) {
	kb := &fakeKeyring{}
	keyringBackend = kb
	dir := t.TempDir()
	require.NoError(t, Store(dir, "gh-token"))
	tok, err := Load(dir)
	require.NoError(t, err)
	assert.Equal(t, "gh-token", tok)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go build ./internal/credentials/... && go vet ./internal/credentials/...`
Expected: Compile error — `StoreService` undefined

- [ ] **Step 3: Refactor credentials.go with private helpers**

Extract common logic into private helpers that take a `user` parameter:

```go
func storeForUser(configDir, user, token string) error {
	err := keyringBackend.Set(keyringSvc, user, token)
	if err == nil {
		return nil
	}
	fmt.Fprintf(os.Stderr, "keychain unavailable: %v; token stored in credentials file\n", err)
	return writeCredFileForUser(configDir, user, token)
}

func loadForUser(configDir, user string) (string, error) {
	tok, err := keyringBackend.Get(keyringSvc, user)
	if err == nil {
		return tok, nil
	}
	if errors.Is(err, goKeyring.ErrNotFound) {
		return readCredFileForUser(configDir, user)
	}
	fmt.Fprintf(os.Stderr, "keychain unavailable: %v; falling back to credentials file\n", err)
	return readCredFileForUser(configDir, user)
}

func deleteForUser(configDir, user string) error {
	keychainErr := keyringBackend.Delete(keyringSvc, user)
	if keychainErr != nil && !errors.Is(keychainErr, errKeyringNotFound) {
		fmt.Fprintf(os.Stderr, "keychain unavailable: %v; trying credentials file\n", keychainErr)
	}
	path := credFileForUser(configDir, user)
	err := os.Remove(path)
	if os.IsNotExist(err) {
		if keychainErr == nil {
			return nil
		}
		return ErrNotFound
	}
	return err
}

func credFileForUser(configDir, user string) string {
	if user == keyringUser {
		return filepath.Join(configDir, "credentials")
	}
	return filepath.Join(configDir, "credentials-"+user)
}

func writeCredFileForUser(configDir, user, token string) error {
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}
	return os.WriteFile(credFileForUser(configDir, user), []byte(token), 0600)
}

func readCredFileForUser(configDir, user string) (string, error) {
	data, err := os.ReadFile(credFileForUser(configDir, user))
	if os.IsNotExist(err) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	tok := strings.TrimSpace(string(data))
	if tok == "" {
		return "", ErrNotFound
	}
	return tok, nil
}
```

Rewrite existing public functions as thin wrappers:

```go
func Store(configDir, token string) error           { return storeForUser(configDir, keyringUser, token) }
func Load(configDir string) (string, error)          { return loadForUser(configDir, keyringUser) }
func Delete(configDir string) error                   { return deleteForUser(configDir, keyringUser) }
```

Keep `Resolve` unchanged (it calls `Load` which now delegates).

Add service-keyed public functions:

```go
func StoreService(configDir, service, token string) error { return storeForUser(configDir, service, token) }
func LoadService(configDir, service string) (string, error) { return loadForUser(configDir, service) }
func DeleteService(configDir, service string) error { return deleteForUser(configDir, service) }

func ResolveService(configDir, service string, envVars []string) string {
	for _, env := range envVars {
		if v := os.Getenv(env); v != "" {
			return v
		}
	}
	tok, err := LoadService(configDir, service)
	if err == nil {
		return tok
	}
	return ""
}
```

Remove the old private `credFile`, `writeCredFile`, `readCredFile` functions.

- [ ] **Step 4: Run all credential tests**

Run: `go build ./internal/credentials/... && go vet ./internal/credentials/...`
Expected: Clean build; all existing + new tests pass conceptually (CI runs tests)

- [ ] **Step 5: Commit**

```bash
git add internal/credentials/credentials.go internal/credentials/credentials_test.go
git commit -m "feat(credentials): add service-keyed storage for YouTube API keys"
```

---

## Task 4: Config — YouTube sync TTL and token --service flag

**Files:**
- Modify: `internal/config/config.go:22-31` (SyncConfig), `internal/config/config.go:73-85` (Default)
- Modify: `cmd/config.go:134-203` (configTokenCmd, configShowCmd, configSetCmd, init)
- Modify: `internal/i18n/catalogs/en.json`, `internal/i18n/catalogs/de.json`

- [ ] **Step 1: Add YouTube to SyncConfig and Default**

In `internal/config/config.go`, add to `SyncConfig`:

```go
YouTube time.Duration `yaml:"youtube"`
```

In `Default()`, add to the Sync block:

```go
YouTube: 6 * time.Hour,
```

- [ ] **Step 2: Add i18n keys for videos and config**

In `internal/i18n/catalogs/en.json`, add keys:

```json
"config.show.sync_youtube": "sync.youtube:   {{.Value}}",
"config.show.youtube_token": "youtube_api_key: {{.Value}}",
"videos.short": "Browse SAP YouTube videos",
"videos.list.short": "List recent videos for your active profile",
"videos.search.short": "Search across all SAP YouTube videos",
"videos.search.no_results": "No videos found matching \"{{.Query}}\"",
"videos.open.short": "Open a video URL in the default browser",
"videos.open.not_found": "Video \"{{.ID}}\" not found.",
"videos.open.browser_fail": "Could not open browser: {{.Err}}. URL: {{.URL}}",
"videos.open.opening": "Opening: {{.Title}} — {{.URL}}",
"videos.no_videos": "No videos cached. Run `sap-devs sync` first.",
"videos.col_num": "#",
"videos.col_date": "DATE",
"videos.col_pack": "PACK",
"videos.col_source": "SOURCE",
"videos.col_title": "TITLE"
```

Add equivalent German translations in `de.json`.

- [ ] **Step 3: Add --service flag to configTokenCmd**

In `cmd/config.go`, add a `tokenServiceFlag` variable and modify `configTokenCmd.RunE` to dispatch to `StoreService`/`DeleteService` when `--service` is set:

```go
var tokenServiceFlag string
```

In the `RunE` function, wrap the delete path:

```go
if tokenDeleteFlag {
	if tokenServiceFlag != "" {
		err := credentials.DeleteService(paths.ConfigDir, tokenServiceFlag)
		// ... same error handling pattern
	} else {
		err := credentials.Delete(paths.ConfigDir)
		// ... existing logic
	}
}
```

And the store path:

```go
if tokenServiceFlag != "" {
	if err := credentials.StoreService(paths.ConfigDir, tokenServiceFlag, token); err != nil {
		return err
	}
} else {
	if err := credentials.Store(paths.ConfigDir, token); err != nil {
		return err
	}
}
```

Register the flag in `init()`:

```go
configTokenCmd.Flags().StringVar(&tokenServiceFlag, "service", "", "Service to store token for (e.g. youtube)")
```

- [ ] **Step 4: Update config show to display YouTube API key status**

After the GitHub token display in `configShowCmd.RunE`, add:

```go
fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "config.show.sync_youtube", map[string]any{"Value": cfg.Sync.YouTube}))
ytTok, ytErr := credentials.LoadService(paths.ConfigDir, "youtube")
fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "config.show.youtube_token", map[string]any{"Value": maskedToken(ytTok, ytErr, i18n.ActiveLang)}))
```

- [ ] **Step 5: Update config set to handle sync.youtube**

In `configSetCmd.RunE`, add a case:

```go
case "sync.youtube":
	d, err := time.ParseDuration(args[1])
	if err != nil {
		return fmt.Errorf("invalid duration: %w", err)
	}
	cfg.Sync.YouTube = d
```

- [ ] **Step 6: Verify build**

Run: `go build ./... && go vet ./...`
Expected: Clean build

- [ ] **Step 7: Commit**

```bash
git add internal/config/config.go cmd/config.go internal/i18n/catalogs/en.json internal/i18n/catalogs/de.json
git commit -m "feat(config): add YouTube sync TTL, --service flag for token, config show update"
```

---

## Task 5: YouTube API v3 client

**Files:**
- Create: `internal/youtube/apiv3.go`
- Create: `internal/youtube/apiv3_test.go`
- Create: `internal/youtube/testdata/apiv3_playlistitems.json`
- Create: `internal/youtube/testdata/apiv3_videos.json`

- [ ] **Step 1: Create API v3 response testdata fixtures**

Create `internal/youtube/testdata/apiv3_playlistitems.json`:

```json
{
  "items": [
    {
      "snippet": {
        "title": "Build a CAP App in 10 Minutes",
        "description": "Quick tutorial on CAP.",
        "publishedAt": "2026-04-10T12:00:00Z",
        "resourceId": { "videoId": "vid001" }
      }
    },
    {
      "snippet": {
        "title": "CAP with HANA Deep Dive",
        "description": "Advanced HANA integration.",
        "publishedAt": "2026-04-08T10:00:00Z",
        "resourceId": { "videoId": "vid002" }
      }
    }
  ],
  "nextPageToken": ""
}
```

Create `internal/youtube/testdata/apiv3_videos.json`:

```json
{
  "items": [
    {
      "id": "vid001",
      "contentDetails": { "duration": "PT10M30S" },
      "snippet": { "tags": ["cap", "tutorial", "sap"] }
    },
    {
      "id": "vid002",
      "contentDetails": { "duration": "PT45M12S" },
      "snippet": { "tags": ["cap", "hana", "advanced"] }
    }
  ]
}
```

- [ ] **Step 2: Write failing tests for API v3 parsing**

Create `internal/youtube/apiv3_test.go`:

```go
package youtube_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/youtube"
)

func TestParsePlaylistItemsResponse(t *testing.T) {
	data, err := os.ReadFile("testdata/apiv3_playlistitems.json")
	require.NoError(t, err)
	items, nextPage, err := youtube.ParsePlaylistItemsResponse(data)
	require.NoError(t, err)
	assert.Len(t, items, 2)
	assert.Equal(t, "vid001", items[0].VideoID)
	assert.Equal(t, "Build a CAP App in 10 Minutes", items[0].Title)
	assert.Empty(t, nextPage)
}

func TestParseVideosResponse(t *testing.T) {
	data, err := os.ReadFile("testdata/apiv3_videos.json")
	require.NoError(t, err)
	details, err := youtube.ParseVideosResponse(data)
	require.NoError(t, err)
	assert.Len(t, details, 2)
	assert.Equal(t, "PT10M30S", details["vid001"].Duration)
	assert.Contains(t, details["vid001"].Tags, "cap")
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go build ./internal/youtube/... && go vet ./internal/youtube/...`
Expected: Compile error — functions undefined

- [ ] **Step 4: Implement apiv3.go**

Create `internal/youtube/apiv3.go`:

```go
package youtube

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type playlistItemsResponse struct {
	Items         []playlistItem `json:"items"`
	NextPageToken string         `json:"nextPageToken"`
}

type playlistItem struct {
	Snippet playlistItemSnippet `json:"snippet"`
}

type playlistItemSnippet struct {
	Title       string     `json:"title"`
	Description string     `json:"description"`
	PublishedAt string     `json:"publishedAt"`
	ResourceID  resourceID `json:"resourceId"`
}

type resourceID struct {
	VideoID string `json:"videoId"`
}

// PlaylistItemParsed is a partially-parsed playlist item from the API.
type PlaylistItemParsed struct {
	VideoID     string
	Title       string
	Description string
	Published   time.Time
}

// ParsePlaylistItemsResponse parses a YouTube Data API v3 playlistItems.list response.
func ParsePlaylistItemsResponse(data []byte) ([]PlaylistItemParsed, string, error) {
	var resp playlistItemsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, "", fmt.Errorf("youtube: parse playlistItems: %w", err)
	}
	items := make([]PlaylistItemParsed, 0, len(resp.Items))
	for _, it := range resp.Items {
		pub, err := time.Parse(time.RFC3339, it.Snippet.PublishedAt)
		if err != nil {
			return nil, "", fmt.Errorf("youtube: parse publishedAt %q: %w", it.Snippet.PublishedAt, err)
		}
		items = append(items, PlaylistItemParsed{
			VideoID:     it.Snippet.ResourceID.VideoID,
			Title:       it.Snippet.Title,
			Description: it.Snippet.Description,
			Published:   pub,
		})
	}
	return items, resp.NextPageToken, nil
}

type videosResponse struct {
	Items []videoItem `json:"items"`
}

type videoItem struct {
	ID             string         `json:"id"`
	ContentDetails contentDetails `json:"contentDetails"`
	Snippet        videoSnippet   `json:"snippet"`
}

type contentDetails struct {
	Duration string `json:"duration"`
}

type videoSnippet struct {
	Tags []string `json:"tags"`
}

// VideoDetails holds enriched metadata from videos.list.
type VideoDetails struct {
	Duration string
	Tags     []string
}

// ParseVideosResponse parses a YouTube Data API v3 videos.list response.
func ParseVideosResponse(data []byte) (map[string]VideoDetails, error) {
	var resp videosResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("youtube: parse videos: %w", err)
	}
	out := make(map[string]VideoDetails, len(resp.Items))
	for _, v := range resp.Items {
		out[v.ID] = VideoDetails{
			Duration: v.ContentDetails.Duration,
			Tags:     v.Snippet.Tags,
		}
	}
	return out, nil
}

// FetchPlaylistAPI fetches all videos from a YouTube playlist using the Data API v3.
// Paginates through all results and enriches with duration/tags from videos.list.
func FetchPlaylistAPI(playlistID, apiKey string) ([]Episode, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	var allItems []PlaylistItemParsed
	pageToken := ""

	for {
		u := buildPlaylistItemsURL(playlistID, apiKey, pageToken)
		body, err := fetchJSON(client, u)
		if err != nil {
			return nil, err
		}
		items, next, err := ParsePlaylistItemsResponse(body)
		if err != nil {
			return nil, err
		}
		allItems = append(allItems, items...)
		if next == "" {
			break
		}
		pageToken = next
	}

	if len(allItems) == 0 {
		return nil, nil
	}

	// Batch fetch video details (duration, tags) in groups of 50
	detailsMap := make(map[string]VideoDetails)
	ids := make([]string, len(allItems))
	for i, it := range allItems {
		ids[i] = it.VideoID
	}
	for i := 0; i < len(ids); i += 50 {
		end := i + 50
		if end > len(ids) {
			end = len(ids)
		}
		batch := ids[i:end]
		u := buildVideosURL(batch, apiKey)
		body, err := fetchJSON(client, u)
		if err != nil {
			break // Non-fatal: proceed without details
		}
		details, err := ParseVideosResponse(body)
		if err != nil {
			break
		}
		for k, v := range details {
			detailsMap[k] = v
		}
	}

	episodes := make([]Episode, 0, len(allItems))
	for _, it := range allItems {
		ep := Episode{
			ID:          it.VideoID,
			Title:       it.Title,
			URL:         "https://www.youtube.com/watch?v=" + it.VideoID,
			Published:   it.Published,
			Description: it.Description,
		}
		if d, ok := detailsMap[it.VideoID]; ok {
			ep.Duration = d.Duration
			ep.Tags = d.Tags
		}
		episodes = append(episodes, ep)
	}
	return episodes, nil
}

func buildPlaylistItemsURL(playlistID, apiKey, pageToken string) string {
	u := fmt.Sprintf(
		"https://www.googleapis.com/youtube/v3/playlistItems?part=snippet&maxResults=50&playlistId=%s&key=%s",
		url.QueryEscape(playlistID), url.QueryEscape(apiKey),
	)
	if pageToken != "" {
		u += "&pageToken=" + url.QueryEscape(pageToken)
	}
	return u
}

func buildVideosURL(ids []string, apiKey string) string {
	return fmt.Sprintf(
		"https://www.googleapis.com/youtube/v3/videos?part=contentDetails,snippet&id=%s&key=%s",
		url.QueryEscape(strings.Join(ids, ",")), url.QueryEscape(apiKey),
	)
}

func fetchJSON(client *http.Client, u string) ([]byte, error) {
	resp, err := client.Get(u) //nolint:gosec // URLs are constructed from validated inputs
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("youtube: API HTTP %d for %s", resp.StatusCode, u)
	}
	const maxBody = 5 << 20 // 5 MiB
	return io.ReadAll(io.LimitReader(resp.Body, maxBody))
}
```

- [ ] **Step 5: Verify build and tests**

Run: `go build ./internal/youtube/... && go vet ./internal/youtube/...`
Expected: Clean build

- [ ] **Step 6: Commit**

```bash
git add internal/youtube/apiv3.go internal/youtube/apiv3_test.go internal/youtube/testdata/apiv3_playlistitems.json internal/youtube/testdata/apiv3_videos.json
git commit -m "feat(youtube): add YouTube Data API v3 client with parsing and pagination"
```

---

## Task 6: YouTube Resolve function

**Files:**
- Create: `internal/youtube/resolve.go`
- Create: `internal/youtube/resolve_test.go`

- [ ] **Step 1: Write failing test for Resolve**

Create `internal/youtube/resolve_test.go`:

```go
package youtube_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/youtube"
)

func TestResolve_VideoType_NoNetworkCall(t *testing.T) {
	src := content.YouTubeSource{
		ID:      "cap-walkthrough",
		Type:    "video",
		Name:    "CAP Full Walkthrough",
		VideoID: "dQw4w9WgXcQ",
		Tags:    []string{"tutorial"},
	}
	episodes, err := youtube.Resolve(src, "")
	require.NoError(t, err)
	require.Len(t, episodes, 1)
	assert.Equal(t, "dQw4w9WgXcQ", episodes[0].ID)
	assert.Equal(t, "CAP Full Walkthrough", episodes[0].Title)
	assert.Equal(t, "https://www.youtube.com/watch?v=dQw4w9WgXcQ", episodes[0].URL)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go build ./internal/youtube/... && go vet ./internal/youtube/...`
Expected: Compile error — `youtube.Resolve` undefined

- [ ] **Step 3: Implement resolve.go**

Create `internal/youtube/resolve.go`:

```go
package youtube

import (
	"fmt"
	"os"
	"time"

	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
)

// Resolve fetches videos for a YouTubeSource.
// For type:"video", returns a synthetic Episode without a network call.
// For type:"playlist" with apiKey, tries API v3 first, falls back to RSS.
// For type:"playlist" without apiKey, uses RSS directly.
func Resolve(src content.YouTubeSource, apiKey string) ([]Episode, error) {
	switch src.Type {
	case "video":
		return []Episode{{
			ID:        src.VideoID,
			Title:     src.Name,
			URL:       "https://www.youtube.com/watch?v=" + src.VideoID,
			Published: time.Now(),
		}}, nil

	case "playlist":
		rssURL := fmt.Sprintf("https://www.youtube.com/feeds/videos.xml?playlist_id=%s", src.PlaylistID)
		if apiKey != "" {
			eps, err := FetchPlaylistAPI(src.PlaylistID, apiKey)
			if err == nil {
				return eps, nil
			}
			fmt.Fprintf(os.Stderr, "sap-devs: YouTube API error for %s, falling back to RSS: %v\n", src.ID, err)
		}
		return FetchPlaylist(rssURL)

	default:
		return nil, fmt.Errorf("youtube: unknown source type %q", src.Type)
	}
}
```

- [ ] **Step 4: Verify build**

Run: `go build ./internal/youtube/... && go vet ./internal/youtube/...`
Expected: Clean build

- [ ] **Step 5: Commit**

```bash
git add internal/youtube/resolve.go internal/youtube/resolve_test.go
git commit -m "feat(youtube): add Resolve function with API v3/RSS/video dispatch"
```

---

## Task 7: Videos cache package

**Files:**
- Create: `internal/videos/cache.go`
- Create: `internal/videos/videos.go`
- Create: `internal/videos/videos_test.go`
- Create: `internal/videos/testdata/base/sap-dev-news.json`

- [ ] **Step 1: Create test fixture**

Create `internal/videos/testdata/base/sap-dev-news.json`:

```json
[
  {
    "id": "base/sap-dev-news/abc123",
    "title": "SAP Developer News Apr 11 2026",
    "url": "https://www.youtube.com/watch?v=abc123",
    "video_id": "abc123",
    "published": "2026-04-11T12:00:00Z",
    "description": "CAP updates and BTP news.",
    "source_id": "sap-dev-news",
    "tags": ["news"],
    "pack_id": "base"
  },
  {
    "id": "base/sap-dev-news/def456",
    "title": "SAP Developer News Apr 4 2026",
    "url": "https://www.youtube.com/watch?v=def456",
    "video_id": "def456",
    "published": "2026-04-04T12:00:00Z",
    "description": "ABAP Cloud updates.",
    "source_id": "sap-dev-news",
    "tags": ["news"],
    "pack_id": "base"
  }
]
```

- [ ] **Step 2: Write failing tests**

Create `internal/videos/videos_test.go`:

```go
package videos_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/videos"
)

func TestLoadSaveCache_Roundtrip(t *testing.T) {
	dir := t.TempDir()
	vids := []content.Video{
		{ID: "base/news/abc", Title: "Test Video", VideoID: "abc", PackID: "base", SourceID: "news"},
	}
	require.NoError(t, videos.SaveCache(dir, "base", "news", vids))
	loaded, err := videos.LoadCache(dir, "base", "news")
	require.NoError(t, err)
	assert.Len(t, loaded, 1)
	assert.Equal(t, "Test Video", loaded[0].Title)
}

func TestCacheAge_NoFile(t *testing.T) {
	dir := t.TempDir()
	age := videos.CacheAge(dir, "base", "news")
	assert.Less(t, age, 0*age) // negative when no file
}

func TestResolveAll_FromFixture(t *testing.T) {
	// Copy fixture into cache structure
	dir := t.TempDir()
	cacheDir := filepath.Join(dir, "youtube", "base")
	require.NoError(t, os.MkdirAll(cacheDir, 0755))
	data, err := os.ReadFile("testdata/base/sap-dev-news.json")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(cacheDir, "sap-dev-news.json"), data, 0644))

	sources := []content.YouTubeSource{
		{ID: "sap-dev-news", Type: "playlist", PackID: "base"},
	}
	vids, err := videos.ResolveAll(sources, dir)
	require.NoError(t, err)
	assert.Len(t, vids, 2)
}

func TestFilterVideos(t *testing.T) {
	vids := []content.Video{
		{ID: "a", Title: "CAP Tutorial", Tags: []string{"cap"}},
		{ID: "b", Title: "ABAP Basics", Tags: []string{"abap"}},
	}
	result := videos.FilterVideos(vids, "cap")
	assert.Len(t, result, 1)
	assert.Equal(t, "a", result[0].ID)
}

func TestFindVideo(t *testing.T) {
	vids := []content.Video{
		{ID: "base/news/abc"},
		{ID: "cap/tut/def"},
	}
	assert.NotNil(t, videos.FindVideo(vids, "cap/tut/def"))
	assert.Nil(t, videos.FindVideo(vids, "nonexistent"))
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go build ./internal/videos/... && go vet ./internal/videos/...`
Expected: Compile error — package doesn't exist

- [ ] **Step 4: Implement cache.go**

Create `internal/videos/cache.go`:

```go
package videos

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
)

// LoadCache reads cached videos for a given pack and source.
func LoadCache(cacheDir, packID, sourceID string) ([]content.Video, error) {
	data, err := os.ReadFile(cachePath(cacheDir, packID, sourceID))
	if err != nil {
		return nil, err
	}
	var vids []content.Video
	if err := json.Unmarshal(data, &vids); err != nil {
		return nil, err
	}
	return vids, nil
}

// SaveCache writes videos to the cache file for a given pack and source.
func SaveCache(cacheDir, packID, sourceID string, vids []content.Video) error {
	dir := filepath.Join(cacheDir, "youtube", packID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.Marshal(vids)
	if err != nil {
		return err
	}
	return os.WriteFile(cachePath(cacheDir, packID, sourceID), data, 0644)
}

// CacheAge returns the age of the cache file, or a negative duration if it doesn't exist.
func CacheAge(cacheDir, packID, sourceID string) time.Duration {
	info, err := os.Stat(cachePath(cacheDir, packID, sourceID))
	if err != nil {
		return -1
	}
	return time.Since(info.ModTime())
}

func cachePath(cacheDir, packID, sourceID string) string {
	return filepath.Join(cacheDir, "youtube", packID, sourceID+".json")
}
```

- [ ] **Step 5: Implement videos.go**

Create `internal/videos/videos.go`:

```go
package videos

import (
	"strings"

	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
)

// ResolveAll reads cached videos for each source and returns them as a flat slice.
func ResolveAll(sources []content.YouTubeSource, cacheDir string) ([]content.Video, error) {
	var out []content.Video
	for _, src := range sources {
		vids, err := LoadCache(cacheDir, src.PackID, src.ID)
		if err != nil {
			continue // Cache miss — skip silently
		}
		out = append(out, vids...)
	}
	return out, nil
}

// FilterVideos returns videos whose title, description, or any tag contains query
// (case-insensitive substring match).
func FilterVideos(vids []content.Video, query string) []content.Video {
	q := strings.ToLower(query)
	var out []content.Video
	for _, v := range vids {
		if strings.Contains(strings.ToLower(v.Title), q) ||
			strings.Contains(strings.ToLower(v.Description), q) {
			out = append(out, v)
			continue
		}
		for _, tag := range v.Tags {
			if strings.Contains(strings.ToLower(tag), q) {
				out = append(out, v)
				break
			}
		}
	}
	return out
}

// FindVideo returns a pointer to the first video with an exact ID match, or nil.
func FindVideo(vids []content.Video, id string) *content.Video {
	for i := range vids {
		if vids[i].ID == id {
			return &vids[i]
		}
	}
	return nil
}
```

- [ ] **Step 6: Verify build**

Run: `go build ./internal/videos/... && go vet ./internal/videos/...`
Expected: Clean build

- [ ] **Step 7: Commit**

```bash
git add internal/videos/
git commit -m "feat(videos): add cache and helper functions for video resolution"
```

---

## Task 8: Sync engine — archive/independent split and YouTube phase

**Files:**
- Modify: `cmd/sync.go:63-69` (ttls), `cmd/sync.go:71-98` (runSync), `cmd/sync.go:213-243` (runEventsFetch, allCategories)

- [ ] **Step 1: Add `"youtube"` to allCategories and TTL map**

In `cmd/sync.go`, update `allCategories()`:

```go
func allCategories() []string {
	return []string{"tips", "tools", "resources", "context", "mcp", "advocates", "events", "youtube"}
}
```

Add to the `ttls` map in `runSync`:

```go
"youtube": cfg.Sync.YouTube,
```

- [ ] **Step 2: Implement archive/independent category split in runSync**

Replace the current `needsSync` loop with the two-group guard:

```go
archiveCats := []string{"tips", "tools", "resources", "context", "mcp", "advocates"}
independentCats := []string{"events", "youtube"}

activeArchive := intersectStrings(archiveCats, categories)
activeIndependent := intersectStrings(independentCats, categories)

archiveNeedsSync := force
for _, cat := range activeArchive {
	if engine.IsStale(cat) {
		archiveNeedsSync = true
		break
	}
}
independentNeedsSync := force
for _, cat := range activeIndependent {
	if engine.IsStale(cat) {
		independentNeedsSync = true
		break
	}
}

if !archiveNeedsSync && !independentNeedsSync {
	fmt.Fprintln(out, i18n.T(i18n.ActiveLang, "sync.up_to_date"))
	return nil
}
```

Add the helper:

```go
func intersectStrings(a, b []string) []string {
	set := make(map[string]bool, len(b))
	for _, s := range b {
		set[s] = true
	}
	var out []string
	for _, s := range a {
		if set[s] {
			out = append(out, s)
		}
	}
	return out
}
```

- [ ] **Step 3: Guard phases 1-2 with archiveNeedsSync**

Wrap the existing archive download + marker expansion in:

```go
if archiveNeedsSync {
	// Phase 1: archive download (existing)
	// Phase 2: marker expansion (existing)
	if err := engine.MarkAllSynced(activeArchive); err != nil {
		return err
	}
}
```

Move `MarkAllSynced` from its current position into this block, only marking archive categories.

- [ ] **Step 4: Guard phase 3 (events) with independent check**

Wrap `runEventsFetch` with:

```go
if containsString(activeIndependent, "events") && (force || engine.IsStale("events")) {
	if err := runEventsFetch(paths.CacheDir, officialCache, force); err != nil {
		fmt.Fprintf(os.Stderr, "sap-devs: events sync warning: %v\n", err)
	}
	_ = engine.MarkSynced("events")
}
```

Add the helper:

```go
func containsString(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
```

- [ ] **Step 5: Add phase 4 (YouTube fetch)**

After phase 3, add:

```go
if containsString(activeIndependent, "youtube") && (force || engine.IsStale("youtube")) {
	if err := runYouTubeFetch(paths.CacheDir, officialCache, cfg.CompanyRepo, paths.ConfigDir, force); err != nil {
		fmt.Fprintf(os.Stderr, "sap-devs: youtube sync warning: %v\n", err)
	}
	_ = engine.MarkSynced("youtube")
}
```

- [ ] **Step 6: Implement runYouTubeFetch**

Add to `cmd/sync.go`:

```go
func runYouTubeFetch(cacheDir, officialCache, companyRepo, configDir string, force bool) error {
	apiKey := credentials.ResolveService(configDir, "youtube", []string{"YOUTUBE_API_KEY"})

	scanDirs := []string{officialCache}
	if companyRepo != "" {
		scanDirs = append(scanDirs, filepath.Join(filepath.Dir(officialCache), "company"))
	}

	for _, base := range scanDirs {
		packsDir := filepath.Join(base, "content", "packs")
		entries, err := os.ReadDir(packsDir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			packID := entry.Name()
			ytPath := filepath.Join(packsDir, packID, "youtube.yaml")
			data, err := os.ReadFile(ytPath)
			if err != nil {
				continue
			}
			var sources []content.YouTubeSource
			if err := yaml.Unmarshal(data, &sources); err != nil {
				continue
			}
			for _, src := range sources {
				if src.Type != "playlist" {
					continue
				}
				src.PackID = packID
				fetchAndCacheSource(cacheDir, src, apiKey)
			}
		}
	}
	return nil
}

func fetchAndCacheSource(cacheDir string, src content.YouTubeSource, apiKey string) {
	episodes, err := youtube.Resolve(src, apiKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sap-devs: fetch %s/%s: %v\n", src.PackID, src.ID, err)
		return
	}
	vids := make([]content.Video, 0, len(episodes))
	for _, ep := range episodes {
		v := content.Video{
			ID:          fmt.Sprintf("%s/%s/%s", src.PackID, src.ID, ep.ID),
			Title:       ep.Title,
			URL:         ep.URL,
			VideoID:     ep.ID,
			Published:   ep.Published,
			Description: ep.Description,
			Duration:    ep.Duration,
			SourceID:    src.ID,
			PackID:      src.PackID,
		}
		// Merge source tags + episode tags
		tagSet := make(map[string]bool)
		for _, t := range src.Tags {
			tagSet[t] = true
		}
		for _, t := range ep.Tags {
			tagSet[t] = true
		}
		for t := range tagSet {
			v.Tags = append(v.Tags, t)
		}
		vids = append(vids, v)
	}
	_ = videos.SaveCache(cacheDir, src.PackID, src.ID, vids)
}
```

Add imports for `videos` and `youtube` packages.

- [ ] **Step 7: Verify build**

Run: `go build ./... && go vet ./...`
Expected: Clean build

- [ ] **Step 8: Commit**

```bash
git add cmd/sync.go
git commit -m "feat(sync): split archive/independent categories, add YouTube sync phase 4"
```

---

## Task 9: CLI command — `sap-devs videos`

**Files:**
- Create: `cmd/videos.go`

- [ ] **Step 1: Implement videos command**

Create `cmd/videos.go` following the patterns from `cmd/resources.go` and `cmd/news.go`:

```go
package cmd

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/config"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/i18n"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/videos"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/xdg"
)

var videosCmd = &cobra.Command{
	Use:   "videos",
	Short: i18n.T(i18n.ActiveLang, "videos.short"),
	RunE: func(cmd *cobra.Command, args []string) error {
		return videosListCmd.RunE(cmd, args)
	},
}

var videosListN int
var videosListSource string
var videosListPack string

var videosListCmd = &cobra.Command{
	Use:   "list",
	Short: i18n.T(i18n.ActiveLang, "videos.list.short"),
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}
		profileCfg, err := config.LoadProfile(paths.ConfigDir)
		if err != nil {
			return err
		}
		loader, err := newContentLoader()
		if err != nil {
			return err
		}
		var activeProfile *content.ProfileDef
		if profileCfg.ID != "" {
			activeProfile, _ = loader.FindProfile(profileCfg.ID)
		}
		packs, err := loader.LoadPacks(activeProfile, i18n.ActiveLang)
		if err != nil {
			return err
		}
		var allVids []content.Video
		for _, p := range packs {
			vids, _ := videos.ResolveAll(p.YouTubeSources, paths.CacheDir)
			allVids = append(allVids, vids...)
		}
		if videosListSource != "" {
			var filtered []content.Video
			for _, v := range allVids {
				if v.SourceID == videosListSource {
					filtered = append(filtered, v)
				}
			}
			allVids = filtered
		}
		if videosListPack != "" {
			var filtered []content.Video
			for _, v := range allVids {
				if v.PackID == videosListPack {
					filtered = append(filtered, v)
				}
			}
			allVids = filtered
		}
		if len(allVids) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "videos.no_videos"))
			return nil
		}
		sort.Slice(allVids, func(i, j int) bool {
			return allVids[i].Published.After(allVids[j].Published)
		})
		n := videosListN
		if n <= 0 || n > len(allVids) {
			n = len(allVids)
		}
		allVids = allVids[:n]
		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "#\tDATE\tPACK\tSOURCE\tTITLE")
		for i, v := range allVids {
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n",
				i+1, v.Published.Format("2006-01-02"), v.PackID, v.SourceID, v.Title)
		}
		w.Flush()
		return nil
	},
}

var videosSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: i18n.T(i18n.ActiveLang, "videos.search.short"),
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
		var allVids []content.Video
		for _, p := range packs {
			vids, _ := videos.ResolveAll(p.YouTubeSources, paths.CacheDir)
			allVids = append(allVids, vids...)
		}
		matched := videos.FilterVideos(allVids, args[0])
		if len(matched) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "videos.search.no_results", map[string]any{"Query": args[0]}))
			return nil
		}
		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "DATE\tPACK\tSOURCE\tTITLE\tURL")
		for _, v := range matched {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
				v.Published.Format("2006-01-02"), v.PackID, v.SourceID, v.Title, v.URL)
		}
		w.Flush()
		return nil
	},
}

var videosOpenCmd = &cobra.Command{
	Use:   "open <id>",
	Short: i18n.T(i18n.ActiveLang, "videos.open.short"),
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
		var allVids []content.Video
		for _, p := range packs {
			vids, _ := videos.ResolveAll(p.YouTubeSources, paths.CacheDir)
			allVids = append(allVids, vids...)
		}
		sort.Slice(allVids, func(i, j int) bool {
			return allVids[i].Published.After(allVids[j].Published)
		})

		// Support positional index (like news open)
		var target *content.Video
		if id, err := strconv.Atoi(args[0]); err == nil && id >= 1 && id <= len(allVids) {
			target = &allVids[id-1]
		} else {
			target = videos.FindVideo(allVids, args[0])
		}
		if target == nil {
			return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "videos.open.not_found", map[string]any{"ID": args[0]}))
		}
		if err := browser.OpenURL(target.URL); err != nil {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "videos.open.browser_fail", map[string]any{"Err": err, "URL": target.URL}))
			return nil
		}
		fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "videos.open.opening", map[string]any{"Title": target.Title, "URL": target.URL}))
		return nil
	},
}

func init() {
	videosListCmd.Flags().IntVarP(&videosListN, "count", "n", 20, "number of videos to show")
	videosListCmd.Flags().StringVar(&videosListSource, "source", "", "filter by source ID")
	videosListCmd.Flags().StringVar(&videosListPack, "pack", "", "filter by pack ID")
	videosCmd.Flags().IntVarP(&videosListN, "count", "n", 20, "number of videos to show")
	videosCmd.Flags().StringVar(&videosListSource, "source", "", "filter by source ID")
	videosCmd.Flags().StringVar(&videosListPack, "pack", "", "filter by pack ID")
	videosCmd.AddCommand(videosListCmd, videosSearchCmd, videosOpenCmd)
	rootCmd.AddCommand(videosCmd)
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./... && go vet ./...`
Expected: Clean build

- [ ] **Step 3: Commit**

```bash
git add cmd/videos.go
git commit -m "feat(cmd): add videos command with list, search, open subcommands"
```

---

## Task 10: Content YAML files and JSON schema

**Files:**
- Create: `content/packs/base/youtube.yaml`
- Create: `content/packs/cap/youtube.yaml`
- Create: `content/schemas/youtube.schema.json`
- Modify: `.vscode/settings.json`

- [ ] **Step 1: Create base pack youtube.yaml**

```yaml
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
```

- [ ] **Step 2: Create CAP pack youtube.yaml**

```yaml
- id: cap-community-call
  type: playlist
  name: SAP CAP Community Call
  playlist_id: PL6RpkC85SLQDYBNvExhO4LQEQVAQA7VM0
  tags: [cap, community, monthly]
```

- [ ] **Step 3: Create JSON schema**

Create `content/schemas/youtube.schema.json`:

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "YouTube Sources",
  "type": "array",
  "items": {
    "type": "object",
    "required": ["id", "type", "name"],
    "properties": {
      "id": { "type": "string" },
      "type": { "type": "string", "enum": ["playlist", "video"] },
      "name": { "type": "string" },
      "playlist_id": { "type": "string" },
      "video_id": { "type": "string" },
      "tags": { "type": "array", "items": { "type": "string" } }
    },
    "allOf": [
      {
        "if": { "properties": { "type": { "const": "playlist" } } },
        "then": { "required": ["playlist_id"] }
      },
      {
        "if": { "properties": { "type": { "const": "video" } } },
        "then": { "required": ["video_id"] }
      }
    ]
  }
}
```

- [ ] **Step 4: Wire schema in .vscode/settings.json**

Add to the `yaml.schemas` object:

```json
"./content/schemas/youtube.schema.json": "**/packs/*/youtube.yaml"
```

- [ ] **Step 5: Verify build**

Run: `go build ./... && go vet ./...`
Expected: Clean build

- [ ] **Step 6: Commit**

```bash
git add content/packs/base/youtube.yaml content/packs/cap/youtube.yaml content/schemas/youtube.schema.json .vscode/settings.json
git commit -m "feat(content): add youtube.yaml for base and cap packs with JSON schema"
```

---

## Task 11: Documentation updates

**Files:**
- Modify: `CLAUDE.md`
- Modify: `docs/content-authoring.md`

- [ ] **Step 1: Add videos command to CLAUDE.md CLI Commands table**

Add row to the commands table:

```
| `videos` | Browse SAP YouTube videos; `videos list/search/open` |
```

- [ ] **Step 2: Document youtube.yaml in content-authoring.md**

Add a section explaining the youtube.yaml format, the two source types (playlist/video), and how videos are fetched and cached during sync.

- [ ] **Step 3: Commit**

```bash
git add CLAUDE.md docs/content-authoring.md
git commit -m "docs: document videos command and youtube.yaml format"
```

---

## Task 12: Final integration verification

- [ ] **Step 1: Full build check**

Run: `go build ./... && go vet ./...`
Expected: Clean build, no warnings

- [ ] **Step 2: Verify CLI help**

Run: `go run . videos --help`
Expected: Shows list/search/open subcommands

- [ ] **Step 3: Verify sync includes YouTube category**

Run: `go run . sync --help`
Expected: `--category` flag documentation shows youtube is valid

- [ ] **Step 4: Verify config token --service flag**

Run: `go run . config token --help`
Expected: Shows `--service` flag

- [ ] **Step 5: Commit any final fixes**

```bash
git add -A
git commit -m "fix: address integration issues from final verification"
```
