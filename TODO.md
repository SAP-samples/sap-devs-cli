# Backlog

Ideas and planned features for `sap-devs`. These are not commitments or a roadmap — they are a shared record of what we want to explore next. Contributions welcome: open an issue or PR to discuss any item.

---

## Release & Distribution

### Migrate to github.com/sap-samples

Move the repository from `github.tools.sap/developer-relations/sap-devs-cli` to `github.com/sap-samples` for public OSS distribution.

**Scope:**

- Create the repo under `github.com/sap-samples/sap-devs-cli` (or appropriate name)
- Update all import paths (`github.tools.sap/developer-relations/sap-devs-cli/...` → `github.com/sap-samples/...`)
- Update `github_urls` in `.goreleaser.yml` (remove the custom `api`/`upload`/`download` overrides — they're only needed for `github.tools.sap`)
- Update release workflow to use `GITHUB_TOKEN` against `github.com`
- Redirect or archive the internal repo; update any internal install instructions

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

### Multi-lingual content packs

i18n infrastructure is complete and all commands are wired (`en` + `de` catalogs). Remaining work:

- Add more language catalogs beyond `de` — `ja`, `fr`, `es`, `pt` are good candidates (add a JSON file to `internal/i18n/catalogs/`)
- Content pack localisation — `context.md`, `tips.md` per locale (pattern already exists for `cap` pack)

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

**Scope:**

- `sap-devs content edit <file>` — open an interactive editor for a specific content file
- `sap-devs content validate` — validate all content files against their schemas
- Support for all content YAML types: pack.yaml, resources.yaml, influencers.yaml, event-types.yaml, event-instances.yaml, mcp.yaml, tools.yaml, hook.yaml

Allows for the editing of content checked out from Git (for contributions to the global SAP Developers content repository), or your company's internal content repository, or the local editing of the overridden content in the user's environment. This ensures that content can be easily maintained and updated across different environments and use cases that already been separated and described in this tool.

Phase 2 - Graphical UI (Optional) to edit the files.
A cross platform graphical UI in the SAP Fiori design language could provide a more intuitive and visually appealing way to edit content files, leveraging familiar SAP Fiori components and patterns.

---

### `sap-devs learn` - DONE ✔️

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
- `get_known_errors(pattern)` — look up a SAP error message and return cause + fix

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

### Error pattern library

A `known_errors.yaml` per pack listing common SAP error messages, their causes, and fixes — injected as a compact reference and queryable via the MCP server.

**Problem:** When an agent encounters `Error: No 'default' database configured`, `AMDP method must be static`, or `EISDIR: illegal operation on a directory`, it guesses or web-searches. Most SAP errors have well-known, stable fixes that belong in the tool.

**YAML structure:**

```yaml
- id: cap-no-default-db
  pattern: "No 'default' database configured"
  pack: cap
  cause: No database binding in cds.requires; common in new projects or missing .env
  fix: Add `cds.requires.db.kind = sqlite` for local dev, or bind a HANA service for BTP
  docs: https://cap.cloud.sap/docs/node.js/databases
```

**Injection:** A compact `## Known Errors` section at the bottom of the injected block (or omitted entirely in `minimal` verbosity mode). Also the backing data for `get_known_errors(pattern)` in the MCP server.

**Collection strategy:** Seed from the most frequently asked questions in the SAP Community and CAP GitHub issues. Updated via `sync`.

---

### Agent-readable CLI manifest

Add a machine-readable summary of `sap-devs` commands and their output contracts to the injected base-pack block, so agents know exactly what to call and what they'll get back.

**Problem:** The current "Agent Instructions" section says "run `sap-devs resources`" in prose. Agents are better at calling tools when they know the precise interface: what arguments are accepted, what the output looks like, and when to use each command.

**Proposed injected block (in base pack `context.md`):**

```markdown
## sap-devs CLI Reference (for AI agents)

| Command | When to use | Output |
| --- | --- | --- |
| `sap-devs tip [--pack <name>]` | Need a quick best-practice reminder | One actionable tip as plain text |
| `sap-devs resources [--pack <name>]` | Need links to SAP docs, samples, tutorials | Numbered list with URLs |
| `sap-devs doctor` | User reports tool version issues | List of tool checks, pass/fail, fix hints |
| `sap-devs sync --force` | Content may be stale | Fetches latest SAP release notes and content |
| `sap-devs news` | User asks about recent SAP announcements | List of recent SAP Developer News episodes |
```

This is low-cost to author and maintain, and meaningfully improves agent precision when deciding which command to run.

---

### Scratch/session context — `sap-devs context add`

Let users append ephemeral working notes to the project-scope injected block, giving the AI agent facts specific to the current task rather than just generic SAP knowledge.

**Problem:** The agent knows CAP best practices. It doesn't know "I'm currently implementing draft enablement for the Books entity" or "the HANA service is only bound in the dev space, not test." This working context lives only in the developer's head.

**Commands:**

- `sap-devs context add "currently implementing draft enablement for Books entity"` — appends a note
- `sap-devs context list` — show current scratch notes
- `sap-devs context clear` — remove all notes (also cleared by `inject --uninstall`)

**Storage:** A `~/.config/sap-devs/scratch.yaml` per project directory (keyed by absolute path), rendered as a `## Current Context` section at the very top of the project-scope injected block — the first thing the agent reads.

**Design note:** Notes are intentionally ephemeral and human-authored. They are not synced, not versioned, and not shared. Their purpose is to close the gap between "what the pack knows" and "what I'm working on right now."

---

### "What's changed since last sync" injection block

When `inject` runs after a `sync` that pulled new content, prepend a brief delta note to the injected block so agents in active sessions learn about changes without the user having to tell them.

**Problem:** A developer runs `sap-devs sync` and then `sap-devs inject`. The AI agent's context window is refreshed, but the agent has no signal that anything changed. If the developer doesn't mention "CAP 9.8 is out," the agent continues reasoning from its training data.

**Proposed behaviour:**

- After each `sync`, record a brief human-readable changelog in `~/.cache/sap-devs/sync-changelog.yaml` (generated from diff of fetched content or from a `changelog` field in `pack.yaml`)
- On the next `inject`, prepend a `## What's New` block containing the last N changes (default: changes since the previous inject)
- Block is auto-removed after one inject cycle (it's a one-time nudge, not permanent content)

**Example output:**

```markdown
## What's New (since last sync, 2026-04-17)
- CAP 9.8: native SQLite support via `cds.requires.db.driver: node` (Node 22.5+)
- CAP 9.8: new `cds repl --ql` query mode for interactive CQL
- ABAP: new Tier-1 API released for business partner validation
```

---

### Inject verbosity modes with semantic section tagging

Give each section of `context.md` a verbosity tag so injection density can be controlled per-adapter without arbitrary truncation.

**Problem:** Claude Code's `CLAUDE.md` can be thousands of tokens; a clipboard export for ChatGPT is capped at ~1400 bytes. Currently the same content blob is injected everywhere, either overwhelming small-context tools or under-serving large-context ones. Truncation by byte count is arbitrary and cuts mid-sentence.

**Proposed approach:**

- Tag sections in `context.md` with HTML comments: `<!-- verbosity:core -->`, `<!-- verbosity:detail -->`, `<!-- verbosity:extended -->`
- Adapter YAML gains a `verbosity` field: `minimal` | `standard` (default) | `full`
- The renderer includes only sections at or below the adapter's verbosity level
- `core` = always included (preamble, constraints, CLI manifest); `detail` = best practices and examples; `extended` = release notes, known errors, full resource lists

**Why semantic over byte-count:** The agent gets a coherent, complete picture at whatever fidelity fits the tool — not a truncated fragment.

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

### BTP active context detection

If `btp` CLI is configured and logged in, detect the current target subaccount and space at inject time and include them in the project-scope injected block.

**Problem:** "User is in a BTP trial account, eu10 region, Cloud Foundry space: dev" changes advice significantly — trial limitations apply, productive HANA is unavailable, certain entitlements may not exist. Currently the agent has no idea what BTP environment the developer is targeting.

**Detection:** Run `btp target` (or parse `~/.btp/config.json`) at inject time; include results in the `## Project Context` section alongside the project-aware detection (see above). Silently skip if `btp` is not installed or not logged in.

**Privacy note:** Subaccount name and space name are included; no credentials, no account IDs beyond what the user has already shown by running `btp target`.

---

### Structured `context.md` conventions

Adopt conventional section headings across all pack `context.md` files so content is addressable, selectively includable, and easier for both agents and humans to navigate.

**Problem:** `context.md` is currently free-form markdown. This makes the verbosity tagging system (above) harder to retrofit, and means agents can't reference sections by name ("the Anti-patterns section says…").

**Proposed standard sections** (not all required in every pack):

| Section | Purpose |
| --- | --- |
| `## Overview` | What this technology is and when to use it |
| `## Key Concepts` | The 3–5 concepts an agent must understand |
| `## Best Practices` | What to do |
| `## Anti-patterns` | What not to do (feeds into `constraints.md`) |
| `## Code Examples` | Short, canonical inline snippets |
| `## Known Errors` | Common error patterns and fixes |
| `## Resources` | Key links (supplements `resources.yaml`) |

**Migration:** Apply to new packs immediately; retrofit existing packs incrementally. The schema can be validated as part of `cds lint` equivalent for content.

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
