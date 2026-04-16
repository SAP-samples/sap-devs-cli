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

### `sap-devs news`

Browse and open SAP Developer News episodes from the terminal.

**Subcommands:**
- `news` / `news list` — list recent episodes (default: last 10), most recent first
- `news latest` — open the most recent episode in the browser immediately
- `news open <id>` — open a specific episode by ID
- `news search <query>` — filter by title, description, or tags

**Data:** `news.yaml` per pack (date, title, URL, description, tags), loaded and merged by `ContentLoader`, updated via `sap-devs sync`. Start with static YAML; see YouTube integration below for live fetching.

---

### `sap-devs influencers`

Browse SAP community influencers and thought leaders relevant to your active profile.

**Subcommands:**
- `influencers` — list influencers matching your active profile's focus tags
- `influencers --all` — list all influencers across all packs
- `influencers --pack <name>` — filter by pack
- `influencers --random` — surface one influencer for discovery

**Data:** `influencers.yaml` per pack with `id`, `name`, `role`, `org`, `focus` tags, and `links` map (blog, github, twitter, youtube). Seed data: SAP Developer Advocates — DJ Adams, Thomas Jung, Marius Obert, Ian Thain, Gregor Wolf, Christian Gurke, Kevin Muessig.

---

### `sap-devs events`

Surface upcoming SAP community events from the CLI.

**Scope:**
- General event listing and calendaring
- Dedicated coverage for Devtoberfest (October), SAP TechEd, and CodeJams

---

### `sap-devs learn`

Guided learning recommendations based on the active profile and experience level.

**Scope:**
- Beginner / intermediate / advanced tier recommendations
- Recommendations draw from tutorials, docs, CodeJams, and sample projects
- Likely integrates with or feeds into the Discovery Center and tutorials features below

---

## Tip Enhancements

### Friday SAP Developer News promotion

Override the daily tip every Friday to always show a promotion for the SAP Developer News weekly show.

**Implementation:**
- Add `pinned_weekday: friday` field to the tip data model
- Add `SelectPinnedTip(packs, weekday)` to `internal/content/tip.go`
- In `cmd/tip.go`, check `time.Now().Weekday() == time.Friday` before the normal `SelectTip` call; fall through if no pinned tip is found

---

### Configurable tip rotation frequency

Let users control how often the tip changes. Current behaviour: once per calendar day.

**Proposed modes** (set via `sap-devs config set tip_rotation <mode>`):

| Mode | Behaviour |
|---|---|
| `daily` | Current default — same tip all day |
| `hourly` | New tip each hour |
| `session` | New tip every terminal session |

Also add `sap-devs tip --new` flag for a one-off fresh tip without changing config.

---

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

**Dependency:** Closely coupled with the system tray feature below — the tray app is the natural host for the daemon on desktop platforms.

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

## Inject Enhancements - DONE ✔️

## Data Sources

### YouTube integration

Fetch and process video metadata from the SAP Developers YouTube channel to keep `news.yaml` and `resources.yaml` current automatically.

**Channel:** https://www.youtube.com/@SAPDevelopers

**Key playlists:** SAP Developer News, CodeJam recordings, SAP TechEd sessions, tutorial series (CAP, ABAP, Fiori, BTP)

**Two-tier approach:**

1. **RSS fallback (no credentials required)** — YouTube exposes a public RSS feed per channel/playlist. Zero-config; limited to title, date, URL. Ships first.

2. **YouTube Data API v3** — richer metadata (tags, descriptions, playlist routing). API key stored via the existing credentials system (`sap-devs config token --service youtube`). Free tier (10,000 units/day) is sufficient for periodic sync.

**Sync integration:** New `youtube` category in `sync.Engine` with its own TTL (6–24h). Skips silently if no key is configured; existing static YAML remains the fallback.

**Dependency:** `sap-devs news` command must exist first.

---

### SAP Discovery Center integration

Integrate with [SAP Discovery Center](https://discovery.sap.com) for mission and tutorial discovery.

**Scope (TBD):**
- Browse and search Discovery Center missions
- Surface relevant missions based on active profile
- Likely bundled with or adjacent to `sap-devs learn`

---

### developers.sap.com tutorial content — render and interactive execution

Fetch, render, and interactively execute tutorials from developers.sap.com without leaving the terminal.

This is a two-phase feature:

#### Phase 1 — Content ingestion and rendering

- Fetch and cache tutorials from developers.sap.com as structured step data (via public API, JSON-LD, or sitemap — needs exploration)
- Store as YAML per pack; updated via `sap-devs sync` with its own TTL category
- `sap-devs tutorial list` — browse tutorials relevant to the active profile
- `sap-devs tutorial show <id>` — render a tutorial in the terminal: markdown output, step navigation (next/prev/jump), progress tracking, resume from last step

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
