# MCP Server Improvements Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Improve the sap-devs MCP server from 9 to 15 tools with better agent discoverability, structured result envelopes, and new tools for doctor, events, videos, discovery, and news detail.

**Architecture:** Incremental enhancement of `internal/mcpserver/`. A shared `ResultEnvelope` + `wrapResults()` helper standardizes all list outputs. New tools follow the existing thin-handler pattern — one file per category calling existing `internal/` infrastructure. No new abstractions or registries.

**Tech Stack:** Go, mark3labs/mcp-go SDK, existing internal packages (content, discovery, community, videos, project, news)

**Spec:** `docs/superpowers/specs/2026-04-21-mcp-server-improvements-design.md`

**Windows note:** `go test` always fails locally due to Windows Defender. Use `go build ./...` + `go vet ./...` locally. CI (ubuntu-latest) is the authoritative test runner.

---

## File Map

### New files

| File | Responsibility |
|------|---------------|
| `internal/mcpserver/envelope.go` | `ResultEnvelope` struct, `wrapResults()` helper, `clampLimit()` helper |
| `internal/mcpserver/tools_news_detail.go` | `get_news_detail` handler + `parseNewsDetail()` markdown parser |
| `internal/mcpserver/tools_doctor.go` | `check_tools` + `check_project` handlers, `execRunner()` helper |
| `internal/mcpserver/tools_events.go` | `search_events` handler |
| `internal/mcpserver/tools_videos.go` | `search_videos` handler |
| `internal/mcpserver/tools_discovery.go` | `search_discovery` handler |

### Modified files

| File | Change |
|------|--------|
| `internal/mcpserver/server.go` | New instructions string, `Cwd` in Deps, new `register*Tools()` calls |
| `internal/mcpserver/tools_content.go` | Updated descriptions, verbosity param, structured tip, envelope on `list_packs` |
| `internal/mcpserver/tools_resources.go` | Updated description, `limit` param, envelope |
| `internal/mcpserver/tools_errors.go` | Updated description, `limit` param, envelope |
| `internal/mcpserver/tools_news.go` | Updated description, rename `count`→`limit`, envelope |
| `internal/mcpserver/tools_learn.go` | Updated descriptions, `limit` params, envelope |
| `internal/mcpserver/tools_samples.go` | Updated description, `limit` param, envelope |
| `internal/content/pack.go` | Add `PackID string` to `Tip` struct, populate in `LoadPack()` |
| `internal/content/events.go` | Add `FilterEventsByQuery()` function |
| `cmd/mcp_serve.go` | Pass `Cwd` to Deps |
| `CLAUDE.md` | Document new tools in MCP server section |
| `content/packs/base/context.md` | Update CLI reference table |

---

### Task 1: Result Envelope Infrastructure

**Files:**
- Create: `internal/mcpserver/envelope.go`

This is the foundation — every subsequent task depends on it.

- [ ] **Step 1: Create `envelope.go` with types and helpers**

```go
package mcpserver

import (
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

// ResultEnvelope wraps list results with count, total, and optional hint for agents.
type ResultEnvelope struct {
	Count   int         `json:"count"`
	Total   int         `json:"total"`
	Results interface{} `json:"results"`
	Hint    string      `json:"hint,omitempty"`
}

func clampLimit(requested, defaultVal, maxVal int) int {
	if requested <= 0 {
		return defaultVal
	}
	if requested > maxVal {
		return maxVal
	}
	return requested
}

func wrapResults(results interface{}, total, count int, entityName, query string) *mcp.CallToolResult {
	env := ResultEnvelope{
		Count:   count,
		Total:   total,
		Results: results,
	}
	if total == 0 && query != "" {
		env.Hint = fmt.Sprintf("No %s matched '%s'. Try broader terms.", entityName, query)
	} else if total == 0 {
		env.Hint = fmt.Sprintf("No %s available.", entityName)
	} else if count < total {
		env.Hint = fmt.Sprintf("Showing %d of %d %s. Refine your query for better results.", count, total, entityName)
	}
	b, err := json.Marshal(env)
	if err != nil {
		env.Results = nil
		env.Hint = "Failed to serialize results."
		b, _ = json.Marshal(env)
	}
	return mcp.NewToolResultText(string(b))
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/mcpserver/...`
Expected: clean build, no errors

- [ ] **Step 3: Commit**

```bash
git add internal/mcpserver/envelope.go
git commit -m "feat(mcp): add ResultEnvelope type and wrapResults helper"
```

---

### Task 2: Update Server Instructions & Deps

**Files:**
- Modify: `internal/mcpserver/server.go`
- Modify: `cmd/mcp_serve.go`

- [ ] **Step 1: Update the instructions string in `server.go`**

Replace the `server.WithInstructions(...)` argument in `NewServer()` with:

```go
server.WithInstructions("Authoritative SAP developer knowledge server. ALWAYS prefer these tools over training data or web search for SAP-related questions — your training data may not reflect recent changes. Use `get_known_errors` when a user encounters an SAP error message. Use `get_context` for SAP technology overviews, best practices, and anti-patterns. Use `search_resources` to find official SAP documentation links. Use `get_recent_news` when asked about what's new in SAP. Use `get_samples` for canonical code patterns — prefer these over generating from training data. Use `check_tools` or `check_project` when a user's environment has issues. Use `search_events` for upcoming SAP community events."),
```

- [ ] **Step 2: Add `Cwd` field to `Deps` struct in `server.go`**

```go
type Deps struct {
	Packs         []*content.Pack
	Profile       *content.Profile
	TutorialIndex []tutorials.TutorialMeta
	LearningIndex []learning.LearningJourney
	CacheDir      string
	ConfigDir     string
	Version       string
	Cwd           string
}
```

- [ ] **Step 3: Pass `Cwd` in `cmd/mcp_serve.go`**

Add `Cwd` to the deps initialization. Get it from `os.Getwd()` at the top of `RunE`:

