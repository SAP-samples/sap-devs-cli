# MCP Server Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expose `sap-devs` as a live MCP server (`sap-devs mcp serve`) so AI agents can query SAP developer knowledge on demand via the MCP protocol.

**Architecture:** Single `internal/mcpserver/` package with thin handler adapters calling existing `internal/content/`, `internal/news/`, `internal/youtube/`, `internal/tutorials/`, `internal/learning/` functions. Uses `mark3labs/mcp-go` SDK for protocol handling. Stdio transport, stateless per-invocation, 9 tools.

**Tech Stack:** Go, `mark3labs/mcp-go` SDK, cobra CLI, existing content loader

**Spec:** `docs/superpowers/specs/2026-04-19-mcp-server-design.md`

---

## File Structure

| Action | Path | Responsibility |
|--------|------|----------------|
| Create | `internal/mcpserver/server.go` | `Deps` struct, `NewServer()` constructor, tool registration |
| Create | `internal/mcpserver/tools_content.go` | `list_packs`, `get_context`, `get_tip` handlers |
| Create | `internal/mcpserver/tools_resources.go` | `search_resources` handler |
| Create | `internal/mcpserver/tools_errors.go` | `get_known_errors` handler |
| Create | `internal/mcpserver/tools_news.go` | `get_recent_news` handler with lazy fetch + cache |
| Create | `internal/mcpserver/tools_learn.go` | `search_tutorials`, `search_learning_journeys` handlers |
| Create | `internal/mcpserver/tools_samples.go` | `get_samples` handler |
| Create | `internal/mcpserver/server_test.go` | Integration test: NewServer registers all 9 tools |
| Create | `internal/mcpserver/tools_content_test.go` | Unit tests for list_packs, get_context, get_tip |
| Create | `internal/mcpserver/tools_resources_test.go` | Unit tests for search_resources |
| Create | `internal/mcpserver/tools_errors_test.go` | Unit tests for get_known_errors |
| Create | `internal/mcpserver/tools_news_test.go` | Unit tests for get_recent_news |
| Create | `internal/mcpserver/tools_learn_test.go` | Unit tests for search_tutorials, search_learning_journeys |
| Create | `internal/mcpserver/tools_samples_test.go` | Unit tests for get_samples |
| Create | `cmd/mcp_serve.go` | `mcp serve` cobra subcommand |
| Create | `content/packs/base/mcp.yaml` | Self-install server definition |
| Modify | `go.mod` / `go.sum` | Add `github.com/mark3labs/mcp-go` dependency |
| Modify | `CLAUDE.md` | Document `mcp serve` command |
| Modify | `content/packs/base/context.md` | Add `mcp serve` to CLI reference table |

---

## Task 1: Add mcp-go dependency

**Files:**
- Modify: `go.mod`

- [ ] **Step 1: Add the mcp-go module**

Run:
```bash
go get github.com/mark3labs/mcp-go@latest
```

- [ ] **Step 2: Tidy modules**

Run:
```bash
go mod tidy
```

- [ ] **Step 3: Verify build**

Run:
```bash
go build ./...
```
Expected: clean build, no errors

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: add mark3labs/mcp-go dependency for MCP server"
```

---

## Task 2: Server scaffold — Deps struct and NewServer constructor

**Files:**
- Create: `internal/mcpserver/server.go`
- Create: `internal/mcpserver/server_test.go`

- [ ] **Step 1: Write the integration test**

Create `internal/mcpserver/server_test.go`:

```go
package mcpserver_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/SAP-samples/sap-devs-cli/internal/content"
	"github.com/SAP-samples/sap-devs-cli/internal/mcpserver"
)

func TestNewServer_RegistersAllTools(t *testing.T) {
	deps := mcpserver.Deps{
		Packs:   []*content.Pack{{ID: "test", Name: "Test Pack"}},
		Version: "1.0.0",
	}
	s := mcpserver.NewServer(deps)
	assert.NotNil(t, s)
}
```

Note: `mcp-go`'s `MCPServer` does not expose a public method to list registered tools, so we verify construction doesn't panic. Each tool handler is individually tested in subsequent tasks.

- [ ] **Step 2: Run test to verify it fails**

Run:
```bash
go build ./internal/mcpserver/...
```
Expected: FAIL — package does not exist

- [ ] **Step 3: Write server.go**

Create `internal/mcpserver/server.go`:

```go
package mcpserver

import (
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/SAP-samples/sap-devs-cli/internal/content"
	"github.com/SAP-samples/sap-devs-cli/internal/learning"
	"github.com/SAP-samples/sap-devs-cli/internal/tutorials"
)

