# External API Consumption Reference

This document catalogs every external HTTP call made by the `sap-devs` CLI, grouped by service. Each entry lists the endpoint, HTTP method, response format, parsing approach, timeout, authentication, and fragility assessment.

All calls use Go's `net/http` standard library directly (no third-party HTTP client wrappers).

---

## Fragility Legend

| Rating | Meaning |
|--------|---------|
| **Low** | Stable JSON API with versioned contract; breakage is unlikely and detectable |
| **Medium** | XML feed or undocumented API; format changes are plausible and may break parsing |
| **High** | HTML scraping, complex protocol, or undocumented internal API; silent breakage likely on upstream changes |

---

## 1. GitHub API — Tutorial Repositories

**Package:** [`internal/tutorials/client.go`](../../internal/tutorials/client.go)
**Client timeout:** 30 seconds
**Authentication:** Optional `token` header via `GITHUB_TOOLS_SAP_TOKEN` / `GH_TOKEN` / `GITHUB_TOKEN` env vars or `sap-devs config token`

### 1a. Fetch Repository List

| Field | Value |
|-------|-------|
| **URL** | `https://raw.githubusercontent.com/sap-tutorials/Tutorials/master/config/repository-groups.json` |
| **Method** | `GET` |
| **Response format** | JSON |
| **Parsing** | `json.Unmarshal` into `[]repoGroupEntry` |
| **Headers** | `User-Agent`, `Authorization: token <token>` (optional) |
| **Error handling** | Returns `ErrRepoUnavailable` on HTTP 403/404 |
| **Fragility** | **Low** — static JSON file in a known repo location |

### 1b. Fetch Default Branch

| Field | Value |
|-------|-------|
| **URL** | `https://api.github.com/repos/sap-tutorials/{repo}` |
| **Method** | `GET` |
| **Response format** | JSON |
| **Parsing** | `json.Unmarshal` extracting `default_branch` field |
| **Error handling** | Returns `ErrRepoUnavailable` on HTTP 403/404 |
| **Fragility** | **Low** — standard GitHub REST API v3 |

### 1c. Fetch Repository Tree

| Field | Value |
|-------|-------|
| **URL** | `https://api.github.com/repos/sap-tutorials/{repo}/git/trees/{branch}?recursive=1` |
| **Method** | `GET` |
| **Response format** | JSON |
| **Parsing** | `json.Unmarshal` into tree struct; filters paths starting with `tutorials/` to extract slugs |
| **Error handling** | Returns `ErrRepoUnavailable` on HTTP 403/404 |
| **Fragility** | **Low** — standard GitHub REST API v3 |

### 1d. Fetch Raw Tutorial Markdown

| Field | Value |
|-------|-------|
| **URL** | `https://raw.githubusercontent.com/sap-tutorials/{repo}/{branch}/tutorials/{slug}/{slug}.md` |
| **Method** | `GET` |
| **Response format** | Raw markdown text |
| **Parsing** | Read as string — no structured parsing |
| **Error handling** | Returns `ErrRepoUnavailable` on HTTP 403/404 |
| **Fragility** | **Medium** — depends on a `tutorials/{slug}/{slug}.md` path convention in the sap-tutorials repos |

---

## 2. GitHub API — Archive Download (Sync)

**Package:** [`internal/sync/fetcher.go`](../../internal/sync/fetcher.go)
**Client timeout:** Default (`http.DefaultClient`, no explicit timeout)
**Authentication:** Optional `token` header

| Field | Value |
|-------|-------|
| **URL** | Configured per source (official repo archive URL, company repo URL) |
| **Method** | `GET` |
| **Response format** | ZIP archive |
| **Parsing** | `archive/zip` extraction with top-level directory prefix stripping |
| **Headers** | `Authorization: token <token>` (optional) |
| **Error handling** | Detects auth redirect to `/login` path; zip-slip guard prevents path traversal |
| **Fragility** | **Medium** — relies on GitHub/GitLab archive format convention (single top-level directory) |

---

## 3. GitHub API — Release Check (Update)

**Package:** [`internal/update/checker.go`](../../internal/update/checker.go)
**Client timeout:** 5 seconds

| Field | Value |
|-------|-------|
| **URL** | `https://{host}/api/v3/repos/{owner}/{repo}/releases/latest` (GitHub Enterprise) |
| **Method** | `GET` |
| **Response format** | JSON |
| **Parsing** | `json.NewDecoder().Decode()` extracting `tag_name` field |
| **Headers** | `Accept: application/vnd.github+json`, `Authorization: Bearer <token>` (optional) |
| **Error handling** | Returns `nil` on HTTP 404 (no releases); error on other non-200 codes |
| **Fragility** | **Low** — standard GitHub REST API v3 |