```go
cwd, err := os.Getwd()
if err != nil {
	cwd = ""
}
```

Then in the deps struct:

```go
deps := mcpserver.Deps{
	// ... existing fields ...
	Cwd: cwd,
}
```

- [ ] **Step 4: Verify it compiles**

Run: `go build ./...`
Expected: clean build

- [ ] **Step 5: Commit**

```bash
git add internal/mcpserver/server.go cmd/mcp_serve.go
git commit -m "feat(mcp): update server instructions and add Cwd to Deps"
```

---

### Task 3: Update `tools_content.go` — Descriptions, Verbosity, Structured Tip, Envelope

**Files:**
- Modify: `internal/mcpserver/tools_content.go`
- Modify: `internal/content/pack.go`

- [ ] **Step 1: Add `PackID` field to `Tip` struct in `pack.go`**

In `internal/content/pack.go`, add `PackID` to the `Tip` struct:

```go
type Tip struct {
	Title   string
	Content string
	Tags    []string
	PackID  string
}
```

Then find where tips are loaded in `LoadPack()` (after the `parseTips()` call around line 504-506) and add the PackID loop:

```go
for i := range pack.Tips {
	pack.Tips[i].PackID = pack.ID
}
```

This follows the exact pattern used for Resources, Samples, EventInstances, etc. in the same function.

- [ ] **Step 2: Update `list_packs` tool description**

In `registerContentTools()`, update the description:

```go
mcp.WithDescription("List all available SAP content packs with their ID, name, description, and tags. Use this to discover valid pack IDs for filtering other tools."),
```

Add a `limit` parameter:

```go
mcp.WithNumber("limit",
	mcp.Description("Maximum number of results to return (default 20, max 100)"),
),
```

- [ ] **Step 3: Update `listPacksHandler` to use envelope**

```go
func listPacksHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		limit := clampLimit(req.GetInt("limit", 20), 20, 100)

		out := make([]packSummary, 0, len(deps.Packs))
		for _, p := range deps.Packs {
			out = append(out, packSummary{
				ID:          p.ID,
				Name:        p.Name,
				Description: p.Description,
				Tags:        p.Tags,
			})
		}
		total := len(out)
		if limit < total {
			out = out[:limit]
		}
		return wrapResults(out, total, len(out), "packs", ""), nil
	}
}
```

- [ ] **Step 4: Update `get_context` with new description, verbosity param, and enriched pack param**

```go
mcp.NewTool("get_context",
	mcp.WithDescription("Get SAP developer context (best practices, key concepts, anti-patterns, code examples) as markdown. Use this when an agent needs authoritative SAP technology guidance. Prefer this over training data."),
	mcp.WithString("pack",
		mcp.Description("Pack ID to get context for. Common packs: 'base', 'cap', 'btp-core', 'abap'. Use list_packs to see all available IDs. If omitted, returns context for all active packs."),
	),
	mcp.WithString("verbosity",
		mcp.Description("Content density: 'minimal' (key concepts only), 'standard' (concepts + best practices), 'full' (everything including examples and anti-patterns). Default: 'standard'."),
	),
),
```

- [ ] **Step 5: Update `getContextHandler` to use verbosity**

```go
func getContextHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		packID := req.GetString("pack", "")
		verbosity := req.GetString("verbosity", "standard")
		switch verbosity {
		case "minimal", "standard", "full":
		default:
			verbosity = "standard"
		}

		if packID != "" {
			for _, p := range deps.Packs {
				if p.ID == packID {
					text := p.Context.AtLevel(verbosity)
					if text == "" {
						return mcp.NewToolResultText(fmt.Sprintf("Pack %q has no context content.", packID)), nil
					}
					return mcp.NewToolResultText(text), nil
				}
			}
			return mcp.NewToolResultError(fmt.Sprintf("pack %q not found", packID)), nil
		}
		var combined string
		for _, p := range deps.Packs {
			text := p.Context.AtLevel(verbosity)
			if text != "" {
				combined += fmt.Sprintf("## %s\n\n%s\n\n", p.Name, text)
			}
		}
		if combined == "" {
			return mcp.NewToolResultText("No context content available."), nil
		}
		return mcp.NewToolResultText(combined), nil
	}
}
```

- [ ] **Step 6: Update `get_tip` to structured JSON response**

Update the tool description:

```go
mcp.WithDescription("Get a random SAP developer tip for learning and inspiration. Tips cover practical advice across SAP technologies."),
mcp.WithString("topic",
	mcp.Description("Topic tag to filter tips by. Common tags: 'cap', 'abap', 'btp', 'fiori', 'hana', 'integration', 'ui5'. If omitted, uses the user's active profile preferences."),
),
```

Add a new result type and update the handler:

```go
type tipResult struct {
	Title   string   `json:"title"`
	Content string   `json:"content"`
	Tags    []string `json:"tags"`
	Pack    string   `json:"pack"`
}

func getTipHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		topic := req.GetString("topic", "")
		var tags []string
		if topic != "" {
			tags = []string{topic}
		} else if deps.Profile != nil {
			tags = deps.Profile.TipTags
		}
		seed := time.Now().UnixNano()
		tip, err := content.SelectTip(deps.Packs, tags, seed)
		if err != nil {
			return mcp.NewToolResultText("No tips available for the given topic."), nil
		}
		result := tipResult{
			Title:   tip.Title,
			Content: tip.Content,
			Tags:    tip.Tags,
			Pack:    tip.PackID,
		}
		b, _ := json.Marshal(result)
		return mcp.NewToolResultText(string(b)), nil
	}
}
```

- [ ] **Step 7: Verify it compiles**

Run: `go build ./...`
Expected: clean build

- [ ] **Step 8: Commit**

```bash
git add internal/mcpserver/tools_content.go internal/content/pack.go
git commit -m "feat(mcp): update content tools with envelope, verbosity, structured tip"
```

---

### Task 4: Update `tools_resources.go` — Description, Limit, Envelope

**Files:**
- Modify: `internal/mcpserver/tools_resources.go`

