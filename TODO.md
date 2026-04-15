# Backlog

Ideas and planned features for `sap-devs`. These are not commitments or a roadmap — they are a shared record of what we want to explore next. Contributions welcome: open an issue or PR to discuss any item.

---

## Profiles

### "All" profile — dynamic catch-all

Add a built-in `all` profile that automatically includes every pack available in the content layers, without needing a static `profiles/all.yaml` that must be kept in sync.

**Problem:** Users who want the full context (e.g. during content development or when working across multiple SAP domains) must either create a custom profile listing every pack ID, or switch profiles constantly. The list would go stale as new packs are added.

**Proposed approach:**

- Reserve the profile ID `all` in `ApplyWeights()` / profile resolution — when this ID is active, skip pack filtering entirely and load all packs from all layers
- `sap-devs profile set all` selects it; `sap-devs profile list` shows it as a built-in entry (not from a file)
- No `profiles/all.yaml` on disk — the behaviour is hardcoded so it never drifts out of sync
- Weight order for `all`: official packs first, then company, then user, then project (same as the content-layer merge order)

---

## Content System

### Additive content layers

Extend `ContentLoader` to support an `additive` merge mode per layer so a later layer can augment a pack rather than fully replace it.

**Problem:** The current system merges by ID — a company or user layer that wants to add a few extra tips must copy and re-maintain an entire pack. Additive layers would allow inheritance-style augmentation: keep the official pack, extend it.

**Proposed approach:**
- Add `additive: true` in `pack.yaml` to mark a pack as augmenting rather than replacing the lower-layer pack with the same ID
- `context.md` — append additive content after the base content
- `tips.md` — merge H2-delimited tip sections, preserving tips from both layers
- `tools.yaml`, `resources.yaml`, `mcp.yaml` — list-merge rather than replace
- Document conflict resolution when the same tip/tool/resource ID appears in both layers

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

## Inject Enhancements

### Inject optimisation and dynamic content

Research and implement optimisations to the `inject` pipeline and support for runtime-generated content that can't live in static pack files.

**Research areas:**

**Token / size optimisation**
- Profile current output size; establish per-adapter budgets (Claude Code CLAUDE.md vs Cursor `.cursorrules`, etc.)
- Per-adapter truncation: ranked sections, profile-weighted trimming, summary vs full detail modes
- Deduplication: strip content from lower-weight packs already covered by higher-weight packs

**Dynamic injection** (generated at inject time, not from pack files)
- Installed tool versions — run `doctor` checks and inject actual versions so the AI knows what's available
- Active BTP context — detect `~/.cf/config.json`, `~/.btp/config.json` for current org/space/subaccount
- Project type — detect `package.json`, `mta.yaml`, `.cdsrc.json` in CWD and inject a project-type summary
- Wired MCP servers — surface which SAP MCP servers are active
- Pack freshness — inject last-sync date so the AI knows how current the context is

**Adapter-specific rendering**
- Adapters declare `max_tokens` / `max_bytes` in their YAML; `RenderContext` trims accordingly
- Different adapters may want different formats (Markdown, XML system prompt tags, JSON)

**Incremental inject**
- Skip re-injection when content hash is unchanged (track in a state file)
- `--watch` mode for live reload during content development

---

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

#### Open questions

- What structured data is available from developers.sap.com? (tutorials.sap.com has a JSON-backed metadata system worth investigating)
- Should execution be purely guided (display + confirm) or should the tool be able to run commands on the user's behalf?