type Deps struct {
	Packs         []*content.Pack
	Profile       *content.Profile
	TutorialIndex []tutorials.TutorialMeta
	LearningIndex []learning.LearningJourney
	Version       string
	// News is fetched lazily by the get_recent_news handler — not stored here.
	// See tools_news.go for the NewsFetcher type.
}

func NewServer(deps Deps) *server.MCPServer {
	s := server.NewMCPServer(
		"sap-devs",
		deps.Version,
		server.WithToolCapabilities(false),
		server.WithInstructions("SAP developer knowledge server. Use these tools to get SAP-specific context, tips, resources, error patterns, news, tutorials, and learning journeys on demand."),
	)

	registerContentTools(s, deps)
	registerResourceTools(s, deps)
	registerErrorTools(s, deps)
	registerNewsTools(s)
	registerLearnTools(s, deps)
	registerSampleTools(s, deps)

	return s
}
```

The `register*` functions will be added in subsequent tasks. For now, create stub files so the package compiles.

Create stub `internal/mcpserver/tools_content.go`:
```go
package mcpserver

import "github.com/mark3labs/mcp-go/server"

func registerContentTools(s *server.MCPServer, deps Deps) {}
```

Create stub `internal/mcpserver/tools_resources.go`:
```go
package mcpserver

import "github.com/mark3labs/mcp-go/server"

func registerResourceTools(s *server.MCPServer, deps Deps) {}
```

Create stub `internal/mcpserver/tools_errors.go`:
```go
package mcpserver

import "github.com/mark3labs/mcp-go/server"

func registerErrorTools(s *server.MCPServer, deps Deps) {}
```

Create stub `internal/mcpserver/tools_news.go`:
```go
package mcpserver

import "github.com/mark3labs/mcp-go/server"

func registerNewsTools(s *server.MCPServer) {}
```

Create stub `internal/mcpserver/tools_learn.go`:
```go
package mcpserver

import "github.com/mark3labs/mcp-go/server"

func registerLearnTools(s *server.MCPServer, deps Deps) {}
```

Create stub `internal/mcpserver/tools_samples.go`:
```go
package mcpserver

import "github.com/mark3labs/mcp-go/server"

func registerSampleTools(s *server.MCPServer, deps Deps) {}
```

- [ ] **Step 4: Verify build + vet**

Run:
```bash
go build ./internal/mcpserver/... && go vet ./internal/mcpserver/...
```
Expected: clean

- [ ] **Step 5: Commit**

```bash
git add internal/mcpserver/
git commit -m "feat(mcp-server): scaffold server package with Deps struct and NewServer constructor"
```

---

## Task 3: list_packs tool

**Files:**
- Modify: `internal/mcpserver/tools_content.go`
- Create: `internal/mcpserver/tools_content_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/mcpserver/tools_content_test.go`:

```go
package mcpserver

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/SAP-samples/sap-devs-cli/internal/content"
)

func TestListPacks(t *testing.T) {
	deps := Deps{
		Packs: []*content.Pack{
			{ID: "cap", Name: "CAP", Description: "Cloud Application Programming", Tags: []string{"cap", "nodejs"}},
			{ID: "abap", Name: "ABAP", Description: "ABAP Cloud", Tags: []string{"abap"}},
		},
	}
	handler := listPacksHandler(deps)
	result, err := handler(context.Background(), mcp.CallToolRequest{})
	require.NoError(t, err)
	require.False(t, result.IsError)

	var packs []map[string]any
	err = json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &packs)
	require.NoError(t, err)
	assert.Len(t, packs, 2)
	assert.Equal(t, "cap", packs[0]["id"])
	assert.Equal(t, "CAP", packs[0]["name"])
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:
```bash
go build ./internal/mcpserver/...
```
Expected: FAIL — `listPacksHandler` undefined

- [ ] **Step 3: Implement list_packs handler**

Replace `internal/mcpserver/tools_content.go`:

```go
package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/SAP-samples/sap-devs-cli/internal/content"
)

func registerContentTools(s *server.MCPServer, deps Deps) {
	s.AddTool(
		mcp.NewTool("list_packs",
			mcp.WithDescription("List all available SAP content packs with their ID, name, description, and tags"),
		),
		listPacksHandler(deps),
	)

	s.AddTool(
		mcp.NewTool("get_context",
			mcp.WithDescription("Get the full SAP developer context markdown for all packs or a specific pack"),
			mcp.WithString("pack",
				mcp.Description("Pack ID to get context for. If omitted, returns context for all packs."),
			),
		),
		getContextHandler(deps),
	)

	s.AddTool(
		mcp.NewTool("get_tip",
			mcp.WithDescription("Get a random SAP developer tip, optionally filtered by topic tag"),
			mcp.WithString("topic",
				mcp.Description("Topic tag to filter tips by (e.g. 'cap', 'abap', 'btp')"),
			),
		),
		getTipHandler(deps),
	)
}

type packSummary struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
}

func listPacksHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		out := make([]packSummary, 0, len(deps.Packs))
		for _, p := range deps.Packs {
			out = append(out, packSummary{
				ID:          p.ID,
				Name:        p.Name,
				Description: p.Description,
				Tags:        p.Tags,
			})
		}
		b, _ := json.Marshal(out)
		return mcp.NewToolResultText(string(b)), nil
	}
}

func getContextHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		packID := req.GetString("pack", "")
		if packID != "" {
			for _, p := range deps.Packs {
				if p.ID == packID {
					text := p.Context.AtLevel("full")
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
			text := p.Context.AtLevel("full")
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
		return mcp.NewToolResultText(fmt.Sprintf("## %s\n\n%s", tip.Title, tip.Content)), nil
	}
}
```