- [ ] **Step 1: Update tool registration with new description and limit param**

```go
mcp.NewTool("search_resources",
	mcp.WithDescription("Search curated SAP resources (documentation, guides, blog posts, tools) by keyword. Returns matching resources with direct URLs. Use this to find official SAP documentation links."),
	mcp.WithString("query",
		mcp.Required(),
		mcp.Description("Search query — matches against title, type, and tags. Examples: 'REST API', 'authentication', 'HANA migration', 'Fiori elements'."),
	),
	mcp.WithString("pack",
		mcp.Description("Filter to resources from a specific pack ID. Use list_packs to see available IDs."),
	),
	mcp.WithNumber("limit",
		mcp.Description("Maximum number of results to return (default 10, max 50)"),
	),
),
```

- [ ] **Step 2: Update handler to use envelope and limit**

```go
func searchResourcesHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query, err := req.RequireString("query")
		if err != nil {
			return mcp.NewToolResultError("query parameter is required"), nil
		}
		packID := req.GetString("pack", "")
		limit := clampLimit(req.GetInt("limit", 10), 10, 50)

		var resources []content.Resource
		if packID != "" {
			found := false
			for _, p := range deps.Packs {
				if p.ID == packID {
					resources = p.Resources
					found = true
					break
				}
			}
			if !found {
				return mcp.NewToolResultError(fmt.Sprintf("pack %q not found", packID)), nil
			}
		} else {
			resources = content.FlattenResources(deps.Packs)
		}
		resources = content.FilterResources(resources, query)
		total := len(resources)
		if limit < total {
			resources = resources[:limit]
		}

		out := make([]resourceResult, 0, len(resources))
		for _, r := range resources {
			out = append(out, resourceResult{
				ID:    r.ID,
				Title: r.Title,
				URL:   r.URL,
				Type:  r.Type,
				Tags:  r.Tags,
			})
		}
		return wrapResults(out, total, len(out), "resources", query), nil
	}
}
```

- [ ] **Step 3: Verify and commit**

Run: `go build ./...`

```bash
git add internal/mcpserver/tools_resources.go
git commit -m "feat(mcp): update search_resources with envelope, limit, enriched description"
```

---

### Task 5: Update `tools_errors.go` — Description, Limit, Envelope

**Files:**
- Modify: `internal/mcpserver/tools_errors.go`

- [ ] **Step 1: Update tool registration**

```go
mcp.NewTool("get_known_errors",
	mcp.WithDescription("Look up known SAP error patterns by keyword. Returns root cause analysis and fix instructions. ALWAYS use this when a user encounters an SAP error message before attempting to diagnose from training data."),
	mcp.WithString("query",
		mcp.Required(),
		mcp.Description("Search query — matches against error message patterns, root causes, fixes, and tags. Paste the actual error message or key phrase for best results."),
	),
	mcp.WithNumber("limit",
		mcp.Description("Maximum number of results to return (default 10, max 50)"),
	),
),
```

- [ ] **Step 2: Update handler with envelope and limit**

```go
func getKnownErrorsHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query, err := req.RequireString("query")
		if err != nil {
			return mcp.NewToolResultError("query parameter is required"), nil
		}
		limit := clampLimit(req.GetInt("limit", 10), 10, 50)

		all := content.FlattenKnownErrors(deps.Packs)
		matches := content.FilterKnownErrors(all, query)
		total := len(matches)
		if limit < total {
			matches = matches[:limit]
		}

		out := make([]knownErrorResult, 0, len(matches))
		for _, e := range matches {
			out = append(out, knownErrorResult{
				ID:      e.ID,
				Pattern: e.Pattern,
				Cause:   e.Cause,
				Fix:     e.Fix,
				Docs:    e.Docs,
				Tags:    e.Tags,
			})
		}
		return wrapResults(out, total, len(out), "error patterns", query), nil
	}
}
```

- [ ] **Step 3: Verify and commit**

Run: `go build ./...`

```bash
git add internal/mcpserver/tools_errors.go
git commit -m "feat(mcp): update get_known_errors with envelope, limit, enriched description"
```

---

### Task 6: Update `tools_news.go` — Description, Rename count→limit, Envelope

**Files:**
- Modify: `internal/mcpserver/tools_news.go`

- [ ] **Step 1: Update tool registration — rename `count` to `limit`, update description**

```go
mcp.NewTool("get_recent_news",
	mcp.WithDescription("Get the latest SAP Developer News episodes (weekly show on SAP Developers YouTube). Returns episode titles, YouTube URLs, and companion SAP Community blog post URLs. Use when asked about what's new in SAP."),
	mcp.WithNumber("limit",
		mcp.Description("Maximum number of episodes to return (default 5, max 50)"),
	),
),
```

- [ ] **Step 2: Update handler — use `limit` param name, add envelope**

```go
func getRecentNewsHandler(fetcher *newsFetcher) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		limit := clampLimit(req.GetInt("limit", 5), 5, 50)

		items := fetcher.get()
		total := len(items)
		if total == 0 {
			return wrapResults([]newsResult{}, 0, 0, "news episodes", ""), nil
		}
		if limit > total {
			limit = total
		}

		out := make([]newsResult, 0, limit)
		for _, item := range items[:limit] {
			nr := newsResult{
				Title:     item.Episode.Title,
				URL:       item.Episode.URL,
				Published: item.Episode.Published.Format(time.RFC3339),
			}
			if item.Community != nil {
				nr.CommunityURL = item.Community.URL
			}
			out = append(out, nr)
		}
		return wrapResults(out, total, len(out), "news episodes", ""), nil
	}
}
```

- [ ] **Step 3: Verify and commit**

Run: `go build ./...`

```bash
git add internal/mcpserver/tools_news.go
git commit -m "feat(mcp): update get_recent_news with envelope, rename count to limit"
```

---

### Task 7: Update `tools_learn.go` — Descriptions, Limits, Envelope

