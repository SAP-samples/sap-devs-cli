# Documentation Design — sap-devs-cli

**Date:** 2026-04-14  
**Status:** Approved  
**Scope:** Three audience-specific documentation guides

---

## Problem

The project has no user-facing documentation. The README is a single title line. Without docs, functional testing cannot begin, and new contributors, content maintainers, and end users have no reference material.

## Goal

Create three standalone Markdown guides, one per audience, covering all aspects of the project needed for functional testing and ongoing use.

---

## Approach

**Approach A — Three standalone guides** (selected)

| File | Audience |
|---|---|
| `docs/developer/developer-guide.md` | Contributors building and releasing the CLI |
| `docs/content/content-guide.md` | Content maintainers adding/translating packs |
| `docs/user/user-guide.md` | End users installing and using the tool |

Each file is fully self-contained for its audience. The project README will be updated to serve as an index linking to each guide.

---

## Guide 1 — Developer Guide (`docs/developer/developer-guide.md`)

### Sections

1. **Prerequisites**  
   Go 1.26+, git, `libx11-dev` on Linux (required by `golang.design/x/clipboard`).

2. **Clone & build**  
   Module path `github.tools.sap/developer-relations/sap-devs-cli`. Build command with ldflags version injection from `git describe`. Output binary named `sap-devs`.

3. **Local development**  
   `SAP_DEVS_DEV=1` env var causes content to load from `./content/` instead of the user cache. Use `go run .` with this set for rapid iteration without rebuilding.

4. **Linting & static analysis**  
   `go build ./...` + `go vet ./...`. Explanation of why `go test` is blocked on Windows: Windows Defender quarantines test binaries executed from `~/.config` paths.

5. **Running tests**  
   `go test ./...` for all packages; single-package form for focused runs. CI runs on a self-hosted `Linux X64` runner (`runs-on: [self-hosted, Linux, X64]`) and is the authoritative test runner.

6. **Project layout**  
   Brief directory map: `cmd/` (cobra commands), `internal/` (adapter, config, content, i18n, sync, update, xdg packages), `content/` (packs, adapters, profiles), `.github/` (CI + release workflows), `.goreleaser.yml`.

7. **Architecture overview**  
   Full prose description of:
   - **Content Layer System** — 4 layers (official cache → company cache → user data → project), merged by ID with later layers winning. `ContentLoader` in `internal/content/loader.go`. Each pack directory structure.
   - **Adapter System** — `content/adapters/` YAML files. Three types: `file-inject` (writes fenced section into config file), `clipboard-export` (copies to clipboard), `mcp-wire` (registers MCP servers). `Engine` in `internal/adapter/engine.go` filters by tool and scope.
   - **Profiles** — `content/profiles/` YAML files tag packs to developer personas. `ApplyWeights()` reorders packs. Active profile stored in `~/.config/sap-devs/profile.yaml`.
   - **Sync** — fetches official repo as `.zip`, extracts to cache. Per-category TTLs in `~/.cache/sap-devs/sync-state.json`. `sync.Engine` in `internal/sync/engine.go`.
   - **i18n** — `internal/i18n` package. Language resolved from config → `LANG`/`LC_ALL` env → `en`. CLI strings in `internal/i18n/catalogs/<lang>.json`. Pack content localised via `context.<lang>.md`, `tips.<lang>.md`, `pack.yaml` locales block.
   - **Update Check** — background goroutine on every command invocation (except `update` and dev builds). Checks GitHub at most once per 168h. Prints to stderr after command completes.
   - **Platform Paths** — `internal/xdg` resolves config/cache/data directories per OS.

8. **Adding a command**  
   Cobra command pattern, file location in `cmd/`, i18n key naming convention (`cmd.subcommand.short`, `cmd.subcommand.long`, etc.), wiring into root or parent command in `cmd/root.go`.

9. **Release workflow** (full walkthrough)  
   - Pre-release checklist: CI green on `main`, all tests passing, changelog entries present.
   - Tag and push: `git tag v1.2.3 && git push origin v1.2.3`.
   - GitHub Actions `release.yml` triggers on `v*` tags, runs GoReleaser on `ubuntu-latest`.
   - GoReleaser reads `.goreleaser.yml`:
     - Builds: Linux amd64/arm64 (`.tar.gz`), macOS amd64/arm64 (`.tar.gz`), Windows amd64 (`.zip`). Windows arm64 excluded.
     - Version injected via `-ldflags "-X github.tools.sap/developer-relations/sap-devs-cli/cmd.Version={{ .Version }}"` (full symbol path required).
     - Archive naming: `sap-devs_<version>_<os>_<arch>.<ext>`.
     - `checksums.txt` (SHA256) included in release assets.
   - Verify artifacts appear on the GitHub Releases page.
   - Post-release: update any documentation version references if needed.