- [ ] **Step 4: Verify build + run tests locally**

Run:
```bash
go build ./internal/mcpserver/... && go vet ./internal/mcpserver/...
```
Expected: clean

- [ ] **Step 5: Commit**

```bash
git add internal/mcpserver/tools_content.go internal/mcpserver/tools_content_test.go
git commit -m "feat(mcp-server): add list_packs, get_context, get_tip tool handlers"
```

---

## Task 4: get_context and get_tip tests

**Files:**
- Modify: `internal/mcpserver/tools_content_test.go`

- [ ] **Step 1: Add get_context and get_tip tests**

Append to `internal/mcpserver/tools_content_test.go`:

```go
func TestGetContext_SpecificPack(t *testing.T) {
	deps := Deps{
		Packs: []*content.Pack{
			{ID: "cap", Name: "CAP", Context: content.VerbositySections{Core: "CAP core.", Detail: "CAP detail.", Extended: "CAP extended."}},
		},
	}
	handler := getContextHandler(deps)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"pack": "cap"}
	result, err := handler(context.Background(), req)
	require.NoError(t, err)
	assert.Contains(t, result.Content[0].(mcp.TextContent).Text, "CAP core.")
	assert.Contains(t, result.Content[0].(mcp.TextContent).Text, "CAP extended.")
}

func TestGetContext_UnknownPack(t *testing.T) {
	deps := Deps{Packs: []*content.Pack{}}
	handler := getContextHandler(deps)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"pack": "nonexistent"}
	result, err := handler(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestGetContext_AllPacks(t *testing.T) {
	deps := Deps{
		Packs: []*content.Pack{
			{ID: "cap", Name: "CAP", Context: content.VerbositySections{Core: "CAP stuff."}},
			{ID: "abap", Name: "ABAP", Context: content.VerbositySections{Core: "ABAP stuff."}},
		},
	}
	handler := getContextHandler(deps)
	result, err := handler(context.Background(), mcp.CallToolRequest{})
	require.NoError(t, err)
	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "CAP stuff.")
	assert.Contains(t, text, "ABAP stuff.")
}

func TestGetTip(t *testing.T) {
	deps := Deps{
		Packs: []*content.Pack{
			{Tips: []content.Tip{{Title: "Use cds watch", Content: "Run `cds watch` for live reload.", Tags: []string{"cap"}}}},
		},
	}
	handler := getTipHandler(deps)
	result, err := handler(context.Background(), mcp.CallToolRequest{})
	require.NoError(t, err)
	assert.Contains(t, result.Content[0].(mcp.TextContent).Text, "cds watch")
}
```

- [ ] **Step 2: Verify build**

Run:
```bash
go build ./internal/mcpserver/... && go vet ./internal/mcpserver/...
```
Expected: clean

- [ ] **Step 3: Commit**

```bash
git add internal/mcpserver/tools_content_test.go
git commit -m "test(mcp-server): add get_context and get_tip unit tests"
```

---

## Task 5: search_resources tool

**Files:**
- Modify: `internal/mcpserver/tools_resources.go`
- Create: `internal/mcpserver/tools_resources_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/mcpserver/tools_resources_test.go`:

```go
package mcpserver

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/SAP-samples/sap-devs-cli/internal/content"
)

func TestSearchResources(t *testing.T) {
	deps := Deps{
		Packs: []*content.Pack{
			{ID: "cap", Resources: []content.Resource{
				{ID: "cap/help", Title: "CAP Help Portal", URL: "https://help.sap.com/cap", Type: "docs", Tags: []string{"cap"}},
				{ID: "cap/samples", Title: "CAP Samples", URL: "https://github.com/sap-samples/cap", Type: "samples", Tags: []string{"cap"}},
			}},
			{ID: "abap", Resources: []content.Resource{
				{ID: "abap/rap", Title: "RAP Guide", URL: "https://help.sap.com/rap", Type: "docs", Tags: []string{"abap"}},
			}},
		},
	}
	handler := searchResourcesHandler(deps)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"query": "CAP"}
	result, err := handler(context.Background(), req)
	require.NoError(t, err)

	var resources []map[string]any
	err = json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &resources)
	require.NoError(t, err)
	assert.Len(t, resources, 2)
}

func TestSearchResources_RequiresQuery(t *testing.T) {
	deps := Deps{Packs: []*content.Pack{}}
	handler := searchResourcesHandler(deps)
	result, err := handler(context.Background(), mcp.CallToolRequest{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
}
```

