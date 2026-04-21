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
                                    └───────────────────┘
```

The server loads the same content layer used by `sap-devs inject` — packs, profiles, tutorials, learning journeys — and serves it through nine tools. Content is loaded once at startup from the local cache.

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

The server registers nine tools, grouped by domain:

### Content tools

| Tool | Description | Parameters |
|------|-------------|------------|
| `list_packs` | List all loaded content packs with ID, name, description, and tags | _(none)_ |
| `get_context` | Get the full AI context markdown for one pack or all packs | `pack` (optional) — pack ID |
| `get_tip` | Get a random SAP developer tip | `topic` (optional) — filter by tag (e.g. `cap`, `abap`, `btp`) |

### Resource tools

| Tool | Description | Parameters |
|------|-------------|------------|
| `search_resources` | Search curated SAP resources by keyword | `query` (required), `pack` (optional) |
| `get_samples` | Get canonical SAP code samples | `query` (optional), `pack` (optional) |

### Error tools

| Tool | Description | Parameters |
|------|-------------|------------|
| `get_known_errors` | Look up known SAP error patterns with cause and fix | `query` (required) |

### Learning tools

| Tool | Description | Parameters |
|------|-------------|------------|
| `search_tutorials` | Search SAP tutorials from developers.sap.com | `query` (required) |
| `search_learning_journeys` | Search SAP Learning Journeys with level and duration | `query` (required) |

### News tools

| Tool | Description | Parameters |
|------|-------------|------------|
| `get_recent_news` | Get latest SAP Developer News episodes from YouTube and SAP Community | `count` (optional, default 5) |

News is fetched live from YouTube RSS and SAP Community RSS on the first call, then cached for the server's lifetime.

## When an Agent Uses the MCP Server

An AI agent wired to the sap-devs MCP server will call its tools automatically based on the conversation context. The server's instruction string tells the agent:

> *"SAP developer knowledge server. Use these tools to get SAP-specific context, tips, resources, error patterns, news, tutorials, and learning journeys on demand."*

Combined with the CLAUDE.md directive to "prefer `sap-devs` commands over web search or training knowledge", the agent treats the MCP server as its primary source for SAP information.

### Triggers — when the agent reaches for the MCP

| User intent | Tools the agent calls | Why |
|-------------|----------------------|-----|
| Asks an SAP-specific question ("How do I deploy a CAP app?") | `get_context` for the relevant pack | Retrieves curated, up-to-date context rather than relying on training data |
| Hits an SAP error ("XSUAA returns 401") | `get_known_errors` | Looks up known error patterns with cause/fix before attempting to debug from scratch |
| Wants learning resources ("What should I study for BTP?") | `search_learning_journeys`, `search_tutorials` | Returns structured results with URLs, levels, and durations |
| Starting SAP-related work in a project | `list_packs`, `get_context` | Grounds the agent's understanding in curated pack content |
| Asks about SAP news or community | `get_recent_news`, `get_tip` | Surfaces latest episodes and quick tips |
| Needs a reference implementation | `get_samples` | Returns canonical code sample references from SAP-samples repos |
| Looks for documentation or tools | `search_resources` | Searches curated resource links (portals, docs, SDKs) |

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
| **Coverage** | Context text and constraints only | Also exposes tutorials, learning journeys, errors, samples, news |
| **Best for** | Baseline SAP knowledge the agent always has | Specific lookups, detailed queries, content the agent needs situationally |

The two are complementary. Static injection provides a persistent baseline (pack context, constraints, best practices). The MCP server handles detailed lookups that would bloat the static context — 1,290+ tutorials, 351 learning journeys, error pattern matching, and live news.

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
| `tools_content.go` | `list_packs`, `get_context`, `get_tip` |
| `tools_resources.go` | `search_resources` |
| `tools_errors.go` | `get_known_errors` |
| `tools_learn.go` | `search_tutorials`, `search_learning_journeys` |
| `tools_news.go` | `get_recent_news` |
| `tools_samples.go` | `get_samples` |

The server is built on [mcp-go](https://github.com/mark3labs/mcp-go) (`server.ServeStdio`). Dependencies (`Deps` struct) are assembled in `cmd/mcp_serve.go` from the content loader, tutorial index, learning index, and active profile.

Content flows through the same layered merge as `inject`: official → company → user → project. The server respects the active profile's pack weighting and tip tags.