---

## 4. GitHub API — Release Asset Download (Update)

**Package:** [`internal/update/installer.go`](../../internal/update/installer.go)
**Client timeout:** 300 seconds (5 minutes)
**Max body:** 100 MB

### 4a. Checksums Download

| Field | Value |
|-------|-------|
| **URL** | `{repoURL}/releases/download/{tagName}/checksums.txt` |
| **Method** | `GET` |
| **Response format** | Plain text (`<sha256hex>  <filename>` per line) |
| **Parsing** | Line-by-line `strings.Fields` matching asset name |
| **Headers** | `Authorization: Bearer <token>` (optional) |
| **Fragility** | **Low** — GoReleaser-generated format |

### 4b. Archive Download

| Field | Value |
|-------|-------|
| **URL** | `{repoURL}/releases/download/{tagName}/sap-devs_{version}_{os}_{arch}{ext}` |
| **Method** | `GET` |
| **Response format** | Binary (`.tar.gz` on Linux/macOS, `.zip` on Windows) |
| **Parsing** | `archive/tar` + `compress/gzip` or `archive/zip` extraction; SHA-256 checksum verification |
| **Fragility** | **Low** — GoReleaser-generated naming convention |

---

## 5. YouTube RSS Feed (Atom)

**Package:** [`internal/youtube/youtube.go`](../../internal/youtube/youtube.go)
**Client timeout:** 10 seconds
**Max body:** 1 MiB
**Authentication:** None

| Field | Value |
|-------|-------|
| **URL** | `https://www.youtube.com/feeds/videos.xml?playlist_id={playlistId}` |
| **Method** | `GET` |
| **Response format** | XML (Atom feed) |
| **Parsing** | `xml.Unmarshal` into custom `atomFeed`/`atomEntry` structs |
| **Key playlist** | `PL6RpkC85SLQAVBSQXN9522_1jNvPavBgg` (SAP Developer News) |
| **Fragility** | **Medium** — YouTube's Atom feed is undocumented; format changes would silently break parsing |

**Consumed by:**
- `cmd/news.go` — `news latest`, `news list`
- `internal/mcpserver/tools_news.go` — MCP `get_recent_news` tool

---

## 6. YouTube Data API v3

**Package:** [`internal/youtube/apiv3.go`](../../internal/youtube/apiv3.go)
**Client timeout:** 15 seconds
**Max body:** 5 MiB
**Authentication:** API key in URL query parameter (`key=`)

### 6a. Playlist Items

| Field | Value |
|-------|-------|
| **URL** | `https://www.googleapis.com/youtube/v3/playlistItems?part=snippet&maxResults=50&playlistId={id}&key={apiKey}` |
| **Method** | `GET` |
| **Response format** | JSON |
| **Parsing** | `json.Unmarshal` into `playlistItemsResponse`; paginates via `nextPageToken` |
| **Fragility** | **Low** — versioned Google API |

### 6b. Video Details (Batch Enrichment)

| Field | Value |
|-------|-------|
| **URL** | `https://www.googleapis.com/youtube/v3/videos?part=contentDetails,snippet&id={csv-ids}&key={apiKey}` |
| **Method** | `GET` |
| **Response format** | JSON |
| **Parsing** | `json.Unmarshal` into `videosResponse`; batched in groups of 50 |
| **Error handling** | **Non-fatal** — enrichment failures are skipped; episodes returned without duration/tags |
| **Fragility** | **Low** — versioned Google API |

---

## 7. SAP Learning Platform

**Package:** [`internal/learning/`](../../internal/learning/)
**Authentication:** None

### 7a. Catalog Download

**File:** [`internal/learning/catalog.go`](../../internal/learning/catalog.go)

| Field | Value |
|-------|-------|
| **URL** | `https://learning.sap.com/service/catalog-download/json` |
| **Method** | `GET` |
| **Response format** | JSON (large array; ~5.4 MB, ~5,100 items) |
| **Parsing** | `json.Unmarshal` into `[]catalogItem`; filtered to `Learning_type == "Learning Journey"` (~351 items) |
| **Client timeout** | 30 seconds |
| **Caching** | 7-day TTL at `~/.cache/sap-devs/learning/index.json` |
| **Fragility** | **Medium** — undocumented API; field names or response structure could change |