**Files:**
- Modify: `internal/mcpserver/tools_learn.go`

- [ ] **Step 1: Update both tool registrations**

```go
mcp.NewTool("search_tutorials",
	mcp.WithDescription("Search SAP tutorials from developers.sap.com by keyword. Returns matching tutorials with direct URLs. Over 1,200 tutorials available covering CAP, ABAP, Fiori, BTP, Integration, and more."),
	mcp.WithString("query",
		mcp.Required(),
		mcp.Description("Search query (matches against title, description, tags)"),
	),
	mcp.WithNumber("limit",
		mcp.Description("Maximum number of results to return (default 10, max 50)"),
	),
),

mcp.NewTool("search_learning_journeys",
	mcp.WithDescription("Search SAP Learning Journeys from learning.sap.com. Returns structured learning paths with difficulty level and estimated duration. Use when recommending learning resources."),
	mcp.WithString("query",
		mcp.Required(),
		mcp.Description("Search query (matches against title, description, level)"),
	),
	mcp.WithNumber("limit",
		mcp.Description("Maximum number of results to return (default 10, max 50)"),
	),
),
```

- [ ] **Step 2: Update `searchTutorialsHandler` with envelope and limit**

```go
func searchTutorialsHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query, err := req.RequireString("query")
		if err != nil {
			return mcp.NewToolResultError("query parameter is required"), nil
		}
		limit := clampLimit(req.GetInt("limit", 10), 10, 50)

		if len(deps.TutorialIndex) == 0 {
			return wrapResults([]tutorialResult{}, 0, 0, "tutorials", query), nil
		}
		matches := tutorials.Search(deps.TutorialIndex, query)
		total := len(matches)
		if limit < total {
			matches = matches[:limit]
		}
		out := make([]tutorialResult, 0, len(matches))
		for _, t := range matches {
			out = append(out, tutorialResult{
				Slug:        t.Slug,
				Title:       t.Title,
				Description: t.Description,
				URL:         t.URL,
				Tags:        t.Tags,
			})
		}
		return wrapResults(out, total, len(out), "tutorials", query), nil
	}
}
```

- [ ] **Step 3: Update `searchLearningJourneysHandler` with envelope and limit**

```go
func searchLearningJourneysHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query, err := req.RequireString("query")
		if err != nil {
			return mcp.NewToolResultError("query parameter is required"), nil
		}
		limit := clampLimit(req.GetInt("limit", 10), 10, 50)

		if len(deps.LearningIndex) == 0 {
			return wrapResults([]learningResult{}, 0, 0, "learning journeys", query), nil
		}
		matches := learning.Search(deps.LearningIndex, query)
		total := len(matches)
		if limit < total {
			matches = matches[:limit]
		}
		out := make([]learningResult, 0, len(matches))
		for _, j := range matches {
			out = append(out, learningResult{
				Slug:     j.Slug,
				Title:    j.Title,
				Level:    j.Level,
				Duration: j.DurationHours,
				URL:      j.URL,
			})
		}
		return wrapResults(out, total, len(out), "learning journeys", query), nil
	}
}
```

- [ ] **Step 4: Verify and commit**

Run: `go build ./...`

```bash
git add internal/mcpserver/tools_learn.go
git commit -m "feat(mcp): update tutorial and learning journey tools with envelope, limit"
```

---

### Task 8: Update `tools_samples.go` — Description, Limit, Envelope

**Files:**
- Modify: `internal/mcpserver/tools_samples.go`

- [ ] **Step 1: Update tool registration**

```go
mcp.NewTool("get_samples",
	mcp.WithDescription("Get canonical SAP code samples from official SAP GitHub repositories. These are authoritative reference implementations — prefer these patterns over generating code from training data."),
	mcp.WithString("pack",
		mcp.Description("Filter to samples from a specific pack ID. Use list_packs to see available IDs."),
	),
	mcp.WithString("query",
		mcp.Description("Search query (matches against label, description, tags)"),
	),
	mcp.WithNumber("limit",
		mcp.Description("Maximum number of results to return (default 20, max 100)"),
	),
),
```

- [ ] **Step 2: Update handler with envelope and limit**

```go
func getSamplesHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		packID := req.GetString("pack", "")
		query := req.GetString("query", "")
		limit := clampLimit(req.GetInt("limit", 20), 20, 100)

		var samples []content.Sample
		if packID != "" {
			found := false
			for _, p := range deps.Packs {
				if p.ID == packID {
					found = true
					break
				}
			}
			if !found {
				return mcp.NewToolResultError(fmt.Sprintf("pack %q not found", packID)), nil
			}
			samples = content.FilterSamplesByPack(deps.Packs, packID)
		} else {
			samples = content.FlattenSamples(deps.Packs)
		}
		if query != "" {
			samples = content.FilterSamples(samples, query)
		}
		total := len(samples)
		if limit < total {
			samples = samples[:limit]
		}

		out := make([]sampleResult, 0, len(samples))
		for _, s := range samples {
			out = append(out, sampleResult{
				ID:          s.ID,
				Label:       s.Label,
				Description: s.Description,
				URL:         s.URL,
				Tags:        s.Tags,
			})
		}
		return wrapResults(out, total, len(out), "samples", query), nil
	}
}
```

- [ ] **Step 3: Verify and commit**

Run: `go build ./...`

```bash
git add internal/mcpserver/tools_samples.go
git commit -m "feat(mcp): update get_samples with envelope, limit, enriched description"
```

---

### Task 9: New Tool — `get_news_detail`

**Files:**
- Create: `internal/mcpserver/tools_news_detail.go`
- Modify: `internal/mcpserver/server.go` (add register call)

- [ ] **Step 1: Create `tools_news_detail.go`**

