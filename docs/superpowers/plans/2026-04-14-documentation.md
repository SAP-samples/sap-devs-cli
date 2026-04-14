# Documentation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create three standalone documentation guides (developer, content maintainer, end user) and update the README to index them.

**Architecture:** Four Markdown files — `docs/developer/developer-guide.md`, `docs/content/content-guide.md`, `docs/user/user-guide.md`, and the updated `README.md`. Each guide is fully self-contained for its audience. No source code changes.

**Tech Stack:** Markdown, verified against Go source in `cmd/`, `internal/`, `content/`, `.goreleaser.yml`, `.github/workflows/`

**Spec:** `docs/superpowers/specs/2026-04-14-documentation-design.md`

---

## File Map

| File | Action | Purpose |
|---|---|---|
| `docs/developer/developer-guide.md` | Create | Developer reference: build, test, architecture, release |
| `docs/content/content-guide.md` | Create | Content maintainer reference: pack/adapter/profile schemas, translation |
| `docs/user/user-guide.md` | Create | End user reference: install, commands, config, troubleshooting |
| `README.md` | Modify | Replace stub with index linking to all three guides |

---

## Task 1: Developer Guide

**Files:**
- Create: `docs/developer/developer-guide.md`

- [ ] **Step 1: Create the parent directory**

```bash
mkdir -p docs/developer
```

- [ ] **Step 2: Create the file**

Create `docs/developer/developer-guide.md` with the following content:

````markdown
# sap-devs Developer Guide

This guide covers everything you need to build, test, and release the `sap-devs` CLI.

---

## Prerequisites