### 7b. Search API

**File:** [`internal/learning/api.go`](../../internal/learning/api.go)

| Field | Value |
|-------|-------|
| **URL** | `https://learning.sap.com/service/learning/search/getCards(types=...,filters=...,sort='',limit=N,page=1)` |
| **Method** | `GET` |
| **Response format** | JSON |
| **Parsing** | `json.Unmarshal` into `searchResponse` struct |
| **Client timeout** | 15 seconds |
| **Query params** | `types` (JSON array), `filters` (JSON object with locale + query), `limit`, `page` |
| **Caching** | 1-hour TTL for search results |
| **Fragility** | **Medium** — undocumented internal API; function-import-style URL pattern |

---

## 8. SAP Discovery Center (OData V2)

**Package:** [`internal/discovery/client.go`](../../internal/discovery/client.go)
**Base URL:** `https://discovery-center.cloud.sap`
**Client timeout:** 15 seconds
**Authentication:** CSRF token (fetched per session)

This is the most complex external integration. Two distinct OData services are consumed:

### 8a. CSRF Token Fetch

| Field | Value |
|-------|-------|
| **URL** | `https://discovery-center.cloud.sap/platformx/` |
| **Method** | `HEAD` |
| **Headers sent** | `x-csrf-token: Fetch`, `x-requested-with: XMLHttpRequest` |
| **Response** | CSRF token extracted from `x-csrf-token` response header |
| **Fragility** | **High** — undocumented; CSRF mechanism could change without notice |

### 8b. Batch Requests (`/platformx/$batch`)

All `/platformx/` function imports are called via OData `$batch`:

| Field | Value |
|-------|-------|
| **URL** | `https://discovery-center.cloud.sap/platformx/$batch` |
| **Method** | `POST` |
| **Content-Type** | `multipart/mixed;boundary=batch_{UnixNano}` |
| **Headers** | `x-csrf-token`, `DataServiceVersion: 2.0`, `MaxDataServiceVersion: 2.0`, `x-requested-with: XMLHttpRequest` |
| **Response format** | Multipart MIME wrapping JSON |
| **Parsing** | Custom `extractBatchJSON()` — finds first `{...}` via brace-depth counting, unwraps `{"d":{"FunctionName":"<double-encoded-json>"}}` envelope |

**Function imports called via batch:**

| Function | Parameters | Returns |
|----------|-----------|---------|
| `GetMissionCatalogContentV2` | `username=''` | `[]MissionCatalogGroup` |
| `GetViewFuzzySearchesCustomV3` | `searchString`, `filterCategory`, `filterType`, `filterProduct`, `filterLoB`, `filterIndustry`, `filterFocusTags`, `filterPartners`, `top` | `[]Mission` |
| `GetProductsCategories` | `version='1'` | `Categories` |
| `GetApplicationFocusTagsIndustryLob` | `version='1'` | `Facets` |
| `GetGuidanceFrameworkTree` | *(none)* | `[]GuidanceNode` |
| `GetGuidanceFrameworkContentById` | `id='<guid>'` | `string` (HTML content) |

**Fragility:** **High** — complex multipart protocol with double-JSON-encoding quirk, custom brace-counting parser, and undocumented function imports. Any protocol or response format change breaks silently.

### 8c. Service Catalog (Direct GET)

| Field | Value |
|-------|-------|
| **URL** | `https://discovery-center.cloud.sap/servicecatalog/ServiceDetailss?$format=json&$select=Id,Name,ShortName,Category,ShortDescription,LicenseModelType,IsDeprecatedService` |
| **Method** | `GET` |
| **Headers** | `Accept: application/json`, `DataServiceVersion: 2.0`, `MaxDataServiceVersion: 2.0` |
| **Response format** | JSON (OData V2 `{"d":{"results":[...]}}` envelope) |
| **Parsing** | `json.Unmarshal` into standard OData wrapper |
| **Fragility** | **Medium** — standard OData but undocumented; field set could change |

---

## 9. SAP Developers Solr Search

**Package:** [`internal/tutorials/enrichment.go`](../../internal/tutorials/enrichment.go)
**Client timeout:** 10 seconds
**Authentication:** None