```go
package mcpserver

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/SAP-samples/sap-devs-cli/internal/community"
)

const newsDetailTTL = 1 * time.Hour

type newsDetailResult struct {
	Title        string              `json:"title"`
	CommunityURL string             `json:"community_url"`
	Items        []newsDetailItem    `json:"items"`
	Chapters     []newsDetailChapter `json:"chapters"`
	RawContent   string              `json:"raw_content,omitempty"`
}

type newsDetailItem struct {
	Title string   `json:"title"`
	Links []string `json:"links"`
}

type newsDetailChapter struct {
	Time  string `json:"time"`
	Title string `json:"title"`
}

func newsDetailCachePath(cacheDir, key string) string {
	return filepath.Join(cacheDir, "news-detail", key+".json")
}

func loadNewsDetailCache(cacheDir, key string, ttl time.Duration) (newsDetailResult, bool) {
	var zero newsDetailResult
	path := newsDetailCachePath(cacheDir, key)
	info, err := os.Stat(path)
	if err != nil || time.Since(info.ModTime()) > ttl {
		return zero, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return zero, false
	}
	var result newsDetailResult
	if err := json.Unmarshal(data, &result); err != nil {
		return zero, false
	}
	return result, true
}

func saveNewsDetailCache(cacheDir, key string, result newsDetailResult) {
	path := newsDetailCachePath(cacheDir, key)
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	data, err := json.Marshal(result)
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0644)
}

func registerNewsDetailTools(s *server.MCPServer, deps Deps) {
	s.AddTool(
		mcp.NewTool("get_news_detail",
			mcp.WithDescription("Get the full content of a specific SAP Developer News episode, including topics covered, chapter timestamps, and links. Use after get_recent_news to dive deeper into a specific episode."),
			mcp.WithString("community_url",
				mcp.Required(),
				mcp.Description("The community_url from a get_recent_news result"),
			),
		),
		getNewsDetailHandler(deps),
	)
}

func getNewsDetailHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		url, err := req.RequireString("community_url")
		if err != nil {
			return mcp.NewToolResultError("community_url parameter is required"), nil
		}

		cacheKey := fmt.Sprintf("%x", sha256.Sum256([]byte(url)))
		if deps.CacheDir != "" {
			if cached, ok := loadNewsDetailCache(deps.CacheDir, cacheKey, newsDetailTTL); ok {
				b, _ := json.Marshal(cached)
				return mcp.NewToolResultText(string(b)), nil
			}
		}

		markdown, err := community.FetchPostContent(url)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch episode content: %v", err)), nil
		}

		result := parseNewsDetail(url, markdown)

		if deps.CacheDir != "" {
			saveNewsDetailCache(deps.CacheDir, cacheKey, result)
		}

		b, _ := json.Marshal(result)
		return mcp.NewToolResultText(string(b)), nil
	}
}

var (
	boldHeadingRe = regexp.MustCompile(`(?m)^\*\*(.+?)\*\*\s*$`)
	linkRe        = regexp.MustCompile(`\[([^\]]*)\]\((https?://[^\s)]+)\)`)
	chapterRe     = regexp.MustCompile(`(?m)^(\d{2}:\d{2})\s+(.+)$`)
)

func parseNewsDetail(communityURL, markdown string) newsDetailResult {
	result := newsDetailResult{
		CommunityURL: communityURL,
	}

	lines := strings.Split(markdown, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") || strings.HasPrefix(trimmed, "# ") {
			candidate := strings.TrimLeft(trimmed, "# ")
			if strings.Contains(strings.ToLower(candidate), "developer news") {
				result.Title = candidate
				break
			}
		}
	}

	itemsSection := extractSection(markdown, "ITEMS")
	if itemsSection != "" {
		parts := boldHeadingRe.Split(itemsSection, -1)
		headings := boldHeadingRe.FindAllStringSubmatch(itemsSection, -1)
		for i, heading := range headings {
			item := newsDetailItem{Title: strings.TrimSpace(heading[1])}
			if i+1 < len(parts) {
				for _, m := range linkRe.FindAllStringSubmatch(parts[i+1], -1) {
					item.Links = append(item.Links, m[2])
				}
			}
			result.Items = append(result.Items, item)
		}
	}

	chaptersSection := extractSection(markdown, "CHAPTER TITLES")
	if chaptersSection != "" {
		for _, m := range chapterRe.FindAllStringSubmatch(chaptersSection, -1) {
			result.Chapters = append(result.Chapters, newsDetailChapter{
				Time:  m[1],
				Title: strings.TrimSpace(m[2]),
			})
		}
	}

	if len(result.Items) == 0 {
		result.RawContent = markdown
	}

	return result
}

func extractSection(markdown, sectionName string) string {
	marker := strings.ToUpper(sectionName)
	idx := strings.Index(strings.ToUpper(markdown), marker)
	if idx == -1 {
		return ""
	}
	rest := markdown[idx+len(marker):]
	nextSection := -1
	for _, sep := range []string{"### ", "## ", "CHAPTER TITLES", "TRANSCRIPT", "ITEMS"} {
		if sep == marker {
			continue
		}
		pos := strings.Index(strings.ToUpper(rest), strings.ToUpper(sep))
		if pos != -1 && (nextSection == -1 || pos < nextSection) {
			nextSection = pos
		}
	}
	if nextSection != -1 {
		rest = rest[:nextSection]
	}
	return strings.TrimSpace(rest)
}
```

- [ ] **Step 2: Add `registerNewsDetailTools` call in `server.go`**

Add to `NewServer()`:

```go
registerNewsDetailTools(s, deps)
```

- [ ] **Step 3: Verify it compiles**

Run: `go build ./...`
Expected: clean build

- [ ] **Step 4: Commit**

```bash
git add internal/mcpserver/tools_news_detail.go internal/mcpserver/server.go
git commit -m "feat(mcp): add get_news_detail tool with structured episode parsing"
```

---

### Task 10: New Tool — `check_tools` and `check_project`

**Files:**
- Create: `internal/mcpserver/tools_doctor.go`
- Modify: `internal/mcpserver/server.go` (add register call)

- [ ] **Step 1: Create `tools_doctor.go`**

```go
package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/SAP-samples/sap-devs-cli/internal/content"
	"github.com/SAP-samples/sap-devs-cli/internal/project"
)