- [ ] **Step 2: Implement search_resources**

Replace `internal/mcpserver/tools_resources.go`:

```go
package mcpserver

import (
	"context"
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/SAP-samples/sap-devs-cli/internal/content"
)

func registerResourceTools(s *server.MCPServer, deps Deps) {
	s.AddTool(
		mcp.NewTool("search_resources",
			mcp.WithDescription("Search curated SAP resources by keyword. Returns matching resources with URLs."),
			mcp.WithString("query",
				mcp.Required(),
				mcp.Description("Search query (matches against title, type, tags)"),
			),
			mcp.WithString("pack",
				mcp.Description("Filter to resources from a specific pack ID"),
			),
		),
		searchResourcesHandler(deps),
	)
}

type resourceResult struct {
	ID    string   `json:"id"`
	Title string   `json:"title"`
	URL   string   `json:"url"`
	Type  string   `json:"type"`
	Tags  []string `json:"tags"`
}

func searchResourcesHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query, err := req.RequireString("query")
		if err != nil {
			return mcp.NewToolResultError("query parameter is required"), nil
		}
		packID := req.GetString("pack", "")

		var resources []content.Resource
		if packID != "" {
			for _, p := range deps.Packs {
				if p.ID == packID {
					resources = p.Resources
					break
				}
			}
		} else {
			resources = content.FlattenResources(deps.Packs)
		}
		resources = content.FilterResources(resources, query)

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
		b, _ := json.Marshal(out)
		return mcp.NewToolResultText(string(b)), nil
	}
}
```

Note: `content.FilterResourcesByPack` does not exist — the inline loop above filters by pack directly, matching the pattern used in `FilterSamplesByPack` and `FilterKnownErrorsByPack`.

- [ ] **Step 3: Verify build**

Run:
```bash
go build ./internal/mcpserver/... && go vet ./internal/mcpserver/...
```
Expected: clean

- [ ] **Step 4: Commit**

```bash
git add internal/mcpserver/tools_resources.go internal/mcpserver/tools_resources_test.go
git commit -m "feat(mcp-server): add search_resources tool handler"
```

---

## Task 6: get_known_errors tool

**Files:**
- Modify: `internal/mcpserver/tools_errors.go`
- Create: `internal/mcpserver/tools_errors_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/mcpserver/tools_errors_test.go`:

```go
package mcpserver

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/SAP-samples/sap-devs-cli/internal/content"
)

func TestGetKnownErrors(t *testing.T) {
	deps := Deps{
		Packs: []*content.Pack{
			{KnownErrors: []content.KnownError{
				{ID: "cap/cds-build", Pattern: "cds build failed", Cause: "Missing dependency", Fix: "Run npm install", Tags: []string{"cap"}},
				{ID: "abap/access", Pattern: "Access not permitted", Cause: "Non-released API", Fix: "Use released API", Tags: []string{"abap"}},
			}},
		},
	}
	handler := getKnownErrorsHandler(deps)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"query": "access"}
	result, err := handler(context.Background(), req)
	require.NoError(t, err)

	var errors []map[string]any
	err = json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &errors)
	require.NoError(t, err)
	assert.Len(t, errors, 1)
	assert.Equal(t, "abap/access", errors[0]["id"])
}
```

- [ ] **Step 2: Implement get_known_errors**

Replace `internal/mcpserver/tools_errors.go`:

```go
package mcpserver

import (
	"context"
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/SAP-samples/sap-devs-cli/internal/content"
)

func registerErrorTools(s *server.MCPServer, deps Deps) {
	s.AddTool(
		mcp.NewTool("get_known_errors",
			mcp.WithDescription("Look up known SAP error patterns by keyword. Returns cause and fix for matching errors."),
			mcp.WithString("query",
				mcp.Required(),
				mcp.Description("Search query (matches against error pattern, cause, fix, tags)"),
			),
		),
		getKnownErrorsHandler(deps),
	)
}

type knownErrorResult struct {
	ID      string   `json:"id"`
	Pattern string   `json:"pattern"`
	Cause   string   `json:"cause"`
	Fix     string   `json:"fix"`
	Docs    string   `json:"docs,omitempty"`
	Tags    []string `json:"tags"`
}

func getKnownErrorsHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query, err := req.RequireString("query")
		if err != nil {
			return mcp.NewToolResultError("query parameter is required"), nil
		}

		all := content.FlattenKnownErrors(deps.Packs)
		matches := content.FilterKnownErrors(all, query)

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
		b, _ := json.Marshal(out)
		return mcp.NewToolResultText(string(b)), nil
	}
}
```

