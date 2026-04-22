# Backlog

Ideas and planned features for `sap-devs`. These are not commitments or a roadmap — they are a shared record of what we want to explore next. Contributions welcome: open an issue or PR to discuss any item.

---

## Release & Distribution

### ~~Migrate to github.com/SAP-samples~~ DONE ✔️

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

### npm wrapper package

Publish a lightweight npm package (`sap-devs` or `@sap/devs`) that downloads the platform-appropriate Go binary on `postinstall`. The target audience is CAP/Node.js developers who already have npm — `npx sap-devs inject` is zero-friction with no separate download step.

**Why:** The primary user persona (CAP developers) lives in the Node.js ecosystem. An npm wrapper meets them where they are — same install flow as `@sap/cds-dk`. Also unlocks `npx` for one-shot usage and makes version management automatic via `package.json`.

**Implementation sketch:**

- Thin JS wrapper: `postinstall` script downloads the correct binary from GitHub Releases into `node_modules/.bin/`
- Platform detection: `os.platform()` + `os.arch()` → map to GoReleaser artifact names
- Version pinning: npm package version matches Go release tag
- Graceful fallback: if binary download fails, print instructions for manual install
- GoReleaser can auto-generate the npm package or we maintain a small `npm/` directory in the repo

**References:**

- [`esbuild`](https://github.com/evanw/esbuild) uses this exact pattern — Go binary distributed via npm with platform-specific optionalDependencies
- [`turbo`](https://github.com/vercel/turbo) similarly wraps a Rust binary in npm

---

### MCP server registry listings

Once `sap-devs mcp serve` ships, list the MCP server on public registries so any MCP-compatible agent can discover and install it.

**Target registries:**

| Registry | URL | Notes |
| --- | --- | --- |
| **Smithery.ai** | <https://smithery.ai> | Largest MCP server directory; submit via PR |
| **mcp.run** | <https://mcp.run> | Cloudflare-backed MCP registry; supports hosted and self-hosted servers |
| **Glama.ai** | <https://glama.ai/mcp/servers> | Curated MCP server directory |
| **MCP Hub** | <https://github.com/nicholascote/mcp-hub> | Community-maintained registry |

**Why:** MCP is agent-agnostic. Listing in registries makes SAP context available to Claude, Cursor, Windsurf, Copilot, and any future MCP client — without building separate adapters for each.

**Dependency:** Requires `sap-devs mcp serve` to be functional and stable.

---

### IDE extension wrappers (VS Code / Cursor)

Publish a VS Code extension (also compatible with Cursor) that wraps `sap-devs inject` and MCP server wiring into the IDE marketplace.

**Why:** The VS Code / Cursor marketplace is a massive discovery surface. An extension can handle binary download, auto-inject on workspace open, surface tips in the status bar, and register the MCP server — all without the user touching a terminal.

**Scope (v1):**

- Download and manage the `sap-devs` binary
- Run `inject --project` on workspace open
- Register MCP server in Cursor/VS Code Copilot settings
- Status bar item showing active profile and last-sync time
- Command palette: Sync, Inject, Show Tip, Open Resources

**Open questions:**

- Publish under SAP-samples org or request official SAP publisher?
- Should the extension embed the binary or download on activate?

---

### GitHub Copilot Extensions

Research publishing `sap-devs` as a GitHub Copilot Extension, making SAP context available natively in Copilot Chat.

**Why:** Copilot has the largest market share of AI coding assistants. A Copilot Extension would let users `@sap-devs` in Copilot Chat to get SAP-specific guidance, similar to how the MCP server works but through Microsoft's agent extensibility model.

**Research needed:**

- Copilot Extensions API and submission requirements
- Whether it can wrap the existing MCP server or needs a separate HTTP agent
- GitHub Marketplace listing process for extensions
- Authentication and rate limiting considerations

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

### "Phone a Friend" — topic-matched expert recommendations

Surface the right SAP community expert to reach out to based on what the user is currently struggling with. Builds on the existing `influencers` command and data, but shifts from "browse the directory" to "who can help me with *this*?"

**Why:** Developers hit a wall and need human help — but the SAP ecosystem is vast and they don't know who to ask. "Phone a Friend" bridges the gap between curated influencer data and a developer's immediate need, turning the influencers list from a passive directory into an active recommendation engine.

**UX sketch:**

```text
$ sap-devs phone-a-friend "HANA deployment failing on CF"

  Looking for experts in: hana, btp, cf

  🎯 Recommended contacts:

  Thomas Jung — Developer Advocate @ SAP
    Focus: abap, cap, fiori, btp
    💬 community: https://community.sap.com/t5/user/viewprofilepage/user-id/139
    📝 blog: https://www.sap-press.com/authors/thomas-jung_697/

  DJ Adams — Developer Advocate @ SAP
    Focus: cap, fiori, nodejs, community, btp
    💬 community: https://community.sap.com/t5/user/viewprofilepage/user-id/53
    📝 blog: https://qmacro.org

  💡 Tip: Ask on SAP Community and tag them — they're active there!
```

**Core mechanics:**

- **Topic extraction:** Parse the user's free-text query into focus tags using keyword matching against the full tag vocabulary across all packs (e.g., "HANA deployment failing on CF" → `hana`, `btp`, `cf`)
- **Scoring:** Rank influencers by how many of their `focus` tags overlap with the extracted topics; break ties by pack relevance to the user's active profile
- **Contact surface priority:** Prefer `community` links (SAP Community profiles) since that's the best async channel for getting help; fall back to `blog`, `twitter`, `github`
- **Output:** Card-style display with name/role/focus, top contact links, and a contextual tip about how to reach out effectively

**Subcommand structure:**

| Command | Purpose |
| --- | --- |
| `sap-devs phone-a-friend <query>` | Recommend experts matching the query |
| `sap-devs phone-a-friend --topic <tag>` | Direct tag match (skip extraction) |
| `sap-devs phone-a-friend --open <id>` | Open the recommended expert's community profile |

**MCP integration:**

Expose as a `phone_a_friend` MCP tool so AI agents can proactively suggest experts when the user is stuck. The agent could call this tool after detecting repeated errors or frustration patterns, surfacing "you might want to ask Thomas Jung about this" inline.

**Data requirements:**

- Existing `influencers.yaml` data is sufficient for v1 — no schema changes needed
- `map[string]string` links field already supports arbitrary contact types
- Consider adding a `community` link to all influencers who have SAP Community profiles (some entries may be missing it)
- Future: add an `availability` or `responsive_on` field to indicate where each expert is most active

**Open questions:**

- Should the topic extraction be purely keyword-based, or use a lightweight fuzzy/synonym map (e.g., "Cloud Foundry" → `cf`, "HANA Cloud" → `hana`)?
- Should the output include a "compose a question" helper that drafts an SAP Community post template tagged with the right topics?
- Alias: should this also be available as `sap-devs ask` for brevity?

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

#### Phase 2 — TUI Enhancements - DONE ✔️

- Undo/redo support within the editor session - DONE ✔️
- Diff view showing changes against the lower-layer version before saving - DONE ✔️
- ~~Git commit/push integration~~ — dropped; developers use their own git tools
- Drag-and-drop reordering of list entries (e.g. resources, tips) - DONE ✔️
- Bulk editing — apply a change across multiple entries at once - DONE ✔️
- Content creation wizard — guided flow for adding a new pack from scratch - DONE ✔️

#### Phase 3 — Graphical UI (Optional) - DONE ✔️

A cross-platform graphical UI in the SAP Fiori design language could provide a more intuitive and visually appealing way to edit content files, leveraging familiar SAP Fiori components and patterns.

#### Phase 4 — Config editing - DONE ✔️

##### TUI config editor - DONE ✔️

- ✅ `sap-devs config edit` — interactive TUI form for all config settings (general, preferences, events, sync TTLs)
- ✅ SAP Fiori Horizon Evening dark theme applied to all TUI forms and list views

##### GUI config editor (tray) - DONE ✔️

- ✅ Webview-based config editor in `sap-devs-tray` — opened from tray context menu "Config..." or dashboard gear button
- ✅ Five collapsible Fiori panels: General, Preferences, Events, Sync TTLs, Service & Tray
- ✅ City typeahead with 647-city embedded database, IP-based location auto-detect via ip-api.com
- ✅ Client-side validation (URL format, integer ranges, Go duration syntax)
- ✅ Service install/uninstall and autostart management via subprocess calls to `sap-devs` CLI
- ✅ Sticky save bar with success/error feedback

---

## Tip Enhancements

### Friday SAP Developer News promotion - DONE ✔️

---

### Friday Developer News hook reminder - DONE ✔️

---

### Configurable tip rotation frequency - DONE ✔️

## Background Automation & System Tray

### ~~Scheduled background sync and inject~~ — DONE ✔️

Implemented as OS-native scheduler in `internal/service/` with platform-specific implementations behind build tags: Windows Task Scheduler (`schtasks`), macOS launchd (plist), Linux systemd (user timer). CLI: `sap-devs service install/uninstall/status`. Config key: `service.interval` (default 6h). Logs to `~/.cache/sap-devs/daemon.log`. See [design spec](docs/superpowers/specs/2026-04-20-system-tray-design.md) and [implementation plan](docs/superpowers/plans/2026-04-20-system-tray-1-os-scheduler.md).

---

### ~~System tray icon~~ — DONE ✔️

Implemented as a separate Wails v3 binary (`sap-devs-tray`) in `cmd/sap-devs-tray/` with its own `go.mod` — the main CLI stays CGO-free. The tray binary provides a system tray icon with context menu (Sync, Inject, Open Terminal, Quit) and a Fiori-themed webview dashboard panel showing sync status, active profile with packs, and injected tool detection. The main CLI manages the tray lifecycle via `internal/trayctl/` (download from GitHub Releases with SHA256 verification, start/stop, cross-platform autostart registration). CLI: `sap-devs tray install/uninstall/start/stop/status`. See [design spec](docs/superpowers/specs/2026-04-20-system-tray-design.md) and implementation plans ([lifecycle](docs/superpowers/plans/2026-04-20-system-tray-2-tray-lifecycle.md), [binary](docs/superpowers/plans/2026-04-20-system-tray-3-wails-binary.md)).

> **Alpha disclaimer:** Wails v3 is in alpha. The tray is strictly optional — all CLI features work without it.

---

## AI Agent Influence

### `sap-devs` as an MCP server - DONE  ✔️

Expose `sap-devs` as a live MCP server so AI agents can query it on demand during a conversation, instead of relying solely on static injected text.

**Problem:** Static injection pushes everything upfront and hopes the agent reads it. With an MCP server, the agent pulls specific context when it needs it — no token budget pressure, always fresh, and topically relevant to the current task.

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

### ~~Scratch/session context — `sap-devs context add`~~ DONE ✔️

Implemented: `context add/list/clear` commands with `.sap-devs/scratch.yaml` storage and `## Current Context` injection in project-scope output.

---

### ~~"What's changed since last sync" injection block~~ — DONE ✔️

Implemented: curated `changelog` entries in `pack.yaml`, collected by `sync` into `sync-changelog.json`, rendered as `## What's New` block by `inject`, consumed after one inject cycle. See [design spec](docs/superpowers/specs/2026-04-19-whats-new-injection-design.md).

---

### ~~Inject verbosity modes with semantic section tagging~~ — DONE ✔️

Implemented: `<!-- verbosity:core/detail/extended -->` markers in context.md, `VerbositySections` parser, per-adapter `verbosity` field, `--verbosity` CLI flag, synthetic section gating. See [design spec](docs/superpowers/specs/2026-04-19-inject-verbosity-modes-design.md).

---

### ~~MCP wrappers for BTP CLI and CF CLI~~ — DONE ✔️

Expose the `btp` and `cf` command-line tools as MCP tool surfaces, so AI agents can interact with them conversationally instead of the user having to context-switch to a terminal.

**Why:** Developers already use `btp` and `cf` heavily for subaccount management, service binding, app deployment, and space configuration. Wrapping these CLIs as MCP tools lets an agent run `cf apps`, `btp list accounts/subaccount`, `cf logs`, etc. on the user's behalf — with the agent interpreting the output and suggesting next steps. This turns the MCP server from a *knowledge* source into an *action* surface.

**Scope:**

- Detect whether `btp` and/or `cf` are installed (reuse `doctor` detection logic)
- Expose a curated set of read-only tools first (list, status, logs) — write operations (push, bind, create) gated behind explicit confirmation
- Pass through the user's existing CLI authentication (no separate OAuth flow) - catch unauthenticated errors and prompt the user to log in if necessary
- Map common multi-step workflows into higher-level tools (e.g., "deploy this CAP app" = `cf push` + `cf bind-service` + `cf start`)

**Open questions:**

- Which commands are safe to expose without confirmation vs. which need explicit user approval?
- Should the MCP tools wrap raw CLI output or parse it into structured JSON for the agent? - parse it into structured JSON!
- How to handle long-running commands (e.g., `cf push`) — streaming output vs. polling?

---

### ~~Expose `doctor` via MCP with install/fix capabilities~~ DONE ✔️

Implemented as two MCP tools: `check_tools` (tool installation status with per-OS install commands) and `check_project` (project health checks with fix suggestions). Both return structured JSON via `ResultEnvelope`. Shipped in PR #8.

---

### ~~MCP-to-MCP interactions with SAP ecosystem servers~~ — RESOLVED (No Action Needed) ✔️

Researched April 2026. The MCP spec (2025-03-26) is strictly client→server; there is no server-to-server protocol. A proxy pattern (sap-devs embedding mcp-go clients to downstream servers) is technically feasible but adds operational complexity without proportional benefit. Claude Code's plugin system already solves multi-server discovery — cds-mcp, ui5-mcp, and sap-devs all appear as independent servers in one agent session, and the LLM coordinates tool calls across them naturally.

**Decision:** Host-mediated composition (the current architecture) is correct. No proxy or aggregator needed. Revisit if the SAP MCP server count exceeds ~6, or if the MCP spec adds a server discovery protocol (SEP-2614).

**Near-term action:** Populate pack `mcp.yaml` files with downstream SAP server metadata so `sap-devs mcp install --all` can wire up the full SAP tool suite.

Full analysis: [docs/mcp-server.md § Cross-Server Orchestration](docs/mcp-server.md#cross-server-orchestration-mcp-to-mcp).

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

#### Phase 2 — Guided execution via MCP - DONE ✔️

Implemented as 4 MCP tools (`get_tutorial_step`, `update_tutorial_progress`, `get_tutorial_progress`, `list_active_tutorials`) plus a heuristic annotation engine that extracts executable commands, file creates, and verification checks from tutorial step markdown. The AI agent drives the tutorial flow — fetching steps, annotating content, tracking progress — while the MCP server stays stateless. Progress is stored in `tutorial-progress.json` in the XDG data directory, shared between MCP tools and the existing TUI.

New files: `internal/tutorials/annotate.go` (annotation engine), `internal/mcpserver/tools_tutorial_exec.go` (MCP handlers), with tests alongside.

#### Phase 2+ — Tutorial Instructor Skill & MCP Enhancements - DONE ✔️

Deliver the "AI Agent as Instructor" vision through a Claude Code skill and targeted MCP tool enhancements, rather than building a custom embedded agent. Analysis (April 2026, documented in `docs/mcp-server.md § Tutorial Guided Execution — Phase 3 Analysis`) concluded that the host AI tool (Claude Code, Cursor) already provides the agent runtime, shell execution, output observation, and streaming UI — embedding a second agent adds complexity without proportional benefit.

**Scope:**

- **Claude Code skill** (`/tutorial`) — orchestrates existing MCP tools with pedagogical awareness: pacing, verification, error recovery, profile-aware explanations, comprehension checks
- **MCP tool enhancements:**
  - `search_tutorials` — add level and duration to results
  - `get_tutorial_step` — add prev/next step titles, tutorial level and duration
  - New `recommend_tutorials` tool — profile-matched suggestions + active tutorials in one call
  - Annotation confidence tagging and prerequisite extraction
- **Discoverability** — proactive resume prompting, context-aware suggestions, one-call recommendations
- **No new dependencies** — no Claude API key, no provider lock-in, works with any MCP-compatible agent

**Stretch goal (standalone agent):** If demand emerges for tutorial instruction without an external AI tool, revisit the embedded agent approach. Unlikely given the trajectory of AI coding tool adoption.

#### Phase 3 — Inline image display for tutorials - DONE ✔️

Implemented inline tutorial images in the MCP server. `get_tutorial_step` now extracts image references from tutorial markdown, resolves relative paths to full GitHub raw URLs, and returns images as MCP `ImageContent` blocks alongside the text JSON response. A new `get_tutorial_image` tool fetches individual images on demand. Images are cached locally with SHA256 hash-prefixed filenames to prevent collisions. A 10 MB size limit protects against oversized downloads.

**Key design decisions:**

- **Dual output strategy:** `ImageContent` blocks give the AI model vision capabilities (it can describe what it sees in screenshots), while agent instructions ensure clickable `[see screenshot](url)` links are always included in text output for clients that don't render `ImageContent` (e.g., VS Code Claude extension)
- **`include_images` parameter** (default `true`): when `false`, returns resolved URLs only without fetching — reduces latency for text-only workflows
- **Image URLs always resolved** in markdown content regardless of `include_images` mode, so links work even without fetching

New files: `internal/tutorials/images.go` (extraction, fetching, caching), updates to `internal/mcpserver/tools_tutorial_exec.go` (handlers), with tests alongside. Shipped in PR #16.
