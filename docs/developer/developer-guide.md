# sap-devs Developer Guide

This guide covers everything you need to build, test, and release the `sap-devs` CLI.

---

## Prerequisites

- **Go 1.26.1+** — [download](https://go.dev/dl/)
- **git**
- **Linux only:** `libx11-dev` (required by the clipboard dependency `golang.design/x/clipboard`)
  ```bash
  sudo apt-get install -y libx11-dev
  ```

---

## Clone & Build

```bash
git clone https://github.tools.sap/developer-relations/sap-devs-cli
cd sap-devs-cli

VERSION=$(git describe --tags --always --dirty)
go build -ldflags "-X github.tools.sap/developer-relations/sap-devs-cli/cmd.Version=${VERSION}" -o sap-devs .
```

This produces a `sap-devs` binary in the current directory. The module path is `github.tools.sap/developer-relations/sap-devs-cli`.

---

## Local Development

Set `SAP_DEVS_DEV=1` to load content from `./content/` instead of the user cache. This lets you iterate on content changes without syncing:

```bash
# macOS / Linux / Git Bash
SAP_DEVS_DEV=1 go run . inject --dry-run
```

```powershell
# PowerShell (Windows)
$env:SAP_DEVS_DEV="1"; go run . inject --dry-run
```

Use `go run .` (rather than rebuilding) for rapid iteration during development.

---

## Linting & Static Analysis

```bash
go build ./...
go vet ./...
```

> **Windows note:** `go test` always fails locally because Windows Defender blocks execution of test binaries from `~/.config` paths. Use `go build` + `go vet` locally. CI is the authoritative test runner.

---

## Running Tests

```bash
# All packages
go test ./...

# Single package
go test ./internal/content/...
go test ./internal/i18n/...
```

CI runs on a self-hosted `Linux X64` runner and is the authoritative test runner. On Windows, tests may fail locally but pass in CI (Linux). A test failure in CI that passes locally indicates a genuine cross-platform bug.

---

## Project Layout

```bash
sap-devs-cli/
├── cmd/                    # Cobra command definitions (one file per command)
├── internal/
│   ├── adapter/            # Adapter engine — pushes context into AI tools
│   ├── config/             # Config file read/write
│   ├── content/            # Content loader — merges 4 content layers
│   ├── credentials/        # Secure token storage (OS keychain + file fallback)
│   ├── discovery/          # Discovery Center API client and cache
│   ├── i18n/               # Internationalisation: language resolution, T(), Tf()
│   │   └── catalogs/       # JSON string catalogs per language (en.json, de.json, …)
│   ├── learn/              # Cross-type learning recommendations, search, and paths
│   ├── learning/           # Learning journey catalog and search API client
│   ├── mcpserver/          # Built-in MCP server (sap-devs mcp serve)
│   ├── project/            # Project detection and health checks
│   ├── sync/               # Sync engine — fetches official/company repo zips
│   ├── tutorials/          # Tutorial fetching, parsing, and search
│   ├── update/             # Self-update logic
│   └── xdg/                # Platform-native config/cache/data paths
├── content/
│   ├── adapters/           # Adapter definitions (one YAML per AI tool)
│   ├── packs/              # Content packs (one directory per pack)
│   ├── profiles/           # Developer persona profiles
│   └── schemas/            # JSON Schema files for YAML validation
├── .github/
│   ├── workflows/ci.yml    # Test + build on every push/PR
│   └── workflows/release.yml  # GoReleaser triggered by v* tags
├── .goreleaser.yml         # Cross-platform release configuration
├── go.mod / go.sum
└── main.go
```

---

## Architecture Overview

### Content Layer System

Content is loaded from up to four sources, merged by `id` with later layers overriding earlier ones:

1. **Official** — fetched from the official repo, cached at `~/.cache/sap-devs/official/`
2. **Company** — optional, set via `sap-devs config company <url>`, cached at `~/.cache/sap-devs/company/`
3. **User** — `~/.local/share/sap-devs/` (Linux), `%LOCALAPPDATA%/sap-devs/data/` (Windows)
4. **Project** — `.sap-devs/` in the current working directory

`ContentLoader` (`internal/content/loader.go`) manages the merge. `LoadPacks()` reads all `content/packs/<name>/` directories. Each pack may contain `context.md` (AI context text), `constraints.md` (AI constraint rules — things agents should NOT do), `preamble.md` (base pack only), `known_errors.yaml` (common SAP error patterns with cause/fix), and various YAML files.

### Adapter System

Adapters (`content/adapters/<tool>.yaml`) define how to push context into a specific AI tool. Three types:

- **`file-inject`** — writes a fenced section into a config file (e.g. `~/.claude/CLAUDE.md`) using HTML comment markers. The section is identified by markers of the form `<!-- sap-devs:start:Section Name -->` and `<!-- sap-devs:end:Section Name -->`. Supports `replace-section` mode (replaces an existing section or appends if not present) and `replace-file` mode (overwrites the file entirely). `inject --uninstall` reverses both modes: `replace-section` removes the fenced block; `replace-file` deletes the file.
- **`clipboard-export`** — copies context to clipboard (global scope only).
- **`mcp-wire`** — registers MCP servers in the tool's JSON config (used by `mcp install`, not `inject`).

The `Engine` (`internal/adapter/engine.go`) iterates adapters, filters by `--tool` flag and scope (`global`/`project`), and dispatches to the appropriate handler. `Run()` returns a `RunResult{Found, DryFound int; Err error}` — `Found` is the count of sections/files removed (live mode), `DryFound` the count that would be removed (dry-run mode).

> `Status() ([]StatusRow, error)` — inspects all `file-inject` targets for the configured scope and returns one `StatusRow` per `(adapter, target)` pair. Each row reports file existence, injection state, staleness (via content-hash comparison using `renderSectionContent`), and stretch-goal file-analysis fields. Defined alongside its types and helpers in `internal/adapter/status.go`.

### Profiles

Profiles (`content/profiles/`) are YAML files that tag which packs belong to a developer persona (e.g. `cap-developer`). `ApplyWeights()` reorders packs to prioritise those matching the active profile. The active profile is stored in `~/.config/sap-devs/profile.yaml`.

### Sync

`sap-devs sync` (`cmd/sync.go`) fetches the official repo as a `.zip` archive and extracts it to the cache. Per-category TTLs are tracked in `~/.cache/sap-devs/sync-state.json` via `sync.Engine` (`internal/sync/engine.go`). Use `--force` to ignore TTLs.

The auth token is resolved once at the top of `syncCmd.RunE` via `credentials.Resolve()` and passed to both `FetchArchive` calls (official + company repo). `FetchArchive` signature: `FetchArchive(rawURL, destDir, token string) error`.

### News

`sap-devs news` (`cmd/news.go`) fetches SAP Developer News episodes live on every invocation — no caching or sync integration.

**Packages:**

| Package | Responsibility |
| --- | --- |
| `internal/youtube` | Fetches and parses the YouTube playlist Atom RSS feed → `[]Episode` |
| `internal/community` | Fetches and parses the SAP Community RSS feed → `[]BlogPost`; also fetches post HTML and converts it to markdown via `html-to-markdown/v2` |
| `internal/news` | Correlates episodes and posts by publish date (±7-day window, LCS tiebreaker) → `[]NewsItem` |

**Key types:**

```go
// internal/youtube
type Episode struct { ID, Title, URL string; Published time.Time; Description string }

// internal/community
type BlogPost struct { Title, URL string; Published time.Time }

// internal/news
type NewsItem struct { Episode youtube.Episode; Community *community.BlogPost }
```

**Subcommands:** `list [-n]`, `latest`, `open <id>`, `search <query>`, `read <id> [--plain]`, `hook`.

**`news hook`:** Prints a Friday reminder message on Fridays, silent otherwise. Designed as a `sessionStart` hook for Claude Code — install with `sap-devs hook install community/friday-developer-news`. The pure helper `fridayHookMessage(day time.Weekday) string` holds all logic and is unit-tested in `cmd/news_test.go`. Note: this is distinct from the Friday tip override in `cmd/tip.go`, which fetches the latest episode live via YouTube RSS; `news hook` prints a static prompt and delegates fetching to the AI.

**Pager resolution** (for `news read`): `$PAGER` env var (split on whitespace to support args like `less -R`) → `exec.LookPath("less")` silent probe → plain print. On Windows, `less` is absent by default; plain print is the expected fallback.

**Static footer constants** in `cmd/news.go`: LinkedIn newsletter URL (always shown); `newsYTMusic` (suppressed when empty); `newsPlaylistURL` (playlist watch link — also used by the Friday tip override in `cmd/tip.go`).

**Friday tip override:** On Fridays, `sap-devs tip` calls `fridayNewsOverride()` (`cmd/tip.go`) which fetches `newsPlaylistRSS` via `youtube.FetchPlaylist` and returns the latest episode as a `*content.Tip`. On fetch failure or an empty playlist it falls back to a hardcoded static tip pointing at `newsPlaylistURL`. The override is skipped when `useRandom` is true (`--new` flag or `SAP_DEVS_DEV=1`).

**HTTP User-Agent:** `FetchBlogPosts` and `FetchPostContent` send `User-Agent: Mozilla/5.0 (compatible; sap-devs/1.0)`. SAP Community returns HTTP 403 to bare Go HTTP clients without this header.

### Credentials

`internal/credentials/` manages token storage and resolution.

**Functions:**

| Function | Behaviour |
| --- | --- |
| `Store(configDir, token string) error` | Saves to OS keychain; falls back to `<configDir>/credentials` (0600) if keychain unavailable. Prints an informational stderr note on fallback. |
| `Load(configDir string) (string, error)` | Reads from keychain; falls back to file on keychain error (prints stderr warning). Returns `ErrNotFound` if no token anywhere. |
| `Delete(configDir string) error` | Removes from keychain; falls back to deleting the file. Returns `ErrNotFound` if nothing stored. |
| `Resolve(configDir string) string` | Full priority chain: `GITHUB_TOOLS_SAP_TOKEN` → `GH_TOKEN` → `GITHUB_TOKEN` → `Load()` → `""`. Never errors. |

**Keychain backend:** `zalando/go-keyring` — macOS Keychain, Windows Credential Manager, Linux Secret Service (D-Bus). Falls back to credentials file when unavailable (headless Linux, CI containers).

**Security properties:**

- Token only sent in `Authorization: token <tok>` header, never in URLs or error strings
- `config show` masks the token: `<first4>****` or `(not set)`
- Credentials file is separate from `config.yaml` to prevent accidental dotfile repo exposure

**Testing:** The package uses an unexported `keyringBackend` variable (`type keyring interface`). Tests (`package credentials`) replace it with `fakeKeyring`, `unavailableKeyring`, or `notFoundKeyring` structs to exercise all paths without a real keychain. No real OS keychain is touched in CI.

**Auth redirect detection in `FetchArchive`:** After reading the response body, `FetchArchive` checks `resp.Request.URL.Host == parsedURL.Host && strings.Contains(resp.Request.URL.Path, "/login")`. If matched, it returns: `authentication required for <host> — set GITHUB_TOOLS_SAP_TOKEN or run 'sap-devs config token'`. The host in the error is always from the original URL, not the redirect target.

### i18n

The `internal/i18n` package resolves the active language and looks up strings from JSON catalogs embedded at build time:

- **Language resolution:** `config language` setting → `LANG` env var → `LC_ALL` env var → fallback `en`. Region suffixes stripped (`de_AT.UTF-8` → `de`).
- **CLI strings:** `internal/i18n/catalogs/<lang>.json`, keyed as `cmd.subcommand.string_name`.
- **Pack content:** `context.<lang>.md`, `tips.<lang>.md` alongside base files.
- **Functions:** `T(lang, key string)` for plain strings; `Tf(lang, key string, data map[string]any)` for Go `text/template` strings. Use `i18n.ActiveLang` as the `lang` argument.

`ActiveLang` is set once in `rootCmd.PersistentPreRunE` before any command body runs.

### Update Check

On every command invocation (except `update` and dev builds), a background goroutine checks GitHub for a newer release, at most once per 7 days (168h). The result is printed to stderr after the command completes, with a 3-second timeout.

### Platform Paths

`internal/xdg` resolves platform-native directories:

| Purpose | Linux | macOS | Windows |
|---|---|---|---|
| Config | `~/.config/sap-devs` | `~/Library/Application Support/sap-devs` | `%APPDATA%/sap-devs` |
| Cache | `~/.cache/sap-devs` | `~/Library/Caches/sap-devs` | `%LOCALAPPDATA%/sap-devs/cache` |
| Data | `~/.local/share/sap-devs` | `~/Library/Application Support/sap-devs/data` | `%LOCALAPPDATA%/sap-devs/data` |

XDG environment variables (`XDG_CONFIG_HOME`, `XDG_CACHE_HOME`, `XDG_DATA_HOME`) are honoured on Linux.

### Learn

`sap-devs learn` (`cmd/learn.go`, `cmd/learn_search.go`, `cmd/learn_path.go`) is an umbrella command aggregating content from learning journeys, tutorials, and Discovery Center missions. The `internal/learn` package provides:

- **`Recommend()`** — three-tier resolution per content type (featured → pack refs → profile-filtered), level normalization, filtering
- **`Search()`** — cross-type substring search with title-priority ranking
- **`LoadPaths()`/`AutoFillPaths()`/`ResolvePaths()`** — curated learning paths from `paths.yaml` + auto-generated paths from featured pack content

Experience level is stored in `config.yaml` as `experience_level` (beginner/intermediate/advanced). Mission effort values map to levels: 0-1→beginner, 2→intermediate, 3→advanced.

### Project Detection & Health Check

`internal/project` provides two entry points consumed by both `cmd/inject.go` and `cmd/doctor.go`:

- **`Detect(cwd string) (*ProjectContext, error)`** — scans well-known files (package.json, pom.xml, mta.yaml, xs-security.json, xs-app.json, chart/helm directories, default-env.json, .cdsrc.json) and returns a `ProjectContext` with typed fields (`Type`, `CAPVersion`, `Database`, `Deployment`, `Auth`) and a `Facts` slice for rendering. No network calls.
- **`Check(ctx *ProjectContext, cwd string, packs []*content.Pack) []Finding`** — runs four categories of health checks (dependency, version staleness, best-practice, constraint compliance) and returns `[]Finding` with severity (`error`/`warning`/`info`) and optional fix suggestion.

**Inject integration:** `GatherDynamic()` calls `Detect()` and converts to `content.ProjectInfo` (mirror types to avoid `content` ↔ `project` import cycle). `cmd/inject.go` then runs `Check()` and converts findings to `content.ProjectFinding`. The `renderDynamic()` function renders facts as a `**Project Context (detected):**` block with error/warning findings prefixed by ⚠.

**Doctor integration:** `cmd/doctor.go` calls `Detect()` and `Check()` directly. The `--tools-only` flag skips project health; `--project-only` skips tool version checks. `printProjectHealth()` renders findings with severity icons and fix suggestions.

**Version staleness:** `semver.go` provides `CompareVersions()` and `VersionStaleness()` for comparing detected versions against latest known versions from `pack.yaml` `versions` maps. Thresholds: ≥1 major behind → error; ≥2 minor behind → warning.

### Built-in MCP Server

`sap-devs mcp serve` (`cmd/mcp_serve.go`) starts a built-in MCP (Model Context Protocol) server on stdio, exposing SAP developer knowledge as live tools for AI agents. The server uses the `mark3labs/mcp-go` SDK.

**Package:** `internal/mcpserver/` — thin handler adapters that delegate to existing content, news, tutorial, and learning packages.

**Architecture:**

```text
cmd/mcp_serve.go          → Cobra subcommand: loads packs, builds Deps, calls NewServer + ServeStdio
internal/mcpserver/
├── server.go             → NewServer(): creates mcp-go MCPServer, registers all tool groups
├── tools_content.go      → list_packs, get_context, get_tip
├── tools_resources.go    → search_resources
├── tools_errors.go       → get_known_errors
├── tools_news.go         → get_recent_news (lazy fetch with sync.Once + timeout)
├── tools_learn.go        → search_tutorials, search_learning_journeys
└── tools_samples.go      → get_samples
```

**Deps struct:** Injected at startup — holds `[]*content.Pack`, `*content.Profile`, `[]tutorials.TutorialMeta`, `[]learning.LearningJourney`, and `Version string`. No global state.

**News fetching:** The `newsFetcher` struct uses `sync.Once` to lazily fetch YouTube RSS + SAP Community RSS on first `get_recent_news` call, with a 5-second timeout. A `sync.Mutex` protects the cached result from data races.

**Self-install:** `content/packs/base/mcp.yaml` defines a `sap-devs-server` entry so `sap-devs mcp install sap-devs-server` wires the built-in server into AI tool configs.

**9 tools:** `list_packs`, `get_context`, `get_tip`, `search_resources`, `get_known_errors`, `get_recent_news`, `search_tutorials`, `search_learning_journeys`, `get_samples`.

---

## Adding a Command

1. Create `cmd/<name>.go`.
2. Define a `*cobra.Command` with `Use`, `Short` (from `i18n.T`), and `RunE`.
3. Follow i18n key convention: `<command>.<subcommand>.short`, `<command>.<subcommand>.long`, etc. Add keys to `internal/i18n/catalogs/en.json`.
4. Register with `rootCmd.AddCommand()` (or the relevant parent) in the file's `init()`.
5. Add flags via `cmd.Flags().StringVar(...)` etc. after the command definition.

Example:

```go
var fooCmd = &cobra.Command{
    Use:   "foo <arg>",
    Short: i18n.T(i18n.ActiveLang, "foo.short"),
    RunE: func(cmd *cobra.Command, args []string) error {
        // implementation
        return nil
    },
}

func init() {
    rootCmd.AddCommand(fooCmd)
}
```

---

## Release Workflow

### Pre-release checklist

- [ ] CI is green on `main` (check `.github/workflows/ci.yml`)
- [ ] All tests pass
- [ ] `CHANGELOG` or commit history is clean and meaningful

### Tag and push

```bash
git tag v1.2.3
git push origin v1.2.3
```

The tag must match the pattern `v*`. Pushing the tag triggers the release workflow at `.github/workflows/release.yml`.

### What GoReleaser does

GoReleaser runs on `ubuntu-latest` and reads `.goreleaser.yml`:

| Platform | Architecture | Archive format |
|---|---|---|
| Linux | amd64, arm64 | `.tar.gz` |
| macOS | amd64, arm64 | `.tar.gz` |
| Windows | amd64 | `.zip` |

Windows arm64 is excluded. Archive naming: `sap-devs_<version>_<os>_<arch>.<ext>`.

Version is injected at build time:
```
-ldflags "-X github.tools.sap/developer-relations/sap-devs-cli/cmd.Version={{ .Version }}"
```

A `checksums.txt` (SHA256) is included in the release assets.

### After the release

1. Go to the GitHub Releases page and verify all platform artifacts are present.
2. Verify `checksums.txt` is attached.
3. Test by downloading and running `sap-devs --version` on at least one platform.

---

## Worktrees

Feature branch worktrees are stored in `.worktrees/` in the project root — **not** in `~/.config`. Windows Defender blocks execution of test binaries from `~/.config` paths.

```bash
# Create a worktree for a feature branch
git worktree add .worktrees/my-feature -b feature/my-feature
```
