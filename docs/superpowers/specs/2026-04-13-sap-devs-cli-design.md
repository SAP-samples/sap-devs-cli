# sap-devs CLI — Design Specification

**Date:** 2026-04-13  
**Status:** Draft  
**Repo:** https://github.tools.sap/developer-relations/sap-devs-cli  
**Author:** Thomas Jung  

---

## Problem Statement

SAP developers work across a sprawling ecosystem — CAP, ABAP, BTP, Fiori, AI/Joule, mobile, integration, analytics — and increasingly rely on AI coding assistants. Those AI tools know almost nothing about SAP's specific tools, APIs, patterns, or best practices out of the box. The result: developers spend time re-explaining SAP context to their AI tools, miss updated resources, and have no single authoritative place to discover what tools, SDKs, and MCP servers exist.

`sap-devs` solves this by being an **AI-first developer tool**: its primary job is to inject up-to-date, curated SAP developer knowledge into every AI tool a developer uses. The secondary job is to surface that same knowledge directly to the developer (resources, tips, environment checks).

---

## Decisions

| Question | Decision |
|---|---|
| Primary audience | AI tools (inject context); developers are secondary |
| Implementation language | Go — single cross-platform binary, no runtime dependency |
| Content storage | Git-based layered system: official repo → company repo → user local |
| Configuration storage | XDG Base Directory Spec (`~/.config/sap-devs/`, `~/.cache/sap-devs/`) |
| AI platform strategy | All platforms via data-driven adapter YAML files; no code change to add a new tool |
| Injection scope | Both global (user-level) and per-project, with inheritance |
| Personalization | Developer profile/persona system that weights and reorders content; never gates it |
| Extensibility | Company-level: second git repo URL in config. User-level: local override files |
| Community contributions | Open source PRs to Markdown/YAML content files; no Go knowledge needed |
| Environment checking | `doctor` command driven by version detection rules in content repo |

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                      CONTENT LAYER                           │
│                                                              │
│  Official Repo          Company Repo         User Local           │
│  (this project)         (git URL in config)  (~/.local/share/     │
│  content/packs/*        same structure       sap-devs/user/)      │
│  content/advocates/     overrides official   overrides all        │
│  content/profiles/                                           │
│  content/adapters/                                           │
└──────────────────────┬──────────────────────────────────────┘
                       │ sap-devs sync
                       ▼
┌─────────────────────────────────────────────────────────────┐
│                   LOCAL CACHE                                │
│                                                              │
│  ~/.cache/sap-devs/official/   (synced from official repo)  │
│  ~/.cache/sap-devs/company/    (synced from company repo)   │
│  ~/.config/sap-devs/config.yaml                             │
│  ~/.config/sap-devs/profile.yaml                            │
│  ~/.local/share/sap-devs/user/ (user overrides)             │
│  .sap-devs/                    (per-project, in repo)        │
└──────────────────────┬──────────────────────────────────────┘
                       │ resolve: merge layers + apply profile weights
                       ▼
┌─────────────────────────────────────────────────────────────┐
│               GO BINARY ENGINE                               │
│                                                              │
│  init · sync · inject · profile · mcp · tip                  │
│  resources · doctor · config · update                        │
│                                                              │
│  AI Adapters (data-driven):                                  │
│  file-inject · clipboard-export · mcp-wire                   │
└──────────────────────┬──────────────────────────────────────┘
                       │ sap-devs inject
                       ▼
┌─────────────────────────────────────────────────────────────┐
│                   AI TOOLS                                   │
│                                                              │
│  Claude Code  Cursor  Copilot  Continue  JetBrains AI  Cody  │
│  ChatGPT  Gemini  Claude.ai  Copilot Chat                    │
│  SAP AI Core  SAP Joule  SAP Build Code AI                   │
│  + any future tool via adapter YAML                          │
└─────────────────────────────────────────────────────────────┘
```

**Data flow:** Content repos → `sync` → local cache → resolve (profile + layer merge) → `inject` → AI tool configs

---

## Content Model

### Pack Structure

Each knowledge domain is a **pack** — a directory under `content/packs/` containing:

```
content/packs/<domain>/
  pack.yaml       — metadata: id, name, description, tags, profiles, base weight
  context.md      — AI context text injected into system prompts / instruction files
  resources.yaml  — curated links: docs, blogs, videos, tools, community (each entry has a stable slug `id` composed of `<pack>/<slug>`, e.g. `cap/docs-official`; slugs are set by the author and never auto-generated so they survive re-syncs)
  tools.yaml      — tool/SDK catalog with version detection rules
  mcp.yaml        — MCP server definitions: install command, host compatibility
  tips.md         — tip pool (one tip per H2 heading, profile-tagged)
```

### Pack Domains (v1)

| ID | Name |
|---|---|
| `cap` | SAP Cloud Application Programming Model |
| `abap` | ABAP & ABAP Cloud |
| `btp-core` | SAP BTP Core (accounts, services, deployment) |
| `fiori` | SAP Fiori & UI5 |
| `ai-joule` | SAP AI / Joule |
| `mobile` | SAP Mobile (BAS, MDK) |
| `integration` | BTP Integration Suite |
| `analytics` | SAP Analytics & Data |

### Supporting Directories

```
content/advocates/         — one YAML per SAP Developer Advocate
  thomas-jung.yaml         — name, role, blog, YouTube, socials, recent content
  dj-adams.yaml
  ...

content/profiles/          — developer persona definitions
  cap-developer.yaml       — lists pack weights/order for CAP-focused devs
  abap-developer.yaml
  btp-developer.yaml
  fiori-developer.yaml
  ai-developer.yaml
  mobile-developer.yaml
  full-stack.yaml

content/adapters/          — AI tool injection definitions (data only, no code)
  claude-code.yaml
  cursor.yaml
  copilot.yaml
  chatgpt.yaml
  gemini.yaml
  continue.yaml
  jetbrains-ai.yaml
  ...
```

### Layer Resolution Order

When building the merged context for injection:

1. Official repo cache (`~/.cache/sap-devs/official/`)
2. Company repo cache overrides official (`~/.cache/sap-devs/company/`)
3. User local overrides both (`~/.local/share/sap-devs/user/`)
4. Per-project overrides all (`<project>/.sap-devs/`)

Within each layer, profile weights reorder pack content — higher-weighted packs appear earlier in injected context. No content is hidden; only priority changes.

---

## Developer Profile System

A profile is a YAML file in `content/profiles/` that declares:

```yaml
id: cap-developer
name: CAP Developer
description: Building cloud-native apps with SAP CAP on BTP
packs:
  - id: cap
    weight: 100
  - id: btp-core
    weight: 80
  - id: fiori
    weight: 60
  - id: ai-joule
    weight: 40
  - id: integration
    weight: 20
  - id: abap
    weight: 10    # available but deprioritized
tip_tags: [cap, nodejs, odata, cds, btp]
```

**Effect of profile:**
- Packs with higher weight appear first in injected AI context
- Tip of the day is drawn from the profile's `tip_tags` pool
- `resources list` defaults to profile-relevant content
- `doctor` checks only the tools required by the profile's packs
- Profile info itself is injected into AI context ("This developer is a CAP developer focused on...")

Developers can set their profile with `sap-devs profile set cap-developer`. Multiple profiles are not supported in v1 — one primary profile per user.

---

## AI Adapter System

Adapters are pure YAML data files in `content/adapters/`. The Go binary reads them at runtime — adding support for a new AI tool requires no code changes.

### Adapter Types

**`file-inject`** — writes context to a file on disk
```yaml
type: file-inject
targets:
  - scope: global
    path: "~/.claude/CLAUDE.md"
    mode: replace-section      # idempotent: find section by markers, replace in-place
    section: "SAP Developer Context"
  - scope: project
    path: "./CLAUDE.md"
    mode: replace-section
    section: "SAP Developer Context"
detect:
  - command: "claude --version"
  - path: "~/.claude"
```

`replace-section` mode is idempotent: on first inject it appends the section; on subsequent injects it finds and replaces it in-place. Section boundaries are written as HTML comments: `<!-- sap-devs:start:SAP Developer Context -->` / `<!-- sap-devs:end:SAP Developer Context -->`. Content outside these markers is never modified.

**Scope flag mapping:** The `inject --global` flag (default) runs only targets where `scope: global`. The `inject --project` flag runs only targets where `scope: project`. The `inject --tool <name>` flag filters to the adapter whose `id` field matches `<name>` (i.e., the adapter filename without extension, e.g. `claude-code`), then applies the current scope. `clipboard-export` adapters have no `scope` field and are only invoked under `--global` (the default); they are skipped entirely when `--project` is specified.

**`clipboard-export`** — renders a formatted prompt for web AI tools
```yaml
type: clipboard-export
format: markdown               # or: plain-text, json
template: chatgpt-custom-instructions.tmpl
instructions: "Paste this into ChatGPT → Settings → Custom Instructions"
```

**`mcp-wire`** — installs MCP server and writes host config
```yaml
type: mcp-wire
mcp_config:
  path: "~/.claude/settings.json"
  format: json
  key: "mcpServers"
```

### MCP Server Definition Schema (`mcp.yaml`)

Each entry in a pack's `mcp.yaml` defines one installable MCP server:

```yaml
- id: cap-mcp-server
  name: SAP CAP MCP Server
  description: Exposes CAP project schema and service definitions as MCP tools
  install:
    command: "npx"
    args: ["-y", "@sap/cap-mcp-server"]
  hosts: [claude-code, cursor, vscode-mcp]   # adapter ids that support mcp-wire
  config: {}                                  # static config merged into host's mcpServers entry
```

The `id` is used by `sap-devs mcp install <id>`. The `hosts` list determines which adapter's `mcp_config` path receives the server entry when `mcp install` runs.

### Known Adapters (v1)

| Tool | Type |
|---|---|
| Claude Code | file-inject + mcp-wire |
| Cursor | file-inject + mcp-wire |
| GitHub Copilot | file-inject |
| Continue.dev | file-inject + mcp-wire |
| JetBrains AI | file-inject |
| Cody | file-inject |
| ChatGPT | clipboard-export |
| Gemini | clipboard-export |
| Claude.ai | clipboard-export |
| SAP AI Core | clipboard-export |
| SAP Joule | clipboard-export |

---

## CLI Commands

```
sap-devs init                     First-time setup wizard
sap-devs sync                     Pull latest from all configured content repos
sap-devs inject                   Push SAP context to all detected AI tools
  --tool <name>                   Target a specific tool only
  --global                        User-level injection (default)
  --project                       Current repo only
  --dry-run                       Preview changes without writing
sap-devs profile
  set <name>                      Set active developer profile
  show                            Display current profile and pack weights
  list                            List all available profiles
sap-devs mcp
  list                            List available SAP MCP servers
  install <id>                    Install and wire a specific MCP server
  install --all                   Install all MCP servers for current profile
  status                          Show installed/configured MCP servers
sap-devs tip                      Print a profile-relevant tip (for shell profile)
sap-devs resources
  list                            Browse curated resources (profile-filtered)
  open <id>                       Open a resource URL in the browser
  search <query>                  Full-text search across all resources
sap-devs doctor                   Check local tool installations vs requirements
  --fix                           Print install commands for missing/outdated tools
sap-devs config
  show                            Display current configuration
  set <key> <value>               Set a configuration value
  company <git-url>               Configure company content repo
sap-devs update                   Self-update the binary from GitHub Releases
```

### Key Workflows

**First-time setup:**
```
sap-devs init
  → downloads content cache
  → interactive: which developer profile are you?
  → interactive: which AI tools do you use?
  → injects SAP context into each detected tool
  → suggests MCP servers to install
  → offers to add `sap-devs tip` to shell profile
```

**Stay current:**
```
sap-devs sync && sap-devs inject
  → pulls latest from official + company repos
  → re-injects updated context to all tools
```

**Per-project setup:**

```
cd my-cap-project
sap-devs inject --project
  → if .sap-devs/context.yaml does not exist: creates it with defaults, then injects
  → if .sap-devs/context.yaml already exists: reads it as-is, never overwrites it
  → injects into ./CLAUDE.md, .cursorrules, .github/copilot-instructions.md
```

---

## Environment Doctor

`sap-devs doctor` checks installed tools against requirements defined in each pack's `tools.yaml`. It is profile-aware — only required tools for the active profile's packs are checked by default.

### Tool Definition Schema (`tools.yaml`)

```yaml
- id: nodejs
  name: Node.js
  required: ">=18.0.0"
  detect:
    command: "node --version"
    pattern: "v(\\d+\\.\\d+\\.\\d+)"    # semver captured in group 1
  install:
    windows: "winget install OpenJS.NodeJS.LTS"
    macos:   "brew install node@20"
    linux:   "nvm install 20"
  docs: "https://nodejs.org"
```

Detection is regex-over-command-output — no platform-specific detection logic in the Go binary. Install commands can be per-platform or use a single `all` key when universal.

`--fix` prints install or upgrade commands for both **missing** tools (not found) and **outdated** tools (found but below required version). It never executes commands without explicit user action.

---

## Storage Layout (XDG-compliant)

```
$XDG_CONFIG_HOME/sap-devs/          (~/.config/sap-devs/ on Linux)
                                     (~/Library/Application Support/sap-devs/ on macOS)
  config.yaml                        tool settings, company repo URL
  profile.yaml                       active developer profile

$XDG_CACHE_HOME/sap-devs/           (~/.cache/sap-devs/ on Linux)
                                     (~/Library/Caches/sap-devs/ on macOS)
  official/                          synced from official repo
  company/                           synced from company repo

$XDG_DATA_HOME/sap-devs/            (~/.local/share/sap-devs/ on Linux)
                                     (~/Library/Application Support/sap-devs/data/ on macOS)
  user/                              user-level content overrides

<project>/.sap-devs/                 per-project config and overrides (committable — see below)
  context.yaml                       project-level profile and pack selections
  overrides/                         pack content overrides for this project

Windows equivalents:
  Config:  %APPDATA%/sap-devs/            (via os.UserConfigDir())
  Cache:   %LOCALAPPDATA%/sap-devs/cache/ (via os.UserCacheDir())
  Data:    %LOCALAPPDATA%/sap-devs/data/  (manual: no os.UserDataDir() in Go stdlib)
```

**Platform path strategy:** `internal/xdg/` uses `os.UserConfigDir()` and `os.UserCacheDir()` for config and cache paths on all platforms (these return the platform-native location — `~/Library/...` on macOS, `%APPDATA%/...` on Windows, `~/.config/...` on Linux). For the data directory there is no Go stdlib equivalent; `internal/xdg/` computes it as `~/Library/Application Support/sap-devs/data/` on macOS and `%LOCALAPPDATA%/sap-devs/data/` on Windows (machine-local, not roaming). On Linux, `$XDG_CONFIG_HOME`, `$XDG_CACHE_HOME`, and `$XDG_DATA_HOME` environment variables are honoured if set; the data fallback is `~/.local/share/sap-devs/`.

**`.sap-devs/` and source control:** This directory is intended to be committed to the project repo, enabling teams to share a consistent SAP context configuration. Developers who want to keep it local should add it to their personal `.git/info/exclude`.

**`config.yaml` schema (user-level):**

```yaml
company_repo: "https://github.mycompany.com/dev/sap-devs-content"  # optional
sync_on_start: false       # auto-sync before inject (default: false)
```

**`profile.yaml` schema (user-level):**

```yaml
id: cap-developer          # must match a profile id in content/profiles/
```

**`.sap-devs/context.yaml` schema:**

```yaml
# Per-project profile and pack configuration
profile: cap-developer          # override user profile for this project (optional)
packs:
  include: [cap, btp-core]      # inject only these packs at project scope
  exclude: []                   # suppress these packs even if in user profile
extra_context: |                # freeform text appended to injected AI context
  This project uses HANA Cloud as its database.
  Deploy target is SAP BTP Cloud Foundry, EU10 region.
```

---

## Sync & Update Mechanism

**Content sync (`sap-devs sync`):**

- Fetches latest from the official content repo via HTTP — downloads a tagged zip archive from GitHub Releases (no git binary required, consistent with the single-binary no-runtime-dependency principle)
- Company repos: if the URL is a GitHub/GitLab HTTPS URL, the same HTTP archive mechanism is used. Private repos require a personal access token configured via `sap-devs config set company_token <token>`, stored in the OS keychain or config file. Internal git servers (non-GitHub/GitLab) are **not supported in v1**; the company repo must be a GitHub or GitLab instance accessible over HTTPS.
- Updates local cache; preserves user overrides
- Runs automatically on `init`; otherwise on-demand
- Full offline support after first sync — no network dependency for `inject`, `tip`, `doctor`, `resources`

**Binary self-update (`sap-devs update`):**

- Checks GitHub Releases for a newer binary
- Downloads, verifies checksum, replaces current binary
- Separate from content sync — binary and content version independently

---

## Extensibility Points

| Level | Mechanism |
|---|---|
| New AI tool | Add `content/adapters/<tool>.yaml` — PR to official repo |
| New knowledge pack | Add `content/packs/<domain>/` — PR to official repo |
| New developer profile | Add `content/profiles/<name>.yaml` — PR to official repo |
| Company customization | Point `sap-devs config company <git-url>` to internal repo with same structure |
| User customization | Add/override files in `$XDG_DATA_HOME/sap-devs/user/` |
| Per-project | Add `.sap-devs/overrides/` in project root |

---

## Dependencies

- **Go stdlib** — HTTP, filesystem, JSON/YAML, `os.UserConfigDir()`, `os.UserCacheDir()`
- **[cobra](https://github.com/spf13/cobra)** — CLI command structure
- **[viper](https://github.com/spf13/viper)** — configuration management; does not handle XDG natively — the `internal/xdg/` package computes XDG paths and passes them to viper explicitly
- **[go-semver](https://github.com/blang/semver)** — version comparison for `doctor`
- **[glamour](https://github.com/charmbracelet/glamour)** — render Markdown in terminal (tip of the day, context previews)
- **YAML parsing** — `gopkg.in/yaml.v3`

---

## Files Created / Modified (v1)

```
sap-devs-cli/
  main.go
  cmd/
    root.go · init.go · sync.go · inject.go · profile.go
    mcp.go · tip.go · resources.go · doctor.go · config.go · update.go
  internal/
    content/     — layer resolver, pack loader
    adapter/     — adapter engine (file-inject, clipboard-export, mcp-wire)
    profile/     — profile loader and weight resolver
    doctor/      — version detection runner
    sync/        — content repo fetcher
    update/      — self-update logic
    xdg/         — XDG path resolution (wraps os.UserConfigDir etc.)
  content/
    packs/       — cap/ abap/ btp-core/ fiori/ ai-joule/ mobile/ integration/ analytics/
    advocates/   — per-advocate YAML files
    profiles/    — developer persona definitions
    adapters/    — AI tool adapter definitions
  go.mod
  go.sum
  .gitignore    — includes .superpowers/ (brainstorm artifacts); .sap-devs/ is NOT gitignored (it is team-shareable)
```
