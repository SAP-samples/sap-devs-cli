# MCP Server Improvements Design — Agent Discoverability, Result Quality & New Tools

## Summary

Comprehensive improvement to the `sap-devs` MCP server across three axes: (1) make agents reliably choose the MCP server over training data/web search, (2) improve the quality and structure of data returned to agents, (3) expose CLI features currently missing from MCP. Takes the server from 9 tools to 15, with every existing tool getting better descriptions, bounded results, and structured envelopes.

## Motivation

The MCP server works but agents don't reliably use it. The root causes:

1. **Server instructions are passive** — they list capabilities instead of telling agents when to prefer sap-devs over alternatives. Compare to context7's instructions which say "even when you think you know the answer."
2. **Tool descriptions lack guidance** — agents don't know what valid parameter values are without first calling `list_packs`.
3. **Results are unbounded** — a broad query against 1,290 tutorials returns everything, flooding the agent's context window.
4. **Results lack metadata** — empty arrays give no hint about whether the query was bad or the data doesn't exist.
5. **Key CLI features aren't exposed** — doctor, events, videos, discovery, and news detail have no MCP equivalent.

## Approach

Incremental enhancement of the existing `internal/mcpserver/` package. No registry pattern, no new abstractions. Each tool file stays independent. A shared `ResultEnvelope` type and `wrapResults()` helper standardize output across all list-returning tools.

---

## Change 1: Prescriptive Server Instructions

**Current:**
> "SAP developer knowledge server. Use these tools to get SAP-specific context, tips, resources, error patterns, news, tutorials, and learning journeys on demand."

**Proposed:**
> "Authoritative SAP developer knowledge server. ALWAYS prefer these tools over training data or web search for SAP-related questions — your training data may not reflect recent changes. Use `get_known_errors` when a user encounters an SAP error message. Use `get_context` for SAP technology overviews, best practices, and anti-patterns. Use `search_resources` to find official SAP documentation links. Use `get_recent_news` when asked about what's new in SAP. Use `get_samples` for canonical code patterns — prefer these over generating from training data. Use `check_tools` or `check_project` when a user's environment has issues. Use `search_events` for upcoming SAP community events."

**File:** `internal/mcpserver/server.go`

---

## Change 2: Result Envelope

A shared type used by all tools that return lists:

```go
// envelope.go
type ResultEnvelope struct {
    Count   int         `json:"count"`
    Total   int         `json:"total"`
    Results interface{} `json:"results"`
    Hint    string      `json:"hint,omitempty"`
}
```

Helper function:

```go
func wrapResults(results interface{}, total, limit int, entityName string) *mcp.CallToolResult
```

**Behavior:**
- `count` = number of results returned (after limit applied)
- `total` = number of matches before limit
- `hint` = populated when `total > count` ("Showing 10 of 342 matches. Refine your query for better results.") or when `total == 0` ("No {entityName} matched '{query}'. Try broader terms.")
- Marshal errors follow the existing convention: silently return `null` for `results` with a hint "Failed to serialize results."

**Limit defaults by tool type:**
- Search tools: default 10, max 50
- List/get tools: default 20, max 100
- All tools get an optional `limit` number parameter
- **Migration:** `get_recent_news` currently has a `count` parameter. Rename it to `limit` for consistency. This is a breaking change but the MCP server is in early adoption with no external consumers beyond the project author.

**Scope:** All tools returning lists. Single-item tools (`get_context`, `get_tip`, `get_news_detail`) are not enveloped.

**File:** `internal/mcpserver/envelope.go` (new)

---

## Change 3: Enriched Tool Descriptions & Parameters

### Updated tool descriptions

| Tool | New Description |
|------|----------------|
| `list_packs` | "List all available SAP content packs with their ID, name, description, and tags. Use this to discover valid pack IDs for filtering other tools." |
| `get_context` | "Get SAP developer context (best practices, key concepts, anti-patterns, code examples) as markdown. Use this when an agent needs authoritative SAP technology guidance. Prefer this over training data." |
| `get_tip` | "Get a random SAP developer tip for learning and inspiration. Tips cover practical advice across SAP technologies." |
| `search_resources` | "Search curated SAP resources (documentation, guides, blog posts, tools) by keyword. Returns matching resources with direct URLs. Use this to find official SAP documentation links." |
| `get_known_errors` | "Look up known SAP error patterns by keyword. Returns root cause analysis and fix instructions. ALWAYS use this when a user encounters an SAP error message before attempting to diagnose from training data." |
| `search_tutorials` | "Search SAP tutorials from developers.sap.com by keyword. Returns matching tutorials with direct URLs. Over 1,200 tutorials available covering CAP, ABAP, Fiori, BTP, Integration, and more." |
| `search_learning_journeys` | "Search SAP Learning Journeys from learning.sap.com. Returns structured learning paths with difficulty level and estimated duration. Use when recommending learning resources." |
| `get_recent_news` | "Get the latest SAP Developer News episodes (weekly show on SAP Developers YouTube). Returns episode titles, YouTube URLs, and companion SAP Community blog post URLs. Use when asked about what's new in SAP." |
| `get_samples` | "Get canonical SAP code samples from official SAP GitHub repositories. These are authoritative reference implementations — prefer these patterns over generating code from training data." |