10. **Worktrees**  
    Feature branch worktrees stored in `.worktrees/` in project root. Not `~/.config` — Windows Defender blocks test binary execution from that path.

---

## Guide 2 — Content Guide (`docs/content/content-guide.md`)

### Sections

1. **Overview**  
   What content is and how the 4-layer merge works. Where each layer lives on disk per platform (official cache, company cache, user data dir, project `.sap-devs/`).

2. **Pack structure**  
   Directory: `content/packs/<pack-id>/`. Full schema for each file:

   - **`pack.yaml`** — `id` (unique slug), `name`, `description`, `tags` (list), `profiles` (list of profile IDs that include this pack), `weight` (integer default priority; profiles can override per-pack in their `packs` list), `locales` block mapping language tag to translated `name`/`description`.
   - **`context.md`** — Free-form Markdown injected as AI context into tools. No special syntax. May be long-form reference material.
   - **`context.<lang>.md`** — Localised version of `context.md` for language `<lang>` (e.g. `context.de.md`). Falls back to `context.md` if absent.
   - **`tips.md`** — H2-delimited tips. Each tip starts with `## <Title>`, followed by optional `Tags: tag1,tag2` line, then tip body.
   - **`tips.<lang>.md`** — Localised version of `tips.md`.
   - **`tools.yaml`** — List of tool requirement entries. Each entry: `id`, `name`, `required` (semver constraint), `detect` (`command` + `pattern` regex), `install` (per-platform or `all` key), `docs` (URL).
   - **`resources.yaml`** — List of resource entries. Each entry: `id` (`<pack>/<slug>`), `title`, `url`, `type` (`official-docs`, `sample`, `community`, `tutorial`, etc.), `tags`.
   - **`mcp.yaml`** — MCP server definitions (covered in MCP section).

3. **Adapters**  
   File: `content/adapters/<tool-id>.yaml`. Schema: `id`, `name`, `type`, `targets` list (each with `scope`, `path`, `mode`, `section`), `detect` list (command or path checks), `mcp_config` (path, format, key). Modes: `replace-section` replaces the named fenced section; `append` adds content if not present. Scope: `global` (user-level config file) or `project` (project-level file in CWD).

4. **Profiles**  
   File: `content/profiles/<profile-id>.yaml`. Schema: `id`, `name`, `description`, `packs` list (each with `id` and `weight`), `tip_tags`. `ApplyWeights()` reorders packs so higher-weight packs appear first in rendered context.

5. **Creating a new pack**  
   Step-by-step:
   1. Create `content/packs/<new-id>/`.
   2. Write `pack.yaml` with a unique `id`.
   3. Add `context.md`, `tips.md`, `tools.yaml`, `resources.yaml` (all optional; omit files with no content).
   4. Reference the pack in at least one profile's `packs` list.
   5. Test: `SAP_DEVS_DEV=1 sap-devs inject --dry-run` — verify the new context appears in output.

6. **Updating existing content**  
   Edit files in the relevant layer. Official content can be overridden at user (`~/.local/share/sap-devs/` on Linux) or project (`.sap-devs/`) layer using the same `id` — the later layer wins.

7. **Translation guide** (full)  
   - **Language resolution:** `config language` setting → `LANG` env var → `LC_ALL` env var → fallback `en`. Region/encoding suffixes stripped (`de_AT.UTF-8` → `de`). Unknown tags fall back to `en`.
   - **Pack content files:** Add `context.<lang>.md` and/or `tips.<lang>.md` alongside the base files. The loader picks the locale-specific file when the active language matches.
   - **Pack metadata:** Add a `locales` block in `pack.yaml`:
     ```yaml
     locales:
       de:
         name: Translated name
         description: Translated description
     ```
   - **CLI string catalog:** Add `internal/i18n/catalogs/<lang>.json`. Keys follow the pattern `cmd.subcommand.string_name` (e.g. `inject.done`). Values may be plain strings or Go `text/template` expressions. Use `T(lang, key)` for plain strings; `Tf(lang, key, data)` for template strings (e.g. `"sync.updated": "Updated: {{.Categories}}"`). Both require `lang` as the first argument — typically `i18n.ActiveLang`.
   - **Adding a new language end-to-end:**
     1. Create `internal/i18n/catalogs/<lang>.json` with translated keys (copy `en.json` as starting point; any missing keys fall back to English).
     2. Add `context.<lang>.md` and `tips.<lang>.md` to each pack you want to translate.
     3. Add `locales.<lang>` blocks to `pack.yaml` for each pack.
     4. Test: `sap-devs config set language <lang> && sap-devs inject --dry-run`.

