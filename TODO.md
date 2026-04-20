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

### GitHub Packages — not planned

GitHub Packages (Go module proxy) is **not a priority**. `sap-devs` is a CLI binary, not a Go library — the target audience (SAP/CAP/Node.js developers) won't use `go install`. GitHub Releases via GoReleaser is the correct artifact host for binary distribution. Revisit only if a Go library extraction becomes relevant.

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

#### Phase 2 — TUI Enhancements - DONE ✔️

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