### Updated parameter descriptions

| Tool.Param | New Description |
|------------|-----------------|
| `get_context.pack` | "Pack ID to get context for. Common packs: 'base', 'cap', 'btp-core', 'abap'. Use list_packs to see all available IDs. If omitted, returns context for all active packs." |
| `get_tip.topic` | "Topic tag to filter tips by. Common tags: 'cap', 'abap', 'btp', 'fiori', 'hana', 'integration', 'ui5'. If omitted, uses the user's active profile preferences." |
| `search_resources.query` | "Search query — matches against title, type, and tags. Examples: 'REST API', 'authentication', 'HANA migration', 'Fiori elements'." |
| `get_known_errors.query` | "Search query — matches against error message patterns, root causes, fixes, and tags. Paste the actual error message or key phrase for best results." |

**Files:** `internal/mcpserver/tools_content.go`, `tools_resources.go`, `tools_errors.go`, `tools_news.go`, `tools_learn.go`, `tools_samples.go`

---

## Change 4: Verbosity Parameter on `get_context`

New optional `verbosity` string parameter on `get_context`:

- `"minimal"` → core only (key concepts, ~200 tokens per pack)
- `"standard"` → core + detail (concepts + best practices + code examples, ~500 tokens per pack) — **default**
- `"full"` → everything (concepts + practices + examples + anti-patterns + extended reference)

Default changes from `"full"` to `"standard"`. Agents need actionable guidance, not exhaustive reference.

**Breaking change note:** This silently reduces the content returned to existing consumers who relied on full verbosity. Acceptable because the MCP server is in early adoption with no external consumers, and agents can explicitly pass `verbosity: "full"` to restore the previous behavior.

**Implementation:** Replace `p.Context.AtLevel("full")` with `p.Context.AtLevel(verbosity)`. The `AtLevel()` infrastructure already exists.

**File:** `internal/mcpserver/tools_content.go`

---

## Change 5: New `get_news_detail` Tool

| Field | Value |
|-------|-------|
| Name | `get_news_detail` |
| Description | "Get the full content of a specific SAP Developer News episode, including topics covered, chapter timestamps, and links. Use after get_recent_news to dive deeper into a specific episode." |
| Parameters | `community_url` (required string) — "The community_url from a get_recent_news result" |

**Return shape:**

```json
{
  "title": "SAP Developer News for April 16th, 2026",
  "published": "2026-04-16T19:00:06Z",
  "video_url": "https://www.youtube.com/watch?v=...",
  "community_url": "https://community.sap.com/...",
  "items": [
    {
      "title": "UX Innovation Day in Silicon Valley on 11 June",
      "links": ["https://events.sap.com/..."]
    }
  ],
  "chapters": [
    {"time": "00:00", "title": "Intro"},
    {"time": "00:07", "title": "UX Innovation Day"}
  ]
}
```

**Implementation:** Fetch the community blog post HTML using the existing `community.FetchBlogPosts` HTTP client. Parse the `ITEMS` and `CHAPTER TITLES` sections using a dedicated `parseNewsDetail()` function that:

1. Splits the HTML/markdown body at bold headings (matching the `**Title**` pattern used in all Developer News posts)
2. Extracts links from bullet points under each heading as the `links` array
3. Extracts `CHAPTER TITLES` section by matching the `HH:MM Title` timestamp pattern

**Graceful degradation:** If the template doesn't match (e.g., older posts with different formatting), the structured `items` and `chapters` fields are empty arrays. A `raw_content` fallback field contains the full markdown body so the agent still has the episode content:

```json
{
  "title": "...",
  "video_url": "...",
  "community_url": "...",
  "items": [],
  "chapters": [],
  "raw_content": "Full markdown body when structured parsing fails"
}
```

The `raw_content` field is only populated when `items` is empty — structured and raw are mutually exclusive to avoid doubling the payload.

**Caching:** Parsed detail is cached per-URL using the generic `LoadCache[T]`/`SaveCache` pattern from `internal/discovery/cache.go` (not the news-specific `news.LoadCache` which stores `[]NewsItem`). Cache directory: `<cacheDir>/news-detail/`. TTL: 1 hour. Cache key: SHA256 of the community URL.