---

## Guide 3 — User Guide (`docs/user/user-guide.md`)

### Sections

1. **What is sap-devs?**  
   One paragraph: injects up-to-date SAP developer knowledge into AI coding tools (Claude Code, Cursor, Copilot, etc.), wires SAP MCP servers, keeps content current via sync.

2. **Installation**  
   - Go to GitHub Releases, find the latest release.
   - Download the archive for your platform:
     - Windows: `sap-devs_<version>_windows_amd64.zip`
     - macOS Intel: `sap-devs_<version>_darwin_amd64.tar.gz`
     - macOS Apple Silicon: `sap-devs_<version>_darwin_arm64.tar.gz`
     - Linux amd64: `sap-devs_<version>_linux_amd64.tar.gz`
     - Linux arm64: `sap-devs_<version>_linux_arm64.tar.gz`
   - Verify checksum against `checksums.txt`.
   - Extract and place `sap-devs` binary in PATH:
     - Windows: extract ZIP, add folder to `PATH` via System Properties or copy to a folder already on PATH.
     - macOS/Linux: `tar -xzf ... && mv sap-devs /usr/local/bin/` (or `~/.local/bin/`).
   - Verify: `sap-devs --version`.

3. **First-time setup**  
   Run `sap-devs init` — interactive wizard that sets your developer profile, runs an initial sync, and injects context into detected AI tools.

4. **Core workflow**  
   - `sap-devs sync` — pull latest SAP developer content from the official repo.
   - `sap-devs inject` — push context into all detected AI tools at global (user) scope. Add `--project` for project scope (writes to `CLAUDE.md`, `.cursorrules`, etc. in current directory). Add `--dry-run` to preview without writing.
   - `sap-devs profile list` / `sap-devs profile set <id>` — list available personas, set the active one.

5. **Command reference**  
   One subsection per command with synopsis, flags, and a short example:
   - `inject` — `--project`, `--tool <id>`, `--dry-run`
   - `sync` — `--force`
   - `profile list` / `profile set <id>` / `profile show`
   - `config show` / `config set <key> <value>` / `config company <url>`
   - `tip` — prints a random tip from active profile packs
   - `doctor` — `--fix` (shows install commands for missing tools)
   - `mcp list` / `mcp status` / `mcp install <id>`
   - `resources list` / `resources search <query>` / `resources open <id>`
   - `update` — self-update the binary
   - `init` — first-time setup wizard

6. **Configuration**  
   Config file: `~/.config/sap-devs/config.yaml` (Linux), `%APPDATA%/sap-devs/config.yaml` (Windows), `~/Library/Application Support/sap-devs/config.yaml` (macOS). Key settings: `language` (language tag, e.g. `de`), `company_repo` (HTTPS URL to company content repo), `sync.disabled` (bool). Use `sap-devs config show` to view, `sap-devs config set <key> <value>` to change.

7. **MCP servers**  
   Brief explanation of what MCP servers are and why they matter. `mcp list` to browse available SAP MCP servers. `mcp status` to see which are already registered in your AI tool configs. `mcp install <id>` to wire a server into all detected tools. Supported AI tools listed (Claude Code, Cursor, etc.).

8. **Keeping up to date**  
   The CLI checks GitHub for a new release in the background on every command (at most once per 7 days). Notification printed to stderr after the command completes. Run `sap-devs update` to update immediately.

9. **Troubleshooting**  
   Common issues:
   - "No tips available" / empty context → run `sap-devs sync`.
   - AI tool not detected → ensure the tool is installed and on PATH; check with `sap-devs doctor`.
   - Windows PATH not updating → open a new terminal after modifying PATH.
   - `doctor` FAIL/MISSING entries → use `--fix` flag to see install commands.

10. **Platform notes**  
    Config, cache, and data directory paths per OS:
    - Linux: `~/.config/sap-devs`, `~/.cache/sap-devs`, `~/.local/share/sap-devs` (XDG env vars honoured)
    - macOS: `~/Library/Application Support/sap-devs`, `~/Library/Caches/sap-devs`, `~/Library/Application Support/sap-devs/data`
    - Windows: `%APPDATA%/sap-devs`, `%LOCALAPPDATA%/sap-devs/cache`, `%LOCALAPPDATA%/sap-devs/data`

---

## Files to Create

| File | New/Modified |
|---|---|
| `docs/developer/developer-guide.md` | New |
| `docs/content/content-guide.md` | New |
| `docs/user/user-guide.md` | New |
| `README.md` | Modified — replace stub with index linking to all three guides |

## Files Not Changed

All source code, `CLAUDE.md`, and `docs/superpowers/` planning docs are out of scope.