- [ ] **Step 3: Verify build**

Run:
```bash
go build ./internal/mcpserver/... && go vet ./internal/mcpserver/...
```
Expected: clean

- [ ] **Step 4: Commit**

```bash
git add internal/mcpserver/tools_errors.go internal/mcpserver/tools_errors_test.go
git commit -m "feat(mcp-server): add get_known_errors tool handler"
```

---

## Task 7: get_recent_news tool with lazy fetch

**Files:**
- Modify: `internal/mcpserver/tools_news.go`
- Create: `internal/mcpserver/tools_news_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/mcpserver/tools_news_test.go`:

```go
package mcpserver

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/SAP-samples/sap-devs-cli/internal/news"
	"github.com/SAP-samples/sap-devs-cli/internal/youtube"
)

func TestGetRecentNews_WithItems(t *testing.T) {
	items := []news.NewsItem{
		{Episode: youtube.Episode{Title: "Episode 1", URL: "https://yt/1", Published: time.Now()}},
		{Episode: youtube.Episode{Title: "Episode 2", URL: "https://yt/2", Published: time.Now()}},
	}
	fetcher := &newsFetcher{}
	fetcher.cached = items
	handler := getRecentNewsHandler(fetcher)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"count": float64(1)}
	result, err := handler(context.Background(), req)
	require.NoError(t, err)

	var out []map[string]any
	err = json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &out)
	require.NoError(t, err)
	assert.Len(t, out, 1)
	assert.Equal(t, "Episode 1", out[0]["title"])
}
```

- [ ] **Step 2: Implement get_recent_news with lazy fetcher**

Replace `internal/mcpserver/tools_news.go`:

```go
package mcpserver

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/SAP-samples/sap-devs-cli/internal/community"
	"github.com/SAP-samples/sap-devs-cli/internal/news"
	"github.com/SAP-samples/sap-devs-cli/internal/youtube"
)

const (
	newsPlaylistRSS  = "https://www.youtube.com/feeds/videos.xml?playlist_id=PL6RpkC85SLQAVBSQXN9522_1jNvPavBgg"
	newsCommunityRSS = "https://community.sap.com/t5/developer-news/bg-p/developer-news/rss"
	newsFetchTimeout = 5 * time.Second
)

type newsFetcher struct {
	once   sync.Once
	cached []news.NewsItem
}

func (f *newsFetcher) get() []news.NewsItem {
	f.once.Do(func() {
		done := make(chan struct{})
		go func() {
			episodes, err := youtube.FetchPlaylist(newsPlaylistRSS)
			if err != nil {
				close(done)
				return
			}
			posts, _ := community.FetchBlogPosts(newsCommunityRSS)
			f.cached = news.Correlate(episodes, posts)
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(newsFetchTimeout):
		}
	})
	return f.cached
}

func registerNewsTools(s *server.MCPServer) {
	fetcher := &newsFetcher{}

	s.AddTool(
		mcp.NewTool("get_recent_news",
			mcp.WithDescription("Get the latest SAP Developer News episodes from YouTube and SAP Community"),
			mcp.WithNumber("count",
				mcp.Description("Number of episodes to return (default 5)"),
			),
		),
		getRecentNewsHandler(fetcher),
	)
}

type newsResult struct {
	Title        string `json:"title"`
	URL          string `json:"url"`
	Published    string `json:"published"`
	CommunityURL string `json:"community_url"`
}

func getRecentNewsHandler(fetcher *newsFetcher) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		count := req.GetInt("count", 5)
		if count <= 0 {
			count = 5
		}

		items := fetcher.get()
		if len(items) == 0 {
			return mcp.NewToolResultText("[]"), nil
		}
		if count > len(items) {
			count = len(items)
		}

		out := make([]newsResult, 0, count)
		for _, item := range items[:count] {
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
		b, _ := json.Marshal(out)
		return mcp.NewToolResultText(string(b)), nil
	}
}
```

- [ ] **Step 3: Verify build**

Run:
```bash
go build ./internal/mcpserver/... && go vet ./internal/mcpserver/...
```
Expected: clean

- [ ] **Step 4: Commit**

```bash
git add internal/mcpserver/tools_news.go internal/mcpserver/tools_news_test.go
git commit -m "feat(mcp-server): add get_recent_news tool with lazy fetch and caching"
```

---

## Task 8: search_tutorials and search_learning_journeys tools