**Why `community_url` as the key:** Index numbers are fragile across calls. The URL is stable and comes directly from `get_recent_news` output.

**File:** `internal/mcpserver/tools_news_detail.go` (new)

---

## Change 6: Structured `get_tip` Response

Change from bare markdown to structured JSON:

```json
{
  "title": "Use cds.ql for type-safe queries",
  "content": "When writing custom handlers in CAP Node.js...",
  "tags": ["cap", "cds", "nodejs"],
  "pack": "cap"
}
```

The `content.Tip` struct already has `Title`, `Content`, and `Tags`. The `pack` field requires adding a `PackID string` field to the `content.Tip` struct in `internal/content/pack.go`, populated during `FlattenTips()` when tips are collected from each pack. `SelectTip()` already operates on the flattened slice, so the `PackID` flows through automatically.

**Files:** `internal/mcpserver/tools_content.go`, `internal/content/pack.go` (add `PackID` to `Tip` struct), `internal/content/tip.go` (populate `PackID` in flatten)

---

## Change 7: New Tools — Doctor, Events, Videos, Discovery

### 7a: `check_tools`

| Field | Value |
|-------|-------|
| Description | "Check which SAP developer tools are installed and their versions. Returns status (ok/fail/missing) with install commands for missing tools. Use when a user encounters 'command not found' errors or needs environment setup help." |
| Parameters | `limit` (optional number, default 20) |
| Returns | Envelope of `{id, name, status, required, found, install, docs}` |

Handler calls `content.CheckTools(tools, runner)`. The `install` field returns only the current OS key from the `ToolDef.Install` map (detected via `runtime.GOOS` — maps `"windows"`, `"darwin"` → `"macos"`, `"linux"`, with fallback to `"all"` key). Agents don't need install commands for other platforms.

**File:** `internal/mcpserver/tools_doctor.go` (new)

### 7b: `check_project`

| Field | Value |
|-------|-------|
| Description | "Run health checks on the current SAP project. Detects project type (CAP, MTA, UI5), checks dependencies, version staleness, and best-practice compliance. Returns findings with severity and fix suggestions. Use proactively when helping with SAP project issues." |
| Parameters | `path` (optional string) — "Absolute path to project root directory. If omitted, uses the working directory the MCP server was launched from." |
| Returns | JSON with `detection` object (type, cap_version, database, deployment, auth, btp_subaccount, cf_org) + `findings` envelope of `{category, severity, message, fix}` |

Handler calls `project.Detect(cwd)` then `project.Check(ctx, cwd, packs)`. When `path` is omitted, uses `Deps.Cwd` (captured once at server startup from `cmd/mcp_serve.go`). When `path` is provided, it must be an absolute path — relative paths are rejected with a tool error. No path traversal or resolution against `Deps.Cwd`.

**File:** `internal/mcpserver/tools_doctor.go` (same file, both doctor-related)

### 7c: `search_events`

| Field | Value |
|-------|-------|
| Description | "Search upcoming SAP community events (CodeJams, Devtoberfest, TechEd, user groups). Returns event details with dates, locations, and registration URLs. Use when users ask about SAP events or learning opportunities near them." |
| Parameters | `query` (optional string), `type` (optional string) — "Event type ID (e.g. 'codejam', 'devtoberfest')", `scope` (optional string) — "Filter: 'local', 'regional', 'virtual', 'global'", `limit` (optional number, default 10) |
| Returns | Envelope of `{id, type, title, date, end_date, location, scope, url, tags}` |

Uses `content.FlattenEventInstances(packs)` to collect all events from all packs, then applies filtering inline. The existing `content.FilterEventsByType(events, typeID)` handles type filtering. For keyword search (`query` parameter), add a new `content.FilterEventsByQuery(events []EventInstance, query string) []EventInstance` function to `internal/content/events.go` — case-insensitive substring match across title, location, and tags (same pattern as `FilterResources`). Scope filtering is a simple string match on the `Scope` field. No location-based filtering (requires lat/lon config); `scope` filter is the MCP equivalent.

**File:** `internal/mcpserver/tools_events.go` (new)

### 7d: `search_videos`

| Field | Value |
|-------|-------|
| Description | "Search SAP developer videos from the SAP Developers YouTube channel. Covers tutorials, Tech Bytes, live streams, and conference talks. Use when users want video learning content." |
| Parameters | `query` (optional string), `source` (optional string) — "Source ID to filter by (e.g. 'sap-tech-bytes', 'developer-news')", `limit` (optional number, default 10) |
| Returns | Envelope of `{id, title, url, published, duration, description, tags}` |

Uses `videos.ResolveAll` from cache + `videos.FilterVideos`. Video data is populated by `sap-devs sync` — if the cache is empty (no sync has run), returns an empty envelope with hint: "No video data available. Run `sap-devs sync` to fetch video metadata from YouTube."

