# Backlog

Ideas and planned features for `sap-devs`. These are not commitments or a roadmap — they are a shared record of what we want to explore next. Contributions welcome: open an issue or PR to discuss any item.

---

## Release & Distribution

### ~~Migrate to github.com/SAP-samples~~ (Done)

Repository migrated from `github.tools.sap/developer-relations/sap-devs-cli` to `github.com/SAP-samples/sap-devs-cli`. Full git history preserved. All import paths, runtime URLs, CI workflows, and documentation updated.

---

### Package manager publishing

Distribute via package managers so users never see a Windows SmartScreen warning on install and updates are handled automatically. GoReleaser has first-class support for Scoop and Homebrew — each can be added as a section in `.goreleaser.yml` and published to a companion "bucket/tap" repo on release.

**Priority order:**

| Manager | Platform | Notes |
| --- | --- | --- |
| **Scoop** | Windows | Best fit for developer CLIs; add `scoop:` section to `.goreleaser.yml`; publish manifest to a `scoop-bucket` companion repo |
| **Homebrew** | macOS / Linux | Add `brews:` section to `.goreleaser.yml`; publish formula to a `homebrew-tap` companion repo |
| **winget** | Windows | Submit to `microsoft/winget-pkgs`; higher friction but reaches non-Scoop Windows users |

**GoReleaser references:**