| Field | Value |
|-------|-------|
| **URL** | `https://developers.sap.com/bin/sapdx/v3/solr/search?json={payload}` |
| **Method** | `GET` |
| **Query payload** | `{"rows":"2000","start":0,"searchField":"","pagePath":"/content/developers/website/languages/en/tutorial-navigator","language":"en_us","addDefaultLanguage":true,"filters":[]}` |
| **Response format** | JSON |
| **Parsing** | `json.Unmarshal` extracting `result[].publicUrl` and `result[].featured` |
| **Headers** | `User-Agent`, `Accept: application/json` |
| **Error handling** | **Fully non-fatal** — returns original index unchanged on any error (HTTP, parse, network) |
| **Fragility** | **High** — undocumented internal SAP CMS endpoint; URL path, query format, and response schema could all change without notice. Failures are silent (returns original data unchanged). |

---

## 10. Khoros Community API (SAP Community Events)

**Package:** [`internal/events/khoros.go`](../../internal/events/khoros.go)
**Client timeout:** Configurable (passed as parameter)
**Max body:** 1 MiB
**Authentication:** None

| Field | Value |
|-------|-------|
| **URL** | `https://groups.community.sap.com/api/2.0/search?q={liql-query}` |
| **Method** | `GET` |
| **Query format** | LiQL (Lithium Query Language): `SELECT id,subject,view_href,occasion_data.* FROM messages WHERE board.id='{boardID}'` |
| **Response format** | JSON |
| **Parsing** | `json.Unmarshal` into `khorosResponse`; checks `status == "success"`; filters out non-occasion items, `/ec-p/` links, and past events |
| **Headers** | `User-Agent: Mozilla/5.0 (compatible; sap-devs/1.0)` |
| **Fragility** | **Medium** — Khoros v2 API is documented but SAP could change board IDs, occasion data fields, or migrate platforms |

---

## 11. RSS Feed Fetching (Generic Events)

**Package:** [`internal/events/rss.go`](../../internal/events/rss.go)
**Client timeout:** Configurable (passed as parameter)
**Max body:** 1 MiB
**Authentication:** None

| Field | Value |
|-------|-------|
| **URL** | Configured per event type in pack YAML |
| **Method** | `GET` |
| **Response format** | XML (RSS 2.0) |
| **Parsing** | `xml.Unmarshal` into `rssFeed`; extracts `title`, `link`, `pubDate`; generates ID via SHA-256 of link |
| **Headers** | `User-Agent: Mozilla/5.0 (compatible; sap-devs/1.0)` |
| **Fragility** | **Medium** — depends on upstream RSS feed availability and standard RSS 2.0 format |

---

## 12. SAP Community Blog Posts

**Package:** [`internal/community/community.go`](../../internal/community/community.go)
**Client timeout:** 10 seconds
**Authentication:** None

### 12a. Blog RSS Feed

| Field | Value |
|-------|-------|
| **URL** | `https://community.sap.com/t5/developer-news/bg-p/developer-news/rss` |
| **Method** | `GET` |
| **Response format** | XML (RSS 2.0) |
| **Parsing** | `xml.Unmarshal` into `rssFeed` struct |
| **Max body** | 1 MiB |
| **Headers** | `User-Agent: Mozilla/5.0 (compatible; sap-devs/1.0)` |
| **Fragility** | **Medium** — standard RSS but URL/board path could change if SAP Community migrates |

### 12b. Blog Post Content Fetch (HTML Scraping)

| Field | Value |
|-------|-------|
| **URL** | Individual blog post URLs from the RSS feed |
| **Method** | `GET` |
| **Response format** | **HTML** |
| **Parsing** | Full HTML page fetched and converted to Markdown via `github.com/JohannesKaufmann/html-to-markdown/v2` |
| **Max body** | 4 MiB |
| **Headers** | `User-Agent: Mozilla/5.0 (compatible; sap-devs/1.0)` |
| **Fragility** | **High** — **HTML scraping**. No DOM scoping or CSS selector is applied; the entire page body is converted. Any change to the SAP Community page template (layout, wrapper divs, navigation HTML) will pollute the markdown output with non-content elements. There is no validation of output quality. |

---

## 13. Dynamic Content Fetch (Sync Markers)

**Package:** [`internal/sync/marker.go`](../../internal/sync/marker.go), [`internal/sync/convert.go`](../../internal/sync/convert.go)
**Client timeout:** 10 seconds (default)
**Max body:** 1 MiB
**Authentication:** None

Content authors embed `<!-- sync:fetch ... -->` HTML comment markers in `context.md` files. During `sap-devs sync`, each marker triggers an HTTP fetch of the specified URL.