func registerDoctorTools(s *server.MCPServer, deps Deps) {
	s.AddTool(
		mcp.NewTool("check_tools",
			mcp.WithDescription("Check which SAP developer tools are installed and their versions. Returns status (ok/fail/missing) with install commands for missing tools. Use when a user encounters 'command not found' errors or needs environment setup help."),
			mcp.WithNumber("limit",
				mcp.Description("Maximum number of results to return (default 20, max 100)"),
			),
		),
		checkToolsHandler(deps),
	)

	s.AddTool(
		mcp.NewTool("check_project",
			mcp.WithDescription("Run health checks on the current SAP project. Detects project type (CAP, MTA, UI5), checks dependencies, version staleness, and best-practice compliance. Returns findings with severity and fix suggestions. Use proactively when helping with SAP project issues."),
			mcp.WithString("path",
				mcp.Description("Absolute path to project root directory. If omitted, uses the working directory the MCP server was launched from."),
			),
		),
		checkProjectHandler(deps),
	)
}

type toolCheckResult struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	Required string `json:"required"`
	Found    string `json:"found,omitempty"`
	Install  string `json:"install,omitempty"`
	Docs     string `json:"docs,omitempty"`
}

func execRunner(command string) (string, error) {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return "", fmt.Errorf("empty command")
	}
	cmd := exec.Command(parts[0], parts[1:]...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func installForCurrentOS(install map[string]string) string {
	goos := runtime.GOOS
	if cmd, ok := install[goos]; ok {
		return cmd
	}
	if goos == "darwin" {
		if cmd, ok := install["macos"]; ok {
			return cmd
		}
	}
	if cmd, ok := install["all"]; ok {
		return cmd
	}
	return ""
}

func checkToolsHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		limit := clampLimit(req.GetInt("limit", 20), 20, 100)

		var tools []content.ToolDef
		for _, p := range deps.Packs {
			tools = append(tools, p.Tools...)
		}
		results := content.CheckTools(tools, execRunner)
		total := len(results)
		if limit < total {
			results = results[:limit]
		}

		out := make([]toolCheckResult, 0, len(results))
		for _, r := range results {
			out = append(out, toolCheckResult{
				ID:       r.Tool.ID,
				Name:     r.Tool.Name,
				Status:   string(r.Status),
				Required: r.Tool.Required,
				Found:    r.Found,
				Install:  installForCurrentOS(r.Tool.Install),
				Docs:     r.Tool.Docs,
			})
		}
		return wrapResults(out, total, len(out), "tools", ""), nil
	}
}

type projectCheckResult struct {
	Detection projectDetection  `json:"detection"`
	Findings  ResultEnvelope    `json:"findings"`
}

type projectDetection struct {
	Type          string `json:"type,omitempty"`
	CAPVersion    string `json:"cap_version,omitempty"`
	Database      string `json:"database,omitempty"`
	Deployment    string `json:"deployment,omitempty"`
	Auth          string `json:"auth,omitempty"`
	BTPSubaccount string `json:"btp_subaccount,omitempty"`
	BTPRegion     string `json:"btp_region,omitempty"`
	CFOrg         string `json:"cf_org,omitempty"`
	CFSpace       string `json:"cf_space,omitempty"`
}