**Files:**
- Modify: `internal/mcpserver/tools_learn.go`
- Create: `internal/mcpserver/tools_learn_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/mcpserver/tools_learn_test.go`:

```go
package mcpserver

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/SAP-samples/sap-devs-cli/internal/learning"
	"github.com/SAP-samples/sap-devs-cli/internal/tutorials"
)

func TestSearchTutorials(t *testing.T) {
	deps := Deps{
		TutorialIndex: []tutorials.TutorialMeta{
			{Slug: "cap-getting-started", Title: "Getting Started with CAP", Description: "Learn CAP basics", URL: "https://developers.sap.com/tutorials/cap-getting-started.html", Tags: []string{"cap"}},
			{Slug: "abap-adt", Title: "ABAP Development Tools", Description: "ADT setup", URL: "https://developers.sap.com/tutorials/abap-adt.html", Tags: []string{"abap"}},
		},
	}
	handler := searchTutorialsHandler(deps)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"query": "CAP"}
	result, err := handler(context.Background(), req)
	require.NoError(t, err)

	var tuts []map[string]any
	err = json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &tuts)
	require.NoError(t, err)
	assert.Len(t, tuts, 1)
	assert.Equal(t, "cap-getting-started", tuts[0]["slug"])
}

func TestSearchLearningJourneys(t *testing.T) {
	deps := Deps{
		LearningIndex: []learning.LearningJourney{
			{Slug: "btp-architect", Title: "Becoming a BTP Architect", Level: "INTERMEDIATE", DurationHours: "6.5", URL: "https://learning.sap.com/learning-journeys/btp-architect"},
		},
	}
	handler := searchLearningJourneysHandler(deps)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"query": "architect"}
	result, err := handler(context.Background(), req)
	require.NoError(t, err)

	var journeys []map[string]any
	err = json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &journeys)
	require.NoError(t, err)
	assert.Len(t, journeys, 1)
}
```

- [ ] **Step 2: Implement handlers**

Replace `internal/mcpserver/tools_learn.go`:

```go
package mcpserver

import (
	"context"
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/SAP-samples/sap-devs-cli/internal/learning"
	"github.com/SAP-samples/sap-devs-cli/internal/tutorials"
)

func registerLearnTools(s *server.MCPServer, deps Deps) {
	s.AddTool(
		mcp.NewTool("search_tutorials",
			mcp.WithDescription("Search SAP tutorials by keyword. Returns matching tutorials with URLs."),
			mcp.WithString("query",
				mcp.Required(),
				mcp.Description("Search query (matches against title, description, tags)"),
			),
		),
		searchTutorialsHandler(deps),
	)

	s.AddTool(
		mcp.NewTool("search_learning_journeys",
			mcp.WithDescription("Search SAP Learning Journeys by keyword. Returns matching journeys with level and duration."),
			mcp.WithString("query",
				mcp.Required(),
				mcp.Description("Search query (matches against title, description, level)"),
			),
		),
		searchLearningJourneysHandler(deps),
	)
}

type tutorialResult struct {
	Slug        string   `json:"slug"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	URL         string   `json:"url"`
	Tags        []string `json:"tags"`
}

func searchTutorialsHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query, err := req.RequireString("query")
		if err != nil {
			return mcp.NewToolResultError("query parameter is required"), nil
		}
		if len(deps.TutorialIndex) == 0 {
			return mcp.NewToolResultText("[]"), nil
		}
		matches := tutorials.Search(deps.TutorialIndex, query)
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
		b, _ := json.Marshal(out)
		return mcp.NewToolResultText(string(b)), nil
	}
}

type learningResult struct {
	Slug     string `json:"slug"`
	Title    string `json:"title"`
	Level    string `json:"level"`
	Duration string `json:"duration"`
	URL      string `json:"url"`
}

func searchLearningJourneysHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query, err := req.RequireString("query")
		if err != nil {
			return mcp.NewToolResultError("query parameter is required"), nil
		}
		if len(deps.LearningIndex) == 0 {
			return mcp.NewToolResultText("[]"), nil
		}
		matches := learning.Search(deps.LearningIndex, query)
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
		b, _ := json.Marshal(out)
		return mcp.NewToolResultText(string(b)), nil
	}
}
```

- [ ] **Step 3: Verify build**

Run:
```bash
go build ./internal/mcpserver/... && go vet ./internal/mcpserver/...
```
Expected: clean

- [ ] **Step 4: Commit**

```bash
git add internal/mcpserver/tools_learn.go internal/mcpserver/tools_learn_test.go
git commit -m "feat(mcp-server): add search_tutorials and search_learning_journeys tool handlers"
```

---

## Task 9: get_samples tool

**Files:**
- Modify: `internal/mcpserver/tools_samples.go`
- Create: `internal/mcpserver/tools_samples_test.go`

- [ ] **Step 1: Write failing test**

Create `internal/mcpserver/tools_samples_test.go`:

```go
package mcpserver

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/SAP-samples/sap-devs-cli/internal/content"
)