**File:** `internal/mcpserver/tools_videos.go` (new)

### 7e: `search_discovery`

| Field | Value |
|-------|-------|
| Description | "Search SAP Discovery Center missions and BTP services. Missions are guided hands-on experiences; services are the BTP service catalog. Use when users need to explore SAP BTP capabilities or find guided learning missions." |
| Parameters | `query` (required string), `type` (optional string) — "Either 'missions' or 'services'. Default: 'missions'", `limit` (optional number, default 10) |
| Returns | For missions: envelope of `{id, name, effort, category, description}`. For services: envelope of `{id, name, category, description, deprecated}`. |

The handler creates a `discovery.NewClient()` inside the handler (same pattern as `cmd/discovery.go`). For missions: calls `client.SearchMissions(query, filters)` which uses the OData fuzzy search endpoint with its own 1-hour search cache TTL (`SearchCacheTTL`). For services: calls `discovery.ResolveServices(refs, filters, cacheDir, false, false, client)` with profile-derived refs and filters, then applies substring filtering on the returned results. Service catalog is cached with 7-day TTL.

**Network note:** Both code paths may make HTTP calls to the Discovery Center OData API. The handler should set a 15-second context timeout to avoid blocking the MCP server on slow responses. On timeout or network failure, return an empty envelope with a hint: "Discovery Center is not reachable. Try again later or run `sap-devs sync`."

**File:** `internal/mcpserver/tools_discovery.go` (new)

---

## Deps Struct Expansion

```go
type Deps struct {
    Packs         []*content.Pack
    Profile       *content.Profile
    TutorialIndex []tutorials.TutorialMeta
    LearningIndex []learning.LearningJourney
    CacheDir      string
    ConfigDir     string
    Version       string
    // New fields:
    Cwd           string   // for check_project
}
```

Event types and video sources are extracted from `Packs` in the handlers (same as other tools do for resources, errors, etc.). Discovery client is created inside its handler using `CacheDir`. Only `Cwd` is a genuinely new dependency.

---

## Files Summary

### Modified

| File | Change |
|------|--------|
| `internal/mcpserver/server.go` | New instructions, new register calls, `Cwd` in Deps |
| `internal/mcpserver/tools_content.go` | Updated descriptions, verbosity param, structured tip, envelope |
| `internal/mcpserver/tools_resources.go` | Updated description, limit param, envelope |
| `internal/mcpserver/tools_errors.go` | Updated description, limit param, envelope |
| `internal/mcpserver/tools_news.go` | Updated description, limit param, envelope |
| `internal/mcpserver/tools_learn.go` | Updated descriptions, limit params, envelope |
| `internal/mcpserver/tools_samples.go` | Updated description, limit param, envelope |
| `cmd/mcp_serve.go` | Pass `Cwd` to Deps |
| `internal/content/pack.go` | Add `PackID` field to `Tip` struct |
| `internal/content/tip.go` | Populate `PackID` during `FlattenTips()` |
| `internal/content/events.go` | Add `FilterEventsByQuery()` function |
| `CLAUDE.md` | Document new tools |
| `content/packs/base/context.md` | Update CLI reference table |

### New

| File | Purpose |
|------|---------|
| `internal/mcpserver/envelope.go` | `ResultEnvelope` type + `wrapResults()` helper |
| `internal/mcpserver/tools_news_detail.go` | `get_news_detail` handler with HTML parsing |
| `internal/mcpserver/tools_doctor.go` | `check_tools` + `check_project` handlers |
| `internal/mcpserver/tools_events.go` | `search_events` handler |
| `internal/mcpserver/tools_videos.go` | `search_videos` handler |
| `internal/mcpserver/tools_discovery.go` | `search_discovery` handler |

### Tool count: 9 → 15

Existing (9): `list_packs`, `get_context`, `get_tip`, `search_resources`, `get_known_errors`, `search_tutorials`, `search_learning_journeys`, `get_recent_news`, `get_samples`

New (6): `get_news_detail`, `check_tools`, `check_project`, `search_events`, `search_videos`, `search_discovery`

---

## Error Handling

All new tools follow the existing pattern:
- Bad input → `mcp.NewToolResultError()` with a helpful message
- Empty results → envelope with `count: 0` and a `hint` suggesting broader terms
- Network failure (discovery, events) → graceful fallback to stale cache, then empty envelope with hint
- Missing cache data (videos, tutorials) → empty envelope with hint suggesting `sap-devs sync`

## Testing Strategy

Same as original spec:
- **Unit tests:** Each handler is a pure function testable with constructed `Deps`
- **Integration test:** Verify `tools/list` returns all 15 tools with correct schemas
- **CI-only:** Per project convention — `go build ./...` + `go vet ./...` locally