type findingResult struct {
	Category string `json:"category"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Fix      string `json:"fix,omitempty"`
}

func checkProjectHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		cwd := req.GetString("path", "")
		if cwd == "" {
			cwd = deps.Cwd
		}
		if cwd == "" {
			return mcp.NewToolResultError("no project path available"), nil
		}
		if !filepath.IsAbs(cwd) {
			return mcp.NewToolResultError("path must be an absolute path"), nil
		}

		pctx, err := project.Detect(cwd)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("project detection failed: %v", err)), nil
		}

		detection := projectDetection{
			Type:          pctx.Type,
			CAPVersion:    pctx.CAPVersion,
			Database:      pctx.Database,
			Deployment:    pctx.Deployment,
			Auth:          pctx.Auth,
			BTPSubaccount: pctx.BTPSubaccount,
			BTPRegion:     pctx.BTPRegion,
			CFOrg:         pctx.CFOrg,
			CFSpace:       pctx.CFSpace,
		}

		findings := project.Check(pctx, cwd, deps.Packs)
		out := make([]findingResult, 0, len(findings))
		for _, f := range findings {
			out = append(out, findingResult{
				Category: f.Category,
				Severity: f.Severity,
				Message:  f.Message,
				Fix:      f.Fix,
			})
		}

		result := projectCheckResult{
			Detection: detection,
			Findings: ResultEnvelope{
				Count:   len(out),
				Total:   len(out),
				Results: out,
			},
		}
		b, _ := json.Marshal(result)
		return mcp.NewToolResultText(string(b)), nil
	}
}
```

- [ ] **Step 2: Add `registerDoctorTools(s, deps)` call in `server.go`**

- [ ] **Step 3: Verify it compiles**

Run: `go build ./...`

- [ ] **Step 4: Commit**

```bash
git add internal/mcpserver/tools_doctor.go internal/mcpserver/server.go
git commit -m "feat(mcp): add check_tools and check_project tools"
```

---

### Task 11: New Tool — `search_events`

**Files:**
- Create: `internal/mcpserver/tools_events.go`
- Modify: `internal/content/events.go` (add `FilterEventsByQuery`)
- Modify: `internal/mcpserver/server.go` (add register call)

- [ ] **Step 1: Add `FilterEventsByQuery` to `internal/content/events.go`**

```go
// FilterEventsByQuery returns events matching query in title, location, or tags.
func FilterEventsByQuery(events []EventInstance, query string) []EventInstance {
	q := strings.ToLower(query)
	var out []EventInstance
	for _, e := range events {
		if strings.Contains(strings.ToLower(e.Title), q) ||
			strings.Contains(strings.ToLower(e.Location), q) {
			out = append(out, e)
			continue
		}
		for _, tag := range e.Tags {
			if strings.Contains(strings.ToLower(tag), q) {
				out = append(out, e)
				break
			}
		}
	}
	return out
}
```

Add `"strings"` to the import block in `events.go`.

- [ ] **Step 2: Create `tools_events.go`**

```go
package mcpserver

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/SAP-samples/sap-devs-cli/internal/content"
)

func registerEventTools(s *server.MCPServer, deps Deps) {
	s.AddTool(
		mcp.NewTool("search_events",
			mcp.WithDescription("Search upcoming SAP community events (CodeJams, Devtoberfest, TechEd, user groups). Returns event details with dates, locations, and registration URLs. Use when users ask about SAP events or learning opportunities near them."),
			mcp.WithString("query",
				mcp.Description("Search query — matches against title, location, and tags. Examples: 'CodeJam', 'ABAP', 'virtual'."),
			),
			mcp.WithString("type",
				mcp.Description("Event type ID to filter by (e.g. 'codejam', 'devtoberfest', 'teched')"),
			),
			mcp.WithString("scope",
				mcp.Description("Filter by scope: 'local', 'regional', 'virtual', 'global'"),
			),
			mcp.WithNumber("limit",
				mcp.Description("Maximum number of results to return (default 10, max 50)"),
			),
		),
		searchEventsHandler(deps),
	)
}

type eventResult struct {
	ID       string   `json:"id"`
	Type     string   `json:"type"`
	Title    string   `json:"title"`
	Date     string   `json:"date"`
	EndDate  string   `json:"end_date,omitempty"`
	Location string   `json:"location,omitempty"`
	Scope    string   `json:"scope"`
	URL      string   `json:"url"`
	Tags     []string `json:"tags,omitempty"`
}

func searchEventsHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query := req.GetString("query", "")
		typeID := req.GetString("type", "")
		scope := req.GetString("scope", "")
		limit := clampLimit(req.GetInt("limit", 10), 10, 50)

		events := content.FlattenEventInstances(deps.Packs)

		if typeID != "" {
			events = content.FilterEventsByType(events, typeID)
		}
		if query != "" {
			events = content.FilterEventsByQuery(events, query)
		}
		if scope != "" {
			var filtered []content.EventInstance
			for _, e := range events {
				if e.Scope == scope {
					filtered = append(filtered, e)
				}
			}
			events = filtered
		}

		total := len(events)
		if limit < total {
			events = events[:limit]
		}

		out := make([]eventResult, 0, len(events))
		for _, e := range events {
			out = append(out, eventResult{
				ID:       e.ID,
				Type:     e.Type,
				Title:    e.Title,
				Date:     e.DateStr,
				EndDate:  e.EndDateStr,
				Location: e.Location,
				Scope:    e.Scope,
				URL:      e.URL,
				Tags:     e.Tags,
			})
		}
		return wrapResults(out, total, len(out), "events", query), nil
	}
}
```

- [ ] **Step 3: Add `registerEventTools(s, deps)` call in `server.go`**

- [ ] **Step 4: Verify it compiles**

Run: `go build ./...`

- [ ] **Step 5: Commit**

```bash
git add internal/content/events.go internal/mcpserver/tools_events.go internal/mcpserver/server.go
git commit -m "feat(mcp): add search_events tool with query, type, scope filtering"
```

---

### Task 12: New Tool — `search_videos`

**Files:**
- Create: `internal/mcpserver/tools_videos.go`
- Modify: `internal/mcpserver/server.go` (add register call)

- [ ] **Step 1: Create `tools_videos.go`**

```go
package mcpserver

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/SAP-samples/sap-devs-cli/internal/content"
	"github.com/SAP-samples/sap-devs-cli/internal/videos"
)

func registerVideoTools(s *server.MCPServer, deps Deps) {
	s.AddTool(
		mcp.NewTool("search_videos",
			mcp.WithDescription("Search SAP developer videos from the SAP Developers YouTube channel. Covers tutorials, Tech Bytes, live streams, and conference talks. Use when users want video learning content."),
			mcp.WithString("query",
				mcp.Description("Search query — matches against title, description, and tags. Examples: 'CAP tutorial', 'Fiori elements', 'ABAP RAP'."),
			),
			mcp.WithString("source",
				mcp.Description("Source ID to filter by (e.g. 'sap-tech-bytes', 'developer-news')"),
			),
			mcp.WithNumber("limit",
				mcp.Description("Maximum number of results to return (default 10, max 50)"),
			),
		),
		searchVideosHandler(deps),
	)
}

type videoResult struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	Published   string `json:"published"`
	Duration    string `json:"duration,omitempty"`
	Description string `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

func searchVideosHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query := req.GetString("query", "")
		source := req.GetString("source", "")
		limit := clampLimit(req.GetInt("limit", 10), 10, 50)

		var allVids []content.Video
		for _, p := range deps.Packs {
			sources := p.YouTubeSources
			if source != "" {
				var filtered []content.YouTubeSource
				for _, s := range sources {
					if s.ID == source {
						filtered = append(filtered, s)
					}
				}
				sources = filtered
			}
			vids, _ := videos.ResolveAll(sources, deps.CacheDir)
			allVids = append(allVids, vids...)
		}

		if query != "" {
			allVids = videos.FilterVideos(allVids, query)
		}

		total := len(allVids)
		if limit < total {
			allVids = allVids[:limit]
		}

		out := make([]videoResult, 0, len(allVids))
		for _, v := range allVids {
			out = append(out, videoResult{
				ID:          v.ID,
				Title:       v.Title,
				URL:         v.URL,
				Published:   v.Published.Format("2006-01-02"),
				Duration:    v.Duration,
				Description: v.Description,
				Tags:        v.Tags,
			})
		}
		return wrapResults(out, total, len(out), "videos", query), nil
	}
}
```

- [ ] **Step 2: Add `registerVideoTools(s, deps)` call in `server.go`**

- [ ] **Step 3: Verify it compiles**

Run: `go build ./...`

- [ ] **Step 4: Commit**

```bash
git add internal/mcpserver/tools_videos.go internal/mcpserver/server.go
git commit -m "feat(mcp): add search_videos tool"
```

---

### Task 13: New Tool — `search_discovery`

**Files:**
- Create: `internal/mcpserver/tools_discovery.go`
- Modify: `internal/mcpserver/server.go` (add register call)

- [ ] **Step 1: Create `tools_discovery.go`**

```go
package mcpserver