- [Scoop support](https://goreleaser.com/customization/scoop/)
- [Homebrew support](https://goreleaser.com/customization/homebrew/)

---

### Claude Code plugin marketplace

Research publishing `sap-devs` as an official Claude Code cloud plugin via the [claude-plugins-official](https://github.com/anthropics/claude-plugins-official) marketplace.

**Why:** Claude Code users could install SAP developer context with a single command (`/install sap-devs`) instead of manually downloading the binary and running `inject`. The plugin system handles distribution, updates, and discovery.

**Research needed:**

- Plugin manifest format and submission requirements
- How the existing `sap-devs` CLI maps to the plugin model (skills, hooks, MCP servers)
- Whether the plugin can wrap/invoke the full CLI or needs a lighter adapter
- Approval process and timeline for marketplace listing

---

### Windows code signing for unsigned binary distribution

Unsigned `.exe` files downloaded from the internet are blocked or warned about by Windows SmartScreen. Investigate free signing options for OSS.

**Options (best to worst for OSS):**

1. **[SignPath.io](https://signpath.io)** — free tier for OSS projects; integrates with GitHub Actions; signs `.exe` artifacts; most straightforward path for a public SAP-samples repo
2. **Azure Trusted Signing — Community tier** — Microsoft's cloud signing service added a free OSS tier in 2024; requires an Azure account and some setup; integrates via `azure/trusted-signing-action` in the release workflow; signs produce full SmartScreen trust immediately
3. **OV code signing cert (self-managed)** — store PFX in GitHub Secrets; sign with `signtool.exe` in CI; OV certs still show a warning until SmartScreen builds reputation over time (many downloads)

**Note:** If distributing primarily via Scoop/Homebrew/winget, SmartScreen is largely a non-issue for the install path — package managers are already trusted. Signing is still useful for users who download the binary directly. Consider signing as a follow-up once package manager publishing is in place.

---

## Profiles

### Built-in profiles - DONE ✔️

#### `all` — dynamic catch-all - DONE ✔️

#### `minimal` — cost-conscious, ecosystem-only - DONE ✔️

### Shared base layer (auto-injected into every profile) - DONE ✔️

---

## Content System

### Additive content layers - DONE ✔️

---

### Multi-lingual content packs - DONE ✔️

---

## Commands

### `sap-devs news` - DONE ✔️

---

### `sap-devs influencers` - DONE ✔️

---

### `sap-devs config location` - DONE ✔️

---

### `sap-devs events` - DONE ✔️

---

### Content Editing UI

Interactive UI for editing and maintaining pack content YAML files (event-types.yaml, event-instances.yaml, influencers.yaml, resources.yaml, etc.).

**Goals:**

- Guided editor that understands JSON schemas — validates input, offers autocomplete for enums (scope, type, tags)
- Lower the barrier for content contributors who don't want to hand-edit YAML
- Could be terminal-based (Bubbletea TUI) or a local web UI served from the CLI

#### Phase 1 — Terminal UI (TUI) - DONE ✔️

- ✅ `sap-devs content edit <file>` — schema-driven TUI editor for content YAML files
- ✅ `sap-devs content validate` — validate all content YAML files against JSON schemas, with `--json` output for CI
- ✅ `sap-devs content list` — list all content files across active layers with `--pack`/`--layer`/`--json` filtering
- ✅ Schema-driven forms: enum fields render as select dropdowns, URI/pattern fields get inline validation, required field highlighting
- ✅ Layer resolution: all 4 layers (official, company, user, project) respected; user-layer overrides auto-created when editing from home directory

#### Phase 2 — TUI Enhancements

- Undo/redo support within the editor session - DONE ✔️
- Diff view showing changes against the lower-layer version before saving - DONE ✔️
- ~~Git commit/push integration~~ — dropped; developers use their own git tools
- Drag-and-drop reordering of list entries (e.g. resources, tips) - DONE ✔️
- Bulk editing — apply a change across multiple entries at once - DONE ✔️
- Content creation wizard — guided flow for adding a new pack from scratch - DONE ✔️

#### Phase 3 — Graphical UI (Optional)

A cross-platform graphical UI in the SAP Fiori design language could provide a more intuitive and visually appealing way to edit content files, leveraging familiar SAP Fiori components and patterns.

#### Phase 4 — Config editing - DONE ✔️

- ✅ `sap-devs config edit` — interactive TUI form for all config settings (general, preferences, events, sync TTLs)
- ✅ SAP Fiori Horizon Evening dark theme applied to all TUI forms and list views

---

## Tip Enhancements

### Friday SAP Developer News promotion - DONE ✔️

---

### Friday Developer News hook reminder - DONE ✔️

---

### Configurable tip rotation frequency - DONE ✔️

## Background Automation & System Tray

### Scheduled background sync and inject

Run `sync` and `inject` automatically on a schedule, without any user interaction, so content stays up to date silently in the background.

**Problem:** Users must remember to run `sap-devs sync` and `sap-devs inject` to keep AI tools current. Most won't. The tool should do this for them.

**Proposed approach:**

- **Daemon / background service** — a long-running process (or OS-scheduled task) that wakes on a configurable interval (e.g. every 6h), runs `sync` if the cache is stale, then `inject` if anything changed
- **Platform integration options:**
  - **Linux/macOS:** `systemd` user unit or `launchd` plist; `sap-devs service install/uninstall` writes the unit file and enables it
  - **Windows:** Windows Task Scheduler entry; `sap-devs service install` registers a scheduled task that runs at login and every N hours
- **Silent by default** — no terminal output; errors written to a rotating log in the cache directory (`~/.cache/sap-devs/daemon.log`)
- **`sap-devs service status`** — show last-run time, next-run time, and whether the service is registered
- **Change detection** — skip `inject` if pack hashes are unchanged since last run, to avoid unnecessary file writes and CLAUDE.md noise
- **Config keys:** `background_sync_interval` (default `6h`), `background_sync_enabled` (default `true` once service is installed)

**Dependency:** Closely coupled with the system tray feature below — the tray app is the natural host for the daemon on desktop platforms. But we need a script injection option to allow non-GUI environments (e.g., headless servers) to still benefit from automated sync and inject functionality.

---

### System tray icon (OS-appropriate visual interaction)

A persistent system-tray (or menu-bar) icon that surfaces tool status, triggers sync/inject on demand, and can start with OS login — making `sap-devs` a first-class background companion rather than a one-shot CLI.

**Problem:** Background automation is invisible. Users have no way to know whether content is current, whether sync failed, or what profile is active — without opening a terminal. A tray icon gives them a glanceable status and a right-click menu for common actions.

**Proposed approach:**

- **Host process:** a small GUI binary (`sap-devs-tray` or a sub-command `sap-devs tray`) that embeds the tray icon and drives the background scheduler
- **Cross-platform tray library options (Go):**
  - [`getlantern/systray`](https://github.com/getlantern/systray) — widely used, supports Windows, macOS, Linux (via AppIndicator)
  - [`fyne-io/fyne`](https://github.com/fyne-io/fyne) — heavier but full GUI toolkit if richer UI is needed later
- **Tray menu (v1 scope):**
  - Status line: "Last synced: 2h ago" / "Up to date" / "Sync failed"
  - Active profile name
  - Actions: **Sync now**, **Inject now**, **Open terminal**, **Settings**
  - **Quit**
- **Icon states:** idle (colour), syncing (animated or alternate icon), error (red badge)
- **OS startup:** `sap-devs tray --install-startup` writes the appropriate autostart entry (macOS `LaunchAgents`, Windows `HKCU\...\Run`, Linux `~/.config/autostart/`)
- **Notifications:** optional desktop notifications on sync completion or error (OS notification API via `gen2brain/beeep` or similar)
- **Architecture note:** This takes `sap-devs` into GUI / CGO territory for the first time. The tray binary should be a separate build target in GoReleaser to keep the main CLI binary free of CGO dependencies.

**Open questions:**

- Separate binary vs. sub-command (`sap-devs tray`)? Separate binary is cleaner for CGO isolation but adds distribution complexity.
- Should the tray host the scheduler, or should it be a thin UI on top of an OS service (systemd/launchd/Task Scheduler)?
- What richer UI surfaces are worth adding later — a mini dashboard, pack browser, tip popover?

---

## AI Agent Influence

### `sap-devs` as an MCP server

Expose `sap-devs` as a live MCP server so AI agents can query it on demand during a conversation, instead of relying solely on static injected text.

**Problem:** Static injection pushes everything upfront and hopes the agent reads it. With an MCP server, the agent pulls specific context when it needs it — no token budget pressure, always fresh, and topically relevant to the current task.

**Proposed tools the server would expose:**

- `get_tip(pack, topic)` — return one actionable tip, optionally filtered by topic keyword
- `search_resources(query, pack)` — return matching curated resources with URLs
- `get_context(profile)` — return the full context block for a profile on demand
- `get_recent_news()` — return latest SAP Developer News episodes
- `list_packs()` — enumerate available packs so the agent knows what domains are covered
- `get_known_errors(pattern)` — look up a SAP error message and return cause + fix (backing data: `known_errors.yaml` ✅)

**Architecture:**

- Implemented as a sub-command: `sap-devs mcp serve` starts the MCP server process
- Registered in the tool's MCP config via `sap-devs mcp install sap-devs-server` (reusing the existing `mcp-wire` adapter mechanism)
- Reads from the same pack content as `inject` — no duplicate data
- Stateless: each tool call loads packs fresh (or from cache); no daemon required

**Why this matters:** This inverts the whole model. Instead of "push everything and hope", the agent fetches what it needs, when it needs it. Particularly powerful for tools like Claude Code where the agent can decide mid-task that it needs CAP-specific context.

**Dependency:** Requires the base MCP infrastructure in `cmd/mcp.go` to be extended to support self-hosting, not just wiring third-party servers.

---

### Behavioral rules / anti-patterns injection - DONE ✔️

---

### ✅ Project-aware context detection on inject - DONE ✔️

> **Implemented** in `internal/project` package. `Detect()` scans project files at inject time, `Check()` runs health checks. Results flow into `sap-devs inject` (project context section) and `sap-devs doctor` (project health table with `--tools-only`/`--project-only` flags). See [design spec](docs/superpowers/specs/2026-04-19-project-detection-health-check-design.md).
>
> **Future work:** ABAP project detection (pending ADT-in-VSCode), UI5 standalone detection, pack-driven checks.yaml, auto-fix mode.

---

### ~~Error pattern library~~ - DONE ✔️

Implemented as `known_errors.yaml` per pack with `KnownError` struct, loader/merge/render integration, JSON schema, i18n, and `sap-devs errors list/search` CLI commands. Seed data: 7 CAP + 5 ABAP error patterns.

---

### ~~Agent-readable CLI manifest~~ - DONE ✔️

Implemented: added `## sap-devs CLI Reference (for AI agents)` table to `content/packs/base/context.md` with 19 commands covering when-to-use and output contracts.

---

### ~~Scratch/session context — `sap-devs context add`~~ ✅

Implemented: `context add/list/clear` commands with `.sap-devs/scratch.yaml` storage and `## Current Context` injection in project-scope output.

---

### ~~"What's changed since last sync" injection block~~ — DONE ✔️

Implemented: curated `changelog` entries in `pack.yaml`, collected by `sync` into `sync-changelog.json`, rendered as `## What's New` block by `inject`, consumed after one inject cycle. See [design spec](docs/superpowers/specs/2026-04-19-whats-new-injection-design.md).

---

### ~~Inject verbosity modes with semantic section tagging~~ — DONE ✔️

Implemented: `<!-- verbosity:core/detail/extended -->` markers in context.md, `VerbositySections` parser, per-adapter `verbosity` field, `--verbosity` CLI flag, synthetic section gating. See [design spec](docs/superpowers/specs/2026-04-19-inject-verbosity-modes-design.md).

---

## New Adapters

### ~~Zed editor adapter~~ — Covered by existing Claude Code adapter

---

### ~~Windsurf (Codeium) adapter~~ — DONE ✔️

---

### ~~Gemini Code Assist adapter~~ — DONE ✔️

---

## Content System Enhancements

### ~~Code sample pinning (`samples.yaml`)~~ — DONE ✔️

---

### BTP active context detection - DONE ✔️

---

### ~~Structured `context.md` conventions~~ — DONE ✔️

---

## Inject Enhancements

### Consolidate tool-injection content into the base pack + add AI-priority preamble - DONE ✔️

#### 1. Move tool-injection content into the base pack - DONE ✔️

#### 2. Add a preamble that biases AI agents toward `sap-devs` first - DONE ✔️

---

### `sap-devs inject --uninstall` (or `sap-devs uninject`) - DONE ✔️

---

### ✅ DONE: `sap-devs inject --status`

---

## Data Sources

### YouTube integration - DONE ✔️

---

### SAP Discovery Center integration - DONE ✔️

---

### developers.sap.com tutorial content — render and interactive execution

This is a three-phase feature:

#### Phase 1 — Content ingestion and rendering - DONE ✔️

#### Phase 2 — Guided execution

- `sap-devs tutorial run <id>` — interactive runner that walks through each step in sequence
- For code steps: display the snippet and optionally copy to clipboard or scaffold files in the current directory
- For CLI steps: display the command and optionally execute it with explicit user confirmation (no silent execution)
- Track completed steps in local state (e.g., `~/.local/share/sap-devs/tutorial-progress/`)

#### Integration points

- Inject active-tutorial context into AI tools via `inject` (e.g., "user is currently on step 3 of tutorial X — tailor suggestions accordingly")
- Likely closely related to `sap-devs learn` — `learn` recommends tutorials, `tutorial run` executes them; decide whether to bundle or keep separate
- Could feed into a future achievement or progress tracking system

#### Phase 3 — AI Agent as Instructor

Replace the static, step-by-step runner with an embedded AI agent that acts as a live instructor: interpreting the tutorial content, answering questions in context, adapting pacing to the user's progress, and validating outcomes intelligently.

**Core idea:** instead of the CLI mechanically displaying step N and waiting for a keypress, an AI instructor holds the tutorial state and converses with the user — explaining *why* a step works, catching common mistakes before they happen, and adjusting explanations based on what the user already knows (active profile + experience level).

**Capabilities:**

- Load the tutorial's structured step data as grounding context for the agent
- Observe CLI output and file diffs after each step; let the agent assess success/failure rather than rely on brittle string matching
- Answer user questions mid-tutorial without losing step state ("what does this flag do?", "why are we using CAP here?")
- Suggest corrections when the user's environment output diverges from expected
- Adapt explanation depth based on active profile and declared experience level (from `sap-devs learn`)
- Summarise progress and suggest next tutorials on completion

**Implementation sketch:**

- Invoked via `sap-devs tutorial run <id> --instructor` (or always on, with `--no-instructor` to opt out)
- Agent context = tutorial YAML + user profile + active pack context.md snippets (already generated by `inject`)
- Shell output capture: pipe stdout/stderr of confirmed commands back to the agent for assessment
- Model: Claude API (streaming); honour `ANTHROPIC_API_KEY` env var; graceful degradation to Phase 2 guided mode if no key is set
- Session state persisted alongside tutorial progress so users can resume a conversation mid-tutorial

**Open questions:**

- Should the instructor be a separate process (sidecar) or embedded in the CLI?
- How do we bound token usage for long tutorials — rolling context window, step-scoped resets, or summary compression?
- Can we use the MCP tool-use pattern so the instructor can directly call `sap-devs` subcommands on the user's behalf?

#### Open questions

- What structured data is available from developers.sap.com? (tutorials.sap.com has a JSON-backed metadata system worth investigating)
- Should execution be purely guided (display + confirm) or should the tool be able to run commands on the user's behalf?