| Field | Value |
|-------|-------|
| **URL** | Author-defined (any URL specified in marker attributes) |
| **Method** | `GET` |
| **Response format** | HTML, plain text, or raw (configurable via `format` attribute) |
| **Parsing** | Three-stage pipeline: (1) `html.Parse` via `golang.org/x/net/html`, (2) optional CSS selector scoping via `github.com/andybalholm/cascadia`, (3) conversion via `html-to-markdown` or text extraction |

### Marker Syntax

```html
<!-- sync:fetch url="https://..." format="markdown" selector=".content" max_lines="50" max_tokens="100" ttl_hours="24" label="Release notes" -->
```

| Attribute | Required | Description |
|-----------|----------|-------------|
| `url` | Yes | URL to fetch |
| `format` | No | `raw`, `text`, or `markdown` (default: `markdown`) |
| `selector` | No | CSS selector to scope HTML extraction |
| `max_lines` | No | Truncate output to N lines |
| `max_tokens` | No | Truncate output to ~N tokens (4 chars/token approximation) |
| `ttl_hours` | No | Cache TTL override |
| `label` | No | Human-readable label for progress UI |

### Fragility Assessment: **High** (when `format` is `markdown` or `text` with a `selector`)

This is the most fragile pattern in the codebase because it chains multiple failure-prone steps:

1. **CSS selector depends on remote page structure** — if the target site changes its DOM, the selector silently matches nothing and falls back to the full page body (logged as a warning but sync continues)
2. **HTML-to-Markdown conversion** depends on the `html-to-markdown` library handling arbitrary HTML correctly
3. **Silent degradation** — a broken selector produces garbage content that gets injected into AI tool context without validation

When `format` is `raw`, fragility is **Low** (just a plain HTTP GET).

---

## 14. IP Geolocation

**Package:** [`cmd/config_location.go`](../../cmd/config_location.go)
**Client timeout:** 3 seconds
**Authentication:** None

| Field | Value |
|-------|-------|
| **URL** | `http://ip-api.com/json` (note: plain HTTP, not HTTPS) |
| **Method** | `GET` |
| **Response format** | JSON |
| **Parsing** | `json.NewDecoder().Decode()` extracting `city` and `country` fields |
| **Error handling** | Fully non-fatal; prints error message and returns empty string |
| **Privacy** | User confirmation prompt required before the call is made |
| **Fragility** | **Low** — simple, well-documented public API; but uses plain HTTP (no TLS) |

---

## Summary

### By Fragility

| Rating | Endpoints |
|--------|-----------|
| **High** | Discovery Center batch protocol, SAP Developers Solr search, Community blog HTML scraping, Sync marker HTML+selector pipeline, Discovery Center CSRF |
| **Medium** | Tutorial raw markdown path convention, Sync archive format, Learning catalog/search APIs, Khoros Community API, YouTube RSS feed, Community blog RSS URL, Service catalog OData, Generic RSS feeds |
| **Low** | GitHub REST API (repos, trees, branches, releases), YouTube Data API v3, IP geolocation, GoReleaser asset downloads |

### By Response Format

| Format | Endpoints |
|--------|-----------|
| **JSON** | GitHub API, YouTube API v3, SAP Learning, Discovery Center (wrapped), Khoros, Solr search, IP geolocation, Release check |
| **XML** | YouTube RSS (Atom), Community blog RSS, Generic event RSS |
| **HTML** | Community blog posts, Sync marker fetches (with `format=markdown`/`text`) |
| **Binary** | Sync archive (ZIP), Update asset (TAR.GZ/ZIP) |
| **Plain text** | Tutorial markdown, Checksums, Sync marker fetches (with `format=raw`) |

### By Authentication

| Method | Endpoints |
|--------|-----------|
| **None** | YouTube RSS, SAP Learning, Khoros, Solr, Community, IP geolocation, Sync markers |
| **Bearer/token header** (optional) | GitHub API, Sync archive, Release check/download |
| **API key in URL** | YouTube Data API v3 |
| **CSRF token** (session) | Discovery Center batch |

### Timeout Summary

| Timeout | Endpoints |
|---------|-----------|
| 3s | IP geolocation |
| 5s | Release check |
| 10s | YouTube RSS, Community blog, Solr search, Sync markers |
| 15s | YouTube API v3, Learning search, Discovery Center |
| 30s | Tutorial GitHub client, Learning catalog |
| 300s | Release asset download |
| Default | Sync archive (`http.DefaultClient`) |
