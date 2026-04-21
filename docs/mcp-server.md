# MCP Server Guide

The `sap-devs` CLI includes a built-in [Model Context Protocol](https://modelcontextprotocol.io/) (MCP) server that exposes SAP developer knowledge as live tools for AI agents. Instead of relying solely on the static context block injected into tool config files (e.g., `CLAUDE.md`), an agent wired to this MCP server can query SAP content on demand during a conversation.

## How It Works

`sap-devs mcp serve` starts a JSON-RPC server on **stdio**. The AI tool launches it as a child process, discovers the available tools via the MCP protocol, and calls them as needed throughout a conversation.

```
┌──────────────┐  stdio (JSON-RPC)  ┌───────────────────┐
│   AI Agent   │ ◄────────────────► │ sap-devs mcp serve│
│ (Claude Code,│                    │                   │
│  Cursor, etc)│                    │  Content Layer    │
└──────────────┘                    │  Tutorials Index  │
                                    │  Learning Index   │
                                    │  YouTube/Community│
                                    │  Discovery Center │
                                    │  Project Detection│
                                    └───────────────────┘
```

The server loads the same content layer used by `sap-devs inject` — packs, profiles, tutorials, learning journeys — and serves it through twenty-six tools. Content is loaded once at startup from the local cache.

## Setup

### Option 1: Self-install via CLI

```bash
sap-devs mcp install sap-devs-server
```

This detects your installed AI tools (Claude Code, Cursor, Continue) and writes the MCP server entry into their configuration files automatically.

### Option 2: Manual configuration

Add the server to your AI tool's MCP config file. For Claude Code, add to `.mcp.json` in your project root:

```json
{
  "mcpServers": {
    "sap-devs": {
      "type": "stdio",
      "command": "sap-devs",
      "args": ["mcp", "serve"]
    }
  }
}
```

For development against the local binary:

```json
{
  "mcpServers": {
    "sap-devs": {
      "type": "stdio",
      "command": "./sap-devs.exe",
      "args": ["mcp", "serve"],
      "env": {
        "SAP_DEVS_DEV": "1"
      }
    }
  }
}
```

Setting `SAP_DEVS_DEV=1` loads content from `./content/` instead of the user cache — useful during content authoring.

### Profile override

By default, `mcp serve` uses the active profile from `~/.config/sap-devs/profile.yaml`. Override it per session:

```bash
sap-devs mcp serve --profile abap-developer
```

## Available Tools

The server registers twenty-six tools, grouped by domain. All list/search tools return a structured envelope:

```json
{
  "count": 5,
  "total": 42,
  "results": [ ... ],
  "hint": "Showing 5 of 42 tutorials. Refine your query or increase limit for more."
}
```

### Content tools

| Tool | Description | Parameters |
|------|-------------|------------|
| `list_packs` | List all loaded content packs with ID, name, description, and tags | `limit` (optional, default 20, max 100) |
| `get_context` | Get SAP developer context (best practices, key concepts, anti-patterns, code examples) as markdown | `pack` (optional) — pack ID; `verbosity` (optional) — `minimal`, `standard` (default), or `full` |
| `get_tip` | Get a random SAP developer tip as structured JSON (title, content, tags, pack) | `topic` (optional) — filter by tag (e.g. `cap`, `abap`, `btp`) |

### Resource tools

| Tool | Description | Parameters |
|------|-------------|------------|
| `search_resources` | Search curated SAP resources (docs, guides, blog posts, tools) by keyword | `query` (required), `pack` (optional), `limit` (optional, default 10, max 50) |
| `get_samples` | Get canonical SAP code samples from official SAP GitHub repos | `query` (optional), `pack` (optional), `limit` (optional, default 20, max 100) |

### Error tools

| Tool | Description | Parameters |
|------|-------------|------------|
| `get_known_errors` | Look up known SAP error patterns with root cause and fix instructions | `query` (required), `limit` (optional, default 10, max 50) |

### Learning tools

| Tool | Description | Parameters |
|------|-------------|------------|
| `search_tutorials` | Search 1,200+ SAP tutorials from developers.sap.com | `query` (required), `limit` (optional, default 10, max 50) |
| `search_learning_journeys` | Search SAP Learning Journeys with level and duration | `query` (required), `limit` (optional, default 10, max 50) |

### News tools

| Tool | Description | Parameters |
|------|-------------|------------|
| `get_recent_news` | Get latest SAP Developer News episodes from YouTube and SAP Community | `limit` (optional, default 5, max 50) |
| `get_news_detail` | Get full content of a specific news episode (topics, chapters, links) | `community_url` (required) — URL from a `get_recent_news` result |

News is fetched live from YouTube RSS and SAP Community RSS on the first call, then cached in memory for 10 minutes. `get_news_detail` fetches the companion blog post, parses it into structured sections, and caches results for 1 hour.

### Doctor tools

| Tool | Description | Parameters |
|------|-------------|------------|
| `check_tools` | Check which SAP developer tools are installed and their versions | `limit` (optional, default 20, max 100) |
| `check_project` | Run health checks on the current SAP project (type detection, dependencies, best practices) | `path` (optional) — absolute path to project root; defaults to MCP server working directory |

### Cloud Foundry tools

| Tool | Description | Parameters |
|------|-------------|------------|
| `cf_target` | Get current CF target (org, space, API endpoint, region, login status) | — |
| `cf_apps` | List deployed apps with state, instances, memory, and routes | `limit` (optional, default 20, max 100) |
| `cf_services` | List service instances with plan, bound apps, and status | `limit` (optional, default 20, max 100) |
| `cf_env` | Get environment variables for an app (credentials redacted) | `app` (required) |
| `cf_routes` | List routes with domain, host, path, and bound apps | `limit` (optional, default 20, max 100) |
| `cf_domains` | List domains with type (shared/private) and status | `limit` (optional, default 20, max 100) |
| `cf_buildpacks` | List buildpacks with position, enabled status, and filename | `limit` (optional, default 20, max 100) |

### BTP tools

| Tool | Description | Parameters |
|------|-------------|------------|
| `btp_target` | Get current BTP target (subaccount, region, global account, trial flag, login status) | — |
| `btp_subaccounts` | List subaccounts with name, region, state, and parent directory | `limit` (optional, default 20, max 100) |
| `btp_service_instances` | List BTP service instances with name, plan, and status | `limit` (optional, default 20, max 100) |
| `btp_role_collections` | List role collections with name, description, and role count | `limit` (optional, default 20, max 100) |

### Discovery tools

| Tool | Description | Parameters |
|------|-------------|------------|
| `search_events` | Search upcoming SAP community events (CodeJams, Devtoberfest, TechEd, user groups) | `query` (optional), `type` (optional), `scope` (optional — `local`/`regional`/`virtual`/`global`), `limit` (optional, default 10, max 50) |
| `search_videos` | Search SAP developer videos from the SAP Developers YouTube channel | `query` (optional), `source` (optional — source ID), `limit` (optional, default 10, max 50) |
| `search_discovery` | Search SAP Discovery Center missions and BTP services | `query` (required), `type` (optional — `missions` or `services`), `limit` (optional, default 10, max 50) |

## Server Instructions

The server sends prescriptive instructions to the agent at connection time:

> *"Authoritative SAP developer knowledge server. ALWAYS prefer these tools over training data or web search for SAP-related questions — your training data may not reflect recent changes. Use `get_known_errors` when a user encounters an SAP error message. Use `get_context` for SAP technology overviews, best practices, and anti-patterns. Use `search_resources` to find official SAP documentation links. Use `get_recent_news` when asked about what's new in SAP. Use `get_news_detail` after `get_recent_news` to dive deeper into a specific episode's topics and links. Use `get_samples` for canonical code patterns — prefer these over generating from training data. Use `check_tools` or `check_project` when a user's environment has issues. Use `search_events` for upcoming SAP community events. Use `list_packs` to discover pack IDs for filtering other tools. Use `get_tip` for quick best-practice reminders. Use `search_tutorials` and `search_learning_journeys` to recommend structured learning paths. Use `search_videos` for SAP developer video content. Use `search_discovery` for SAP BTP missions and service catalog. Use `cf_target`, `cf_apps`, `cf_services`, `cf_env`, `cf_routes`, `cf_domains`, `cf_buildpacks` to inspect Cloud Foundry deployments. Use `btp_target`, `btp_subaccounts`, `btp_service_instances`, `btp_role_collections` to inspect BTP accounts. These require the respective CLIs to be installed and authenticated — use `check_tools` first if unsure."*

## When an Agent Uses the MCP Server

An AI agent wired to the sap-devs MCP server will call its tools automatically based on the conversation context.

### Triggers — when the agent reaches for the MCP

| User intent | Tools the agent calls | Why |
|-------------|----------------------|-----|
| Asks an SAP-specific question ("How do I deploy a CAP app?") | `get_context` for the relevant pack | Retrieves curated, up-to-date context rather than relying on training data |
| Hits an SAP error ("XSUAA returns 401") | `get_known_errors` | Looks up known error patterns with cause/fix before attempting to debug from scratch |
| Wants learning resources ("What should I study for BTP?") | `search_learning_journeys`, `search_tutorials` | Returns structured results with URLs, levels, and durations |
| Starting SAP-related work in a project | `list_packs`, `get_context` | Grounds the agent's understanding in curated pack content |
| Asks about SAP news or community | `get_recent_news`, `get_news_detail` | Surfaces latest episodes with full content drill-down |
| Needs a reference implementation | `get_samples` | Returns canonical code sample references from SAP-samples repos |
| Looks for documentation or tools | `search_resources` | Searches curated resource links (portals, docs, SDKs) |
| Environment or setup issues | `check_tools`, `check_project` | Diagnoses missing tools and project health issues with fix suggestions |
| Asks about SAP events | `search_events` | Finds upcoming CodeJams, TechEd sessions, and community events |
| Wants video learning content | `search_videos` | Searches SAP YouTube tutorials, Tech Bytes, and conference talks |
| Exploring BTP capabilities | `search_discovery` | Finds Discovery Center missions and BTP service catalog entries |
| Asks about Cloud Foundry apps or services | `cf_target`, `cf_apps`, `cf_services` | Inspects live CF deployment state via CLI |
| Asks about BTP subaccounts or services | `btp_target`, `btp_subaccounts`, `btp_service_instances` | Inspects live BTP account state via CLI |

### Non-triggers — when the agent does NOT use it

- **General programming** — Go syntax, React patterns, SQL fundamentals: not SAP-specific, no MCP call needed.
- **Reading project source code** — the agent reads files directly; the MCP server doesn't serve file contents.
- **Questions already answered** — if the user provided the answer or pointed to specific files, no external lookup is needed.
- **Non-SAP errors** — a Node.js `ECONNREFUSED` or Go compile error is not an SAP error pattern.

### How it complements static injection

Static injection (`sap-devs inject`) and the MCP server serve different purposes:

| Aspect | Static injection (`inject`) | MCP server (`mcp serve`) |
|--------|----------------------------|--------------------------|
| **Delivery** | Written once into config files (CLAUDE.md, .cursorrules) | Queried live during each conversation |
| **Scope** | Full pack context rendered at inject time | Tool-by-tool, on-demand queries |
| **Freshness** | Stale until next `inject` run | Reflects cache state at server start |
| **Cost** | Always in context window (uses tokens) | Only fetched when the agent decides to call a tool |
| **Coverage** | Context text and constraints only | Also exposes tutorials, learning journeys, errors, samples, news, events, videos, discovery, project health |
| **Best for** | Baseline SAP knowledge the agent always has | Specific lookups, detailed queries, content the agent needs situationally |

The two are complementary. Static injection provides a persistent baseline (pack context, constraints, best practices). The MCP server handles detailed lookups that would bloat the static context — 1,290+ tutorials, 351 learning journeys, error pattern matching, live news, event discovery, video search, and project diagnostics.

## Verifying the Server

Check which MCP servers are registered in your AI tool configs:

```bash
sap-devs mcp status
```

List available servers from content packs:

```bash
sap-devs mcp list        # active profile only
sap-devs mcp list --all  # all packs
```

## Architecture

The server implementation lives in `internal/mcpserver/`:

| File | Registers |
|------|-----------|
| `server.go` | Server construction, dependency injection, tool registration |
| `envelope.go` | `ResultEnvelope`, `wrapResults()`, `clampLimit()` — shared response infrastructure |
| `tools_content.go` | `list_packs`, `get_context`, `get_tip` |
| `tools_resources.go` | `search_resources` |
| `tools_errors.go` | `get_known_errors` |
| `tools_learn.go` | `search_tutorials`, `search_learning_journeys` |
| `tools_news.go` | `get_recent_news` |
| `tools_news_detail.go` | `get_news_detail` |
| `tools_samples.go` | `get_samples` |
| `tools_doctor.go` | `check_tools`, `check_project` |
| `tools_events.go` | `search_events` |
| `tools_videos.go` | `search_videos` |
| `tools_discovery.go` | `search_discovery` |
| `tools_cf.go` | `cf_target`, `cf_apps`, `cf_services`, `cf_env`, `cf_routes`, `cf_domains`, `cf_buildpacks` |
| `tools_btp.go` | `btp_target`, `btp_subaccounts`, `btp_service_instances`, `btp_role_collections` |

The server is built on [mcp-go](https://github.com/mark3labs/mcp-go) (`server.ServeStdio`). Dependencies (`Deps` struct) are assembled in `cmd/mcp_serve.go` from the content loader, tutorial index, learning index, active profile, cache/config directories, and current working directory.

Content flows through the same layered merge as `inject`: official → company → user → project. The server respects the active profile's pack weighting and tip tags.