- **Go 1.26+** — [download](https://go.dev/dl/)
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
SAP_DEVS_DEV=1 go run . inject --dry-run
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

CI runs on a self-hosted `Linux X64` runner and is the authoritative test runner. Tests that pass locally will pass in CI; the reverse is not always true on Windows.

---

## Project Layout

```
sap-devs-cli/
├── cmd/                    # Cobra command definitions (one file per command)
├── internal/
│   ├── adapter/            # Adapter engine — pushes context into AI tools
│   ├── config/             # Config file read/write
│   ├── content/            # Content loader — merges 4 content layers
│   ├── i18n/               # Internationalisation: language resolution, T(), Tf()
│   │   └── catalogs/       # JSON string catalogs per language (en.json, de.json, …)
│   ├── sync/               # Sync engine — fetches official/company repo zips
│   ├── update/             # Self-update logic
│   └── xdg/                # Platform-native config/cache/data paths
├── content/
│   ├── adapters/           # Adapter definitions (one YAML per AI tool)
│   ├── packs/              # Content packs (one directory per pack)
│   └── profiles/           # Developer persona profiles
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

`ContentLoader` (`internal/content/loader.go`) manages the merge. `LoadPacks()` reads all `content/packs/<name>/` directories.

### Adapter System

Adapters (`content/adapters/<tool>.yaml`) define how to push context into a specific AI tool. Three types:

- **`file-inject`** — writes a fenced section into a config file (e.g. `~/.claude/CLAUDE.md`). Modes: `replace-section` (replaces a named fenced block) or `append` (adds if not present).
- **`clipboard-export`** — copies context to clipboard (global scope only).
- **`mcp-wire`** — registers MCP servers in the tool's JSON config (used by `mcp install`, not `inject`).

The `Engine` (`internal/adapter/engine.go`) iterates adapters, filters by `--tool` flag and scope (`global`/`project`), and dispatches to the appropriate handler.

### Profiles

Profiles (`content/profiles/`) are YAML files that tag which packs belong to a developer persona (e.g. `cap-developer`). `ApplyWeights()` reorders packs to prioritise those matching the active profile. The active profile is stored in `~/.config/sap-devs/profile.yaml`.

### Sync

`sap-devs sync` (`cmd/sync.go`) fetches the official repo as a `.zip` archive and extracts it to the cache. Per-category TTLs are tracked in `~/.cache/sap-devs/sync-state.json` via `sync.Engine` (`internal/sync/engine.go`). Use `--force` to ignore TTLs.

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
````

- [ ] **Step 2: Verify accuracy**

Spot-check these facts against source:

```bash
# Confirm Go version
head -5 go.mod

# Confirm inject flags
grep "Flags()" cmd/inject.go

# Confirm ldflags symbol path
grep "ldflags" .goreleaser.yml

# Confirm CI runner
grep "runs-on" .github/workflows/ci.yml

# Confirm xdg paths
grep -A3 "configDir\|cacheDir\|dataDir" internal/xdg/xdg.go | head -20
```

Expected: Go 1.26+, flags `--project`/`--tool`/`--dry-run`, full symbol path, `self-hosted Linux X64` runner.

- [ ] **Step 3: Commit**

```bash
git add docs/developer/developer-guide.md
git commit -m "docs: add developer guide"
```

---

## Task 2: Content Guide

**Files:**
- Create: `docs/content/content-guide.md`

- [ ] **Step 1: Create the parent directory**

```bash
mkdir -p docs/content
```

- [ ] **Step 2: Create the file**

Create `docs/content/content-guide.md` with the following content:

````markdown
# sap-devs Content Guide

This guide covers how to add, update, and translate content for the `sap-devs` CLI — packs, adapters, and profiles.

---

## Overview: The Content Layer System

Content is loaded from up to four sources, merged by `id` with later layers overriding earlier ones:

| Layer | Location | Purpose |
|---|---|---|
| Official | `~/.cache/sap-devs/official/` (Linux/macOS) or `%LOCALAPPDATA%/sap-devs/cache/official/` (Windows) | Fetched from the official repo via `sap-devs sync` |
| Company | `~/.cache/sap-devs/company/` | Optional; configured via `sap-devs config company <url>` |
| User | `~/.local/share/sap-devs/` (Linux), `~/Library/Application Support/sap-devs/data/` (macOS), `%LOCALAPPDATA%/sap-devs/data/` (Windows) | Per-user overrides |
| Project | `.sap-devs/` in the current working directory | Per-project overrides |

If two layers define a pack with the same `id`, the later layer wins completely.

---

## Pack Structure

Each pack lives in `content/packs/<pack-id>/`. All files are optional except `pack.yaml`.

```
content/packs/cap/
├── pack.yaml          # Pack metadata
├── context.md         # AI context text (English)
├── context.de.md      # German AI context text (optional)
├── tips.md            # Tips (English)
├── tips.de.md         # German tips (optional)
├── tools.yaml         # Tool version requirements
├── resources.yaml     # Curated resource links
└── mcp.yaml           # MCP server definitions
```

### `pack.yaml`

```yaml
id: cap                          # unique slug — used as the merge key
name: SAP Cloud Application Programming Model
description: Node.js and Java framework for building cloud-native business applications on BTP
tags: [cloud, btp, nodejs, java, odata, cds]
profiles: [cap-developer, btp-developer]   # profiles that include this pack
weight: 100                      # default weight; profiles can override this per-pack
locales:
  de:
    name: SAP Cloud Application Programming Model
    description: Node.js- und Java-Framework für Cloud-native Business-Anwendungen auf BTP
```

> **Note:** `weight` sets the default priority for this pack. Individual profiles can override it in their `packs` list (see [Profiles](#profiles) below).

### `context.md`

Free-form Markdown injected as AI context into tools. No special syntax. May be long-form reference material, code examples, or conventions.

### `context.<lang>.md`

Localised version of `context.md` for language `<lang>` (e.g. `context.de.md`). Falls back to `context.md` if absent for the active language.

### `tips.md`

H2-delimited tips. Each tip:

```markdown
## Use cds watch for local development
Tags: cap,nodejs
Run `cds watch` instead of `node server.js` — it reloads on every file change.

## Define managed entities for audit fields
Tags: cap,cds
Add `: managed` to your entities to get createdAt, createdBy, modifiedAt, modifiedBy for free.
```

- `## <Title>` — required
- `Tags: tag1,tag2` — optional, one line immediately after the heading
- Body — tip content, may be multi-line Markdown

### `tips.<lang>.md`

Localised version of `tips.md`. Same format.

### `tools.yaml`

```yaml
- id: nodejs
  name: Node.js
  required: ">=18.0.0"          # semver constraint
  detect:
    command: "node --version"
    pattern: "v(\\d+\\.\\d+\\.\\d+)"   # regex capturing the version string
  install:
    windows: "winget install OpenJS.NodeJS.LTS"
    macos: "brew install node@20"
    linux: "nvm install 20"
    # use "all" key for cross-platform: all: "npm install -g @sap/cds-dk"
  docs: "https://nodejs.org"
```

### `resources.yaml`

```yaml
- id: cap/docs-official           # <pack-id>/<slug>
  title: CAP Documentation
  url: https://cap.cloud.sap/docs
  type: official-docs             # official-docs | sample | community | tutorial | blog
  tags: [reference, getting-started]
```

### `mcp.yaml`

MCP server definitions for the pack. See the MCP documentation for the full schema.

---

## Adapters

Adapters define how to push content into a specific AI tool. Files live in `content/adapters/<tool-id>.yaml`.

```yaml
id: claude-code
name: Claude Code
type: file-inject                 # file-inject | clipboard-export | mcp-wire
targets:
  - scope: global                 # global (user-level) or project (current dir)
    path: "~/.claude/CLAUDE.md"
    mode: replace-section         # replace-section | append
    section: "SAP Developer Context"
  - scope: project
    path: "./CLAUDE.md"
    mode: replace-section
    section: "SAP Developer Context"
detect:
  - command: "claude --version"   # run this; if it exits 0, tool is present
  - path: "~/.claude"             # or check if this path exists
mcp_config:
  path: "~/.claude/settings.json"
  format: json
  key: "mcpServers"
```

**Modes:**
- `replace-section` — replaces the named fenced section. The section is identified by an HTML comment fence: `<!-- SAP Developer Context -->`.
- `append` — appends content if the section is not present.

**Types:**
- `file-inject` — writes context to a config file. Used by `sap-devs inject`.
- `clipboard-export` — copies context to clipboard (global scope only).
- `mcp-wire` — registers MCP servers in the tool's JSON config. Used by `sap-devs mcp install`, not `inject`.

---

## Profiles

Profiles tag which packs belong to a developer persona. Files live in `content/profiles/<profile-id>.yaml`.

```yaml
id: cap-developer
name: CAP Developer
description: Building cloud-native apps with SAP CAP on BTP
packs:
  - id: cap
    weight: 100       # higher weight = appears earlier in rendered context
  - id: btp-core
    weight: 80
  - id: abap
    weight: 10
tip_tags: [cap, nodejs, odata, cds, btp]   # tips with these tags are preferred for this profile
```

`ApplyWeights()` reorders packs so higher-weight packs appear first in the rendered context injected into AI tools.

---

## Creating a New Pack

1. Create the directory:
   ```bash
   mkdir content/packs/<new-id>
   ```

2. Write `pack.yaml` with a unique `id`:
   ```yaml
   id: my-pack
   name: My Pack
   description: What this pack covers
   tags: [tag1, tag2]
   profiles: [cap-developer]   # or whichever profile(s) should include it
   ```

3. Add content files as needed (`context.md`, `tips.md`, `tools.yaml`, `resources.yaml`). All are optional.

4. Reference the pack in at least one profile:
   ```yaml
   # content/profiles/cap-developer.yaml
   packs:
     - id: my-pack
       weight: 50
   ```

5. Test with dev mode:
   ```bash
   SAP_DEVS_DEV=1 sap-devs inject --dry-run
   ```
   Verify the new context appears in the output.

---

## Updating Existing Content

Edit the file in the relevant layer. The official layer is under `content/packs/` in this repo. To override official content without modifying it:

- **User override:** Copy the pack to `~/.local/share/sap-devs/packs/<id>/` (Linux) and edit there.
- **Project override:** Copy to `.sap-devs/packs/<id>/` in the project directory.

The override pack must use the same `id` as the official pack. The later layer wins.

---

## Translation Guide

### How Language Resolution Works

The active language is resolved in this order:

1. `sap-devs config set language <tag>` — explicit config setting
2. `LANG` environment variable (e.g. `de_AT.UTF-8` → `de`)
3. `LC_ALL` environment variable
4. Fallback: `en`

Region and encoding suffixes are stripped: `de_AT.UTF-8` → `de`. If the resolved tag has no catalog, it falls back to `en`.

### Translating Pack Content Files

Add localised content files alongside the base files:

```
content/packs/cap/
├── context.md         # base (English)
├── context.de.md      # German translation
├── tips.md            # base (English)
└── tips.de.md         # German translation
```

The loader automatically picks the locale-specific file when the active language matches. Falls back to the base file if the locale file is absent.

### Translating Pack Metadata

Add a `locales` block to `pack.yaml`:

```yaml
locales:
  de:
    name: SAP Cloud Application Programming Model
    description: Node.js- und Java-Framework für Cloud-native Business-Anwendungen auf BTP
  fr:
    name: SAP Cloud Application Programming Model
    description: Framework Node.js et Java pour applications cloud natives sur BTP
```

### Translating CLI Strings

CLI strings live in `internal/i18n/catalogs/<lang>.json`. Copy `en.json` as a starting point — any missing keys fall back to English automatically.

```bash
cp internal/i18n/catalogs/en.json internal/i18n/catalogs/fr.json
# Edit fr.json with French translations
```

**Key naming convention:** `<command>.<subcommand>.<string_name>`

```json
{
  "inject.short": "Pousser le contexte SAP dans vos outils IA",
  "inject.done": "Contexte SAP injecté (portée {{.Scope}})."
}
```

Values may be plain strings or Go `text/template` expressions (e.g. `{{.Scope}}`).

**In Go code**, strings are looked up with:
- `i18n.T(lang, key)` — plain string
- `i18n.Tf(lang, key, data)` — template string, where `data` is `map[string]any`
- Pass `i18n.ActiveLang` as the `lang` argument

### Adding a New Language End-to-End

1. **Create the CLI catalog:**
   ```bash
   cp internal/i18n/catalogs/en.json internal/i18n/catalogs/<lang>.json
   ```
   Translate values. Leave any untranslated keys as-is (they fall back to English).

2. **Add localised content files** to each pack you want to translate:
   ```bash
   # For each pack:
   cp content/packs/<id>/context.md content/packs/<id>/context.<lang>.md
   cp content/packs/<id>/tips.md content/packs/<id>/tips.<lang>.md
   # Edit the new files with translations
   ```

3. **Add `locales` blocks** to each `pack.yaml` you translated.

4. **Test:**
   ```bash
   sap-devs config set language <lang>
   SAP_DEVS_DEV=1 sap-devs inject --dry-run
   sap-devs tip
   ```
   Verify translated strings appear in output.

5. **Reset language** when done testing:
   ```bash
   sap-devs config set language en
   ```
````

- [ ] **Step 2: Verify accuracy**

Spot-check these facts against source:

```bash
# Confirm pack.yaml has no weight field
cat content/packs/cap/pack.yaml

# Confirm weight is in profile packs list
cat content/profiles/cap-developer.yaml

# Confirm tips.md format
head -10 content/packs/cap/tips.md

# Confirm T/Tf signatures
grep "^func T\|^func Tf" internal/i18n/i18n.go
```

Expected: `weight: 100` present in pack.yaml (default weight); profile packs list also has `weight` (per-profile override); tips use `## Title` + `Tags:` line; `T(lang, key string)` and `Tf(lang, key string, data map[string]any)`.

- [ ] **Step 3: Commit**

```bash
git add docs/content/content-guide.md
git commit -m "docs: add content maintainer guide"
```

---

## Task 3: User Guide

**Files:**
- Create: `docs/user/user-guide.md`

- [ ] **Step 1: Create the parent directory**

```bash
mkdir -p docs/user
```

- [ ] **Step 2: Create the file**

Create `docs/user/user-guide.md` with the following content:

````markdown
# sap-devs User Guide

`sap-devs` injects up-to-date SAP developer knowledge into your AI coding tools (Claude Code, Cursor, GitHub Copilot, and more), wires SAP MCP servers, and keeps content current automatically.

---

## Installation

### Download

Go to the [GitHub Releases page](https://github.tools.sap/developer-relations/sap-devs-cli/releases) and download the archive for your platform:

| Platform | Architecture | File |
|---|---|---|
| Windows | x64 | `sap-devs_<version>_windows_amd64.zip` |
| macOS | Intel | `sap-devs_<version>_darwin_amd64.tar.gz` |
| macOS | Apple Silicon | `sap-devs_<version>_darwin_arm64.tar.gz` |
| Linux | x64 | `sap-devs_<version>_linux_amd64.tar.gz` |
| Linux | ARM64 | `sap-devs_<version>_linux_arm64.tar.gz` |

### Verify checksum (recommended)

Download `checksums.txt` from the same release and verify:

```bash
# macOS / Linux
sha256sum --check checksums.txt

# Windows (PowerShell)
Get-FileHash sap-devs_<version>_windows_amd64.zip -Algorithm SHA256
# Compare output against checksums.txt
```

### Install

**macOS / Linux:**
```bash
tar -xzf sap-devs_<version>_<os>_<arch>.tar.gz
sudo mv sap-devs /usr/local/bin/
# or without sudo:
mkdir -p ~/.local/bin && mv sap-devs ~/.local/bin/
```

If using `~/.local/bin/`, ensure it is on your `PATH`:
```bash
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc
```

**Windows:**
1. Extract the ZIP file.
2. Move `sap-devs.exe` to a folder on your `PATH`, or add its folder to `PATH`:
   - Open **System Properties** → **Environment Variables**
   - Under **User variables**, edit `Path` and add the folder containing `sap-devs.exe`
   - Open a new terminal for the change to take effect

### Verify

```bash
sap-devs --version
```

---

## First-Time Setup

Run the setup wizard:

```bash
sap-devs init
```

The wizard will:
1. Ask you to select a developer profile (e.g. `cap-developer`, `btp-developer`, `abap-developer`)
2. Run an initial content sync
3. Inject SAP context into all detected AI tools

---

## Core Workflow

### Keep content current

```bash
sap-devs sync
```

Fetches the latest SAP developer content from the official repo. Run this periodically or after major SAP releases.

### Inject context into AI tools

```bash
# Inject into all detected tools at user (global) scope
sap-devs inject

# Inject into the current project only
sap-devs inject --project

# Preview what would be written without making changes
sap-devs inject --dry-run
```

### Choose your developer profile

```bash
sap-devs profile list           # see available profiles
sap-devs profile set cap-developer  # set the active profile
sap-devs profile show           # show active profile and pack weights
```

---

## Command Reference

### `inject`

Push SAP context into all detected AI tools.

```
sap-devs inject [flags]
```

| Flag | Description |
|---|---|
| `--project` | Inject at project scope (writes to project config files in the current directory) |
| `--tool <id>` | Inject into a specific tool only (e.g. `claude-code`, `cursor`) |
| `--dry-run` | Preview changes without writing files |

**Example:**
```bash
sap-devs inject --tool claude-code --dry-run
```

---

### `sync`

Pull the latest SAP developer content from the official repo.

```
sap-devs sync [flags]
```

| Flag | Description |
|---|---|
| `--force` | Re-sync all content regardless of TTL |
| `--category` | Sync a single category only |

---

### `profile`

Manage your developer profile.

```
sap-devs profile list
sap-devs profile set <profile-id>
sap-devs profile show
```

| Subcommand | Description |
|---|---|
| `list` | List all available profiles |
| `set <id>` | Set the active profile |
| `show` | Show the active profile and pack weights |

**Example:**
```bash
sap-devs profile set btp-developer
```

---

### `config`

View and edit `sap-devs` configuration.

```
sap-devs config show
sap-devs config set <key> <value>
sap-devs config company <git-url>
```

| Subcommand | Description |
|---|---|
| `show` | Display the current configuration |
| `set <key> <value>` | Set a configuration value |
| `company <url>` | Configure the company content repo URL (HTTPS) |

**Common config keys:**

| Key | Description | Example |
|---|---|---|
| `language` | Language tag for CLI output and content | `de`, `en` |

---

### `tip`

Print a random SAP developer tip from your active profile's packs. Add to your shell profile for a tip on every new terminal:

```bash
# ~/.bashrc or ~/.zshrc
sap-devs tip
```

---

### `doctor`

Check that the tools required by your active profile are installed and meet version requirements.

```
sap-devs doctor [flags]
```

| Flag | Description |
|---|---|
| `--fix` | Print install commands for failed or missing tools |
| `--profile <id>` | Check a specific profile (`@active` for the configured profile) |

**Example:**
```bash
sap-devs doctor --fix
```

Output:
```
TOOL       REQUIRED    FOUND      STATUS
Node.js    >=18.0.0    20.11.0    ok
cds-dk     >=7.0.0     -          MISSING

Install commands:
  cds-dk: npm install -g @sap/cds-dk
```

---

### `mcp`

Manage SAP MCP (Model Context Protocol) servers. MCP servers give AI tools direct access to SAP APIs and documentation.

```
sap-devs mcp list [--all]
sap-devs mcp status
sap-devs mcp install [id] [--all] [--dry-run]
```

| Subcommand | Description |
|---|---|
| `list` | List available SAP MCP servers (active profile by default; `--all` for all) |
| `status` | Show which SAP MCP servers are registered in your AI tool configs |
| `install [id]` | Wire an SAP MCP server into your AI tools; `--all` installs all for active profile |

**Example:**
```bash
sap-devs mcp list
sap-devs mcp install cap-mcp-server
```

---

### `resources`

Browse curated SAP developer resources from your active profile's packs.

```
sap-devs resources list
sap-devs resources search <query>
sap-devs resources open <id>
```

| Subcommand | Description |
|---|---|
| `list` | List all resources for the active profile |
| `search <query>` | Search resources by keyword |
| `open <id>` | Open a resource URL in the default browser |

---

### `update`

Update `sap-devs` to the latest release.

```bash
sap-devs update
```

Checks GitHub for a newer release and installs it if found.

---

### `init`

First-time setup wizard. Run once after installation.

```bash
sap-devs init
```

---

## Configuration

The configuration file is at:

| OS | Path |
|---|---|
| Linux | `~/.config/sap-devs/config.yaml` |
| macOS | `~/Library/Application Support/sap-devs/config.yaml` |
| Windows | `%APPDATA%/sap-devs/config.yaml` |

View with `sap-devs config show`. Edit with `sap-devs config set <key> <value>`.

---

## MCP Servers

MCP (Model Context Protocol) servers extend AI tools with direct access to external APIs and data. `sap-devs` can configure SAP MCP servers in your AI tool settings automatically.

```bash
sap-devs mcp list          # see what's available
sap-devs mcp status        # see what's already configured
sap-devs mcp install <id>  # wire a server into your AI tools
```

Supported AI tools include Claude Code, Cursor, and others detected on your system.

---

## Keeping Up to Date

On every command invocation, `sap-devs` checks GitHub for a newer release in the background (at most once per 7 days). If a new version is available, you'll see a notification in the terminal after the command completes.

To update immediately:

```bash
sap-devs update
```

---

## Troubleshooting

**"No tips available" or context appears empty**
→ Run `sap-devs sync` to download the latest content.

**AI tool not detected by `inject`**
→ Ensure the tool is installed and its CLI is on your `PATH`. Check with `sap-devs doctor`.

**`doctor` shows FAIL or MISSING**
→ Run `sap-devs doctor --fix` to see install commands for the missing tools.

**Windows: `sap-devs` not found after installation**
→ Open a new terminal after adding the folder to `PATH`. Environment variable changes require a new shell session.

**Inject writes the wrong language**
→ Set your language: `sap-devs config set language en` (or `de`, etc.).

---

## Platform Notes

Config, cache, and data directories per OS:

| Purpose | Linux | macOS | Windows |
|---|---|---|---|
| Config | `~/.config/sap-devs` | `~/Library/Application Support/sap-devs` | `%APPDATA%/sap-devs` |
| Cache | `~/.cache/sap-devs` | `~/Library/Caches/sap-devs` | `%LOCALAPPDATA%/sap-devs/cache` |
| Data (user content) | `~/.local/share/sap-devs` | `~/Library/Application Support/sap-devs/data` | `%LOCALAPPDATA%/sap-devs/data` |

On Linux, XDG environment variables (`XDG_CONFIG_HOME`, `XDG_CACHE_HOME`, `XDG_DATA_HOME`) are honoured.
````

- [ ] **Step 2: Verify accuracy**

Spot-check these facts against source:

```bash
# Confirm all command Use: strings match what's in the guide
grep "Use:" cmd/inject.go cmd/sync.go cmd/doctor.go cmd/mcp.go cmd/profile.go cmd/config.go cmd/resources.go cmd/tip.go cmd/update.go cmd/init.go

# Confirm doctor flags
grep "Flags()" cmd/doctor.go

# Confirm mcp install flags
grep "Flags()" cmd/mcp.go

# Confirm sync --category flag exists
grep "category" cmd/sync.go
```

Expected: `Use:` strings match guide synopsis, doctor has `--fix` and `--profile`, mcp install has `--all` and `--dry-run`.

- [ ] **Step 3: Commit**

```bash
git add docs/user/user-guide.md
git commit -m "docs: add user guide"
```

---

## Task 4: Update README

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Replace README content**

Replace the entire contents of `README.md` with:

```markdown
# sap-devs

`sap-devs` injects up-to-date SAP developer knowledge into your AI coding tools, wires SAP MCP servers, and keeps content current automatically.

## Documentation

- [User Guide](docs/user/user-guide.md) — Install, configure, and use `sap-devs`
- [Content Guide](docs/content/content-guide.md) — Add, update, and translate packs
- [Developer Guide](docs/developer/developer-guide.md) — Build, test, and release the CLI

## Quick Start

```bash
# Install: download from GitHub Releases, extract, add to PATH
sap-devs --version

# First-time setup
sap-devs init

# Keep content current
sap-devs sync

# Inject SAP context into your AI tools
sap-devs inject
```

## License

See [LICENSE](LICENSE).
```

- [ ] **Step 2: Verify links resolve**

```bash
test -f docs/user/user-guide.md && echo "user-guide OK" || echo "MISSING"
test -f docs/content/content-guide.md && echo "content-guide OK" || echo "MISSING"
test -f docs/developer/developer-guide.md && echo "developer-guide OK" || echo "MISSING"
```

Expected: all three print `OK`.

- [ ] **Step 3: Commit**

```bash
git add README.md
git commit -m "docs: update README with index and quick start"
```