func TestGetSamples(t *testing.T) {
	deps := Deps{
		Packs: []*content.Pack{
			{ID: "cap", Samples: []content.Sample{
				{ID: "cap/bookshop", Label: "CAP Bookshop", URL: "https://github.com/sap-samples/bookshop", Description: "Reference app", Tags: []string{"cap"}},
			}},
		},
	}
	handler := getSamplesHandler(deps)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"query": "bookshop"}
	result, err := handler(context.Background(), req)
	require.NoError(t, err)

	var samples []map[string]any
	err = json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &samples)
	require.NoError(t, err)
	assert.Len(t, samples, 1)
	assert.Equal(t, "CAP Bookshop", samples[0]["label"])
}
```

- [ ] **Step 2: Implement get_samples**

Replace `internal/mcpserver/tools_samples.go`:

```go
package mcpserver

import (
	"context"
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/SAP-samples/sap-devs-cli/internal/content"
)

func registerSampleTools(s *server.MCPServer, deps Deps) {
	s.AddTool(
		mcp.NewTool("get_samples",
			mcp.WithDescription("Get canonical SAP code samples, optionally filtered by pack or keyword"),
			mcp.WithString("pack",
				mcp.Description("Filter to samples from a specific pack ID"),
			),
			mcp.WithString("query",
				mcp.Description("Search query (matches against label, description, tags)"),
			),
		),
		getSamplesHandler(deps),
	)
}

type sampleResult struct {
	ID          string   `json:"id"`
	Label       string   `json:"label"`
	Description string   `json:"description"`
	URL         string   `json:"url"`
	Tags        []string `json:"tags"`
}

func getSamplesHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		packID := req.GetString("pack", "")
		query := req.GetString("query", "")

		var samples []content.Sample
		if packID != "" {
			samples = content.FilterSamplesByPack(deps.Packs, packID)
		} else {
			samples = content.FlattenSamples(deps.Packs)
		}
		if query != "" {
			samples = content.FilterSamples(samples, query)
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
		b, _ := json.Marshal(out)
		return mcp.NewToolResultText(string(b)), nil
	}
}
```

- [ ] **Step 3: Verify build**

Run:
```bash
go build ./internal/mcpserver/... && go vet ./internal/mcpserver/...
```
Expected: clean

- [ ] **Step 4: Commit**

```bash
git add internal/mcpserver/tools_samples.go internal/mcpserver/tools_samples_test.go
git commit -m "feat(mcp-server): add get_samples tool handler"
```

---

## Task 10: `mcp serve` cobra command

**Files:**
- Create: `cmd/mcp_serve.go`

- [ ] **Step 1: Implement the command**

Create `cmd/mcp_serve.go`:

```go
package cmd

import (
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
	"github.com/SAP-samples/sap-devs-cli/internal/config"
	"github.com/SAP-samples/sap-devs-cli/internal/content"
	"github.com/SAP-samples/sap-devs-cli/internal/i18n"
	"github.com/SAP-samples/sap-devs-cli/internal/learning"
	"github.com/SAP-samples/sap-devs-cli/internal/mcpserver"
	"github.com/SAP-samples/sap-devs-cli/internal/tutorials"
	"github.com/SAP-samples/sap-devs-cli/internal/xdg"
)

var mcpServeProfile string

var mcpServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the SAP developer context MCP server (stdio)",
	Long:  "Starts a Model Context Protocol server on stdio. AI tools spawn this as a child process to query SAP developer knowledge on demand.",
	RunE: func(cmd *cobra.Command, args []string) error {
		loader, err := newContentLoader()
		if err != nil {
			return fmt.Errorf("failed to initialise content loader: %w", err)
		}

		paths, err := xdg.New()
		if err != nil {
			return err
		}

		profileID := mcpServeProfile
		if profileID == "" {
			cp, err := config.LoadProfile(paths.ConfigDir)
			if err != nil {
				return err
			}
			profileID = cp.ID
		}

		var activeProfile *content.Profile
		if profileID != "" {
			activeProfile, err = loader.FindProfile(profileID)
			if err != nil {
				return err
			}
			if activeProfile == nil {
				return fmt.Errorf("profile %q not found", profileID)
			}
		}

		packs, err := loader.LoadPacks(activeProfile, i18n.ActiveLang)
		if err != nil {
			return fmt.Errorf("failed to load packs: %w", err)
		}

		tutorialIndex, _ := tutorials.LoadIndex(paths.CacheDir)
		learningIndex, _ := learning.LoadIndex(paths.CacheDir, learning.CacheTTL)

		deps := mcpserver.Deps{
			Packs:         packs,
			Profile:       activeProfile,
			TutorialIndex: tutorialIndex,
			LearningIndex: learningIndex,
			Version:       Version,
		}

		s := mcpserver.NewServer(deps)

		fmt.Fprintln(os.Stderr, "sap-devs MCP server starting...")
		if err := server.ServeStdio(s); err != nil {
			return fmt.Errorf("MCP server error: %w", err)
		}
		return nil
	},
}

