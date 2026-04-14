# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Development Commands

```bash
# Build (with version injection)
VERSION=$(git describe --tags --always --dirty)
go build -ldflags "-X github.tools.sap/developer-relations/sap-devs-cli/cmd.Version=${VERSION}" -o sap-devs .

# Lint / static analysis (use instead of go test on Windows — Windows Defender blocks test binaries from ~/.config)
go build ./...
go vet ./...

# Run tests (CI only — authoritative test runner is ubuntu-latest in GitHub Actions)
go test ./...

# Run a single test package
go test ./internal/content/...

# Local dev mode — loads content from ./content/ instead of the cache
SAP_DEVS_DEV=1 go run . inject --dry-run
```

> **Windows note:** `go test` always fails locally due to Windows Defender blocking binary execution from `.config` paths. Use `go build` + `go vet` locally; CI is the authoritative test runner.

## Architecture Overview

This is a Go CLI built with [cobra](https://github.com/spf13/cobra). Its core purpose is to inject SAP developer knowledge into AI coding tools (Claude Code, Cursor, Copilot, etc.) and wire up SAP MCP servers.

### Content Layer System

Content is loaded from up to four layered sources, with later layers overriding earlier ones by ID:

1. **Official** — cached from `github.tools.sap/developer-relations/sap-devs-cli` repo at `~/.cache/sap-devs/official/`
2. **Company** — optional, configured via `sap-devs config company <git-url>`, cached at `~/.cache/sap-devs/company/`
3. **User** — `~/.local/share/sap-devs/` (Linux), `%LOCALAPPDATA%/sap-devs/data/` (Windows)
4. **Project** — `.sap-devs/` in the current working directory

`ContentLoader` ([internal/content/loader.go](internal/content/loader.go)) manages this merge. `LoadPacks()` reads all `content/packs/<name>/` directories; each pack contains: `pack.yaml` (metadata), `context.md` (AI context text), `tips.md` (H2-delimited tips), `tools.yaml`, `resources.yaml`, `mcp.yaml`.

### Adapter System

Adapters ([internal/adapter/](internal/adapter/)) define how to push context into a specific AI tool. They are YAML files in `content/adapters/` and support three types:
- **`file-inject`** — writes a fenced section into a config file (e.g., `~/.claude/CLAUDE.md`), using `replace-section` or `append` mode
- **`clipboard-export`** — copies context to clipboard (global scope only)
- **`mcp-wire`** — registers MCP servers in a tool's JSON config (handled by `mcp install`, not `inject`)

The `Engine` ([internal/adapter/engine.go](internal/adapter/engine.go)) iterates adapters, filters by `--tool` flag and scope (`global`/`project`), and dispatches to the appropriate handler.

### Profiles

Profiles ([content/profiles/](content/profiles/)) are YAML files that tag which packs belong to a developer persona (e.g., `cap-developer`, `abap-developer`). `ApplyWeights()` reorders packs to prioritise those matching the active profile. The active profile ID is stored in `~/.config/sap-devs/profile.yaml`.

### Sync

`sap-devs sync` ([cmd/sync.go](cmd/sync.go)) fetches the official repo as a `.zip` archive and extracts it into the cache. Per-category TTLs are tracked in `~/.cache/sap-devs/sync-state.json` via `sync.Engine` ([internal/sync/engine.go](internal/sync/engine.go)). Forced refresh: `--force`.

### Credentials

`internal/credentials` ([internal/credentials/credentials.go](internal/credentials/credentials.go)) provides secure token storage. `Store`/`Load`/`Delete` use the OS keychain via `zalando/go-keyring` with a `<configDir>/credentials` file fallback (0600). `Resolve()` implements the full priority chain: env vars (`GITHUB_TOOLS_SAP_TOKEN`, `GH_TOKEN`, `GITHUB_TOKEN`) → keychain → file → `""`. Used by `sync` and `config token`.

### Platform Paths

`internal/xdg` ([internal/xdg/xdg.go](internal/xdg/xdg.go)) resolves platform-native directories:
- **Linux**: `~/.config/sap-devs`, `~/.cache/sap-devs`, `~/.local/share/sap-devs` (XDG env vars honoured)
- **macOS**: `~/Library/Application Support/sap-devs`, `~/Library/Caches/sap-devs`
- **Windows**: `%APPDATA%/sap-devs`, `%LOCALAPPDATA%/sap-devs/cache`, `%LOCALAPPDATA%/sap-devs/data`

### Update Check

On every command invocation (except `update` and dev builds), a background goroutine checks GitHub for a newer release at most once every 7 days (168h). Results are printed to stderr after the command completes, with a 3-second timeout. State tracked in the cache directory.

### CLI Commands

| Command | Purpose |
|---|---|
| `inject` | Push rendered context into detected AI tools (`--project` for project scope) |
| `sync` | Fetch latest content from official/company repos |
| `profile set/list` | Manage active developer persona |
| `config show/set/company` | View and edit `~/.config/sap-devs/config.yaml` |
| `tip` | Show a random tip from the active profile's packs |
| `doctor` | Check local tool versions against pack requirements (`--fix` for install hints) |
| `mcp list/install/status` | Browse and wire SAP MCP servers into AI tool configs |
| `resources` | List curated resources from active packs |
| `update` | Self-update the binary |
| `init` | First-time setup wizard |

### Release

Releases use GoReleaser triggered by `v*` tags. The binary is named `sap-devs`. Version is injected at build time via `-ldflags`.

### Worktrees

Git worktrees for feature branches are stored in `.worktrees/` in the project root (not in `~/.config` — Windows Defender blocks test binary execution from that path).