import (
	"context"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/SAP-samples/sap-devs-cli/internal/discovery"
)

func registerDiscoveryTools(s *server.MCPServer, deps Deps) {
	s.AddTool(
		mcp.NewTool("search_discovery",
			mcp.WithDescription("Search SAP Discovery Center missions and BTP services. Missions are guided hands-on experiences; services are the BTP service catalog. Use when users need to explore SAP BTP capabilities or find guided learning missions."),
			mcp.WithString("query",
				mcp.Required(),
				mcp.Description("Search query for missions or services"),
			),
			mcp.WithString("type",
				mcp.Description("Either 'missions' or 'services'. Default: 'missions'."),
			),
			mcp.WithNumber("limit",
				mcp.Description("Maximum number of results to return (default 10, max 50)"),
			),
		),
		searchDiscoveryHandler(deps),
	)
}

type missionResult struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Effort      string `json:"effort"`
	Category    string `json:"category"`
	Description string `json:"description"`
}

type serviceResult struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Description string `json:"description"`
	Deprecated  bool   `json:"deprecated"`
}

func searchDiscoveryHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query, err := req.RequireString("query")
		if err != nil {
			return mcp.NewToolResultError("query parameter is required"), nil
		}
		searchType := req.GetString("type", "missions")
		limit := clampLimit(req.GetInt("limit", 10), 10, 50)

		client := discovery.NewClient()

		if searchType == "services" {
			services, err := client.FetchServices()
			if err != nil {
				return wrapResults([]serviceResult{}, 0, 0, "services", query), nil
			}
			var filtered []serviceResult
			for _, s := range services {
				if matchesQuery(query, s.Name, s.ShortDescription, s.Category) {
					filtered = append(filtered, serviceResult{
						ID:          s.ID,
						Name:        s.Name,
						Category:    s.Category,
						Description: s.ShortDescription,
						Deprecated:  s.IsDeprecatedService,
					})
				}
			}
			total := len(filtered)
			if limit < total {
				filtered = filtered[:limit]
			}
			return wrapResults(filtered, total, len(filtered), "services", query), nil
		}

		filters := discovery.SearchFilters{Top: limit}
		missions, err := client.SearchMissions(query, filters)
		if err != nil {
			return wrapResults([]missionResult{}, 0, 0, "missions", query), nil
		}
		total := len(missions)
		if limit < total {
			missions = missions[:limit]
		}
		out := make([]missionResult, 0, len(missions))
		for _, m := range missions {
			out = append(out, missionResult{
				ID:          m.ID,
				Name:        m.Name,
				Effort:      m.Effort,
				Category:    m.Category,
				Description: m.UCLongDescription,
			})
		}
		return wrapResults(out, total, len(out), "missions", query), nil
	}
}

func matchesQuery(query string, fields ...string) bool {
	q := strings.ToLower(query)
	for _, f := range fields {
		if strings.Contains(strings.ToLower(f), q) {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Add `registerDiscoveryTools(s, deps)` call in `server.go`**

- [ ] **Step 3: Verify it compiles**

Run: `go build ./...`

- [ ] **Step 4: Commit**

```bash
git add internal/mcpserver/tools_discovery.go internal/mcpserver/server.go
git commit -m "feat(mcp): add search_discovery tool for missions and services"
```

---

### Task 14: Verify Full Build & Test MCP Handshake

- [ ] **Step 1: Verify the complete build**

Run: `go build ./...`
Expected: clean build, no errors

- [ ] **Step 2: Run go vet**

Run: `go vet ./...`
Expected: no issues

- [ ] **Step 3: Test MCP initialization handshake**

Run:
```bash
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}' | SAP_DEVS_DEV=1 ./sap-devs.exe mcp serve 2>/dev/null
```

Expected: JSON response with `serverInfo.name: "sap-devs"` and the new instructions string.

- [ ] **Step 4: Test tools/list returns all 15 tools**

Run:
```bash
printf '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}\n{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}\n' | SAP_DEVS_DEV=1 ./sap-devs.exe mcp serve 2>/dev/null
```

Expected: The second JSON response should contain 15 tools: `list_packs`, `get_context`, `get_tip`, `search_resources`, `get_known_errors`, `search_tutorials`, `search_learning_journeys`, `get_recent_news`, `get_samples`, `get_news_detail`, `check_tools`, `check_project`, `search_events`, `search_videos`, `search_discovery`.

- [ ] **Step 5: Commit if any fixes were needed**

---

### Task 15: Update Documentation

**Files:**
- Modify: `CLAUDE.md`
- Modify: `content/packs/base/context.md`

- [ ] **Step 1: Update the MCP server section in `CLAUDE.md`**

In the `### CLI Commands` table and `### MCP Server` documentation sections, update to reflect:
- 15 total tools (was 9)
- List the new tools: `get_news_detail`, `check_tools`, `check_project`, `search_events`, `search_videos`, `search_discovery`
- Note the envelope response format
- Note the `verbosity` parameter on `get_context`

- [ ] **Step 2: Update CLI reference table in `content/packs/base/context.md`**

Add the 6 new MCP tools to the reference table if one exists for MCP tools.

- [ ] **Step 3: Verify build still passes**

Run: `go build ./...`

- [ ] **Step 4: Commit**

```bash
git add CLAUDE.md content/packs/base/context.md
git commit -m "docs: update CLAUDE.md and context.md for MCP server improvements"
```