func init() {
	mcpServeCmd.Flags().StringVar(&mcpServeProfile, "profile", "", "override active profile")
	mcpCmd.AddCommand(mcpServeCmd)
}
```

- [ ] **Step 2: Skip update check for mcp serve**

In `cmd/root.go`, find the update check skip condition (line ~45):

```go
if cmd.Name() == "update" || Version == "dev" {
```

Change to:

```go
if cmd.Name() == "update" || cmd.Name() == "serve" || Version == "dev" {
```

- [ ] **Step 3: Verify build**

Run:
```bash
go build ./... && go vet ./...
```
Expected: clean build of entire project

- [ ] **Step 4: Quick smoke test**

Run:
```bash
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"0.1"}}}' | go run . mcp serve 2>/dev/null
```
Expected: JSON response containing `"serverInfo":{"name":"sap-devs",...}`

- [ ] **Step 5: Commit**

```bash
git add cmd/mcp_serve.go cmd/root.go
git commit -m "feat(mcp-server): add 'mcp serve' cobra command with stdio transport"
```

---

## Task 11: Self-install mcp.yaml entry

**Files:**
- Create: `content/packs/base/mcp.yaml`

- [ ] **Step 1: Create mcp.yaml**

Create `content/packs/base/mcp.yaml`:

```yaml
- id: sap-devs-server
  name: SAP Developer Context Server
  description: Live MCP server exposing SAP tips, resources, error patterns, news, tutorials, and learning journeys
  install:
    command: sap-devs
    args: ["mcp", "serve"]
  hosts:
    - claude-code
    - cursor
    - continue
```

- [ ] **Step 2: Verify the entry is discoverable**

Run:
```bash
SAP_DEVS_DEV=1 go run . mcp list --all
```
Expected: output includes `sap-devs-server` with description

- [ ] **Step 3: Commit**

```bash
git add content/packs/base/mcp.yaml
git commit -m "feat(mcp-server): add self-install entry to base pack mcp.yaml"
```

---

## Task 12: Update documentation

**Files:**
- Modify: `CLAUDE.md`
- Modify: `content/packs/base/context.md`

- [ ] **Step 1: Add mcp serve to CLAUDE.md CLI commands table**

In `CLAUDE.md`, find the `### CLI Commands` table and add a row for `mcp serve`:

```
| `mcp list/install/status/serve` | Browse, wire, and self-host SAP MCP servers |
```

(Update the existing `mcp list/install/status` row to include `serve`.)

- [ ] **Step 2: Add mcp serve to base context.md CLI reference**

In `content/packs/base/context.md`, find the CLI reference table (the `sap-devs CLI Reference` section) and update the `mcp` row to include `serve`:

```
| `mcp` | `list/install/status/serve` | Manage SAP MCP servers; `serve` starts the built-in MCP server on stdio |
```

- [ ] **Step 3: Commit**

```bash
git add CLAUDE.md content/packs/base/context.md
git commit -m "docs: add mcp serve to CLI reference tables"
```

---

## Task 13: Final verification

- [ ] **Step 1: Full build + vet**

Run:
```bash
go build ./... && go vet ./...
```
Expected: clean

- [ ] **Step 2: Verify mcp serve starts and responds**

Run:
```bash
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"0.1"}}}' | SAP_DEVS_DEV=1 go run . mcp serve 2>/dev/null | head -1
```
Expected: JSON response with `"serverInfo":{"name":"sap-devs"}`

- [ ] **Step 3: Verify tool listing**

Send a tools/list request after initialize:
```bash
printf '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"0.1"}}}\n{"jsonrpc":"2.0","id":2,"method":"tools/list"}\n' | SAP_DEVS_DEV=1 go run . mcp serve 2>/dev/null
```
Expected: response contains all 9 tool names: `list_packs`, `get_context`, `get_tip`, `search_resources`, `get_known_errors`, `get_recent_news`, `search_tutorials`, `search_learning_journeys`, `get_samples`

- [ ] **Step 4: Verify self-install is discoverable**

Run:
```bash
SAP_DEVS_DEV=1 go run . mcp list --all
```
Expected: `sap-devs-server` appears in the list

- [ ] **Step 5: Final commit if any fixes were needed**

Only if changes were required during verification.
